package interp

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"eyelang/internal/wire"
	"eyelang/ir"
)

// Serving contracts over HTTP (SPECS.md §17). eye run listens on the port of
// each contract the program serves, and answers a request the way a caller
// sends it (callHTTP): the operation's @http method and path, its arguments
// in the path, the query or a JSON body, the key in its header, the
// credential @auth names. Each request runs the operation's impl as one
// transaction, and its result goes back in the one JSON shape, with the
// status that `status:` gives its variant, else 200.
//
// Requests take the interpreter's lock (gil), as parallel tasks do, so they
// are answered one at a time: between the program's steps, and while a run
// waits for a timer or another service. An operation never waits, so a
// request holds the lock only while its transaction runs.
//
// NOTIFY_PORT (the module's name, as for NOTIFY_URL) serves contract module
// notify on another port; 0 picks a free one, which the start line names.

// listener serves one contract on its port.
type listener struct {
	ct     *ir.Contract
	srv    *http.Server
	addr   string // where it listens, as the start line shows it: ":3001"
	auth   string // the value @auth's header must have; "" without @auth
	routes []*route
}

// route is one operation's request: its method and its path's segments.
type route struct {
	op       *ir.Op
	method   string
	segments []string // ":name" takes the argument of that name
}

// servedContracts returns the contracts the program serves with impl, by
// module and name.
func servedContracts(prog *ir.Program) []*ir.Contract {
	var out []*ir.Contract
	for _, m := range prog.Modules {
		for _, ct := range m.Contracts {
			for _, op := range ct.Ops {
				if op.Impl != nil {
					out = append(out, ct)
					break
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return contractName(out[i]) < contractName(out[j]) })
	return out
}

// contractName is how eye run names a contract: notify.Notify.
func contractName(ct *ir.Contract) string {
	if ct.Module.Path == "" {
		return ct.Name
	}
	return ct.Module.Path + "." + ct.Name
}

// servePort is the port a served contract listens on: its @port, or the one
// in the environment variable named after its module.
func servePort(ct *ir.Contract) (int, error) {
	name := envName(ct.Module.Path) + "_PORT"
	if ct.Module.Path == "" {
		return ct.Port, nil
	}
	s, ok := os.LookupEnv(name)
	if !ok || s == "" {
		return ct.Port, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 65535 {
		return 0, fmt.Errorf("%s is %q; it takes a port from 0 to 65535, where 0 is any free one", name, s)
	}
	return n, nil
}

// listen starts serving the contracts on their ports and says where on w.
// From here on, the code that runs Eyelang holds the interpreter's lock, and
// gives it up while it waits, so that requests can be answered. The stop it
// returns closes the ports and lets go of the lock; requests still waiting
// for it are answered 503.
func (in *Interp) listen(served []*ir.Contract, w io.Writer) (stop func(), err error) {
	rt := in.rt
	ls, err := in.listeners(served)
	if err != nil {
		return nil, err
	}
	var ports []net.Listener
	closeAll := func() {
		for _, ln := range ports {
			_ = ln.Close()
		}
	}
	// Every port is taken before any request is let in, so a program that
	// can't serve one of its contracts stops before it has answered anyone.
	for _, l := range ls {
		port, err := servePort(l.ct)
		if err != nil {
			closeAll()
			return nil, err
		}
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			closeAll()
			return nil, fmt.Errorf("can't serve %s on port %d: %v", contractName(l.ct), port, err)
		}
		l.addr = fmt.Sprintf(":%d", ln.Addr().(*net.TCPAddr).Port)
		l.srv = &http.Server{Handler: in.handler(l), ReadHeaderTimeout: 10 * time.Second}
		ports = append(ports, ln)
	}
	if in.g == nil {
		in.g = &gil{}
		in.g.mu.Lock() // the code running now holds it from here on
	}
	rt.kick = make(chan struct{}, 1)
	for i, l := range ls {
		go func() { _ = l.srv.Serve(ports[i]) }()
		fmt.Fprintf(w, "eye run: serving %s on %s\n", contractName(l.ct), l.addr)
	}
	return func() {
		rt.stopped = true
		for _, l := range ls {
			_ = l.srv.Close()
		}
		in.g.mu.Unlock()
	}, nil
}

// listeners makes a listener for each served contract, with the credential
// its @auth asks callers for and a route for each operation it implements,
// but no port: listen takes those, and a Service answers in memory.
func (in *Interp) listeners(served []*ir.Contract) ([]*listener, error) {
	var ls []*listener
	for _, ct := range served {
		l := &listener{ct: ct}
		if ct.HTTP.AuthSecret != "" {
			s, ok := in.lookupSecret(ct.HTTP.AuthSecret)
			if !ok {
				return nil, fmt.Errorf("%s needs the secret %q to check its callers' @auth: set %s, or put a file %s in $EYE_SECRETS",
					contractName(ct), ct.HTTP.AuthSecret, envName(ct.HTTP.AuthSecret), ct.HTTP.AuthSecret)
			}
			if ct.HTTP.AuthBearer {
				s = "Bearer " + s
			}
			l.auth = s
		}
		names := make([]string, 0, len(ct.Ops))
		for name := range ct.Ops {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			op := ct.Ops[name]
			if op.HTTP == nil || op.Impl == nil {
				continue // reached only from inside the program
			}
			l.routes = append(l.routes, &route{op: op, method: op.HTTP.Method, segments: strings.Split(op.HTTP.Path, "/")})
		}
		ls = append(ls, l)
	}
	return ls, nil
}

// match finds the route for a request. allowed lists the methods its path
// takes when only the method is wrong.
func (l *listener) match(method, path string) (r *route, args map[string]string, allowed []string) {
	segments := strings.Split(path, "/")
	for _, rt := range l.routes {
		if len(rt.segments) != len(segments) {
			continue
		}
		vals := map[string]string{}
		ok := true
		for i, seg := range rt.segments {
			if name, isArg := strings.CutPrefix(seg, ":"); isArg {
				v, err := url.PathUnescape(segments[i])
				if err != nil || v == "" {
					ok = false
					break
				}
				vals[name] = v
			} else if seg != segments[i] {
				ok = false
				break
			}
		}
		switch {
		case !ok:
		case rt.method == method:
			return rt, vals, nil
		default:
			allowed = append(allowed, rt.method)
		}
	}
	return nil, nil, allowed
}

// handler answers the requests to one served contract.
func (in *Interp) handler(l *listener) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The credential is checked first, so a caller without it learns
		// nothing, not even which paths exist.
		if l.auth != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get(l.ct.HTTP.AuthHeader)), []byte(l.auth)) != 1 {
			how := "Bearer and the secret"
			if !l.ct.HTTP.AuthBearer {
				how = "the secret"
			}
			writeError(w, http.StatusUnauthorized, fmt.Sprintf("%s takes only requests with %s in the %s header", contractName(l.ct), how, l.ct.HTTP.AuthHeader))
			return
		}
		rt, pathArgs, allowed := l.match(r.Method, r.URL.EscapedPath())
		if rt == nil {
			if len(allowed) > 0 {
				w.Header().Set("Allow", strings.Join(allowed, ", "))
				writeError(w, http.StatusMethodNotAllowed, fmt.Sprintf("%s takes %s, not %s", r.URL.Path, strings.Join(allowed, " or "), r.Method))
				return
			}
			writeError(w, http.StatusNotFound, fmt.Sprintf("%s has no operation at %s %s", contractName(l.ct), r.Method, r.URL.Path))
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
		if err != nil {
			writeError(w, http.StatusBadRequest, "the request's body couldn't be read: "+err.Error())
			return
		}
		req := request{path: pathArgs, query: r.URL.Query(), header: r.Header, body: body}
		stopped := false
		var status int
		var reply []byte
		var contentType string
		func() {
			in.g.mu.Lock()
			defer in.g.mu.Unlock()
			if stopped = in.rt.stopped; !stopped {
				status, reply, contentType = in.answer(rt.op, req)
			}
		}()
		if stopped {
			writeError(w, http.StatusServiceUnavailable, "the program is stopping")
			return
		}
		// The reply waits until what the request committed is on disk.
		// The lock is free meanwhile, so requests answered together share
		// one sync.
		if j := in.rt.journal; j != nil {
			if err := j.Sync(); err != nil {
				fmt.Fprintf(in.log, "eye run: %s.%s couldn't be saved, answered 503: %v\n", contractName(l.ct), rt.op.Name, err)
				writeError(w, http.StatusServiceUnavailable, "the request couldn't be saved: "+err.Error())
				return
			}
		}
		select {
		case in.rt.kick <- struct{}{}: // the runner runs the jobs it started
		default:
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write(reply)
	})
}

// request is what a request gives an operation.
type request struct {
	path   map[string]string // the path's :name segments, unescaped
	query  url.Values
	header http.Header
	body   []byte
}

// writeError answers a request that reaches no operation: { "error": "…" }.
func writeError(w http.ResponseWriter, status int, msg string) {
	data, _ := json.Marshal(map[string]string{"error": msg})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(data, '\n'))
}

func errorBody(msg string) []byte {
	data, _ := json.Marshal(map[string]string{"error": msg})
	return append(data, '\n')
}

// answer runs an operation for a request, as one transaction, and returns
// the reply. The caller holds the interpreter's lock.
func (in *Interp) answer(op *ir.Op, req request) (status int, reply []byte, contentType string) {
	rt := in.rt
	name := contractName(op.Contract) + "." + op.Name
	// The impl runs on an interpreter of its own, which keeps the lock even
	// if something it calls would wait: the runs it interrupts don't see it.
	c := in.child(nil)
	c.g, c.budgets, c.stepSpend, c.scope = nil, nil, nil, nil
	prevDraining := rt.draining
	rt.draining = true // the jobs it starts run on the runner, not here
	defer func() {
		rt.draining = prevDraining
		if rt.tx != nil {
			rt.tx = nil // a fault rolls the transaction back
		}
		if p := recover(); p != nil {
			switch x := p.(type) {
			case *Fault:
				fmt.Fprintf(in.log, "eye run: %s faulted, answered 500: %s\n", name, x.Error())
				status, reply, contentType = http.StatusInternalServerError, errorBody(name+" faulted: "+x.Msg), "application/json"
			case *CommitError:
				fmt.Fprintf(in.log, "eye run: %s couldn't be saved, answered 503: %v\n", name, x)
				status, reply, contentType = http.StatusServiceUnavailable, errorBody(name+" couldn't be saved: "+x.Error()), "application/json"
			default:
				panic(p)
			}
		}
	}()
	args, err := c.requestArgs(op, req)
	if err != nil {
		return http.StatusBadRequest, errorBody(err.Error()), "application/json"
	}
	if rt.real {
		if now := time.Now(); now.After(rt.now) {
			rt.now = now
		}
	}
	tx := rt.begin()
	v, _ := c.callFunc(op.Impl, nil, nil, args, op.Pos)
	rt.tx = nil
	rt.commit(tx)
	return c.replyFor(op, v)
}

// requestArgs reads an operation's arguments from a request: path segments,
// query parameters for GET and DELETE, else a JSON object. An argument left
// out takes its default, and an Option None.
func (in *Interp) requestArgs(op *ir.Op, req request) ([]argVal, error) {
	snake := op.Contract.HTTP.Snake
	inQuery := wire.InQuery(op.HTTP.Method)
	var obj map[string]any
	if !inQuery {
		x, err := decodeJSON(req.body)
		if err != nil {
			return nil, fmt.Errorf("the body isn't JSON: %v", err)
		}
		if x != nil {
			var ok bool
			if obj, ok = x.(map[string]any); !ok {
				return nil, fmt.Errorf("the body must be a JSON object of %s's arguments, found %s", op.Name, describeJSON(x))
			}
		}
	}
	var args []argVal
	for _, p := range op.Params {
		onWire := wireName(p.Name, snake)
		var v Value
		var err error
		raw, inPath := req.path[p.Name]
		values, hasQuery := req.query[onWire]
		field, inBody := obj[onWire]
		switch {
		case inPath:
			v, err = in.fromText(raw, p.Type, snake)
		case inQuery && hasQuery:
			v, err = in.fromQuery(values, p.Type, snake)
		case inBody:
			v, err = in.fromWire(field, p.Type, snake)
		case p.Default != nil:
			v = in.eval(&frame{fn: &ir.Func{Name: op.Name}, slots: make([]Value, 16)}, p.Default)
		case p.Type.Decl != nil && in.isOption(p.Type.Decl):
			v = in.none()
		default:
			err = errors.New("it is missing")
		}
		if err != nil {
			return nil, fmt.Errorf("%s's argument %s: %v", op.Name, onWire, err)
		}
		args = append(args, argVal{v: v})
	}
	if op.Key {
		header := wire.KeyHeader(op)
		key := req.header.Get(header)
		if s, ok := obj[op.KeyField].(string); key == "" && op.KeyField != "" && ok {
			key = s
		}
		if key == "" {
			where := "the " + header + " header"
			if op.KeyField != "" {
				where += " or the body's field " + op.KeyField
			}
			return nil, fmt.Errorf("%s takes a key, in %s", op.Name, where)
		}
		args = append(args, argVal{name: "key", v: key})
	}
	return args, nil
}

// fromText reads an argument written as text, in a path segment or a query
// parameter: a string as it is, anything else as JSON (5, true, null).
func (in *Interp) fromText(s string, t *ir.TypeExpr, snake bool) (Value, error) {
	v, err := in.fromWire(s, t, snake)
	if err == nil {
		return v, nil
	}
	if x, jerr := decodeJSON([]byte(s)); jerr == nil {
		if v, jerr := in.fromWire(x, t, snake); jerr == nil {
			return v, nil
		}
	}
	return nil, err
}

// fromQuery reads an argument from a query parameter: a list or a set from
// the parameter repeated, anything else from its first value.
func (in *Interp) fromQuery(values []string, t *ir.TypeExpr, snake bool) (Value, error) {
	if t.Decl != nil && in.isOption(t.Decl) && len(t.Args) == 1 {
		// Option(T) reads T: the type argument, not the declaration's
		// variable.
		v, err := in.fromQuery(values, t.Args[0], snake)
		if err != nil {
			return nil, err
		}
		return in.some(v), nil
	}
	if t.Decl == nil && (t.Name == "List" || t.Name == "Set") && len(t.Args) == 1 {
		elems := make([]Value, len(values))
		for i, s := range values {
			v, err := in.fromText(s, t.Args[0], snake)
			if err != nil {
				return nil, fmt.Errorf("[%d]: %v", i, err)
			}
			elems[i] = v
		}
		if t.Name == "Set" {
			return newSet(elems), nil
		}
		return &List{Elems: elems}, nil
	}
	return in.fromText(values[0], t, snake)
}

// replyFor is an operation's reply: its result in the one JSON shape, with
// the status `status:` gives the result's variant (the lowest, when several
// give it one), else 200. A result of Result(T, http.HttpError) sends T, or
// the error's status and body.
func (in *Interp) replyFor(op *ir.Op, v Value) (int, []byte, string) {
	if httpErrorType(op.Return) != nil {
		if e, ok := isVariant(v, "Err"); ok {
			if bad, ok := isVariant(e.Vals[0], "BadStatus"); ok && len(bad.Vals) == 2 {
				status, _ := bad.Vals[0].(int64)
				body, _ := bad.Vals[1].(string)
				if status >= 100 && status <= 999 {
					return int(status), []byte(body), "text/plain; charset=utf-8"
				}
			}
			return http.StatusBadGateway, errorBody(op.Name + ": " + show(e.Vals[0])), "application/json"
		}
		if o, ok := isVariant(v, "Ok"); ok {
			v = o.Vals[0]
		}
	}
	status := http.StatusOK
	if x, ok := v.(*Variant); ok {
		status = wire.ReplyStatus(op, x.V.Name)
	}
	data, err := json.Marshal(in.toWire(v, op.Contract.HTTP.Snake))
	if err != nil {
		panic(&Fault{Pos: op.Pos, Msg: fmt.Sprintf("%s's result can't be written as JSON: %v", op.Name, err)})
	}
	return status, append(data, '\n'), "application/json"
}
