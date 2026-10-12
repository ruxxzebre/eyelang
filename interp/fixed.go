package interp

import (
	"math"
	"math/big"

	"eyelang/ir"
)

// The fixed-width integers (SPECS.md §5). At run time a Byte, Int32, Int64,
// UInt32 or UInt64 is an Int that its type holds, so comparing, hashing,
// showing and saving need nothing of their own. Their methods need the
// width, which the value doesn't carry: the call's receiver has its type in
// the typed IR, and evalMethod and evalCall pass it to fixedCall.

// width is a fixed-width integer type: how many bits it has, and whether
// it has a sign.
type width struct {
	bits   uint
	signed bool
}

var widths = map[string]width{
	"Byte":   {8, false},
	"Int32":  {32, true},
	"Int64":  {64, true},
	"UInt32": {32, false},
	"UInt64": {64, false},
}

// widthOf is the width of a fixed-width integer type.
func widthOf(t ir.Ty) (width, bool) {
	c, ok := t.(*ir.TCon)
	if !ok || c.Decl != nil || c.Entity != nil {
		return width{}, false
	}
	w, ok := widths[c.Name]
	return w, ok
}

// fixedMethods are the built-ins that need their receiver's width; toInt
// doesn't, since every fixed-width value is already an Int.
var fixedMethods = map[string]bool{
	"wrappingAdd": true, "wrappingSub": true, "wrappingMul": true,
	"checkedAdd": true, "checkedSub": true, "checkedMul": true,
	"and": true, "or": true, "xor": true, "not": true, "shiftLeft": true, "shiftRight": true,
}

func (w width) mask() uint64 {
	if w.bits == 64 {
		return math.MaxUint64
	}
	return 1<<w.bits - 1
}

// bitsOf is v's two's complement bits; v is an Int that w holds.
func bitsOf(v Value) uint64 {
	switch x := v.(type) {
	case int64:
		return uint64(x)
	case *big.Int:
		return x.Uint64() // a UInt64 above the largest int64
	}
	return 0
}

// value is the value of w whose bits are b; bits above w's width are
// dropped, which is what wraps a result around.
func (w width) value(b uint64) Value {
	b &= w.mask()
	if w.signed {
		shift := 64 - w.bits
		return int64(b<<shift) >> shift
	}
	if b > math.MaxInt64 {
		return new(big.Int).SetUint64(b)
	}
	return int64(b)
}

// bounds are the smallest and largest values w holds.
func (w width) bounds() (*big.Int, *big.Int) {
	if w.signed {
		hi := new(big.Int).Lsh(big.NewInt(1), w.bits-1)
		lo := new(big.Int).Neg(hi)
		return lo, hi.Sub(hi, big.NewInt(1))
	}
	hi := new(big.Int).Lsh(big.NewInt(1), w.bits)
	return big.NewInt(0), hi.Sub(hi, big.NewInt(1))
}

func (w width) holds(n *big.Int) bool {
	lo, hi := w.bounds()
	return n.Cmp(lo) >= 0 && n.Cmp(hi) <= 0
}

// fixedCall runs a method of the fixed-width integers on args, the receiver
// first, whose type is t.
func (in *Interp) fixedCall(name string, t ir.Ty, args []argVal, at ir.Pos) Value {
	w, ok := widthOf(t)
	if !ok {
		in.faultf(at, "%s takes a fixed-width integer, and this is %s", name, t)
	}
	pos, named := split(args)
	if len(pos) == 0 {
		in.faultf(at, "%s needs a fixed-width integer to work on", name)
	}
	a := pos[0]
	arg := func(param string) Value {
		if len(pos) > 1 {
			return pos[1]
		}
		if v, ok := named[param]; ok {
			return v
		}
		in.faultf(at, "%s needs an argument for %s", name, param)
		return nil
	}
	switch name {
	case "wrappingAdd":
		return w.value(bitsOf(a) + bitsOf(arg("b")))
	case "wrappingSub":
		return w.value(bitsOf(a) - bitsOf(arg("b")))
	case "wrappingMul":
		return w.value(bitsOf(a) * bitsOf(arg("b")))
	case "checkedAdd", "checkedSub", "checkedMul":
		x, _ := bigOf(a)
		y, _ := bigOf(arg("b"))
		r := new(big.Int)
		switch name {
		case "checkedAdd":
			r.Add(x, y)
		case "checkedSub":
			r.Sub(x, y)
		default:
			r.Mul(x, y)
		}
		if !w.holds(r) {
			return in.none()
		}
		return in.some(normInt(r))
	case "and":
		return w.value(bitsOf(a) & bitsOf(arg("b")))
	case "or":
		return w.value(bitsOf(a) | bitsOf(arg("b")))
	case "xor":
		return w.value(bitsOf(a) ^ bitsOf(arg("b")))
	case "not":
		return w.value(^bitsOf(a))
	case "shiftLeft":
		n := in.shiftCount(name, arg("by"), at)
		if n >= w.bits {
			return w.value(0)
		}
		return w.value(bitsOf(a) << n)
	case "shiftRight":
		n := in.shiftCount(name, arg("by"), at)
		if w.signed {
			// A signed value is a sign-extended int64, so >> copies its
			// sign bit down.
			return w.value(uint64(int64(bitsOf(a)) >> min(n, 63)))
		}
		if n >= w.bits {
			return w.value(0)
		}
		return w.value((bitsOf(a) & w.mask()) >> n)
	}
	in.faultf(at, "%s isn't a method of the fixed-width integers", name)
	return nil
}

// shiftCount is how far a shift moves bits: at most 64, since moving them
// further changes nothing more.
func (in *Interp) shiftCount(name string, v Value, at ir.Pos) uint {
	n, ok := bigOf(v)
	if !ok {
		in.faultf(at, "%s takes an Int to shift by, and this is %s", name, kindName(v))
	}
	if n.Sign() < 0 {
		in.faultf(at, "%s takes a count that isn't negative, and this is %s", name, n)
	}
	if !n.IsInt64() || n.Int64() > 64 {
		return 64
	}
	return uint(n.Int64())
}

// fixedBuiltins adds T.from for each fixed-width type, and the methods that
// need a width, which evalMethod and evalCall call with the receiver's type
// instead of through the table.
func fixedBuiltins(add func(string, builtinFunc)) {
	for name, w := range widths {
		add(name+".from", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
			in.need(args, 1, name+".from", at)
			n, ok := bigOf(args[0])
			if !ok {
				in.faultf(at, "%s.from takes an Int, and this is %s", name, kindName(args[0]))
			}
			if !w.holds(n) {
				return in.none()
			}
			return in.some(args[0])
		})
	}
	for name := range fixedMethods {
		add(name, func(in *Interp, _ []Value, _ map[string]Value, at ir.Pos) Value {
			in.faultf(at, "internal error: %s ran without the type of what it works on", name)
			return nil
		})
	}
}
