package interp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"eyelang/internal/wire"
	"eyelang/ir"
)

// A running program finds the services its contracts describe, and the
// secrets they use, in its environment (SPECS.md §17):
//
//	NOTIFY_URL     the base URL of the service behind contract module notify;
//	               module lab.notify reads LAB_NOTIFY_URL
//	NOTIFY_TOKEN   secret("notify-token"): the name in upper case, with - and . as _
//	EYE_SECRETS    a directory with a file per secret, read when no variable is set
//
// Nothing about a service's address or credentials is written in the code.

// envName turns a module path or a secret's name into an environment
// variable's name: lab.notify → LAB_NOTIFY, notify-token → NOTIFY_TOKEN.
func envName(name string) string {
	return strings.ToUpper(strings.NewReplacer("-", "_", ".", "_").Replace(name))
}

// serviceURL is the base URL configured for a contract module.
func serviceURL(module string) (string, bool) {
	u, ok := os.LookupEnv(envName(module) + "_URL")
	return strings.TrimSuffix(u, "/"), ok && u != ""
}

// lookupSecret reads a secret from the environment or from $EYE_SECRETS.
func lookupSecret(name string) (string, bool) {
	if v, ok := os.LookupEnv(envName(name)); ok {
		return v, true
	}
	if dir := os.Getenv("EYE_SECRETS"); dir != "" {
		if data, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			return strings.TrimRight(string(data), "\r\n"), true
		}
	}
	return "", false
}

// SecretVal is a Secret(String). It shows as its name only, and can't be
// saved, so it never reaches a log or a stored step.
type SecretVal struct {
	name, value string
}

func (in *Interp) secret(name string, at ir.Pos) *SecretVal {
	v, ok := in.lookupSecret(name)
	if !ok {
		in.faultf(at, "secret %q isn't set: set %s, or put a file %s in $EYE_SECRETS", name, envName(name), name)
	}
	return &SecretVal{name: name, value: v}
}

// httpClient is shared by contracts, http.get and model calls.
var httpClient = &http.Client{Timeout: 60 * time.Second}

// retryDelays are the waits between tries of a query whose service doesn't
// answer: about a minute in all, after which the run ends as Unavailable, or
// the call returns Err when its result says how it fails (http.get and
// ai.generate).
var retryDelays = []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second}

// notSent reports whether a request failed before it reached the service, so
// that sending it again can't do anything twice.
func notSent(err error) bool {
	var op *net.OpError
	if errors.As(err, &op) && op.Op == "dial" {
		return true
	}
	var dns *net.DNSError
	return errors.As(err, &dns)
}

// tryAgain waits before the next try of a call whose service didn't answer,
// try being the number of tries so far less one. It reports false, without
// waiting, when the tries are used up. Other tasks run while it waits.
func (in *Interp) tryAgain(try int) bool {
	if try >= len(retryDelays) {
		return false
	}
	rt := in.rt
	if rt.beforeRetry != nil {
		rt.beforeRetry(try)
	}
	delay := rt.retryDelay(try)
	in.blocking(func(ctx context.Context) {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
		}
	})
	return true
}

// whyNoAnswer says, in words, why a request to target had no answer: "www.rte.ie
// refused the connection". via is the proxy it went through, if any, which is
// then what refused or couldn't be found. tries is how many times it was sent.
func whyNoAnswer(target, via string, err error, tries int) string {
	who := target
	if u, perr := url.Parse(target); perr == nil && u.Hostname() != "" {
		who = u.Hostname()
	}
	if via != "" {
		who = "the proxy " + via
		if u, perr := url.Parse(via); perr == nil && u.Hostname() != "" {
			who = "the proxy " + u.Hostname()
		}
	}
	var dns *net.DNSError
	var op *net.OpError
	var why string
	switch {
	case errors.As(err, &dns):
		why = who + " couldn't be found: the name " + dns.Name + " doesn't resolve"
	case errors.Is(err, syscall.ECONNREFUSED):
		why = who + " refused the connection"
	case errors.As(err, &op) && op.Op == "dial" && op.Timeout():
		why = who + " didn't accept the connection in time"
	case errors.As(err, &op) && op.Op == "dial":
		why = who + " couldn't be reached: " + op.Err.Error()
	case errors.Is(err, syscall.ECONNRESET), errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		why = who + " closed the connection before answering"
	default:
		why = who + ": " + err.Error()
		var ue *url.Error
		if errors.As(err, &ue) {
			why = who + ": " + ue.Err.Error() // without the method and URL Go puts first
		}
	}
	if tries > 1 {
		why += fmt.Sprintf(", %d times over %s", tries, triedFor(tries))
	}
	return why
}

// triedFor is how long tries took, as retryDelays spaces them, in words.
func triedFor(tries int) string {
	var d time.Duration
	for _, w := range retryDelays[:min(tries-1, len(retryDelays))] {
		d += w
	}
	if d >= time.Minute {
		return "a minute"
	}
	return d.Round(time.Second).String()
}

// callHTTP calls an operation of a contract over HTTP, when the operation
// says how (@http) and a URL is configured for its module. ok is false when
// it can't, so the caller reports what's missing.
func (rt *runtime) callHTTP(op *ir.Op, args []Value, named map[string]Value, at ir.Pos) (Value, bool) {
	if op.HTTP == nil {
		return nil, false
	}
	base, ok := serviceURL(op.Contract.Module.Path)
	if !ok {
		return nil, false
	}
	in := rt.in
	shared := op.Contract.HTTP // what the contract's operations share
	// The arguments by parameter, defaults filled in. A None is left out,
	// which is how JSON says a value is absent.
	given := map[string]Value{}
	for i, a := range args {
		if i < len(op.Params) {
			given[op.Params[i].Name] = a
		}
	}
	var key Value
	for name, v := range named {
		if name == "key" && op.Key {
			key = v
			continue
		}
		given[name] = v
	}
	var order []string
	for _, p := range op.Params {
		v, ok := given[p.Name]
		if !ok && p.Default != nil {
			v = in.eval(&frame{fn: &ir.Func{Name: op.Name}, slots: make([]Value, 16)}, p.Default)
			given[p.Name] = v
		}
		if variant, isVariant := v.(*Variant); isVariant && variant.V == in.pre.none {
			delete(given, p.Name)
			continue
		}
		if _, ok := given[p.Name]; ok {
			order = append(order, p.Name)
		}
	}
	if op.Key && key == nil {
		in.faultf(at, "%s.%s takes a key: %s(…, key: \"…\")", op.Contract.Module.Name, op.Name, op.Name)
	}

	// A path segment `:name` takes the argument of that name.
	segments := strings.Split(op.HTTP.Path, "/")
	for i, seg := range segments {
		if name, ok := strings.CutPrefix(seg, ":"); ok {
			v, ok := given[name]
			if !ok {
				in.faultf(at, "%s's path needs %s, which has no value", op.Name, name)
			}
			segments[i] = url.PathEscape(fmt.Sprint(in.toWire(v, shared.Snake)))
			delete(given, name)
		}
	}
	path := strings.Join(segments, "/")
	target := base + path
	var body []byte
	if wire.InQuery(op.HTTP.Method) {
		q := url.Values{}
		for _, name := range order {
			v, ok := given[name]
			if !ok {
				continue
			}
			switch x := in.toWire(v, shared.Snake).(type) {
			case []any:
				for _, e := range x {
					q.Add(wireName(name, shared.Snake), fmt.Sprint(e))
				}
			case nil:
			default:
				q.Set(wireName(name, shared.Snake), fmt.Sprint(x))
			}
		}
		if len(q) > 0 {
			target += "?" + q.Encode()
		}
	} else {
		obj := map[string]any{}
		for _, name := range order {
			if v, ok := given[name]; ok {
				obj[wireName(name, shared.Snake)] = in.toWire(v, shared.Snake)
			}
		}
		if key != nil && op.KeyField != "" {
			obj[op.KeyField] = in.toWire(key, false)
		}
		body, _ = json.Marshal(obj)
	}

	headers := http.Header{}
	if body != nil {
		headers.Set("Content-Type", "application/json")
	}
	headers.Set("Accept", "application/json")
	if key != nil {
		headers.Set(wire.KeyHeader(op), show(key))
	}
	if shared.AuthSecret != "" {
		s := in.secret(shared.AuthSecret, at).value
		if shared.AuthBearer {
			s = "Bearer " + s
		}
		headers.Set(shared.AuthHeader, s)
	}

	name := op.Contract.Module.Name + "." + op.Name
	uncertain := func() Value {
		u := &Variant{V: in.prog.Prelude.Variants["Uncertain"], Vals: []Value{key}}
		if key == nil {
			u.Vals = []Value{fmt.Sprintf("%s-%d", name, time.Now().UnixNano())}
		}
		rt.lost(keyOf(u.Vals[0]), sentCommand{op, args, named}) // kept in the journal until settled
		return u
	}
	httpErr := httpErrorType(op.Return)
	rt.durable() // the service sees the call
	for try := 0; ; try++ {
		status, reply, err := in.doRequest(op.HTTP.Method, target, headers, body)
		retry := false
		switch {
		case err != nil && op.Command && !notSent(err):
			return uncertain(), true // it may have arrived: only reconcile can tell
		case err != nil:
			if httpErr != nil {
				return in.errV(in.httpError(httpErr, err, 0, "")), true
			}
			retry = true
		case op.HTTP.Status[status] != "":
			v, derr := in.decodeStatus(op, status, reply)
			if derr != nil {
				in.faultf(at, "%s answered %d with a body that isn't %s: %v", name, status, op.HTTP.Status[status], derr)
			}
			return v, true
		case op.Fresh && (status == wire.StaleStatus || status == wire.ExpiredStatus):
			// A @fresh command's service refused it: the state changed after
			// basedOn (409), or what it was based on is gone (410).
			if status == wire.StaleStatus {
				return &Variant{V: in.pre.stale}, true
			}
			return &Variant{V: in.pre.expired}, true
		case status >= 200 && status < 300:
			t := op.Return
			if httpErr != nil {
				t = op.Return.Args[0]
			}
			x, derr := decodeJSON(reply)
			var v Value
			if derr == nil {
				v, derr = in.opResult(op, x, t, shared.Snake)
			}
			if derr != nil {
				in.faultf(at, "%s answered with a body that isn't %s: %v", name, typeString(t), derr)
			}
			if httpErr != nil {
				v = in.ok(v)
			}
			return v, true
		case httpErr != nil:
			return in.errV(in.httpError(httpErr, nil, status, string(reply))), true
		case op.Command && status >= 500:
			return uncertain(), true
		case status >= 500 || status == 429:
			retry = true
		default:
			in.faultf(at, "%s was refused with status %d: %s", name, status, clipText(string(reply), 300))
		}
		if !retry || !in.tryAgain(try) {
			panic(&Fault{Pos: at, Msg: name + " stayed out of reach", Kind: "Unavailable", Arg: name})
		}
	}
}

func (rt *runtime) retryDelay(try int) time.Duration {
	if rt.fastRetries {
		return time.Millisecond
	}
	return retryDelays[try]
}

// doRequest makes an HTTP request. Other tasks run while it waits, and a
// task cancelled meanwhile stops when it returns.
func (in *Interp) doRequest(method, target string, headers http.Header, body []byte) (status int, data []byte, err error) {
	in.blocking(func(ctx context.Context) {
		var r io.Reader
		if body != nil {
			r = bytes.NewReader(body)
		}
		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, method, target, r)
		if err != nil {
			return
		}
		req.Header = headers
		var resp *http.Response
		resp, err = httpClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		status = resp.StatusCode
		data, err = io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	})
	return status, data, err
}

func decodeJSON(data []byte) (any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	var x any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	err := dec.Decode(&x)
	return x, err
}

// decodeStatus reads a reply whose status @http maps to a variant: the body's
// fields are the variant's.
func (in *Interp) decodeStatus(op *ir.Op, status int, reply []byte) (Value, error) {
	name := op.HTTP.Status[status]
	if v := in.freshOutcome(op, name, false); v != nil {
		return v, nil // status: { 412: Stale }
	}
	d := op.Return.Decl
	if httpErrorType(op.Return) != nil {
		d = op.Return.Args[0].Decl
	}
	if d == nil {
		return nil, fmt.Errorf("its result isn't a variant type")
	}
	var variant *ir.Variant
	for _, v := range d.Variants {
		if v.Name == name {
			variant = v
		}
	}
	if variant == nil {
		return nil, fmt.Errorf("%s has no variant %s", d.Name, name)
	}
	x, err := decodeJSON(reply)
	if err != nil {
		return nil, err
	}
	obj, _ := x.(map[string]any)
	if obj == nil {
		obj = map[string]any{}
	}
	v, err := in.decodeFields(obj, variant, bindParams(d, op.Return.Args, nil), op.Contract.HTTP.Snake, "")
	if err == nil && httpErrorType(op.Return) != nil {
		v = in.ok(v)
	}
	return v, err
}

// httpErrorType returns http's HttpError when t is Result(T, http.HttpError):
// a query that handles its own failures, which come back as Err instead of
// being retried.
func httpErrorType(t *ir.TypeExpr) *ir.Type { return wire.HTTPErrorType(t) }

// httpError makes an http.HttpError: no answer, or an answer with a status
// that isn't a success.
func (in *Interp) httpError(t *ir.Type, err error, status int, body string) Value {
	variant := func(name string) *ir.Variant {
		for _, v := range t.Variants {
			if v.Name == name {
				return v
			}
		}
		panic("http.HttpError has no variant " + name)
	}
	var netErr net.Error
	switch {
	case err != nil && errors.As(err, &netErr) && netErr.Timeout():
		return &Variant{V: variant("TimedOut")}
	case err != nil:
		return &Variant{V: variant("NoAnswer"), Vals: []Value{err.Error()}}
	}
	return &Variant{V: variant("BadStatus"), Vals: []Value{int64(status), clipText(body, 4000)}}
}

// httpGetError is http.get's HttpError for a request with no answer:
// TimedOut, or NoAnswer with the reason in words.
func (in *Interp) httpGetError(t *ir.Type, target, via string, err error, tries int) Value {
	v := in.httpError(t, err, 0, "")
	if nv, ok := v.(*Variant); ok && nv.V.Name == "NoAnswer" {
		nv.Vals = []Value{whyNoAnswer(target, via, err, tries)}
	}
	return v
}

func clipText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ---------------------------------------------------------------- host functions

// hostFunc runs an extern fn that the interpreter provides. params holds the
// arguments by parameter name, defaults filled in.
type hostFunc func(in *Interp, params map[string]Value, at ir.Pos) Value

var hostFuncs = map[string]hostFunc{
	"http.get": hostHTTPGet,
}

// ai.generate runs the functions it is given as tools, so it is added
// when the package starts rather than in the table it is reached through.
func init() { hostFuncs["ai.generate"] = hostGenerate }

// callHost runs an extern fn: with a host function, the interpreter's own
// or one the embedding program registered, or with a C function
// (externs.go).
func (in *Interp) callHost(fn *ir.Func, args []argVal, at ir.Pos) Value {
	h, err := in.externFunc(fn)
	if err != nil {
		in.faultf(at, "%v", err)
	}
	in.rt.durable() // what it does may be seen outside
	if fn.Host != "" {
		in.rt.crossed(fn.Host)
	} else {
		in.rt.crossed("extern " + fn.Name)
	}
	fr := &frame{fn: fn, slots: make([]Value, fn.Slots)}
	in.bind(fn, fr, args, at)
	params := map[string]Value{}
	for _, p := range fn.Params {
		params[p.Name] = fr.slots[p.Slot]
	}
	return h(in, params, at)
}

// hostHTTPGet is http.get: an answer of any status is Ok; Err is no answer.
func hostHTTPGet(in *Interp, params map[string]Value, at ir.Pos) Value {
	if v, ok := in.fakeHTTPGet(params, at); ok {
		return v
	}
	target, _ := params["url"].(string)
	v := in.httpGet(target, params, at)
	in.rt.recordGet(target, v)
	return v
}

// httpGet reads target over HTTP, for http.get.
func (in *Interp) httpGet(target string, params map[string]Value, at ir.Pos) Value {
	headers := http.Header{}
	if m, ok := params["headers"].(*Map); ok {
		for i, k := range m.Keys {
			headers.Set(show(k), show(m.val(i)))
		}
	}
	client, via := httpClient, ""
	if v, ok := params["via"].(*Variant); ok && v.V == in.pre.some {
		via = show(v.Vals[0])
		proxy, err := url.Parse(via)
		if err != nil {
			in.faultf(at, "http.get's via: isn't a URL: %v", err)
		}
		client = &http.Client{Timeout: httpClient.Timeout, Transport: &http.Transport{Proxy: http.ProxyURL(proxy)}}
	}
	httpMod := in.stdModule("http", at)
	var resp *http.Response
	var data []byte
	var err error
	// A request that never reached the server is sent again, as a query's
	// is; one that went out and had no answer in time isn't.
	for try := 0; ; try++ {
		in.blocking(func(ctx context.Context) {
			var req *http.Request
			req, err = http.NewRequestWithContext(ctx, "GET", target, nil)
			if err != nil {
				return
			}
			req.Header = headers
			resp, err = client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			data, err = io.ReadAll(io.LimitReader(resp.Body, 32<<20))
		})
		if err == nil || !notSent(err) || !in.tryAgain(try) {
			if err != nil {
				return in.errV(in.httpGetError(httpMod.Types["HttpError"], target, via, err, try+1))
			}
			break
		}
	}
	var keys, vals []Value
	names := make([]string, 0, len(resp.Header))
	for k := range resp.Header {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		keys, vals = append(keys, strings.ToLower(k)), append(vals, resp.Header.Get(k))
	}
	return in.ok(&Record{
		Type:  httpMod.Types["Response"],
		Names: []string{"status", "headers", "body"},
		Vals:  []Value{int64(resp.StatusCode), newMap(keys, vals), string(data)},
	})
}

// stdModule is a standard module the program imports.
func (in *Interp) stdModule(path string, at ir.Pos) *ir.Module {
	for _, m := range in.prog.Modules {
		if m.Path == path && m.Dir == "" {
			return m
		}
	}
	in.faultf(at, "the program doesn't import %s", path)
	return nil
}
