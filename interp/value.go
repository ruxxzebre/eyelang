// Package interp is Eyelang's reference interpreter and its in-memory
// runtime. It runs checked programs (package ir): main, test blocks,
// entities with transactional methods, views, and jobs that wait on a
// controllable clock and resume by replaying their recorded steps.
package interp

import (
	"fmt"
	"maps"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"eyelang/ir"
)

// growth is how much of a backing array the lists that share it use.
type growth struct{ used atomic.Int64 }

// appended returns l with vals after its elements, l itself unchanged. It
// takes constant time on average, so a list built one element at a time,
// `xs = xs |> add(x)` or `xs += [x]`, takes linear time, not quadratic.
//
// A list made by appended shares a backing array, with room to spare, with
// the list it came from. Appending to the list that uses all of the array
// writes into the spare room: no other list can see those slots, since each
// reads only its own length. Appending to an older list, whose next slot
// another list already holds, copies. The compare-and-swap makes that safe
// when tasks append to one list at once.
func (l *List) appended(vals ...Value) *List {
	n, m := len(l.Elems), len(l.Elems)+len(vals)
	if g := l.grow; g != nil && m <= cap(l.Elems) && g.used.CompareAndSwap(int64(n), int64(m)) {
		return &List{Elems: append(l.Elems, vals...), grow: g}
	}
	elems := make([]Value, m, max(2*m, 4))
	copy(elems, l.Elems)
	copy(elems[n:], vals)
	g := &growth{}
	g.used.Store(int64(m))
	return &List{Elems: elems, grow: g}
}

// Value is a run-time value: int64 or *big.Int, float64, string, bool, rune,
// ir.Unit, ir.Duration, ir.Size, ir.Date, ir.TimeOfDay, time.Time, or one of
// the types below. Values never change; an update makes a new one.
type Value any

type (
	// List is a list value. Never write into Elems in place: lists made by
	// appended share one backing array, so a write would change every list
	// that shares it. Build a new slice instead, as sort and reverse do; an
	// in-place operation, such as element assignment or a mut method, if one
	// ever comes, must copy Elems first.
	List struct {
		Elems []Value
		// grow is shared by the lists that share Elems' backing array: how
		// much of it they use. See appended.
		grow *growth
	}
	// Set is a set value, its elements in the order they were added. Make
	// one with newSet or added; never write into Elems.
	Set struct {
		Elems []Value
		shelf *shelf // shared with the sets Elems shares an array with
	}
	// Map is a map value, its entries in the order they were inserted.
	// Make one with newMap or with; never write into Keys. Its values are
	// in a persistent vector, in the same order: read them with val or
	// values.
	Map struct {
		Keys  []Value
		vals  vec
		shelf *shelf // shared with the maps Keys shares an array with
	}
	// Record is a record value. Records copied to change a value share
	// Names: never write into Names, and never append to the Names of a
	// record that is already made.
	Record struct {
		Names []string
		Vals  []Value
		// Type is the record's declared type, which its operators and Show
		// come from; nil for a record of no declared type, as an entity's
		// snapshot or a fake's call.
		Type *ir.Type
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
	// handler is what a `with` statement applies in a test.
	handler interface{ apply(in *Interp, at ir.Pos) }
)

// ---------------------------------------------------------------- constructors

// shelf is what sets, or maps, made one from another share, so that adding
// to one takes constant time on average, as appending to a list does (see
// appended): the array their keys sit in, and the slot of each key.
//
// Slots fill in order and never change once filled, and each set or map
// uses the first n of them, n its own length. So each key is in at most one
// slot, and a set holds a key when the key's slot is below its length: an
// older, shorter set never sees what a newer one added. A set that uses
// every filled slot adds a key by filling the next one, which no other set
// can see; any other set copies its own slots to a new shelf first. The
// lock makes that safe when tasks add to sets on one shelf at once.
//
// Replacing a map key's value leaves the key in its slot, so the new map
// shares the old one's shelf; only the vec of values changes, in time that
// grows with the logarithm of the map's size. A persistent hash map would
// make every insertion take that time; the shelf keeps inserting a new key,
// and finding one, as fast as a Go map.
type shelf struct {
	mu    sync.RWMutex
	index map[string]int // each key's slot; len(index) slots are filled
	keys  []Value        // Set.Elems' or Map.Keys' array, to its capacity
}

// slot finds key k among the first n slots.
func (sh *shelf) slot(k string, n int) (int, bool) {
	if sh == nil {
		return 0, false
	}
	sh.mu.RLock()
	i, ok := sh.index[k]
	sh.mu.RUnlock()
	return i, ok && i < n
}

// fill puts a key in slot n, if n is the next slot to fill and there is
// room; k is key's keyOf. It reports whether it did. The caller has checked
// that no slot below n holds k.
func (sh *shelf) fill(n int, k string, key Value) bool {
	if sh == nil {
		return false
	}
	sh.mu.Lock()
	defer sh.mu.Unlock()
	if _, taken := sh.index[k]; taken || len(sh.index) != n || n >= len(sh.keys) {
		return false
	}
	sh.keys[n] = key
	sh.index[k] = n
	return true
}

// add returns the shelf that holds keys and then key, filling slot
// len(keys) of sh when it can and copying keys to a new shelf when it
// can't. The caller has checked that keys doesn't hold key.
func (sh *shelf) add(keys []Value, k string, key Value) *shelf {
	n := len(keys)
	if !sh.fill(n, k, key) {
		sh = newShelf(sh, keys)
		sh.keys[n] = key
		sh.index[k] = n
	}
	return sh
}

// newShelf returns a shelf of its own for keys, with room for more; old is
// the shelf they came from, or nil.
func newShelf(old *shelf, keys []Value) *shelf {
	n := len(keys)
	sh := &shelf{keys: make([]Value, max(2*n, 4))}
	copy(sh.keys, keys)
	if old != nil {
		// Taking every key from a full shelf is quicker than working them
		// out again.
		old.mu.RLock()
		if len(old.index) == n {
			sh.index = maps.Clone(old.index)
		}
		old.mu.RUnlock()
	}
	if sh.index == nil {
		sh.index = make(map[string]int, len(sh.keys))
		for i, k := range keys {
			sh.index[keyOf(k)] = i
		}
	}
	return sh
}

func newSet(elems []Value) *Set {
	s := &Set{}
	for _, e := range elems {
		s = s.added(e)
	}
	return s
}

// added returns s with v in it, s itself unchanged. It takes constant time
// on average; see shelf.
func (s *Set) added(v Value) *Set {
	k, n := keyOf(v), len(s.Elems)
	if _, ok := s.shelf.slot(k, n); ok {
		return s
	}
	sh := s.shelf.add(s.Elems, k, v)
	return &Set{Elems: sh.keys[: n+1 : n+1], shelf: sh}
}

func (s *Set) has(v Value) bool {
	_, ok := s.shelf.slot(keyOf(v), len(s.Elems))
	return ok
}

// keys returns the keyOf of each element, in no order.
func (s *Set) keys() []string {
	out := make([]string, 0, len(s.Elems))
	if sh := s.shelf; sh != nil {
		sh.mu.RLock()
		if len(sh.index) == len(s.Elems) {
			for k := range sh.index {
				out = append(out, k)
			}
		}
		sh.mu.RUnlock()
	}
	if len(out) != len(s.Elems) {
		out = out[:0]
		for _, e := range s.Elems {
			out = append(out, keyOf(e))
		}
	}
	return out
}

func newMap(keys, vals []Value) *Map {
	m := &Map{}
	for i, k := range keys {
		m = m.with(k, vals[i])
	}
	return m
}

// with returns m with key set to val, m itself unchanged. Inserting a new
// key takes constant time on average, and replacing a key's value time that
// grows with the logarithm of the map's size (see shelf and vec).
func (m *Map) with(key, val Value) *Map {
	k, n := keyOf(key), len(m.Keys)
	if i, has := m.shelf.slot(k, n); has {
		return &Map{Keys: m.Keys, vals: m.vals.set(i, val), shelf: m.shelf}
	}
	sh := m.shelf.add(m.Keys, k, key)
	return &Map{Keys: sh.keys[: n+1 : n+1], vals: m.vals.push(val), shelf: sh}
}

func (m *Map) get(key Value) (Value, bool) {
	i, ok := m.shelf.slot(keyOf(key), len(m.Keys))
	if !ok {
		return nil, false
	}
	return m.vals.get(i), true
}

// val returns the value of the i-th key, Keys[i].
func (m *Map) val(i int) Value { return m.vals.get(i) }

// values returns m's values in the order of its keys, as a new slice.
func (m *Map) values() []Value { return m.vals.values() }

func (r *Record) get(name string) (Value, bool) {
	for i, n := range r.Names {
		if n == name {
			return r.Vals[i], true
		}
	}
	return nil, false
}

// Records of a few fields, with room for their values, so that copying a
// record to change a field makes one allocation, not two, of the same size.
type (
	record2 struct {
		r   Record
		buf [2]Value
	}
	record4 struct {
		r   Record
		buf [4]Value
	}
	record8 struct {
		r   Record
		buf [8]Value
	}
)

// copied returns a copy of r to change values of: it has its own Vals, and
// shares Names, which no record changes in place.
func (r *Record) copied() *Record {
	var out *Record
	n := len(r.Vals)
	switch {
	case n <= 2:
		b := &record2{}
		b.r.Vals = b.buf[:n:n]
		out = &b.r
	case n <= 4:
		b := &record4{}
		b.r.Vals = b.buf[:n:n]
		out = &b.r
	case n <= 8:
		b := &record8{}
		b.r.Vals = b.buf[:n:n]
		out = &b.r
	default:
		out = &Record{Vals: make([]Value, n)}
	}
	copy(out.Vals, r.Vals)
	out.Names, out.Type = r.Names, r.Type
	return out
}

// field returns the index of r's field called name, or -1.
func (r *Record) field(name string) int {
	for i, n := range r.Names {
		if n == name {
			return i
		}
	}
	return -1
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
type preludeVariants struct {
	some, none, ok, err, problem, offset *ir.Variant
	// What a @fresh command may answer besides its declared variants.
	stale, expired *ir.Variant
}

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
		writeSorted(b, x.keys())
	case *Map:
		keys, vals := make([]string, 0, len(x.Keys)), x.values()
		for i, k := range x.Keys {
			keys = append(keys, keyOf(k)+"="+keyOf(vals[i]))
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

// typeName is a declared type's name qualified by its module's path, as it
// is saved.
func typeName(t *ir.Type) string {
	if t.Module == nil {
		return t.Name
	}
	return t.Module.Path + "." + t.Name
}

func variantName(v *ir.Variant) string {
	if v.Type == nil {
		return v.Name
	}
	return v.Type.Module.Path + "." + v.Type.Name + "." + v.Name
}

// equal reports whether a == b: whether their keys are equal. It compares
// the common kinds directly, part by part, and builds keys only for the
// rest.
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
	case float64:
		if y, ok := b.(float64); ok {
			// As their keys: NaN equals NaN, and 0.0 doesn't equal -0.0.
			if x == y {
				return math.Signbit(x) == math.Signbit(y)
			}
			return x != x && y != y
		}
	case rune:
		if y, ok := b.(rune); ok {
			return x == y
		}
	case *Variant:
		if y, ok := b.(*Variant); ok {
			if x.V != y.V && (x.V.Name != y.V.Name || variantName(x.V) != variantName(y.V)) {
				return false
			}
			return equalAll(x.Vals, y.Vals)
		}
	case *Record:
		if y, ok := b.(*Record); ok {
			return equalRecords(x, y)
		}
	case *Tuple:
		if y, ok := b.(*Tuple); ok {
			return equalAll(x.Elems, y.Elems)
		}
	case *List:
		if y, ok := b.(*List); ok {
			return equalAll(x.Elems, y.Elems)
		}
	}
	return keyOf(a) == keyOf(b)
}

// equalAll reports whether two lists of values are equal one by one.
func equalAll(xs, ys []Value) bool {
	if len(xs) != len(ys) {
		return false
	}
	for i := range xs {
		if !equal(xs[i], ys[i]) {
			return false
		}
	}
	return true
}

// equalRecords reports whether two records have the same fields with equal
// values, in any order and whatever their types, as their keys do.
func equalRecords(x, y *Record) bool {
	if len(x.Names) != len(y.Names) {
		return false
	}
	for i, n := range x.Names {
		var yv Value
		if i < len(y.Names) && y.Names[i] == n {
			yv = y.Vals[i]
		} else if v, ok := y.get(n); ok {
			yv = v
		} else {
			return false
		}
		if !equal(x.Vals[i], yv) {
			return false
		}
	}
	return true
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
	case *Record:
		// Records part by part, in the order their type declares the fields.
		y, ok := b.(*Record)
		if !ok || len(x.Names) != len(y.Names) {
			return 0, false
		}
		for i, name := range x.Names {
			yv, has := y.get(name)
			if !has {
				return 0, false
			}
			if c, ok := compare(x.Vals[i], yv); !ok || c != 0 {
				return c, ok
			}
		}
		return 0, true
	case *Tuple:
		y, ok := b.(*Tuple)
		if !ok || len(x.Elems) != len(y.Elems) {
			return 0, false
		}
		for i := range x.Elems {
			if c, ok := compare(x.Elems[i], y.Elems[i]); !ok || c != 0 {
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

func writeShow(b *strings.Builder, v Value, nested bool) { writeShowIn(b, v, nested, nil) }

// writeShowIn writes v in its derived form; when custom isn't nil, it
// writes each value inside v whose type has an impl of Show through that
// impl (SPECS.md §11).
func writeShowIn(b *strings.Builder, v Value, nested bool, custom func(Value) (string, bool)) {
	if custom != nil {
		if s, ok := custom(v); ok {
			b.WriteString(s)
			return
		}
	}
	switch x := v.(type) {
	case *SecretVal:
		// The checker will refuse to show a secret; until then only its
		// name is ever written.
		b.WriteString("secret(" + strconv.Quote(x.name) + ")")
	case *ForeignVal:
		// The checker refuses to show one; a step's value in eye runs may.
		b.WriteString("<" + x.Type.Name + ">")
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
			writeShowIn(b, e, true, custom)
		}
		b.WriteString("]")
	case *Tuple:
		b.WriteString("(")
		for i, e := range x.Elems {
			if i > 0 {
				b.WriteString(", ")
			}
			writeShowIn(b, e, true, custom)
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
			writeShowIn(b, e, true, custom)
		}
		b.WriteString("}")
	case *Map:
		if len(x.Keys) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteString("{ ")
		vals := x.values()
		for i, k := range x.Keys {
			if i > 0 {
				b.WriteString(", ")
			}
			writeShowIn(b, k, true, custom)
			b.WriteString(": ")
			writeShowIn(b, vals[i], true, custom)
		}
		b.WriteString(" }")
	case *Record:
		b.WriteString("{ ")
		for i, n := range x.Names {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(n + ": ")
			writeShowIn(b, x.Vals[i], true, custom)
		}
		b.WriteString(" }")
	case Empty:
		b.WriteString("{}")
	case *Variant:
		if x.V.Type != nil && x.V.Type.Newtype() && len(x.Vals) == 1 {
			writeShowIn(b, x.Vals[0], nested, custom)
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
			writeShowIn(b, e, true, custom)
		}
		b.WriteString(")")
	case *EntityRef:
		b.WriteString(x.Entity.Name + "(")
		writeShowIn(b, x.ID, true, custom)
		b.WriteString(")")
	case *RunVal:
		fmt.Fprintf(b, "Run(%d)", x.ID)
	case *Closure:
		b.WriteString("fn " + x.Fn.Name)
	case *BuiltinFn:
		b.WriteString("fn " + x.Name)
	case *IterVal:
		b.WriteString("Iter(…)")
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
	case *ForeignVal:
		return x.Type.Name
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
	case *GroupVal:
		return "TaskGroup"
	case *SenderVal:
		return "Sender"
	case *ReceiverVal:
		return "Receiver"
	case *IterVal:
		return "Iter"
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
