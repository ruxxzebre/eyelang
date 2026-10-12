package lir

import "eyelang/ir"

// What a back end needs to lay values out: the parts of a type, with the
// type's arguments put in for its type variables.

// RecordFields lists the fields of a record type in the order its
// declaration gives them, with their types, and reports whether t is a
// record type.
func RecordFields(t ir.Ty) ([]string, []ir.Ty, bool) {
	c, ok := t.(*ir.TCon)
	if !ok || c.Decl == nil {
		return nil, nil, false
	}
	d := c.Decl
	for d != nil && d.Alias != nil && d.Alias.Name != "record" && d.Alias.Decl != nil {
		d = d.Alias.Decl
	}
	if d == nil || d.Alias == nil || d.Alias.Name != "record" {
		return nil, nil, false
	}
	names := append([]string(nil), d.Alias.Fields...)
	types := make([]ir.Ty, len(names))
	for i, n := range names {
		types[i] = fieldTy(t, n)
	}
	return names, types, true
}

// Resolve follows a type alias, as `type Reason = String`, to the type it
// stands for; any other type is itself.
func Resolve(t ir.Ty) ir.Ty {
	for i := 0; i < 100; i++ {
		c, ok := t.(*ir.TCon)
		if !ok || c.Decl == nil || c.Decl.Alias == nil || c.Decl.Alias.Name == "record" || len(c.Decl.Variants) > 0 {
			return t
		}
		t = texprTy(c.Decl.Alias, envOf(t, c.Decl))
	}
	return t
}

// VariantFieldTy is the type of field i of variant v in a value of type t.
func VariantFieldTy(t ir.Ty, v *ir.Variant, i int) ir.Ty { return variantFieldTy(t, v, i) }

// ElemOf is the type of what a for loop over a collection of type t gives.
func ElemOf(t ir.Ty) ir.Ty { return elemOf(t) }

// TypeKey names a type uniquely in the program.
func TypeKey(t ir.Ty) string { return typeKey(t) }

// Dst is the value an instruction makes, or for SetPlace the variable it
// sets; nil for one that makes none.
func Dst(in Instr) *Value { return dstOf(in) }

// Reads lists the values an instruction reads.
func Reads(in Instr) []*Value { return readsOf(in) }
