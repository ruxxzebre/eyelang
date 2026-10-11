package interp

import "eyelang/ir"

// Traits at run time: the checker has settled which impl each use means, and
// the interpreter finds it again from the value. Operators on declared types
// call their impls of Add, Sub, Mul, Div, Rem and Neg; an impl of Show gives
// how a value is written.

var opMethods = map[string]struct{ trait, method string }{
	"+": {"Add", "add"}, "-": {"Sub", "sub"}, "*": {"Mul", "mul"}, "/": {"Div", "div"}, "%": {"Rem", "rem"},
}

// typeOf is the declared type of a record or a variant, or nil for a value
// of no declared type.
func (in *Interp) typeOf(v Value) *ir.Type {
	switch x := v.(type) {
	case *Variant:
		return x.V.Type
	case *Record:
		return x.Type
	}
	return nil
}

// userOp runs `a op b` through a's impl of the operator's trait, choosing
// among several impls by the right-hand side.
func (in *Interp) userOp(op string, a, b Value, at ir.Pos) (Value, bool) {
	om, ok := opMethods[op]
	if !ok {
		return nil, false
	}
	t := in.typeOf(a)
	if t == nil {
		return nil, false
	}
	var pick *ir.Impl
	for _, impl := range t.Impls {
		if impl.Trait == nil || impl.Trait.Name != om.trait || len(impl.Args) == 0 {
			continue
		}
		if pick == nil || in.fits(impl.Args[0], b) {
			pick = impl
		}
	}
	if pick == nil {
		return nil, false
	}
	fn := pick.Methods[om.method]
	if fn == nil {
		return nil, false
	}
	r, _ := in.callMethodFunc(fn, a, []argVal{{v: b}}, at)
	return r, true
}

// userNeg runs `-a` through a's impl of Neg.
func (in *Interp) userNeg(a Value, at ir.Pos) (Value, bool) {
	t := in.typeOf(a)
	if t == nil {
		return nil, false
	}
	for _, impl := range t.Impls {
		if impl.Trait == nil || impl.Trait.Name != "Neg" || impl.Trait.Module != in.prog.Prelude {
			continue
		}
		if fn := impl.Methods["neg"]; fn != nil {
			r, _ := in.callMethodFunc(fn, a, nil, at)
			return r, true
		}
	}
	return nil, false
}

// fits reports whether a value is of a type as written, as far as an impl's
// choice needs: its declared type, or a built-in type's name.
func (in *Interp) fits(t *ir.TypeExpr, v Value) bool {
	if t == nil {
		return false
	}
	if t.Decl != nil {
		return in.typeOf(v) == t.Decl
	}
	return t.Name == kindName(v)
}

// display writes a value as `"{…}"` and print do: through its type's impl
// of Show, if it has one, and otherwise in the derived form.
func (in *Interp) display(v Value) string {
	if t := in.typeOf(v); t != nil {
		for _, impl := range t.Impls {
			if impl.Trait != nil && impl.Trait.Name == "Show" && impl.Trait.Module == in.prog.Prelude {
				if fn := impl.Methods["show"]; fn != nil {
					r, _ := in.callMethodFunc(fn, v, nil, fn.Pos)
					if s, ok := r.(string); ok {
						return s
					}
				}
			}
		}
	}
	return show(v)
}

// enumFn is `T.variants` or `T.parse`, derived on a type whose variants have
// no fields: every variant in the order declared, or Some of the variant
// whose name is the argument, else None (SPECS.md §11).
func enumFn(d ir.EnumFn) *BuiltinFn {
	name := d.Type.Name + "." + d.Name
	if d.Name == "variants" {
		return &BuiltinFn{Name: name, Fn: func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
			in.need(args, 0, name, at)
			out := make([]Value, len(d.Type.Variants))
			for i, v := range d.Type.Variants {
				out[i] = &Variant{V: v}
			}
			return &List{Elems: out}
		}}
	}
	return &BuiltinFn{Name: name, Fn: func(in *Interp, args []Value, named map[string]Value, at ir.Pos) Value {
		if v, ok := named["name"]; ok {
			args = append(args, v)
		}
		s := in.str(args, name, at)
		for _, v := range d.Type.Variants {
			if v.Name == s {
				return in.some(&Variant{V: v})
			}
		}
		return in.none()
	}}
}
