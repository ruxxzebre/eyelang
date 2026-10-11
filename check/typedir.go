package check

import (
	"sort"

	"eyelang/ir"
)

// The typed IR (ROADMAP 7.1): once a program's types are all known, each
// expression the checker typed gets its type in the IR, as an ir.Ty, so the
// interpreter and the compiler don't find types again at run time.

// typedExpr is an expression and the type the checker found for it.
type typedExpr struct {
	e ir.Expr
	t typ
}

// Bodies lists every function of a module that has a body, in source order:
// functions, methods, entity methods and their state defaults and
// migrations, jobs and their schedules, views, served operations, tests,
// trait defaults, impl methods and globals' initializers. Lambdas are inside
// them.
func Bodies(m *ir.Module) []*ir.Func {
	var out []*ir.Func
	add := func(f *ir.Func) {
		if f != nil && f.Body != nil {
			out = append(out, f)
		}
	}
	for _, f := range m.Funcs {
		add(f)
	}
	for _, g := range m.Globals {
		add(g.Init)
	}
	for _, t := range m.Types {
		for _, f := range t.Methods {
			if f.Receiver == t {
				add(f)
			}
		}
	}
	for _, e := range m.Entities {
		for _, f := range e.Methods {
			add(f)
		}
		for _, s := range e.States {
			add(s.Default)
		}
		for _, mg := range e.Migrations {
			add(mg.Func)
		}
	}
	for _, j := range m.Jobs {
		add(j.Func)
		for _, t := range j.Triggers {
			add(t.Schedule)
		}
	}
	for _, v := range m.Views {
		add(v.Func)
	}
	for _, ct := range m.Contracts {
		for _, op := range ct.Ops {
			add(op.Impl)
		}
	}
	for _, t := range m.Tests {
		add(t.Func)
	}
	for _, tr := range m.Traits {
		for _, name := range tr.Order {
			add(tr.Methods[name])
		}
	}
	for _, impl := range m.Impls {
		if impl.For == nil || impl.For.Decl != nil || impl.For.Entity != nil {
			continue // a declared type's impl methods are among its methods
		}
		for _, f := range impl.Methods {
			if impl.Trait == nil || impl.Trait.Methods[f.Name] != f {
				add(f)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Pos, out[j].Pos
		if a.File != b.File && a.File != nil && b.File != nil {
			return a.File.Path < b.File.Path
		}
		return a.Offset < b.Offset
	})
	return out
}

// writeTypes writes each checked expression's type into the IR. A few
// expressions are checked as part of the call around them rather than on
// their own: the name of a built-in or an entity that is called, as in
// `size(xs)` and `World(id)`. Those get the type of what they are at that
// call.
func (c *checker) writeTypes() {
	for _, te := range c.ts.exprs {
		if at, ok := te.e.(interface{ SetTy(ir.Ty) }); ok {
			at.SetTy(toTy(te.t))
		}
	}
	for fn, sig := range c.ts.sigs {
		fn.Sig, _ = toTy(sig).(*ir.TFun)
	}
	for _, m := range c.prog.Modules {
		for _, f := range Bodies(m) {
			walk(f.Body, false, func(e ir.Expr, _ bool) bool {
				if call, ok := e.(*ir.Call); ok && call.Func.Ty() == nil && call.Ty() != nil {
					setCallee(call)
				}
				if r, ok := e.(*ir.Ref); ok && r.Ty() == nil {
					if ent, ok := r.Decl.(*ir.Entity); ok {
						r.SetTy(&ir.TEntity{Entity: ent})
					}
				}
				return true
			})
		}
	}
}

// setCallee gives a called built-in its type at the call: the types of the
// arguments as written, and the call's result.
func setCallee(call *ir.Call) {
	r, ok := call.Func.(*ir.Ref)
	if !ok {
		return
	}
	if _, ok := r.Decl.(ir.Builtin); !ok {
		return
	}
	f := &ir.TFun{Ret: call.Ty()}
	for _, a := range call.Args {
		t := a.Value.Ty()
		if t == nil {
			return
		}
		f.Params = append(f.Params, t)
	}
	r.SetTy(f)
}

// toTy is a checker's type as a resolved type: nil where a part of it isn't
// known, which only a program with errors has.
func toTy(t typ) ir.Ty {
	switch x := defaultLiteral(t).(type) {
	case *tcon:
		args, ok := toTys(x.args)
		if !ok {
			return nil
		}
		return &ir.TCon{Name: x.name, Decl: x.decl, Entity: x.entity, Args: args, Uncertain: x.uncertain}
	case *tfun:
		params := make([]ir.Ty, len(x.params))
		for i, p := range x.params {
			if params[i] = toTy(p.t); params[i] == nil {
				return nil
			}
		}
		ret := toTy(x.ret)
		if ret == nil {
			return nil
		}
		f := &ir.TFun{Params: params, Ret: ret}
		if x.hasUses || len(x.uses) > 0 {
			f.Uses = append([]string{}, x.uses...)
		}
		return f
	case *ttuple:
		elems, ok := toTys(x.elems)
		if !ok {
			return nil
		}
		return &ir.TTuple{Elems: elems}
	case *tvar:
		if x.never {
			return ir.TNever{}
		}
		return &ir.TVar{ID: x.id, Name: x.name}
	case *tdyn:
		args, ok := toTys(x.args)
		if !ok {
			return nil
		}
		return &ir.TDyn{Trait: x.trait, Args: args}
	case *tjob:
		return &ir.TJob{Job: x.job}
	case *tentity:
		return &ir.TEntity{Entity: x.e}
	case tmodule:
		return ir.TModule{}
	case tdiscard:
		return &ir.TCon{Name: "()"}
	}
	return nil
}

func toTys(ts []typ) ([]ir.Ty, bool) {
	out := make([]ir.Ty, len(ts))
	for i, t := range ts {
		if out[i] = toTy(t); out[i] == nil {
			return nil, false
		}
	}
	return out, true
}
