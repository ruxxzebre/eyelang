package lir

import "eyelang/ir"

// A value written in its derived form writes the values inside it (in a
// list, an Option, a record, a variant) through their own types' impls of
// Show when they have one (SPECS.md §11), as the interpreter does. The
// derived Show is the runtime's, which walks a value by its type; it calls
// the impls of the types Program.Shows names.

// WalkShown calls visit with t and each type whose values a value of type
// t holds, as the derived Show walks them: a collection's elements, a
// tuple's and a record's fields, each variant's fields. It doesn't go
// inside a type for which visit says false, and visits each type once.
func WalkShown(t ir.Ty, visit func(ir.Ty) bool) {
	walkShown(t, map[string]bool{}, visit)
}

func walkShown(t ir.Ty, seen map[string]bool, visit func(ir.Ty) bool) {
	t = Resolve(t)
	if t == nil {
		return
	}
	k := typeKey(t)
	if seen[k] {
		return
	}
	seen[k] = true
	if !visit(t) {
		return
	}
	switch x := t.(type) {
	case *ir.TTuple:
		for _, e := range x.Elems {
			walkShown(e, seen, visit)
		}
	case *ir.TCon:
		switch {
		case x.Entity != nil:
		case x.Decl != nil && len(x.Decl.Variants) > 0:
			for _, v := range x.Decl.Variants {
				for i := range v.Fields {
					walkShown(variantFieldTy(x, v, i), seen, visit)
				}
			}
		case x.Decl != nil:
			if _, fts, ok := RecordFields(x); ok {
				for _, f := range fts {
					walkShown(f, seen, visit)
				}
			}
		default:
			for _, a := range x.Args {
				walkShown(a, seen, visit)
			}
		}
	}
}

// noteShows finds the impls of Show of the declared types inside a value
// of type t that is written in its derived form, and asks for an instance
// of each, which Program.Shows keeps by the type's key.
func (b *builder) noteShows(t ir.Ty, parent *instance, at ir.Pos) {
	tr := b.prog.Prelude.Traits["Show"]
	if tr == nil || generic(t) {
		return
	}
	WalkShown(t, func(u ir.Ty) bool {
		c, ok := u.(*ir.TCon)
		if !ok || c.Decl == nil {
			return true
		}
		k := typeKey(u)
		if _, done := b.out.Shows[k]; done {
			return false
		}
		if m := b.implMethod(tr, "show", u); m != nil && m.Body != nil {
			if b.out.Shows == nil {
				b.out.Shows = map[string]*Func{}
			}
			b.out.Shows[k] = b.instanceAt(m, &ir.TFun{Params: []ir.Ty{u}, Ret: tyStr}, KindFunc, parent, at)
			return false
		}
		return true
	})
}
