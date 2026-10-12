package lir

import (
	"fmt"
	"sort"
	"strings"

	"eyelang/ir"
)

// How values are laid out in memory (ROADMAP 7.13,
// docs/internals/compilation.md, "Values laid out by type"). The low IR is
// monomorphic, so each type's representation is chosen from the type
// alone: a back end reads it here instead of holding every value in one
// word with everything else on the heap. Values without aliasing are what
// permit it: nothing can notice that a record was copied into registers, or
// that two strings share bytes.

// Repr is how the values of a type are held.
type Repr uint8

const (
	// ReprNone holds nothing: () and Never.
	ReprNone Repr = iota
	// ReprScalar is a number of a machine type (Scalar names it): Bool,
	// Byte, Char, the fixed-width integers, Float, Duration, Size, Date and
	// Time. Never counted.
	ReprScalar
	// ReprInt is Int: one word, a small integer n as (n << 1) | 1 or a
	// pointer to a counted big integer.
	ReprInt
	// ReprEnum is a variant type whose variants have no fields: the
	// variant's number, in one byte (two past 256 variants).
	ReprEnum
	// ReprStruct is held inline, by value, as a C struct: a record, a
	// tuple, a range, an Instant, or a variant type with fields that is
	// neither large nor recursive. Counted when a field is.
	ReprStruct
	// ReprBox is a record, tuple or variant type in a counted heap object:
	// one word, a pointer to the object, or for a variant without fields an
	// immediate (n << 1) | 1. Large types are boxed, and recursive ones.
	ReprBox
	// ReprString is String, Bytes, Key and Zone: a view of counted bytes,
	// {p, len, own}, which slices share.
	ReprString
	// ReprObject is a pointer to a counted object of the runtime: a List,
	// Set or Map buffer, an Iter, a closure, a dyn value, and the runner's
	// values that compiled code doesn't hold yet.
	ReprObject
)

var reprNames = [...]string{"none", "scalar", "int", "enum", "struct", "box", "string", "object"}

func (r Repr) String() string { return reprNames[r] }

const (
	// MaxInline is the largest record, tuple or variant type held inline,
	// in bytes; a larger one is boxed. Copying 64 bytes costs a few moves,
	// less than allocating and freeing an object, and the C ABI passes any
	// struct over 16 bytes in memory anyway, so the cost is flat up to
	// here; past it copies and the dups of many fields cost more than a
	// box's one count.
	MaxInline = 64
	// HeaderSize is the size of a heap object's header: a 32-bit count and
	// 32 bits the object's kind uses (a boxed variant's number, a closure's
	// size, …). A box's fields start after it.
	HeaderSize = 8
	// wordSize is the size of a pointer and of an Int.
	wordSize = 8
)

// Layout is how the values of one monomorphic type are laid out.
type Layout struct {
	Ty    ir.Ty
	Repr  Repr
	Size  int // bytes where a value is held: a variable, a field, an element
	Align int
	// Counted reports whether copying a value needs a dup and ending one a
	// drop: an Int, a String, an object, a box, or an inline value with a
	// counted field.
	Counted bool
	// Scalar is the C type of a ReprScalar or ReprEnum value.
	Scalar string
	// Fields are what a ReprStruct or ReprBox record, tuple, range or
	// Instant holds, in declared order, each at its offset: from the start
	// of the struct, or of a box's object (after its header). A variant
	// type has Variants instead.
	Fields []Slot
	// Variants are a variant type's variants in declared order, with their
	// fields and how a value says which it is (Tag).
	Variants []VariantLayout
	Tag      Tag
	// Niche is the spare values of a part of the value that no value of the
	// type uses, which a variant type holding it can use for its variants
	// without fields: Option(Int)'s None is 0, a word no Int is.
	Niche Niche
	// Elem is the element type of a List, Set or Iter; Key and Val a Map's.
	Elem, Key, Val ir.Ty
	// Recursive reports a box chosen because the type's declaration holds
	// itself, as `type Tree = Leaf | Node(Tree, Tree)` does.
	Recursive bool

	payload bool // a box's Fields and Variants are worked out
}

// Slot is one field of an inline value or a box, or of a variant.
type Slot struct {
	Name   string // a record's field name; "0", "1", … for a tuple's elements and positional fields
	Offset int
	L      *Layout // a box field's layout may not have its own payload worked out: Layouts.Of gives it
}

// VariantLayout is one variant of a variant type.
type VariantLayout struct {
	Variant *ir.Variant
	Fields  []Slot
	// Value says how a value of the type is this variant: its number for
	// TagField and ReprEnum, the value of the niche for a variant without
	// fields under TagNiche, the immediate word for one without fields
	// under TagBox. It means nothing for the variant with fields under
	// TagNiche, which is any other value.
	Value uint64
}

// TagKind is how a variant type's value says which variant it is.
type TagKind uint8

const (
	// TagNone: the type has one variant.
	TagNone TagKind = iota
	// TagField: a tag of Tag.Size bytes at Tag.Offset holds the variant's
	// number. A ReprEnum value is its tag.
	TagField
	// TagNiche: one variant has fields and the others don't; those others
	// are values of a part of the first that no value of it uses (its
	// niche), Tag.Size bytes at Tag.Offset.
	TagNiche
	// TagBox: a boxed variant type. A variant without fields is the
	// immediate word (n << 1) | 1; one with fields is a pointer to an
	// object whose header holds n.
	TagBox
)

var tagNames = [...]string{"none", "field", "niche", "box"}

func (k TagKind) String() string { return tagNames[k] }

// Tag is where a variant type's value says which variant it is.
type Tag struct {
	Kind         TagKind
	Offset, Size int
}

// Niche is a set of values of Size bytes at Offset that no value of the
// type uses: Start, Start+Step, … (Count of them). Count 0 means none.
type Niche struct {
	Offset, Size int
	Start, Step  uint64
	Count        int
}

func (n Niche) value(i int) uint64 { return n.Start + uint64(i)*n.Step }

// rest is the niche left after the first k values are used.
func (n Niche) rest(k int) Niche {
	n.Start += uint64(k) * n.Step
	n.Count -= k
	if n.Count <= 0 {
		return Niche{}
	}
	return n
}

// The niches of the words the runtime holds. A pointer to an object is
// never below 4096; an Int's or a box's word may be odd (a small integer,
// an immediate), so only its even values below 4096 are spare.
var (
	pointerNiche = Niche{Size: wordSize, Start: 0, Step: 1, Count: 4096}
	wordNiche    = Niche{Size: wordSize, Start: 0, Step: 2, Count: 2048}
)

// Layouts works out and keeps the layout of each type of a program.
type Layouts struct {
	byKey     map[string]*Layout
	recursive map[*ir.Type]bool
	// CountedBy, when set, decides which types are counted instead of
	// their layouts: a back end whose values aren't laid out by type yet
	// counts by its own rule.
	CountedBy func(ir.Ty) bool
}

// NewLayouts is an empty table of layouts.
func NewLayouts() *Layouts {
	return &Layouts{byKey: map[string]*Layout{}, recursive: map[*ir.Type]bool{}}
}

// Of is the layout of t, with a box's fields worked out.
func (ls *Layouts) Of(t ir.Ty) *Layout {
	l := ls.shallow(t)
	if l.Repr == ReprBox && !l.payload {
		l.payload = true
		ls.boxPayload(l)
	}
	return l
}

// Counted reports whether values of t are counted.
func (ls *Layouts) Counted(t ir.Ty) bool {
	if ls.CountedBy != nil {
		return ls.CountedBy(t)
	}
	return ls.shallow(t).Counted
}

// shallow is the layout of t, leaving a box's fields for Of: a box is a
// word whatever it holds, and a recursive type's fields may name ever
// bigger instances of it.
func (ls *Layouts) shallow(t ir.Ty) *Layout {
	t = Resolve(t)
	key := typeKey(t)
	if c, ok := t.(*ir.TCon); ok && (c.Uncertain || c.Fresh) {
		// A command's result, which may also be Uncertain(key), Stale or
		// Expired: the runner's value.
		key = "uncertain " + key
		if l, ok := ls.byKey[key]; ok {
			return l
		}
		l := object(t)
		ls.byKey[key] = l
		return l
	}
	if l, ok := ls.byKey[key]; ok {
		return l
	}
	l := ls.compute(t)
	if prev, ok := ls.byKey[key]; ok {
		return prev // a newtype's field registered it already
	}
	ls.byKey[key] = l
	return l
}

func scalarOf(c string, size int) *Layout {
	return &Layout{Repr: ReprScalar, Size: size, Align: size, Scalar: c}
}

var scalarLayouts = map[string]struct {
	c    string
	size int
}{
	"Bool": {"bool", 1}, "Byte": {"uint8_t", 1}, "Char": {"uint32_t", 4},
	"Int32": {"int32_t", 4}, "UInt32": {"uint32_t", 4}, "Int64": {"int64_t", 8}, "UInt64": {"uint64_t", 8},
	"Float": {"double", 8}, "Duration": {"int64_t", 8}, "Size": {"int64_t", 8},
	"Date": {"int64_t", 8}, "Time": {"int32_t", 4},
}

func (ls *Layouts) compute(t ir.Ty) *Layout {
	switch x := t.(type) {
	case ir.TNever:
		return &Layout{Ty: t, Repr: ReprNone, Align: 1}
	case *ir.TTuple:
		slots := make([]Slot, len(x.Elems))
		for i, e := range x.Elems {
			slots[i] = Slot{Name: fmt.Sprint(i), L: ls.shallow(e)}
		}
		return record(t, slots)
	case *ir.TFun, *ir.TDyn:
		return object(t)
	case *ir.TCon:
		return ls.con(x)
	}
	return object(t) // jobs, entities and modules as values: the runner's
}

func object(t ir.Ty) *Layout {
	return &Layout{Ty: t, Repr: ReprObject, Size: wordSize, Align: wordSize, Counted: true, Niche: pointerNiche}
}

func (ls *Layouts) con(c *ir.TCon) *Layout {
	if c.Entity != nil {
		return object(c)
	}
	if d := c.Decl; d != nil {
		switch {
		case d.Extern:
			return object(c)
		case ls.isRecursive(d):
			// Boxed before its fields are looked at, which name it.
			return box(c, true)
		case d.Newtype():
			inner := ls.shallow(VariantFieldTy(c, d.Variants[0], 0))
			return inner
		case len(d.Variants) > 0:
			return ls.variants(c)
		}
		names, types, ok := RecordFields(c)
		if !ok {
			return object(c)
		}
		slots := make([]Slot, len(names))
		for i, n := range names {
			slots[i] = Slot{Name: n, L: ls.shallow(types[i])}
		}
		return record(c, slots)
	}
	if s, ok := scalarLayouts[c.Name]; ok {
		l := scalarOf(s.c, s.size)
		l.Ty = c
		switch c.Name {
		case "Bool":
			l.Niche = Niche{Size: 1, Start: 2, Step: 1, Count: 254}
		case "Char":
			l.Niche = Niche{Size: 4, Start: 0x110000, Step: 1, Count: 1 << 20}
		}
		return l
	}
	switch c.Name {
	case "()":
		return &Layout{Ty: c, Repr: ReprNone, Align: 1}
	case "Int", "Version":
		return &Layout{Ty: c, Repr: ReprInt, Size: wordSize, Align: wordSize, Counted: true, Niche: wordNiche}
	case "NonZero":
		if len(c.Args) == 1 {
			return ls.shallow(c.Args[0])
		}
	case "String", "Bytes", "Key", "Zone":
		ptr := scalarOf("const uint8_t *", wordSize)
		ptr.Niche = pointerNiche
		own := object(&ir.TCon{Name: "bytes"})
		l := &Layout{Ty: c, Repr: ReprString, Counted: true, Fields: []Slot{
			{Name: "p", L: ptr}, {Name: "len", L: scalarOf("int64_t", wordSize)}, {Name: "own", L: own},
		}}
		place(l, l.Fields, 0)
		l.Niche = ptr.Niche // at offset 0: p is never below 4096
		return l
	case "Instant":
		l := &Layout{Ty: c, Repr: ReprStruct, Fields: []Slot{
			{Name: "ns", L: scalarOf("int64_t", wordSize)}, {Name: "zone", L: scalarOf("const ey_zone *", wordSize)},
		}}
		place(l, l.Fields, 0)
		return l
	case "Range":
		if len(c.Args) == 1 {
			el := ls.shallow(c.Args[0])
			return record(c, []Slot{{Name: "start", L: el}, {Name: "end", L: el}, {Name: "flags", L: scalarOf("uint8_t", 1)}})
		}
	case "List", "Set", "Iter":
		l := object(c)
		if len(c.Args) == 1 {
			l.Elem = c.Args[0]
		}
		return l
	case "Map":
		l := object(c)
		if len(c.Args) == 2 {
			l.Key, l.Val = c.Args[0], c.Args[1]
		}
		return l
	}
	return object(c)
}

// place gives each slot an offset, largest alignment first so that little
// is lost to padding (declared order among equals), from base on, and sets
// l's size and alignment.
func place(l *Layout, slots []Slot, base int) {
	order := make([]int, len(slots))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return slots[order[a]].L.Align > slots[order[b]].L.Align })
	off, align := base, 1
	for _, i := range order {
		s := &slots[i]
		off = roundUp(off, s.L.Align)
		s.Offset = off
		off += s.L.Size
		align = max(align, s.L.Align)
		if s.L.Counted {
			l.Counted = true
		}
	}
	l.Size, l.Align = roundUp(off, align), align
}

func roundUp(n, align int) int {
	if align <= 1 {
		return n
	}
	return (n + align - 1) / align * align
}

// record lays out a record, a tuple or a range that doesn't hold itself:
// inline, or boxed when it is large.
func record(t ir.Ty, slots []Slot) *Layout {
	l := &Layout{Ty: t, Repr: ReprStruct, Fields: slots}
	place(l, l.Fields, 0)
	l.Niche = firstNiche(l.Fields)
	if l.Size > MaxInline {
		return box(t, false)
	}
	l.payload = true
	return l
}

func box(t ir.Ty, recursive bool) *Layout {
	return &Layout{Ty: t, Repr: ReprBox, Size: wordSize, Align: wordSize, Counted: true, Niche: wordNiche, Recursive: recursive}
}

// firstNiche is the niche of the first slot, by offset, that has one.
func firstNiche(slots []Slot) Niche {
	best := Niche{}
	bestOff := -1
	for _, s := range slots {
		if s.L.Niche.Count > 0 && (bestOff < 0 || s.Offset < bestOff) {
			best, bestOff = s.L.Niche, s.Offset
			best.Offset += s.Offset
		}
	}
	return best
}

// variants lays out a variant type that isn't a newtype.
func (ls *Layouts) variants(c *ir.TCon) *Layout {
	d := c.Decl
	n := len(d.Variants)
	vs := make([]VariantLayout, n)
	var dataful []int
	for i, v := range d.Variants {
		vs[i].Variant = v
		for j, f := range v.Fields {
			name := f.Name
			if name == "" {
				name = fmt.Sprint(j)
			}
			vs[i].Fields = append(vs[i].Fields, Slot{Name: name, L: ls.shallow(VariantFieldTy(c, v, j))})
		}
		if len(v.Fields) > 0 {
			dataful = append(dataful, i)
		}
	}
	tagSize := 1
	if n > 256 {
		tagSize = 2
	}
	if len(dataful) == 0 {
		l := &Layout{Ty: c, Repr: ReprEnum, Size: tagSize, Align: tagSize, Variants: vs, Tag: Tag{Kind: TagField, Size: tagSize}, payload: true}
		l.Scalar = "uint8_t"
		limit := 256
		if tagSize == 2 {
			l.Scalar, limit = "uint16_t", 65536
		}
		for i := range vs {
			vs[i].Value = uint64(i)
		}
		l.Niche = Niche{Size: tagSize, Start: uint64(n), Step: 1, Count: limit - n}
		return l
	}
	if l := inlineVariants(c, vs, dataful, tagSize); l.Size <= MaxInline {
		l.payload = true
		return l
	}
	return box(c, false)
}

// inlineVariants lays out a variant type inline: a struct for one variant,
// a niche when one variant has fields and its fields have spare values for
// the others, and a tag before a union of the variants' fields otherwise.
func inlineVariants(c *ir.TCon, vs []VariantLayout, dataful []int, tagSize int) *Layout {
	n := len(vs)
	l := &Layout{Ty: c, Repr: ReprStruct, Variants: vs}
	if n == 1 {
		place(l, vs[0].Fields, 0)
		l.Niche = firstNiche(vs[0].Fields)
		return l
	}
	if len(dataful) == 1 {
		d := &vs[dataful[0]]
		var payload Layout
		place(&payload, d.Fields, 0)
		if niche := firstNiche(d.Fields); niche.Count >= n-1 {
			l.Size, l.Align, l.Counted = payload.Size, payload.Align, payload.Counted
			l.Tag = Tag{Kind: TagNiche, Offset: niche.Offset, Size: niche.Size}
			k := 0
			for i := range vs {
				if i != dataful[0] {
					vs[i].Value = niche.value(k)
					k++
				}
			}
			l.Niche = niche.rest(n - 1)
			return l
		}
	}
	// A tag, then the variants' fields over each other.
	align, size := tagSize, 0
	for i := range vs {
		var p Layout
		place(&p, vs[i].Fields, 0)
		align = max(align, p.Align)
	}
	base := roundUp(tagSize, align)
	for i := range vs {
		var p Layout
		place(&p, vs[i].Fields, base)
		size = max(size, p.Size)
		if p.Counted {
			l.Counted = true
		}
		vs[i].Value = uint64(i)
	}
	l.Size, l.Align = roundUp(max(size, base), align), align
	l.Tag = Tag{Kind: TagField, Size: tagSize}
	limit := 256
	if tagSize == 2 {
		limit = 65536
	}
	l.Niche = Niche{Size: tagSize, Start: uint64(n), Step: 1, Count: limit - n}
	return l
}

// boxPayload works out what a box's object holds after its header.
func (ls *Layouts) boxPayload(l *Layout) {
	c, _ := l.Ty.(*ir.TCon)
	if c != nil && c.Decl != nil && len(c.Decl.Variants) > 0 {
		l.Tag = Tag{Kind: TagBox}
		for i, v := range c.Decl.Variants {
			vl := VariantLayout{Variant: v}
			for j, f := range v.Fields {
				name := f.Name
				if name == "" {
					name = fmt.Sprint(j)
				}
				vl.Fields = append(vl.Fields, Slot{Name: name, L: ls.shallow(VariantFieldTy(c, v, j))})
			}
			var p Layout
			place(&p, vl.Fields, HeaderSize)
			if len(v.Fields) == 0 {
				vl.Value = uint64(i)<<1 | 1
			} else {
				vl.Value = uint64(i)
			}
			l.Variants = append(l.Variants, vl)
		}
		return
	}
	var slots []Slot
	switch t := l.Ty.(type) {
	case *ir.TTuple:
		for i, e := range t.Elems {
			slots = append(slots, Slot{Name: fmt.Sprint(i), L: ls.shallow(e)})
		}
	default:
		names, types, _ := RecordFields(l.Ty)
		for i, n := range names {
			slots = append(slots, Slot{Name: n, L: ls.shallow(types[i])})
		}
	}
	var p Layout
	place(&p, slots, HeaderSize)
	l.Fields = slots
}

// isRecursive reports whether a declared type holds itself inline: through
// its fields, a tuple, or another declared type's arguments, but not
// through a List, Set, Map, Iter, function or dyn, which are pointers
// already.
func (ls *Layouts) isRecursive(d *ir.Type) bool {
	if r, ok := ls.recursive[d]; ok {
		return r
	}
	ls.recursive[d] = false // while looking: a cycle elsewhere isn't this one's
	seen := map[*ir.Type]bool{}
	var reaches func(t *ir.Type) bool
	var inExpr func(te *ir.TypeExpr) bool
	inExpr = func(te *ir.TypeExpr) bool {
		if te == nil {
			return false
		}
		if te.Decl != nil {
			if te.Decl == d {
				return true
			}
			if reaches(te.Decl) {
				return true
			}
			for _, a := range te.Args {
				if inExpr(a) {
					return true
				}
			}
			return false
		}
		switch te.Name {
		case "tuple", "record":
			for _, a := range te.Args {
				if inExpr(a) {
					return true
				}
			}
		case "NonZero", "Range":
			for _, a := range te.Args {
				if inExpr(a) {
					return true
				}
			}
		}
		return false
	}
	reaches = func(t *ir.Type) bool {
		if seen[t] {
			return false
		}
		seen[t] = true
		if t.Alias != nil {
			if t.Alias.Name == "record" {
				for _, a := range t.Alias.Args {
					if inExpr(a) {
						return true
					}
				}
				return false
			}
			return inExpr(t.Alias)
		}
		for _, v := range t.Variants {
			for _, f := range v.Fields {
				if inExpr(f.Type) {
					return true
				}
			}
		}
		return false
	}
	seen[d] = true
	r := false
	if d.Alias != nil {
		if d.Alias.Name == "record" {
			for _, a := range d.Alias.Args {
				r = r || inExpr(a)
			}
		} else {
			r = inExpr(d.Alias)
		}
	}
	for _, v := range d.Variants {
		for _, f := range v.Fields {
			r = r || inExpr(f.Type)
		}
	}
	ls.recursive[d] = r
	return r
}

// String describes a layout in one line, for tests and `eye check --lir`:
// its representation, size and alignment, whether it is counted, and its
// parts.
func (l *Layout) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s", l.Repr)
	if l.Scalar != "" {
		fmt.Fprintf(&b, " %s", l.Scalar)
	}
	fmt.Fprintf(&b, " %d/%d", l.Size, l.Align)
	if l.Counted {
		b.WriteString(" counted")
	}
	if l.Recursive {
		b.WriteString(" recursive")
	}
	slots := func(ss []Slot) string {
		parts := make([]string, len(ss))
		for i, s := range ss {
			parts[i] = fmt.Sprintf("%s@%d", s.Name, s.Offset)
		}
		return strings.Join(parts, " ")
	}
	if len(l.Fields) > 0 {
		fmt.Fprintf(&b, " {%s}", slots(l.Fields))
	}
	if len(l.Variants) > 0 && l.Repr != ReprEnum {
		fmt.Fprintf(&b, " tag %s", l.Tag.Kind)
		if l.Tag.Kind == TagField || l.Tag.Kind == TagNiche {
			fmt.Fprintf(&b, "@%d/%d", l.Tag.Offset, l.Tag.Size)
		}
		parts := make([]string, len(l.Variants))
		for i, v := range l.Variants {
			s := v.Variant.Name
			if len(v.Fields) > 0 {
				s += "{" + slots(v.Fields) + "}"
			}
			if l.Tag.Kind != TagNiche || len(v.Fields) == 0 {
				s += fmt.Sprintf("=%d", v.Value)
			}
			parts[i] = s
		}
		fmt.Fprintf(&b, " [%s]", strings.Join(parts, " | "))
	}
	if l.Niche.Count > 0 {
		fmt.Fprintf(&b, " niche@%d", l.Niche.Offset)
	}
	return b.String()
}
