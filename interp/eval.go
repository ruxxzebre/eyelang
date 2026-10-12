package interp

import (
	"fmt"
	"io"
	"math"
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
	types    map[string]*ir.Type    // record types by qualified name, for decoding
	entities map[string]*ir.Entity
	jobs     map[string]*ir.Job
	rt       *runtime
	run      *jobRun      // the job run being executed, if any
	call     string       // what the waiting call being recorded calls, for its step
	scope    *viewScope   // while a view is evaluated for one entity
	want     *ir.TypeExpr // while a call runs: the type its value is wanted as (ir.Call.Want)
	session  *Session     // the REPL's session, when the program is one
	task     *taskCtx     // the task this interpreter runs, for parallelMap and friends
	g        *gil         // guards shared state while tasks run on goroutines
	budgets  []*budget    // the @budgets being kept, outermost first
	// stepSpend collects what the model calls of the step being taken
	// spent, which the step records; nil outside a step.
	stepSpend *spend
	// toolRun is set while durable calls ai.generate with tools: the call
	// records each reply and each tool's result as a step of that run.
	toolRun *jobRun
	// files are recordings held in memory, read in place of the disk's.
	files map[string][]byte
	// hosts are the functions the embedding Go program registered for
	// @host extern fns, and externs the code each extern fn called so far
	// runs (externs.go).
	hosts   *Hosts
	externs map[*ir.Func]hostFunc
	// funcs is what the interpreter works out once per function, as what
	// it does with return, break and continue (control.go); units are the
	// values of the variants without fields, made once.
	funcs *funcCache
	units map[*ir.Variant]*Variant
	// free holds frames whose calls returned, for calls to come; each
	// interpreter, a task's included, keeps its own.
	free []*frame
}

// New prepares to run prog, printing to out and logging to log.
func New(prog *ir.Program, out, log io.Writer) *Interp {
	in := &Interp{
		prog: prog, out: out, log: log,
		globals: map[*ir.Global]Value{}, initing: map[*ir.Global]bool{},
		variants: map[string]*ir.Variant{}, types: map[string]*ir.Type{}, entities: map[string]*ir.Entity{}, jobs: map[string]*ir.Job{},
		funcs: &funcCache{}, units: map[*ir.Variant]*Variant{},
	}
	in.builtins = builtinTable()
	for _, m := range prog.Modules {
		for _, t := range m.Types {
			in.types[typeName(t)] = t
			for _, v := range t.Variants {
				in.variants[variantName(v)] = v
				if len(v.Fields) == 0 {
					in.units[v] = &Variant{V: v}
				}
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
	in.pre = preludeVariants{some: p.Variants["Some"], none: p.Variants["None"], ok: p.Variants["Ok"], err: p.Variants["Err"],
		problem: p.Variants["Problem"], offset: p.Variants["Offset"], stale: p.Variants["Stale"], expired: p.Variants["Expired"]}
	in.rt = newRuntime(in)
	return in
}

type frame struct {
	fn    *ir.Func
	slots []Value
	self  Value
	// row, in an entity method, holds self: the entity's in-progress state
	// in the transaction, which every call on the entity in the call tree
	// reads and changes (SPECS.md §14).
	row *txRow
	ck  *checkpointing // a job run's own frame keeps its checkpoints
	// ctl says a return, break or continue is leaving, and ret is what the
	// return returns (control.go). info is the function's ctlInfo, once a
	// return or a loop needed it.
	ctl  ctl
	ret  Value
	info *ctlInfo
	// inline holds the slots of a function with few locals, so that a call
	// makes one allocation, not two.
	inline [4]Value
}

// newFrame makes a frame for a call of fn, reusing one whose call returned
// when it can: a call that returns gives its frame back with freeFrame,
// since nothing keeps a frame once its call has returned.
func (in *Interp) newFrame(fn *ir.Func) *frame {
	var fr *frame
	if n := len(in.free); n > 0 {
		fr = in.free[n-1]
		in.free = in.free[:n-1]
	} else {
		fr = &frame{}
	}
	fr.fn = fn
	if fn.Slots <= len(fr.inline) {
		fr.slots = fr.inline[:fn.Slots]
	} else {
		fr.slots = make([]Value, fn.Slots)
	}
	return fr
}

// maxFree is how many frames an interpreter keeps for reuse.
const maxFree = 256

// freeFrame gives back the frame of a call that returned. A call that
// leaves by a panic doesn't give its frame back.
func (in *Interp) freeFrame(fr *frame) {
	*fr = frame{}
	if len(in.free) < maxFree {
		in.free = append(in.free, fr)
	}
}

// getSelf returns the receiver: an entity method's in-progress state, or a
// `mut self` method's value.
func (f *frame) getSelf() Value {
	if f.row != nil {
		return f.row.state
	}
	return f.self
}

func (f *frame) setSelf(v Value) {
	if f.row != nil {
		f.row.state = v.(*Record)
		return
	}
	f.self = v
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

// callFunc runs fn with self and args. It returns the result and the
// receiver as the function left it.
func (in *Interp) callFunc(fn *ir.Func, self Value, captured []Value, args []argVal, at ir.Pos) (Value, Value) {
	fr := in.newFrame(fn)
	fr.self = self
	r := in.callFrame(fr, captured, args, at)
	self = fr.getSelf()
	in.freeFrame(fr)
	return r, self
}

// callOnRow calls an entity method on the entity's row in the transaction,
// which holds its state for the method and for every call that comes back
// to the entity while it runs.
func (in *Interp) callOnRow(fn *ir.Func, row *txRow, args []argVal, at ir.Pos) Value {
	fr := in.newFrame(fn)
	fr.row = row
	r := in.callFrame(fr, nil, args, at)
	in.freeFrame(fr)
	return r
}

func (in *Interp) callFrame(fr *frame, captured []Value, args []argVal, at ir.Pos) Value {
	fn := fr.fn
	if fn.Body == nil {
		return in.callHost(fn, args, at)
	}
	for i, c := range fn.Captures {
		fr.slots[c.Slot] = captured[i]
	}
	in.bind(fn, fr, args, at)
	return in.runFrame(fr)
}

// runFrame runs the body of a frame whose parameters are bound.
func (in *Interp) runFrame(fr *frame) Value {
	if fr.fn.Budget != nil {
		return in.underBudget(fr.fn, func() Value { return in.runBody(fr) })
	}
	return in.runBody(fr)
}

// callDirect runs c, a call of fn, a function with a body, without making
// a list of its arguments when it can: each positional argument goes
// straight into its parameter's slot. A `mut` argument is written back
// from the slot when the call returns.
func (in *Interp) callDirect(f *frame, c *ir.Call, fn *ir.Func, captured []Value) Value {
	if len(c.Args) > len(fn.Params) {
		return in.callArgs(f, c, &Closure{Fn: fn, Captured: captured})
	}
	for _, a := range c.Args {
		if a.Name != "" {
			return in.callArgs(f, c, &Closure{Fn: fn, Captured: captured})
		}
	}
	fr := in.newFrame(fn)
	for i, a := range c.Args {
		fr.slots[fn.Params[i].Slot] = in.eval(f, a.Value)
	}
	for i, cp := range fn.Captures {
		fr.slots[cp.Slot] = captured[i]
	}
	prev := in.want
	in.want = wanted(c)
	for _, param := range fn.Params[len(c.Args):] {
		if param.Default == nil {
			in.faultf(c.Pos, "%s needs an argument for %s", fn.Name, param.Name)
		}
		fr.slots[param.Slot] = in.eval(fr, param.Default)
	}
	result := in.runFrame(fr)
	in.want = prev
	for i, a := range c.Args {
		if a.Mut {
			if v := plain(fr.slots[fn.Params[i].Slot]); v != nil {
				in.writePlace(f, a.Place, nil, v)
			}
		}
	}
	in.freeFrame(fr)
	return result
}

// bind puts arguments into the callee's parameter slots and fills in
// defaults.
func (in *Interp) bind(fn *ir.Func, fr *frame, args []argVal, at ir.Pos) {
	in.bindTo(fn, fr, args, at, nil)
}

// bindTo binds as bind does. When paramOf isn't nil, it sets paramOf[i] to
// the index of the parameter argument i was bound to.
func (in *Interp) bindTo(fn *ir.Func, fr *frame, args []argVal, at ir.Pos, paramOf []int) {
	bound := make([]bool, len(fn.Params))
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
		if paramOf != nil {
			paramOf[i] = p
		}
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
}

// runBody runs a frame's function's body and returns what it returns. A
// return leaves by the frame's flag or, outside a statement position, by
// a panic (control.go).
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
	return returned(fr, in.eval(fr, fr.fn.Body))
}

// apply calls a function value.
func (in *Interp) apply(callee Value, args []argVal, at ir.Pos) Value {
	switch c := callee.(type) {
	case *Closure:
		r, _ := in.callFunc(c.Fn, nil, c.Captured, args, at)
		return r
	case *BuiltinFn:
		pos, named := split(args)
		return c.Fn(in, pos, named, at)
	case *constructor:
		return in.construct(c.V, args, at)
	case *entityVal:
		pos, _ := split(args)
		if len(pos) != 1 {
			in.faultf(at, "%s(…) takes the entity's identity", c.E.Name)
		}
		in.rt.crossed("entity.ref")
		return &EntityRef{Entity: c.E, ID: pos[0]}
	case *OpVal:
		in.rt.crossed("op.call")
		// A program that serves the contract calls itself, unless the
		// service's URL is configured: then the service is elsewhere.
		_, elsewhere := serviceURL(c.Op.Contract.Module.Path)
		if c.Op.Impl != nil && in.rt.fakes[c.Op.Contract] == nil && !elsewhere {
			return in.rt.serve(c.Op, args, at)
		}
		pos, named := split(args)
		return in.rt.callOp(c.Op, pos, named, at)
	case *JobVal:
		in.faultf(at, "a job isn't called; start it with %s.start(…)", c.Job.Name)
	}
	in.faultf(at, "%s isn't a function", kindName(callee))
	return nil
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

// eval evaluates an expression. It holds only the cases that nest deeply,
// such as blocks, ifs, lets and calls, and keeps their work in functions of
// their own, so that its stack frame stays small: every level of Eyelang code
// that runs or waits costs one eval frame on the task's goroutine, and a
// waiting task's memory is mostly its stack (interp/manytasks_test.go). The
// other cases are in evalMore.
func (in *Interp) eval(f *frame, e ir.Expr) Value {
	if f.ck != nil && f.ck.stmt == e {
		// A statement of a job's body that holds statements keeps its
		// place for checkpoints (checkpoint.go).
		f.ck.stmt = nil
		if v, ok := in.stmt(f, e); ok {
			return v
		}
	}
	switch x := e.(type) {
	case *ir.Lit:
		return in.lit(x)
	case *ir.Local:
		switch v := f.slots[x.Slot].(type) {
		case unsaved:
			in.lostLocal(x)
		case *textBuf:
			return v.string()
		case *ownRec:
			// The record may be held elsewhere from now on (own.go).
			f.slots[x.Slot] = (*Record)(v)
			return (*Record)(v)
		default:
			return v
		}
	case *ir.Self:
		return f.getSelf()
	case *ir.SelfField:
		return in.field(f.getSelf(), x.Name, x.Pos)
	case *ir.Ref:
		return in.ref(x)
	case *ir.Call:
		return in.evalCall(f, x)
	case *ir.MethodCall:
		return in.evalMethod(f, x)
	case *ir.Waiting:
		return in.durable(f, x)
	case *ir.Field:
		return in.localField(f, x)
	case *ir.If:
		if in.truth(in.eval(f, x.Cond), x.Pos) {
			return in.eval(f, x.Then)
		}
		if x.Else != nil {
			return in.eval(f, x.Else)
		}
		return ir.Unit{}
	case *ir.Match:
		return in.evalMatch(f, x)
	case *ir.Block:
		var v Value = ir.Unit{}
		for _, s := range x.Exprs {
			v = in.eval(f, s)
			if f.ctl != ctlNone {
				return nil
			}
		}
		if !x.Value {
			return ir.Unit{}
		}
		return v
	case *ir.Let:
		v := in.eval(f, x.Value)
		if f.ctl != ctlNone {
			return nil
		}
		if !in.match(f, x.Pattern, v) {
			in.letElse(f, x, v)
		} else if b, ok := x.Pattern.(ir.PBind); ok && fresh(x.Value) {
			own(f, b.Slot, v)
		}
		return ir.Unit{}
	}
	return in.evalMore(f, e)
}

// lostLocal faults on reading a local that a checkpoint couldn't keep.
func (in *Interp) lostLocal(x *ir.Local) {
	in.faultf(x.Pos, "%s held a value that can't be saved, which a checkpoint didn't keep, and is read after it: the checker should have refused this", x.Name)
}

// evalMatch runs the first arm whose pattern matches and whose guard holds.
func (in *Interp) evalMatch(f *frame, x *ir.Match) Value {
	v := in.eval(f, x.Value)
	if f.ctl != ctlNone {
		return nil
	}
	for _, arm := range x.Arms {
		if in.match(f, arm.Pattern, v) && (arm.Guard == nil || in.truth(in.eval(f, arm.Guard), x.Pos)) {
			return in.eval(f, arm.Body)
		}
	}
	in.faultf(x.Pos, "no arm matches %s", show(v))
	return nil
}

// letElse runs a let's else block, for a value its pattern didn't match;
// the block must leave.
func (in *Interp) letElse(f *frame, x *ir.Let, v Value) {
	if x.Else == nil {
		in.faultf(x.Pos, "%s doesn't match the pattern", show(v))
	}
	in.eval(f, x.Else)
	if f.ctl != ctlNone {
		return
	}
	in.faultf(x.Pos, "the else block of a let must leave with return, break or continue")
}

// evalMore evaluates the expressions eval leaves out.
func (in *Interp) evalMore(f *frame, e ir.Expr) Value {
	switch x := e.(type) {
	case *ir.InZone:
		return in.inZone(f, x)
	case *ir.EmptyBraces:
		return Empty{}
	case *ir.Index:
		return in.index(in.eval(f, x.Value), in.eval(f, x.Index), x.Pos)
	case *ir.Unary:
		return in.unary(x.Op, in.eval(f, x.X), x.Pos)
	case *ir.Binary:
		if len(x.Op) == 2 && x.Op[0] == x.Op[1] {
			switch x.Op[0] {
			case '&':
				if !in.truth(in.eval(f, x.L), x.Pos) {
					return false
				}
				return in.truth(in.eval(f, x.R), x.Pos)
			case '|':
				if in.truth(in.eval(f, x.L), x.Pos) {
					return true
				}
				return in.truth(in.eval(f, x.R), x.Pos)
			}
		}
		return in.binary(x.Op, in.eval(f, x.L), in.eval(f, x.R), x.Pos)
	case *ir.Assign:
		if in.appendInPlace(f, x) {
			return ir.Unit{}
		}
		if x.Op == "=" && !x.Target.Self && len(x.Target.Path) == 0 && fresh(x.Value) {
			if u, ok := x.Value.(*ir.RecordUpdate); ok {
				in.assignWith(f, x.Target.Slot, u)
			} else {
				own(f, x.Target.Slot, in.eval(f, x.Value))
			}
			return ir.Unit{}
		}
		v := in.eval(f, x.Value)
		keys := in.placeKeys(f, x.Target)
		if x.Op != "=" {
			v = in.binary(strings.TrimSuffix(x.Op, "="), in.readPlace(f, x.Target, keys), v, x.Pos)
		}
		in.writePlace(f, x.Target, keys, v)
		return ir.Unit{}
	case *ir.For:
		in.forLoop(f, x)
		return ir.Unit{}
	case *ir.While:
		catch := in.ctlOf(f).catch[x]
		for in.truth(in.eval(f, x.Cond), x.Pos) {
			if in.loopBody(f, x.Body, catch) {
				break
			}
		}
		return ir.Unit{}
	case *ir.Break:
		return in.leave(f, x, ctlBreak, nil)
	case *ir.Continue:
		return in.leave(f, x, ctlContinue, nil)
	case *ir.Return:
		var v Value = ir.Unit{}
		if x.Value != nil {
			v = in.eval(f, x.Value)
			if f.ctl != ctlNone {
				return nil
			}
		}
		return in.leave(f, x, ctlReturn, v)
	case *ir.Lambda:
		c := &Closure{Fn: x.Func}
		for _, cap := range x.Func.Captures {
			c.Captured = append(c.Captured, in.eval(f, cap.From))
		}
		return c
	case *ir.Record:
		r := &Record{Type: x.Type}
		for i, n := range x.Names {
			r.Names = append(r.Names, n)
			r.Vals = append(r.Vals, in.eval(f, x.Values[i]))
		}
		if x.Type != nil && x.Type.Alias != nil && len(x.Type.Alias.Fields) == len(r.Names) {
			// Fields in the order the type declares them, which is the order
			// a derived Ord compares them in.
			sorted := &Record{Type: x.Type}
			for _, name := range x.Type.Alias.Fields {
				if v, ok := r.get(name); ok {
					sorted.Names = append(sorted.Names, name)
					sorted.Vals = append(sorted.Vals, v)
				}
			}
			if len(sorted.Names) == len(r.Names) {
				r = sorted
			}
		}
		return r
	case *ir.RecordUpdate:
		base := in.eval(f, x.Base)
		r, ok := base.(*Record)
		if !ok {
			in.faultf(x.Pos, "`with` updates a record, and this is %s", kindName(base))
		}
		// One copy of the values for all the fields set; the names are
		// shared.
		out := r.copied()
		for i, n := range x.Names {
			j := r.field(n)
			if j < 0 {
				in.faultf(x.Pos, "the record has no field %s", n)
			}
			out.Vals[j] = in.eval(f, x.Values[i])
		}
		return out
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
			b.WriteString(in.display(in.eval(f, p)))
		}
		return b.String()
	case *ir.Try:
		return in.try(f, in.eval(f, x.X), x)
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
		return in.closure(d)
	case *ir.Global:
		return in.global(d, x.Pos)
	case *ir.Variant:
		if len(d.Fields) == 0 {
			if u, ok := in.units[d]; ok {
				return u
			}
			return &Variant{V: d}
		}
		return &constructor{V: d}
	case *ir.Entity:
		return &entityVal{E: d}
	case *ir.Job:
		return &JobVal{Job: d}
	case *ir.View:
		return in.rt.viewMembers(d, x.Pos)
	case *ir.Op:
		return &OpVal{Op: d}
	case *ir.Event:
		return &EventVal{Event: d}
	case *ir.Module:
		return &ModuleVal{Module: d}
	case *ir.Type:
		return &TypeVal{Type: d}
	case ir.EnumFn:
		return enumFn(d)
	case ir.Builtin:
		if b, ok := in.builtins[d.Name]; ok {
			return b
		}
		in.faultf(x.Pos, "%s isn't available in the interpreter yet", d.Name)
	}
	in.faultf(x.Pos, "can't use %T as a value", x.Decl)
	return nil
}

// evalGlobalFunc computes a function with no parameters that the checker
// made for a declaration's value, such as a job's @deadline.
func (in *Interp) evalGlobalFunc(fn *ir.Func) Value {
	prev := in.run
	in.run = nil
	defer func() { in.run = prev }()
	v, _ := in.callFunc(fn, nil, nil, nil, fn.Pos)
	return v
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
	v, _ := in.callFunc(g.Init, nil, nil, nil, at)
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
	if r, ok := c.Func.(*ir.Ref); ok {
		switch d := r.Decl.(type) {
		case *ir.Func:
			if d.Body != nil {
				// A call of a named function: no closure to make.
				return in.callDirect(f, c, d, nil)
			}
		case ir.Builtin:
			if len(c.Args) > 0 && fixedMethods[d.Name] {
				// `wrappingAdd(a, b)` and `a |> wrappingAdd(b)`: the first
				// argument's type gives the width.
				return in.fixedCall(d.Name, c.Args[0].Value.Ty(), in.evalArgs(f, c.Args), c.Pos)
			}
		}
	}
	callee := in.eval(f, c.Func)
	switch cl := callee.(type) {
	case *Closure:
		if cl.Fn.Body != nil {
			return in.callDirect(f, c, cl.Fn, cl.Captured)
		}
	case *BuiltinFn:
		if cl.Name == "expect" && len(c.Args) == 1 {
			return in.expect(f, c)
		}
	}
	return in.callArgs(f, c, callee)
}

// callArgs runs c, a call of callee, with its arguments in a list.
func (in *Interp) callArgs(f *frame, c *ir.Call, callee Value) Value {
	args := in.evalArgs(f, c.Args)
	prev := in.want
	in.want = wanted(c)
	cl, ok := callee.(*Closure)
	if !ok || cl.Fn.Body == nil {
		result := in.apply(callee, args, c.Pos)
		in.want = prev
		return result
	}
	fn := cl.Fn
	fr := in.newFrame(fn)
	for i, cp := range fn.Captures {
		fr.slots[cp.Slot] = cl.Captured[i]
	}
	paramOf := make([]int, len(args))
	in.bindTo(fn, fr, args, c.Pos, paramOf)
	result := in.runFrame(fr)
	in.want = prev
	for i, a := range c.Args {
		if a.Mut {
			if v := plain(fr.slots[fn.Params[paramOf[i]].Slot]); v != nil {
				in.writePlace(f, a.Place, nil, v)
			}
		}
	}
	in.freeFrame(fr)
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
		in.faultf(c.Pos, "expected %s, but %s is %s", src, sourceOf(b.L), in.displayNested(l))
	}
	if !in.truth(in.eval(f, arg), c.Pos) {
		in.faultf(c.Pos, "expected %s", src)
	}
	return ir.Unit{}
}

// sourceOf returns the source text of an expression, for messages.
func sourceOf(e ir.Expr) string {
	if t := e.Position().Text(); t != "" {
		return t
	}
	return "it"
}

func (in *Interp) evalMethod(f *frame, m *ir.MethodCall) Value {
	switch t := m.Target.(type) {
	case ir.FieldCall:
		return in.evalCall(f, t.Call) // a field that holds a function
	case ir.FuncCall:
		return in.evalCall(f, t.Call) // a function called on its first argument
	}
	recv := in.eval(f, m.Recv)
	if m.Name == "read" && entitySet(m.Recv) {
		return in.snapshots(recv, m.Pos)
	}
	args := in.evalArgs(f, m.Args)
	if d, ok := m.Target.(ir.EnumFn); ok && d.Name == "index" {
		// The variant's place in T.variants() (SPECS.md §11).
		v, ok := recv.(*Variant)
		if !ok {
			in.faultf(m.Pos, "index() takes a variant, and this is %s", kindName(recv))
		}
		return int64(variantIndex(v.V))
	}
	if b, ok := m.Target.(ir.Builtin); ok && fixedMethods[b.Name] {
		return in.fixedCall(b.Name, m.Recv.Ty(), append([]argVal{{v: recv}}, args...), m.Pos)
	}
	if _, isSelf := m.Recv.(*ir.Self); isSelf && f.fn.Entity != nil {
		// An entity method calling another of the entity's methods, in the
		// same transaction, on the same state.
		if meth, ok := f.fn.Entity.Methods[m.Name]; ok {
			if f.row != nil {
				return in.callOnRow(meth, f.row, args, m.Pos)
			}
			r, self := in.callFunc(meth, f.self, nil, args, m.Pos)
			f.self = self
			return r
		}
		if m.Name == "end" {
			in.rt.end(args, m.Pos)
			return ir.Unit{}
		}
	}
	result, newRecv := in.method(recv, m.Name, args, m.Pos, false)
	if newRecv != nil && m.RecvPlace != nil {
		in.writePlace(f, *m.RecvPlace, nil, newRecv)
	}
	return result
}

// entitySet reports whether e is a view or E.all, whose read() gives its
// snapshots (SPECS.md §15).
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

// snapshots is what read() on a view or E.all gives: the list of its
// entities' snapshots, as they are now.
func (in *Interp) snapshots(set Value, at ir.Pos) Value {
	if s, ok := set.(*EntitySet); ok {
		return in.rt.all(s.Entity, at)
	}
	return set // a view is its list already
}

// method calls a method on a value. durable is set for a waiting call in a
// job. It returns the result and, for a `mut self` method, the changed
// receiver.
func (in *Interp) method(recv Value, name string, args []argVal, at ir.Pos, durable bool) (Value, Value) {
	switch r := recv.(type) {
	case *fakesHandler:
		if name == "calls" {
			pos, _ := split(args)
			return r.calls(in, pos, at), nil
		}
	case *EventVal:
		if name == "publish" {
			pos, _ := split(args)
			if len(pos) != 1 {
				in.faultf(at, "publish takes the event's value")
			}
			if in.run != nil && !durable {
				in.faultf(at, "in a job, publishing is a durable step: write publish!(…)")
			}
			in.rt.crossed("event.publish")
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
			in.rt.crossed("entity.read")
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
			if run := in.run; run != nil && in.rt.tx == nil {
				return in.startChild(run, r.Job, pos, named, at), nil
			}
			return in.rt.start(r.Job, pos, named, at), nil
		}
	case *RunVal:
		if name == "outcome" {
			if in.run != nil {
				in.faultf(at, "in a job, a run's outcome is read with join!(), a step that waits for it: outcome() isn't recorded, so a resumed run could see another")
			}
			return in.runOutcome(r, at), nil
		}
	case *entityVal:
		in.faultf(at, "%s has no function %s", r.E.Name, name)
	case *Variant:
		if name == "reconcile" {
			in.rt.crossed("op.reconcile")
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
	if v, ok := in.channelMethod(recv, name, args, at); ok {
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
	r, self := in.callFunc(fn, recv, nil, args, at)
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
		if name == "id" || (x.Entity.ID != nil && name == x.Entity.ID.Name) {
			return x.ID // the identity, which needs no load: an ended entity has it too
		}
		// Tests read an entity's committed state directly.
		in.rt.crossed("entity.field")
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
	case *ApprovalVal:
		switch name {
		case "what":
			return x.What
		case "by":
			return x.By
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
	case Empty:
		return in.none() // `{}` as a map: it holds nothing
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
		if r, ok := in.userNeg(v, at); ok {
			return r
		}
	}
	in.faultf(at, "%s%s isn't defined", op, kindName(v))
	return nil
}

// binOp is a binary operator the interpreter has a fast path for.
type binOp uint8

const (
	opOther binOp = iota
	opAdd
	opSub
	opMul
	opDiv
	opRem
	opEq
	opNe
	opLt
	opLe
	opGt
	opGe
)

// opOf finds the binOp an operator's text names by its bytes, without
// comparing strings.
func opOf(s string) binOp {
	switch len(s) {
	case 1:
		switch s[0] {
		case '+':
			return opAdd
		case '-':
			return opSub
		case '*':
			return opMul
		case '/':
			return opDiv
		case '%':
			return opRem
		case '<':
			return opLt
		case '>':
			return opGt
		}
	case 2:
		if s[1] != '=' {
			return opOther
		}
		switch s[0] {
		case '=':
			return opEq
		case '!':
			return opNe
		case '<':
			return opLe
		case '>':
			return opGe
		}
	}
	return opOther
}

// intOp computes x op y for two Ints that fit in int64, or reports that it
// can't: on overflow, a division by zero, or an operator it doesn't know.
func intOp(op binOp, x, y int64) (Value, bool) {
	switch op {
	case opAdd:
		if r := x + y; (r > x) == (y > 0) {
			return r, true
		}
	case opSub:
		if r := x - y; (r < x) == (y > 0) {
			return r, true
		}
	case opMul:
		if x == 0 || y == 0 {
			return int64(0), true
		}
		if r := x * y; r/y == x && (x != -1 || y != math.MinInt64) && (y != -1 || x != math.MinInt64) {
			return r, true
		}
	case opDiv, opRem:
		if y != 0 && (x != math.MinInt64 || y != -1) {
			q := floorDiv(x, y)
			if op == opDiv {
				return q, true
			}
			return x - q*y, true
		}
	case opEq:
		return x == y, true
	case opNe:
		return x != y, true
	case opLt:
		return x < y, true
	case opLe:
		return x <= y, true
	case opGt:
		return x > y, true
	case opGe:
		return x >= y, true
	}
	return nil, false
}

func (in *Interp) binary(op string, a, b Value, at ir.Pos) Value {
	code := opOf(op)
	if x, ok := a.(int64); ok {
		if y, ok := b.(int64); ok {
			if r, ok := intOp(code, x, y); ok {
				return r
			}
		}
	}
	switch code {
	case opEq:
		return equal(a, b)
	case opNe:
		return !equal(a, b)
	case opLt, opLe, opGt, opGe:
		c, ok := compare(a, b)
		if !ok {
			in.faultf(at, "%s and %s can't be compared", kindName(a), kindName(b))
		}
		switch code {
		case opLt:
			return c < 0
		case opLe:
			return c <= 0
		case opGt:
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
	case int64:
		// `3 * 5 min`, the same as `5 min * 3`.
		if op == "*" {
			switch y := b.(type) {
			case ir.Duration:
				return ir.Duration(x) * y
			case ir.Size:
				return ir.Size(x) * y
			}
		}
	case string:
		if y, ok := b.(string); ok && op == "+" {
			return x + y
		}
	case *List:
		if y, ok := b.(*List); ok && op == "+" {
			// `xs += [x]` and `xs + ys` append in place when they can.
			return x.appended(y.Elems...)
		}
	case *Set:
		// `seen + toSet(new)` is the union, in the order the elements came.
		if op == "+" {
			switch y := b.(type) {
			case *Set:
				for _, e := range y.Elems {
					x = x.added(e)
				}
				return x
			case Empty:
				return x
			}
		}
	case Empty:
		if _, ok := b.(*Set); ok && op == "+" {
			return b
		}
		if _, ok := b.(Empty); ok && op == "+" {
			return b
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
				if y == 0 {
					in.faultf(at, "division by zero")
				}
				return x / ir.Duration(y)
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
			switch op {
			case "*":
				return x * ir.Size(y)
			case "/":
				if y == 0 {
					in.faultf(at, "division by zero")
				}
				return x / ir.Size(y)
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
	if v, ok := in.userOp(op, a, b, at); ok {
		return v
	}
	in.faultf(at, "%s %s %s isn't defined", kindName(a), op, kindName(b))
	return nil
}

// try is postfix `?`: it unwraps Ok and Some, and returns Err and None from
// the function, wrapping an error into the function's error type.
func (in *Interp) try(f *frame, v Value, x *ir.Try) Value {
	if o, ok := isVariant(v, "Ok"); ok {
		return o.Vals[0]
	}
	if s, ok := isVariant(v, "Some"); ok {
		return s.Vals[0]
	}
	if _, ok := isVariant(v, "None"); ok {
		return in.leave(f, x, ctlReturn, v)
	}
	e, ok := isVariant(v, "Err")
	if !ok {
		in.faultf(x.Pos, "? works on a Result or an Option, and this is %s", kindName(v))
	}
	return in.leave(f, x, ctlReturn, in.errV(in.wrapError(e.Vals[0], x.Wrap)))
}

// wrapError puts an error into the variant of target whose one field holds
// the error's type, when the error isn't already of type target. The checker
// has made sure one variant holds it; the error's value finds which, as fits
// does: a String error goes into the variant that holds a String.
func (in *Interp) wrapError(err Value, target *ir.Type) Value {
	if p := in.pre.problem; p != nil && target == p.Type {
		return in.problem(err, nil)
	}
	if target == nil || in.typeOf(err) == target {
		return err
	}
	for _, v := range target.Variants {
		if len(v.Fields) != 1 {
			continue
		}
		t := v.Fields[0].Type
		for t != nil && t.Decl != nil && t.Decl.Alias != nil && len(t.Decl.Fields) == 0 {
			t = t.Decl.Alias
		}
		if in.fits(t, err) {
			return &Variant{V: v, Vals: []Value{err}}
		}
	}
	return err
}

// ---------------------------------------------------------------- places

// readPlace reads a place. keys holds the value of each element's index on
// its path, from placeKeys; nil when the path has none.
func (in *Interp) readPlace(f *frame, p ir.Place, keys []Value) Value {
	var v Value
	if p.Self {
		v = f.getSelf()
	} else {
		v = plain(f.slots[p.Slot])
		if _, ok := f.slots[p.Slot].(*ownRec); ok && len(p.Path) == 0 {
			f.slots[p.Slot] = v // the record may be held elsewhere from now on
		}
	}
	for i, step := range p.Path {
		if step.Index != nil {
			v = in.element(v, keys[i], p.Pos)
			continue
		}
		v = in.field(v, step.Field, p.Pos)
	}
	return v
}

// placeKeys evaluates the indexes of the elements on a place's path, in
// order: one value for each step, nil for a field.
func (in *Interp) placeKeys(f *frame, p ir.Place) []Value {
	if !p.HasIndex() {
		return nil
	}
	keys := make([]Value, len(p.Path))
	for i, step := range p.Path {
		if step.Index != nil {
			keys[i] = in.eval(f, step.Index)
		}
	}
	return keys
}

// writePlace sets a place to v; keys are as for readPlace.
func (in *Interp) writePlace(f *frame, p ir.Place, keys []Value, v Value) {
	if p.Self {
		f.setSelf(in.setPath(f.getSelf(), p.Path, keys, v, p.Pos))
		return
	}
	if len(p.Path) == 0 {
		f.slots[p.Slot] = v
		return
	}
	if o, ok := f.slots[p.Slot].(*ownRec); ok && p.Path[0].Index == nil {
		// Only this variable holds the record: change it in place.
		r := (*Record)(o)
		i := r.field(p.Path[0].Field)
		if i < 0 {
			in.faultf(p.Pos, "the record has no field %s", p.Path[0].Field)
		}
		var rest []Value
		if keys != nil {
			rest = keys[1:]
		}
		r.Vals[i] = in.setPath(r.Vals[i], p.Path[1:], rest, v, p.Pos)
		return
	}
	// setPath makes a new record, which only this variable holds.
	own(f, p.Slot, in.setPath(plain(f.slots[p.Slot]), p.Path, keys, v, p.Pos))
}

// setPath returns base with what is at path replaced by v: a field of a
// record, an element of a list, which must be there, or a map's value for
// a key, which is inserted or replaced. keys are as for readPlace.
func (in *Interp) setPath(base Value, path []ir.Step, keys []Value, v Value, at ir.Pos) Value {
	if len(path) == 0 {
		return v
	}
	var key, rest []Value
	if keys != nil {
		key, rest = keys[:1], keys[1:]
	}
	if path[0].Index != nil {
		switch c := base.(type) {
		case *List:
			i := in.listIndex(c, key[0], at)
			elems := make([]Value, len(c.Elems))
			copy(elems, c.Elems)
			elems[i] = in.setPath(c.Elems[i], path[1:], rest, v, at)
			return &List{Elems: elems}
		case *Map:
			return c.with(key[0], v)
		case Empty: // `{}`, an empty map here
			return newMap(nil, nil).with(key[0], v)
		}
		in.faultf(at, "only a list's elements and a map's values can be assigned, and this is %s", kindName(base))
	}
	r, ok := base.(*Record)
	if !ok {
		in.faultf(at, "only a record's fields can be assigned, and this is %s", kindName(base))
	}
	i := r.field(path[0].Field)
	if i < 0 {
		in.faultf(at, "the record has no field %s", path[0].Field)
	}
	out := r.copied()
	out.Vals[i] = in.setPath(r.Vals[i], path[1:], rest, v, at)
	return out
}

// element reads the element at key of a list, which must be there, for a
// compound assignment to it: `xs[i] += 1`.
func (in *Interp) element(v, key Value, at ir.Pos) Value {
	switch c := v.(type) {
	case *List:
		return c.Elems[in.listIndex(c, key, at)]
	case *Map:
		if x, ok := c.get(key); ok {
			return x
		}
		in.faultf(at, "the map has no key %s", show(key))
	}
	in.faultf(at, "only a list's elements and a map's values can be assigned, and this is %s", kindName(v))
	return nil
}

// listIndex is key as an index of l's elements. An index out of range is a
// fault, as SPECS.md §7 says: the code assigning there has a bug.
func (in *Interp) listIndex(l *List, key Value, at ir.Pos) int {
	i, ok := key.(int64)
	if !ok || i < 0 || i >= int64(len(l.Elems)) {
		in.faultf(at, "index %s is out of range for a list of %d", show(key), len(l.Elems))
	}
	return int(i)
}

// ---------------------------------------------------------------- loops

func (in *Interp) forLoop(f *frame, x *ir.For) {
	in.forOver(f, x, in.eval(f, x.Iter))
}

// forOver runs a for loop over iter, already evaluated.
func (in *Interp) forOver(f *frame, x *ir.For, iter Value) {
	catch := in.ctlOf(f).catch[x]
	each := func(v Value) bool {
		if !in.match(f, x.Pattern, v) {
			in.faultf(x.Pos, "%s doesn't match the loop's pattern", show(v))
		}
		return in.loopBody(f, x.Body, catch)
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
		vals := it.values()
		for i, k := range it.Keys {
			if each(&Tuple{Elems: []Value{k, vals[i]}}) {
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
	case *ReceiverVal:
		for {
			v, ok := in.take(it.c, x.Pos)
			if !ok || each(v) {
				return
			}
		}
	case *IterVal:
		in.walk(it, x.Pos, func(v Value) bool { return !each(v) })
	case Empty:
	default:
		in.faultf(x.Pos, "a for loop can't walk %s", kindName(iter))
	}
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
