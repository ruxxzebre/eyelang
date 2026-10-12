package interp

import "eyelang/ir"

// ownRec is a record that one local variable holds and nothing else does,
// as the variable's frame slot holds it. Assigning to one of its fields,
// `p.x += 1`, or assigning a `with` of it back to the variable,
// `p = { p with x: 1 }`, changes it in place instead of copying it: no
// one else can see the change.
//
// A slot holds an ownRec only when the record in it is one the interpreter
// has just made: the copy an assignment to a field makes, or the value of a
// record literal or a `with` stored in the variable. Reading the variable
// as a whole gives the plain *Record and puts that back in the slot, since
// the record may then be held elsewhere too; reading one of its fields,
// `p.x`, doesn't. Only the record's own values are changed in place, never
// a record or list inside it, which may be shared.
type ownRec Record

// fresh reports whether e makes a new record that nothing else holds: a
// record literal or a `with`.
func fresh(e ir.Expr) bool {
	switch e.(type) {
	case *ir.Record, *ir.RecordUpdate:
		return true
	}
	return false
}

// own stores v, a new record that nothing else holds, in a slot as an
// ownRec.
func own(f *frame, slot int, v Value) {
	if r, ok := v.(*Record); ok {
		f.slots[slot] = (*ownRec)(r)
		return
	}
	f.slots[slot] = v
}

// localField reads a field of a local variable, `p.x`, without taking the
// record out of the slot: it stays an ownRec.
func (in *Interp) localField(f *frame, x *ir.Field) Value {
	if l, ok := x.Value.(*ir.Local); ok {
		if o, ok := f.slots[l.Slot].(*ownRec); ok {
			return in.field((*Record)(o), x.Name, x.Pos)
		}
	}
	return in.field(in.eval(f, x.Value), x.Name, x.Pos)
}

// assignWith runs `p = { … with … }` for a local variable p. When the base
// is p itself and p's slot holds an ownRec, it changes the record in place;
// otherwise it stores the new record as an ownRec.
func (in *Interp) assignWith(f *frame, slot int, x *ir.RecordUpdate) {
	l, ok := x.Base.(*ir.Local)
	if !ok || l.Slot != slot || len(x.Names) > 8 {
		own(f, slot, in.eval(f, x))
		return
	}
	o, ok := f.slots[slot].(*ownRec)
	if !ok {
		own(f, slot, in.eval(f, x))
		return
	}
	// Every value is computed from p as it was, before any is set.
	r := (*Record)(o)
	var idx [8]int
	var vals [8]Value
	for i, n := range x.Names {
		idx[i] = r.field(n)
		if idx[i] < 0 {
			in.faultf(x.Pos, "the record has no field %s", n)
		}
		vals[i] = in.eval(f, x.Values[i])
	}
	if f.slots[slot] != Value(o) {
		// A value read p as a whole, so the record may be held elsewhere
		// now: change a copy.
		r = r.copied()
	}
	for i := range x.Names {
		r.Vals[idx[i]] = vals[i]
	}
	own(f, slot, r)
}
