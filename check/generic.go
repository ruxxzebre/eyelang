package check

import (
	"fmt"
	"sort"
	"strings"

	"eyelang/ir"
)

// A private function that leaves its types out is generic (SPECS.md §6):
// its body is checked first, on its own, and what it leaves undecided is a
// type variable that each call decides afresh. `fn twice(n) = n + n` adds
// Ints at one call and Floats at the next.
//
// The body is checked when the first call needs its type, or in its turn.
// The type variables of its parameters and result that nothing in the body
// decided are generalized; the trait requirements and operators still
// waiting on them go with them, and each call meets them for its own types.
// Functions that call each other are checked one inside the other, and
// share one type while they are: a variable that still belongs to a
// function being checked isn't generalized.

type inferState int

const (
	notInferred inferState = iota
	inferring
	inferredDone
)

// scheme is an inferred function's generic type.
type scheme struct {
	vars   map[*tvar]bool // the generalized type variables
	params []typ          // each untyped parameter's type, nil for a typed one
	ret    typ            // the inferred result, nil if it is written or ()
	reqs   []requirement  // trait requirements on the variables
	ops    []pendingOp    // operators on the variables
}

// inferrable reports whether a function's types are inferred: a function of
// a module, not a method, with a parameter whose type is left out or a
// result its expression gives, and no type variables of its own.
func (c *checker) inferrable(fn *ir.Func) bool {
	if fn.Receiver != nil || fn.HasSelf || fn.Extern || fn.Body == nil || len(fn.Bounds) > 0 {
		return false
	}
	if fn.Module == nil || fn.Module.Funcs[fn.Name] != fn {
		return false
	}
	inferred := false
	for _, p := range fn.Params {
		if p.Type == nil {
			inferred = true
		} else if mentionsVar(p.Type) {
			return false
		}
	}
	if fn.Return != nil {
		return inferred && !mentionsVar(fn.Return)
	}
	if _, block := fn.Body.(*ir.Block); !block {
		inferred = true
	}
	return inferred
}

// mentionsVar reports whether a written type names a type variable.
func mentionsVar(te *ir.TypeExpr) bool {
	if te == nil {
		return false
	}
	if te.Decl == nil && te.Entity == nil && te.Trait == nil && isVarName(te.Name) &&
		te.Name != "record" && te.Name != "tuple" && te.Name != "fn" && te.Name != "dyn" {
		return true
	}
	for _, a := range te.Args {
		if mentionsVar(a) {
			return true
		}
	}
	return mentionsVar(te.Result)
}

// inferFunc checks an inferred function's body and generalizes its type.
func (c *checker) inferFunc(fn *ir.Func) {
	c.ts.inferred[fn] = inferring
	c.ts.active = append(c.ts.active, fn)
	c.checkBody(body{fn: fn})
	c.ts.active = c.ts.active[:len(c.ts.active)-1]
	c.ts.inferred[fn] = inferredDone
	c.generalize(fn)
}

// generalize makes an inferred function's undecided types its own type
// variables.
func (c *checker) generalize(fn *ir.Func) {
	s := &scheme{vars: map[*tvar]bool{}}
	var own []typ
	for i, p := range fn.Params {
		var t typ
		if p.Type == nil {
			t = c.ts.params[fn][i]
			own = append(own, t)
		}
		s.params = append(s.params, t)
	}
	if r, ok := c.ts.results[fn]; ok {
		s.ret = r
		own = append(own, r)
	}
	// Variables of the functions still being checked stay as they are.
	busy := map[*tvar]bool{}
	for _, g := range c.ts.active {
		for _, t := range c.ts.params[g] {
			freeVars(t, busy)
		}
		if r, ok := c.ts.results[g]; ok {
			freeVars(r, busy)
		}
	}
	for _, t := range own {
		found := map[*tvar]bool{}
		freeVars(t, found)
		for v := range found {
			if !busy[v] {
				s.vars[v] = true
			}
		}
	}
	mentions := func(ts ...typ) bool {
		for _, t := range ts {
			found := map[*tvar]bool{}
			freeVars(t, found)
			for v := range found {
				if s.vars[v] {
					return true
				}
			}
		}
		return false
	}
	var rest []requirement
	for _, r := range c.ts.deferred {
		if mentions(append([]typ{r.t}, r.b.args...)...) {
			s.reqs = append(s.reqs, r)
		} else {
			rest = append(rest, r)
		}
	}
	c.ts.deferred = rest
	var ops []pendingOp
	for _, o := range c.ts.ops {
		if mentions(o.l, o.r, o.out) {
			s.ops = append(s.ops, o)
		} else {
			ops = append(ops, o)
		}
	}
	c.ts.ops = ops
	c.ts.schemes[fn] = s
	// In its own body they are its type variables, as a written `t` is.
	n := 0
	for _, v := range sortedVars(s.vars) {
		v.rigid, v.name = true, string(rune('a'+n%26))
		n++
	}
}

// sortedVars lists type variables in the order they were made, so names
// and messages don't depend on map order.
func sortedVars(vars map[*tvar]bool) []*tvar {
	var out []*tvar
	for v := range vars {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

// instantiate is an inferred function's type at one use: its generalized
// variables are fresh, and so are the requirements and operators on them.
func (c *checker) instantiate(fn *ir.Func, s *scheme) *tfun {
	fresh := map[*tvar]*tvar{}
	for v := range s.vars {
		n := c.fresh()
		n.lit, n.never = v.lit, v.never
		fresh[v] = n
	}
	sub := func(t typ) typ { return substVars(t, fresh) }
	t := &tfun{}
	for i, p := range fn.Params {
		pt := s.params[i]
		if pt == nil {
			pt = c.fromExpr(p.Type, typeEnv{}, false)
		} else {
			pt = sub(pt)
		}
		t.params = append(t.params, fparam{name: p.Name, t: pt, hasDefault: p.Default != nil, mut: p.Mut})
	}
	if s.ret != nil {
		t.ret = sub(s.ret)
	} else {
		t.ret = c.resultType(fn, typeEnv{}, false)
	}
	for _, r := range s.reqs {
		b := cbound{trait: r.b.trait}
		for _, a := range r.b.args {
			b.args = append(b.args, sub(a))
		}
		where := fn.Name
		if r.where != "" && r.where != fn.Name {
			where = fmt.Sprintf("%s, in %s,", strings.TrimSuffix(r.where, ","), fn.Name)
		}
		t.bounds = append(t.bounds, boundUse{t: sub(r.t), b: b, fn: where})
	}
	for _, o := range s.ops {
		o.l, o.r, o.out = sub(o.l), sub(o.r), sub(o.out)
		if o.in == "" {
			o.in = fn.Name
		} else {
			o.in += ", in " + fn.Name
		}
		t.ops = append(t.ops, o)
	}
	return t
}

// freeVars adds the type variables in t that nothing has decided.
func freeVars(t typ, into map[*tvar]bool) {
	switch x := prune(t).(type) {
	case *tvar:
		if !x.rigid {
			into[x] = true
		}
	case *tcon:
		for _, a := range x.args {
			freeVars(a, into)
		}
	case *ttuple:
		for _, e := range x.elems {
			freeVars(e, into)
		}
	case *tfun:
		for _, p := range x.params {
			freeVars(p.t, into)
		}
		freeVars(x.ret, into)
	case *tdyn:
		for _, a := range x.args {
			freeVars(a, into)
		}
	}
}

// substVars copies t with the variables in sub replaced.
func substVars(t typ, sub map[*tvar]*tvar) typ {
	switch x := prune(t).(type) {
	case *tvar:
		if n, ok := sub[x]; ok {
			return n
		}
		return x
	case *tcon:
		if len(x.args) == 0 {
			return x
		}
		cp := *x
		cp.args = make([]typ, len(x.args))
		for i, a := range x.args {
			cp.args[i] = substVars(a, sub)
		}
		return &cp
	case *ttuple:
		cp := &ttuple{elems: make([]typ, len(x.elems))}
		for i, e := range x.elems {
			cp.elems[i] = substVars(e, sub)
		}
		return cp
	case *tfun:
		cp := *x
		cp.params = make([]fparam, len(x.params))
		for i, p := range x.params {
			p.t = substVars(p.t, sub)
			cp.params[i] = p
		}
		cp.ret = substVars(x.ret, sub)
		cp.bounds = nil
		for _, b := range x.bounds {
			nb := b
			nb.t = substVars(b.t, sub)
			nb.b.args = nil
			for _, a := range b.b.args {
				nb.b.args = append(nb.b.args, substVars(a, sub))
			}
			cp.bounds = append(cp.bounds, nb)
		}
		return &cp
	case *tdyn:
		cp := &tdyn{trait: x.trait, args: make([]typ, len(x.args))}
		for i, a := range x.args {
			cp.args[i] = substVars(a, sub)
		}
		return cp
	}
	return t
}
