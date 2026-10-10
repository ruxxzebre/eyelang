package interp

import (
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	"eyelang/ir"
)

// Interp runs one program. It isn't safe for concurrent use.
type Interp struct {
	prog     *ir.Program
	out      io.Writer // print
	log      io.Writer // log.info and friends
	pre      preludeVariants
	globals  map[*ir.Global]Value
	initing  map[*ir.Global]bool
	builtins map[string]*BuiltinFn
	variants map[string]*ir.Variant // by qualified name, for decoding
	entities map[string]*ir.Entity
	jobs     map[string]*ir.Job
	rt       *runtime
	run      *jobRun      // the job run being executed, if any
	call     string       // what the waiting call being recorded calls, for its step
	scope    *viewScope   // while a view is evaluated for one entity
	want     *ir.TypeExpr // while a call runs: the type its value is wanted as (ir.Call.Want)
}

// New prepares to run prog, printing to out and logging to log.
func New(prog *ir.Program, out, log io.Writer) *Interp {
	in := &Interp{
		prog: prog, out: out, log: log,
		globals: map[*ir.Global]Value{}, initing: map[*ir.Global]bool{},
		variants: map[string]*ir.Variant{}, entities: map[string]*ir.Entity{}, jobs: map[string]*ir.Job{},
	}
	in.builtins = builtinTable()
	for _, m := range prog.Modules {
		for _, t := range m.Types {
			for _, v := range t.Variants {
				in.variants[variantName(v)] = v
			}
		}
		for _, e := range m.Entities {
			in.entities[m.Path+"."+e.Name] = e
		}
		for _, j := range m.Jobs {
			in.jobs[m.Path+"."+j.Name] = j
		}
	}
	p := prog.Prelude
	in.pre = preludeVariants{some: p.Variants["Some"], none: p.Variants["None"], ok: p.Variants["Ok"], err: p.Variants["Err"]}
	in.rt = newRuntime(in)
	return in
}

type frame struct {
	fn    *ir.Func
	slots []Value
	self  Value
}

// Control flow that leaves an expression early.
type (
	returnSignal   struct{ v Value }
	breakSignal    struct{}
	continueSignal struct{}
	parkSignal     struct{} // a job run waits at a durable step
)

// Fault is a run-time fault: it ends the program, the job's run or the test.
type Fault struct {
	Pos ir.Pos
	Msg string
	// Kind is the prelude's Fault variant a run that ends this way shows
	// (Unavailable, DeadlinePassed, …), with Arg its text; empty is
	// Unreachable(Msg).
	Kind, Arg string
}

func (f *Fault) Error() string { return f.Pos.String() + ": " + f.Msg }

func (in *Interp) faultf(at ir.Pos, format string, args ...any) {
	panic(&Fault{Pos: at, Msg: fmt.Sprintf(format, args...)})
}

// ---------------------------------------------------------------- calls

// argVal is an evaluated argument.
type argVal struct {
	name string
	v    Value
}

func (in *Interp) evalArgs(f *frame, args []ir.Arg) []argVal {
	out := make([]argVal, len(args))
	for i, a := range args {
		out[i] = argVal{name: a.Name, v: in.eval(f, a.Value)}
	}
	return out
}

func split(args []argVal) ([]Value, map[string]Value) {
	var pos []Value
	var named map[string]Value
	for _, a := range args {
		if a.name == "" {
			pos = append(pos, a.v)
			continue
		}
		if named == nil {
			named = map[string]Value{}
		}
		named[a.name] = a.v
	}
	return pos, named
}

// callFunc runs fn with self and args. It returns the result, the receiver
// as the function left it, and the final value of the parameter each
// argument was bound to, for `mut` arguments.
func (in *Interp) callFunc(fn *ir.Func, self Value, captured []Value, args []argVal, at ir.Pos) (Value, Value, []Value) {
	if fn.Body == nil {
		if fn.Host != "" {
			return in.callHost(fn, args, at), nil, make([]Value, len(args))
		}
		in.faultf(at, "%s has no body: extern functions arrive with task 3.6", fn.Name)
	}
	fr := &frame{fn: fn, slots: make([]Value, fn.Slots), self: self}
	for i, c := range fn.Captures {
		fr.slots[c.Slot] = captured[i]
	}
	paramOf := in.bind(fn, fr, args, at)
	result := in.runBody(fr)
	final := make([]Value, len(args))
	for i, p := range paramOf {
		if p >= 0 {
			final[i] = fr.slots[fn.Params[p].Slot]
		}
	}
	return result, fr.self, final
}

// bind puts arguments into the callee's parameter slots and fills in
// defaults. It returns, for each argument, the index of its parameter.
func (in *Interp) bind(fn *ir.Func, fr *frame, args []argVal, at ir.Pos) []int {
	bound := make([]bool, len(fn.Params))
	paramOf := make([]int, len(args))
	next := 0
	for i, a := range args {
		p := -1
		if a.name != "" {
			for j, param := range fn.Params {
				if param.Name == a.name {
					p = j
				}
			}
			if p < 0 {
				in.faultf(at, "%s has no parameter named %s", fn.Name, a.name)
			}
		} else {
			for next < len(fn.Params) && bound[next] {
				next++
			}
			if next >= len(fn.Params) {
				in.faultf(at, "too many arguments to %s: it takes %d", fn.Name, len(fn.Params))
			}
			p = next
		}
		if bound[p] {
			in.faultf(at, "%s is given twice", fn.Params[p].Name)
		}
		bound[p] = true
		fr.slots[fn.Params[p].Slot] = a.v
		paramOf[i] = p
	}
	for j, param := range fn.Params {
		if bound[j] {
			continue
		}
		if param.Default == nil {
			in.faultf(at, "%s needs an argument for %s", fn.Name, param.Name)
		}
		fr.slots[param.Slot] = in.eval(fr, param.Default)
	}
	return paramOf
}

func (in *Interp) runBody(fr *frame) (v Value) {
	defer func() {
		if r := recover(); r != nil {
			if rs, ok := r.(returnSignal); ok {
				v = rs.v
				return
			}
			panic(r)
		}
	}()
	return in.eval(fr, fr.fn.Body)
}

// apply calls a function value.
func (in *Interp) apply(callee Value, args []argVal, at ir.Pos) (Value, []Value) {
	switch c := callee.(type) {
	case *Closure:
		r, _, final := in.callFunc(c.Fn, nil, c.Captured, args, at)
		return r, final
	case *BuiltinFn:
		pos, named := split(args)
		return c.Fn(in, pos, named, at), nil
	case *constructor:
		return in.construct(c.V, args, at), nil
	case *entityVal:
		pos, _ := split(args)
		if len(pos) != 1 {
			in.faultf(at, "%s(…) takes the entity's identity", c.E.Name)
		}
		return &EntityRef{Entity: c.E, ID: pos[0]}, nil
	case *OpVal:
		// A program that serves the contract calls itself, unless the
		// service's URL is configured: then the service is elsewhere.
		_, elsewhere := serviceURL(c.Op.Contract.Module.Path)
		if c.Op.Impl != nil && in.rt.fakes[c.Op.Contract] == nil && !elsewhere {
			return in.rt.serve(c.Op, args, at), nil
		}
		pos, named := split(args)
		return in.rt.callOp(c.Op, pos, named, at), nil
	case *JobVal:
		in.faultf(at, "a job isn't called; start it with %s.start(…)", c.Job.Name)
	}
	in.faultf(at, "%s isn't a function", kindName(callee))
	return nil, nil
}

// constructor is a variant with fields, used as a function.
type constructor struct{ V *ir.Variant }

// entityVal is an entity type in an expression: World(id), World.all.
type entityVal struct{ E *ir.Entity }

func (in *Interp) construct(v *ir.Variant, args []argVal, at ir.Pos) Value {
	vals := make([]Value, len(v.Fields))
	set := make([]bool, len(v.Fields))
	next := 0
	for _, a := range args {
		i := -1
		if a.name != "" {
			for j, f := range v.Fields {
				if f.Name == a.name {
					i = j
				}
			}
			if i < 0 {
				in.faultf(at, "%s has no field %s", v.Name, a.name)
			}
		} else {
			for next < len(v.Fields) && set[next] {
				next++
			}
			if next >= len(v.Fields) {
				in.faultf(at, "too many values for %s: it holds %d", v.Name, len(v.Fields))
			}
			i = next
		}
		vals[i] = a.v
		set[i] = true
	}
	for i, ok := range set {
		if !ok {
			in.faultf(at, "%s needs a value for its field %d", v.Name, i+1)
		}
	}
	return &Variant{V: v, Vals: vals}
}

// ---------------------------------------------------------------- expressions

func (in *Interp) eval(f *frame, e ir.Expr) Value {
	switch x := e.(type) {
	case *ir.Lit:
		return in.lit(x)
	case *ir.InZone:
		return in.inZone(f, x)
	case *ir.EmptyBraces:
		return Empty{}
	case *ir.Local:
		return f.slots[x.Slot]
	case *ir.Self:
		return f.self
	case *ir.SelfField:
		return in.field(f.self, x.Name, x.Pos)
	case *ir.Ref:
		return in.ref(x)
	case *ir.Call:
		return in.evalCall(f, x)
	case *ir.MethodCall:
		return in.evalMethod(f, x)
	case *ir.Waiting:
		return in.durable(f, x)
	case *ir.Field:
		return in.field(in.eval(f, x.Value), x.Name, x.Pos)
	case *ir.Index:
		return in.index(in.eval(f, x.Value), in.eval(f, x.Index), x.Pos)
	case *ir.Unary:
		return in.unary(x.Op, in.eval(f, x.X), x.Pos)
	case *ir.Binary:
		switch x.Op {
		case "&&":
			if !in.truth(in.eval(f, x.L), x.Pos) {
				return false
			}
			return in.truth(in.eval(f, x.R), x.Pos)
		case "||":
			if in.truth(in.eval(f, x.L), x.Pos) {
				return true
			}
			return in.truth(in.eval(f, x.R), x.Pos)
		}
		return in.binary(x.Op, in.eval(f, x.L), in.eval(f, x.R), x.Pos)
	case *ir.If:
		if in.truth(in.eval(f, x.Cond), x.Pos) {
			return in.eval(f, x.Then)
		}
		if x.Else != nil {
			return in.eval(f, x.Else)
		}
		return ir.Unit{}
	case *ir.Match:
		v := in.eval(f, x.Value)
		for _, arm := range x.Arms {
			if in.match(f, arm.Pattern, v) && (arm.Guard == nil || in.truth(in.eval(f, arm.Guard), x.Pos)) {
				return in.eval(f, arm.Body)
			}
		}
		in.faultf(x.Pos, "no arm matches %s", show(v))
	case *ir.Block:
		var v Value = ir.Unit{}
		for _, s := range x.Exprs {
			v = in.eval(f, s)
		}
		if !x.Value {
			return ir.Unit{}
		}
		return v
	case *ir.Let:
		v := in.eval(f, x.Value)
		if !in.match(f, x.Pattern, v) {
			if x.Else == nil {
				in.faultf(x.Pos, "%s doesn't match the pattern", show(v))
			}
			in.eval(f, x.Else)
			in.faultf(x.Pos, "the else block of a let must leave with return, break or continue")
		}
		return ir.Unit{}
	case *ir.Assign:
		v := in.eval(f, x.Value)
		if x.Op != "=" {
			v = in.binary(strings.TrimSuffix(x.Op, "="), in.readPlace(f, x.Target), v, x.Pos)
		}
		in.writePlace(f, x.Target, v)
		return ir.Unit{}
	case *ir.For:
		in.forLoop(f, x)
		return ir.Unit{}
	case *ir.While:
		for in.truth(in.eval(f, x.Cond), x.Pos) {
			if in.loopBody(f, x.Body) {
				break
			}
		}
		return ir.Unit{}
	case *ir.Break:
		panic(breakSignal{})
	case *ir.Continue:
		panic(continueSignal{})
	case *ir.Return:
		var v Value = ir.Unit{}
		if x.Value != nil {
			v = in.eval(f, x.Value)
		}
		panic(returnSignal{v})
	case *ir.Lambda:
		c := &Closure{Fn: x.Func}
		for _, cap := range x.Func.Captures {
			c.Captured = append(c.Captured, in.eval(f, cap.From))
		}
		return c
	case *ir.Record:
		r := &Record{}
		for i, n := range x.Names {
			r.Names = append(r.Names, n)
			r.Vals = append(r.Vals, in.eval(f, x.Values[i]))
		}
		return r
	case *ir.RecordUpdate:
		base := in.eval(f, x.Base)
		r, ok := base.(*Record)
		if !ok {
			in.faultf(x.Pos, "`with` updates a record, and this is %s", kindName(base))
		}
		for i, n := range x.Names {
			if _, has := r.get(n); !has {
				in.faultf(x.Pos, "the record has no field %s", n)
			}
			r = r.with(n, in.eval(f, x.Values[i]))
		}
		return r
	case *ir.List:
		l := &List{}
		for _, e := range x.Elems {
			l.Elems = append(l.Elems, in.eval(f, e))
		}
		return l
	case *ir.Map:
		var keys, vals []Value
		for i := range x.Keys {
			keys = append(keys, in.eval(f, x.Keys[i]))
			vals = append(vals, in.eval(f, x.Values[i]))
		}
		return newMap(keys, vals)
	case *ir.Tuple:
		t := &Tuple{}
		for _, e := range x.Elems {
			t.Elems = append(t.Elems, in.eval(f, e))
		}
		return t
	case *ir.Interpolate:
		var b strings.Builder
		for _, p := range x.Parts {
			b.WriteString(show(in.eval(f, p)))
		}
		return b.String()
	case *ir.Try:
		return in.try(in.eval(f, x.X), x)
	case *ir.Default:
		v := in.eval(f, x.X)
		if s, ok := isVariant(v, "Some"); ok {
			return s.Vals[0]
		}
		if o, ok := isVariant(v, "Ok"); ok {
			return o.Vals[0]
		}
		return in.eval(f, x.Or)
	case *ir.Range:
		r := &RangeVal{Inclusive: x.Inclusive}
		if x.Start != nil {
			r.Start = in.eval(f, x.Start)
		}
		if x.End != nil {
			r.End = in.eval(f, x.End)
		}
		return r
	case *ir.With:
		h, ok := in.eval(f, x.Value).(handler)
		if !ok {
			in.faultf(x.Pos, "with takes a handler, such as clock(…) or fakes(…)")
		}
		h.apply(in, x.Pos)
		return ir.Unit{}
	}
	in.faultf(e.Position(), "the interpreter can't run %T yet", e)
	return nil
}

// RangeVal is a range of integers: 0..n, 0..=n, or open at either end in a slice.
type RangeVal struct {
	Start, End Value
	Inclusive  bool
}

// inZone resolves a time whose zone is a value.
func (in *Interp) inZone(f *frame, x *ir.InZone) Value {
	zone, ok := in.eval(f, x.Zone).(string)
	if !ok {
		in.faultf(x.Pos, "a time's zone is a Zone, such as \"Europe/Dublin\"")
	}
	v := x.Instant
	v.Zone = zone
	return in.instant(v, x.Pos)
}

func (in *Interp) lit(x *ir.Lit) Value {
	switch v := x.Value.(type) {
	case *big.Int:
		return normInt(v)
	case ir.Instant:
		return in.instant(v, x.Pos)
	}
	return x.Value
}

// instant resolves a date and time in a zone, or, without a date, the next
// time that time of day comes round.
func (in *Interp) instant(v ir.Instant, at ir.Pos) time.Time {
	loc, err := loadLocation(v.Zone)
	if err != nil {
		in.faultf(at, "unknown time zone %q", v.Zone)
	}
	if v.Date == (ir.Date{}) {
		now := in.rt.now.In(loc)
		t := time.Date(now.Year(), now.Month(), now.Day(), v.Time.Hour, v.Time.Minute, v.Time.Second, 0, loc)
		if !t.After(now) {
			t = t.AddDate(0, 0, 1)
		}
		return t
	}
	return time.Date(v.Date.Year, time.Month(v.Date.Month), v.Date.Day, v.Time.Hour, v.Time.Minute, v.Time.Second, 0, loc)
}

func (in *Interp) ref(x *ir.Ref) Value {
	switch d := x.Decl.(type) {
	case *ir.Func:
		return &Closure{Fn: d}
	case *ir.Global:
		return in.global(d, x.Pos)
	case *ir.Variant:
		if len(d.Fields) == 0 {
			return &Variant{V: d}
		}
		return &constructor{V: d}
	case *ir.Entity:
		return &entityVal{E: d}
	case *ir.Job:
		return &JobVal{Job: d}
	case *ir.View:
		return &ViewVal{View: d}
	case *ir.Op:
		return &OpVal{Op: d}
	case *ir.Event:
		return &EventVal{Event: d}
	case *ir.Module:
		return &ModuleVal{Module: d}
	case *ir.Type:
		return &TypeVal{Type: d}
	case ir.Builtin:
		if b, ok := in.builtins[d.Name]; ok {
			return b
		}
		in.faultf(x.Pos, "%s isn't available in the interpreter yet", d.Name)
	}
	in.faultf(x.Pos, "can't use %T as a value", x.Decl)
	return nil
}

// global returns a module-level let, computing it the first time.
func (in *Interp) global(g *ir.Global, at ir.Pos) Value {
	if v, ok := in.globals[g]; ok {
		return v
	}
	if in.initing[g] {
		in.faultf(at, "%s is defined in terms of itself", g.Name)
	}
	in.initing[g] = true
	v, _, _ := in.callFunc(g.Init, nil, nil, nil, at)
	in.globals[g] = v
	return v
}

// wanted is the type a call's value is wanted as: its result type, as the
// checker inferred it (ir.Call.Expected), with Result(T, E) read as T, since
// a host function such as ai.generate makes the Ok value and its errors are
// its own.
func wanted(c *ir.Call) *ir.TypeExpr {
	t := c.Expected
	if t != nil && t.Name == "Result" && len(t.Args) == 2 {
		return t.Args[0]
	}
	return t
}

func (in *Interp) evalCall(f *frame, c *ir.Call) Value {
	callee := in.eval(f, c.Func)
	if b, ok := callee.(*BuiltinFn); ok && b.Name == "expect" && len(c.Args) == 1 {
		return in.expect(f, c)
	}
	args := in.evalArgs(f, c.Args)
	prev := in.want
	in.want = wanted(c)
	result, final := in.apply(callee, args, c.Pos)
	in.want = prev
	for i, a := range c.Args {
		if a.Mut && final != nil && final[i] != nil {
			in.writePlace(f, a.Place, final[i])
		}
	}
	return result
}

// expect fails the test when its condition is false, showing both sides of
// a comparison.
func (in *Interp) expect(f *frame, c *ir.Call) Value {
	arg := c.Args[0].Value
	src := sourceOf(arg)
	if b, ok := arg.(*ir.Binary); ok && (b.Op == "==" || b.Op == "!=") {
		l, r := in.eval(f, b.L), in.eval(f, b.R)
		if equal(l, r) == (b.Op == "==") {
			return ir.Unit{}
		}
		in.faultf(c.Pos, "expected %s, but %s is %s", src, sourceOf(b.L), showNested(l))
	}
	if !in.truth(in.eval(f, arg), c.Pos) {
		in.faultf(c.Pos, "expected %s", src)
	}
	return ir.Unit{}
}

func showNested(v Value) string {
	var b strings.Builder
	writeShow(&b, v, true)
	return b.String()
}

// sourceOf returns the source text of an expression, for messages.
func sourceOf(e ir.Expr) string {
	if t := e.Position().Text(); t != "" {
		return t
	}
	return "it"
}

func (in *Interp) evalMethod(f *frame, m *ir.MethodCall) Value {
	recv := in.eval(f, m.Recv)
	args := in.evalArgs(f, m.Args)
	if _, isSelf := m.Recv.(*ir.Self); isSelf && f.fn.Entity != nil {
		// An entity method calling another of the entity's methods, in the
		// same transaction, on the same state.
		if meth, ok := f.fn.Entity.Methods[m.Name]; ok {
			r, self, _ := in.callFunc(meth, f.self, nil, args, m.Pos)
			f.self = self
			return r
		}
	}
	result, newRecv := in.method(recv, m.Name, args, m.Pos, false)
	if newRecv != nil && m.RecvPlace != nil {
		in.writePlace(f, *m.RecvPlace, newRecv)
	}
	return result
}

// method calls a method on a value. durable is set for a waiting call in a
// job. It returns the result and, for a `mut self` method, the changed
// receiver.
func (in *Interp) method(recv Value, name string, args []argVal, at ir.Pos, durable bool) (Value, Value) {
	switch r := recv.(type) {
	case *EventVal:
		if name == "publish" {
			pos, _ := split(args)
			if len(pos) != 1 {
				in.faultf(at, "publish takes the event's value")
			}
			if in.run != nil && !durable {
				in.faultf(at, "in a job, publishing is a durable step: write publish!(…)")
			}
			in.rt.publish(r.Event, pos[0])
			return ir.Unit{}, nil
		}
	case *EntityRef:
		if name == "read" {
			// Outside a job (a test, main, a transaction) a read is
			// ordinary; in a job it is a durable step.
			if !durable && in.run != nil {
				in.faultf(at, "a job reads an entity with read!()")
			}
			return in.rt.snapshot(r, at), nil
		}
		meth, ok := r.Entity.Methods[name]
		if !ok {
			in.faultf(at, "%s has no method %s", r.Entity.Name, name)
		}
		if in.run != nil && !durable {
			in.faultf(at, "in a job, an entity call is a durable step: write %s!(…)", name)
		}
		return in.rt.callEntity(r, meth, args, at), nil
	case *JobVal:
		if name == "start" {
			pos, named := split(args)
			return in.rt.start(r.Job, pos, named, at), nil
		}
	case *entityVal:
		in.faultf(at, "%s has no function %s", r.E.Name, name)
	case *Variant:
		if name == "reconcile" {
			return in.rt.reconcile(r, at), nil
		}
		if r.V.Type != nil {
			if fn, ok := r.V.Type.Methods[name]; ok {
				return in.callMethodFunc(fn, r, args, at)
			}
		}
	case *Record:
		if fn := in.recordMethod(r, name); fn != nil {
			return in.callMethodFunc(fn, r, args, at)
		}
	}
	if v, ok := in.timeMethod(recv, name, args, at); ok {
		return v, nil
	}
	if b, ok := in.builtins[name]; ok {
		pos, named := split(args)
		return b.Fn(in, append([]Value{recv}, pos...), named, at), nil
	}
	in.faultf(at, "%s has no method %s", kindName(recv), name)
	return nil, nil
}

func (in *Interp) callMethodFunc(fn *ir.Func, recv Value, args []argVal, at ir.Pos) (Value, Value) {
	if !fn.HasSelf {
		in.faultf(at, "%s is called on the type: %s.%s(…)", fn.Name, fn.Receiver.Name, fn.Name)
	}
	r, self, _ := in.callFunc(fn, recv, nil, args, at)
	if fn.MutSelf {
		return r, self
	}
	return r, nil
}

// recordMethod finds the impl method for a record by the record types whose
// fields it has. With type checking (phase 2) the static type will decide.
func (in *Interp) recordMethod(r *Record, name string) *ir.Func {
	for _, m := range in.prog.Modules {
		for _, t := range m.Types {
			if len(t.Fields) != len(r.Names) {
				continue
			}
			fn, ok := t.Methods[name]
			if !ok {
				continue
			}
			match := true
			for _, n := range t.Fields {
				if _, has := r.get(n); !has {
					match = false
				}
			}
			if match {
				return fn
			}
		}
	}
	return nil
}

func (in *Interp) field(v Value, name string, at ir.Pos) Value {
	switch x := v.(type) {
	case *Record:
		if f, ok := x.get(name); ok {
			return f
		}
	case *Variant:
		if f, ok := x.field(name); ok {
			return f
		}
	case *EntityRef:
		if name == "id" {
			return x.ID
		}
		// Tests read an entity's committed state directly.
		state := in.rt.current(x, at)
		if f, ok := state.get(name); ok {
			return f
		}
	case *entityVal:
		if name == "all" {
			return &EntitySet{Entity: x.E}
		}
	case *RangeVal:
		switch {
		case name == "start" && x.Start != nil:
			return x.Start
		case name == "end" && x.End != nil:
			return x.End
		}
	}
	in.faultf(at, "%s has no field %s", kindName(v), name)
	return nil
}

func (in *Interp) index(v, i Value, at ir.Pos) Value {
	switch x := v.(type) {
	case *List:
		if r, ok := i.(*RangeVal); ok {
			lo, hi := in.bounds(r, len(x.Elems), at)
			return &List{Elems: append([]Value(nil), x.Elems[lo:hi]...)}
		}
		n, ok := i.(int64)
		if !ok {
			in.faultf(at, "a list is indexed by an Int")
		}
		if n < 0 || n >= int64(len(x.Elems)) {
			return in.none()
		}
		return in.some(x.Elems[n])
	case *Map:
		if val, ok := x.get(i); ok {
			return in.some(val)
		}
		return in.none()
	case string:
		if r, ok := i.(*RangeVal); ok {
			lo, hi := in.bounds(r, len(x), at)
			return x[lo:hi]
		}
	}
	in.faultf(at, "%s can't be indexed", kindName(v))
	return nil
}

// bounds clamps a range to [0, n].
func (in *Interp) bounds(r *RangeVal, n int, at ir.Pos) (int, int) {
	lo, hi := 0, n
	if r.Start != nil {
		s, ok := r.Start.(int64)
		if !ok {
			in.faultf(at, "a range of Ints is needed here")
		}
		lo = int(max(0, min(s, int64(n))))
	}
	if r.End != nil {
		e, ok := r.End.(int64)
		if !ok {
			in.faultf(at, "a range of Ints is needed here")
		}
		if r.Inclusive {
			e++
		}
		hi = int(max(0, min(e, int64(n))))
	}
	return lo, max(lo, hi)
}

func (in *Interp) truth(v Value, at ir.Pos) bool {
	b, ok := v.(bool)
	if !ok {
		in.faultf(at, "a condition must be a Bool, and this is %s", kindName(v))
	}
	return b
}

func (in *Interp) unary(op string, v Value, at ir.Pos) Value {
	switch op {
	case "!":
		return !in.truth(v, at)
	case "-":
		switch x := v.(type) {
		case int64, *big.Int:
			r, _ := intArith("-", int64(0), x)
			return r
		case float64:
			return -x
		case ir.Duration:
			return -x
		}
	}
	in.faultf(at, "%s%s isn't defined", op, kindName(v))
	return nil
}

func (in *Interp) binary(op string, a, b Value, at ir.Pos) Value {
	switch op {
	case "==":
		return equal(a, b)
	case "!=":
		return !equal(a, b)
	case "<", "<=", ">", ">=":
		c, ok := compare(a, b)
		if !ok {
			in.faultf(at, "%s and %s can't be compared", kindName(a), kindName(b))
		}
		switch op {
		case "<":
			return c < 0
		case "<=":
			return c <= 0
		case ">":
			return c > 0
		}
		return c >= 0
	}
	if isInt(a) && isInt(b) {
		r, err := intArith(op, a, b)
		if err != nil {
			in.faultf(at, "%v", err)
		}
		return r
	}
	switch x := a.(type) {
	case string:
		if y, ok := b.(string); ok && op == "+" {
			return x + y
		}
	case *List:
		if y, ok := b.(*List); ok && op == "+" {
			elems := make([]Value, 0, len(x.Elems)+len(y.Elems))
			return &List{Elems: append(append(elems, x.Elems...), y.Elems...)}
		}
	case float64:
		if y, ok := b.(float64); ok {
			switch op {
			case "+":
				return x + y
			case "-":
				return x - y
			case "*":
				return x * y
			case "/":
				return x / y
			}
		}
	case ir.Duration:
		switch y := b.(type) {
		case ir.Duration:
			switch op {
			case "+":
				return x + y
			case "-":
				return x - y
			}
		case int64:
			switch op {
			case "*":
				return x * ir.Duration(y)
			case "/":
				if y != 0 {
					return x / ir.Duration(y)
				}
			}
		}
	case ir.Size:
		switch y := b.(type) {
		case ir.Size:
			switch op {
			case "+":
				return x + y
			case "-":
				return x - y
			}
		case int64:
			if op == "*" {
				return x * ir.Size(y)
			}
		}
	case time.Time:
		switch y := b.(type) {
		case ir.Duration:
			switch op {
			case "+":
				return x.Add(time.Duration(y))
			case "-":
				return x.Add(-time.Duration(y))
			}
		case time.Time:
			if op == "-" {
				return ir.Duration(x.Sub(y))
			}
		}
	case ir.Date:
		if y, ok := b.(ir.Duration); ok && (op == "+" || op == "-") {
			days := int(int64(y) / int64(24*time.Hour))
			if op == "-" {
				days = -days
			}
			t := time.Date(x.Year, time.Month(x.Month), x.Day, 0, 0, 0, 0, time.UTC).AddDate(0, 0, days)
			return ir.Date{Year: t.Year(), Month: int(t.Month()), Day: t.Day()}
		}
	}
	in.faultf(at, "%s %s %s isn't defined", kindName(a), op, kindName(b))
	return nil
}

// try is postfix `?`: it unwraps Ok and Some, and returns Err and None from
// the function, wrapping an error into the function's error type.
func (in *Interp) try(v Value, x *ir.Try) Value {
	if o, ok := isVariant(v, "Ok"); ok {
		return o.Vals[0]
	}
	if s, ok := isVariant(v, "Some"); ok {
		return s.Vals[0]
	}
	if _, ok := isVariant(v, "None"); ok {
		panic(returnSignal{v})
	}
	e, ok := isVariant(v, "Err")
	if !ok {
		in.faultf(x.Pos, "? works on a Result or an Option, and this is %s", kindName(v))
	}
	panic(returnSignal{in.errV(in.wrapError(e.Vals[0], x.Wrap))})
}

// wrapError puts an error into the variant of target whose one field holds
// the error's type, when the error isn't already of type target.
func (in *Interp) wrapError(err Value, target *ir.Type) Value {
	ev, ok := err.(*Variant)
	if target == nil || !ok || ev.V.Type == target {
		return err
	}
	for _, v := range target.Variants {
		if len(v.Fields) == 1 && v.Fields[0].Type != nil && v.Fields[0].Type.Decl == ev.V.Type {
			return &Variant{V: v, Vals: []Value{err}}
		}
	}
	return err
}

// ---------------------------------------------------------------- places

func (in *Interp) readPlace(f *frame, p ir.Place) Value {
	var v Value
	if p.Self {
		v = f.self
	} else {
		v = f.slots[p.Slot]
	}
	for _, name := range p.Path {
		v = in.field(v, name, p.Pos)
	}
	return v
}

func (in *Interp) writePlace(f *frame, p ir.Place, v Value) {
	if p.Self {
		f.self = in.setPath(f.self, p.Path, v, p.Pos)
		return
	}
	f.slots[p.Slot] = in.setPath(f.slots[p.Slot], p.Path, v, p.Pos)
}

// setPath returns base with the field at path replaced by v.
func (in *Interp) setPath(base Value, path []string, v Value, at ir.Pos) Value {
	if len(path) == 0 {
		return v
	}
	r, ok := base.(*Record)
	if !ok {
		in.faultf(at, "only a record's fields can be assigned, and this is %s", kindName(base))
	}
	old, has := r.get(path[0])
	if !has {
		in.faultf(at, "the record has no field %s", path[0])
	}
	return r.with(path[0], in.setPath(old, path[1:], v, at))
}

// ---------------------------------------------------------------- loops

func (in *Interp) forLoop(f *frame, x *ir.For) {
	iter := in.eval(f, x.Iter)
	each := func(v Value) bool {
		if !in.match(f, x.Pattern, v) {
			in.faultf(x.Pos, "%s doesn't match the loop's pattern", show(v))
		}
		return in.loopBody(f, x.Body)
	}
	switch it := iter.(type) {
	case *List:
		for _, v := range it.Elems {
			if each(v) {
				return
			}
		}
	case *Set:
		for _, v := range it.Elems {
			if each(v) {
				return
			}
		}
	case *Map:
		for i, k := range it.Keys {
			if each(&Tuple{Elems: []Value{k, it.Vals[i]}}) {
				return
			}
		}
	case *RangeVal:
		lo, ok1 := it.Start.(int64)
		hi, ok2 := it.End.(int64)
		if !ok1 || !ok2 {
			in.faultf(x.Pos, "a for loop over a range needs Int bounds")
		}
		if it.Inclusive {
			hi++
		}
		for i := lo; i < hi; i++ {
			if each(i) {
				return
			}
		}
	case *EntitySet:
		for _, v := range in.rt.all(it.Entity, x.Pos).Elems {
			if each(v) {
				return
			}
		}
	case Empty:
	default:
		in.faultf(x.Pos, "a for loop can't walk %s", kindName(iter))
	}
}

// loopBody runs one iteration and reports whether the loop should stop.
func (in *Interp) loopBody(f *frame, body ir.Expr) (stop bool) {
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case breakSignal:
				stop = true
			case continueSignal:
				stop = false
			default:
				panic(r)
			}
		}
	}()
	in.eval(f, body)
	return false
}

// ---------------------------------------------------------------- patterns

func (in *Interp) match(f *frame, p ir.Pattern, v Value) bool {
	switch x := p.(type) {
	case ir.PWild:
		return true
	case ir.PBind:
		f.slots[x.Slot] = v
		return true
	case ir.PLit:
		lit := x.Value
		if b, ok := lit.(*big.Int); ok {
			lit = normInt(b)
		}
		return equal(lit, v)
	case ir.PEmpty:
		return keyOf(v) == "{}"
	case ir.PVariant:
		vv, ok := v.(*Variant)
		if !ok {
			return false
		}
		if x.Variant.Type != nil {
			if vv.V != x.Variant {
				return false
			}
		} else if vv.V.Name != x.Variant.Name {
			return false
		}
		next := 0
		for i, sub := range x.Args {
			var field Value
			if x.Names[i] != "" {
				f, ok := vv.field(x.Names[i])
				if !ok {
					return false
				}
				field = f
			} else {
				if next >= len(vv.Vals) {
					return false
				}
				field = vv.Vals[next]
				next++
			}
			if !in.match(f, sub, field) {
				return false
			}
		}
		if !x.Rest && len(x.Args) > 0 && len(x.Args) != len(vv.Vals) {
			return false
		}
		return true
	case ir.PRecord:
		r, ok := v.(*Record)
		if !ok {
			return false
		}
		for i, n := range x.Names {
			fv, has := r.get(n)
			if !has || !in.match(f, x.Pats[i], fv) {
				return false
			}
		}
		return x.Rest || len(x.Names) == len(r.Names)
	case ir.PTuple:
		t, ok := v.(*Tuple)
		if !ok || len(t.Elems) != len(x.Pats) {
			return false
		}
		for i, sub := range x.Pats {
			if !in.match(f, sub, t.Elems[i]) {
				return false
			}
		}
		return true
	case ir.PList:
		l, ok := v.(*List)
		if !ok {
			return false
		}
		if x.Rest < 0 {
			if len(l.Elems) != len(x.Pats) {
				return false
			}
			for i, sub := range x.Pats {
				if !in.match(f, sub, l.Elems[i]) {
					return false
				}
			}
			return true
		}
		before, after := x.Pats[:x.Rest], x.Pats[x.Rest:]
		if len(l.Elems) < len(before)+len(after) {
			return false
		}
		for i, sub := range before {
			if !in.match(f, sub, l.Elems[i]) {
				return false
			}
		}
		for i, sub := range after {
			if !in.match(f, sub, l.Elems[len(l.Elems)-len(after)+i]) {
				return false
			}
		}
		if x.RestPat != nil {
			rest := &List{Elems: append([]Value(nil), l.Elems[len(before):len(l.Elems)-len(after)]...)}
			return in.match(f, x.RestPat, rest)
		}
		return true
	case ir.PRange:
		lo, hi := x.Lo, x.Hi
		if b, ok := lo.(*big.Int); ok {
			lo = normInt(b)
		}
		if b, ok := hi.(*big.Int); ok {
			hi = normInt(b)
		}
		c1, ok1 := compare(lo, v)
		c2, ok2 := compare(v, hi)
		if !ok1 || !ok2 {
			return false
		}
		return c1 <= 0 && (c2 < 0 || x.Inclusive && c2 == 0)
	case ir.POr:
		for _, alt := range x.Alts {
			if in.match(f, alt, v) {
				return true
			}
		}
		return false
	}
	return false
}
