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
// What can't be typed is reported, never left quietly unknown (unknownf).

// typeState is what type checking keeps across functions.
type typeState struct {
	// badCalls are the calls whose arguments had errors, which say what
	// was expected, so their results aren't checked against what is wanted.
	badCalls map[ir.Expr]bool
	params   map[*ir.Func][]typ // inferred types of parameters a function leaves untyped
	results  map[*ir.Func]typ   // inferred results of functions that leave them out
	globals  map[*ir.Global]typ
	checking map[*ir.Global]bool
	effects  map[*ir.Func]map[string]bool // effects each function has itself
	callees  map[*ir.Func][]*ir.Func      // the functions each calls
	// The same for each lambda, whose effects also count in the function
	// it is written in, and the function types that limit function values
	// (fnfx.go).
	lambdaEffects map[*ir.Func]map[string]bool
	lambdaCallees map[*ir.Func][]*ir.Func
	fxLimits      []fxLimit
	// giving is the function whose call's arguments are being checked,
	// where what a function value given as a callback states counts.
	giving *fnCheck
	// sigs are each checked body's parameter and result types, and exprs
	// the expressions checked and the type each has, written into the IR
	// once every type is known (typedir.go).
	sigs  map[*ir.Func]*tfun
	exprs []typedExpr
	// saveSlots are the slot types of jobs and waiting functions, whose
	// frames a run saves, for the Save check.
	saveSlots map[*ir.Func][]typ
	steps     []stepResult    // a job's `!` calls and what they return, for the Save check
	forIters  map[*ir.For]typ // the type of what each for loop walks, for the Save check
	// inferred says how far a function whose types are inferred has got,
	// and schemes holds the generic type each one has once it is checked.
	inferred map[*ir.Func]inferState
	schemes  map[*ir.Func]*scheme
	active   []*ir.Func         // the inferred functions being checked, innermost last
	zones    []zoneLit          // string literals that may be zones
	ints     []intLit           // integer literals, which must fit a fixed-width type they become
	chars    []charLit          // character literals, which may become Bytes
	negated  map[*ir.Lit]ir.Pos // integer literals with a minus sign: `-5`, and where it starts
	option   *ir.Type
	result   *ir.Type
	outcome  *ir.Type
	decision *ir.Type
	uncert   *ir.Variant
	// stale and expired are what a @fresh command may answer besides its
	// declared variants; expired is approval!'s Expired.
	stale, expired *ir.Variant
	// outcomes are the types contract commands return, whose values a
	// statement may not drop.
	outcomes map[*ir.Type]bool
	// bounds holds the traits a function's own type variables have, from
	// its `where` clause.
	bounds map[*tvar][]cbound
	// The rows of entities' older versions, which migrations read, and the
	// types of their fields (migrations.go).
	rows  map[*ir.Entity]map[int]*tcon
	rowOf map[*ir.Type]*row
	// deferred are trait requirements on types no body had decided when
	// its function was checked, and ops are operators on such types.
	deferred []requirement
	ops      []pendingOp
	// calls are the record types of fakes.calls(op), one per operation,
	// and of fakes.calls(process.run).
	calls map[any]*ir.Type
	// tools are the functions named where an ai.Tool is expected (tools.go).
	tools []toolUse
	// unkeyed are the jobs each function starts without a key, which main
	// may not do, even through the functions it calls.
	unkeyed map[*ir.Func][]unkeyedStart
}

// unkeyedStart is a job started without a key, and the function that
// starts it.
type unkeyedStart struct {
	job *ir.Job
	in  *ir.Func
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
	ret     typ // what `return` returns: the outermost function's result, as lowering rejects `return` in a lambda
	res     typ // what `?` returns from: a lambda's own result; nil means ret
	self    typ
	entity  *ir.Entity
	env     typeEnv // the function's type variables
	outer   *fnCheck
	top     *fnCheck // the outermost function, for effects and pending checks
	waiting ir.Expr  // the call the innermost `!` marks
	calling ir.Expr  // the callee of the call being checked
	reading ir.Expr  // the view or E.all whose read() is being checked
	// durable says where the body runs in a run, which records its steps:
	// "in a job" or "in a waiting function", and "" elsewhere. It is set
	// on the outermost function.
	durable string
	callAt  *ir.Pos // where to report an operator of this generic body, for one call
	records []pendingRecord
	matches []pendingMatch
	lets    []pendingLet
	calls   []pendingCall
	reqs    []requirement
	tries   []pendingTry
	drops   []pendingDrop
	// variants are variant names several imported modules declare, which
	// wait for the type expected of them (chooseVariant).
	variants []pendingVariant
}

// pendingVariant is a variant name written alone that more than one
// imported module declares, whose expected type wasn't known where it was
// checked.
type pendingVariant struct {
	e       *ir.Ref
	choices []*ir.Variant
	t       *tvar
}

// pendingDrop is the value of a statement, which nothing uses: a Result or
// a command's outcome may not be dropped (SPECS.md §10).
type pendingDrop struct {
	at ir.Pos
	t  typ
}

// pendingTry is a `?` whose function's result wasn't known where it was
// checked, an inferred function's or a lambda's, or whose operand's type
// wasn't.
type pendingTry struct {
	x   *fnCheck // the function `?` returns from
	e   *ir.Try
	k   *tcon // the Option or Result it is applied to; nil while op is unknown
	op  typ   // the operand's type, when k is nil
	val typ   // what `?` gave the code after it, when k is nil
	res typ
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

func (ts *typeState) badCall(e ir.Expr) {
	if ts.badCalls == nil {
		ts.badCalls = map[ir.Expr]bool{}
	}
	ts.badCalls[e] = true
}

// checkTypes checks every body of the program.
func (c *checker) checkTypes() {
	pre := c.prelude.mod
	c.ts = &typeState{
		params: map[*ir.Func][]typ{}, results: map[*ir.Func]typ{}, globals: map[*ir.Global]typ{},
		checking: map[*ir.Global]bool{}, effects: map[*ir.Func]map[string]bool{},
		callees: map[*ir.Func][]*ir.Func{}, saveSlots: map[*ir.Func][]typ{}, forIters: map[*ir.For]typ{},
		lambdaEffects: map[*ir.Func]map[string]bool{}, lambdaCallees: map[*ir.Func][]*ir.Func{},
		sigs: map[*ir.Func]*tfun{}, unkeyed: map[*ir.Func][]unkeyedStart{},
		inferred: map[*ir.Func]inferState{}, schemes: map[*ir.Func]*scheme{},
		option: pre.Types["Option"], result: pre.Types["Result"], outcome: pre.Types["Outcome"],
		decision: pre.Types["Decision"], uncert: pre.Variants["Uncertain"],
		stale: pre.Variants["Stale"], expired: pre.Variants["Expired"],
		bounds: map[*tvar][]cbound{},
		rows:   map[*ir.Entity]map[int]*tcon{}, rowOf: map[*ir.Type]*row{},
		outcomes: map[*ir.Type]bool{},
	}
	for _, m := range c.prog.Modules {
		for _, ct := range m.Contracts {
			for _, op := range ct.Ops {
				// A variant type, as a command's result is; an Option stays
				// free to drop.
				if op.Command && op.Return != nil && op.Return.Decl != nil && len(op.Return.Decl.Variants) > 0 && op.Return.Decl != c.ts.option {
					c.ts.outcomes[op.Return.Decl] = true
				}
			}
		}
	}
	c.fillImplMethods()
	c.checkDyns()
	c.checkFieldMethods()
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
				// A migration takes the row as it was stored at its version.
				c.ts.params[mg.Func] = []typ{c.rowType(e, mg.From)}
				bodies = append(bodies, body{fn: mg.Func, kind: kindGlobal, want: c.migrationResult(e, mg)})
			}
		}
		for _, j := range m.Jobs {
			bodies = append(bodies, body{fn: j.Func, kind: kindJob})
			for _, t := range j.Triggers {
				c.triggeredParams(j, t)
				if t.Event != nil {
					// A run started by an event reads it off its queue.
					addEffect(c.ts.effects, j.Func, "consume "+t.Event.Contract.Name+"."+t.Event.Name)
				}
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
		switch {
		case c.ts.inferred[b.fn] != notInferred:
			// checked already, where a call needed its type
		case c.inferrable(b.fn):
			c.inferFunc(b.fn)
		default:
			c.checkBody(b)
		}
	}
	c.checkZones()
	c.settleOps()
	c.settleDeferred()
	c.settleLiterals()
	c.checkRowReads()
	c.checkEffects()
	c.checkExterns()
	c.writeTypes()
	c.checkWitnessed()
}

// body is a function body to check, with what it is checked in.
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

// checkBody checks one body against its signature.
func (c *checker) checkBody(b body) {
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
	sig := &tfun{ret: ret}
	if b.fn.HasSelf && x.self != nil {
		sig.params = append(sig.params, fparam{name: "self", t: x.self})
	}
	for _, p := range b.fn.Params {
		sig.params = append(sig.params, fparam{name: p.Name, t: x.slots[p.Slot]})
	}
	c.ts.sigs[b.fn] = sig
	x.body(ret)
	if x.durable != "" {
		c.ts.saveSlots[b.fn] = x.slots
	}
}

func (c *checker) newFnCheck(fn *ir.Func, kind fnKind, outer *fnCheck) *fnCheck {
	x := &fnCheck{c: c, fn: fn, kind: kind, slots: make([]typ, fn.Slots), env: typeEnv{}, outer: outer}
	x.top = x
	switch {
	case kind == kindJob:
		x.durable = "in a job"
	case kind == kindPlain && hasSteps(fn) && (fn.Module == nil || fn.Module.Main != fn):
		// A waiting function runs only in a run, which replays its steps,
		// so it follows a job's rules (SPECS.md §12).
		x.durable = "in a waiting function"
	}
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
	return freshResult(op, x.c.fromExpr(op.Return, x.env, true))
}

// paramType is the type of a function's parameter i, in env. A parameter
// written without a type has one inferred type for the whole program.
func (c *checker) paramType(fn *ir.Func, i int, env typeEnv, rigid bool) typ {
	p := fn.Params[i]
	if p.Type != nil {
		t := c.fromExpr(p.Type, env, rigid)
		if !p.Mut {
			markCallbacks(t, p.Name)
		}
		return t
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
	if c.ts != nil && c.inferrable(fn) {
		if c.ts.inferred[fn] == notInferred {
			c.inferFunc(fn)
		}
		if s := c.ts.schemes[fn]; s != nil {
			return c.instantiate(fn, s)
		}
		// Still being checked, as when it calls itself: one type until then.
	}
	if env == nil {
		env = typeEnv{}
	}
	t := &tfun{}
	for i, p := range fn.Params {
		t.params = append(t.params, fparam{name: p.Name, t: c.paramType(fn, i, env, false), hasDefault: p.Default != nil, mut: p.Mut})
		if fn.Destructive && i == len(fn.Params)-1 {
			t.params[i].why = " is @destructive, so it takes the Approval that approval!(…) gives"
		}
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
	// A record settled here gives its fields their types, which may settle
	// a record literal inside it, as `{ name: u.name, home: { city } }`,
	// so the literals are settled until none is left whose type is known.
	pending := x.records
	for settled := true; settled; {
		settled = false
		var open []pendingRecord
		for _, r := range pending {
			if _, isVar := prune(r.t).(*tvar); isVar {
				open = append(open, r)
				continue
			}
			settled = true
			rec, ok := isRecord(r.t)
			if !ok {
				c.typeErrorf(r.at, "expected %s, found a record", show(r.t))
				continue
			}
			if d, ok := c.opaqueTo(x.fn.Module, rec); ok {
				c.typeErrorf(r.at, "%s", c.opaqueMessage(d, "fields"))
			}
			x.recordFields(rec, r.names, r.values, r.pos, r.at)
			if r.e != nil {
				r.e.Type = rec.decl
			}
		}
		pending = open
	}
	for _, r := range pending {
		c.typeErrorf(r.at, "which record type is this? Nothing here says; write it where the value is bound, as `let x: T = …`")
	}
	for _, p := range x.variants {
		v := pickVariant(p.choices, p.t)
		if v == nil {
			c.typeErrorf(p.e.Pos, "%s", ambiguousVariant(p.choices))
			v = p.choices[0]
		}
		x.settleVariant(p.e, v)
		x.expect(p.e.Pos, p.t, x.ref(p.e))
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
		if asked, ok := c.generated(call); ok {
			free := map[*tvar]bool{}
			freeVars(defaultLiteral(asked), free)
			if len(free) > 0 {
				c.typeErrorf(call.call.Pos, "ai.generate needs to know the type to ask for, and nothing here says: write `let x: T = ai.generate!(…)`, or return it from a function that says its type")
			}
		}
	}
	for _, t := range x.tries {
		if t.k == nil {
			t.settle()
		} else {
			t.x.tryInto(t.e, t.k, t.res)
		}
	}
	x.settleRequirements()
	for _, d := range x.drops {
		c.checkDrop(d)
	}
}

// checkDrop reports a statement's value that may not be dropped: a Result,
// or a command's outcome (SPECS.md §10 and §17).
func (c *checker) checkDrop(d pendingDrop) {
	k, ok := prune(d.t).(*tcon)
	if !ok || k.decl == nil {
		return
	}
	switch {
	case k.decl == c.ts.result:
		c.typeErrorf(d.at, "this drops a %s, so a failure would go unnoticed: use it, match it, pass its error on with ?, or discard it with let _ = …", show(k))
	case k.uncertain || k.fresh || c.ts.outcomes[k.decl]:
		may := ""
		switch {
		case k.fresh:
			may = ", which may be Uncertain, Stale or Expired"
		case k.uncertain:
			may = ", which may be Uncertain"
		}
		c.typeErrorf(d.at, "this drops %s, a command's outcome%s, so a refusal would go unnoticed: match it, or discard it with let _ = …", show(k), may)
	}
}

// ---------------------------------------------------------------- expressions

// check infers e's type and makes it want, when want is given.
func (x *fnCheck) check(e ir.Expr, want typ) typ {
	if t := x.tool(e, want); t != nil {
		return t
	}
	got := x.infer(e, want)
	x.c.typed(e.Position(), got)
	x.c.ts.exprs = append(x.c.ts.exprs, typedExpr{e, got})
	if want == discard {
		// A block passes discard on to its last expression, which is
		// where its value comes from, and what gives no value, as an
		// endless loop, drops nothing.
		v, isVar := prune(got).(*tvar)
		if _, isBlock := e.(*ir.Block); !isBlock && (!isVar || !v.never) {
			x.top.drops = append(x.top.drops, pendingDrop{at: e.Position(), t: got})
		}
		return got
	}
	if want == nil {
		return got
	}
	if _, isDiscard := got.(tdiscard); isDiscard {
		return want
	}
	if x.c.ts.badCalls[e] {
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
	if w, ok := prune(want).(*tcon); ok && !w.fresh {
		if g, ok := prune(got).(*tcon); ok && g.fresh && sameCon(w, g) {
			x.c.typeErrorf(at, "this command is @fresh, so it may be Stale or Expired: match it with Stale and Expired arms")
			return false
		}
	}
	if g, ok := prune(got).(*tcon); ok && x.c.ts.stale != nil && g.decl == x.c.ts.stale.Type {
		if w, ok := prune(want).(*tcon); ok && w.decl != g.decl {
			x.c.typeErrorf(at, "expected %s, found Stale, which only a @fresh command answers", show(want))
			return false
		}
	}
	if _, isFn := prune(got).(*tfun); isFn && isTool(want) {
		x.c.typeErrorf(at, "a tool is a function declared with fn and named here, as tools: [searchIssues], since its name, its ## comment and its parameters' types describe it to the model; a lambda or a function passed in has none of them")
		return false
	}
	if m := x.c.unify(want, got); m != nil {
		msg := fmt.Sprintf("expected %s, found %s", show(defaultLiteral(want)), show(defaultLiteral(got)))
		if m.dyn != nil {
			msg += fmt.Sprintf(": %s doesn't implement %s", show(m.lacks), m.dyn.trait.Name)
		}
		if m.why != "" {
			msg += ": " + m.why
		}
		msg += x.c.offsetHint(want, got)
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
		// An empty map, set or record, as the type expected says.
		if rec, ok := isRecord(orNil(want)); ok && len(rec.decl.Fields) > 0 {
			x.recordFields(rec, nil, nil, nil, e.Pos)
			return rec
		}
		return c.freshLit(litBraces)
	case *ir.Local:
		if e.Slot < len(x.slots) {
			return x.slots[e.Slot]
		}
		return c.unknownf(e.Pos, "internal error: %s has no slot", e.Name)
	case *ir.Self:
		if x.self == nil {
			return c.unknownf(e.Pos, "internal error: self has no type here")
		}
		return x.self
	case *ir.SelfField:
		if x.entity != nil {
			if t, ok := c.stateType(x.entity, e.Name); ok {
				return t
			}
		}
		return c.unknownf(e.Pos, "internal error: %s isn't a state field here", e.Name)
	case *ir.Ref:
		if choices, ok := c.variantChoices[e]; ok {
			return x.chooseVariant(e, choices, want)
		}
		if t := x.freshOutcome(e, want); t != nil {
			return t
		}
		return x.ref(e)
	case *ir.Call:
		return x.call(e, want)
	case *ir.MethodCall:
		return x.methodCall(e, want)
	case *ir.Waiting:
		x.effect("wait")
		prev := x.waiting
		x.waiting = e.Call
		t := x.check(e.Call, want)
		x.waiting = prev
		if x.top.durable != "" && x.kind != kindLambda {
			c.ts.steps = append(c.ts.steps, stepResult{job: x.top.fn, w: e, t: t})
		}
		return t
	case *ir.Field:
		return x.field(e)
	case *ir.Index:
		return x.index(e)
	case *ir.Unary:
		if lit, ok := e.X.(*ir.Lit); ok && e.Op == "-" {
			if _, isInt := lit.Value.(*big.Int); isInt {
				if c.ts.negated == nil {
					c.ts.negated = map[*ir.Lit]ir.Pos{}
				}
				c.ts.negated[lit] = e.Pos
			}
		}
		t := x.check(e.X, nil)
		if e.Op == "!" {
			x.expect(e.X.Position(), tBool, t)
			return tBool
		}
		return x.negate(t, e)
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
			if !diverges(e.Else) {
				c.typeErrorf(e.Else.Position(), "the else block of a let must leave, since the names the pattern binds have no value there: end it with return, break, continue or unreachable(…)")
			}
		}
		e.Pattern = x.bindPattern(e.Pattern, t, e.Pos)
		x.top.lets = append(x.top.lets, pendingLet{l: e, t: t})
		return tUnit
	case *ir.Assign:
		target := x.placeType(e.Target, e.Op)
		if e.Op == "=" {
			x.check(e.Value, target)
			return tUnit
		}
		value := x.check(e.Value, nil)
		result := x.arith(strings.TrimSuffix(e.Op, "="), target, value, e.Value, e.Pos, e)
		x.expect(e.Pos, target, result)
		return tUnit
	case *ir.For:
		walked := x.check(e.Iter, nil)
		c.ts.forIters[e] = walked
		elem := x.elemOf(walked, e.Iter.Position())
		e.Pattern = x.bindPattern(e.Pattern, elem, e.Pos)
		x.check(e.Body, discard)
		return tUnit
	case *ir.While:
		x.check(e.Cond, tBool)
		x.check(e.Body, discard)
		if endless(e) {
			// `while true` without `break` never ends but by `return`, so
			// its value can be of any type, as code after it never runs.
			return c.never()
		}
		return tUnit
	case *ir.Break, *ir.Continue:
		return c.never()
	case *ir.Return:
		if e.Value != nil {
			x.check(e.Value, x.ret)
		}
		return c.never()
	case *ir.Lambda:
		return x.lambda(e, want)
	case *ir.Record:
		return x.record(e, want)
	case *ir.RecordUpdate:
		base := x.check(e.Base, want)
		rec, ok := isRecord(base)
		if ok {
			c.useFields(e, rec, e.Names)
			if d, opaque := c.opaqueTo(x.top.fn.Module, rec); opaque {
				c.typeErrorf(e.Base.Position(), "%s", c.opaqueMessage(d, "fields"))
			}
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
		// clock(…), or fakes(…), which is a Handler with calls of its own.
		t := x.check(e.Value, nil)
		if k, ok := prune(t).(*tcon); !ok || k.decl != nil || k.name != "Fakes" {
			x.expect(e.Value.Position(), tHandler, t)
		}
		return tUnit
	}
	return c.unknownf(e.Position(), "internal error: the checker doesn't know the expression %T", e)
}

// triggeredParams checks a job's parameters against what its trigger passes
// it (SPECS.md §15): a schedule, the date of the firing if the job takes
// one; enters or leaves, the Ref of the entity that crossed; an event, the
// event.
func (c *checker) triggeredParams(j *ir.Job, t ir.Trigger) {
	var by, what string
	var want typ
	switch {
	case t.Schedule != nil:
		by, what, want = "a schedule", "the date of the firing", con("Date")
	case t.View != nil:
		e := t.View.Entity
		if e == nil {
			return
		}
		by, what = t.Kind+"("+t.View.Name+")", "the entity that "+map[string]string{"enters": "entered", "leaves": "left"}[t.Kind]
		want = con("Ref", &tcon{name: e.Name, entity: e})
	case t.Event != nil:
		if t.Event.Type == nil || t.Event.Contract == nil {
			return
		}
		owner := t.Event.Contract.Name
		if m := t.Event.Contract.Module; m != nil && m.Path != "" {
			owner = m.Name // written as the module it is imported as
		}
		by, what = "event("+owner+"."+t.Event.Name+")", "the event"
		want = c.fromExpr(t.Event.Type, typeEnv{}, false)
	default:
		return
	}
	ps := j.Func.Params
	switch {
	case len(ps) == 0 && t.Schedule != nil:
		return
	case len(ps) != 1:
		takes := "one parameter, " + articled([]string{show(want)})[0]
		if t.Schedule != nil {
			takes = "no parameters, or one Date"
		}
		at := j.Pos
		if len(ps) > 1 && ps[1].Type != nil {
			if p, ok := c.typePos[ps[1].Type]; ok {
				at = p // the first parameter too many
			}
		}
		c.typeErrorf(at, "%s passes %s %s, so it takes %s; it takes %d", by, j.Name, what, takes, len(ps))
		return
	}
	at := j.Pos
	if ps[0].Type != nil {
		if p, ok := c.typePos[ps[0].Type]; ok {
			at = p
		}
	}
	got := c.paramType(j.Func, 0, typeEnv{}, false)
	if c.unify(want, got) != nil {
		c.typeErrorf(at, "%s passes %s %s, %s, and %s is %s", by, j.Name, what, articled([]string{show(want)})[0], ps[0].Name, articled([]string{show(got)})[0])
	}
}

// diverges reports whether e never finishes with a value: it returns,
// breaks, continues, stops with unreachable or loops forever, or every way
// through it does.
func diverges(e ir.Expr) bool {
	switch e := e.(type) {
	case *ir.Return, *ir.Break, *ir.Continue:
		return true
	case *ir.Block:
		for _, s := range e.Exprs {
			if diverges(s) {
				return true
			}
		}
	case *ir.If:
		return e.Else != nil && diverges(e.Then) && diverges(e.Else)
	case *ir.Match:
		for _, a := range e.Arms {
			if !diverges(a.Body) {
				return false
			}
		}
		return len(e.Arms) > 0
	case *ir.Call:
		if r, ok := e.Func.(*ir.Ref); ok {
			if b, ok := r.Decl.(ir.Builtin); ok && b.Name == "unreachable" {
				return true
			}
		}
	case *ir.While:
		return endless(e)
	}
	return false
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

// orNil is what is expected, or discard when nothing is: a type that is
// none of the shapes callers look for.
func orNil(t typ) typ {
	if t == nil {
		return discard
	}
	return t
}

func (x *fnCheck) literal(e *ir.Lit, want typ) typ {
	switch v := e.Value.(type) {
	case *big.Int:
		k := litInt
		if v.Sign() == 0 {
			k = litZero
		}
		t := x.c.freshLit(k)
		x.c.ts.ints = append(x.c.ts.ints, intLit{lit: e, t: t})
		return t
	case float64:
		return tFloat
	case string:
		t := x.c.freshLit(litString)
		x.c.ts.zones = append(x.c.ts.zones, zoneLit{t: t, name: v, pos: e.Pos})
		return t
	case bool:
		return tBool
	case rune:
		// A Char, or a Byte where one is expected (SPECS.md §5).
		t := x.c.freshLit(litChar)
		x.c.ts.chars = append(x.c.ts.chars, charLit{lit: e, t: t})
		return t
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
	if _, ok := e.Value.(brokenRef); ok {
		// Its declaration's syntax error is reported. Any type, as the
		// first use decides, so that one more use is still checked.
		return x.c.never()
	}
	return x.c.unknownf(e.Pos, "internal error: the checker doesn't know the literal %T", e.Value)
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

// chooseVariant types a variant name written alone that several imported
// modules declare: the type expected where it is written says which it is,
// as the matched value's type does in a pattern (SPECS.md §13). When that
// type isn't known yet, the name waits for the end of the body.
func (x *fnCheck) chooseVariant(e *ir.Ref, choices []*ir.Variant, want typ) typ {
	if v := pickVariant(choices, want); v != nil {
		x.settleVariant(e, v)
		return x.ref(e)
	}
	t := x.c.fresh()
	x.top.variants = append(x.top.variants, pendingVariant{e, choices, t})
	return t
}

// pickVariant is the one of choices whose type want is, or nil.
func pickVariant(choices []*ir.Variant, want typ) *ir.Variant {
	if want == nil || want == discard {
		return nil
	}
	w, ok := prune(want).(*tcon)
	if !ok || w.decl == nil {
		return nil
	}
	for _, v := range choices {
		if v.Type == w.decl {
			return v
		}
	}
	return nil
}

// settleVariant makes e name v, the variant its expected type chose.
func (x *fnCheck) settleVariant(e *ir.Ref, v *ir.Variant) {
	e.Decl = v
	delete(x.c.variantChoices, e)
	x.c.use(e.Pos, v)
}

// ambiguousVariant says that a variant name is several imported modules',
// and nothing where it is written says which.
func ambiguousVariant(choices []*ir.Variant) string {
	v := choices[0]
	var mods []string
	for _, c := range choices {
		mods = append(mods, c.Type.Module.Name+"."+c.Type.Name)
	}
	return fmt.Sprintf("%s is a variant of %s, and nothing here says which: write it where its type is expected, or with its module, as %s.%s",
		v.Name, strings.Join(mods, " and "), v.Type.Module.Name, v.Name)
}

func (x *fnCheck) ref(e *ir.Ref) typ {
	c := x.c
	switch d := e.Decl.(type) {
	case *ir.Func:
		// A function named as a value may be called anywhere it goes, so
		// its effects count where it is named, as a lambda's do where it is
		// written (SPECS.md §12): `xs |> map(stamp)` uses what stamp uses.
		x.callee(d)
		if d.Extern && d.HasUses {
			for _, u := range d.Uses {
				x.effect(u)
			}
		}
		f := c.funcType(d, nil)
		if x.calling != ir.Expr(e) {
			// Named as a value: it is called wherever it goes, with the
			// types it is used at here.
			x.useGeneric(f, d.Name, e.Pos)
			f.fx = newFx(fxSrc{fn: d, name: d.Name, at: e.Pos})
		}
		return f
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
			x.effect("entity " + d.Entity.Name)
			x.unrecordedSet(e, d.Name)
			return con("List", &tcon{name: d.Entity.Name, entity: d.Entity})
		}
		return c.unknownf(e.Pos, "internal error: view %s has no entity", d.Name)
	case *ir.Op:
		effect := "net " + d.Contract.Name + "." + d.Name
		x.effect(effect)
		t := c.opType(d)
		if f, ok := t.(*tfun); ok && x.calling != ir.Expr(e) {
			f.fx = newFx(fxSrc{fixed: []string{effect}, name: d.Name, at: e.Pos})
		}
		return t
	case *ir.Event:
		return con("Event", c.fromExpr(d.Type, typeEnv{}, false))
	case *ir.Module:
		return tmodule{}
	case ir.EnumFn:
		// Derived on a type whose variants have no fields (SPECS.md §11).
		t, _ := c.declType(d.Type)
		if d.Name == "variants" {
			return &tfun{ret: con("List", t)}
		}
		return &tfun{params: []fparam{{name: "name", t: tString}}, ret: &tcon{name: "Option", decl: c.ts.option, args: []typ{t}}}
	case ir.Builtin:
		if fn := c.builtins.funcs[d.Name]; fn != nil {
			x.builtinEffects(d.Name, nil) // named as a value: the bare kind
			f := c.funcType(fn, nil)
			if x.calling != ir.Expr(e) {
				var fixed []string
				for _, u := range c.builtins.uses[d.Name] {
					fixed = append(fixed, namedEffect(u, nil))
				}
				f.fx = newFx(fxSrc{fixed: fixed, name: d.Name, at: e.Pos})
			}
			return f
		}
		return c.unknownf(e.Pos, "%s can't be passed by name yet, as it takes more than one kind of argument: pass a lambda that calls it, as fn x: %s(x)", d.Name, d.Name)
	}
	return c.unknownf(e.Pos, "%s isn't a value", e.Pos.Text())
}

// variantType is a variant as a value: the value, for one without fields,
// or its constructor.
func (c *checker) variantType(v *ir.Variant) typ {
	if v.Type == nil {
		return c.unknownf(ir.Pos{}, "internal error: variant %s has no type", v.Name)
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
// Uncertain, a @fresh one Stale or Expired, and a command with @key takes the
// key last.
func (c *checker) opType(op *ir.Op) typ {
	env := typeEnv{}
	f := &tfun{}
	for _, p := range op.Params {
		f.params = append(f.params, fparam{name: p.Name, t: c.fromExpr(p.Type, env, false), hasDefault: p.Default != nil, why: addedWhy(op, p)})
	}
	if op.Key {
		f.params = append(f.params, fparam{name: "key", t: tKey})
	}
	ret := c.fromExpr(op.Return, env, false)
	if op.Command {
		if r, ok := ret.(*tcon); ok && r.decl != nil {
			cp := *r
			cp.uncertain, cp.reconcilable, cp.fresh = true, op.Reconcile != "", op.Fresh
			ret = &cp
		}
	}
	f.ret = ret
	return f
}

// addedWhy says why a call needs a parameter that @fresh or @destructive
// added to a command, after the command's name.
func addedWhy(op *ir.Op, p *ir.Param) string {
	switch {
	case op.Fresh && p.Name == "basedOn":
		return " is @fresh, so it takes the Version of the state it was based on, as basedOn: snapshot.version"
	case op.Destructive && p.Name == "approval":
		return " is @destructive, so it takes the Approval that approval!(…) gives"
	}
	return ""
}

// freshVariant is Stale or Expired, by name, when k is a @fresh command's
// result, which may be either besides its declared variants.
func (c *checker) freshVariant(name string, k *tcon) *ir.Variant {
	if k == nil || !k.fresh {
		return nil
	}
	switch name {
	case "Stale":
		return c.ts.stale
	case "Expired":
		return c.ts.expired
	}
	return nil
}

// freshOutcome types Stale or Expired where a @fresh command's result is
// expected, as in the impl that serves the command: it is that result. The
// reference is pointed at the prelude's variant, whichever one its name
// found.
func (x *fnCheck) freshOutcome(e *ir.Ref, want typ) typ {
	d, ok := e.Decl.(*ir.Variant)
	if !ok || want == nil {
		return nil
	}
	k, _ := prune(want).(*tcon)
	v := x.c.freshVariant(d.Name, k)
	if v == nil {
		return nil
	}
	e.Decl = v
	return k
}

// freshResult returns t with Stale and Expired among its outcomes, when it
// is a @fresh command's result type, so that an impl of the command can
// answer them.
func freshResult(op *ir.Op, t typ) typ {
	if r, ok := t.(*tcon); ok && op.Fresh && r.decl != nil && len(r.decl.Variants) > 0 {
		cp := *r
		cp.fresh = true
		return &cp
	}
	return t
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
		if choices, ok := c.variantChoices[r]; ok {
			// A variant with fields, called: the type expected of the
			// call says which module's it is.
			v := pickVariant(choices, want)
			if v == nil {
				c.typeErrorf(r.Pos, "%s", ambiguousVariant(choices))
				v = choices[0]
			}
			x.settleVariant(r, v)
		}
		c.useArgs(e, r.Decl)
		switch d := r.Decl.(type) {
		case ir.Builtin:
			name := d.Name
			if w, ok := collectionWaits[name]; ok && x.waiting == ir.Expr(e) {
				// all!(runs) and first!(runs): ! never changes which
				// function a name means (SPECS.md §15), so these are
				// errors, checked as the waits they meant.
				c.typeErrorf(e.Pos, "%s! isn't a wait: %s, write %s!(…); %s is the collection function, written without !", name, w.what, w.wait, name)
				name = w.wait
			}
			t := x.builtin(name, e.Args, want, e.Pos)
			if v, ok := prune(t).(*tvar); ok && d.Name == "unreachable" {
				v.never = true // it stops the program, so it gives no value
			}
			x.top.calls = append(x.top.calls, pendingCall{e, t})
			return t
		case *ir.Entity:
			var id typ
			if d.ID != nil {
				id = c.fromExpr(d.ID.Type, typeEnv{}, false)
			} else {
				id = c.unknownf(e.Pos, "internal error: entity %s has no identity", d.Name)
			}
			x.args(&tfun{params: []fparam{{name: "id", t: id}}, ret: tUnit}, e.Args, d.Name, e.Pos)
			return con("Ref", &tcon{name: d.Name, entity: d})
		case *ir.Op:
			x.effect("net " + d.Contract.Name + "." + d.Name)
		case *ir.Func:
			x.callee(d)
			if d.Extern && d.HasUses {
				for _, u := range d.Uses {
					x.effect(u)
				}
			}
		}
	}
	prevCalling := x.calling
	x.calling = e.Func // called here, so the call applies its bounds
	callee := x.check(e.Func, nil)
	x.calling = prevCalling
	before := len(c.diags)
	t := x.apply(callee, e.Args, want, calleeName(e.Func), e.Pos)
	if len(c.diags) > before {
		// What its arguments' errors say was expected; its result
		// meeting what is wanted of it would only say it again.
		c.ts.badCall(e)
	}
	x.top.calls = append(x.top.calls, pendingCall{e, t})
	if r, ok := e.Func.(*ir.Ref); ok {
		if fn, ok := r.Decl.(*ir.Func); ok && fn.Host == "ai.generate" {
			// A model is asked for a value by its type's JSON Schema (§19).
			if res, ok := prune(t).(*tcon); ok && res.decl == c.ts.result && len(res.args) == 2 {
				x.requireNamed(res.args[0], "Schema", e.Pos, "ai.generate")
			}
			c.checkToolApprovals(e)
		}
	}
	return t
}

// generated is the type an ai.generate call asks a model for, if it is one.
func (c *checker) generated(call pendingCall) (typ, bool) {
	r, ok := call.call.Func.(*ir.Ref)
	if !ok {
		return nil, false
	}
	if fn, ok := r.Decl.(*ir.Func); !ok || fn.Host != "ai.generate" {
		return nil, false
	}
	if res, ok := prune(call.t).(*tcon); ok && res.decl == c.ts.result && len(res.args) == 2 {
		return res.args[0], true
	}
	return nil, false
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
		case ir.EnumFn:
			return d.Type.Name + "." + d.Name
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
		for _, u := range f.uses {
			x.effect(u) // what the function's type says it may do
		}
		x.argsWant(f, args, name, at, want)
		return f.ret
	case *tvar:
		if f.rigid {
			x.inferArgs(args)
			return x.c.unknownf(at, "%s's type is %s, which may be any type, so it can't be called: give it a function type, as fn(…) -> …", name, f.name)
		}
		// An untyped parameter called as a function: whatever is passed in
		// is a callback.
		fn := &tfun{ret: x.c.fresh(), callback: name}
		for _, a := range args {
			fn.params = append(fn.params, fparam{t: x.check(a.Value, nil)})
		}
		x.c.unify(f, fn)
		return fn.ret
	case tunknown:
		x.inferArgs(args)
		return f
	}
	x.inferArgs(args)
	return x.c.unknownf(at, "%s isn't a function; it is %s", name, show(callee))
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
	x.argsWant(f, args, name, at, nil)
}

// argsWant is args for a call expected to give want: before the lambdas are
// checked, the call's result meets want, so that a lambda's body knows the
// type it returns (settleEarly).
func (x *fnCheck) argsWant(f *tfun, args []ir.Arg, name string, at ir.Pos, want typ) {
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
		if pass == 1 {
			x.settleEarly(f.ret, want)
		}
		for i, a := range args {
			_, isLambda := a.Value.(*ir.Lambda)
			if isLambda != (pass == 1) {
				continue
			}
			if target[i] < 0 {
				x.check(a.Value, nil)
				continue
			}
			x.giving(func() { x.check(a.Value, f.params[target[i]].t) })
		}
	}
	for j, p := range f.params {
		if !assigned[j] && !p.hasDefault {
			why := ""
			if p.why != "" {
				why = ": " + name + p.why
			}
			x.c.typeErrorf(at, "%s needs an argument for %s%s", name, paramName(p, j), why)
		}
	}
	x.useGeneric(f, name, at)
	x.mutArgs(f, args, target, name)
}

// giving runs check, which checks a call's argument: a function value given
// there as a callback counts what its type states here (fnfx.go).
func (x *fnCheck) giving(check func()) {
	outer := x.c.ts.giving
	x.c.ts.giving = x
	defer func() { x.c.ts.giving = outer }()
	check()
}

// useGeneric records what a use of a generic function asks of the types it
// is used at: the traits it requires of its type variables, and for an
// inferred one the operators its body applies to them.
func (x *fnCheck) useGeneric(f *tfun, name string, at ir.Pos) {
	for _, bu := range f.bounds {
		where := name
		if bu.v == "" && bu.fn != "" {
			where = bu.fn // an inferred function's requirement: "showing a value in a string, in show,"
		}
		x.top.reqs = append(x.top.reqs, requirement{t: bu.t, b: bu.b, at: at, where: where, role: bu.v})
	}
	for _, o := range f.ops {
		o.callAt = at
		x.c.ts.ops = append(x.c.ts.ops, o)
	}
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

// endEntity checks `end(ok)` in an entity's method, which ends the entity
// when the transaction commits. It takes one Approval, which only approval!
// makes, so an entity can't be ended without a person's consent (SPECS.md
// §14).
func (x *fnCheck) endEntity(e *ir.MethodCall) typ {
	c := x.c
	c.useName(e, GoText{File: "check/infer.go", Text: "func (x *fnCheck) endEntity"})
	approval := con("Approval")
	if len(e.Args) != 1 || e.Args[0].Name != "" {
		x.inferArgs(e.Args)
		c.typeErrorf(e.Pos, "ending an entity needs an Approval: end(ok), with ok from approval!(…) in the job that calls this method")
		return tUnit
	}
	got := x.check(e.Args[0].Value, nil)
	if c.unify(approval, got) != nil {
		c.typeErrorf(e.Args[0].Value.Position(), "ending an entity needs an Approval, found %s: pass the one approval!(…) gave the job that calls this method", show(defaultLiteral(got)))
	}
	return tUnit
}

// methodCall checks `recv.name(args)`.
func (x *fnCheck) methodCall(e *ir.MethodCall, want typ) typ {
	c := x.c
	if e.Name == "read" && entitySet(e.Recv) {
		// idleWorlds.read() and World.all.read() give the snapshots, and
		// in a job are a recorded step (SPECS.md §15).
		prev := x.reading
		x.reading = e.Recv
		t := x.check(e.Recv, nil)
		x.reading = prev
		c.useName(e, method("read"))
		x.stepCall(e, "reading "+e.Recv.Position().Text())
		if len(e.Args) > 0 {
			x.inferArgs(e.Args)
			c.typeErrorf(e.Pos, "read takes no arguments")
		}
		return t
	}
	recv := x.check(e.Recv, nil)
	if _, isSelf := e.Recv.(*ir.Self); isSelf && x.entity != nil {
		if m, ok := x.entity.Methods[e.Name]; ok {
			c.useMethod(e, m)
			x.callee(m) // its effects count here
			return x.applyFunc(m, nil, e.Args, e.Name, e.Pos)
		}
		if e.Name == "end" {
			return x.endEntity(e)
		}
	}
	switch r := prune(recv).(type) {
	case *tcon:
		switch {
		case r.name == "Ref" && r.decl == nil && len(r.args) == 1:
			if s, ok := prune(r.args[0]).(*tcon); ok && s.entity != nil {
				if _, isMethod := s.entity.Methods[e.Name]; !isMethod && e.Name != "read" && e.Name != "end" && e.Func != nil {
					return x.funcMethod(e, recv, want)
				}
				x.effect("entity " + s.entity.Name)
				x.entityCall(e)
				if e.Name == "read" {
					c.useName(e, method("read"))
					x.inferArgs(e.Args)
					return s
				}
				if m, ok := s.entity.Methods[e.Name]; ok {
					c.useMethod(e, m)
					x.callee(m) // its effects count here
					x.privateMethod(m, e)
					return x.applyFunc(m, nil, e.Args, s.entity.Name+"."+e.Name, e.Pos)
				}
				x.inferArgs(e.Args)
				if e.Name == "end" {
					return c.unknownf(e.Pos, "an entity ends itself, so end can't be called on another: give %s a method that calls end(ok), as pub fn retire(ok: Approval) { end(ok) }", s.entity.Name)
				}
				return c.unknownf(e.Pos, "%s has no method %s", s.entity.Name, e.Name)
			}
		case r.name == "Run" && r.decl == nil && e.Name == "join":
			c.useName(e, method("join"))
			x.inferArgs(e.Args)
			x.runJoin(e)
			return &tcon{name: "Outcome", decl: c.ts.outcome, args: []typ{r.args[0]}}
		case r.name == "Run" && r.decl == nil && e.Name == "outcome":
			c.useName(e, method("outcome"))
			x.args(&tfun{ret: tUnit}, e.Args, "outcome", e.Pos)
			if x.top.durable != "" {
				join := placeText(e.Recv) + ".join!()"
				if join == "it.join!()" {
					join = "join!()"
				}
				c.typeErrorf(e.Pos, "%s, a run's outcome is read with %s, a step that waits for it: outcome() isn't recorded, so a resumed run could see another",
					x.top.durable, join)
			}
			return &tcon{name: "Option", decl: c.ts.option, args: []typ{&tcon{name: "Outcome", decl: c.ts.outcome, args: []typ{r.args[0]}}}}
		case r.name == "Event" && r.decl == nil && e.Name == "publish":
			c.useName(e, method("publish"))
			if ev := eventOf(e.Recv); ev != nil {
				x.effect("publish " + ev.Contract.Name + "." + ev.Name)
			} else {
				x.effect("publish")
			}
			x.stepCall(e, "publishing")
			x.args(&tfun{params: []fparam{{name: "value", t: r.args[0]}}, ret: tUnit}, e.Args, "publish", e.Pos)
			return tUnit
		case r.decl == nil && r.entity == nil && (r.name == "TaskGroup" || r.name == "Sender" || r.name == "Receiver"):
			return x.channelMethod(r, e)
		case r.decl == nil && r.name == "Fakes" && e.Name == "calls":
			return x.fakeCalls(e)
		case r.decl == nil && r.name == "Handler" && e.Name == "calls":
			x.inferArgs(e.Args)
			return c.unknownf(e.Pos, "this Handler, such as clock(…), keeps no calls: only fakes(…) lists the calls it answered")
		case e.Name == "reconcile" && r.decl != nil:
			c.useName(e, method("reconcile"))
			x.stepCall(e, "reconciling")
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
				x.privateMethod(m, e)
				x.callee(m)
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
			if t, ok := x.fieldCall(r, e, want); ok {
				return t
			}
			if e.Name == "index" {
				if t, ok := x.variantIndex(r, e); ok {
					return t
				}
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
			x.stepCall(e, "starting a child run")
			return x.start(r.job, e)
		}
	case *tdyn:
		// A dyn value has its trait's methods, looked up at run time.
		if m, ok := r.trait.Methods[e.Name]; ok && m.HasSelf {
			c.useMethod(e, m)
			x.callee(m) // its effects count here
			env := typeEnv{}
			for i, p := range r.trait.Params {
				if i < len(r.args) {
					env[p] = r.args[i]
				}
			}
			return x.applyFunc(m, env, e.Args, r.trait.Name+"."+e.Name, e.Pos)
		}
		if !isBuiltin(e.Name) && e.Func == nil {
			x.inferArgs(e.Args)
			return c.unknownf(e.Pos, "%s has no method %s: a dyn value has only the methods of its trait", show(recv), e.Name)
		}
	case *tvar:
		if r.rigid {
			// A type variable has the methods of the traits it requires.
			for _, b := range c.ts.bounds[r] {
				if m, ok := b.trait.Methods[e.Name]; ok && m.HasSelf {
					c.useMethod(e, m)
					x.callee(m) // its effects count here
					env := typeEnv{}
					for i, p := range b.trait.Params {
						if i < len(b.args) {
							env[p] = b.args[i]
						}
					}
					return x.applyFunc(m, env, e.Args, b.trait.Name+"."+e.Name, e.Pos)
				}
			}
			if !isBuiltin(e.Name) && e.Func == nil {
				x.inferArgs(e.Args)
				if r.name == "Self" && len(c.ts.bounds[r]) == 1 {
					tr := c.ts.bounds[r][0].trait
					return c.unknownf(e.Pos, "self is any type that implements %s, so it has only the methods of %s, and %s isn't one", tr.Name, tr.Name, e.Name)
				}
				return c.unknownf(e.Pos, "%s may be any type, so it has no method %s: require a trait that has one, as `where %s: Trait`", r.name, e.Name, r.name)
			}
		}
	}
	if r, ok := prune(recv).(*tcon); ok && r.decl == nil && r.entity == nil {
		if m := c.implMethod(r, e.Name); m != nil {
			c.useMethod(e, m)
			x.callee(m) // its effects count here
			return x.applyFunc(m, typeEnv{}, e.Args, r.name+"."+e.Name, e.Pos)
		}
	}
	if e.Func != nil {
		// No method and no field: a function in scope whose first parameter
		// takes the value, called on it, or else the built-in of that name
		// (SPECS.md §8). On a value whose type isn't known yet, a method of
		// its type might be meant, so the type must be written.
		if v, open := defaultLiteral(recv).(*tvar); !open || v.rigid {
			if !isBuiltin(e.Name) || x.c.takesFirst(e.Func, recv) {
				return x.funcMethod(e, recv, want)
			}
			e.Func = nil
		}
	}
	if isBuiltin(e.Name) && e.Func == nil {
		c.useName(e, ir.Builtin{Name: e.Name})
		c.useArgs(e, ir.Builtin{Name: e.Name})
		args := append([]ir.Arg{{Value: e.Recv}}, e.Args...)
		return x.builtinWith(e.Name, args, recv, want, e.Pos)
	}
	x.inferArgs(e.Args)
	switch r := defaultLiteral(recv).(type) {
	case tunknown:
		return r
	case *tvar:
		if !r.rigid {
			return c.unknownf(e.Pos, "the type of %s isn't known here, so its method %s can't be found: write its type where it is declared", e.Recv.Position().Text(), e.Name)
		}
	}
	return c.unknownf(e.Pos, "%s has no method %s%s", show(defaultLiteral(recv)), e.Name, x.importHint(e.Name))
}

// variantIndex checks `v.index()`, the place of v's variant in
// `T.variants()`, derived on a type whose variants have no fields (SPECS.md
// §11). On another type it is an error, unless a function named index is in
// scope to call instead.
func (x *fnCheck) variantIndex(r *tcon, e *ir.MethodCall) (typ, bool) {
	c := x.c
	d := r.decl
	why := ""
	switch {
	case d.Fields != nil || d.Alias != nil:
		why = d.Name + " is a record"
		if d.Alias != nil && d.Alias.Name != "record" {
			why = d.Name + " is another name for " + d.Alias.Name
		}
	case len(d.Variants) == 0:
		why = d.Name + " has no variants"
	default:
		for _, v := range d.Variants {
			if len(v.Fields) > 0 {
				why = "its variant " + v.Name + " has fields"
				break
			}
		}
	}
	if why != "" {
		if e.Func != nil {
			return nil, false
		}
		x.inferArgs(e.Args)
		return c.unknownf(e.Pos, "index() is derived only for a type whose variants have no fields, and %s: match the value and give each variant its number", why), true
	}
	if od, ok := c.opaqueTo(x.top.fn.Module, r); ok {
		x.inferArgs(e.Args)
		return c.unknownf(e.Pos, "%s", c.opaqueMessage(od, "variants")), true
	}
	c.useName(e, ir.EnumFn{Type: d, Name: "index"})
	x.args(&tfun{ret: tInt}, e.Args, "index", e.Pos)
	return tInt, true
}

// funcMethod checks `x.f(a)` where f is a function in scope and x's type has
// no method or field f: the call f(x, a), kept as the method call's target
// (SPECS.md §8). x, already checked as recv, goes to f's first parameter.
func (x *fnCheck) funcMethod(e *ir.MethodCall, recv, want typ) typ {
	c := x.c
	fn := e.Func
	at := e.Pos
	if nameAt, ok := c.names.name[e]; ok {
		c.use(nameAt, fn)
		at = nameAt
	}
	c.useArgs(e, fn)
	ref := &ir.Ref{At: ir.At{Pos: at}, Decl: fn}
	call := &ir.Call{At: e.At, Func: ref, Args: append([]ir.Arg{{Value: e.Recv}}, e.Args...)}
	e.Target = ir.FuncCall{Call: call}
	prev := x.calling
	x.calling = ref // called here, so the call applies its bounds
	callee := x.check(ref, nil)
	x.calling = prev
	f, ok := prune(callee).(*tfun)
	if !ok {
		x.inferArgs(e.Args)
		return c.fresh()
	}
	for _, u := range f.uses {
		x.effect(u)
	}
	shown := show(defaultLiteral(recv))
	switch {
	case len(f.params) == 0:
		x.inferArgs(e.Args)
		return c.unknownf(e.Pos, "%s has no method %s, and the function %s takes no parameters, so it can't be called on one", shown, fn.Name, fn.Name)
	case f.params[0].mut:
		c.typeErrorf(e.Pos, "%s changes its first parameter, so the change shows at the call: write %s(mut %s, …)", fn.Name, fn.Name, e.Recv.Position().Text())
	case c.unify(f.params[0].t, recv) != nil:
		c.typeErrorf(e.Recv.Position(), "%s has no method %s, and the function %s takes %s first, not %s", shown, fn.Name, fn.Name, show(defaultLiteral(f.params[0].t)), shown)
	}
	rest := &tfun{params: f.params[1:], ret: f.ret, bounds: f.bounds, ops: f.ops}
	x.argsWant(rest, e.Args, fn.Name, e.Pos, want)
	c.ts.exprs = append(c.ts.exprs, typedExpr{call, f.ret})
	x.top.calls = append(x.top.calls, pendingCall{call, f.ret})
	return f.ret
}

// takesFirst reports whether fn's first parameter may take a value of type
// t, without binding anything: x.f() calls a function of the program only
// when it does, and the built-in of that name otherwise.
func (c *checker) takesFirst(fn *ir.Func, t typ) bool {
	f := c.funcType(fn, nil)
	return len(f.params) > 0 && mayUnify(f.params[0].t, t)
}

// mayUnify reports whether want and got could unify, binding nothing. An
// open type variable may be anything, so a variable used twice may still
// make unify fail where this says yes; the call is checked properly after.
func mayUnify(want, got typ) bool {
	a, b := prune(want), prune(got)
	if a == b {
		return true
	}
	switch v := b.(type) {
	case tunknown:
		return true
	case *tvar:
		if !v.rigid {
			ct, isCon := a.(*tcon)
			return v.lit == litNone || !isCon || v.lit.allows(ct)
		}
	}
	switch x := a.(type) {
	case tunknown:
		return true
	case *tvar:
		if x.rigid {
			return false
		}
		ct, isCon := b.(*tcon)
		return x.lit == litNone || !isCon || x.lit.allows(ct)
	case *tcon:
		y, ok := b.(*tcon)
		if !ok || !sameCon(x, y) || len(x.args) != len(y.args) {
			return false
		}
		for i := range x.args {
			if !mayUnify(x.args[i], y.args[i]) {
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
			if !mayUnify(x.elems[i], y.elems[i]) {
				return false
			}
		}
		return true
	case *tfun:
		y, ok := b.(*tfun)
		return ok && len(x.params) == len(y.params)
	case *tdyn:
		return true // whether the value converts is checked at the call
	}
	return true
}

// importHint says how to call an imported module's function as a method,
// when the module has one of that name: only names in scope are found.
func (x *fnCheck) importHint(name string) string {
	m := x.c.modules[x.top.fn.Module.Path]
	if m == nil {
		return ""
	}
	for _, imp := range m.imports {
		if fn, ok := imp.values[name].(*ir.Func); ok && fn.Pub && !fn.HasSelf {
			return fmt.Sprintf("; module %s has a function %s: import it by name, import %s.{%s}, to call it on a value, or write %s.%s(…)", imp.mod.Name, name, imp.mod.Name, name, imp.mod.Name, name)
		}
	}
	return ""
}

// fieldCall checks `timer.read()` where read is a field of the record that
// holds a function: a call of the field's value (SPECS.md §8). The call is
// kept as the method call's target.
func (x *fnCheck) fieldCall(r *tcon, e *ir.MethodCall, want typ) (typ, bool) {
	c := x.c
	ft, ok := c.fieldType(r, e.Name)
	if !ok {
		return nil, false
	}
	if _, isFn := prune(ft).(*tfun); !isFn {
		return nil, false
	}
	if d, ok := c.opaqueTo(x.top.fn.Module, r); ok {
		return c.unknownf(e.Pos, "%s", c.opaqueMessage(d, "field "+e.Name)), true
	}
	c.useField(e, r, e.Name)
	field := &ir.Field{At: ir.At{Pos: e.Pos}, Value: e.Recv, Name: e.Name}
	call := &ir.Call{At: ir.At{Pos: e.Pos}, Func: field, Args: e.Args}
	e.Target = ir.FieldCall{Call: call}
	t := x.apply(ft, e.Args, want, e.Name, e.Pos)
	c.ts.exprs = append(c.ts.exprs, typedExpr{field, ft}, typedExpr{call, t})
	return t, true
}

// privateMethod reports a call of a method another module doesn't make
// public.
func (x *fnCheck) privateMethod(m *ir.Func, e *ir.MethodCall) {
	if msg := x.c.privateUse(x.top.fn.Module, m, e.Name); msg != "" {
		x.c.typeErrorf(e.Pos, "%s", msg)
	}
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
		return sig(tUnit, fparam{name: "f", t: &tfun{ret: c.fresh(), callback: "f"}})
	case r.name == "Sender" && e.Name == "put":
		return sig(tUnit, fparam{name: "value", t: r.args[0]})
	case r.name == "Sender" && e.Name == "close":
		return sig(tUnit)
	case r.name == "Receiver" && e.Name == "take":
		return sig(&tcon{name: "Option", decl: c.ts.option, args: []typ{r.args[0]}})
	}
	methods := map[string]string{"TaskGroup": "spawn", "Sender": "put and close", "Receiver": "take, and a for loop reads it"}
	x.inferArgs(e.Args)
	return c.unknownf(e.Pos, "%s has no method %s; its methods are %s", show(r), e.Name, methods[r.name])
}

// fakeCalls checks fakes.calls(notify.send): the calls a test's fake
// answered for one operation, each a record of the call's arguments, with
// defaults filled in, and its key (SPECS.md §17).
func (x *fnCheck) fakeCalls(e *ir.MethodCall) typ {
	c := x.c
	c.useName(e, method("calls"))
	var op *ir.Op
	var run *ir.Func // process.run, which fakes(process, …) answers
	if len(e.Args) == 1 && e.Args[0].Name == "" {
		if r, ok := e.Args[0].Value.(*ir.Ref); ok {
			op, _ = r.Decl.(*ir.Op)
			if f, ok := r.Decl.(*ir.Func); ok && f.Host == "process.run" {
				run = f
			}
		}
	}
	if c.ts.calls == nil {
		c.ts.calls = map[any]*ir.Type{}
	}
	if run != nil {
		x.check(e.Args[0].Value, nil)
		d := c.ts.calls[run]
		if d == nil {
			rec := &ir.TypeExpr{Name: "record"}
			for _, p := range run.Params {
				rec.Fields, rec.Args = append(rec.Fields, p.Name), append(rec.Args, p.Type)
			}
			d = &ir.Type{Name: "Call(process.run)", Module: run.Module, Fields: rec.Fields, Alias: rec, Pos: run.Pos}
			c.ts.calls[run] = d
		}
		return con("List", &tcon{name: d.Name, decl: d})
	}
	if op == nil {
		x.inferArgs(e.Args)
		return c.unknownf(e.Pos, "calls takes an operation of a contract the fakes answer for, as calls(notify.send), or process.run")
	}
	c.ts.exprs = append(c.ts.exprs, typedExpr{e.Args[0].Value, c.opType(op)}) // names the operation
	d := c.ts.calls[op]
	if d == nil {
		name := "Call(" + op.Contract.Module.Name + "." + op.Name + ")"
		rec := &ir.TypeExpr{Name: "record"}
		for _, p := range op.Params {
			rec.Fields, rec.Args = append(rec.Fields, p.Name), append(rec.Args, p.Type)
		}
		if op.Key {
			rec.Fields, rec.Args = append(rec.Fields, "key"), append(rec.Args, &ir.TypeExpr{Name: "Key"})
		}
		d = &ir.Type{Name: name, Module: op.Contract.Module, Fields: rec.Fields, Alias: rec, Pos: op.Pos}
		c.ts.calls[op] = d
	}
	return con("List", &tcon{name: d.Name, decl: d})
}

// entityCall reports an entity call in a job that isn't a durable step.
func (x *fnCheck) entityCall(e *ir.MethodCall) {
	x.stepCall(e, "an entity call")
}

// stepCall reports a recorded step in a job or a waiting function written
// without `!`: an entity call, publishing an event, starting a child run or
// reading a view (SPECS.md §12).
func (x *fnCheck) stepCall(e *ir.MethodCall, what string) {
	if x.top.durable != "" && x.kind != kindLambda && x.waiting != ir.Expr(e) {
		x.c.typeErrorf(e.Pos, "%s, %s is a durable step: write %s!(…)", x.top.durable, what, e.Name)
	}
}

// runJoin reports `run.join()` written without `!`. join on a run is always
// the wait, never the strings' join, so the message names the call to write:
// join!() where the code can wait, outcome() where it can't (SPECS.md §15).
func (x *fnCheck) runJoin(e *ir.MethodCall) {
	if x.waiting == ir.Expr(e) {
		return // join!(), which lowering checks may wait here
	}
	run := placeText(e.Recv)
	if x.top.durable != "" {
		join := run + ".join!()"
		if run == "it" {
			join = "join!()"
		}
		x.c.typeErrorf(e.Pos, "%s, joining a run is a durable step that waits for it: write %s", x.top.durable, join)
		return
	}
	outcome := run + ".outcome()"
	if run == "it" {
		outcome = "outcome() on the run"
	}
	who := "this code"
	switch {
	case x.top.kind == kindTest:
		who = "a test"
	case x.top.kind == kindEntity:
		who = "an entity method"
	case x.top.kind == kindOperation:
		who = "a served operation"
	case x.top.fn != nil && x.top.fn.Module != nil && x.top.fn.Module.Main == x.top.fn:
		who = "main"
	}
	x.c.typeErrorf(e.Pos, "%s can't wait for a run: read where it got to with %s, which is None while the run is still going (only a job waits, with join!())",
		who, outcome)
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
	x.effect("start " + job.Name)
	keyed := false
	for _, a := range e.Args {
		keyed = keyed || a.Name == "key"
	}
	switch main := x.top.fn; {
	case keyed || x.top.durable != "":
	case main.Module != nil && main.Module.Main == main && x.top.kind == kindPlain:
		x.c.typeErrorf(e.Pos, "eye run runs main every time it starts, so this would start %s again after each restart; give the run a key, which starts it once: %s.start(…, key: \"…\")", job.Name, job.Name)
	default:
		// A function main calls, directly or not, would do the same
		// (checkUnkeyedStarts).
		x.c.ts.unkeyed[x.top.fn] = append(x.c.ts.unkeyed[x.top.fn], unkeyedStart{job: job, in: x.top.fn})
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
	case r.name == "Instant" && e.Name == "unix":
		return sig(tInt)
	case r.name == "Duration" && (e.Name == "seconds" || e.Name == "millis"):
		return sig(tInt)
	case r.name == "Time" && e.Name == "next":
		return sig(tInstant, in, fparam{name: "after", t: tInstant})
	case r.name == "Date" && e.Name == "at":
		return sig(tInstant, fparam{name: "time", t: tTime}, in)
	case (r.name == "Date" || r.name == "Time") && e.Name == "format":
		return sig(tString, fparam{name: "pattern", t: tString})
	}
	return nil, false
}

// ---------------------------------------------------------------- other expressions

func (x *fnCheck) field(e *ir.Field) typ {
	c := x.c
	recv := x.check(e.Value, nil)
	switch r := defaultLiteral(recv).(type) {
	case *tentity:
		if e.Name == "all" {
			c.useName(e, GoText{File: "check/infer.go", Text: `e.Name == "all"`})
			x.effect("entity " + r.e.Name)
			x.unrecordedSet(e, r.e.Name+".all")
			return con("List", &tcon{name: r.e.Name, entity: r.e})
		}
	case *tcon:
		if r.name == "Ref" && r.decl == nil && len(r.args) == 1 {
			if s, ok := prune(r.args[0]).(*tcon); ok && s.entity != nil && !isIdentity(s.entity, e.Name) {
				x.effect("entity " + s.entity.Name)
				if x.top.durable != "" {
					// Outside jobs ref.field is short for ref.read().field.
					ref := e.Value.Position().Text()
					c.typeErrorf(e.Pos, "%s, %s reads %s without recording it, so a resumed run could see another value: read a snapshot with %s.read!() and take %s from it", x.top.durable, e.Pos.Text(), s.entity.Name, ref, e.Name)
				}
			}
		}
		if d, ok := c.opaqueTo(x.top.fn.Module, r); ok {
			return c.unknownf(e.Pos, "%s", c.opaqueMessage(d, "field "+e.Name))
		}
		if t, ok := c.fieldType(r, e.Name); ok {
			if r.uncertain || r.fresh {
				// A command that returns a record may come back as an
				// outcome it adds instead, which has no fields.
				may := "Uncertain(key): match it with an Uncertain(_) arm, or reconcile it with .reconcile!(),"
				if r.fresh {
					may = "Uncertain, Stale or Expired: match it with arms for them"
				}
				c.typeErrorf(e.Pos, "this command's reply may have been lost, so it may be %s before reading its %s", may, e.Name)
			}
			c.useField(e, r, e.Name)
			if row := c.ts.rowOf[r.decl]; row != nil && r.decl != nil {
				row.reads = append(row.reads, rowRead{e.Name, e.Pos})
			}
			return t
		}
	case *tdyn:
		return c.unknownf(e.Pos, "%s has no field %s: a dyn value has only the methods of its trait", show(r), e.Name)
	case *ttuple:
		return c.unknownf(e.Pos, "%s has no field %s: take a tuple apart with a pattern, as `let (a, b) = pair`", show(r), e.Name)
	case tunknown:
		return r
	case *tvar:
		if !r.rigid {
			return c.unknownf(e.Pos, "the type of %s isn't known here, so its field %s can't be found: write its type where it is declared", e.Value.Position().Text(), e.Name)
		}
	}
	return c.unknownf(e.Pos, "%s has no field %s", show(defaultLiteral(recv)), e.Name)
}

// isIdentity reports whether name is an entity's identity field, which a
// Ref reads without loading the entity (SPECS.md §14).
func isIdentity(e *ir.Entity, name string) bool {
	return name == "id" || e.ID != nil && e.ID.Name == name
}

// unrecordedSet reports a view or E.all read in a job or a waiting function
// other than with read!(): a resumed run would read it again and could see
// other entities (SPECS.md §15).
func (x *fnCheck) unrecordedSet(e ir.Expr, name string) {
	if x.top.durable != "" && x.reading != e {
		x.c.typeErrorf(e.Position(), "%s, reading %s isn't recorded, so a resumed run could see other entities: read it with %s.read!(), a recorded step that gives a list of snapshots", x.top.durable, name, name)
	}
}

// entitySet reports whether e is a view or E.all, whose read() gives its
// snapshots.
func entitySet(e ir.Expr) bool {
	switch e := e.(type) {
	case *ir.Ref:
		_, ok := e.Decl.(*ir.View)
		return ok
	case *ir.Field:
		if r, ok := e.Value.(*ir.Ref); ok {
			_, isEntity := r.Decl.(*ir.Entity)
			return isEntity && e.Name == "all"
		}
	}
	return false
}

func (x *fnCheck) index(e *ir.Index) typ {
	c := x.c
	coll := x.check(e.Value, nil)
	if _, isRange := e.Index.(*ir.Range); isRange {
		x.check(e.Index, con("Range", tInt))
		return coll
	}
	option := func(t typ) typ { return &tcon{name: "Option", decl: c.ts.option, args: []typ{t}} }
	switch r := defaultLiteral(coll).(type) {
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
	case tunknown:
		x.check(e.Index, nil)
		return option(r)
	case *tvar:
		if !r.rigid {
			x.check(e.Index, nil)
			return c.unknownf(e.Pos, "the type of %s isn't known here, so it can't be indexed: write its type where it is declared", e.Value.Position().Text())
		}
	}
	x.check(e.Index, nil)
	return c.unknownf(e.Pos, "%s can't be indexed: xs[i] takes an element of a List or Bytes, and m[key] a value of a Map", show(defaultLiteral(coll)))
}

// placeType is the type of what an assignment changes; op is its operator,
// "=" or a compound one such as "+=".
func (x *fnCheck) placeType(p ir.Place, op string) typ {
	var t typ
	shift := 0 // names before p.Path in the target as written
	if p.Self {
		t = x.self
		if x.entity != nil && len(p.Path) > 0 {
			x.c.usePlace(p.Pos, 0, Member{Of: x.entity, Name: p.Path[0].Field})
			shift = 1
			st, ok := x.c.stateType(x.entity, p.Path[0].Field)
			if !ok {
				return x.c.unknownf(p.Pos, "%s has no state %s", x.entity.Name, p.Path[0].Field)
			}
			t = st
			p.Path = p.Path[1:]
		}
	} else if p.Slot < len(x.slots) {
		t = x.slots[p.Slot]
	}
	if t == nil {
		return x.c.unknownf(p.Pos, "internal error: what this assigns to has no type")
	}
	for i, step := range p.Path {
		if step.Index != nil {
			t = x.elementType(t, step.Index, i == len(p.Path)-1, op, p.Pos)
			continue
		}
		name := step.Field
		if owner := x.c.memberOwner(t, name); owner != nil {
			x.c.usePlace(p.Pos, i+shift, Member{Of: owner, Name: name})
		}
		if d, ok := x.c.opaqueTo(x.top.fn.Module, t); ok {
			return x.c.unknownf(p.Pos, "%s", x.c.opaqueMessage(d, "field "+name))
		}
		ft, ok := x.c.fieldType(t, name)
		if !ok {
			switch r := prune(t).(type) {
			case tunknown:
				return r
			case *tvar:
				if !r.rigid {
					return x.c.unknownf(p.Pos, "the type of what this assigns to isn't known here, so its field %s can't be found: write its type where it is declared", name)
				}
			}
			return x.c.unknownf(p.Pos, "%s has no field %s", show(t), name)
		}
		t = ft
	}
	return t
}

// elementType is the type of the element `[index]` of a value of type t on
// the left of an assignment (SPECS.md §7): an element of a List, which must
// be there, or a Map's value for a key, which `=` inserts or replaces.
func (x *fnCheck) elementType(t typ, index ir.Expr, last bool, op string, at ir.Pos) typ {
	c := x.c
	if r, ok := defaultLiteral(t).(*tcon); ok && r.decl == nil && r.entity == nil {
		switch r.name {
		case "List":
			x.check(index, tInt)
			return r.args[0]
		case "Map":
			x.check(index, r.args[0])
			switch {
			case !last:
				return c.unknownf(index.Position(), "a map's value can't be changed inside, since the key may be missing: read it with m[key], change the copy, and assign it back with m[key] = v")
			case op != "=":
				return c.unknownf(index.Position(), "a map may not hold the key, so m[key] %s v has nothing to start from: write m[key] = (m[key] ?? 0) %s v", op, strings.TrimSuffix(op, "="))
			}
			return r.args[1]
		case "Bytes":
			x.check(index, nil)
			return c.unknownf(index.Position(), "Bytes don't change one byte at a time: build a List(Byte), assign its elements, and make Bytes with toBytes(list)")
		}
	}
	x.check(index, nil)
	switch r := defaultLiteral(t).(type) {
	case tunknown:
		return r
	case *tvar:
		if !r.rigid {
			return c.unknownf(at, "the type of what this assigns to isn't known here, so its elements can't be found: write its type where it is declared")
		}
	}
	return c.unknownf(index.Position(), "%s has no elements to assign: xs[i] = v changes an element of a List, and m[key] = v a value of a Map", show(defaultLiteral(t)))
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
	return x.arith(e.Op, l, r, e.R, e.Pos, e)
}

// arith is the type of `l op r` for + - * / %: the combinations the
// built-in types have (SPECS.md §11), and a declared type's own operator
// methods. When the left side's type isn't decided yet, as an untyped
// parameter's isn't, whether it has the operator waits for what decides it
// (settleOps).
// e is the Binary or Assign the operator is in, which records the impl
// method that answers it; nil in a generic function's body checked for one
// call, where each call's types may choose another.
func (x *fnCheck) arith(op string, l, r typ, rhs ir.Expr, at ir.Pos, e ir.Expr) typ {
	v, undecided := prune(l).(*tvar)
	out := x.arithNow(op, l, r, rhs, at, e)
	if undecided && !v.rigid && !v.never {
		x.c.ts.ops = append(x.c.ts.ops, pendingOp{x: x, op: op, l: l, r: r, out: out, rhs: rhs, at: at, e: e})
	}
	return out
}

// setOpImpl records the impl method an operator calls (7.1).
func setOpImpl(e ir.Expr, method *ir.Func) {
	switch e := e.(type) {
	case *ir.Binary:
		e.Impl = method
	case *ir.Assign:
		e.Impl = method
	}
}

func (x *fnCheck) arithNow(op string, l, r typ, rhs ir.Expr, at ir.Pos, e ir.Expr) typ {
	c := x.c
	lt, rt := prune(l), prune(r)
	name := func(t typ) string {
		if k, ok := t.(*tcon); ok && k.decl == nil && k.entity == nil {
			return k.name
		}
		if v, ok := t.(*tvar); ok {
			switch v.lit {
			case litInt, litZero:
				return "Int"
			case litString:
				return "String"
			case litChar:
				return "Char"
			}
		}
		return ""
	}
	ln, rn := name(lt), name(rt)
	// A NonZero(Int) counts as an Int in arithmetic, and what it gives may
	// be 0: a `let k = 4` that divides a Duration can scale one too. As a
	// divisor it stays what / and % need.
	if k, ok := lt.(*tcon); ok && ln == "NonZero" && len(k.args) == 1 && name(prune(k.args[0])) == "Int" {
		l, lt, ln = tInt, tInt, "Int"
	}
	if k, ok := rt.(*tcon); ok && rn == "NonZero" && len(k.args) == 1 && name(prune(k.args[0])) == "Int" && op != "/" && op != "%" {
		r, rt, rn = tInt, tInt, "Int"
	}
	same := func(result typ) typ {
		x.expect(at, l, r) // on a mismatch, the result is still what the left says
		return result
	}
	if k, ok := lt.(*tcon); ok && (k.decl != nil || k.entity != nil) {
		// A declared type's operators are its impls of Add, Sub, … (§11).
		if rhs, out, method, found := x.opImpl(k, op, r); found {
			x.expect(at, rhs, r)
			if e != nil && method != nil {
				setOpImpl(e, method)
			}
			return out
		}
		if k.decl != nil && k.decl.Module != x.top.fn.Module {
			return c.unknownf(at, "%s %s %s isn't defined: %s doesn't implement %s(%s, …)", show(lt), op, show(defaultLiteral(r)), show(lt), opTrait(op), show(defaultLiteral(r)))
		}
		return c.unknownf(at, "%s has no %s: implement %s for it, as `impl %s(%s, %s) for %s`", show(lt), op, opTrait(op), opTrait(op), show(lt), show(lt), show(lt))
	}
	if fixedRanges[ln] != nil || fixedRanges[rn] != nil {
		// Fixed-width arithmetic is a method that says what happens at the
		// edges (SPECS.md §5).
		if fixedRanges[ln] != nil {
			return c.unknownf(at, "%s", fixedOpMessage(ln, op))
		}
		return c.unknownf(at, "%s", fixedOpMessage(rn, op))
	}
	if (op == "/" || op == "%") && (ln == "Int" || ln == "" && (rn == "Int" || rn == "NonZero")) {
		// An Int is divided only by what can't be 0 (SPECS.md §10).
		x.expect(at, tInt, l)
		x.divisor(r, rhs, at)
		return l
	}
	if lv, ok := lt.(*tvar); ok && !lv.rigid && lv.lit == litNone && (op == "/" || op == "%") {
		if rv, ok := rt.(*tvar); ok && !rv.rigid && rv.lit == litNone {
			// Untyped parameters, which later calls decide: if they are
			// Ints, the divisor is checked then (see arith).
			return l
		}
	}
	lv, _ := lt.(*tvar)
	switch {
	case ln == "Int" || rn == "Int" && ln == "" && (lv == nil || !lv.rigid):
		// A function's own type variable times an Int is what its bounds
		// say, below: `where t: Mul(Int, t)`.
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
	case (ln == "List" || ln == "Set") && op == "+":
		return same(l)
	case ln == "Duration":
		switch {
		case op == "/" && rn == "Duration":
			// How many times one goes into the other (SPECS.md §20).
			return tFloat
		case op == "/":
			x.divisor(r, rhs, at)
			return tDuration
		case op == "*":
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
	case (ln == "Instant" || ln == "Date") && op != "+" && op != "-":
		x.expect(at, tDuration, r)
		return c.unknownf(at, "%s %s … isn't defined: a time moves by adding or subtracting a Duration", ln, op)
	case ln == "Instant":
		if rn == "Instant" && op == "-" {
			return tDuration
		}
		x.expect(at, tDuration, r)
		return tInstant
	case ln == "Date":
		x.expect(at, tDuration, r)
		return tDate
	case ln != "":
		return c.unknownf(at, "%s %s %s isn't defined", show(defaultLiteral(lt)), op, show(defaultLiteral(rt)))
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
			return c.unknownf(at, "%s %s … needs %s: %s, and %s may be any type: require it, as `where %s: %s(%s, %s)`", v.name, op, v.name, opTrait(op), v.name, v.name, opTrait(op), v.name, v.name)
		}
		if _, rvar := rt.(*tvar); !rvar && rn != "" {
			return same(r)
		}
		return same(l)
	case tunknown:
		return v
	}
	if _, ok := rt.(tunknown); ok {
		return rt
	}
	return c.unknownf(at, "%s %s %s isn't defined", show(lt), op, show(rt))
}

// negate is the type of `-x`: a number's or a duration's own type, or what
// the type's impl of Neg gives (SPECS.md §11).
func (x *fnCheck) negate(t typ, e *ir.Unary) typ {
	c, at := x.c, e.Pos
	neg := c.preludeTrait("Neg")
	switch k := prune(t).(type) {
	case tunknown:
		return k
	case *tvar:
		if k.lit.intLiteral() {
			return k
		}
		if !k.rigid {
			x.require(k, neg, []typ{k}, at, "-")
			return k
		}
		for _, b := range c.ts.bounds[k] {
			if b.trait == neg && len(b.args) == 1 {
				return b.args[0]
			}
		}
		return c.unknownf(at, "-%s needs %s: Neg, and %s may be any type: require it, as `where %s: Neg(%s)`", k.name, k.name, k.name, k.name, k.name)
	case *tcon:
		if k.decl != nil {
			for _, impl := range k.decl.Impls {
				if impl.Trait == neg && len(impl.Args) == 1 {
					e.Impl = impl.Methods["neg"]
					return c.fromExpr(impl.Args[0], envOf(k), false)
				}
			}
			if k.decl.Module != x.top.fn.Module {
				return c.unknownf(at, "-%s isn't defined: %s doesn't implement Neg", show(k), show(k))
			}
			return c.unknownf(at, "%s has no -: implement Neg for it, as `impl Neg(%s) for %s`", show(k), show(k), show(k))
		}
		if ok, _ := c.builtinHas(k, cbound{trait: neg, args: []typ{k}}, map[*ir.Type]bool{}); ok {
			return k
		}
	}
	return c.unknownf(at, "-%s isn't defined: only numbers and durations have a minus sign", show(defaultLiteral(t)))
}

// pendingOp is `l op r` of types not yet decided, giving out.
type pendingOp struct {
	x    *fnCheck
	op   string
	l, r typ
	out  typ
	rhs  ir.Expr
	at   ir.Pos
	e    ir.Expr // the Binary or Assign, which records the impl it calls
	// in names the generic functions it is inside, for one a call
	// instantiated: its errors are reported at the call, and say where.
	in     string
	callAt ir.Pos
}

// settleOps checks the operators that waited for the whole program, now
// that their calls have decided their types: `n + n` in `fn twice(n)` is
// fine for Ints and an error for Bools, and a division of Ints takes a
// NonZero(Int) divisor. A type still undecided may be any type.
func (c *checker) settleOps() {
	ops := c.ts.ops
	c.ts.ops = nil
	for _, o := range ops {
		if v, ok := defaultLiteral(o.l).(*tvar); ok && !v.rigid {
			continue
		}
		if o.in == "" {
			o.x.expect(o.at, o.out, o.x.arith(o.op, o.l, o.r, o.rhs, o.at, o.e))
			continue
		}
		// From a generic function's body: report at the call that gave
		// these types, saying where in the function.
		before := len(c.diags)
		o.x.callAt = &o.callAt
		o.x.expect(o.callAt, o.out, o.x.arith(o.op, o.l, o.r, o.rhs, o.callAt, nil))
		o.x.callAt = nil
		for i := before; i < len(c.diags); i++ {
			c.diags[i].Message = fmt.Sprintf("in %s, %s", o.in, c.diags[i].Message)
		}
	}
}

// divisor checks what an Int is divided by: a NonZero(Int), which a
// literal other than 0 is too (SPECS.md §10).
func (x *fnCheck) divisor(r typ, rhs ir.Expr, at ir.Pos) {
	where := rhs.Position()
	if x.callAt != nil {
		where = *x.callAt // a generic function's division, for one call
	}
	if v, ok := prune(r).(*tvar); ok && v.lit == litZero {
		x.c.typeErrorf(where, "dividing by 0 has no answer")
		return
	}
	if x.c.unify(con("NonZero", tInt), r) != nil {
		x.c.typeErrorf(where, "dividing by %s fails when it is 0, so / and %% take a NonZero(Int): match nonzero(%s), which is Some(n) unless it is 0, or divide by a literal other than 0", show(defaultLiteral(r)), rhs.Position().Text())
	}
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
	matched := scrutinee // what the arms after an Uncertain(_) arm can be
	for i := range e.Arms {
		arm := &e.Arms[i]
		arm.Pattern = x.bindPattern(arm.Pattern, matched, e.Pos)
		if arm.Guard != nil {
			x.check(arm.Guard, tBool)
		} else if k, ok := prune(matched).(*tcon); ok && k.uncertain && x.handlesUncertain(arm.Pattern) {
			// Past an arm that takes Uncertain(_), the reply wasn't lost.
			certain := *k
			certain.uncertain, certain.reconcilable = false, false
			matched = &certain
		}
		x.check(arm.Body, result)
	}
	x.top.matches = append(x.top.matches, pendingMatch{e, scrutinee})
	return result
}

// handlesUncertain reports whether a pattern takes every Uncertain(key).
func (x *fnCheck) handlesUncertain(p ir.Pattern) bool {
	switch p := p.(type) {
	case ir.PVariant:
		if p.Variant != x.c.ts.uncert || len(p.Args) > 1 {
			return false
		}
		return len(p.Args) == 0 || wildLike(p.Args[0])
	case ir.POr:
		for _, a := range p.Alts {
			if x.handlesUncertain(a) {
				return true
			}
		}
	}
	return false
}

// try checks `x?`: a Result's error passes to the caller, wrapped in the
// caller's error type when exactly one of its variants holds it, and an
// Option's None passes on as None.
func (x *fnCheck) try(e *ir.Try) typ {
	c := x.c
	t := prune(x.check(e.X, nil))
	res := x.res
	if res == nil {
		res = x.ret
	}
	if v, ok := t.(*tvar); ok && !v.rigid {
		// A lambda's parameter whose type a later call gives: whether it
		// is an Option or a Result waits for the whole body.
		val := c.fresh()
		x.top.tries = append(x.top.tries, pendingTry{x: x, e: e, op: t, val: val, res: res})
		return val
	}
	k, ok := x.tryOperand(e, t)
	if !ok {
		return c.fresh()
	}
	if _, open := prune(res).(*tvar); open {
		// An inferred function's or a lambda's result: the rest of the
		// code decides it.
		x.top.tries = append(x.top.tries, pendingTry{x: x, e: e, k: k, res: res})
	} else {
		x.tryInto(e, k, res)
	}
	return k.args[0]
}

// tryOperand returns the Option or Result `?` is applied to, and reports
// any other type.
func (x *fnCheck) tryOperand(e *ir.Try, t typ) (*tcon, bool) {
	c := x.c
	if _, unknown := t.(tunknown); unknown {
		return nil, false // an error was reported
	}
	if k, ok := t.(*tcon); ok && k.decl != nil && (k.decl == c.ts.option || k.decl == c.ts.result) {
		return k, true
	}
	c.typeErrorf(e.Pos, "? passes on the error of a Result or the None of an Option; this is %s", show(t))
	return nil, false
}

// settle checks a `?` whose operand's type wasn't known where it was
// checked, once the whole body has said what it is.
func (t pendingTry) settle() {
	x, c := t.x, t.x.c
	op := prune(t.op)
	if v, ok := op.(*tvar); ok && v.lit != litNone {
		op = defaultLiteral(op) // a literal: `f(3)` makes `x?` an Int's
	}
	if _, open := op.(*tvar); open {
		c.typeErrorf(t.e.Pos, "is this an Option or a Result? Nothing here says, so ? can't tell what it passes on; write its type where the value is bound, as `x: Option(Int)` on a parameter or `let x: Option(Int) = …`")
		return
	}
	k, ok := x.tryOperand(t.e, op)
	if !ok {
		return
	}
	x.expect(t.e.Pos, t.val, k.args[0])
	x.tryInto(t.e, k, t.res)
}

// tryInto checks that `?` on k can return its None or its error from a
// function whose result is res: an Option's None needs an Option, and a
// Result's error a Result whose error type is it or holds it.
func (x *fnCheck) tryInto(e *ir.Try, k *tcon, res typ) {
	// Errors here are about the function `?` returns from, so they point at
	// the `?`, not at the value before it.
	c := x.c
	ret, ok := prune(res).(*tcon)
	if !ok {
		return // still open, or an error was reported
	}
	if k.decl == c.ts.option {
		if ret.decl != c.ts.option {
			who := x.who()
			c.typeErrorf(questionMark(e), "? on an Option returns its None from %s, but %s returns %s: give a default with ?? value, match the None, or return an Option",
				who, who, show(ret))
		}
		return
	}
	if ret.decl != c.ts.result {
		c.typeErrorf(questionMark(e), "? on a Result needs a function that returns a Result, and %s returns %s", x.who(), show(ret))
		return
	}
	if d, ok := prune(ret.args[1]).(*tcon); ok && d.decl != nil {
		e.Wrap = d.decl // a lambda's own error type, or an inferred one
	} else if x.kind == kindLambda {
		e.Wrap = nil // lowering gave the enclosing function's, which a lambda's `?` doesn't return to
	}
	if x.intoProblem(k.args[1], ret.args[1], questionMark(e)) {
		return
	}
	if !x.converts(k.args[1], ret.args[1]) {
		c.typeErrorf(questionMark(e), "? can't pass on a %s from a function whose error is %s: none of its variants holds a %s; use .mapErr(…)?",
			show(k.args[1]), show(ret.args[1]), show(k.args[1]))
	}
}

// questionMark is where the `?` of e is: the last character of its span.
func questionMark(e *ir.Try) ir.Pos {
	p := e.Pos
	if p.End > p.Offset {
		p.Offset = p.End - 1
	}
	return p
}

// who names the function `?` returns from, for messages.
func (x *fnCheck) who() string {
	switch {
	case x.kind == kindLambda:
		return "this lambda"
	case x.kind == kindJob:
		return "job " + x.fn.Name
	case x.kind == kindTest:
		return "this test"
	case x.fn.Name == "":
		return "this function"
	}
	return x.fn.Name
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
	f := &tfun{ret: c.fresh(), fx: newFx(fxSrc{fn: e.Func, lambda: true, name: "this function", at: e.Pos})}
	for _, capt := range e.Func.Captures {
		if capt.Slot < len(inner.slots) {
			t := x.check(capt.From, nil)
			inner.slots[capt.Slot] = t
			// A lambda that captures a callback may call it, so what it
			// does is whatever was passed in.
			if g, ok := prune(t).(*tfun); ok && g.callback != "" && f.fx.callback == "" {
				f.fx.callback, f.fx.callbackAt = g.callback, e.Pos
			}
		}
	}
	params := e.Func.Params
	for _, p := range params {
		if p.Type != nil {
			inner.slots[p.Slot] = c.fromExpr(p.Type, x.env, true)
		}
	}
	w, _ := prune(orNil(want)).(*tfun)
	if w == nil || w.callback == "" {
		x.changingCaptures(e)
	}
	given := make([]typ, len(params)) // what a parameter takes, where it differs from its slot
	switch {
	case w != nil && len(w.params) == len(params):
		for i, p := range params {
			if p.Type != nil {
				// A type written on the parameter must be what the lambda
				// is given. The body still sees the written type, and the
				// lambda's own type takes the given one, so that the call
				// doesn't report the mismatch a second time.
				if !x.expect(e.Pos, w.params[i].t, inner.slots[p.Slot]) {
					given[i] = w.params[i].t
				}
				continue
			}
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
		inner.res = f.ret
		inner.check(e.Func.Body, f.ret)
		return f
	}
	for i, p := range params {
		t := inner.slots[p.Slot]
		if given[i] != nil {
			t = given[i]
		}
		f.params = append(f.params, fparam{name: p.Name, t: t})
	}
	ret := f.ret
	inner.res = ret // `?` in the lambda returns from it
	if r, ok := prune(ret).(*tcon); ok && r.name == "()" && r.decl == nil {
		inner.check(e.Func.Body, discard)
	} else {
		inner.check(e.Func.Body, ret)
	}
	return f
}

// settleEarly makes a call's result what the call is expected to give, before
// its lambdas are checked, so that a lambda's body knows the type it returns:
// where a List(Row) is expected, `users |> map(fn u: { name: u.name })`
// builds Rows, and a later stage's lambda reads their fields (SPECS.md §5).
// It only binds the result's open type variables to the parts of want they
// stand at, and binds nothing unless the two have one shape, so the call is
// still checked against want afterwards and a mismatch is reported there.
func (x *fnCheck) settleEarly(result, want typ) {
	if want == nil || want == discard || result == nil {
		return
	}
	type binding struct {
		v *tvar
		t typ
	}
	var binds []binding
	var fits func(r, w typ) bool
	fits = func(r, w typ) bool {
		r, w = prune(r), prune(w)
		if _, open := w.(*tvar); open {
			return true
		}
		if _, unknown := w.(tunknown); unknown {
			return true
		}
		switch rt := r.(type) {
		case *tvar:
			if rt.rigid || rt.lit != litNone || rt.never || occurs(rt, w) {
				return true
			}
			switch w.(type) {
			case *tcon, *ttuple:
				binds = append(binds, binding{rt, w})
			}
			return true
		case *tcon:
			wt, ok := w.(*tcon)
			if !ok || !sameCon(rt, wt) || len(rt.args) != len(wt.args) {
				return false
			}
			for i := range rt.args {
				if !fits(rt.args[i], wt.args[i]) {
					return false
				}
			}
			return true
		case *ttuple:
			wt, ok := w.(*ttuple)
			if !ok || len(rt.elems) != len(wt.elems) {
				return false
			}
			for i := range rt.elems {
				if !fits(rt.elems[i], wt.elems[i]) {
					return false
				}
			}
			return true
		}
		return true
	}
	if !fits(result, want) {
		return
	}
	for _, b := range binds {
		x.c.unify(b.t, b.v)
	}
}

// changingCaptures reports what a lambda that isn't a callback captures and
// can change: a var, a mut parameter, or a mut self method's receiver. A
// callback, given straight to a parameter whose function type has no uses,
// runs before the call returns, so it reads the value at the call; any
// other lambda may run later, after the value changed (SPECS.md §7).
func (x *fnCheck) changingCaptures(e *ir.Lambda) {
	for _, capt := range e.Func.Captures {
		switch capt.Changing {
		case "":
		case "self":
			what := "self, the receiver of a mut self method"
			if capt.Name != "self" {
				what = capt.Name + ", the state of the entity"
			}
			x.c.typeErrorf(capt.From.Position(), "a closure can't capture %s, unless it is a callback given straight to a call; copy what it needs into a let first", what)
		default:
			x.c.typeErrorf(capt.From.Position(), "a closure can't capture %s, %s, unless it is a callback given straight to a call; copy it into a let first", capt.Name, capt.Changing)
		}
	}
}

// record checks a record literal, which takes the record type expected
// where it is. When nothing here says yet, the rest of the function may.
func (x *fnCheck) record(e *ir.Record, want typ) typ {
	c := x.c
	values := make([]typ, len(e.Values))
	pos := make([]ir.Pos, len(e.Values))
	if rec, ok := isRecord(orNil(want)); ok {
		if d, ok := c.opaqueTo(x.top.fn.Module, rec); ok {
			c.typeErrorf(e.Pos, "%s", c.opaqueMessage(d, "fields"))
		}
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
	if row, ok := prune(orNil(want)).(*tcon); ok && row.decl != nil && c.ts.rowOf[row.decl] != nil {
		// A migration's row for the next version has the fields it gives.
		for i, v := range e.Values {
			ft, _ := c.rowField(row.decl, e.Names[i])
			c.ts.rowOf[row.decl].given[e.Names[i]] = true
			x.check(v, ft)
		}
		return row
	}
	for i, v := range e.Values {
		values[i] = x.check(v, nil)
		pos[i] = v.Position()
	}
	if want == nil || want == discard {
		v := c.fresh()
		x.top.records = append(x.top.records, pendingRecord{t: v, at: e.Pos, names: e.Names, values: values, pos: pos, e: e})
		return v
	}
	switch w := prune(want).(type) {
	case *tvar:
		if !w.rigid {
			x.top.records = append(x.top.records, pendingRecord{t: w, at: e.Pos, names: e.Names, values: values, pos: pos, e: e})
			return w
		}
	case tunknown:
		return w
	}
	return c.unknownf(e.Pos, "expected %s, found a record", show(want))
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
	switch r := defaultLiteral(t).(type) {
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
	case *tvar:
		if r.lit == litString {
			return tChar
		}
		if !r.rigid {
			return x.c.unknownf(at, "the type of what for walks isn't known here: write its type where it is declared")
		}
	case tunknown:
		return r
	}
	return x.c.unknownf(at, "%s isn't a collection that for can walk", show(t))
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
		if s, ok := p.Value.(string); ok && bytesLike(t) {
			// A string where Bytes are matched is its bytes (SPECS.md §9).
			return ir.PList{Pats: byteLits(s, p.Pos), Rest: -1}
		}
		p.Value = x.literalPattern(p.Value, t, written(p.Pos, at))
		return p
	case ir.PRange:
		p.Lo = x.literalPattern(p.Lo, t, written(p.LoPos, at))
		p.Hi = x.literalPattern(p.Hi, t, written(p.HiPos, at))
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
		if fixedName(elem) == "Byte" {
			p = spliceStrings(p)
		}
		for i := range p.Pats {
			p.Pats[i] = x.bindPattern(p.Pats[i], elem, at)
		}
		if p.RestPat != nil {
			p.RestPat = x.bindPattern(p.RestPat, prune(t), at)
		}
		return p
	case ir.PRecord:
		if d, ok := c.opaqueTo(x.top.fn.Module, t); ok {
			c.typeErrorf(at, "%s", c.opaqueMessage(d, "fields"))
		}
		for i, name := range p.Names {
			ft, ok := c.fieldType(t, name)
			if !ok {
				switch k := prune(t).(type) {
				case tunknown:
					ft = k
				case *tvar:
					if !k.rigid {
						ft = c.unknownf(at, "the type of what this pattern matches isn't known here, so its field %s can't be found: write its type where it is declared", name)
						break
					}
					ft = c.unknownf(at, "%s has no field %s", show(t), name)
				default:
					ft = c.unknownf(at, "%s has no field %s", show(t), name)
				}
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
	if d, ok := c.opaqueTo(x.top.fn.Module, t); ok {
		c.typeErrorf(at, "%s", c.opaqueMessage(d, "variants"))
	}
	if k != nil && k.decl != nil {
		// The variant of the matched type with that name, whichever module
		// the name was found in.
		found := false
		byName := p.Variant.Type == nil // a name several imported modules declare
		for _, v := range k.decl.Variants {
			if v.Name == p.Variant.Name {
				p.Variant, found = v, true
				if byName {
					c.use(at, v)
				}
			}
		}
		if !found && p.Variant.Name == "Uncertain" && k.uncertain {
			p.Variant, found = c.ts.uncert, true
		}
		if v := c.freshVariant(p.Variant.Name, k); !found && v != nil {
			p.Variant, found = v, true
		}
		if !found {
			x.bindAnyArgs(&p, c.unknownf(at, "%s isn't a variant of %s, the type of what is matched", p.Variant.Name, show(t)), at)
			return p
		}
	}
	v := p.Variant
	if v.Type == nil {
		x.bindAnyArgs(&p, c.unknownf(at, "internal error: variant %s has no type", v.Name), at)
		return p
	}
	var env typeEnv
	if k != nil && k.decl == v.Type {
		env = envOf(k)
	} else if v == c.ts.uncert || c.freshVariant(v.Name, k) == v {
		env = typeEnv{}
	} else {
		vt, e := c.declType(v.Type)
		env = e
		x.expect(at, t, vt)
	}
	next := 0
	for i, sub := range p.Args {
		var ft typ
		if p.Names[i] != "" {
			for _, f := range v.Fields {
				if f.Name == p.Names[i] {
					ft = c.fromExpr(f.Type, env, false)
				}
			}
			if ft == nil {
				ft = c.unknownf(at, "%s has no field %s", v.Name, p.Names[i])
			}
		} else {
			if next < len(v.Fields) {
				ft = c.fromExpr(v.Fields[next].Type, env, false)
			} else {
				ft = c.unknownf(at, "%s holds %d values, and the pattern gives more", v.Name, len(v.Fields))
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

// bindAnyArgs binds a variant pattern's arguments to t, the unknown type
// of an error already reported.
func (x *fnCheck) bindAnyArgs(p *ir.PVariant, t typ, at ir.Pos) {
	for i := range p.Args {
		p.Args[i] = x.bindPattern(p.Args[i], t, at)
	}
}

// ---------------------------------------------------------------- effects as they are found

// effect records an effect where it is found: in the outermost function, and
// in each lambda it is written inside, whose type may have to state it.
func (x *fnCheck) effect(e string) {
	for y := x; y.outer != nil; y = y.outer {
		addEffect(x.c.ts.lambdaEffects, y.fn, e)
	}
	addEffect(x.c.ts.effects, x.top.fn, e)
}

func addEffect(into map[*ir.Func]map[string]bool, f *ir.Func, e string) {
	set := into[f]
	if set == nil {
		set = map[string]bool{}
		into[f] = set
	}
	set[e] = true
}

// callee records a function whose effects count here, as effect does.
func (x *fnCheck) callee(f *ir.Func) {
	for y := x; y.outer != nil; y = y.outer {
		x.c.ts.lambdaCallees[y.fn] = append(x.c.ts.lambdaCallees[y.fn], f)
	}
	x.c.ts.callees[x.top.fn] = append(x.c.ts.callees[x.top.fn], f)
}
