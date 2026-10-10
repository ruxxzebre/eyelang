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
	} `json:"exchanges"`
}

// fakesHandler is the value of `fakes(…)`.
type fakesHandler struct {
	modules []*ir.Module
	dir     string
}

func (h *fakesHandler) apply(in *Interp, at ir.Pos) {
	base := in.prog.Root.Dir
	if at.File != nil {
		base = filepath.Dir(at.File.Path)
	}
	for _, m := range h.modules {
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
		v, err := in.fromJSON(e.Result, op.Return)
		if err != nil {
			return nil, fmt.Errorf("%s: exchange %d (%s): %v", path, i+1, e.Op, err)
		}
		fs.exchanges = append(fs.exchanges, exchange{op: e.Op, key: e.Key, args: e.Args, result: v})
	}
	return fs, nil
}

// callOp calls an operation of another service, which in tests is a fake.
func (rt *runtime) callOp(op *ir.Op, args []Value, named map[string]Value, at ir.Pos) Value {
	fs := rt.fakes[op.Contract]
	mod := op.Contract.Module.Name
	if fs == nil {
		if rt.real {
			rt.in.faultf(at, "calling %s.%s needs the contract runtime of phase 6; for now, run it in a test with fakes", mod, op.Name)
		}
		rt.in.faultf(at, "%s.%s calls another service; replay recordings with `with fakes(%s, from: \"…\")`", mod, op.Name, mod)
	}
	key, _ := named["key"].(string)
	for _, e := range fs.exchanges {
		if e.op == op.Name && (key == "" || e.key == key) {
			return e.result
		}
	}
	if key != "" {
		rt.in.faultf(at, "%s has no exchange for %s.%s with key %q", fs.path, mod, op.Name, key)
	}
	rt.in.faultf(at, "%s has no exchange for %s.%s", fs.path, mod, op.Name)
	return nil
}

// reconcile resolves an Uncertain outcome; any other outcome is already
// known.
func (rt *runtime) reconcile(v *Variant, at ir.Pos) Value {
	if v.V.Name == "Uncertain" {
		rt.in.faultf(at, "reconciling an Uncertain outcome comes with contracts in phase 6")
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
				v, err := in.fromAny(obj[k], arg(1))
				if err != nil {
					return nil, err
				}
				keys, vals = append(keys, k), append(vals, v)
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
		return in.fromAny(x, d.Alias)
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
