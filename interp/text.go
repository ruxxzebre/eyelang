package interp

import (
	"unsafe"

	"eyelang/ir"
)

// textBuf is a String that a local variable is building with `+=`, as the
// variable's frame slot holds it. It never leaves the slot: reading the
// variable gives a plain Go string (plain), so every other part of the
// interpreter sees only strings.
//
// Go's `x + y` copies both strings, so a loop that adds to one string took
// time that grows with the square of its length. A textBuf has room to
// spare, and appending to it writes into that room, as List.appended does:
// the bytes a string read from it covers are never written again, and an
// older textBuf, or a string read from one, reads only its own length. So
// a string built one piece at a time takes time in proportion to its
// length.
type textBuf struct {
	b []byte
	// grow is shared by the textBufs that share b's array: how much of it
	// they use.
	grow *growth
}

// string returns the text as a Go string, without copying it: the bytes
// below len(t.b) never change.
func (t *textBuf) string() string { return unsafe.String(unsafe.SliceData(t.b), len(t.b)) }

// appendText returns old, a string or a textBuf, with s after it. It takes
// constant time on average.
func appendText(old Value, s string) *textBuf {
	var prefix string
	switch o := old.(type) {
	case *textBuf:
		n, m := len(o.b), len(o.b)+len(s)
		if m <= cap(o.b) && o.grow.used.CompareAndSwap(int64(n), int64(m)) {
			return &textBuf{b: append(o.b, s...), grow: o.grow}
		}
		prefix = o.string()
	case string:
		prefix = o
	}
	m := len(prefix) + len(s)
	b := make([]byte, 0, max(2*m, 64))
	b = append(append(b, prefix...), s...)
	g := &growth{}
	g.used.Store(int64(m))
	return &textBuf{b: b, grow: g}
}

// plain returns v as the rest of the interpreter reads it: a textBuf as a
// string, anything else as it is.
func plain(v Value) Value {
	if t, ok := v.(*textBuf); ok {
		return t.string()
	}
	return v
}

// appendInPlace runs `s += t`, or `s = s + t`, on a local variable s that
// holds a String, by appending t to the textBuf s holds, and reports
// whether it did. It leaves every other assignment to the caller.
func (in *Interp) appendInPlace(f *frame, x *ir.Assign) bool {
	p := x.Target
	if p.Self || len(p.Path) > 0 {
		return false
	}
	var old, add Value
	switch x.Op {
	case "+=":
		// As for any compound assignment, the value comes first, then
		// the variable is read.
		v := f.slots[p.Slot]
		if !isText(v) {
			return false
		}
		add = in.eval(f, x.Value)
		old = f.slots[p.Slot]
	case "=":
		b, ok := x.Value.(*ir.Binary)
		if !ok || b.Op != "+" {
			return false
		}
		if l, ok := b.L.(*ir.Local); !ok || l.Slot != p.Slot {
			return false
		}
		old = f.slots[p.Slot]
		if !isText(old) {
			return false
		}
		add = in.eval(f, b.R)
	default:
		return false
	}
	s, ok := add.(string)
	if !ok || !isText(old) {
		f.slots[p.Slot] = in.binary("+", plain(old), add, x.Pos)
		return true
	}
	f.slots[p.Slot] = appendText(old, s)
	return true
}

func isText(v Value) bool {
	switch v.(type) {
	case string, *textBuf:
		return true
	}
	return false
}
