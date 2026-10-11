package lir

import (
	"fmt"
	"sort"
	"strings"

	"eyelang/ir"
)

// Specialisation (docs/internals/compilation.md, section 3): starting from
// the program's roots, each generic function is lowered once for each set
// of types it is used at. A call's types say what the callee's type
// variables stand for: they are matched against the callee's signature
// (ir.Func.Sig), which shares its type variables with the callee's body.

// Error is a program the low IR can't be made from: polymorphic
// recursion, for now.
type Error struct {
	Pos ir.Pos
	Msg string
}

func (e *Error) Error() string { return e.Pos.String() + ": " + e.Msg }

// subst says what each type variable, by its id, stands for.
type subst map[int]ir.Ty

// builder makes a program's low IR.
type builder struct {
	prog    *ir.Program
	out     *Program
	insts   map[string]*instance
	queue   []*instance
	names   map[string]int
	traitOf map[*ir.Func]*ir.Trait
	dyn     map[string]*DynTable
	counts  map[*ir.Func]int // instances of each function, for runaway specialisation
	errs    []error
}

// instance is a function instance waiting to be lowered, or lowered.
type instance struct {
	fn     *Func
	src    *ir.Func
	sub    subst
	lambda *ir.Lambda // for a lifted lambda
	parent *instance  // the instance whose call asked for it
	at     ir.Pos     // that call
	size   int        // how big its types are, for polymorphic recursion
}

// Lower makes a checked program's low IR.
func Lower(prog *ir.Program) (*Program, []error) {
	b := &builder{
		prog: prog, out: &Program{}, insts: map[string]*instance{}, names: map[string]int{},
		traitOf: map[*ir.Func]*ir.Trait{}, dyn: map[string]*DynTable{}, counts: map[*ir.Func]int{},
	}
	for _, m := range prog.Modules {
		for _, tr := range m.Traits {
			for _, f := range tr.Methods {
				b.traitOf[f] = tr
			}
		}
	}
	for _, r := range roots(prog) {
		if r.fn.Sig != nil && !generic(r.fn.Sig) {
			b.request(r.fn, subst{}, r.kind, nil, r.fn.Pos)
		}
	}
	for len(b.queue) > 0 {
		in := b.queue[0]
		b.queue = b.queue[1:]
		b.lowerInstance(in)
	}
	return b.out, b.errs
}

// request returns the instance of src at sub, adding it to the queue when
// it is new. A function that asks for itself, through its calls, at bigger
// types can't be specialised: that is an error at the call.
func (b *builder) request(src *ir.Func, sub subst, kind Kind, parent *instance, at ir.Pos) *Func {
	return b.requestInst(&instance{src: src, sub: sub, parent: parent, at: at}, kind)
}

func (b *builder) requestLambda(l *ir.Lambda, sub subst, parent *instance) *Func {
	return b.requestInst(&instance{src: l.Func, sub: sub, lambda: l, parent: parent, at: l.Pos}, KindLambda)
}

func (b *builder) requestInst(in *instance, kind Kind) *Func {
	key := fmt.Sprintf("%p|%s", in.src, substKey(in.sub))
	if got, ok := b.insts[key]; ok {
		return got.fn
	}
	in.size = substSize(in.sub)
	for p := in.parent; p != nil; p = p.parent {
		if p.src == in.src && in.size > p.size {
			b.errs = append(b.errs, &Error{Pos: in.at, Msg: fmt.Sprintf(
				"%s calls itself at bigger types each time, as %s, so it can't be specialised: give the inner call a type of its own", in.src.Name, b.instName(in))})
			in.fn = &Func{Name: b.instName(in), Src: in.src, Kind: kind, Result: tyUnit}
			b.insts[key] = in
			return in.fn
		}
	}
	if b.counts[in.src]++; b.counts[in.src] > 1000 {
		b.errs = append(b.errs, &Error{Pos: in.at, Msg: fmt.Sprintf("%s is used at more than 1000 sets of types", in.src.Name)})
	}
	in.fn = &Func{Name: b.unique(b.instName(in)), Src: in.src, Kind: kind}
	b.insts[key] = in
	b.out.Funcs = append(b.out.Funcs, in.fn)
	b.queue = append(b.queue, in)
	return in.fn
}

// instName names an instance by its module, its owner and the types its
// type variables stand for.
func (b *builder) instName(in *instance) string {
	f := in.src
	name := f.Name
	switch {
	case in.lambda != nil:
		name = "lambda"
	case f.Receiver != nil:
		name = f.Receiver.Name + "." + name
	case f.Entity != nil:
		name = f.Entity.Name + "." + name
	}
	if f.Module != nil && f.Module.Path != "" {
		name = f.Module.Path + "." + name
	}
	if in.lambda != nil && in.parent != nil {
		return in.parent.fn.Name + ".lambda"
	}
	if f.Sig != nil && in.lambda == nil {
		var args []string
		for _, id := range tvarsOf(f.Sig) {
			if t, ok := in.sub[id]; ok {
				args = append(args, t.String())
			}
		}
		if len(args) > 0 {
			name += "<" + strings.Join(args, ", ") + ">"
		}
	}
	return name
}

func (b *builder) unique(name string) string {
	n := b.names[name]
	b.names[name] = n + 1
	if n == 0 {
		return name
	}
	return fmt.Sprintf("%s#%d", name, n+1)
}

// ---------------------------------------------------------------- types

var tyUnit ir.Ty = &ir.TCon{Name: "()"}

// generic reports whether a type has type variables.
func generic(t ir.Ty) bool {
	found := false
	walkTy(t, func(t ir.Ty) {
		if _, ok := t.(*ir.TVar); ok {
			found = true
		}
	})
	return found
}

func walkTy(t ir.Ty, visit func(ir.Ty)) {
	if t == nil {
		return
	}
	visit(t)
	switch x := t.(type) {
	case *ir.TCon:
		for _, a := range x.Args {
			walkTy(a, visit)
		}
	case *ir.TFun:
		for _, p := range x.Params {
			walkTy(p, visit)
		}
		walkTy(x.Ret, visit)
	case *ir.TTuple:
		for _, e := range x.Elems {
			walkTy(e, visit)
		}
	case *ir.TDyn:
		for _, a := range x.Args {
			walkTy(a, visit)
		}
	}
}

// tvarsOf lists a type's variables by id, in the order they first appear.
func tvarsOf(t ir.Ty) []int {
	var ids []int
	seen := map[int]bool{}
	walkTy(t, func(t ir.Ty) {
		if v, ok := t.(*ir.TVar); ok && !seen[v.ID] {
			seen[v.ID] = true
			ids = append(ids, v.ID)
		}
	})
	return ids
}

// apply replaces the type variables sub names.
func (sub subst) apply(t ir.Ty) ir.Ty {
	if len(sub) == 0 || t == nil {
		return t
	}
	switch x := t.(type) {
	case *ir.TVar:
		if r, ok := sub[x.ID]; ok {
			return r
		}
		return x
	case *ir.TCon:
		if len(x.Args) == 0 {
			return x
		}
		cp := *x
		cp.Args = sub.applyAll(x.Args)
		return &cp
	case *ir.TFun:
		return &ir.TFun{Params: sub.applyAll(x.Params), Ret: sub.apply(x.Ret), Uses: x.Uses}
	case *ir.TTuple:
		return &ir.TTuple{Elems: sub.applyAll(x.Elems)}
	case *ir.TDyn:
		return &ir.TDyn{Trait: x.Trait, Args: sub.applyAll(x.Args)}
	}
	return t
}

func (sub subst) applyAll(ts []ir.Ty) []ir.Ty {
	out := make([]ir.Ty, len(ts))
	for i, t := range ts {
		out[i] = sub.apply(t)
	}
	return out
}

// match binds pattern's type variables to the parts of t they stand for.
// Where the two differ otherwise, as a dyn Trait meeting the type it is
// made from, it binds nothing there.
func match(pattern, t ir.Ty, sub subst) {
	if pattern == nil || t == nil {
		return
	}
	switch p := pattern.(type) {
	case *ir.TVar:
		if _, ok := sub[p.ID]; !ok {
			if _, never := t.(ir.TNever); !never {
				sub[p.ID] = t
			}
		}
	case *ir.TCon:
		if c, ok := t.(*ir.TCon); ok && len(c.Args) == len(p.Args) {
			for i := range p.Args {
				match(p.Args[i], c.Args[i], sub)
			}
		}
	case *ir.TFun:
		if f, ok := t.(*ir.TFun); ok && len(f.Params) == len(p.Params) {
			for i := range p.Params {
				match(p.Params[i], f.Params[i], sub)
			}
			match(p.Ret, f.Ret, sub)
		}
	case *ir.TTuple:
		if u, ok := t.(*ir.TTuple); ok && len(u.Elems) == len(p.Elems) {
			for i := range p.Elems {
				match(p.Elems[i], u.Elems[i], sub)
			}
		}
	case *ir.TDyn:
		if d, ok := t.(*ir.TDyn); ok && len(d.Args) == len(p.Args) {
			for i := range p.Args {
				match(p.Args[i], d.Args[i], sub)
			}
		}
	}
}

// typeKey names a type uniquely in the program.
func typeKey(t ir.Ty) string {
	switch x := t.(type) {
	case *ir.TCon:
		name := x.Name
		switch {
		case x.Decl != nil && x.Decl.Module != nil:
			name = x.Decl.Module.Path + "." + x.Decl.Name
		case x.Entity != nil && x.Entity.Module != nil:
			name = "entity " + x.Entity.Module.Path + "." + x.Entity.Name
		}
		if len(x.Args) == 0 {
			return name
		}
		parts := make([]string, len(x.Args))
		for i, a := range x.Args {
			parts[i] = typeKey(a)
		}
		return name + "(" + strings.Join(parts, ",") + ")"
	case *ir.TFun:
		parts := make([]string, len(x.Params))
		for i, p := range x.Params {
			parts[i] = typeKey(p)
		}
		return "fn(" + strings.Join(parts, ",") + ")" + typeKey(x.Ret)
	case *ir.TTuple:
		parts := make([]string, len(x.Elems))
		for i, e := range x.Elems {
			parts[i] = typeKey(e)
		}
		return "(" + strings.Join(parts, ",") + ")"
	case *ir.TDyn:
		return "dyn " + x.Trait.Module.Path + "." + x.Trait.Name
	case nil:
		return "?"
	}
	return t.String()
}

func substKey(sub subst) string {
	ids := make([]int, 0, len(sub))
	for id := range sub {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%d=%s", id, typeKey(sub[id]))
	}
	return strings.Join(parts, ";")
}

// substSize is how big the types of a substitution are.
func substSize(sub subst) int {
	n := 0
	for _, t := range sub {
		walkTy(t, func(ir.Ty) { n++ })
	}
	return n
}

// ---------------------------------------------------------------- impls

// implMethod finds the method that answers a trait's method for a type: its
// own impl's, or the trait's default it takes.
func (b *builder) implMethod(tr *ir.Trait, name string, t ir.Ty) *ir.Func {
	for _, impl := range b.impls(tr) {
		if impl.For != nil && texprFits(impl.For, t) {
			if m := impl.Methods[name]; m != nil {
				return m
			}
		}
	}
	return nil
}

// impls lists a trait's impls in every module.
func (b *builder) impls(tr *ir.Trait) []*ir.Impl {
	var out []*ir.Impl
	for _, m := range b.prog.Modules {
		for _, impl := range m.Impls {
			if impl.Trait == tr {
				out = append(out, impl)
			}
		}
	}
	return out
}

// opImpl finds a declared type's impl method for an operator's trait whose
// right-hand side is r.
func (b *builder) opImpl(trait string, l, r ir.Ty) *ir.Func {
	var first *ir.Func
	for _, m := range b.prog.Modules {
		for _, impl := range m.Impls {
			if impl.Trait == nil || impl.Trait.Name != trait || impl.Trait.Module != b.prog.Prelude || impl.For == nil || !texprFits(impl.For, l) {
				continue
			}
			f := impl.Methods[strings.ToLower(trait)]
			if f == nil {
				continue
			}
			if first == nil {
				first = f
			}
			if r == nil || len(impl.Args) == 0 || texprFits(impl.Args[0], r) {
				return f
			}
		}
	}
	return first
}

// texprFits reports whether a type as written, with its lower-case type
// variables standing for anything, describes t.
func texprFits(te *ir.TypeExpr, t ir.Ty) bool {
	if te == nil {
		return false
	}
	if te.Name != "" && te.Name[0] >= 'a' && te.Name[0] <= 'z' && te.Decl == nil && te.Entity == nil && te.Trait == nil {
		return true // a type variable
	}
	switch x := t.(type) {
	case *ir.TCon:
		switch {
		case te.Decl != nil:
			if x.Decl != te.Decl {
				return false
			}
		case te.Entity != nil:
			return x.Entity == te.Entity
		default:
			if x.Decl != nil || x.Entity != nil || x.Name != te.Name {
				return false
			}
		}
		for i := range te.Args {
			if i < len(x.Args) && !texprFits(te.Args[i], x.Args[i]) {
				return false
			}
		}
		return true
	case *ir.TTuple:
		return te.Name == "tuple" && len(te.Args) == len(x.Elems)
	}
	return false
}

// instanceAt is the instance of target that a call with these types
// reaches: target's type variables take the call's types.
func (b *builder) instanceAt(target *ir.Func, call ir.Ty, kind Kind, parent *instance, at ir.Pos) *Func {
	sub := subst{}
	if target.Sig != nil {
		match(target.Sig, call, sub)
	}
	return b.request(target, sub, kind, parent, at)
}

// dynTable is the table of a type's impl of a trait, with an instance for
// each of the trait's methods.
func (b *builder) dynTable(t ir.Ty, tr *ir.Trait, parent *instance, at ir.Pos) *DynTable {
	key := typeKey(t) + " as " + tr.Name
	if d, ok := b.dyn[key]; ok {
		return d
	}
	d := &DynTable{Type: t, Trait: tr, Methods: map[string]*Func{}}
	b.dyn[key] = d
	b.out.Dyn = append(b.out.Dyn, d)
	for _, name := range tr.Order {
		m := b.implMethod(tr, name, t)
		if m == nil || m.Sig == nil {
			continue
		}
		sub := subst{}
		if len(m.Sig.Params) > 0 {
			match(m.Sig.Params[0], t, sub)
		}
		d.Methods[name] = b.request(m, sub, KindFunc, parent, at)
	}
	return d
}
