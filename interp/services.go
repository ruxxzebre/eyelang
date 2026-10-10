package interp

import (
	"bytes"
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
	"time"

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
	v, ok := lookupSecret(name)
	if !ok {
		in.faultf(at, "secret %q isn't set: set %s, or put a file %s in $EYE_SECRETS", name, envName(name), name)
	}
	return &SecretVal{name: name, value: v}
}

// httpClient is shared by contracts, http.get and model calls.
var httpClient = &http.Client{Timeout: 60 * time.Second}

// retryDelays are the waits between tries of a query whose service doesn't
// answer: about a minute in all, after which the run ends as Unavailable.
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
	wire := op.Contract.HTTP
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
			segments[i] = url.PathEscape(fmt.Sprint(in.toWire(v, wire.Snake)))
			delete(given, name)
		}
	}
	path := strings.Join(segments, "/")
	target := base + path
	var body []byte
	if op.HTTP.Method == "GET" || op.HTTP.Method == "DELETE" {
		q := url.Values{}
		for _, name := range order {
			v, ok := given[name]
			if !ok {
				continue
			}
			switch x := in.toWire(v, wire.Snake).(type) {
			case []any:
				for _, e := range x {
					q.Add(wireName(name, wire.Snake), fmt.Sprint(e))
				}
			case nil:
			default:
				q.Set(wireName(name, wire.Snake), fmt.Sprint(x))
			}
		}
		if len(q) > 0 {
			target += "?" + q.Encode()
		}
	} else {
		obj := map[string]any{}
		for _, name := range order {
			if v, ok := given[name]; ok {
				obj[wireName(name, wire.Snake)] = in.toWire(v, wire.Snake)
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
		h := op.KeyHeader
		if h == "" {
			h = "Idempotency-Key"
		}
		headers.Set(h, show(key))
	}
	if wire.AuthSecret != "" {
		s := in.secret(wire.AuthSecret, at).value
		if wire.AuthBearer {
			s = "Bearer " + s
		}
		headers.Set(wire.AuthHeader, s)
	}

	name := op.Contract.Module.Name + "." + op.Name
	uncertain := func() Value {
		u := &Variant{V: in.prog.Prelude.Variants["Uncertain"], Vals: []Value{key}}
		if key == nil {
			u.Vals = []Value{fmt.Sprintf("%s-%d", name, time.Now().UnixNano())}
		}
		rt.uncertain[keyOf(u.Vals[0])] = sentCommand{op, args, named}
		return u
	}
	httpErr := httpErrorType(op.Return)
	for try := 0; ; try++ {
		status, reply, err := doRequest(op.HTTP.Method, target, headers, body)
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
		case status >= 200 && status < 300:
			t := op.Return
			if httpErr != nil {
				t = op.Return.Args[0]
			}
			x, derr := decodeJSON(reply)
			var v Value
			if derr == nil {
				v, derr = in.fromWire(x, t, wire.Snake)
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
		if !retry || try >= len(retryDelays) {
			panic(&Fault{Pos: at, Msg: name + " stayed out of reach", Kind: "Unavailable", Arg: name})
		}
		time.Sleep(rt.retryDelay(try))
	}
}

func (rt *runtime) retryDelay(try int) time.Duration {
	if rt.fastRetries {
		return time.Millisecond
	}
	return retryDelays[try]
}

func doRequest(method, target string, headers http.Header, body []byte) (int, []byte, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, target, r)
	if err != nil {
		return 0, nil, err
	}
	req.Header = headers
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	return resp.StatusCode, data, err
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
func httpErrorType(t *ir.TypeExpr) *ir.Type {
	if t == nil || t.Name != "Result" || len(t.Args) != 2 {
		return nil
	}
	e := t.Args[1].Decl
	if e == nil || e.Name != "HttpError" || e.Module == nil || e.Module.Path != "http" {
		return nil
	}
	return e
}

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
	"http.get":    hostHTTPGet,
	"ai.generate": hostGenerate,
}

// callHost runs an extern fn with a host function.
func (in *Interp) callHost(fn *ir.Func, args []argVal, at ir.Pos) Value {
	h, ok := hostFuncs[fn.Host]
	if !ok {
		in.faultf(at, "%s has no body, and the interpreter provides no host function %q", fn.Name, fn.Host)
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
	target, _ := params["url"].(string)
	headers := http.Header{}
	if m, ok := params["headers"].(*Map); ok {
		for i, k := range m.Keys {
			headers.Set(show(k), show(m.Vals[i]))
		}
	}
	client := httpClient
	if via, ok := params["via"].(*Variant); ok && via.V == in.pre.some {
		proxy, err := url.Parse(show(via.Vals[0]))
		if err != nil {
			in.faultf(at, "http.get's via: isn't a URL: %v", err)
		}
		client = &http.Client{Timeout: httpClient.Timeout, Transport: &http.Transport{Proxy: http.ProxyURL(proxy)}}
	}
	httpMod := in.stdModule("http", at)
	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		in.faultf(at, "http.get: %v", err)
	}
	req.Header = headers
	resp, err := client.Do(req)
	if err != nil {
		return in.errV(in.httpError(httpMod.Types["HttpError"], err, 0, ""))
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return in.errV(in.httpError(httpMod.Types["HttpError"], err, 0, ""))
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
