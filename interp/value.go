// Package interp is Eyelang's reference interpreter and its in-memory
// runtime. It runs checked programs (package ir): main, test blocks,
// entities with transactional methods, views, and jobs that wait on a
// controllable clock and resume by replaying their recorded steps.
package interp

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"eyelang/ir"
)

// Value is a run-time value: int64 or *big.Int, float64, string, bool, rune,
// ir.Unit, ir.Duration, ir.Size, ir.Date, ir.TimeOfDay, time.Time, or one of
// the types below. Values never change; an update makes a new one.
type Value any

type (
	List struct{ Elems []Value }
	Set  struct {
		Elems []Value
		index map[string]int
	}
	Map struct {
		Keys, Vals []Value
		index      map[string]int
	}
	Record struct {
		Names []string
		Vals  []Value
	}
	Variant struct {
		V    *ir.Variant
		Vals []Value
	}
	Tuple struct{ Elems []Value }
	// Empty is `{}` before anything decides whether it is a record, map or set.
	Empty struct{}
	// Closure is a lambda with its captured values, or a function.
	Closure struct {
		Fn       *ir.Func
		Captured []Value
	}
	// BuiltinFn is a function provided by the interpreter.
	BuiltinFn struct {
		Name string
		Fn   func(in *Interp, args []Value, named map[string]Value, at ir.Pos) Value
	}
	// EntityRef is Ref(Entity): an entity's identity.
	EntityRef struct {
		Entity *ir.Entity
		ID     Value
	}
	// EntitySet is Entity.all.
	EntitySet struct{ Entity *ir.Entity }
	JobVal    struct{ Job *ir.Job }
	RunVal    struct{ ID int64 }
	ModuleVal struct{ Module *ir.Module }
	OpVal     struct{ Op *ir.Op }
	EventVal  struct{ Event *ir.Event }
	// ApprovalVal is an Approval: a person's yes to what a job asked.
	ApprovalVal struct{ What, By string }
	TypeVal     struct{ Type *ir.Type }
	ViewVal     struct{ View *ir.View }
	// handler is what a `with` statement applies in a test.
	handler interface{ apply(in *Interp, at ir.Pos) }
)

// ---------------------------------------------------------------- constructors

func newSet(elems []Value) *Set {
	s := &Set{index: map[string]int{}}
	for _, e := range elems {
		k := keyOf(e)
		if _, ok := s.index[k]; ok {
			continue
		}
		s.index[k] = len(s.Elems)
		s.Elems = append(s.Elems, e)
	}
	return s
}

func (s *Set) has(v Value) bool {
	_, ok := s.index[keyOf(v)]
	return ok
}

func newMap(keys, vals []Value) *Map {
	m := &Map{index: map[string]int{}}
	for i, k := range keys {
		m = m.with(k, vals[i])
	}
	return m
}

// with returns a copy of m with key set to val.
func (m *Map) with(key, val Value) *Map {
	k := keyOf(key)
	out := &Map{Keys: append([]Value(nil), m.Keys...), Vals: append([]Value(nil), m.Vals...), index: make(map[string]int, len(m.index)+1)}
	for kk, i := range m.index {
		out.index[kk] = i
	}
	if i, ok := out.index[k]; ok {
		out.Vals[i] = val
		return out
	}
	out.index[k] = len(out.Keys)
	out.Keys = append(out.Keys, key)
	out.Vals = append(out.Vals, val)
	return out
}

func (m *Map) get(key Value) (Value, bool) {
	i, ok := m.index[keyOf(key)]
	if !ok {
		return nil, false
	}
	return m.Vals[i], true
}

func (r *Record) get(name string) (Value, bool) {
	for i, n := range r.Names {
		if n == name {
			return r.Vals[i], true
		}
	}
	return nil, false
}

// with returns a copy of r with the field set.
func (r *Record) with(name string, v Value) *Record {
	out := &Record{Names: append([]string(nil), r.Names...), Vals: append([]Value(nil), r.Vals...)}
	for i, n := range out.Names {
		if n == name {
			out.Vals[i] = v
			return out
		}
	}
	out.Names = append(out.Names, name)
	out.Vals = append(out.Vals, v)
	return out
}

func (v *Variant) field(name string) (Value, bool) {
	for i, f := range v.V.Fields {
		if f.Name == name && i < len(v.Vals) {
			return v.Vals[i], true
		}
	}
	return nil, false
}

// prelude variants, found when the interpreter starts.
type preludeVariants struct{ some, none, ok, err *ir.Variant }

func (in *Interp) some(v Value) Value { return &Variant{V: in.pre.some, Vals: []Value{v}} }
func (in *Interp) none() Value        { return &Variant{V: in.pre.none} }
func (in *Interp) ok(v Value) Value   { return &Variant{V: in.pre.ok, Vals: []Value{v}} }
func (in *Interp) errV(v Value) Value { return &Variant{V: in.pre.err, Vals: []Value{v}} }

func isVariant(v Value, name string) (*Variant, bool) {
	x, ok := v.(*Variant)
	if !ok || x.V.Name != name {
		return nil, false
	}
	return x, true
}

// ---------------------------------------------------------------- integers

// normInt returns a *big.Int as an int64 when it fits.
func normInt(b *big.Int) Value {
	if b.IsInt64() {
		return b.Int64()
	}
	return b
}

func bigOf(v Value) (*big.Int, bool) {
	switch x := v.(type) {
	case int64:
		return big.NewInt(x), true
	case *big.Int:
		return x, true
	}
	return nil, false
}

func isInt(v Value) bool {
	switch v.(type) {
	case int64, *big.Int:
		return true
	}
	return false
}

func intArith(op string, a, b Value) (Value, error) {
	if x, ok := a.(int64); ok {
		if y, ok := b.(int64); ok {
			switch op {
			case "+":
				if r := x + y; (r > x) == (y > 0) {
					return r, nil
				}
			case "-":
				if r := x - y; (r < x) == (y > 0) {
					return r, nil
				}
			case "*":
				if x == 0 || y == 0 {
					return int64(0), nil
				}
				if r := x * y; r/y == x && (x != -1 || y != math.MinInt64) && (y != -1 || x != math.MinInt64) {
					return r, nil
				}
			case "/", "%":
				if y == 0 {
					return nil, fmt.Errorf("division by zero")
				}
				if x != math.MinInt64 || y != -1 {
					if op == "/" {
						return floorDiv(x, y), nil
					}
					return x - floorDiv(x, y)*y, nil
				}
			}
		}
	}
	x, _ := bigOf(a)
	y, _ := bigOf(b)
	r := new(big.Int)
	switch op {
	case "+":
		r.Add(x, y)
	case "-":
		r.Sub(x, y)
	case "*":
		r.Mul(x, y)
	case "/", "%":
		if y.Sign() == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		m := new(big.Int)
		r.DivMod(x, y, m) // Euclidean; adjust to floor division
		if m.Sign() != 0 && y.Sign() < 0 {
			r.Add(r, big.NewInt(1))
			m.Add(m, y)
		}
		if op == "%" {
			return normInt(m), nil
		}
	}
	return normInt(r), nil
}

func floorDiv(x, y int64) int64 {
	q := x / y
	if (x%y != 0) && ((x < 0) != (y < 0)) {
		q--
	}
	return q
}

// ---------------------------------------------------------------- keys and equality

// keyOf encodes a value so that equal values have equal keys. Sets and maps
// use it, and so does ==.
func keyOf(v Value) string {
	var b strings.Builder
	writeKey(&b, v)
	return b.String()
}

func writeKey(b *strings.Builder, v Value) {
	switch x := v.(type) {
	case int64:
		b.WriteString("i")
		b.WriteString(strconv.FormatInt(x, 10))
	case *big.Int:
		b.WriteString("i")
		b.WriteString(x.String())
	case float64:
		b.WriteString("f")
		b.WriteString(strconv.FormatFloat(x, 'g', -1, 64))
	case string:
		b.WriteString("s")
		b.WriteString(strconv.Itoa(len(x)))
		b.WriteString(":")
		b.WriteString(x)
	case bool:
		if x {
			b.WriteString("T")
		} else {
			b.WriteString("F")
		}
	case rune:
		b.WriteString("c")
		b.WriteString(strconv.Itoa(int(x)))
	case ir.Unit:
		b.WriteString("()")
	case ir.Duration:
		b.WriteString("d")
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case ir.Size:
		b.WriteString("z")
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case ir.Date:
		fmt.Fprintf(b, "D%04d-%02d-%02d", x.Year, x.Month, x.Day)
	case ir.TimeOfDay:
		fmt.Fprintf(b, "t%02d:%02d:%02d", x.Hour, x.Minute, x.Second)
	case time.Time:
		b.WriteString("I")
		b.WriteString(strconv.FormatInt(x.UnixNano(), 10))
	case *List:
		b.WriteString("[")
		for _, e := range x.Elems {
			writeKey(b, e)
			b.WriteString(",")
		}
		b.WriteString("]")
	case *Tuple:
		b.WriteString("(")
		for _, e := range x.Elems {
			writeKey(b, e)
			b.WriteString(",")
		}
		b.WriteString(")")
	case *Set:
		keys := make([]string, 0, len(x.Elems))
		for k := range x.index {
			keys = append(keys, k)
		}
		writeSorted(b, keys)
	case *Map:
		keys := make([]string, 0, len(x.Keys))
		for i, k := range x.Keys {
			keys = append(keys, keyOf(k)+"="+keyOf(x.Vals[i]))
		}
		writeSorted(b, keys)
	case *Record:
		keys := make([]string, 0, len(x.Names))
		for i, n := range x.Names {
			keys = append(keys, n+"="+keyOf(x.Vals[i]))
		}
		writeSorted(b, keys)
	case Empty:
		b.WriteString("{}")
	case *Variant:
		b.WriteString("v")
		b.WriteString(variantName(x.V))
		b.WriteString("(")
		for _, e := range x.Vals {
			writeKey(b, e)
			b.WriteString(",")
		}
		b.WriteString(")")
	case *EntityRef:
		b.WriteString("e")
		b.WriteString(x.Entity.Module.Path + "." + x.Entity.Name)
		b.WriteString("(")
		writeKey(b, x.ID)
		b.WriteString(")")
	case *RunVal:
		b.WriteString("r")
		b.WriteString(strconv.FormatInt(x.ID, 10))
	default:
		fmt.Fprintf(b, "?%p", v)
	}
}

// writeSorted writes a set-like key: empty collections of every kind are
// equal, since `{}` is any of them.
func writeSorted(b *strings.Builder, keys []string) {
	sort.Strings(keys)
	b.WriteString("{")
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(",")
	}
	b.WriteString("}")
}

func variantName(v *ir.Variant) string {
	if v.Type == nil {
		return v.Name
	}
	return v.Type.Module.Path + "." + v.Type.Name + "." + v.Name
}

func equal(a, b Value) bool {
	switch x := a.(type) {
	case int64:
		if y, ok := b.(int64); ok {
			return x == y
		}
	case string:
		if y, ok := b.(string); ok {
			return x == y
		}
	case bool:
		if y, ok := b.(bool); ok {
			return x == y
		}
	}
	return keyOf(a) == keyOf(b)
}

// compare orders two values of the same ordered kind.
func compare(a, b Value) (int, bool) {
	switch x := a.(type) {
	case *Variant:
		// Variants of one type are ordered as they are declared, then by
		// their fields: Low < Normal < High.
		y, ok := b.(*Variant)
		if !ok || x.V.Type != y.V.Type || x.V.Type == nil {
			return 0, false
		}
		if x.V != y.V {
			return variantIndex(x.V) - variantIndex(y.V), true
		}
		for i := range x.Vals {
			if i >= len(y.Vals) {
				break
			}
			if c, ok := compare(x.Vals[i], y.Vals[i]); !ok || c != 0 {
				return c, ok
			}
		}
		return 0, true
	case int64, *big.Int:
		if !isInt(b) {
			return 0, false
		}
		if xi, ok := x.(int64); ok {
			if yi, ok := b.(int64); ok {
				switch {
				case xi < yi:
					return -1, true
				case xi > yi:
					return 1, true
				}
				return 0, true
			}
		}
		xb, _ := bigOf(a)
		yb, _ := bigOf(b)
		return xb.Cmp(yb), true
	case float64:
		y, ok := b.(float64)
		if !ok {
			return 0, false
		}
		switch {
		case x < y:
			return -1, true
		case x > y:
			return 1, true
		}
		return 0, true
	case string:
		y, ok := b.(string)
		if !ok {
			return 0, false
		}
		return strings.Compare(x, y), true
	case rune:
		y, ok := b.(rune)
		if !ok {
			return 0, false
		}
		return int(x - y), true
	case ir.Duration:
		y, ok := b.(ir.Duration)
		if !ok {
			return 0, false
		}
		return cmpInt64(int64(x), int64(y)), true
	case ir.Size:
		y, ok := b.(ir.Size)
		if !ok {
			return 0, false
		}
		return cmpInt64(int64(x), int64(y)), true
	case ir.Date:
		y, ok := b.(ir.Date)
		if !ok {
			return 0, false
		}
		return strings.Compare(keyOf(x), keyOf(y)), true
	case ir.TimeOfDay:
		y, ok := b.(ir.TimeOfDay)
		if !ok {
			return 0, false
		}
		return strings.Compare(keyOf(x), keyOf(y)), true
	case time.Time:
		y, ok := b.(time.Time)
		if !ok {
			return 0, false
		}
		return x.Compare(y), true
	}
	return 0, false
}

func cmpInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// ---------------------------------------------------------------- showing

// show gives a value as string interpolation prints it: strings as they
// are, everything else in its literal form.
func show(v Value) string {
	if s, ok := v.(string); ok {
		return s
	}
	if r, ok := v.(rune); ok {
		return string(r)
	}
	var b strings.Builder
	writeShow(&b, v, false)
	return b.String()
}

func writeShow(b *strings.Builder, v Value, nested bool) {
	switch x := v.(type) {
	case *SecretVal:
		// The checker will refuse to show a secret; until then only its
		// name is ever written.
		b.WriteString("secret(" + strconv.Quote(x.name) + ")")
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
	case *big.Int:
		b.WriteString(x.String())
	case float64:
		s := strconv.FormatFloat(x, 'g', -1, 64)
		if !strings.ContainsAny(s, ".eEn") {
			s += ".0"
		}
		b.WriteString(s)
	case string:
		if nested {
			b.WriteString(strconv.Quote(x))
		} else {
			b.WriteString(x)
		}
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case rune:
		if nested {
			b.WriteString(strconv.QuoteRune(x))
		} else {
			b.WriteRune(x)
		}
	case ir.Unit:
		b.WriteString("()")
	case ir.Duration:
		b.WriteString(showDuration(x))
	case ir.Size:
		b.WriteString(showSize(x))
	case ir.Date:
		fmt.Fprintf(b, "%04d-%02d-%02d", x.Year, x.Month, x.Day)
	case ir.TimeOfDay:
		fmt.Fprintf(b, "%02d:%02d", x.Hour, x.Minute)
		if x.Second != 0 {
			fmt.Fprintf(b, ":%02d", x.Second)
		}
	case time.Time:
		b.WriteString(x.Format("2006-01-02 15:04"))
		b.WriteString(` in "` + x.Location().String() + `"`)
	case *List:
		b.WriteString("[")
		for i, e := range x.Elems {
			if i > 0 {
				b.WriteString(", ")
			}
			writeShow(b, e, true)
		}
		b.WriteString("]")
	case *Tuple:
		b.WriteString("(")
		for i, e := range x.Elems {
			if i > 0 {
				b.WriteString(", ")
			}
			writeShow(b, e, true)
		}
		b.WriteString(")")
	case *Set:
		if len(x.Elems) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteString("{")
		for i, e := range x.Elems {
			if i > 0 {
				b.WriteString(", ")
			}
			writeShow(b, e, true)
		}
		b.WriteString("}")
	case *Map:
		if len(x.Keys) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteString("{ ")
		for i, k := range x.Keys {
			if i > 0 {
				b.WriteString(", ")
			}
			writeShow(b, k, true)
			b.WriteString(": ")
			writeShow(b, x.Vals[i], true)
		}
		b.WriteString(" }")
	case *Record:
		b.WriteString("{ ")
		for i, n := range x.Names {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(n + ": ")
			writeShow(b, x.Vals[i], true)
		}
		b.WriteString(" }")
	case Empty:
		b.WriteString("{}")
	case *Variant:
		if x.V.Type != nil && x.V.Type.Newtype() && len(x.Vals) == 1 {
			writeShow(b, x.Vals[0], nested)
			return
		}
		b.WriteString(x.V.Name)
		if len(x.Vals) == 0 {
			return
		}
		b.WriteString("(")
		for i, e := range x.Vals {
			if i > 0 {
				b.WriteString(", ")
			}
			if i < len(x.V.Fields) && x.V.Fields[i].Name != "" {
				b.WriteString(x.V.Fields[i].Name + ": ")
			}
			writeShow(b, e, true)
		}
		b.WriteString(")")
	case *EntityRef:
		b.WriteString(x.Entity.Name + "(")
		writeShow(b, x.ID, true)
		b.WriteString(")")
	case *RunVal:
		fmt.Fprintf(b, "Run(%d)", x.ID)
	case *Closure:
		b.WriteString("fn " + x.Fn.Name)
	case *BuiltinFn:
		b.WriteString("fn " + x.Name)
	default:
		fmt.Fprintf(b, "%v", v)
	}
}

func showDuration(d ir.Duration) string {
	units := []struct {
		name string
		ns   int64
	}{{"days", 86400e9}, {"h", 3600e9}, {"min", 60e9}, {"s", 1e9}, {"ms", 1e6}}
	for _, u := range units {
		if int64(d) != 0 && int64(d)%u.ns == 0 {
			n := int64(d) / u.ns
			if u.name == "days" && n == 1 {
				return "1 day"
			}
			return strconv.FormatInt(n, 10) + " " + u.name
		}
	}
	return strconv.FormatInt(int64(d)/1e6, 10) + " ms"
}

func showSize(s ir.Size) string {
	units := []struct {
		name string
		n    int64
	}{{"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10}}
	for _, u := range units {
		if int64(s) != 0 && int64(s)%u.n == 0 {
			return strconv.FormatInt(int64(s)/u.n, 10) + " " + u.name
		}
	}
	return strconv.FormatInt(int64(s), 10) + " B"
}

// kindName describes a value's kind in messages.
func kindName(v Value) string {
	switch x := v.(type) {
	case *SecretVal:
		return "Secret"
	case int64, *big.Int:
		return "Int"
	case float64:
		return "Float"
	case string:
		return "String"
	case bool:
		return "Bool"
	case rune:
		return "Char"
	case ir.Unit:
		return "()"
	case ir.Duration:
		return "Duration"
	case ir.Size:
		return "Size"
	case ir.Date:
		return "Date"
	case ir.TimeOfDay:
		return "Time"
	case time.Time:
		return "Instant"
	case *List:
		return "List"
	case *Set:
		return "Set"
	case *Map:
		return "Map"
	case *Record:
		return "a record"
	case *Tuple:
		return "a tuple"
	case Empty:
		return "{}"
	case *Variant:
		if x.V.Type != nil {
			return x.V.Type.Name
		}
		return x.V.Name
	case *EntityRef:
		return "Ref(" + x.Entity.Name + ")"
	case *Closure, *BuiltinFn:
		return "a function"
	}
	return fmt.Sprintf("%T", v)
}

func variantIndex(v *ir.Variant) int {
	for i, w := range v.Type.Variants {
		if w == v {
			return i
		}
	}
	return -1
}
