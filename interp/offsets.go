package interp

import (
	"strings"
	"unicode/utf8"

	"eyelang/ir"
)

// Positions in strings (SPECS.md §5): an Offset is the prelude's
// Offset(Int), a byte offset that only these functions make, move and use.

func (in *Interp) offset(i int) Value {
	return &Variant{V: in.pre.offset, Vals: []Value{int64(i)}}
}

// offsetOf is the byte offset an Offset holds.
func (in *Interp) offsetOf(v Value, name string, at ir.Pos) int {
	if o, ok := v.(*Variant); ok && o.V == in.pre.offset {
		if n, ok := o.Vals[0].(int64); ok {
			return int(n)
		}
	}
	in.faultf(at, "%s takes an Offset, such as s.start(), and this is %s", name, kindName(v))
	return 0
}

// charStart reports whether byte offset i of s is in it and starts a
// character, or is its end.
func charStart(s string, i int) bool {
	return i >= 0 && (i == len(s) || i < len(s) && utf8.RuneStart(s[i]))
}

func offsetBuiltins(add func(string, builtinFunc)) {
	add("start", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.str(args, "start", at)
		return in.offset(0)
	})
	add("end", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		return in.offset(len(in.str(args, "end", at)))
	})
	add("indexOf", func(in *Interp, args []Value, named map[string]Value, at ir.Pos) Value {
		if len(args) == 3 { // indexOf(s, part, i)
			named = map[string]Value{"from": args[2]}
			args = args[:2]
		}
		in.need(args, 2, "indexOf", at)
		s, ok1 := args[0].(string)
		part, ok2 := args[1].(string)
		if !ok1 || !ok2 {
			in.faultf(at, "indexOf takes two Strings")
		}
		from := 0
		if v, ok := named["from"]; ok {
			from = max(0, min(in.offsetOf(v, "indexOf's from:", at), len(s)))
		}
		i := strings.Index(s[from:], part)
		if i < 0 {
			return in.none()
		}
		return in.some(in.offset(from + i))
	})
	add("slice", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "slice", at)
		s, ok1 := args[0].(string)
		r, ok2 := args[1].(*RangeVal)
		if !ok1 || !ok2 {
			in.faultf(at, "slice takes a String and a range of positions: s.slice(s.start()..i)")
		}
		lo := max(0, min(in.offsetOf(r.Start, "slice", at), len(s)))
		hi := max(0, min(in.offsetOf(r.End, "slice", at), len(s)))
		if r.Inclusive && hi < len(s) {
			if !charStart(s, hi) {
				return in.none()
			}
			_, n := utf8.DecodeRuneInString(s[hi:])
			hi += n // ..= takes in the character at the end
		}
		if hi < lo {
			return in.some("")
		}
		if !charStart(s, lo) || !charStart(s, hi) {
			return in.none()
		}
		return in.some(s[lo:hi])
	})
	step := func(name string, forward bool) {
		add(name, func(in *Interp, args []Value, named map[string]Value, at ir.Pos) Value {
			if len(args) == 3 {
				named = map[string]Value{"by": args[2]}
				args = args[:2]
			}
			in.need(args, 2, name, at)
			s, ok := args[0].(string)
			if !ok {
				in.faultf(at, "%s takes a String and an Offset", name)
			}
			i := in.offsetOf(args[1], name, at)
			by := 1
			if v, ok := named["by"]; ok {
				by = in.count(v, at)
			}
			if !charStart(s, i) {
				return in.none()
			}
			for ; by > 0; by-- {
				var n int
				if forward {
					if i == len(s) {
						return in.none()
					}
					_, n = utf8.DecodeRuneInString(s[i:])
				} else {
					if i == 0 {
						return in.none()
					}
					_, n = utf8.DecodeLastRuneInString(s[:i])
					n = -n
				}
				i += n
			}
			return in.some(in.offset(i))
		})
	}
	step("after", true)
	step("before", false)
	add("charAt", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "charAt", at)
		s, ok := args[0].(string)
		if !ok {
			in.faultf(at, "charAt takes a String and an Offset")
		}
		i := in.offsetOf(args[1], "charAt", at)
		if i == len(s) || !charStart(s, i) {
			return in.none()
		}
		r, _ := utf8.DecodeRuneInString(s[i:])
		return in.some(r)
	})
}
