package interp

import "eyelang/ir"

// Traits at run time: the checker has settled which impl each use means, and
// the interpreter finds it again from the value. Operators on declared types
// call their impls of Add, Sub, Mul, Div and Rem; an impl of Show gives how a
// value is written.

var opMethods = map[string]struct{ trait, method string }{
	"+": {"Add", "add"}, "-": {"Sub", "sub"}, "*": {"Mul", "mul"}, "/": {"Div", "div"}, "%": {"Rem", "rem"},
}

// typeOf finds the declared type of a record or a variant.
func (in *Interp) typeOf(v Value) *ir.Type {
	switch x := v.(type) {
	case *Variant:
		return x.V.Type
	case *Record:
		for _, m := range in.prog.Modules {
			for _, t := range m.Types {
				if len(t.Fields) != len(x.Names) || len(t.Fields) == 0 {
					continue
				}
				match := true
				for _, n := range t.Fields {
					if _, has := x.get(n); !has {
						match = false
					}
				}
				if match {
					return t
				}
			}
		}
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
