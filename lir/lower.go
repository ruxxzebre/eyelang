package lir

import (
	"strings"

	"eyelang/ir"
)

// fnb lowers one function instance: each expression of its typed IR, with
// the instance's types put in for the function's type variables, becomes
// instructions that name their values, in blocks that end in jumps.
type fnb struct {
	b       *builder
	in      *instance
	f       *Func
	sub     subst
	cur     *Block // nil after a jump, until the next block starts
	slots   map[int]*Value
	self    *Value
	loops   []loopBlocks
	durable bool // the call being lowered is a `!` step
	after   []*SetPlace
	unitV   *Value
}

type loopBlocks struct{ next, exit *Block }

func (b *builder) lowerInstance(in *instance) {
	x := &fnb{b: b, in: in, f: in.fn, sub: in.sub, slots: map[int]*Value{}}
	src := in.src
	x.cur = x.newBlock()

	var params []ir.Ty
	result := tyUnit
	if in.lambda != nil {
		if ft, ok := x.ty(in.lambda).(*ir.TFun); ok {
			params, result = ft.Params, ft.Ret
		}
		for _, c := range src.Captures {
			x.slots[c.Slot] = x.param(nameOf(c.From), x.ty(c.From))
		}
	} else if src.Sig != nil {
		params, result = x.sub.applyAll(src.Sig.Params), x.sub.apply(src.Sig.Ret)
	}
	x.f.Result = result
	switch {
	case src.Entity != nil && in.lambda == nil:
		x.self = x.param("state", &ir.TCon{Name: src.Entity.Name, Entity: src.Entity})
	case src.HasSelf && len(params) > 0:
		x.self = x.param("self", params[0])
		params = params[1:]
	}
	if in.lambda != nil && len(params) == 1 && len(src.Params) > 1 {
		// A lambda with as many parameters as a tuple has elements takes
		// the tuple apart.
		tuple := x.param("tuple", params[0])
		tt, _ := params[0].(*ir.TTuple)
		for i, p := range src.Params {
			var et ir.Ty
			if tt != nil && i < len(tt.Elems) {
				et = tt.Elems[i]
			}
			v := x.value(p.Name, et)
			x.emit(&TupleGet{Dst: v, Src: tuple, Index: i})
			x.slots[p.Slot] = v
		}
	} else {
		for i, p := range src.Params {
			var t ir.Ty
			if i < len(params) {
				t = params[i]
			}
			x.slots[p.Slot] = x.param(p.Name, t)
		}
	}
	// A parameter left out of a call takes its default here.
	for _, p := range src.Params {
		if p.Default == nil {
			continue
		}
		v := x.slots[p.Slot]
		absent := x.value("", tyBool)
		x.emit(&Builtin{Dst: absent, Name: "absent", Args: []*Value{v}})
		give, cont := x.newBlock(), x.newBlock()
		x.term(&Branch{Cond: absent, Then: give, Else: cont})
		x.cur = give
		d := x.expr(p.Default)
		if x.cur != nil {
			x.emit(&Move{Dst: v, Src: d})
			x.term(&Jump{To: cont})
		}
		x.cur = cont
	}
	v := x.expr(src.Body)
	if x.cur != nil {
		x.ret(v)
	}
	x.prune()
}

// ---------------------------------------------------------------- helpers

var (
	tyBool ir.Ty = &ir.TCon{Name: "Bool"}
	tyInt  ir.Ty = &ir.TCon{Name: "Int"}
	tyStr  ir.Ty = &ir.TCon{Name: "String"}
)

func (x *fnb) ty(e ir.Expr) ir.Ty {
	t := x.sub.apply(e.Ty())
	if t == nil {
		return tyUnit
	}
	return t
}

func (x *fnb) value(name string, t ir.Ty) *Value {
	x.f.values++
	if t == nil {
		t = tyUnit
	}
	return &Value{ID: x.f.values, Name: name, Ty: t}
}

func (x *fnb) param(name string, t ir.Ty) *Value {
	v := x.value(name, t)
	x.f.Params = append(x.f.Params, v)
	return v
}

func (x *fnb) newBlock() *Block {
	b := &Block{ID: len(x.f.Blocks)}
	x.f.Blocks = append(x.f.Blocks, b)
	return b
}

func (x *fnb) emit(in Instr) {
	if x.cur == nil {
		x.cur = x.newBlock() // after a jump: code nothing reaches
	}
	x.cur.Instrs = append(x.cur.Instrs, in)
}

func (x *fnb) term(t Term) {
	if x.cur == nil {
		x.cur = x.newBlock()
	}
	x.cur.Term = t
	x.cur = nil
}

func (x *fnb) constant(v any, t ir.Ty) *Value {
	d := x.value("", t)
	x.emit(&Const{Dst: d, Value: v})
	return d
}

// unit is the function's () value, made once where it starts.
func (x *fnb) unit() *Value {
	if x.unitV == nil {
		x.unitV = x.value("", tyUnit)
		entry := x.f.Blocks[0]
		entry.Instrs = append([]Instr{&Const{Dst: x.unitV, Value: ir.Unit{}}}, entry.Instrs...)
	}
	return x.unitV
}

func (x *fnb) ret(v *Value) {
	x.term(&Return{Value: x.coerce(v, x.f.Result)})
}

// branchOn goes on in a new block when cond holds, and to fail otherwise.
func (x *fnb) branchOn(cond *Value, fail *Block) {
	ok := x.newBlock()
	x.term(&Branch{Cond: cond, Then: ok, Else: fail})
	x.cur = ok
}

// stop is a block that ends the code with a fault.
func (x *fnb) stop(why string) *Block {
	saved := x.cur
	b := x.newBlock()
	x.cur = b
	x.term(&Stop{Why: why})
	x.cur = saved
	return b
}

// prune skips blocks that only jump, drops the blocks nothing reaches, and
// numbers the rest in order.
func (x *fnb) prune() {
	if len(x.f.Blocks) == 0 {
		return
	}
	skip := func(b *Block) *Block {
		for i := 0; i < len(x.f.Blocks) && b != nil && len(b.Instrs) == 0; i++ {
			j, ok := b.Term.(*Jump)
			if !ok || j.To == b {
				break
			}
			b = j.To
		}
		return b
	}
	for _, b := range x.f.Blocks {
		switch t := b.Term.(type) {
		case *Jump:
			t.To = skip(t.To)
		case *Branch:
			t.Then, t.Else = skip(t.Then), skip(t.Else)
		}
	}
	seen := map[*Block]bool{}
	var order []*Block
	var visit func(b *Block)
	visit = func(b *Block) {
		if b == nil || seen[b] {
			return
		}
		seen[b] = true
		order = append(order, b)
		switch t := b.Term.(type) {
		case *Jump:
			visit(t.To)
		case *Branch:
			visit(t.Then)
			visit(t.Else)
		case nil:
			b.Term = &Stop{Why: "the end of code that gives no value"}
		}
	}
	visit(x.f.Blocks[0])
	for i, b := range order {
		b.ID = i
	}
	x.f.Blocks = order
}

func nameOf(e ir.Expr) string {
	switch e := e.(type) {
	case *ir.Local:
		return e.Name
	case *ir.SelfField:
		return e.Name
	case *ir.Self:
		return "self"
	}
	return ""
}

func (x *fnb) takeDurable() bool {
	d := x.durable
	x.durable = false
	return d
}

// slot is the variable of a slot of the function's frame.
func (x *fnb) slot(slot int, name string, t ir.Ty) *Value {
	if v, ok := x.slots[slot]; ok {
		return v
	}
	v := x.value(name, t)
	x.slots[slot] = v
	return v
}

// coerce makes v a want: a value of a type that implements a trait becomes
// a dyn of that trait where one is expected.
func (x *fnb) coerce(v *Value, want ir.Ty) *Value {
	d, ok := want.(*ir.TDyn)
	if !ok || v == nil {
		return v
	}
	if _, already := v.Ty.(*ir.TDyn); already {
		return v
	}
	if _, never := v.Ty.(ir.TNever); never {
		return v
	}
	table := x.b.dynTable(v.Ty, d.Trait, x.in, x.in.at)
	r := x.value("", want)
	x.emit(&MakeDyn{Dst: r, Src: v, Table: table})
	return r
}

// ---------------------------------------------------------------- expressions

func (x *fnb) expr(e ir.Expr) *Value {
	switch e := e.(type) {
	case *ir.Lit:
		return x.constant(e.Value, x.ty(e))
	case *ir.EmptyBraces:
		return x.empty(x.ty(e))
	case *ir.Local:
		return x.slot(e.Slot, e.Name, x.ty(e))
	case *ir.Self:
		if x.self == nil {
			x.self = x.value("self", x.ty(e))
		}
		return x.self
	case *ir.SelfField:
		v := x.value("", x.ty(e))
		x.emit(&Field{Dst: v, Src: x.expr(&ir.Self{At: e.At}), Name: e.Name})
		return v
	case *ir.Ref:
		return x.ref(e)
	case *ir.Call:
		return x.call(e)
	case *ir.MethodCall:
		return x.methodCall(e)
	case *ir.Waiting:
		x.durable = true
		v := x.expr(e.Call)
		x.durable = false
		return v
	case *ir.Field:
		return x.field(e)
	case *ir.Index:
		return x.index(e)
	case *ir.Unary:
		return x.unary(e)
	case *ir.Binary:
		return x.binary(e)
	case *ir.If:
		return x.ifExpr(e)
	case *ir.Match:
		return x.match(e)
	case *ir.Block:
		var v *Value
		for _, s := range e.Exprs {
			v = x.expr(s)
		}
		if !e.Value || v == nil {
			return x.unit()
		}
		return v
	case *ir.Let:
		return x.let(e)
	case *ir.Assign:
		return x.assign(e)
	case *ir.For:
		return x.forLoop(e)
	case *ir.While:
		return x.while(e)
	case *ir.Break:
		if n := len(x.loops); n > 0 {
			x.term(&Jump{To: x.loops[n-1].exit})
		}
		return x.unit()
	case *ir.Continue:
		if n := len(x.loops); n > 0 {
			x.term(&Jump{To: x.loops[n-1].next})
		}
		return x.unit()
	case *ir.Return:
		var v *Value
		if e.Value != nil {
			v = x.expr(e.Value)
		} else {
			v = x.unit()
		}
		if x.cur != nil {
			x.ret(v)
		}
		return x.unit()
	case *ir.Lambda:
		var caps []*Value
		for _, c := range e.Func.Captures {
			caps = append(caps, x.expr(c.From))
		}
		fn := x.b.requestLambda(e, x.sub, x.in)
		v := x.value("", x.ty(e))
		x.emit(&MakeClosure{Dst: v, Fn: fn, Captures: caps})
		return v
	case *ir.Record:
		return x.record(e)
	case *ir.RecordUpdate:
		base := x.expr(e.Base)
		var vals []*Value
		for i, n := range e.Names {
			vals = append(vals, x.coerce(x.expr(e.Values[i]), fieldTy(base.Ty, n)))
		}
		v := x.value("", x.ty(e))
		x.emit(&Update{Dst: v, Src: base, Names: e.Names, Fields: vals})
		return v
	case *ir.List:
		t := x.ty(e)
		elem := elemOf(t)
		var elems []*Value
		for _, el := range e.Elems {
			elems = append(elems, x.coerce(x.expr(el), elem))
		}
		v := x.value("", t)
		x.emit(&MakeList{Dst: v, Elems: elems})
		return v
	case *ir.Map:
		t := x.ty(e)
		var kt, vt ir.Ty
		if c, ok := t.(*ir.TCon); ok && len(c.Args) == 2 {
			kt, vt = c.Args[0], c.Args[1]
		}
		var keys, vals []*Value
		for i := range e.Keys {
			keys = append(keys, x.coerce(x.expr(e.Keys[i]), kt))
			vals = append(vals, x.coerce(x.expr(e.Values[i]), vt))
		}
		v := x.value("", t)
		x.emit(&MakeMap{Dst: v, Keys: keys, Values: vals})
		return v
	case *ir.Tuple:
		t := x.ty(e)
		tt, _ := t.(*ir.TTuple)
		var elems []*Value
		for i, el := range e.Elems {
			v := x.expr(el)
			if tt != nil && i < len(tt.Elems) {
				v = x.coerce(v, tt.Elems[i])
			}
			elems = append(elems, v)
		}
		v := x.value("", t)
		x.emit(&MakeTuple{Dst: v, Elems: elems})
		return v
	case *ir.InZone:
		zone := x.expr(e.Zone)
		at := x.constant(e.Instant, &ir.TCon{Name: "Instant"})
		v := x.value("", x.ty(e))
		x.emit(&Builtin{Dst: v, Name: "inZone", Args: []*Value{at, zone}})
		return v
	case *ir.Interpolate:
		var parts []*Value
		for _, p := range e.Parts {
			parts = append(parts, x.show(x.expr(p)))
		}
		v := x.value("", tyStr)
		x.emit(&Builtin{Dst: v, Name: "concat", Args: parts})
		return v
	case *ir.Try:
		return x.try(e)
	case *ir.Default:
		return x.orElse(e)
	case *ir.Range:
		start, end := x.absentOr(e.Start), x.absentOr(e.End)
		incl := x.constant(e.Inclusive, tyBool)
		v := x.value("", x.ty(e))
		x.emit(&Builtin{Dst: v, Name: "range", Args: []*Value{start, end, incl}})
		return v
	case *ir.With:
		h := x.expr(e.Value)
		x.emit(&Runtime{Op: "test.with", Args: []*Value{h}})
		return x.unit()
	}
	x.b.errs = append(x.b.errs, &Error{Pos: e.Position(), Msg: "the low IR can't lower this expression yet"})
	return x.unit()
}

func (x *fnb) absentOr(e ir.Expr) *Value {
	if e == nil {
		return x.constant(Absent{}, tyUnit)
	}
	return x.expr(e)
}

func (x *fnb) empty(t ir.Ty) *Value {
	v := x.value("", t)
	if c, ok := t.(*ir.TCon); ok {
		switch {
		case c.Decl != nil:
			x.emit(&MakeRecord{Dst: v, Type: c.Decl})
			return v
		case c.Name == "Set":
			x.emit(&MakeSet{Dst: v})
			return v
		}
	}
	x.emit(&MakeMap{Dst: v})
	return v
}

func (x *fnb) record(e *ir.Record) *Value {
	t := x.ty(e)
	decl := e.Type
	if c, ok := t.(*ir.TCon); ok && decl == nil {
		decl = c.Decl
	}
	var vals []*Value
	for i, n := range e.Names {
		vals = append(vals, x.coerce(x.expr(e.Values[i]), fieldTy(t, n)))
	}
	v := x.value("", t)
	x.emit(&MakeRecord{Dst: v, Type: decl, Names: e.Names, Fields: vals})
	return v
}

// show is a value as a string in interpolation: a string as it is, and
// anything else through its type's Show, its own impl's or the derived one.
func (x *fnb) show(v *Value) *Value {
	if c, ok := v.Ty.(*ir.TCon); ok && c.Name == "String" && c.Decl == nil {
		return v
	}
	s := x.value("", tyStr)
	if tr := x.b.prog.Prelude.Traits["Show"]; tr != nil {
		if m := x.b.implMethod(tr, "show", v.Ty); m != nil && m.Body != nil {
			fn := x.b.instanceAt(m, &ir.TFun{Params: []ir.Ty{v.Ty}, Ret: tyStr}, KindFunc, x.in, x.in.at)
			x.emit(&Call{Dst: s, Fn: fn, Args: []*Value{v}})
			return s
		}
	}
	x.emit(&Derived{Dst: s, Method: "show", Args: []*Value{v}})
	return s
}

func (x *fnb) ref(e *ir.Ref) *Value {
	t := x.ty(e)
	v := x.value("", t)
	switch d := e.Decl.(type) {
	case *ir.Func:
		if d.Body == nil {
			x.emit(&Runtime{Dst: v, Op: "fn.value", Target: d})
			return v
		}
		fn := x.b.instanceAt(d, t, kindOf(d), x.in, e.Pos)
		x.emit(&MakeClosure{Dst: v, Fn: fn})
	case *ir.Global:
		x.emit(&Global{Dst: v, Init: x.b.request(d.Init, subst{}, KindGlobal, x.in, e.Pos)})
	case *ir.Variant:
		if len(d.Fields) == 0 {
			x.emit(&MakeVariant{Dst: v, Variant: d})
		} else {
			x.emit(&Runtime{Dst: v, Op: "variant.constructor", Target: d})
		}
	case *ir.Entity:
		x.emit(&Runtime{Dst: v, Op: "entity.type", Target: d})
	case *ir.Job:
		x.emit(&Runtime{Dst: v, Op: "job.value", Target: d})
	case *ir.View:
		x.emit(&Runtime{Dst: v, Op: "view.read", Target: d})
	case *ir.Op:
		x.emit(&Runtime{Dst: v, Op: "op.value", Target: d})
	case *ir.Event:
		x.emit(&Runtime{Dst: v, Op: "event.value", Target: d})
	case *ir.Module:
		x.emit(&Runtime{Dst: v, Op: "module.value", Target: d.Name})
	case *ir.Type:
		x.emit(&Runtime{Dst: v, Op: "type.value", Target: d.Name})
	case ir.Builtin:
		x.emit(&Runtime{Dst: v, Op: "builtin.value", Target: d.Name})
	case ir.EnumFn:
		x.emit(&Runtime{Dst: v, Op: "derived.value", Target: d.Type.Name + "." + d.Name})
	default:
		x.b.errs = append(x.b.errs, &Error{Pos: e.Pos, Msg: "the low IR can't name this yet"})
		x.emit(&Const{Dst: v, Value: ir.Unit{}})
	}
	return v
}

// args lowers a call's arguments as written, with their names.
func (x *fnb) args(as []ir.Arg) ([]*Value, []string) {
	var vals []*Value
	var names []string
	for _, a := range as {
		vals = append(vals, x.expr(a.Value))
		names = append(names, a.Name)
	}
	return vals, names
}

// callArgs lowers the arguments of a call of an instance, in the order of
// the callee's parameters, from offset on: Absent for one left out, and a
// `mut` one by its variable.
func (x *fnb) callArgs(target *ir.Func, as []ir.Arg, fn *Func, offset int) ([]*Value, []int) {
	vals := make([]*Value, len(target.Params))
	var mut []int
	next := 0
	for _, a := range as {
		i := -1
		if a.Name != "" {
			for j, p := range target.Params {
				if p.Name == a.Name {
					i = j
				}
			}
		} else {
			for next < len(vals) && vals[next] != nil {
				next++
			}
			i = next
		}
		if i < 0 || i >= len(vals) {
			x.expr(a.Value)
			continue
		}
		var v *Value
		if a.Mut {
			v = x.mutArg(a)
			mut = append(mut, offset+i)
		} else {
			v = x.expr(a.Value)
		}
		if offset+i < len(fn.Params) {
			v = x.coerce(v, fn.Params[offset+i].Ty)
		}
		vals[i] = v
	}
	for i, v := range vals {
		if v == nil {
			t := tyUnit
			if offset+i < len(fn.Params) {
				t = fn.Params[offset+i].Ty
			}
			vals[i] = x.constant(Absent{}, t)
		}
	}
	return vals, mut
}

// mutArg is a `mut` argument: the variable itself, or for `mut x.f` a copy
// of the field written back after the call.
func (x *fnb) mutArg(a ir.Arg) *Value {
	if len(a.Place.Path) == 0 {
		return x.placeRoot(a.Place, x.ty(a.Value))
	}
	v := x.readPlace(a.Place)
	tmp := x.value("", v.Ty)
	x.emit(&Move{Dst: tmp, Src: v})
	x.after = append(x.after, &SetPlace{Var: x.placeRoot(a.Place, nil), Path: a.Place.Path, Src: tmp})
	return tmp
}

func (x *fnb) flushAfter() {
	for _, s := range x.after {
		x.emit(s)
	}
	x.after = nil
}

func (x *fnb) call(e *ir.Call) *Value {
	durable := x.takeDurable()
	t := x.ty(e)
	if r, ok := e.Func.(*ir.Ref); ok {
		switch d := r.Decl.(type) {
		case *ir.Func:
			if d.Body == nil {
				args, names := x.args(e.Args)
				v := x.value("", t)
				x.emit(&Runtime{Dst: v, Op: "extern.call", Target: d, Args: args, Names: names, Durable: durable})
				return v
			}
			fn := x.b.instanceAt(d, x.ty(r), kindOf(d), x.in, e.Pos)
			args, mut := x.callArgs(d, e.Args, fn, 0)
			v := x.value("", t)
			x.emit(&Call{Dst: v, Fn: fn, Args: args, Mut: mut, Durable: durable})
			x.flushAfter()
			return v
		case ir.Builtin:
			args, names := x.args(e.Args)
			v := x.value("", t)
			x.emit(&Builtin{Dst: v, Name: d.Name, Args: args, Names: names, Durable: durable})
			return v
		case *ir.Variant:
			args := make([]*Value, len(d.Fields))
			next := 0
			for _, a := range e.Args {
				i := -1
				if a.Name != "" {
					for j, f := range d.Fields {
						if f.Name == a.Name {
							i = j
						}
					}
				} else {
					i = next
					next++
				}
				v := x.expr(a.Value)
				if i >= 0 && i < len(args) {
					args[i] = x.coerce(v, variantFieldTy(t, d, i))
				}
			}
			v := x.value("", t)
			x.emit(&MakeVariant{Dst: v, Variant: d, Args: args})
			return v
		case *ir.Op:
			args, names := x.args(e.Args)
			v := x.value("", t)
			x.emit(&Runtime{Dst: v, Op: "op.call", Target: d, Args: args, Names: names, Durable: durable})
			return v
		case *ir.Entity:
			args, names := x.args(e.Args)
			v := x.value("", t)
			x.emit(&Runtime{Dst: v, Op: "entity.ref", Target: d, Args: args, Names: names})
			return v
		case ir.EnumFn:
			// T.variants() and T.parse(s), derived for an enum: the back
			// end writes them for the type, which the result's says.
			args, _ := x.args(e.Args)
			v := x.value("", t)
			x.emit(&Derived{Dst: v, Method: d.Name, Args: args})
			return v
		}
	}
	callee := x.expr(e.Func)
	args, _ := x.args(e.Args)
	v := x.value("", t)
	x.emit(&CallValue{Dst: v, Callee: callee, Args: args, Durable: durable})
	return v
}

func (x *fnb) methodCall(e *ir.MethodCall) *Value {
	durable := x.takeDurable()
	t := x.ty(e)
	switch target := e.Target.(type) {
	case ir.FieldCall:
		x.durable = durable
		return x.call(target.Call)
	case ir.Builtin:
		recv := x.expr(e.Recv)
		args, names := x.args(e.Args)
		v := x.value("", t)
		x.emit(&Builtin{Dst: v, Name: target.Name, Args: append([]*Value{recv}, args...), Names: append([]string{""}, names...), Durable: durable})
		return v
	case ir.RuntimeMethod:
		recv := x.expr(e.Recv)
		args, names := x.args(e.Args)
		v := x.value("", t)
		op, what := runtimeMethod(recv.Ty, e.Name)
		if op == "" {
			x.emit(&Builtin{Dst: v, Name: e.Name, Args: append([]*Value{recv}, args...), Names: append([]string{""}, names...), Durable: durable})
			return v
		}
		x.emit(&Runtime{Dst: v, Op: op, Target: what, Args: append([]*Value{recv}, args...), Names: append([]string{""}, names...), Durable: durable})
		return v
	case *ir.Func:
		return x.methodFunc(e, target, t, durable)
	}
	x.b.errs = append(x.b.errs, &Error{Pos: e.Pos, Msg: "the checker left this method call without a target"})
	return x.unit()
}

// runtimeMethod is the runtime operation a method of the runtime's values
// is, by the receiver's type: "" for one the back end does itself, as a
// time's.
func runtimeMethod(recv ir.Ty, name string) (string, any) {
	switch r := recv.(type) {
	case *ir.TJob:
		return "job." + name, r.Job
	case *ir.TCon:
		switch {
		case r.Decl != nil && name == "reconcile":
			return "op.reconcile", nil
		case r.Name == "Ref" && r.Decl == nil:
			return "entity." + name, nil
		case r.Name == "Event" && r.Decl == nil:
			return "event." + name, nil
		case r.Name == "Run" && r.Decl == nil:
			return "run." + name, nil
		case r.Name == "Handler":
			return "fakes." + name, nil
		case r.Name == "TaskGroup" || r.Name == "Sender" || r.Name == "Receiver":
			return "task." + name, nil
		}
	}
	return "", nil
}

func (x *fnb) methodFunc(e *ir.MethodCall, target *ir.Func, t ir.Ty, durable bool) *Value {
	_, onSelf := e.Recv.(*ir.Self)
	recv := x.expr(e.Recv)
	switch {
	case target.Entity != nil && onSelf && x.in.src.Entity == target.Entity:
		// One method of an entity calling another: the same state, in the
		// same transaction.
		fn := x.b.request(target, subst{}, KindEntity, x.in, e.Pos)
		args, mut := x.callArgs(target, e.Args, fn, 1)
		v := x.value("", t)
		x.emit(&Call{Dst: v, Fn: fn, Args: append([]*Value{recv}, args...), Mut: append([]int{0}, mut...)})
		x.flushAfter()
		return v
	case target.Entity != nil:
		fn := x.b.request(target, subst{}, KindEntity, x.in, e.Pos)
		args, names := x.args(e.Args)
		v := x.value("", t)
		x.emit(&Runtime{Dst: v, Op: "entity.call", Target: fn, Args: append([]*Value{recv}, args...), Names: append([]string{""}, names...), Durable: durable})
		return v
	}
	if d, ok := recv.Ty.(*ir.TDyn); ok {
		args, _ := x.args(e.Args)
		v := x.value("", t)
		x.emit(&CallDyn{Dst: v, Recv: recv, Trait: d.Trait, Method: e.Name, Args: args})
		return v
	}
	if tr := x.b.traitOf[target]; tr != nil {
		// A trait's method on a type the instance knows: that type's impl.
		if m := x.b.implMethod(tr, target.Name, recv.Ty); m != nil {
			target = m
		} else {
			args, _ := x.args(e.Args)
			v := x.value("", t)
			x.emit(&Derived{Dst: v, Method: target.Name, Args: append([]*Value{recv}, args...)})
			return v
		}
	}
	sub := subst{}
	if target.Sig != nil {
		if len(target.Sig.Params) > 0 {
			match(target.Sig.Params[0], recv.Ty, sub)
		}
		match(target.Sig.Ret, t, sub)
	}
	fn := x.b.request(target, sub, kindOf(target), x.in, e.Pos)
	args, mut := x.callArgs(target, e.Args, fn, 1)
	if target.MutSelf && e.RecvPlace != nil {
		recv = x.mutArg(ir.Arg{Value: e.Recv, Mut: true, Place: *e.RecvPlace})
		mut = append([]int{0}, mut...)
	}
	v := x.value("", t)
	x.emit(&Call{Dst: v, Fn: fn, Args: append([]*Value{recv}, args...), Mut: mut, Durable: durable})
	x.flushAfter()
	return v
}

func (x *fnb) field(e *ir.Field) *Value {
	v := x.expr(e.Value)
	t := x.ty(e)
	r := x.value("", t)
	switch vt := v.Ty.(type) {
	case *ir.TEntity:
		x.emit(&Runtime{Dst: r, Op: "entity." + e.Name, Target: vt.Entity})
		return r
	case *ir.TCon:
		if vt.Name == "Ref" && vt.Decl == nil {
			x.emit(&Runtime{Dst: r, Op: "entity.field", Target: e.Name, Args: []*Value{v}})
			return r
		}
	}
	x.emit(&Field{Dst: r, Src: v, Name: e.Name})
	return r
}

func (x *fnb) index(e *ir.Index) *Value {
	v := x.expr(e.Value)
	r := x.value("", x.ty(e))
	if rg, ok := e.Index.(*ir.Range); ok {
		start, end := x.absentOr(rg.Start), x.absentOr(rg.End)
		incl := x.constant(rg.Inclusive, tyBool)
		x.emit(&Builtin{Dst: r, Name: "slice", Args: []*Value{v, start, end, incl}})
		return r
	}
	i := x.expr(e.Index)
	x.emit(&Builtin{Dst: r, Name: "index", Args: []*Value{v, i}})
	return r
}

var opTraits = map[string]string{"+": "Add", "-": "Sub", "*": "Mul", "/": "Div", "%": "Rem"}

func declared(t ir.Ty) bool {
	c, ok := t.(*ir.TCon)
	return ok && (c.Decl != nil || c.Entity != nil)
}

func (x *fnb) unary(e *ir.Unary) *Value {
	v := x.expr(e.X)
	t := x.ty(e)
	r := x.value("", t)
	if e.Op == "!" {
		x.emit(&Op{Dst: r, Op: "!", Args: []*Value{v}})
		return r
	}
	impl := e.Impl
	if impl == nil && declared(v.Ty) {
		impl = x.b.opImpl("Neg", v.Ty, nil)
	}
	if impl != nil {
		fn := x.b.instanceAt(impl, &ir.TFun{Params: []ir.Ty{v.Ty}, Ret: t}, KindFunc, x.in, e.Pos)
		x.emit(&Call{Dst: r, Fn: fn, Args: []*Value{v}})
		return r
	}
	x.emit(&Op{Dst: r, Op: "neg", Args: []*Value{v}})
	return r
}

func (x *fnb) binary(e *ir.Binary) *Value {
	switch e.Op {
	case "&&", "||":
		l := x.expr(e.L)
		r := x.value("", tyBool)
		x.emit(&Move{Dst: r, Src: l})
		rhs, join := x.newBlock(), x.newBlock()
		if e.Op == "&&" {
			x.term(&Branch{Cond: l, Then: rhs, Else: join})
		} else {
			x.term(&Branch{Cond: l, Then: join, Else: rhs})
		}
		x.cur = rhs
		rv := x.expr(e.R)
		x.emit(&Move{Dst: r, Src: rv})
		x.term(&Jump{To: join})
		x.cur = join
		return r
	case "==", "!=", "<", "<=", ">", ">=":
		return x.compare(e.Op, x.expr(e.L), x.expr(e.R))
	}
	l := x.expr(e.L)
	r := x.expr(e.R)
	return x.arith(e.Op, l, r, e.Impl, x.ty(e), e.Pos)
}

// scalar reports whether a type's values compare with the back end's own
// operators.
func scalar(t ir.Ty) bool {
	c, ok := t.(*ir.TCon)
	if !ok || c.Decl != nil || c.Entity != nil {
		return false
	}
	switch c.Name {
	case "Int", "Float", "Bool", "Char", "String", "Duration", "Size", "Date", "Time", "Instant",
		"Zone", "Key", "Byte", "Int32", "Int64", "UInt32", "UInt64", "NonZero", "Offset", "()":
		return true
	}
	return false
}

func (x *fnb) compare(op string, l, r *Value) *Value {
	res := x.value("", tyBool)
	if scalar(l.Ty) {
		x.emit(&Op{Dst: res, Op: op, Args: []*Value{l, r}})
		return res
	}
	switch op {
	case "==":
		x.emit(&Derived{Dst: res, Method: "eq", Args: []*Value{l, r}})
	case "!=":
		eq := x.value("", tyBool)
		x.emit(&Derived{Dst: eq, Method: "eq", Args: []*Value{l, r}})
		x.emit(&Op{Dst: res, Op: "!", Args: []*Value{eq}})
	default:
		c := x.value("", tyInt)
		x.emit(&Derived{Dst: c, Method: "cmp", Args: []*Value{l, r}})
		zero := x.constant(int64(0), tyInt)
		x.emit(&Op{Dst: res, Op: op, Args: []*Value{c, zero}})
	}
	return res
}

// arith is `l op r`: the impl method a declared type's operator calls, or
// the back end's operator on built-in types.
func (x *fnb) arith(op string, l, r *Value, impl *ir.Func, t ir.Ty, at ir.Pos) *Value {
	if impl == nil && declared(l.Ty) {
		impl = x.b.opImpl(opTraits[op], l.Ty, r.Ty)
	}
	res := x.value("", t)
	if impl != nil {
		fn := x.b.instanceAt(impl, &ir.TFun{Params: []ir.Ty{l.Ty, r.Ty}, Ret: t}, KindFunc, x.in, at)
		x.emit(&Call{Dst: res, Fn: fn, Args: []*Value{l, r}})
		return res
	}
	x.emit(&Op{Dst: res, Op: op, Args: []*Value{l, r}})
	return res
}

func (x *fnb) ifExpr(e *ir.If) *Value {
	c := x.expr(e.Cond)
	t := x.ty(e)
	r := x.result(t)
	then, other, join := x.newBlock(), x.newBlock(), x.newBlock()
	x.term(&Branch{Cond: c, Then: then, Else: other})
	x.cur = then
	x.into(r, x.expr(e.Then), join)
	x.cur = other
	if e.Else != nil {
		x.into(r, x.expr(e.Else), join)
	} else {
		x.term(&Jump{To: join})
	}
	x.cur = join
	if r == nil {
		return x.unit()
	}
	return r
}

// result is the variable a branching expression of type t leaves its value
// in, or nil for one that gives none.
func (x *fnb) result(t ir.Ty) *Value {
	switch c := t.(type) {
	case ir.TNever:
		return nil
	case *ir.TCon:
		if c.Name == "()" && c.Decl == nil {
			return nil
		}
	}
	return x.value("", t)
}

// into ends a branch: its value goes into r, and it jumps to join.
func (x *fnb) into(r, v *Value, join *Block) {
	if x.cur == nil {
		return // the branch left already
	}
	if r != nil {
		x.emit(&Move{Dst: r, Src: x.coerce(v, r.Ty)})
	}
	x.term(&Jump{To: join})
}

func (x *fnb) match(e *ir.Match) *Value {
	v := x.expr(e.Value)
	r := x.result(x.ty(e))
	join := x.newBlock()
	next := x.newBlock()
	x.term(&Jump{To: next})
	for _, arm := range e.Arms {
		x.cur = next
		next = x.newBlock()
		x.pattern(v, arm.Pattern, next)
		if arm.Guard != nil {
			g := x.expr(arm.Guard)
			x.branchOn(g, next)
		}
		x.into(r, x.expr(arm.Body), join)
	}
	x.cur = next
	x.term(&Stop{Why: "no arm matches"})
	x.cur = join
	if r == nil {
		return x.unit()
	}
	return r
}

func (x *fnb) let(e *ir.Let) *Value {
	v := x.expr(e.Value)
	if e.Else == nil {
		x.pattern(v, e.Pattern, nil)
		return x.unit()
	}
	fail := x.newBlock()
	x.pattern(v, e.Pattern, fail)
	cont := x.cur
	x.cur = fail
	x.expr(e.Else)
	if x.cur != nil {
		x.term(&Stop{Why: "the else block of a let must leave"})
	}
	x.cur = cont
	return x.unit()
}

func (x *fnb) assign(e *ir.Assign) *Value {
	v := x.expr(e.Value)
	if e.Op != "=" {
		old := x.readPlace(e.Target)
		v = x.arith(strings.TrimSuffix(e.Op, "="), old, v, e.Impl, old.Ty, e.Pos)
	}
	x.emit(&SetPlace{Var: x.placeRoot(e.Target, v.Ty), Path: e.Target.Path, Src: v})
	return x.unit()
}

// placeRoot is the variable a place starts at: the receiver, or a slot.
func (x *fnb) placeRoot(p ir.Place, t ir.Ty) *Value {
	if p.Self {
		return x.expr(&ir.Self{At: ir.At{Pos: p.Pos}})
	}
	return x.slot(p.Slot, "", t)
}

func (x *fnb) readPlace(p ir.Place) *Value {
	v := x.placeRoot(p, nil)
	for _, name := range p.Path {
		f := x.value("", fieldTy(v.Ty, name))
		x.emit(&Field{Dst: f, Src: v, Name: name})
		v = f
	}
	return v
}

func (x *fnb) forLoop(e *ir.For) *Value {
	exit := x.newBlock()
	if rg, ok := e.Iter.(*ir.Range); ok && rg.Start != nil && rg.End != nil {
		lo, hi := x.expr(rg.Start), x.expr(rg.End)
		if c, ok := lo.Ty.(*ir.TCon); ok && c.Name == "Int" {
			i := x.value("", tyInt)
			x.emit(&Move{Dst: i, Src: lo})
			head, body, next := x.newBlock(), x.newBlock(), x.newBlock()
			x.term(&Jump{To: head})
			x.cur = head
			op := "<"
			if rg.Inclusive {
				op = "<="
			}
			more := x.value("", tyBool)
			x.emit(&Op{Dst: more, Op: op, Args: []*Value{i, hi}})
			x.term(&Branch{Cond: more, Then: body, Else: exit})
			x.cur = body
			x.pattern(i, e.Pattern, nil)
			x.loopBody(e.Body, next, exit)
			x.cur = next
			one := x.constant(int64(1), tyInt)
			x.emit(&Op{Dst: i, Op: "+", Args: []*Value{i, one}})
			x.term(&Jump{To: head})
			x.cur = exit
			return x.unit()
		}
	}
	coll := x.expr(e.Iter)
	elem := elemOf(coll.Ty)
	it := x.value("", &ir.TCon{Name: "Iter", Args: []ir.Ty{elem}})
	x.emit(&Builtin{Dst: it, Name: "iter", Args: []*Value{coll}})
	head, body := x.newBlock(), x.newBlock()
	x.term(&Jump{To: head})
	x.cur = head
	more := x.value("", tyBool)
	x.emit(&Builtin{Dst: more, Name: "iter.more", Args: []*Value{it}})
	x.term(&Branch{Cond: more, Then: body, Else: exit})
	x.cur = body
	v := x.value("", elem)
	x.emit(&Builtin{Dst: v, Name: "iter.next", Args: []*Value{it}})
	x.pattern(v, e.Pattern, nil)
	x.loopBody(e.Body, head, exit)
	x.cur = exit
	return x.unit()
}

func (x *fnb) while(e *ir.While) *Value {
	head, body, exit := x.newBlock(), x.newBlock(), x.newBlock()
	x.term(&Jump{To: head})
	x.cur = head
	c := x.expr(e.Cond)
	x.term(&Branch{Cond: c, Then: body, Else: exit})
	x.cur = body
	x.loopBody(e.Body, head, exit)
	x.cur = exit
	return x.unit()
}

func (x *fnb) loopBody(body ir.Expr, next, exit *Block) {
	x.loops = append(x.loops, loopBlocks{next: next, exit: exit})
	x.expr(body)
	x.loops = x.loops[:len(x.loops)-1]
	if x.cur != nil {
		x.term(&Jump{To: next})
	}
}

// try is postfix `?`: the value inside, or a return of the error (wrapped
// in the caller's error variant that holds it) or of None.
func (x *fnb) try(e *ir.Try) *Value {
	v := x.expr(e.X)
	pre := x.b.prog.Prelude.Variants
	c, _ := v.Ty.(*ir.TCon)
	ok, fail := x.newBlock(), x.newBlock()
	isOption := c != nil && c.Name == "Option"
	good := pre["Ok"]
	if isOption {
		good = pre["Some"]
	}
	is := x.value("", tyBool)
	x.emit(&IsVariant{Dst: is, Src: v, Variant: good})
	x.term(&Branch{Cond: is, Then: ok, Else: fail})
	x.cur = fail
	out := x.value("", x.f.Result)
	if isOption {
		x.emit(&MakeVariant{Dst: out, Variant: pre["None"]})
	} else {
		errTy := tyUnit
		if c != nil && len(c.Args) == 2 {
			errTy = c.Args[1]
		}
		err := x.value("", errTy)
		x.emit(&Payload{Dst: err, Src: v, Variant: pre["Err"], Index: 0})
		var want ir.Ty
		if rc, ok := x.f.Result.(*ir.TCon); ok && len(rc.Args) == 2 {
			want = rc.Args[1]
		}
		if e.Wrap != nil && want != nil && typeKey(want) != typeKey(errTy) {
			for _, vr := range e.Wrap.Variants {
				if len(vr.Fields) == 1 && texprFits(vr.Fields[0].Type, errTy) {
					w := x.value("", want)
					x.emit(&MakeVariant{Dst: w, Variant: vr, Args: []*Value{err}})
					err = w
					break
				}
			}
		}
		x.emit(&MakeVariant{Dst: out, Variant: pre["Err"], Args: []*Value{err}})
	}
	x.term(&Return{Value: out})
	x.cur = ok
	r := x.value("", x.ty(e))
	x.emit(&Payload{Dst: r, Src: v, Variant: good, Index: 0})
	return r
}

// orElse is `x ?? y`: the value inside x, or y.
func (x *fnb) orElse(e *ir.Default) *Value {
	v := x.expr(e.X)
	pre := x.b.prog.Prelude.Variants
	good := pre["Some"]
	if c, ok := v.Ty.(*ir.TCon); ok && c.Name == "Result" {
		good = pre["Ok"]
	}
	r := x.value("", x.ty(e))
	is := x.value("", tyBool)
	x.emit(&IsVariant{Dst: is, Src: v, Variant: good})
	some, none, join := x.newBlock(), x.newBlock(), x.newBlock()
	x.term(&Branch{Cond: is, Then: some, Else: none})
	x.cur = some
	inner := x.value("", r.Ty)
	x.emit(&Payload{Dst: inner, Src: v, Variant: good, Index: 0})
	x.into(r, inner, join)
	x.cur = none
	x.into(r, x.expr(e.Or), join)
	x.cur = join
	return r
}

// ---------------------------------------------------------------- patterns

// pattern tests v against p in the current block, binds its variables, and
// goes on in a new block when it matches; it goes to fail when it doesn't.
// A nil fail is for a pattern that can't fail, as a let's without else: a
// test there stops the code.
func (x *fnb) pattern(v *Value, p ir.Pattern, fail *Block) {
	failOr := func() *Block {
		if fail == nil {
			return x.stop("the value doesn't match the pattern")
		}
		return fail
	}
	switch p := p.(type) {
	case ir.PWild, nil:
	case ir.PBind:
		vr := x.slot(p.Slot, p.Name, v.Ty)
		x.emit(&Move{Dst: vr, Src: v})
	case ir.PLit:
		c := x.constant(p.Value, v.Ty)
		x.branchOn(x.compare("==", v, c), failOr())
	case ir.PVariant:
		if p.Variant.Type == nil || len(p.Variant.Type.Variants) > 1 || fail != nil {
			is := x.value("", tyBool)
			x.emit(&IsVariant{Dst: is, Src: v, Variant: p.Variant})
			x.branchOn(is, failOr())
		}
		for i, a := range p.Args {
			idx := i
			if i < len(p.Names) && p.Names[i] != "" {
				for j, f := range p.Variant.Fields {
					if f.Name == p.Names[i] {
						idx = j
					}
				}
			}
			field := x.value("", variantFieldTy(v.Ty, p.Variant, idx))
			x.emit(&Payload{Dst: field, Src: v, Variant: p.Variant, Index: idx})
			x.pattern(field, a, fail)
		}
	case ir.PRecord:
		for i, name := range p.Names {
			f := x.value("", fieldTy(v.Ty, name))
			x.emit(&Field{Dst: f, Src: v, Name: name})
			x.pattern(f, p.Pats[i], fail)
		}
	case ir.PTuple:
		tt, _ := v.Ty.(*ir.TTuple)
		for i, q := range p.Pats {
			var et ir.Ty
			if tt != nil && i < len(tt.Elems) {
				et = tt.Elems[i]
			}
			el := x.value("", et)
			x.emit(&TupleGet{Dst: el, Src: v, Index: i})
			x.pattern(el, q, fail)
		}
	case ir.PList:
		elem := elemOf(v.Ty)
		size := x.value("", tyInt)
		x.emit(&Builtin{Dst: size, Name: "size", Args: []*Value{v}})
		n := x.constant(int64(len(p.Pats)), tyInt)
		ok := x.value("", tyBool)
		if p.Rest < 0 {
			x.emit(&Op{Dst: ok, Op: "==", Args: []*Value{size, n}})
		} else {
			x.emit(&Op{Dst: ok, Op: ">=", Args: []*Value{size, n}})
		}
		x.branchOn(ok, failOr())
		for i, q := range p.Pats {
			var at *Value
			if p.Rest < 0 || i < p.Rest {
				at = x.constant(int64(i), tyInt)
			} else {
				back := x.constant(int64(len(p.Pats)-i), tyInt)
				at = x.value("", tyInt)
				x.emit(&Op{Dst: at, Op: "-", Args: []*Value{size, back}})
			}
			el := x.value("", elem)
			x.emit(&Builtin{Dst: el, Name: "at", Args: []*Value{v, at}})
			x.pattern(el, q, fail)
		}
		if p.Rest >= 0 && p.RestPat != nil {
			start := x.constant(int64(p.Rest), tyInt)
			after := x.constant(int64(len(p.Pats)-p.Rest), tyInt)
			end := x.value("", tyInt)
			x.emit(&Op{Dst: end, Op: "-", Args: []*Value{size, after}})
			rest := x.value("", v.Ty)
			incl := x.constant(false, tyBool)
			x.emit(&Builtin{Dst: rest, Name: "slice", Args: []*Value{v, start, end, incl}})
			x.pattern(rest, p.RestPat, fail)
		}
	case ir.PRange:
		lo, hi := x.constant(p.Lo, v.Ty), x.constant(p.Hi, v.Ty)
		x.branchOn(x.compare(">=", v, lo), failOr())
		op := "<"
		if p.Inclusive {
			op = "<="
		}
		x.branchOn(x.compare(op, v, hi), failOr())
	case ir.POr:
		matched := x.newBlock()
		for _, alt := range p.Alts {
			next := x.newBlock()
			x.pattern(v, alt, next)
			x.term(&Jump{To: matched})
			x.cur = next
		}
		x.term(&Jump{To: failOr()})
		x.cur = matched
	case ir.PEmpty:
		empty := x.value("", tyBool)
		x.emit(&Builtin{Dst: empty, Name: "isEmpty", Args: []*Value{v}})
		x.branchOn(empty, failOr())
	}
}

// ---------------------------------------------------------------- types of parts

// elemOf is the type of what a for loop over a collection of type t gives.
func elemOf(t ir.Ty) ir.Ty {
	if c, ok := t.(*ir.TCon); ok && c.Decl == nil {
		switch {
		case len(c.Args) == 1:
			return c.Args[0]
		case c.Name == "Map" && len(c.Args) == 2:
			return &ir.TTuple{Elems: []ir.Ty{c.Args[0], c.Args[1]}}
		case c.Name == "String":
			return &ir.TCon{Name: "Char"}
		case c.Name == "Bytes":
			return &ir.TCon{Name: "Byte"}
		}
	}
	return tyUnit
}

// envOf is what a declared type's type variables stand for in t.
func envOf(t ir.Ty, decl *ir.Type) map[string]ir.Ty {
	env := map[string]ir.Ty{}
	if c, ok := t.(*ir.TCon); ok && decl != nil {
		for i, p := range decl.Params {
			if i < len(c.Args) {
				env[p] = c.Args[i]
			}
		}
	}
	return env
}

// fieldTy is the type of a field of a record or an entity's state.
func fieldTy(t ir.Ty, name string) ir.Ty {
	c, ok := t.(*ir.TCon)
	if !ok {
		return tyUnit
	}
	if c.Entity != nil {
		if c.Entity.ID != nil && c.Entity.ID.Name == name {
			return texprTy(c.Entity.ID.Type, nil)
		}
		for _, s := range c.Entity.States {
			if s.Name == name {
				return texprTy(s.Type, nil)
			}
		}
		if name == "version" {
			return tyInt
		}
		return tyUnit
	}
	d := c.Decl
	for d != nil && d.Alias != nil && d.Alias.Name != "record" && d.Alias.Decl != nil {
		d = d.Alias.Decl
	}
	if d == nil || d.Alias == nil {
		return tyUnit
	}
	for i, f := range d.Alias.Fields {
		if f == name && i < len(d.Alias.Args) {
			return texprTy(d.Alias.Args[i], envOf(t, c.Decl))
		}
	}
	return tyUnit
}

// variantFieldTy is the type of a variant's field in a value of type t.
func variantFieldTy(t ir.Ty, v *ir.Variant, i int) ir.Ty {
	if i < 0 || i >= len(v.Fields) {
		return tyUnit
	}
	return texprTy(v.Fields[i].Type, envOf(t, v.Type))
}

// texprTy is a type as written, with env for its type variables.
func texprTy(te *ir.TypeExpr, env map[string]ir.Ty) ir.Ty {
	if te == nil {
		return tyUnit
	}
	args := func() []ir.Ty {
		out := make([]ir.Ty, len(te.Args))
		for i, a := range te.Args {
			out[i] = texprTy(a, env)
		}
		return out
	}
	switch {
	case te.Decl != nil:
		d := te.Decl
		if d.Alias != nil && d.Alias.Name != "record" && len(d.Variants) == 0 {
			inner := map[string]ir.Ty{}
			for i, p := range d.Params {
				if i < len(te.Args) {
					inner[p] = texprTy(te.Args[i], env)
				}
			}
			return texprTy(d.Alias, inner)
		}
		return &ir.TCon{Name: d.Name, Decl: d, Args: args()}
	case te.Entity != nil:
		return &ir.TCon{Name: te.Entity.Name, Entity: te.Entity}
	case te.Trait != nil:
		return &ir.TDyn{Trait: te.Trait}
	}
	switch te.Name {
	case "()":
		return tyUnit
	case "tuple":
		return &ir.TTuple{Elems: args()}
	case "fn":
		f := &ir.TFun{Params: args(), Ret: tyUnit}
		if te.Result != nil {
			f.Ret = texprTy(te.Result, env)
		}
		if te.HasUses {
			f.Uses = append([]string{}, te.Uses...)
		}
		return f
	case "dyn":
		if len(te.Args) == 1 && te.Args[0].Trait != nil {
			d := &ir.TDyn{Trait: te.Args[0].Trait}
			for _, a := range te.Args[0].Args {
				d.Args = append(d.Args, texprTy(a, env))
			}
			return d
		}
	}
	if te.Name != "" && te.Name[0] >= 'a' && te.Name[0] <= 'z' {
		if t, ok := env[te.Name]; ok {
			return t
		}
		return &ir.TVar{Name: te.Name}
	}
	return &ir.TCon{Name: te.Name, Args: args()}
}
