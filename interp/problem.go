package interp

import "eyelang/ir"

// problem makes err a Problem (SPECS.md §10), with the line what, if any,
// before its context. A Problem keeps its context and gains the line; any
// other error becomes one, shown as `"{err}"` shows it.
func (in *Interp) problem(err Value, what []string) Value {
	if v, ok := err.(*Variant); ok && v.V == in.pre.problem {
		if len(what) == 0 {
			return v
		}
		context := append(append([]Value{}, toValues(what)...), v.Vals[0].(*List).Elems...)
		return &Variant{V: v.V, Vals: []Value{&List{Elems: context}, v.Vals[1]}}
	}
	return &Variant{V: in.pre.problem, Vals: []Value{&List{Elems: toValues(what)}, in.display(err)}}
}

func toValues(ss []string) []Value {
	out := make([]Value, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

// problemBuiltins adds context, which turns a Result's error into a Problem.
func problemBuiltins(add func(string, builtinFunc)) {
	add("context", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "context", at)
		r, ok := args[0].(*Variant)
		what, ok2 := args[1].(string)
		if !ok || !ok2 || (r.V != in.pre.ok && r.V != in.pre.err) {
			in.faultf(at, `context works on a Result, and takes a String: r.context("reading {path}")`)
		}
		if r.V == in.pre.ok {
			return r
		}
		return in.errV(in.problem(r.Vals[0], []string{what}))
	})
}
