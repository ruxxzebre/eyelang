package interp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"eyelang/ir"
)

// Fakes replay recorded exchanges with other services. A recording is a JSON
// file per contract module, named after the module, in the directory that
// `with fakes(module, from: "dir/")` names:
//
//	{
//	  "contract": "Zeropod",
//	  "exchanges": [
//	    { "op": "checkpoint", "key": "sleep-survival-3", "result": { "Done": ["ckpt-1"] } }
//	  ]
//	}
//
// Each result is checked against the operation's declared result type when
// the fakes are applied: a variant is an object with the variant's name as
// its only key and its fields as a list or, when named, an object; a type
// that wraps one value, such as `| CheckpointId(String)`, may be written as
// that value.
type fakeSet struct {
	path      string
	exchanges []exchange
	// answered are the calls the fakes answered, in order, for
	// fakes.calls(op).
	answered []fakeCall
}

// fakeCall is a call a fake answered: the operation, and its arguments as
// a record, with defaults filled in and the key last.
type fakeCall struct {
	op   *ir.Op
	args *Record
}

type exchange struct {
	op     string
	key    string
	args   json.RawMessage
	result Value
}

type recording struct {
	Contract  string `json:"contract"`
	Exchanges []struct {
		Op     string          `json:"op"`
		Key    string          `json:"key"`
		Args   json.RawMessage `json:"args"`
		Result json.RawMessage `json:"result"`
		Usage  *recordedUsage  `json:"usage"`
		Calls  []recordedCall  `json:"calls"`
	} `json:"exchanges"`
}

// recordedReply is a model's reply in a recording, and what it spent, if
// the recording says: `"usage": {"input": 1200, "output": 300}`, with
// `"usd"` when the price was known. A reply that calls tools has Calls in
// place of a Result.
type recordedReply struct {
	Result json.RawMessage
	Usage  *recordedUsage
	Calls  []recordedCall
}

// recordedCall is a model's call of a tool in a recording:
// `{"tool": "searchIssues", "input": {"phrase": "crash"}}`.
type recordedCall struct {
	Tool  string          `json:"tool"`
	Input json.RawMessage `json:"input"`
}

type recordedUsage struct {
	Input  int64    `json:"input"`
	Output int64    `json:"output"`
	USD    *float64 `json:"usd"`
}

// spent is what the recorded reply counts against a budget.
func (r recordedReply) spent(model string) spend {
	if r.Usage == nil {
		return spend{}
	}
	return usage(model, r.Usage.Input, r.Usage.Output, r.Usage.USD)
}

// fakesHandler is the value of `fakes(…)`. Applied, it keeps the fakes it
// made, so a test can ask what they were called with.
type fakesHandler struct {
	modules []*ir.Module
	dir     string
	sets    map[*ir.Contract]*fakeSet
}

// calls is fakes.calls(op): the calls the fakes answered for op, oldest
// first, each a record of its arguments.
func (h *fakesHandler) calls(in *Interp, args []Value, at ir.Pos) Value {
	var o *ir.Op
	if len(args) == 1 {
		if v, ok := args[0].(*OpVal); ok {
			o = v.Op
		}
	}
	if o == nil {
		in.faultf(at, "calls takes an operation of a contract the fakes answer for, as calls(notify.send)")
	}
	faked := false
	for _, m := range h.modules {
		faked = faked || m == o.Contract.Module
	}
	if !faked {
		in.faultf(at, "these fakes don't answer for %s, so they saw no call of %s", o.Contract.Module.Name, o.Name)
	}
	list := &List{}
	if fs := h.sets[o.Contract]; fs != nil {
		for _, c := range fs.answered {
			if c.op == o {
				list.Elems = append(list.Elems, c.args)
			}
		}
	}
	return list
}

func (h *fakesHandler) apply(in *Interp, at ir.Pos) {
	base := in.prog.Root.Dir
	if at.File != nil {
		base = filepath.Dir(at.File.Path)
	}
	for _, m := range h.modules {
		if m.Path == "ai" && m.Dir == "" {
			in.rt.aiReplies = in.loadReplies(filepath.Join(base, h.dir, "ai.json"), at)
			continue
		}
		if m.Path == "http" && m.Dir == "" {
			in.rt.httpFakes = in.loadHTTPFakes(filepath.Join(base, h.dir, "http.json"), at)
			continue
		}
		if len(m.Contracts) != 1 {
			in.faultf(at, "fakes take modules that declare one contract, and %s declares %d", m.Name, len(m.Contracts))
		}
		var ct *ir.Contract
		for _, c := range m.Contracts {
			ct = c
		}
		path := filepath.Join(base, h.dir, m.Name+".json")
		fs, err := in.loadFakes(path, ct)
		if err != nil {
			in.faultf(at, "%v", err)
		}
		in.rt.fakes[ct] = fs
		if h.sets == nil {
			h.sets = map[*ir.Contract]*fakeSet{}
		}
		h.sets[ct] = fs
	}
}

func (in *Interp) loadFakes(path string, ct *ir.Contract) (*fakeSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no recordings for %s: %v", ct.Name, err)
	}
	var rec recording
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&rec); err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	if rec.Contract != ct.Name {
		return nil, fmt.Errorf("%s records contract %q, not %s", path, rec.Contract, ct.Name)
	}
	fs := &fakeSet{path: path}
	for i, e := range rec.Exchanges {
		op, ok := ct.Ops[e.Op]
		if !ok {
			return nil, fmt.Errorf("%s: exchange %d: %s has no operation %q", path, i+1, ct.Name, e.Op)
		}
		v, err := in.recorded(e.Result, op.Return)
		if err != nil && op.Fresh {
			// A @fresh command may answer {"kind": "Stale"} or {"kind": "Expired"}.
			if x, xerr := decodeJSON(e.Result); xerr == nil {
				if f := in.freshOutcome(op, x, false); f != nil {
					v, err = f, nil
				}
			}
		}
		if err != nil && op.Command {
			// A command's reply may have been lost: {"kind": "Uncertain", "key": "…"}.
			if u, uerr := in.recorded(e.Result, &ir.TypeExpr{Name: "Unconfirmed", Decl: in.prog.Prelude.Types["Unconfirmed"]}); uerr == nil {
				v, err = u, nil
			}
		}
		if err != nil {
			return nil, fmt.Errorf("%s: exchange %d (%s): %v", path, i+1, e.Op, err)
		}
		fs.exchanges = append(fs.exchanges, exchange{op: e.Op, key: e.Key, args: e.Args, result: v})
	}
	return fs, nil
}

// recorded reads a recorded value in the JSON shape every value travels in
// (json.go): a variant is {"kind": "Sent", …}. Recordings made before that
// shape was settled write a variant as {"Sent": fields}, which is still read.
func (in *Interp) recorded(raw json.RawMessage, t *ir.TypeExpr) (Value, error) {
	x, err := decodeJSON(raw)
	if err != nil {
		return nil, err
	}
	v, err := in.fromWire(x, t, false)
	if err == nil {
		return v, nil
	}
	if old, oldErr := in.fromJSON(raw, t); oldErr == nil {
		return old, nil
	}
	return nil, err
}

// loadReplies reads recorded model replies for `with fakes(ai, …)`: an
// "exchanges" list whose results are the replies, each the value asked for,
// used in order.
func (in *Interp) loadReplies(path string, at ir.Pos) []recordedReply {
	data, err := os.ReadFile(path)
	if err != nil {
		in.faultf(at, "no recordings for ai: %v", err)
	}
	var rec recording
	if err := json.Unmarshal(data, &rec); err != nil {
		in.faultf(at, "%s: %v", path, err)
	}
	replies := []recordedReply{}
	for _, e := range rec.Exchanges {
		replies = append(replies, recordedReply{Result: e.Result, Usage: e.Usage, Calls: e.Calls})
	}
	return replies
}

// callOp calls an operation of another service: a fake in tests, or the
// service itself over HTTP when its contract says how and a URL is
// configured for it.
func (rt *runtime) callOp(op *ir.Op, args []Value, named map[string]Value, at ir.Pos) Value {
	fs := rt.fakes[op.Contract]
	mod := op.Contract.Module.Name
	if fs == nil {
		if v, ok := rt.callHTTP(op, args, named, at); ok {
			return v
		}
		if op.HTTP == nil {
			rt.in.faultf(at, "%s.%s calls another service; replay recordings with `with fakes(%s, from: \"…\")`, or say how it goes over HTTP with @http", mod, op.Name, mod)
		}
		rt.in.faultf(at, "%s.%s calls another service: set %s_URL to its address, or replay recordings with `with fakes(%s, from: \"…\")`", mod, op.Name, envName(op.Contract.Module.Path), mod)
	}
	key, _ := named["key"].(string)
	for _, e := range fs.exchanges {
		if e.op == op.Name && (key == "" || e.key == key) {
			fs.answered = append(fs.answered, fakeCall{op, rt.callArgs(op, args, named)})
			if v, ok := e.result.(*Variant); ok && v.V.Name == "Uncertain" && len(v.Vals) == 1 {
				rt.lost(keyOf(v.Vals[0]), sentCommand{op, args, named})
			}
			return e.result
		}
	}
	if key != "" {
		rt.in.faultf(at, "%s has no exchange for %s.%s with key %q", fs.path, mod, op.Name, key)
	}
	rt.in.faultf(at, "%s has no exchange for %s.%s", fs.path, mod, op.Name)
	return nil
}

// callArgs is a call's arguments as a record: one field per parameter, in
// order, with defaults filled in, and the key last for a command with @key.
func (rt *runtime) callArgs(op *ir.Op, args []Value, named map[string]Value) *Record {
	rec := &Record{}
	for i, p := range op.Params {
		var v Value
		switch {
		case i < len(args):
			v = args[i]
		case named[p.Name] != nil:
			v = named[p.Name]
		case p.Default != nil:
			v = rt.in.eval(&frame{fn: &ir.Func{Name: op.Name}, slots: make([]Value, 16)}, p.Default)
		}
		rec.Names, rec.Vals = append(rec.Names, p.Name), append(rec.Vals, v)
	}
	if op.Key {
		rec.Names, rec.Vals = append(rec.Names, "key"), append(rec.Vals, named["key"])
	}
	return rec
}

// sentCommand is a command whose reply was lost, kept so that reconcile can
// ask about it and, if it never arrived, send it again with the same key.
type sentCommand struct {
	op    *ir.Op
	args  []Value
	named map[string]Value
}

// reconcile resolves an Uncertain outcome; any other outcome is already
// known. It asks the contract's @reconcile query by key: Some is the
// command's outcome, and None means the command never arrived, so it is sent
// again with the same key, which can't do it twice.
func (rt *runtime) reconcile(v *Variant, at ir.Pos) Value {
	var keys []string // settled once the outcome is known
	defer func() {
		if v.V.Name != "Uncertain" {
			for _, k := range keys {
				rt.settled(k)
			}
		}
	}()
	for tries := 0; v.V.Name == "Uncertain"; tries++ {
		key := v.Vals[0]
		keys = append(keys, keyOf(key))
		sent, ok := rt.uncertain[keyOf(key)]
		if !ok || sent.op.Reconcile == "" {
			rt.in.faultf(at, "this outcome can't be reconciled: its command names no @reconcile query, so match Uncertain(key) instead")
		}
		query := sent.op.Contract.Ops[sent.op.Reconcile]
		if query == nil {
			rt.in.faultf(at, "%s's @reconcile names %s, which isn't a query of %s", sent.op.Name, sent.op.Reconcile, sent.op.Contract.Name)
		}
		if tries == 3 {
			rt.in.faultf(at, "%s with key %s is still uncertain after asking %s three times", sent.op.Name, show(key), query.Name)
		}
		if query == sent.op {
			// @reconcile(send): the service keeps what a key did, so sending
			// again with the same key answers without doing it twice.
			next, ok := rt.callOp(sent.op, sent.args, sent.named, at).(*Variant)
			if !ok {
				rt.in.faultf(at, "%s returned something other than its outcome", sent.op.Name)
			}
			v = next
			continue
		}
		found, ok := rt.callOp(query, nil, map[string]Value{"key": key}, at).(*Variant)
		switch {
		case ok && found.V == rt.in.pre.some:
			v = found.Vals[0].(*Variant)
			return v
		case ok && found.V == rt.in.pre.none:
			next, ok := rt.callOp(sent.op, sent.args, sent.named, at).(*Variant)
			if !ok {
				rt.in.faultf(at, "%s returned something other than its outcome", sent.op.Name)
			}
			v = next
		default:
			rt.in.faultf(at, "%s, a @reconcile query, returns an Option of the command's outcome", query.Name)
		}
	}
	return v
}

// fromJSON decodes a recorded value as type t.
func (in *Interp) fromJSON(raw json.RawMessage, t *ir.TypeExpr) (Value, error) {
	var x any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if len(raw) == 0 {
		x = nil
	} else if err := dec.Decode(&x); err != nil {
		return nil, err
	}
	return in.fromAny(x, t)
}

func (in *Interp) fromAny(x any, t *ir.TypeExpr) (Value, error) {
	if t == nil {
		return nil, fmt.Errorf("no type to read the value as")
	}
	if t.Decl != nil {
		return in.fromDecl(x, t.Decl, t.Args)
	}
	arg := func(i int) *ir.TypeExpr {
		if i < len(t.Args) {
			return t.Args[i]
		}
		return nil
	}
	switch t.Name {
	case "String", "Key":
		if s, ok := x.(string); ok {
			return s, nil
		}
	case "Int":
		if n, ok := x.(json.Number); ok {
			if i, err := n.Int64(); err == nil {
				return i, nil
			}
		}
	case "Float":
		if n, ok := x.(json.Number); ok {
			if f, err := n.Float64(); err == nil {
				return f, nil
			}
		}
	case "Bool":
		if b, ok := x.(bool); ok {
			return b, nil
		}
	case "Size":
		if n, ok := x.(json.Number); ok {
			if i, err := n.Int64(); err == nil {
				return ir.Size(i), nil
			}
		}
	case "Duration":
		if n, ok := x.(json.Number); ok {
			if i, err := n.Int64(); err == nil {
				return ir.Duration(i * int64(time.Millisecond)), nil
			}
		}
	case "Instant":
		if s, ok := x.(string); ok {
			if when, err := time.Parse(time.RFC3339, s); err == nil {
				return when.UTC(), nil
			}
		}
	case "Date":
		if s, ok := x.(string); ok && len(s) == 10 {
			y, _ := strconv.Atoi(s[:4])
			m, _ := strconv.Atoi(s[5:7])
			d, _ := strconv.Atoi(s[8:])
			return ir.Date{Year: y, Month: m, Day: d}, nil
		}
	case "()":
		if x == nil {
			return ir.Unit{}, nil
		}
	case "List", "Set":
		if xs, ok := x.([]any); ok {
			var vals []Value
			for _, e := range xs {
				v, err := in.fromAny(e, arg(0))
				if err != nil {
					return nil, err
				}
				vals = append(vals, v)
			}
			if t.Name == "Set" {
				return newSet(vals), nil
			}
			return &List{Elems: vals}, nil
		}
	case "Map":
		if obj, ok := x.(map[string]any); ok {
			var keys, vals []Value
			for _, k := range sortedKeys(obj) {
				key, err := in.fromAny(k, arg(0))
				if err != nil {
					return nil, fmt.Errorf("key %q: %v", k, err)
				}
				v, err := in.fromAny(obj[k], arg(1))
				if err != nil {
					return nil, err
				}
				keys, vals = append(keys, key), append(vals, v)
			}
			return newMap(keys, vals), nil
		}
	case "record":
		if obj, ok := x.(map[string]any); ok {
			rec := &Record{}
			for i, n := range t.Fields {
				v, err := in.fromAny(obj[n], t.Args[i])
				if err != nil {
					return nil, fmt.Errorf("%s: %v", n, err)
				}
				rec.Names, rec.Vals = append(rec.Names, n), append(rec.Vals, v)
			}
			return rec, nil
		}
	}
	return nil, fmt.Errorf("expected %s, found %s", typeString(t), describeJSON(x))
}

func (in *Interp) fromDecl(x any, d *ir.Type, args []*ir.TypeExpr) (Value, error) {
	if d.Alias != nil && len(d.Variants) == 0 {
		v, err := in.fromAny(x, d.Alias)
		if r, ok := v.(*Record); ok && d.Alias.Name == "record" {
			r.Type = d
		}
		return v, err
	}
	// Type variables in the fields take the type's arguments.
	sub := func(t *ir.TypeExpr) *ir.TypeExpr {
		for i, p := range d.Params {
			if t != nil && t.Name == p && t.Decl == nil && i < len(args) {
				return args[i]
			}
		}
		return t
	}
	if d.Newtype() {
		if _, isObj := x.(map[string]any); !isObj {
			v, err := in.fromAny(x, sub(d.Variants[0].Fields[0].Type))
			if err != nil {
				return nil, err
			}
			return &Variant{V: d.Variants[0], Vals: []Value{v}}, nil
		}
	}
	if s, ok := x.(string); ok {
		for _, v := range d.Variants {
			if v.Name == s && len(v.Fields) == 0 {
				return &Variant{V: v}, nil
			}
		}
	}
	obj, ok := x.(map[string]any)
	if !ok || len(obj) != 1 {
		return nil, fmt.Errorf("expected one of %s's variants, as {\"Name\": fields}, found %s", d.Name, describeJSON(x))
	}
	for name, payload := range obj {
		var variant *ir.Variant
		for _, v := range d.Variants {
			if v.Name == name {
				variant = v
			}
		}
		if variant == nil {
			return nil, fmt.Errorf("%s has no variant %s", d.Name, name)
		}
		vals := make([]Value, len(variant.Fields))
		switch p := payload.(type) {
		case nil:
			if len(variant.Fields) != 0 {
				return nil, fmt.Errorf("%s holds %d values, and the recording has none", name, len(variant.Fields))
			}
		case []any:
			if len(p) != len(variant.Fields) {
				return nil, fmt.Errorf("%s holds %d values, and the recording has %d", name, len(variant.Fields), len(p))
			}
			for i, e := range p {
				v, err := in.fromAny(e, sub(variant.Fields[i].Type))
				if err != nil {
					return nil, fmt.Errorf("%s: %v", name, err)
				}
				vals[i] = v
			}
		case map[string]any:
			for i, f := range variant.Fields {
				e, ok := p[f.Name]
				if !ok || f.Name == "" {
					return nil, fmt.Errorf("%s: the recording has no field %q", name, f.Name)
				}
				v, err := in.fromAny(e, sub(f.Type))
				if err != nil {
					return nil, fmt.Errorf("%s.%s: %v", name, f.Name, err)
				}
				vals[i] = v
			}
		default:
			return nil, fmt.Errorf("%s's fields are a list or an object, found %s", name, describeJSON(payload))
		}
		return &Variant{V: variant, Vals: vals}, nil
	}
	return nil, nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func typeString(t *ir.TypeExpr) string {
	if len(t.Args) == 0 {
		return t.Name
	}
	var parts []string
	for _, a := range t.Args {
		parts = append(parts, typeString(a))
	}
	return t.Name + "(" + strings.Join(parts, ", ") + ")"
}

func describeJSON(x any) string {
	switch v := x.(type) {
	case nil:
		return "null"
	case string:
		return strconv.Quote(v)
	case json.Number:
		return v.String()
	case bool:
		return strconv.FormatBool(v)
	case []any:
		return "a list"
	case map[string]any:
		return "an object"
	}
	return fmt.Sprint(x)
}
