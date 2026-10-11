package interp

import (
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"

	"eyelang/ir"
)

// stringBuiltins adds the functions on strings, characters and Floats that
// builtin.eye declares under "Strings and numbers". Offsets into a string
// count bytes of UTF-8, as slice and indexOf use them; size, take and drop
// count characters.
func stringBuiltins(add func(string, builtinFunc)) {
	strs := func(in *Interp, args []Value, n int, name string, at ir.Pos) []string {
		in.need(args, n, name, at)
		out := make([]string, n)
		for i, a := range args {
			s, ok := a.(string)
			if !ok {
				in.faultf(at, "%s takes Strings, and argument %d is %s", name, i+1, kindName(a))
			}
			out[i] = s
		}
		return out
	}
	add("endsWith", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		s := strs(in, args, 2, "endsWith", at)
		return strings.HasSuffix(s[0], s[1])
	})
	add("replace", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		s := strs(in, args, 3, "replace", at)
		if s[1] == "" {
			return s[0] // replacing nothing changes nothing
		}
		return strings.ReplaceAll(s[0], s[1], s[2])
	})
	add("words", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		var out []Value
		for _, w := range strings.Fields(in.str(args, "words", at)) {
			out = append(out, w)
		}
		return &List{Elems: out}
	})
	add("chars", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		var out []Value
		for _, r := range in.str(args, "chars", at) {
			out = append(out, r)
		}
		return &List{Elems: out}
	})
	add("byteSize", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		return int64(len(in.str(args, "byteSize", at)))
	})
	add("indexOf", func(in *Interp, args []Value, named map[string]Value, at ir.Pos) Value {
		if len(args) == 3 { // indexOf(s, part, 4)
			named = map[string]Value{"from": args[2]}
			args = args[:2]
		}
		s := strs(in, args, 2, "indexOf", at)
		from := 0
		if v, ok := named["from"]; ok {
			n, ok := v.(int64)
			if !ok {
				in.faultf(at, "indexOf's from: is a byte offset, an Int")
			}
			from = int(max(0, min(n, int64(len(s[0])))))
		}
		i := strings.Index(s[0][from:], s[1])
		if i < 0 {
			return in.none()
		}
		return in.some(int64(from + i))
	})
	add("slice", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "slice", at)
		s, ok1 := args[0].(string)
		r, ok2 := args[1].(*RangeVal)
		if !ok1 || !ok2 {
			in.faultf(at, "slice takes a String and a range of byte offsets: s.slice(2..5)")
		}
		lo, ok1 := r.Start.(int64)
		hi, ok2 := r.End.(int64)
		if !ok1 || !ok2 {
			in.faultf(at, "slice takes a range of Ints")
		}
		if r.Inclusive {
			hi++
		}
		lo, hi = max(0, min(lo, int64(len(s)))), max(0, min(hi, int64(len(s))))
		if hi < lo {
			return in.some("")
		}
		if !charStart(s, lo) || !charStart(s, hi) {
			return in.none()
		}
		return in.some(s[lo:hi])
	})
	add("code", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "code", at)
		c, ok := args[0].(rune)
		if !ok {
			in.faultf(at, "code takes a Char, and this is %s", kindName(args[0]))
		}
		return int64(c)
	})
	add("fromCode", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "fromCode", at)
		n, ok := args[0].(int64)
		if !ok || n < 0 || n > utf8.MaxRune || !utf8.ValidRune(rune(n)) {
			return in.none()
		}
		return in.some(rune(n))
	})
	add("parseInt", func(in *Interp, args []Value, named map[string]Value, at ir.Pos) Value {
		if len(args) == 2 { // parseInt(s, 16)
			named = map[string]Value{"base": args[1]}
			args = args[:1]
		}
		s := in.str(args, "parseInt", at)
		base := int64(10)
		if v, ok := named["base"]; ok {
			b, ok := v.(int64)
			if !ok || b < 2 || b > 36 {
				in.faultf(at, "parseInt's base: is from 2 to 36")
			}
			base = b
		}
		n, ok := new(big.Int).SetString(strings.TrimSpace(s), int(base))
		if !ok {
			if base == 10 {
				return in.errV("not a whole number: " + strconv.Quote(s))
			}
			return in.errV("not a whole number in base " + strconv.FormatInt(base, 10) + ": " + strconv.Quote(s))
		}
		return in.ok(normInt(n))
	})
	add("parseFloat", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		s := in.str(args, "parseFloat", at)
		text := strings.TrimSpace(s)
		// Decimal only: no hex floats, no "inf" or "nan", no underscores.
		for _, r := range text {
			if !strings.ContainsRune("0123456789.eE+-", r) {
				return in.errV("not a number: " + strconv.Quote(s))
			}
		}
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
				return in.errV("too large for a Float: " + strconv.Quote(s))
			}
			return in.errV("not a number: " + strconv.Quote(s))
		}
		return in.ok(f)
	})
	floatFn := func(name string, f func(float64) float64) {
		add(name, func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
			in.need(args, 1, name, at)
			x, ok := args[0].(float64)
			if !ok {
				in.faultf(at, "%s takes a Float, and this is %s", name, kindName(args[0]))
			}
			return f(x)
		})
	}
	floatFn("round", math.Round)
	floatFn("floor", math.Floor)
	floatFn("ceil", math.Ceil)
	add("toInt", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "toInt", at)
		x, ok := args[0].(float64)
		if !ok {
			in.faultf(at, "toInt takes a Float, and this is %s", kindName(args[0]))
		}
		if math.IsInf(x, 0) || math.IsNaN(x) {
			return in.none()
		}
		n, _ := big.NewFloat(math.Trunc(x)).Int(nil)
		return in.some(normInt(n))
	})
	add("toFloat", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "toFloat", at)
		switch n := args[0].(type) {
		case int64:
			return float64(n)
		case *big.Int:
			f, _ := new(big.Float).SetInt(n).Float64()
			return f
		}
		in.faultf(at, "toFloat takes an Int, and this is %s", kindName(args[0]))
		return nil
	})
}

// charOffset returns the byte offset after the first n characters of s, or
// its length if it has fewer.
func charOffset(s string, n int) int {
	for i := range s {
		if n == 0 {
			return i
		}
		n--
	}
	return len(s)
}

// charStart reports whether byte offset i of s starts a character, or is
// its end.
func charStart(s string, i int64) bool {
	return i == int64(len(s)) || utf8.RuneStart(s[i])
}
