package check

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"eyelang/ir"
)

// The fixed-width integers (SPECS.md §5): Byte, Int32, Int64, UInt32 and
// UInt64 have no arithmetic operators, only methods that say what happens
// at the edges of the type, and a literal that becomes one must fit it. An
// ASCII character literal is a Byte where one is expected.

// fixedRanges are the fixed-width integer types and the values each holds.
var fixedRanges = map[string]*[2]*big.Int{
	"Byte":   {big.NewInt(0), big.NewInt(255)},
	"Int32":  {big.NewInt(-1 << 31), big.NewInt(1<<31 - 1)},
	"Int64":  {big.NewInt(-1 << 63), big.NewInt(1<<63 - 1)},
	"UInt32": {big.NewInt(0), big.NewInt(1<<32 - 1)},
	"UInt64": {big.NewInt(0), new(big.Int).SetUint64(1<<64 - 1)},
}

// fixedName is the name of the fixed-width integer type t is, or "".
func fixedName(t typ) string {
	if k, ok := prune(t).(*tcon); ok && k.decl == nil && k.entity == nil && fixedRanges[k.name] != nil {
		return k.name
	}
	return ""
}

// fixedOp is the shape of a method of the fixed-width integers.
type fixedOp int

const (
	fixedSame    fixedOp = iota // a.f(b), both and the result of one type
	fixedChecked                // a.f(b), both of one type, giving an Option of it
	fixedUnary                  // a.f()
	fixedShift                  // a.f(by), by an Int
	fixedToInt                  // a.toInt(), which also takes a Float
)

// fixedOps are the methods of the fixed-width integers, which builtinWith
// types with fixedRule.
var fixedOps = map[string]fixedOp{
	"wrappingAdd": fixedSame, "wrappingSub": fixedSame, "wrappingMul": fixedSame,
	"checkedAdd": fixedChecked, "checkedSub": fixedChecked, "checkedMul": fixedChecked,
	"and": fixedSame, "or": fixedSame, "xor": fixedSame, "not": fixedUnary,
	"shiftLeft": fixedShift, "shiftRight": fixedShift, "toInt": fixedToInt,
}

// fixedOpMessage says why `l op r` isn't defined for a fixed-width type, and
// what to write instead.
func fixedOpMessage(name, op string) string {
	method := map[string]string{"+": "Add", "-": "Sub", "*": "Mul"}[op]
	if method == "" {
		return fmt.Sprintf("%s has no %s: convert to an Int with .toInt(), which can't overflow", name, op)
	}
	return fmt.Sprintf("%s has no %s, since the result may not fit: write a.wrapping%s(b) to wrap around, or a.checked%s(b) for None when it doesn't fit", name, op, method, method)
}

// fixedRule types a call of a method of the fixed-width integers. The
// receiver is the first argument.
func (x *fnCheck) fixedRule(name string, b *builtinArgs, at ir.Pos) typ {
	c := x.c
	x.ruleShape(name, b, at)
	shape := fixedOps[name]
	recv := b.arg(0, nil)
	if recv == nil {
		return c.fresh()
	}
	recvAt := b.pos[0].Value.Position()
	if shape == fixedToInt {
		t := prune(recv)
		if v, ok := t.(*tvar); ok && v.lit != litNone && !v.lit.intLiteral() {
			t = defaultLiteral(v) // "x".toInt(): a String
		}
		switch t := t.(type) {
		case tunknown:
			return t
		case *tcon:
			if fixedName(t) != "" {
				return tInt
			}
			if t.name != "Float" || t.decl != nil {
				return c.unknownf(recvAt, "toInt takes a Float or a fixed-width integer (Byte, Int32, Int64, UInt32 or UInt64), and this is %s", show(t))
			}
		}
		x.expect(recvAt, tFloat, recv)
		return &tcon{name: "Option", decl: c.ts.option, args: []typ{tInt}}
	}
	t := recv
	if v, ok := prune(recv).(*tvar); ok && v.lit.intLiteral() && (shape == fixedSame || shape == fixedChecked) {
		// `255.wrappingAdd(b)`: the other side says which type it is.
		if other := x.fixedArg(b, 1, "b", nil); other != nil && fixedName(other) != "" {
			x.expect(recvAt, other, recv)
			t = other
		}
	}
	if fixedName(t) == "" {
		if _, ok := prune(t).(tunknown); ok {
			return t
		}
		return c.unknownf(recvAt, "%s takes a fixed-width integer (Byte, Int32, Int64, UInt32 or UInt64), and this is %s%s", name, show(defaultLiteral(t)), fixedHint(name, t))
	}
	switch shape {
	case fixedSame:
		x.fixedArg(b, 1, "b", t)
		return t
	case fixedChecked:
		x.fixedArg(b, 1, "b", t)
		return &tcon{name: "Option", decl: c.ts.option, args: []typ{t}}
	case fixedShift:
		x.fixedArg(b, 1, "by", tInt)
	}
	return t
}

// fixedArg checks the argument after the receiver, given by position or by
// its name.
func (x *fnCheck) fixedArg(b *builtinArgs, i int, name string, want typ) typ {
	if t := b.arg(i, want); t != nil {
		return t
	}
	return b.name(name, want)
}

// fixedHint says what to write instead of a fixed-width method on another
// type.
func fixedHint(name string, t typ) string {
	arith := strings.HasPrefix(name, "wrapping") || strings.HasPrefix(name, "checked")
	switch k := defaultLiteral(t).(type) {
	case *tcon:
		switch {
		case k.name == "Int" && arith:
			return ": an Int never overflows, so use +, - and *"
		case k.name == "Int":
			return ": an Int has no fixed width; convert it first, as UInt64.from(n)"
		case k.name == "Bool" && name == "and":
			return ": for Bools, write a && b"
		case k.name == "Bool" && name == "or":
			return ": for Bools, write a || b"
		case k.name == "Bool" && name == "not":
			return ": for a Bool, write !a"
		}
	}
	return ""
}

// intLit is an integer literal, which must fit the fixed-width type it
// becomes.
type intLit struct {
	lit *ir.Lit
	t   typ
}

// charLit is a character literal, which is a Byte where one is expected.
type charLit struct {
	lit *ir.Lit
	t   typ
}

// settleLiterals checks the literals that became fixed-width integers, now
// that every type is known: an integer literal must fit its type, and a
// character literal that became a Byte must be ASCII, and becomes the
// number it is, so that the interpreter and the compiler see a Byte.
func (c *checker) settleLiterals() {
	for _, l := range c.ts.ints {
		name := fixedName(l.t)
		if name == "" {
			continue
		}
		v, at := l.lit.Value.(*big.Int), l.lit.Pos
		if neg, ok := c.ts.negated[l.lit]; ok {
			v, at = new(big.Int).Neg(v), neg
			at.End = l.lit.Pos.End
		}
		if r := fixedRanges[name]; v.Cmp(r[0]) < 0 || v.Cmp(r[1]) > 0 {
			article := "a"
			if name[0] == 'I' {
				article = "an"
			}
			c.typeErrorf(at, "%s doesn't fit in %s %s, which holds %s to %s", v, article, name, r[0], r[1])
		}
	}
	for _, l := range c.ts.chars {
		if fixedName(l.t) != "Byte" {
			continue
		}
		r := l.lit.Value.(rune)
		if msg := byteChar(r); msg != "" {
			c.typeErrorf(l.lit.Pos, "%s", msg)
			continue
		}
		l.lit.Value = big.NewInt(int64(r))
	}
}

// byteChar says why a character can't be a Byte, or "" if it can.
func byteChar(r rune) string {
	if r < 0x80 {
		return ""
	}
	return fmt.Sprintf("%s isn't ASCII, so it can't be a Byte: it is %d bytes in UTF-8; write the bytes as numbers, or match a String", strconv.QuoteRune(r), len(string(r)))
}

// written is where a part of a pattern is written, or at, the place of the
// whole, when the part doesn't know.
func written(p, at ir.Pos) ir.Pos {
	if p.File == nil {
		return at
	}
	return p
}

// literalPattern checks a literal in a pattern against t, the type of what
// the pattern matches, and returns the literal as the pattern keeps it: a
// character literal matching a Byte is a number.
func (x *fnCheck) literalPattern(v any, t typ, at ir.Pos) any {
	r, ok := v.(rune)
	if !ok {
		x.expect(at, t, x.literal(&ir.Lit{At: ir.At{Pos: at}, Value: v}, nil))
		return v
	}
	if fixedName(t) != "Byte" {
		x.expect(at, t, tChar)
		return v
	}
	if msg := byteChar(r); msg != "" {
		x.c.typeErrorf(at, "%s", msg)
		return v
	}
	return big.NewInt(int64(r))
}
