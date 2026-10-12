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
// builtin.eye declares under "Strings and numbers", apart from those on
// positions (offsets.go). size, take and drop count characters.
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
		return &IterVal{Src: in.str(args, "chars", at)}
	})
	// Bytes are, for now, a list of Ints from 0 to 255: indexing, slicing,
	// size and patterns work on them as on any list.
	add("bytes", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		s := in.str(args, "bytes", at)
		out := make([]Value, len(s))
		for i := 0; i < len(s); i++ {
			out[i] = int64(s[i])
		}
		return &List{Elems: out}
	})
	add("toText", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "toText", at)
		es := in.elems(args[0], at)
		b := make([]byte, len(es))
		for i, e := range es {
			n, ok := e.(int64)
			if !ok || n < 0 || n > 255 {
				in.faultf(at, "toText takes Bytes, and element %d is %s", i, show(e))
			}
			b[i] = byte(n)
		}
		if !utf8.Valid(b) {
			return in.none()
		}
		return in.some(string(b))
	})
	add("toBytes", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "toBytes", at)
		return &List{Elems: in.elems(args[0], at)}
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
		if isInt(args[0]) {
			return args[0] // a fixed-width integer, which is an Int at run time
		}
		x, ok := args[0].(float64)
		if !ok {
			in.faultf(at, "toInt takes a Float or a fixed-width integer, and this is %s", kindName(args[0]))
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
// textLines is the lines of s, as `lines` and fs.lines give them: a line
// ends at "\n" or "\r\n", which it leaves out, and a final line break adds
// no empty line, so "" has no lines and "a\n" has one.
func textLines(s string) []Value {
	if s == "" {
		return nil
	}
	var out []Value
	for _, l := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
		out = append(out, strings.TrimSuffix(l, "\r"))
	}
	return out
}

func charOffset(s string, n int) int {
	for i := range s {
		if n == 0 {
			return i
		}
		n--
	}
	return len(s)
}
