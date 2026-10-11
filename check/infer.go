package check

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"eyelang/ir"
)

// This file infers and checks types over the resolved form in package ir.
//
// Inference is Hindley-Milner style inside bodies, checked against the
// signatures as written: a public function states its types, and a private
// one may leave them out. Declared type variables (`List(a)`) are generic: a
// call instantiates them afresh, and inside the function they stand for any
// type. Parameters and results a private function leaves out are inferred
// from its body and its calls; they are the same at every call.
//
// A few things the spec doesn't yet say enough about are approximated and
// say so where they are: function types kept in data, inline record types,
// `dyn`, traits beyond the operators the built-in types have, and NonZero
// divisors, which are their number type.

// typeState is what type checking keeps across functions.
type typeState struct {
	params   map[*ir.Func][]typ // inferred types of parameters a function leaves untyped
	results  map[*ir.Func]typ   // inferred results of functions that leave them out
	globals  map[*ir.Global]typ
	checking map[*ir.Global]bool
	effects  map[*ir.Func]map[string]bool // effects each function has itself
	callees  map[*ir.Func][]*ir.Func      // the functions each calls
	jobSlots map[*ir.Func][]typ           // slot types of job bodies, for the Save check
	zones    []zoneLit                    // string literals that may be zones
	option   *ir.Type
	result   *ir.Type
	outcome  *ir.Type
	decision *ir.Type
	uncert   *ir.Variant
	// bounds holds the traits a function's own type variables have, from
	// its `where` clause.
	bounds map[*tvar][]cbound
}

type zoneLit struct {
	t    typ
	name string
	pos  ir.Pos
}

// fnKind is the kind of code a body is, which decides its rules.
type fnKind int

const (
	kindPlain fnKind = iota
	kindJob
	kindEntity
	kindOperation
	kindTest
	kindGlobal
	kindView
	kindLambda
)

// fnCheck checks one function's body.
type fnCheck struct {
	c       *checker
	fn      *ir.Func
	kind    fnKind
	slots   []typ
	ret     typ // what `return` returns: the outermost function's result
	self    typ
	entity  *ir.Entity
	env     typeEnv // the function's type variables
	outer   *fnCheck
	top     *fnCheck // the outermost function, for effects and pending checks
	waiting bool     // checking the call a `!` marks
	records []pendingRecord
	matches []pendingMatch
	lets    []pendingLet
	calls   []pendingCall
	reqs    []requirement
}

type pendingRecord struct {
	t      *tvar
	at     ir.Pos
	names  []string
	values []typ
	pos    []ir.Pos
	e      *ir.Record
}

type pendingMatch struct {
	m *ir.Match
	t typ
}

type pendingLet struct {
	l *ir.Let
	t typ
}

type pendingCall struct {
	call *ir.Call
	t    typ
}

// discard is the expectation of a value nobody uses: a statement, or the
// last expression of a function that returns nothing. A match or an if there
// needs no common type for its branches.
type tdiscard struct{}

func (tdiscard) isType() {}

var discard typ = tdiscard{}

func (c *checker) typeErrorf(at ir.Pos, format string, args ...any) {
	c.errorf(at, format, args...)
}

// checkTypes checks every body of the program.
func (c *checker) checkTypes() {
	pre := c.prelude.mod
	c.ts = &typeState{
		params: map[*ir.Func][]typ{}, results: map[*ir.Func]typ{}, globals: map[*ir.Global]typ{},
		checking: map[*ir.Global]bool{}, effects: map[*ir.Func]map[string]bool{},
		callees: map[*ir.Func][]*ir.Func{}, jobSlots: map[*ir.Func][]typ{},
		option: pre.Types["Option"], result: pre.Types["Result"], outcome: pre.Types["Outcome"],
		decision: pre.Types["Decision"], uncert: pre.Variants["Uncertain"],
		bounds: map[*tvar][]cbound{},
	}
	c.fillImplMethods()
	c.checkDyns()
	type body struct {
		fn     *ir.Func
		kind   fnKind
		entity *ir.Entity
		self   *ir.Type
		selfT  typ       // self's type, for a method of no declared type
		trait  *ir.Trait // the trait whose default method this is
		op     *ir.Op
		want   typ
	}
	var bodies []body
	for _, m := range c.prog.Modules {
		for _, f := range m.Funcs {
			if f.Body != nil {
				bodies = append(bodies, body{fn: f})
			}
		}
		for _, t := range m.Types {
			for _, f := range t.Methods {
				if f.Body != nil && f.Receiver == t { // a trait's default is checked with the trait
					bodies = append(bodies, body{fn: f, self: t})
				}
			}
		}
		for _, e := range m.Entities {
			for _, f := range e.Methods {
				bodies = append(bodies, body{fn: f, kind: kindEntity, entity: e})
			}
			for _, s := range e.States {
				if s.Default != nil {
					bodies = append(bodies, body{fn: s.Default, kind: kindGlobal, want: c.fromExpr(s.Type, typeEnv{}, false)})
				}
			}
			for _, mg := range e.Migrations {
				bodies = append(bodies, body{fn: mg.Func, kind: kindGlobal, want: c.migrationResult(e, mg)})
			}
		}
		for _, j := range m.Jobs {
			bodies = append(bodies, body{fn: j.Func, kind: kindJob})
			for _, t := range j.Triggers {
				if t.Schedule != nil {
					bodies = append(bodies, body{fn: t.Schedule, kind: kindGlobal, want: con("Schedule")})
				}
			}
		}
		for _, v := range m.Views {
			if v.Func != nil {
				bodies = append(bodies, body{fn: v.Func, kind: kindView})
			}
		}
		for _, ct := range m.Contracts {
			for _, op := range ct.Ops {
				if op.Impl != nil {
					bodies = append(bodies, body{fn: op.Impl, kind: kindOperation, op: op})
				}
			}
		}
		for _, t := range m.Tests {
			bodies = append(bodies, body{fn: t.Func, kind: kindTest})
		}
		for _, tr := range m.Traits {
			for _, f := range tr.Methods {
				if f.Body != nil {
					bodies = append(bodies, body{fn: f, trait: tr}) // a default: self is whatever implements it
				}
			}
		}
		for _, impl := range m.Impls {
			if impl.For == nil || impl.For.Decl != nil || impl.For.Entity != nil {
				continue // a declared type's impl methods are among its methods
			}
			for _, f := range impl.Methods {
				if f.Body != nil && (impl.Trait == nil || impl.Trait.Methods[f.Name] != f) {
					bodies = append(bodies, body{fn: f, selfT: c.fromExpr(impl.For, typeEnv{}, false)})
				}
			}
		}
	}
	sort.SliceStable(bodies, func(i, j int) bool {
		a, b := bodies[i].fn.Pos, bodies[j].fn.Pos
		if a.File != b.File {
			if a.File == nil || b.File == nil {
				return a.File == nil
			}
			return a.File.Path < b.File.Path
		}
		return a.Offset < b.Offset
	})
	for _, m := range c.prog.Modules {
		for _, g := range m.Globals {
			c.globalType(g)
		}
	}
	for _, b := range bodies {
		x := c.newFnCheck(b.fn, b.kind, nil)
		x.entity = b.entity
		if b.entity != nil {
			x.self = &tcon{name: b.entity.Name, entity: b.entity}
		}
		if b.self != nil {
			self, env := c.declType(b.self)
			for _, a := range self.args {
				a.(*tvar).rigid = true
			}
			for name, v := range env {
				v.(*tvar).name = name
				x.env[name] = v
			}
			x.self = self
		}
		if b.selfT != nil {
			x.self = b.selfT
		}
		if b.trait != nil {
			x.self = x.traitSelf(b.trait)
		}
		var ret typ
		if b.op != nil {
			ret = x.operation(b.op)
		} else {
			ret = x.signature()
		}
		if b.want != nil {
			ret = b.want
		}
		x.ret = ret
		x.body(ret)
		if b.kind == kindJob {
			c.ts.jobSlots[b.fn] = x.slots
		}
	}
	c.checkZones()
	c.checkEffects()
}

func (c *checker) newFnCheck(fn *ir.Func, kind fnKind, outer *fnCheck) *fnCheck {
	x := &fnCheck{c: c, fn: fn, kind: kind, slots: make([]typ, fn.Slots), env: typeEnv{}, outer: outer}
	x.top = x
	if outer != nil {
		x.top = outer.top
		x.ret = outer.ret
		x.self = outer.self
		x.entity = outer.entity
		for k, v := range outer.env {
			x.env[k] = v
		}
	}
	for i := range x.slots {
		x.slots[i] = c.fresh()
	}
	return x
}

// signature gives the function's parameters their types, as written or
// inferred, and returns its result type.
func (x *fnCheck) signature() typ {
	fn := x.fn
	for i, p := range fn.Params {
		x.slots[p.Slot] = x.c.paramType(fn, i, x.env, true)
		if p.Default != nil {
			x.check(p.Default, x.slots[p.Slot])
		}
	}
	ret := x.c.resultType(fn, x.env, true)
	// The function's own type variables have the traits it requires.
	for _, b := range fn.Bounds {
		v, ok := prune(x.env[b.Var]).(*tvar)
		if !ok {
			continue
		}
		cb := cbound{trait: b.Trait}
		for _, a := range b.Args {
			cb.args = append(cb.args, x.c.fromExpr(a, x.env, true))
		}
		x.c.ts.bounds[v] = append(x.c.ts.bounds[v], cb)
	}
	return ret
}

// traitSelf is self's type in a trait's default method: whatever type
// implements the trait, so it has the trait's methods and nothing else. The
// trait's own type parameters stand for any type there.
func (x *fnCheck) traitSelf(tr *ir.Trait) typ {
	self := x.c.fresh()
	self.name, self.rigid = "Self", true
	b := cbound{trait: tr}
	for _, p := range tr.Params {
		v := x.c.fresh()
		v.name, v.rigid = p, true
		x.env[p] = v
		b.args = append(b.args, v)
	}
	x.c.ts.bounds[self] = append(x.c.ts.bounds[self], b)
	return self
}

// operation gives a function that serves a contract's operation the types
// the contract declares.
func (x *fnCheck) operation(op *ir.Op) typ {
	for i, p := range x.fn.Params {
		switch {
		case i < len(op.Params):
			x.slots[p.Slot] = x.c.fromExpr(op.Params[i].Type, x.env, true)
		case op.Key && i == len(op.Params):
			x.slots[p.Slot] = tKey
		}
	}
	return x.c.fromExpr(op.Return, x.env, true)
}

// paramType is the type of a function's parameter i, in env. A parameter
// written without a type has one inferred type for the whole program.
func (c *checker) paramType(fn *ir.Func, i int, env typeEnv, rigid bool) typ {
	p := fn.Params[i]
	if p.Type != nil {
		return c.fromExpr(p.Type, env, rigid)
	}
	ps := c.ts.params[fn]
	if ps == nil {
		ps = make([]typ, len(fn.Params))
		for j := range ps {
			ps[j] = c.fresh()
		}
		c.ts.params[fn] = ps
	}
	return ps[i]
}

// resultType is what a function returns: its type as written; for one
// without `->`, nothing if its body is a block, and otherwise the type of
// its expression, inferred.
func (c *checker) resultType(fn *ir.Func, env typeEnv, rigid bool) typ {
	if fn.Return != nil {
		return c.fromExpr(fn.Return, env, rigid)
	}
	if _, block := fn.Body.(*ir.Block); block || fn.Body == nil {
		return tUnit
	}
	r, ok := c.ts.results[fn]
	if !ok {
		r = c.fresh()
		c.ts.results[fn] = r
	}
	return r
}

// funcType is the type of a declared function at one use: its type
// variables are fresh.
func (c *checker) funcType(fn *ir.Func, env typeEnv) *tfun {
	if env == nil {
		env = typeEnv{}
	}
	t := &tfun{}
	for i, p := range fn.Params {
		t.params = append(t.params, fparam{name: p.Name, t: c.paramType(fn, i, env, false), hasDefault: p.Default != nil, mut: p.Mut})
	}
	t.ret = c.resultType(fn, env, false)
	// Each call needs the traits the function requires of its type variables.
	for _, b := range fn.Bounds {
		v, ok := env[b.Var]
		if !ok {
			v = c.fresh()
			env[b.Var] = v
		}
		cb := cbound{trait: b.Trait}
		for _, a := range b.Args {
			cb.args = append(cb.args, c.fromExpr(a, env, false))
		}
		t.bounds = append(t.bounds, boundUse{t: v, b: cb, fn: fn.Name, v: b.Var})
	}
	return t
}

// body checks the function's body against its result.
func (x *fnCheck) body(ret typ) {
	if x.fn.Body == nil {
		return
	}
	want := ret
	if r, ok := prune(ret).(*tcon); ok && r.name == "()" && r.decl == nil {
		want = discard
	}
	x.check(x.fn.Body, want)
	x.finish()
}

// finish settles what waited for the whole body: record literals whose
// type came from later code, exhaustiveness, and the types calls returned.
func (x *fnCheck) finish() {
	c := x.c
	for _, r := range x.records {
		rec, ok := isRecord(r.t)
		if !ok {
			if _, isVar := prune(r.t).(*tvar); isVar {
				c.typeErrorf(r.at, "which record type is this? Nothing here says; write it where the value is bound, as `let x: T = …`")
			} else {
				c.typeErrorf(r.at, "expected %s, found a record", show(r.t))
			}
			continue
		}
		x.recordFields(rec, r.names, r.values, r.pos, r.at)
		if r.e != nil {
			r.e.Type = rec.decl
		}
	}
	for _, m := range x.matches {
		c.checkExhaustive(m.m, m.t)
	}
	for _, l := range x.lets {
		if l.l.Else == nil {
			c.checkIrrefutable(l.l, l.t)
		}
	}
	for _, call := range x.calls {
		call.call.Expected = toExpr(defaultLiteral(call.t))
	}
	x.settleRequirements()
}

// ---------------------------------------------------------------- expressions

// check infers e's type and makes it want, when want is given.
func (x *fnCheck) check(e ir.Expr, want typ) typ {
	got := x.infer(e, want)
	x.c.typed(e.Position(), got)
	if want == nil || want == discard {
		return got
	}
	if _, isDiscard := got.(tdiscard); isDiscard {
		return want
	}
	x.expect(e.Position(), want, got)
	return want
}

// expect reports where got isn't want.
func (x *fnCheck) expect(at ir.Pos, want, got typ) bool {
	if w, ok := prune(want).(*tcon); ok && !w.uncertain {
		if g, ok := prune(got).(*tcon); ok && g.uncertain && sameCon(w, g) {
			x.c.typeErrorf(at, "this command's reply may have been lost, so it may be Uncertain(key): reconcile it with .reconcile!(), or match it with an Uncertain(_) arm")
			return false
		}
	}
	if m := x.c.unify(want, got); m != nil {
		msg := fmt.Sprintf("expected %s, found %s", show(defaultLiteral(want)), show(defaultLiteral(got)))
		if m.dyn != nil {
			msg += fmt.Sprintf(": %s doesn't implement %s", show(m.lacks), m.dyn.trait.Name)
		}
		x.c.typeErrorf(at, "%s", msg)
		return false
	}
	return true
}

func (x *fnCheck) infer(e ir.Expr, want typ) typ {
	c := x.c
	switch e := e.(type) {
	case *ir.Lit:
		return x.literal(e, want)
	case *ir.Interpolate:
		for _, p := range e.Parts {
			t := x.check(p, nil)
			if _, text := p.(*ir.Lit); !text {
				x.requireNamed(t, "Show", p.Position(), "showing a value in a string")
			}
		}
		return c.freshLit(litString)
	case *ir.InZone:
		x.check(e.Zone, tZone)
		return tInstant
	case *ir.EmptyBraces:
		if want != nil && want != discard {
			return want
		}
		return c.fresh()
	case *ir.Local:
		if e.Slot < len(x.slots) {
			return x.slots[e.Slot]
		}
		return unknown
	case *ir.Self:
		if x.self == nil {
			return unknown
		}
		return x.self
	case *ir.SelfField:
		if x.entity != nil {
			if t, ok := c.stateType(x.entity, e.Name); ok {
				return t
			}
		}
		return unknown
	case *ir.Ref:
		return x.ref(e)
	case *ir.Call:
		return x.call(e, want)
	case *ir.MethodCall:
		return x.methodCall(e, want)
	case *ir.Waiting:
		x.top.effect("wait")
		prev := x.waiting
		x.waiting = true
		t := x.check(e.Call, want)
		x.waiting = prev
		return t
	case *ir.Field:
		return x.field(e)
	case *ir.Index:
		return x.index(e)
	case *ir.Unary:
		t := x.check(e.X, nil)
		if e.Op == "!" {
			x.expect(e.X.Position(), tBool, t)
			return tBool
		}
		return t
	case *ir.Binary:
		return x.binary(e)
	case *ir.Range:
		elem := c.fresh()
		if e.Start != nil {
			x.check(e.Start, elem)
		}
		if e.End != nil {
			x.check(e.End, elem)
		}
		return con("Range", elem)
	case *ir.If:
		x.check(e.Cond, tBool)
		if e.Else == nil {
			x.check(e.Then, discard)
			return tUnit
		}
		if want == discard {
			x.check(e.Then, discard)
			x.check(e.Else, discard)
			return discard
		}
		t := want
		if t == nil {
			t = c.fresh()
		}
		x.check(e.Then, t)
		x.check(e.Else, t)
		return t
	case *ir.Match:
		return x.match(e, want)
	case *ir.Block:
		for i, s := range e.Exprs {
			if i == len(e.Exprs)-1 && e.Value {
				return x.check(s, want)
			}
			t := x.check(s, discard)
			if i == len(e.Exprs)-1 && endless(s) {
				return t // code after it never runs
			}
		}
		return tUnit
	case *ir.Let:
		var t typ = c.fresh()
		if e.Type != nil {
			t = c.fromExpr(e.Type, x.env, true)
		}
		x.check(e.Value, t)
		if e.Else != nil {
			x.check(e.Else, discard)
		}
		e.Pattern = x.bindPattern(e.Pattern, t, e.Pos)
		x.top.lets = append(x.top.lets, pendingLet{l: e, t: t})
		return tUnit
	case *ir.Assign:
		target := x.placeType(e.Target)
		if e.Op == "=" {
			x.check(e.Value, target)
			return tUnit
		}
		value := x.check(e.Value, nil)
		result := x.arith(strings.TrimSuffix(e.Op, "="), target, value, e.Pos)
		x.expect(e.Pos, target, result)
		return tUnit
	case *ir.For:
		elem := x.elemOf(x.check(e.Iter, nil), e.Iter.Position())
		e.Pattern = x.bindPattern(e.Pattern, elem, e.Pos)
		x.check(e.Body, discard)
		return tUnit
	case *ir.While:
		x.check(e.Cond, tBool)
		x.check(e.Body, discard)
		if endless(e) {
			// `while true` without `break` never ends but by `return`, so
			// its value can be of any type, as code after it never runs.
			return c.fresh()
		}
		return tUnit
	case *ir.Break, *ir.Continue:
		return c.fresh()
	case *ir.Return:
		if e.Value != nil {
			want := x.ret
			if r, ok := prune(want).(*tcon); ok && r.name == "()" && r.decl == nil {
				want = nil
			}
			x.check(e.Value, want)
		}
		return c.fresh()
	case *ir.Lambda:
		return x.lambda(e, want)
	case *ir.Record:
		return x.record(e, want)
	case *ir.RecordUpdate:
		base := x.check(e.Base, want)
		rec, ok := isRecord(base)
		if ok {
			c.useFields(e, rec, e.Names)
		}
		if !ok {
			if _, isVar := prune(base).(*tvar); !isVar {
				c.typeErrorf(e.Base.Position(), "only a record can be updated with `with`; this is %s", show(base))
			}
			for _, v := range e.Values {
				x.check(v, nil)
			}
			return base
		}
		for i, name := range e.Names {
			ft, ok := c.fieldType(rec, name)
			if !ok {
				c.typeErrorf(e.Values[i].Position(), "%s has no field %s", show(rec), name)
				x.check(e.Values[i], nil)
				continue
			}
			x.check(e.Values[i], ft)
		}
		return base
	case *ir.List:
		elem := typ(c.fresh())
		if w, ok := prune(orNil(want)).(*tcon); ok && w.name == "List" && w.decl == nil && len(w.args) == 1 {
			elem = w.args[0]
		}
		for _, el := range e.Elems {
			x.check(el, elem)
		}
		return con("List", elem)
	case *ir.Map:
		k, v := typ(c.fresh()), typ(c.fresh())
		if w, ok := prune(orNil(want)).(*tcon); ok && w.name == "Map" && w.decl == nil && len(w.args) == 2 {
			k, v = w.args[0], w.args[1]
		}
		for i := range e.Keys {
			x.check(e.Keys[i], k)
			x.check(e.Values[i], v)
		}
		return con("Map", k, v)
	case *ir.Tuple:
		t := &ttuple{}
		var wantElems []typ
		if w, ok := prune(orNil(want)).(*ttuple); ok && len(w.elems) == len(e.Elems) {
			wantElems = w.elems
		}
		for i, el := range e.Elems {
			var w typ
			if wantElems != nil {
				w = wantElems[i]
			}
			t.elems = append(t.elems, x.check(el, w))
		}
		return t
	case *ir.Try:
		return x.try(e)
	case *ir.Default:
		inner := c.fresh()
		x.check(e.X, &tcon{name: "Option", decl: c.ts.option, args: []typ{inner}})
		x.check(e.Or, inner)
		return inner
	case *ir.With:
		x.check(e.Value, nil)
		return tUnit
	}
	return unknown
}

// endless reports whether e is `while true` without a `break`.
func endless(e ir.Expr) bool {
	w, ok := e.(*ir.While)
	if !ok {
		return false
	}
	lit, ok := w.Cond.(*ir.Lit)
	return ok && lit.Value == true && !breaks(w.Body)
}

// breaks reports whether a loop's body has a `break` that leaves it.
func breaks(body ir.Expr) bool {
	found := false
	walk(body, false, func(e ir.Expr, _ bool) bool {
		switch e.(type) {
		case *ir.Break:
			found = true
		case *ir.While, *ir.For, *ir.Lambda:
			return false // a break in there leaves that loop
		}
		return !found
	})
	return found
}

func orNil(t typ) typ {
	if t == nil || t == discard {
		return unknown
	}
	return t
}

func (x *fnCheck) literal(e *ir.Lit, want typ) typ {
	switch v := e.Value.(type) {
	case *big.Int:
		return x.c.freshLit(litInt)
	case float64:
		return tFloat
	case string:
		t := x.c.freshLit(litString)
		x.c.ts.zones = append(x.c.ts.zones, zoneLit{t: t, name: v, pos: e.Pos})
		return t
	case bool:
		return tBool
	case rune:
		return tChar
	case ir.Unit:
		return tUnit
	case ir.Duration:
		return tDuration
	case ir.Size:
		return tSize
	case ir.Date:
		return tDate
	case ir.TimeOfDay:
		return tTime
	case ir.Instant:
		if _, err := time.LoadLocation(v.Zone); err != nil {
			x.c.typeErrorf(e.Pos, "unknown time zone %q: a zone is named as in the tz database, such as \"Europe/Dublin\"", v.Zone)
		}
		return tInstant
	}
	return unknown
}

// checkZones checks the string literals that became zones against the
// time-zone database.
func (c *checker) checkZones() {
	for _, z := range c.ts.zones {
		if t, ok := prune(z.t).(*tcon); ok && t.name == "Zone" && t.decl == nil {
			if _, err := time.LoadLocation(z.name); err != nil {
				c.typeErrorf(z.pos, "unknown time zone %q: a zone is named as in the tz database, such as \"Europe/Dublin\"", z.name)
			}
		}
	}
}

func (x *fnCheck) ref(e *ir.Ref) typ {
	c := x.c
	switch d := e.Decl.(type) {
	case *ir.Func:
		return c.funcType(d, nil)
	case *ir.Global:
		return c.globalType(d)
	case *ir.Variant:
		return c.variantType(d)
	case *ir.Entity:
		return &tentity{e: d}
	case *ir.Job:
		return &tjob{job: d}
	case *ir.View:
		if d.Entity != nil {
			return con("List", &tcon{name: d.Entity.Name, entity: d.Entity})
		}
	case *ir.Op:
		return c.opType(d)
	case *ir.Event:
		return con("Event", c.fromExpr(d.Type, typeEnv{}, false))
	case *ir.Module:
		return tmodule{}
	}
	return unknown
}

// variantType is a variant as a value: the value, for one without fields,
// or its constructor.
func (c *checker) variantType(v *ir.Variant) typ {
	if v.Type == nil {
		return unknown
	}
	t, env := c.declType(v.Type)
	if len(v.Fields) == 0 {
		return t
	}
	f := &tfun{ret: t}
	for _, field := range v.Fields {
		f.params = append(f.params, fparam{name: field.Name, t: c.fromExpr(field.Type, env, false)})
	}
	return f
}

// opType is a contract operation as a function. A command may also come back
// Uncertain, and a command with @key takes the key last.
func (c *checker) opType(op *ir.Op) typ {
	env := typeEnv{}
	f := &tfun{}
	for _, p := range op.Params {
		f.params = append(f.params, fparam{name: p.Name, t: c.fromExpr(p.Type, env, false), hasDefault: p.Default != nil})
	}
	if op.Key {
		f.params = append(f.params, fparam{name: "key", t: tKey})
	}
	ret := c.fromExpr(op.Return, env, false)
	if op.Command {
		if r, ok := ret.(*tcon); ok && r.decl != nil && len(r.decl.Variants) > 0 {
			cp := *r
			cp.uncertain, cp.reconcilable = true, op.Reconcile != ""
			ret = &cp
		}
	}
	f.ret = ret
	return f
}

func (c *checker) globalType(g *ir.Global) typ {
	if t, ok := c.ts.globals[g]; ok {
		return t
	}
	var t typ = c.fresh()
	if g.Type != nil {
		t = c.fromExpr(g.Type, typeEnv{}, false)
	}
	c.ts.globals[g] = t
	if g.Init != nil && !c.ts.checking[g] {
		c.ts.checking[g] = true
		x := c.newFnCheck(g.Init, kindGlobal, nil)
		x.ret = t
		x.check(g.Init.Body, t)
		x.finish()
	}
	return t
}

// ---------------------------------------------------------------- calls

// call checks a call of a function value, a constructor, an entity or a
// built-in function.
func (x *fnCheck) call(e *ir.Call, want typ) typ {
	c := x.c
	if r, ok := e.Func.(*ir.Ref); ok {
		c.useArgs(e, r.Decl)
		switch d := r.Decl.(type) {
		case ir.Builtin:
			t := x.builtin(d.Name, e.Args, want, e.Pos)
			x.top.calls = append(x.top.calls, pendingCall{e, t})
			return t
		case *ir.Entity:
			id := unknown
			if d.ID != nil {
				id = c.fromExpr(d.ID.Type, typeEnv{}, false)
			}
			x.args(&tfun{params: []fparam{{name: "id", t: id}}, ret: tUnit}, e.Args, d.Name, e.Pos)
			return con("Ref", &tcon{name: d.Name, entity: d})
		case *ir.Op:
			x.top.effect("net " + d.Contract.Name + "." + d.Name)
		case *ir.Func:
			x.top.callee(d)
			if d.Extern && d.HasUses {
				for _, u := range d.Uses {
					x.top.effect(u)
				}
			}
		}
	}
	callee := x.check(e.Func, nil)
	t := x.apply(callee, e.Args, want, calleeName(e.Func), e.Pos)
	x.top.calls = append(x.top.calls, pendingCall{e, t})
	if r, ok := e.Func.(*ir.Ref); ok {
		if fn, ok := r.Decl.(*ir.Func); ok && fn.Host == "ai.generate" {
			// A model is asked for a value by its type's JSON Schema (§19).
			if res, ok := prune(t).(*tcon); ok && res.decl == c.ts.result && len(res.args) == 2 {
				x.requireNamed(res.args[0], "Schema", e.Pos, "ai.generate")
			}
		}
	}
	return t
}

func calleeName(e ir.Expr) string {
	switch f := e.(type) {
	case *ir.Ref:
		switch d := f.Decl.(type) {
		case *ir.Func:
			return d.Name
		case *ir.Variant:
			return d.Name
		case *ir.Op:
			return d.Name
		}
	case *ir.Local:
		return f.Name
	}
	return "this function"
}

// apply checks a call of a value of type callee.
func (x *fnCheck) apply(callee typ, args []ir.Arg, want typ, name string, at ir.Pos) typ {
	switch f := prune(callee).(type) {
	case *tfun:
		x.args(f, args, name, at)
		return f.ret
	case *tvar:
		if f.rigid {
			break
		}
		fn := &tfun{ret: x.c.fresh()}
		for _, a := range args {
			fn.params = append(fn.params, fparam{t: x.check(a.Value, nil)})
		}
		x.c.unify(f, fn)
		return fn.ret
	case tunknown:
	default:
		x.c.typeErrorf(at, "%s isn't a function; it is %s", name, show(callee))
		x.inferArgs(args)
		return unknown
	}
	x.inferArgs(args)
	return unknown
}

func (x *fnCheck) inferArgs(args []ir.Arg) {
	for _, a := range args {
		x.check(a.Value, nil)
	}
}

// args matches a call's arguments to the parameters of f, by name and then
// in order, and checks each. Lambdas are checked last, so that the other
// arguments can give their parameters types.
func (x *fnCheck) args(f *tfun, args []ir.Arg, name string, at ir.Pos) {
	assigned := make([]bool, len(f.params))
	target := make([]int, len(args))
	next := 0
	for i, a := range args {
		target[i] = -1
		if a.Name != "" {
			for j, p := range f.params {
				if p.name == a.Name {
					target[i] = j
				}
			}
			if target[i] < 0 {
				x.c.typeErrorf(a.Value.Position(), "%s has no parameter %s", name, a.Name)
				continue
			}
		} else {
			for next < len(f.params) && assigned[next] {
				next++
			}
			if next >= len(f.params) {
				x.c.typeErrorf(a.Value.Position(), "too many arguments to %s: it takes %d", name, len(f.params))
				continue
			}
			target[i] = next
		}
		if assigned[target[i]] {
			x.c.typeErrorf(a.Value.Position(), "%s is given twice", f.params[target[i]].name)
		}
		assigned[target[i]] = true
	}
	for pass := 0; pass < 2; pass++ {
		for i, a := range args {
			_, isLambda := a.Value.(*ir.Lambda)
			if isLambda != (pass == 1) {
				continue
			}
			if target[i] < 0 {
				x.check(a.Value, nil)
				continue
			}
			x.check(a.Value, f.params[target[i]].t)
		}
	}
	for j, p := range f.params {
		if !assigned[j] && !p.hasDefault {
			x.c.typeErrorf(at, "%s needs an argument for %s", name, paramName(p, j))
		}
	}
	for _, bu := range f.bounds {
		x.top.reqs = append(x.top.reqs, requirement{t: bu.t, b: bu.b, at: at, where: name, role: bu.v})
	}
	x.mutArgs(f, args, target, name)
}

// mutArgs checks a call's `mut` arguments (SPECS.md §7): a mut parameter is
// passed as `mut x`, only a mut parameter is, and a variable passed as mut is
// neither passed twice nor read in the same call, so no value is changed
// through two names.
func (x *fnCheck) mutArgs(f *tfun, args []ir.Arg, target []int, name string) {
	c := x.c
	for i, a := range args {
		if target[i] < 0 {
			continue
		}
		p := f.params[target[i]]
		switch {
		case p.mut && !a.Mut:
			c.typeErrorf(a.Value.Position(), "%s changes its argument %s, so the call says so: write mut %s", name, p.name, placeText(a.Value))
		case !p.mut && a.Mut:
			c.typeErrorf(a.Value.Position(), "%s doesn't change its argument %s, so it takes it without mut", name, paramName(p, target[i]))
		}
	}
	for i, a := range args {
		if !a.Mut {
			continue
		}
		for j, b := range args {
			if j == i {
				continue
			}
			if b.Mut && j < i && sameRoot(a.Place, b.Place) {
				c.typeErrorf(a.Value.Position(), "%s is passed as mut twice in one call, and a value can't be changed through two arguments", placeText(a.Value))
				continue
			}
			if !b.Mut && reads(b.Value, a.Place) {
				c.typeErrorf(b.Value.Position(), "%s is passed as mut and read in the same call; copy what you need into a let first", rootName(a))
			}
		}
	}
}

func sameRoot(a, b ir.Place) bool {
	if a.Self || b.Self {
		return a.Self && b.Self
	}
	return a.Slot == b.Slot
}

// reads reports whether e reads the variable at the root of p.
func reads(e ir.Expr, p ir.Place) bool {
	found := false
	walk(e, false, func(e ir.Expr, _ bool) bool {
		switch v := e.(type) {
		case *ir.Local:
			found = found || !p.Self && v.Slot == p.Slot
		case *ir.Self, *ir.SelfField:
			found = found || p.Self
		case *ir.Lambda:
			for _, capt := range v.Func.Captures {
				if reads(capt.From, p) {
					found = true
				}
			}
			return false
		}
		return !found
	})
	return found
}

// placeText writes a mut argument's place as the program does.
func placeText(e ir.Expr) string {
	switch v := e.(type) {
	case *ir.Local:
		return v.Name
	case *ir.Self:
		return "self"
	case *ir.SelfField:
		return v.Name
	case *ir.Field:
		return placeText(v.Value) + "." + v.Name
	}
	return "it"
}

func rootName(a ir.Arg) string {
	e := a.Value
	for {
		f, ok := e.(*ir.Field)
		if !ok {
			return placeText(e)
		}
		e = f.Value
	}
}

func paramName(p fparam, i int) string {
	if p.name != "" && p.name != "_" {
		return p.name
	}
	return fmt.Sprintf("its argument %d", i+1)
}

// methodCall checks `recv.name(args)`.
func (x *fnCheck) methodCall(e *ir.MethodCall, want typ) typ {
	c := x.c
	recv := x.check(e.Recv, nil)
	if _, isSelf := e.Recv.(*ir.Self); isSelf && x.entity != nil {
		if m, ok := x.entity.Methods[e.Name]; ok {
			c.useMethod(e, m)
			return x.applyFunc(m, nil, e.Args, e.Name, e.Pos)
		}
	}
	switch r := prune(recv).(type) {
	case *tcon:
		switch {
		case r.name == "Ref" && r.decl == nil && len(r.args) == 1:
			if s, ok := prune(r.args[0]).(*tcon); ok && s.entity != nil {
				x.top.effect("entity " + s.entity.Name)
				x.entityCall(e)
				if e.Name == "read" {
					c.useName(e, method("read"))
					x.inferArgs(e.Args)
					return s
				}
				if m, ok := s.entity.Methods[e.Name]; ok {
					c.useMethod(e, m)
					return x.applyFunc(m, nil, e.Args, s.entity.Name+"."+e.Name, e.Pos)
				}
				c.typeErrorf(e.Pos, "%s has no method %s", s.entity.Name, e.Name)
				x.inferArgs(e.Args)
				return unknown
			}
		case r.name == "Run" && r.decl == nil && e.Name == "join":
			c.useName(e, method("join"))
			x.inferArgs(e.Args)
			return &tcon{name: "Outcome", decl: c.ts.outcome, args: []typ{r.args[0]}}
		case r.name == "Event" && r.decl == nil && e.Name == "publish":
			c.useName(e, method("publish"))
			x.args(&tfun{params: []fparam{{name: "value", t: r.args[0]}}, ret: tUnit}, e.Args, "publish", e.Pos)
			return tUnit
		case r.decl == nil && r.entity == nil && (r.name == "TaskGroup" || r.name == "Sender" || r.name == "Receiver"):
			return x.channelMethod(r, e)
		case e.Name == "reconcile" && r.decl != nil:
			c.useName(e, method("reconcile"))
			x.inferArgs(e.Args)
			if r.uncertain && !r.reconcilable {
				c.typeErrorf(e.Pos, "this command names no @reconcile query, so its Uncertain outcome can't be reconciled: match Uncertain(key) instead")
			}
			cp := *r
			cp.uncertain, cp.reconcilable = false, false
			return &cp
		case r.decl != nil:
			if m, ok := r.decl.Methods[e.Name]; ok && m.HasSelf {
				c.useMethod(e, m)
				x.top.callee(m)
				if m.MutSelf && e.RecvPlace == nil {
					what := placeText(e.Recv)
					if what == "it" {
						c.typeErrorf(e.Pos, "%s takes mut self, so it changes what it is called on, and this value has no name to change: call it on a var", e.Name)
					} else {
						c.typeErrorf(e.Pos, "%s takes mut self, so it changes %s, which can't change: declare it with var, or as a mut parameter", e.Name, what)
					}
				}
				return x.applyFunc(m, envOf(r), e.Args, r.name+"."+e.Name, e.Pos)
			}
		case r.decl == nil && r.entity == nil:
			if t, ok := x.timeMethod(r, e); ok {
				c.useName(e, method(e.Name))
				c.useArgs(e, method(e.Name))
				return t
			}
		}
	case *tjob:
		if e.Name == "start" {
			c.useName(e, method("start"))
			c.useArgs(e, r.job)
			return x.start(r.job, e)
		}
	case *tdyn:
		// A dyn value has its trait's methods, looked up at run time.
		if m, ok := r.trait.Methods[e.Name]; ok && m.HasSelf {
			c.useMethod(e, m)
			env := typeEnv{}
			for i, p := range r.trait.Params {
				if i < len(r.args) {
					env[p] = r.args[i]
				}
			}
			return x.applyFunc(m, env, e.Args, r.trait.Name+"."+e.Name, e.Pos)
		}
		if !isBuiltin(e.Name) {
			c.typeErrorf(e.Pos, "%s has no method %s: a dyn value has only the methods of its trait", show(recv), e.Name)
			x.inferArgs(e.Args)
			return unknown
		}
	case *tvar:
		if r.rigid {
			// A type variable has the methods of the traits it requires.
			for _, b := range c.ts.bounds[r] {
				if m, ok := b.trait.Methods[e.Name]; ok && m.HasSelf {
					c.useMethod(e, m)
					env := typeEnv{}
					for i, p := range b.trait.Params {
						if i < len(b.args) {
							env[p] = b.args[i]
						}
					}
					return x.applyFunc(m, env, e.Args, b.trait.Name+"."+e.Name, e.Pos)
				}
			}
			if !isBuiltin(e.Name) {
				if r.name == "Self" && len(c.ts.bounds[r]) == 1 {
					tr := c.ts.bounds[r][0].trait
					c.typeErrorf(e.Pos, "self is any type that implements %s, so it has only the methods of %s, and %s isn't one", tr.Name, tr.Name, e.Name)
					x.inferArgs(e.Args)
					return unknown
				}
				c.typeErrorf(e.Pos, "%s may be any type, so it has no method %s: require a trait that has one, as `where %s: Trait`", r.name, e.Name, r.name)
				x.inferArgs(e.Args)
				return unknown
			}
		}
	}
	if r, ok := prune(recv).(*tcon); ok && r.decl == nil && r.entity == nil {
		if m := c.implMethod(r, e.Name); m != nil {
			c.useMethod(e, m)
			return x.applyFunc(m, typeEnv{}, e.Args, r.name+"."+e.Name, e.Pos)
		}
	}
	if isBuiltin(e.Name) {
		c.useName(e, ir.Builtin{Name: e.Name})
		args := append([]ir.Arg{{Value: e.Recv}}, e.Args...)
		return x.builtinWith(e.Name, args, recv, want, e.Pos)
	}
	if r, ok := prune(recv).(*tcon); ok && (r.decl != nil || r.entity != nil) {
		c.typeErrorf(e.Pos, "%s has no method %s", show(recv), e.Name)
	}
	x.inferArgs(e.Args)
	return unknown
}

// channelMethod checks group.spawn(f), send.put(x), send.close() and
// receive.take(): the methods of a task group and a channel's two ends.
func (x *fnCheck) channelMethod(r *tcon, e *ir.MethodCall) typ {
	c := x.c
	sig := func(ret typ, params ...fparam) typ {
		c.useName(e, method(e.Name))
		x.args(&tfun{params: params, ret: ret}, e.Args, r.name+"."+e.Name, e.Pos)
		return ret
	}
	switch {
	case r.name == "TaskGroup" && e.Name == "spawn":
		return sig(tUnit, fparam{name: "f", t: &tfun{ret: c.fresh()}})
	case r.name == "Sender" && e.Name == "put":
		return sig(tUnit, fparam{name: "value", t: r.args[0]})
	case r.name == "Sender" && e.Name == "close":
		return sig(tUnit)
	case r.name == "Receiver" && e.Name == "take":
		return sig(&tcon{name: "Option", decl: c.ts.option, args: []typ{r.args[0]}})
	}
	methods := map[string]string{"TaskGroup": "spawn", "Sender": "put and close", "Receiver": "take, and a for loop reads it"}
	c.typeErrorf(e.Pos, "%s has no method %s; its methods are %s", show(r), e.Name, methods[r.name])
	x.inferArgs(e.Args)
	return unknown
}

// entityCall reports an entity call in a job that isn't a durable step.
func (x *fnCheck) entityCall(e *ir.MethodCall) {
	if x.top.kind == kindJob && x.kind != kindLambda && !x.waiting {
		x.c.typeErrorf(e.Pos, "in a job, an entity call is a durable step: write %s!(…)", e.Name)
	}
}

// applyFunc checks a call of a declared function or method; env holds the
// receiver's type arguments.
func (x *fnCheck) applyFunc(fn *ir.Func, env typeEnv, args []ir.Arg, name string, at ir.Pos) typ {
	if env == nil {
		env = typeEnv{}
	} else {
		cp := typeEnv{}
		for k, v := range env {
			cp[k] = v
		}
		env = cp
	}
	f := x.c.funcType(fn, env)
	x.args(f, args, name, at)
	return f.ret
}

// start checks `job.start(args…, after: d)` or `at: instant`.
func (x *fnCheck) start(job *ir.Job, e *ir.MethodCall) typ {
	f := x.c.funcType(job.Func, nil)
	f.params = append(f.params,
		fparam{name: "after", t: tDuration, hasDefault: true},
		fparam{name: "at", t: tInstant, hasDefault: true},
		fparam{name: "key", t: tKey, hasDefault: true})
	if main := x.top.fn; main.Module != nil && main.Module.Main == main && x.top.kind == kindPlain {
		keyed := false
		for _, a := range e.Args {
			keyed = keyed || a.Name == "key"
		}
		if !keyed {
			x.c.typeErrorf(e.Pos, "eye run runs main every time it starts, so this would start %s again after each restart; give the run a key, which starts it once: %s.start(…, key: \"…\")", job.Name, job.Name)
		}
	}
	x.args(f, e.Args, job.Name+".start", e.Pos)
	return con("Run", f.ret)
}

// timeMethod checks the methods of time values (SPECS.md §20).
func (x *fnCheck) timeMethod(r *tcon, e *ir.MethodCall) (typ, bool) {
	sig := func(ret typ, params ...fparam) (typ, bool) {
		x.args(&tfun{params: params, ret: ret}, e.Args, e.Name, e.Pos)
		return ret, true
	}
	in := fparam{name: "in", t: tZone}
	switch {
	case r.name == "Instant" && e.Name == "date":
		return sig(tDate, in)
	case r.name == "Instant" && e.Name == "time":
		return sig(tTime, in)
	case r.name == "Time" && e.Name == "next":
		return sig(tInstant, in, fparam{name: "after", t: tInstant})
	case r.name == "Date" && e.Name == "at":
		return sig(tInstant, fparam{name: "time", t: tTime}, in)
	case r.name == "Date" && e.Name == "format":
		return sig(tString, fparam{name: "pattern", t: tString})
	}
	return nil, false
}

// ---------------------------------------------------------------- other expressions

func (x *fnCheck) field(e *ir.Field) typ {
	c := x.c
	recv := x.check(e.Value, nil)
	switch r := prune(recv).(type) {
	case *tentity:
		if e.Name == "all" {
			c.useName(e, GoText{File: "check/infer.go", Text: `e.Name == "all"`})
			x.top.effect("entity " + r.e.Name)
			return con("List", &tcon{name: r.e.Name, entity: r.e})
		}
	case *tcon:
		if r.name == "Ref" && r.decl == nil && len(r.args) == 1 {
			if s, ok := prune(r.args[0]).(*tcon); ok && s.entity != nil && e.Name != "id" {
				x.top.effect("entity " + s.entity.Name)
			}
		}
		if t, ok := c.fieldType(r, e.Name); ok {
			c.useField(e, r, e.Name)
			return t
		}
		if r.decl != nil || r.entity != nil || builtinTypes[r.name] {
			c.typeErrorf(e.Pos, "%s has no field %s", show(r), e.Name)
		}
	case *tdyn:
		c.typeErrorf(e.Pos, "%s has no field %s: a dyn value has only the methods of its trait", show(r), e.Name)
	}
	return unknown
}

func (x *fnCheck) index(e *ir.Index) typ {
	c := x.c
	coll := x.check(e.Value, nil)
	if _, isRange := e.Index.(*ir.Range); isRange {
		x.check(e.Index, con("Range", tInt))
		return coll
	}
	option := func(t typ) typ { return &tcon{name: "Option", decl: c.ts.option, args: []typ{t}} }
	switch r := prune(coll).(type) {
	case *tcon:
		if r.decl == nil {
			switch r.name {
			case "List":
				x.check(e.Index, tInt)
				return option(r.args[0])
			case "Map":
				x.check(e.Index, r.args[0])
				return option(r.args[1])
			case "Bytes":
				x.check(e.Index, tInt)
				return option(con("Byte"))
			}
		}
	}
	x.check(e.Index, nil)
	return option(c.fresh())
}

// placeType is the type of what an assignment changes.
func (x *fnCheck) placeType(p ir.Place) typ {
	var t typ
	shift := 0 // names before p.Path in the target as written
	if p.Self {
		t = x.self
		if x.entity != nil && len(p.Path) > 0 {
			x.c.usePlace(p.Pos, 0, Member{Of: x.entity, Name: p.Path[0]})
			shift = 1
			st, ok := x.c.stateType(x.entity, p.Path[0])
			if !ok {
				return unknown
			}
			t = st
			p.Path = p.Path[1:]
		}
	} else if p.Slot < len(x.slots) {
		t = x.slots[p.Slot]
	}
	if t == nil {
		return unknown
	}
	for i, name := range p.Path {
		if owner := x.c.memberOwner(t, name); owner != nil {
			x.c.usePlace(p.Pos, i+shift, Member{Of: owner, Name: name})
		}
		ft, ok := x.c.fieldType(t, name)
		if !ok {
			if r, isCon := prune(t).(*tcon); isCon && (r.decl != nil || r.entity != nil) {
				x.c.typeErrorf(p.Pos, "%s has no field %s", show(t), name)
			}
			return unknown
		}
		t = ft
	}
	return t
}

func (x *fnCheck) binary(e *ir.Binary) typ {
	switch e.Op {
	case "&&", "||":
		x.check(e.L, tBool)
		x.check(e.R, tBool)
		return tBool
	case "==", "!=", "<", "<=", ">", ">=":
		trait := "Ord"
		if e.Op == "==" || e.Op == "!=" {
			trait = "Eq"
		}
		// A record literal takes its type from the other side.
		if _, isRecord := e.L.(*ir.Record); isRecord {
			r := x.check(e.R, nil)
			x.check(e.L, r)
			x.requireNamed(r, trait, e.Pos, "comparing with "+e.Op)
			return tBool
		}
		l := x.check(e.L, nil)
		x.check(e.R, l)
		x.requireNamed(l, trait, e.Pos, "comparing with "+e.Op)
		return tBool
	}
	l := x.check(e.L, nil)
	r := x.check(e.R, nil)
	return x.arith(e.Op, l, r, e.Pos)
}

// arith is the type of `l op r` for + - * / %: the combinations the
// built-in types have (SPECS.md §11), and a declared type's own operator
// methods.
func (x *fnCheck) arith(op string, l, r typ, at ir.Pos) typ {
	c := x.c
	lt, rt := prune(l), prune(r)
	name := func(t typ) string {
		if k, ok := t.(*tcon); ok && k.decl == nil && k.entity == nil {
			return k.name
		}
		if v, ok := t.(*tvar); ok {
			switch v.lit {
			case litInt:
				return "Int"
			case litString:
				return "String"
			}
		}
		return ""
	}
	ln, rn := name(lt), name(rt)
	same := func(result typ) typ {
		if !x.expect(at, l, r) {
			return unknown
		}
		return result
	}
	if k, ok := lt.(*tcon); ok && (k.decl != nil || k.entity != nil) {
		// A declared type's operators are its impls of Add, Sub, … (§11).
		if rhs, out, found := x.opImpl(k, op, r); found {
			x.expect(at, rhs, r)
			return out
		}
		c.typeErrorf(at, "%s has no %s: implement %s for it, as `impl %s(%s, %s) for %s`", show(lt), op, opTrait(op), opTrait(op), show(lt), show(lt), show(lt))
		return unknown
	}
	switch {
	case ln == "Int" || rn == "Int" && ln == "":
		if rn == "Duration" || rn == "Size" {
			if op == "*" {
				return rt
			}
		}
		return same(l)
	case ln == "Float":
		return same(l)
	case ln == "String" && op == "+":
		return same(tString)
	case ln == "List" && op == "+":
		return same(l)
	case ln == "Duration":
		if op == "*" || op == "/" {
			x.expect(at, tInt, r)
			return tDuration
		}
		return same(tDuration)
	case ln == "Size":
		if op == "*" || op == "/" {
			x.expect(at, tInt, r)
			return tSize
		}
		return same(tSize)
	case ln == "Instant":
		if rn == "Instant" && op == "-" {
			return tDuration
		}
		x.expect(at, tDuration, r)
		return tInstant
	case ln == "Date":
		x.expect(at, tDuration, r)
		return tDate
	}
	switch v := lt.(type) {
	case *tvar:
		if v.rigid {
			// A function's own type variable has the operator its bounds give it.
			for _, b := range c.ts.bounds[v] {
				if b.trait.Name == opTrait(op) && b.trait.Module == c.prelude.mod && len(b.args) == 2 {
					x.expect(at, b.args[0], r)
					return b.args[1]
				}
			}
			c.typeErrorf(at, "%s %s … needs %s: %s, and %s may be any type: require it, as `where %s: %s(%s, %s)`", v.name, op, v.name, opTrait(op), v.name, v.name, opTrait(op), v.name, v.name)
			return unknown
		}
		if _, rvar := rt.(*tvar); !rvar && rn != "" {
			return same(r)
		}
		return same(l)
	case tunknown:
		return unknown
	}
	if _, ok := rt.(tunknown); ok {
		return unknown
	}
	c.typeErrorf(at, "%s %s %s isn't defined", show(lt), op, show(rt))
	return unknown
}

func opTrait(op string) string {
	return map[string]string{"+": "Add", "-": "Sub", "*": "Mul", "/": "Div", "%": "Rem"}[op]
}

func (x *fnCheck) match(e *ir.Match, want typ) typ {
	c := x.c
	scrutinee := x.check(e.Value, nil)
	result := want
	if result == nil {
		result = c.fresh()
	}
	for i := range e.Arms {
		arm := &e.Arms[i]
		arm.Pattern = x.bindPattern(arm.Pattern, scrutinee, e.Pos)
		if arm.Guard != nil {
			x.check(arm.Guard, tBool)
		}
		x.check(arm.Body, result)
	}
	x.top.matches = append(x.top.matches, pendingMatch{e, scrutinee})
	return result
}

// try checks `x?`: a Result's error passes to the caller, wrapped in the
// caller's error type when exactly one of its variants holds it, and an
// Option's None passes on as None.
func (x *fnCheck) try(e *ir.Try) typ {
	c := x.c
	t := prune(x.check(e.X, nil))
	ret, _ := prune(x.ret).(*tcon)
	k, ok := t.(*tcon)
	if !ok || k.decl == nil {
		if _, isVar := t.(*tvar); !isVar {
			if _, isUnknown := t.(tunknown); !isUnknown {
				c.typeErrorf(e.Pos, "? passes on the error of a Result or the None of an Option; this is %s", show(t))
			}
		}
		return c.fresh()
	}
	switch k.decl {
	case c.ts.option:
		if ret != nil && ret.decl != c.ts.option && ret.decl != nil {
			c.typeErrorf(e.Pos, "? on an Option needs a function that returns an Option, and this one returns %s", show(ret))
		}
		return k.args[0]
	case c.ts.result:
		if ret == nil {
			return k.args[0]
		}
		if ret.decl != c.ts.result {
			c.typeErrorf(e.Pos, "? on a Result needs a function that returns a Result, and this one returns %s", show(ret))
			return k.args[0]
		}
		if !x.converts(k.args[1], ret.args[1]) {
			c.typeErrorf(e.Pos, "? can't pass on a %s from a function whose error is %s: none of its variants holds a %s; use .mapErr(…)?",
				show(k.args[1]), show(ret.args[1]), show(k.args[1]))
		}
		return k.args[0]
	}
	c.typeErrorf(e.Pos, "? passes on the error of a Result or the None of an Option; this is %s", show(t))
	return c.fresh()
}

// converts reports whether an error of type from can become one of type to:
// it is one, or exactly one of to's variants holds it.
func (x *fnCheck) converts(from, to typ) bool {
	f, t := prune(from), prune(to)
	if _, ok := f.(*tvar); ok {
		return x.c.unify(to, from) == nil
	}
	if _, ok := t.(*tvar); ok {
		return x.c.unify(to, from) == nil
	}
	if equalTypes(f, t) {
		return true
	}
	tc, ok := t.(*tcon)
	if !ok || tc.decl == nil {
		return false
	}
	holders := 0
	for _, v := range tc.decl.Variants {
		if len(v.Fields) == 1 && equalTypes(prune(x.c.fromExpr(v.Fields[0].Type, envOf(tc), false)), f) {
			holders++
		}
	}
	return holders == 1
}

// equalTypes reports whether two types are the same without binding anything.
func equalTypes(a, b typ) bool {
	a, b = prune(a), prune(b)
	if a == b {
		return true
	}
	switch x := a.(type) {
	case *tcon:
		y, ok := b.(*tcon)
		if !ok || !sameCon(x, y) || len(x.args) != len(y.args) {
			return false
		}
		for i := range x.args {
			if !equalTypes(x.args[i], y.args[i]) {
				return false
			}
		}
		return true
	case *ttuple:
		y, ok := b.(*ttuple)
		if !ok || len(x.elems) != len(y.elems) {
			return false
		}
		for i := range x.elems {
			if !equalTypes(x.elems[i], y.elems[i]) {
				return false
			}
		}
		return true
	case tunknown:
		return true
	}
	_, unknownB := b.(tunknown)
	return unknownB
}

func (x *fnCheck) lambda(e *ir.Lambda, want typ) typ {
	c := x.c
	inner := c.newFnCheck(e.Func, kindLambda, x)
	for _, capt := range e.Func.Captures {
		if capt.Slot < len(inner.slots) {
			inner.slots[capt.Slot] = x.check(capt.From, nil)
		}
	}
	params := e.Func.Params
	f := &tfun{ret: c.fresh()}
	w, _ := prune(orNil(want)).(*tfun)
	switch {
	case w != nil && len(w.params) == len(params):
		for i, p := range params {
			inner.slots[p.Slot] = w.params[i].t
		}
		f.ret = w.ret
	case w != nil && len(w.params) == 1 && len(params) > 1:
		// A lambda with as many parameters as a tuple has elements takes
		// the tuple apart.
		elems := make([]typ, len(params))
		for i, p := range params {
			elems[i] = inner.slots[p.Slot]
		}
		x.expect(e.Pos, w.params[0].t, &ttuple{elems: elems})
		f.ret = w.ret
		f.params = []fparam{{t: w.params[0].t}}
		inner.check(e.Func.Body, f.ret)
		return f
	}
	for _, p := range params {
		f.params = append(f.params, fparam{name: p.Name, t: inner.slots[p.Slot]})
	}
	ret := f.ret
	if r, ok := prune(ret).(*tcon); ok && r.name == "()" && r.decl == nil {
		inner.check(e.Func.Body, discard)
	} else {
		inner.check(e.Func.Body, ret)
	}
	return f
}

// record checks a record literal, which takes the record type expected
// where it is. When nothing here says yet, the rest of the function may.
func (x *fnCheck) record(e *ir.Record, want typ) typ {
	c := x.c
	values := make([]typ, len(e.Values))
	pos := make([]ir.Pos, len(e.Values))
	if rec, ok := isRecord(orNil(want)); ok {
		for i, v := range e.Values {
			ft, has := c.fieldType(rec, e.Names[i])
			if !has {
				values[i] = x.check(v, nil)
			} else {
				values[i] = x.check(v, ft)
			}
			pos[i] = v.Position()
		}
		x.recordFields(rec, e.Names, values, pos, e.Pos)
		e.Type = rec.decl
		return rec
	}
	for i, v := range e.Values {
		values[i] = x.check(v, nil)
		pos[i] = v.Position()
	}
	switch w := prune(orNil(want)).(type) {
	case *tvar:
		if !w.rigid {
			x.top.records = append(x.top.records, pendingRecord{t: w, at: e.Pos, names: e.Names, values: values, pos: pos, e: e})
			return w
		}
	case tunknown:
		if want == nil || want == discard {
			v := c.fresh()
			x.top.records = append(x.top.records, pendingRecord{t: v, at: e.Pos, names: e.Names, values: values, pos: pos, e: e})
			return v
		}
		return unknown
	}
	c.typeErrorf(e.Pos, "expected %s, found a record", show(want))
	return unknown
}

// recordFields checks a record literal's fields against its type.
func (x *fnCheck) recordFields(rec *tcon, names []string, values []typ, pos []ir.Pos, at ir.Pos) {
	c := x.c
	given := map[string]bool{}
	for i, name := range names {
		given[name] = true
		ft, ok := c.fieldType(rec, name)
		if !ok {
			c.typeErrorf(pos[i], "%s has no field %s", show(rec), name)
			continue
		}
		x.expect(pos[i], ft, values[i])
	}
	var missing []string
	for _, f := range rec.decl.Fields { // the fields a literal must give
		if !given[f] {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		c.typeErrorf(at, "this %s has no %s: every field needs a value", show(rec), strings.Join(missing, ", "))
	}
}

// elemOf is the type of what `for` takes from a collection.
func (x *fnCheck) elemOf(t typ, at ir.Pos) typ {
	switch r := prune(t).(type) {
	case *tcon:
		if r.decl == nil && r.entity == nil {
			switch r.name {
			case "List", "Set", "Iter", "Range", "Receiver":
				return r.args[0]
			case "Map":
				return &ttuple{elems: []typ{r.args[0], r.args[1]}}
			case "String":
				return tChar
			case "Bytes":
				return con("Byte")
			}
		}
		x.c.typeErrorf(at, "%s isn't a collection that for can walk", show(t))
		return unknown
	case *tvar:
		if r.lit == litString {
			return tChar
		}
	}
	return x.c.fresh()
}

// ---------------------------------------------------------------- patterns

// bindPattern gives the names a pattern binds the types of what they match,
// and returns the pattern with each variant resolved in the matched type: a
// variant pattern names a variant of the value's type, whichever module
// declares another of the same name (SPECS.md §9).
func (x *fnCheck) bindPattern(p ir.Pattern, t typ, at ir.Pos) ir.Pattern {
	c := x.c
	switch p := p.(type) {
	case ir.PBind:
		if p.Slot < len(x.slots) {
			c.unify(x.slots[p.Slot], t)
			x.slots[p.Slot] = t
		}
		return p
	case ir.PWild, ir.PEmpty:
		return p
	case ir.PLit:
		x.expect(at, t, x.literal(&ir.Lit{At: ir.At{Pos: at}, Value: p.Value}, nil))
		return p
	case ir.PRange:
		x.expect(at, t, x.literal(&ir.Lit{At: ir.At{Pos: at}, Value: p.Lo}, nil))
		return p
	case ir.POr:
		for i := range p.Alts {
			p.Alts[i] = x.bindPattern(p.Alts[i], t, at)
		}
		return p
	case ir.PTuple:
		elems := make([]typ, len(p.Pats))
		for i := range elems {
			elems[i] = c.fresh()
		}
		x.expect(at, t, &ttuple{elems: elems})
		for i := range p.Pats {
			p.Pats[i] = x.bindPattern(p.Pats[i], elems[i], at)
		}
		return p
	case ir.PList:
		elem := typ(c.fresh())
		switch k := prune(t).(type) {
		case *tcon:
			if k.decl == nil && k.name == "Bytes" {
				elem = con("Byte")
				break
			}
			if k.decl == nil && k.name == "List" {
				elem = k.args[0]
				break
			}
			x.expect(at, t, con("List", elem))
		default:
			x.expect(at, t, con("List", elem))
		}
		for i := range p.Pats {
			p.Pats[i] = x.bindPattern(p.Pats[i], elem, at)
		}
		if p.RestPat != nil {
			p.RestPat = x.bindPattern(p.RestPat, prune(t), at)
		}
		return p
	case ir.PRecord:
		for i, name := range p.Names {
			ft, ok := c.fieldType(t, name)
			if !ok {
				if k, isCon := prune(t).(*tcon); isCon && (k.decl != nil || k.entity != nil) {
					c.typeErrorf(at, "%s has no field %s", show(t), name)
				}
				ft = unknown
			}
			p.Pats[i] = x.bindPattern(p.Pats[i], ft, at)
		}
		return p
	case ir.PVariant:
		return x.bindVariant(p, t, at)
	}
	return p
}

func (x *fnCheck) bindVariant(p ir.PVariant, t typ, at ir.Pos) ir.Pattern {
	c := x.c
	k, _ := prune(t).(*tcon)
	if k != nil && k.decl != nil {
		// The variant of the matched type with that name, whichever module
		// the name was found in.
		found := false
		for _, v := range k.decl.Variants {
			if v.Name == p.Variant.Name {
				p.Variant, found = v, true
			}
		}
		if !found && p.Variant.Name == "Uncertain" && k.uncertain {
			p.Variant, found = c.ts.uncert, true
		}
		if !found {
			c.typeErrorf(at, "%s isn't a variant of %s, the type of what is matched", p.Variant.Name, show(t))
			x.bindAnyArgs(&p, at)
			return p
		}
	}
	v := p.Variant
	if v.Type == nil {
		x.bindAnyArgs(&p, at)
		return p
	}
	var env typeEnv
	if k != nil && k.decl == v.Type {
		env = envOf(k)
	} else if v == c.ts.uncert {
		env = typeEnv{}
	} else {
		vt, e := c.declType(v.Type)
		env = e
		x.expect(at, t, vt)
	}
	next := 0
	for i, sub := range p.Args {
		ft := unknown
		if p.Names[i] != "" {
			ok := false
			for _, f := range v.Fields {
				if f.Name == p.Names[i] {
					ft, ok = c.fromExpr(f.Type, env, false), true
				}
			}
			if !ok {
				c.typeErrorf(at, "%s has no field %s", v.Name, p.Names[i])
			}
		} else {
			if next < len(v.Fields) {
				ft = c.fromExpr(v.Fields[next].Type, env, false)
			} else {
				c.typeErrorf(at, "%s holds %d values, and the pattern gives more", v.Name, len(v.Fields))
			}
			next++
		}
		p.Args[i] = x.bindPattern(sub, ft, at)
	}
	if !p.Rest && len(p.Args) < len(v.Fields) && len(p.Args) > 0 {
		c.typeErrorf(at, "%s holds %d values, and the pattern gives %d; add .. to skip the rest", v.Name, len(v.Fields), len(p.Args))
	}
	return p
}

func (x *fnCheck) bindAnyArgs(p *ir.PVariant, at ir.Pos) {
	for i := range p.Args {
		p.Args[i] = x.bindPattern(p.Args[i], unknown, at)
	}
}

// ---------------------------------------------------------------- effects as they are found

func (x *fnCheck) effect(e string) {
	set := x.c.ts.effects[x.fn]
	if set == nil {
		set = map[string]bool{}
		x.c.ts.effects[x.fn] = set
	}
	set[e] = true
}

func (x *fnCheck) callee(f *ir.Func) {
	x.c.ts.callees[x.fn] = append(x.c.ts.callees[x.fn], f)
}
