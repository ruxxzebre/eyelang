package check

import (
	"fmt"
	"sort"
	"strings"

	"eyelang/ir"
)

// Function values and the effects their types state (SPECS.md §12).
//
// A function type written without uses is pure, except as a parameter's
// type: there it is a callback, and the function passed in may do anything,
// since its effects count where it is given (a lambda's where it is written,
// a named function's where it is named). Everywhere else, in a field, a
// collection, a result or a let, a function type states everything a
// function kept there may do, because whoever calls it later counts only
// what the type states.
//
// What a lambda does is known only once every function's effects are, so the
// check waits: where the type of a function value meets a function type that
// states its effects, unify records a limit, and checkFnLimits checks each
// one when effects are known.

// fxVar is what a function value may do: the functions whose effects it has.
// Two function values that must have one type, as the branches of an if,
// share one.
type fxVar struct {
	link *fxVar
	srcs []fxSrc
	// callback is set when the value may call a callback: a lambda that
	// captures one, or a branch that is one.
	callback   string
	callbackAt ir.Pos
}

// fxSrc is one function value: a lambda, or a function, contract operation
// or built-in named as a value.
type fxSrc struct {
	fn     *ir.Func // a lambda or a named function
	lambda bool
	fixed  []string // a contract operation's or a built-in's effects
	name   string   // for messages
	at     ir.Pos
}

// fxLimit is a function type that states its effects, met by a function value.
type fxLimit struct {
	fx *fxVar
	t  *tfun
}

// markCallbacks marks the function types a parameter's type is made of, as
// fn(Int) or List(fn() -> a), that state no effects: what is passed in for
// them are callbacks. A function type's own parameters and result keep their
// meaning.
func markCallbacks(t typ, param string) {
	switch x := t.(type) {
	case *tfun:
		if !x.hasUses {
			x.callback = param
		}
	case *tcon:
		for _, a := range x.args {
			markCallbacks(a, param)
		}
	case *ttuple:
		for _, e := range x.elems {
			markCallbacks(e, param)
		}
	}
}

// checkFieldMethods reports a method named like a field of its record that
// holds a function: `timer.read()` calls the field, so a method read would
// make the call mean two things (SPECS.md §8).
func (c *checker) checkFieldMethods() {
	for _, m := range c.prog.Modules {
		for _, t := range m.Types {
			if t.Alias == nil || t.Alias.Name != "record" {
				continue
			}
			for i, name := range t.Alias.Fields {
				meth := t.Methods[name]
				if meth == nil || i >= len(t.Alias.Args) || t.Alias.Args[i].Name != "fn" {
					continue
				}
				c.typeErrorf(meth.Pos, "%s has a field %s that holds a function, which .%s() calls, so a method %s would make that call mean two things: rename one of them", t.Name, name, name, name)
			}
		}
	}
}

func newFx(src fxSrc) *fxVar { return &fxVar{srcs: []fxSrc{src}} }

func (v *fxVar) root() *fxVar {
	for v.link != nil {
		v = v.link
	}
	return v
}

func joinFx(a, b *fxVar) {
	a, b = a.root(), b.root()
	if a == b {
		return
	}
	b.link = a
	a.srcs = append(a.srcs, b.srcs...)
	b.srcs = nil
	if a.callback == "" {
		a.callback, a.callbackAt = b.callback, b.callbackAt
	}
}

// unifyEffects checks that a function of type got may go where a want is
// expected, as far as what each may do, and says why not. Where a function
// value is involved, what it does isn't known yet, and a limit is recorded.
func (c *checker) unifyEffects(want, got *tfun) string {
	switch {
	case want.callback != "":
		// A callback may do anything. A lambda's or a named function's
		// effects count where it is written; a function kept in data, as
		// h.run, does what its type states, and that counts where it is
		// given.
		if got.fx == nil && c.ts.giving != nil {
			for _, u := range got.uses {
				c.ts.giving.effect(u)
			}
		}
		return ""
	case want.fx != nil && got.fx != nil:
		joinFx(want.fx, got.fx)
	case want.fx != nil:
		c.limitFx(want.fx, got)
	case got.fx != nil:
		c.limitFx(got.fx, want)
	case got.callback != "":
		return fmt.Sprintf("%s is a function passed in, whose effects count where it is given, so it can't be kept where a function type says what it does: state its effects in its parameter's type, as %s: fn(…) uses {…}", got.callback, got.callback)
	default:
		var extra []string
		for _, u := range got.uses {
			if !covered(want.uses, u) {
				extra = append(extra, u)
			}
		}
		if len(extra) > 0 {
			return fmt.Sprintf("it uses %s, which the function type expected doesn't list", strings.Join(extra, ", "))
		}
	}
	return ""
}

// limitFx records that a function value goes where a function of type t is.
// A callback taints it instead: it may then do anything.
func (c *checker) limitFx(v *fxVar, t *tfun) {
	if t.callback != "" {
		if r := v.root(); r.callback == "" {
			r.callback = t.callback
			if len(r.srcs) > 0 {
				r.callbackAt = r.srcs[0].at
			}
		}
		return
	}
	c.ts.fxLimits = append(c.ts.fxLimits, fxLimit{fx: v, t: t})
}

// checkFnLimits reports each function value kept where a function type
// states less than the value does.
func (c *checker) checkFnLimits(all map[*ir.Func]map[string]bool) {
	seen := map[string]bool{}
	report := func(at ir.Pos, format string, args ...any) {
		key := fmt.Sprintf("%d:%s", at.Offset, fmt.Sprintf(format, args...))
		if !seen[key] {
			seen[key] = true
			c.typeErrorf(at, format, args...)
		}
	}
	for _, l := range c.ts.fxLimits {
		r := l.fx.root()
		if r.callback != "" {
			report(r.callbackAt, "this function may call %s, a function passed in, whose effects count where it is given, so it can't be kept where its type is %s: state %s's effects in its parameter's type, as %s: fn(…) uses {…}", r.callback, show(l.t), r.callback, r.callback)
			continue
		}
		for _, s := range r.srcs {
			var extra []string
			for e := range c.srcEffects(s, all) {
				if !covered(l.t.uses, e) {
					extra = append(extra, e)
				}
			}
			if len(extra) == 0 {
				continue
			}
			sort.Strings(extra)
			list := strings.Join(extra, ", ")
			report(s.at, "%s uses %s, which its type here, %s, doesn't list: a function kept in data does only what its type says, so add uses {%s} to the type", s.name, list, show(l.t), list)
		}
	}
}

func sortedEffects(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for e := range set {
		out = append(out, e)
	}
	sort.Strings(out)
	return out
}

// srcEffects is what calling a function value does.
func (c *checker) srcEffects(s fxSrc, all map[*ir.Func]map[string]bool) map[string]bool {
	set := map[string]bool{}
	switch {
	case s.fn == nil:
		for _, e := range s.fixed {
			set[e] = true
		}
	case s.lambda:
		for e := range c.ts.lambdaEffects[s.fn] {
			set[e] = true
		}
		for _, g := range c.ts.lambdaCallees[s.fn] {
			for e := range effectsOf(g, all) {
				set[e] = true
			}
		}
	default:
		for e := range effectsOf(s.fn, all) {
			set[e] = true
		}
	}
	return set
}
