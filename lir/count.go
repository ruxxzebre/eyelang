package lir

import (
	"fmt"
	"math/bits"

	"eyelang/ir"
)

// Reference counting in the low IR (ROADMAP 7.5;
// docs/internals/compilation.md, "Reference counting in the low IR").
//
// Count turns the low IR's values into owned and borrowed ones and makes
// every change of a count an instruction, following Perceus (Reinking et
// al., PLDI 2021) with Lean 4's borrowed parameters:
//
//   - A value of a counted type (Layouts.Counted) that an instruction
//     makes owns one reference. It is dropped right after its last use
//     (Drop), or passed on by that use, which takes it; a use that takes it
//     while it is still needed takes a copy (Dup).
//   - A field read (Field, TupleGet, Payload), a module-level let and a
//     parameter the function only reads are borrowed: valid while what
//     they borrow from is, which stays alive until their last use, and
//     never dropped. A borrowed value that is kept is dup'ed.
//   - A parameter is owned when the function keeps it, returns it or
//     updates it in place, and borrowed otherwise (Func.Modes), decided
//     over the whole program at once.
//   - An update whose operand's last use it is (a built-in's add, insert,
//     remove or concat, `+` on a String, List or Set, `{ r with … }`) takes
//     the operand and is marked InPlace: it changes the value in place when
//     it holds the only reference. Where the operand comes from doesn't
//     matter: a call, a `let`, a record field the next instruction refills
//     (Field.Take).
//
// Faults leak what is live, until runs get regions (7.12).
func Count(p *Program) {
	if p.Layouts == nil {
		p.Layouts = NewLayouts()
	}
	for _, f := range p.Funcs {
		f.Modes = initialModes(f, p.Layouts)
	}
	// Borrowed until a body shows the parameter is kept: each round can
	// only make more parameters owned, so this ends.
	for changed := true; changed; {
		changed = false
		for _, f := range p.Funcs {
			c := newCounter(f, p.Layouts)
			c.analyse()
			for _, i := range c.wantOwned() {
				if f.Modes[i] == Borrowed {
					f.Modes[i] = Owned
					changed = true
				}
			}
		}
	}
	for _, f := range p.Funcs {
		c := newCounter(f, p.Layouts)
		c.analyse()
		c.rewrite()
	}
}

// MutParams says which of f's parameters are `mut`: the address of the
// caller's variable, whose value the function may change.
func MutParams(f *Func) []bool {
	mut := make([]bool, len(f.Params))
	src := f.Src
	if src == nil || f.Kind == KindLambda {
		return mut
	}
	i := 0
	if src.Entity != nil || src.HasSelf {
		if i < len(mut) {
			mut[i] = src.MutSelf
		}
		i++
	}
	for _, p := range src.Params {
		if i < len(mut) {
			mut[i] = p.Mut
		}
		i++
	}
	return mut
}

// initialModes are f's parameters' modes before the bodies are read: `mut`
// ones are Mut, one the function sets (to its default) is Owned, and the
// rest Borrowed. A lambda's are all Borrowed: a closure is called through
// its value, which keeps its captures and lends its arguments.
func initialModes(f *Func, ls *Layouts) []Mode {
	modes := make([]Mode, len(f.Params))
	mut := MutParams(f)
	set := map[*Value]bool{}
	for _, b := range f.Blocks {
		for _, in := range b.Instrs {
			if d := defOf(in); d != nil {
				set[d] = true
			}
		}
	}
	for i, p := range f.Params {
		switch {
		case mut[i]:
			modes[i] = Mut
		case f.Kind == KindLambda:
		case set[p] && ls.Counted(p.Ty):
			modes[i] = Owned
		}
	}
	return modes
}

// defOf is the variable an instruction sets: its Dst, or a SetPlace's
// variable when it sets the whole of it. A SetPlace with a path changes the
// variable it starts from in place, which isn't a new value.
func defOf(in Instr) *Value {
	if s, ok := in.(*SetPlace); ok && len(s.Path) > 0 {
		return nil
	}
	return dstOf(in)
}

// ---------------------------------------------------------------- one function

// class is what a value holds.
type class uint8

const (
	clsNone     class = iota // not of a counted type
	clsOwned                 // its own reference
	clsBorrowed              // a reference that something else holds
	clsStatic                // a constant, which is never freed
	clsMut                   // a `mut` parameter: the caller's variable
)

type vinfo struct {
	cls class
	// root is what a borrowed value borrows from: the variable that holds
	// the reference, which must stay alive and unchanged while the value
	// is used; nil when the borrow holds for the whole function (a
	// parameter, a module-level let, or a part of one).
	root *Value
	path []string // the fields from root to the value
	// param is the borrowed parameter a borrowed value is (through `let`s),
	// which the function would have to own to keep it.
	param *Value
	defs  int
}

type counter struct {
	f   *Func
	ls  *Layouts
	inf []vinfo // by value ID
	// written are the variables a SetPlace or a `mut` argument changes in
	// place: never borrowed.
	written map[*Value]bool
	// forced are borrowed values made owned: a field taken, or one whose
	// root changes while it is still used.
	forced map[*Value]bool
	takes  map[*Field]bool
	mut    []bool
	exit   bitset // live where the function returns: its `mut` parameters
	preds  map[*Block][]*Block
	in     map[*Block]bitset
	out    map[*Block]bitset
	after  map[*Block][]bitset // live after each instruction
	// wanted are borrowed parameters a body would keep.
	wanted map[*Value]bool
	byID   map[int]*Value
	// aliases are variables set once by a Move from an owned variable that
	// is still live: they borrow from it.
	aliases map[*Value]bool
}

func newCounter(f *Func, ls *Layouts) *counter {
	return &counter{f: f, ls: ls, written: map[*Value]bool{}, forced: map[*Value]bool{}, takes: map[*Field]bool{}, mut: MutParams(f)}
}

func (c *counter) info(v *Value) *vinfo { return &c.inf[v.ID] }

// counts reports whether v takes part in counting: an owned, borrowed or
// mut value of a counted type.
func (c *counter) counts(v *Value) bool {
	switch c.info(v).cls {
	case clsOwned, clsBorrowed, clsMut:
		return true
	}
	return false
}

// owns reports whether v holds a reference it must drop or pass on.
func (c *counter) owns(v *Value) bool {
	k := c.info(v).cls
	return k == clsOwned || k == clsMut
}

// analyse classifies the values, works out where each is live, and finds
// the fields that can be taken and the borrowed values that must be owned,
// until nothing changes.
func (c *counter) analyse() {
	for _, b := range c.f.Blocks {
		for _, in := range b.Instrs {
			switch x := in.(type) {
			case *SetPlace:
				c.written[x.Var] = true
			case *Call:
				for _, i := range x.Mut {
					if i < len(x.Args) {
						c.written[x.Args[i]] = true
					}
				}
			}
		}
	}
	c.predecessors()
	for range 100 {
		c.classify()
		c.liveness()
		n, a := len(c.forced), len(c.aliases)
		c.findTakes()
		c.findConflicts()
		c.findAliases()
		if len(c.forced) == n && len(c.aliases) == a {
			return
		}
	}
}

// findAliases finds the Moves of an owned variable that is still live
// after them into a variable set once, as `for` binds a loop's counter or
// `let x = y` names a var: the new variable can borrow from the old
// instead of taking a reference of its own, unless the old changes while
// the new is used.
func (c *counter) findAliases() {
	for _, b := range c.f.Blocks {
		for i, in := range b.Instrs {
			m, ok := in.(*Move)
			if !ok || c.aliases[m.Dst] || c.forced[m.Dst] || c.written[m.Dst] {
				continue
			}
			if c.info(m.Dst).defs == 1 && c.info(m.Dst).cls == clsOwned && c.owns(m.Src) && c.after[b][i].has(m.Src.ID) && !c.isParam(m.Dst) {
				if c.aliases == nil {
					c.aliases = map[*Value]bool{}
				}
				c.aliases[m.Dst] = true
			}
		}
	}
}

func (c *counter) predecessors() {
	c.preds = map[*Block][]*Block{}
	for _, b := range c.f.Blocks {
		for _, s := range succs(b) {
			c.preds[s] = append(c.preds[s], b)
		}
	}
}

func succs(b *Block) []*Block {
	switch t := b.Term.(type) {
	case *Jump:
		return []*Block{t.To}
	case *Branch:
		if t.Then == t.Else {
			return []*Block{t.Then}
		}
		return []*Block{t.Then, t.Else}
	}
	return nil
}

// classify decides what each value holds.
func (c *counter) classify() {
	c.inf = make([]vinfo, c.f.values+1)
	for _, b := range c.f.Blocks {
		for _, in := range b.Instrs {
			if d := defOf(in); d != nil {
				c.info(d).defs++
			}
		}
	}
	for i, p := range c.f.Params {
		inf := c.info(p)
		inf.defs++
		if !c.ls.Counted(p.Ty) {
			continue
		}
		switch c.f.Modes[i] {
		case Mut:
			inf.cls = clsMut
		case Owned:
			inf.cls = clsOwned
		default:
			inf.cls, inf.param = clsBorrowed, p
		}
	}
	// A value's class depends on its source's, which is made before it on
	// every path; two rounds settle the rare loop that lists it later.
	for range 2 {
		for _, b := range c.f.Blocks {
			for _, in := range b.Instrs {
				c.classifyDef(in)
			}
		}
	}
}

func (c *counter) classifyDef(in Instr) {
	d := defOf(in)
	if d == nil || c.isParam(d) {
		return
	}
	inf := c.info(d)
	if !c.ls.Counted(d.Ty) {
		inf.cls = clsNone
		return
	}
	if inf.defs > 1 || c.written[d] || c.forced[d] {
		inf.cls, inf.root, inf.path, inf.param = clsOwned, nil, nil, nil
		return
	}
	project := func(src *Value, name string) {
		s := c.info(src)
		switch s.cls {
		case clsBorrowed:
			inf.cls, inf.root, inf.param = clsBorrowed, s.root, nil
			inf.path = append(append([]string(nil), s.path...), name)
		case clsOwned, clsMut:
			inf.cls, inf.root, inf.path, inf.param = clsBorrowed, src, []string{name}, nil
		case clsStatic:
			inf.cls, inf.root, inf.path, inf.param = clsBorrowed, nil, nil, nil
		default:
			inf.cls = clsOwned
		}
	}
	switch x := in.(type) {
	case *Const:
		// A constant is static, except a time, which is worked out
		// when it runs and is counted where it is an object.
		if _, at := x.Value.(ir.Instant); at {
			inf.cls = clsOwned
		} else {
			inf.cls = clsStatic
		}
	case *Global:
		inf.cls = clsBorrowed
	case *Field:
		project(x.Src, x.Name)
	case *TupleGet:
		project(x.Src, fmt.Sprintf("#%d", x.Index))
	case *Payload:
		project(x.Src, fmt.Sprintf("%s.%d", x.Variant.Name, x.Index))
	case *Move:
		s := c.info(x.Src)
		switch s.cls {
		case clsBorrowed:
			*inf = vinfo{cls: clsBorrowed, root: s.root, path: s.path, param: s.param, defs: inf.defs}
		case clsStatic:
			inf.cls = clsStatic
		case clsOwned, clsMut:
			if c.aliases[d] {
				inf.cls, inf.root, inf.path, inf.param = clsBorrowed, x.Src, nil, nil
			} else {
				inf.cls = clsOwned
			}
		default:
			inf.cls = clsOwned
		}
	default:
		inf.cls = clsOwned
	}
}

func (c *counter) isParam(v *Value) bool {
	for _, p := range c.f.Params {
		if p == v {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- liveness

type bitset []uint64

func newBitset(n int) bitset    { return make(bitset, (n+64)/64) }
func (s bitset) has(i int) bool { return i/64 < len(s) && s[i/64]&(1<<(i%64)) != 0 }
func (s bitset) add(i int)      { s[i/64] |= 1 << (i % 64) }
func (s bitset) del(i int)      { s[i/64] &^= 1 << (i % 64) }
func (s bitset) copy() bitset   { return append(bitset(nil), s...) }

func (s bitset) union(t bitset) bool {
	changed := false
	for i := range s {
		if n := s[i] | t[i]; n != s[i] {
			s[i], changed = n, true
		}
	}
	return changed
}

func (s bitset) each(f func(int)) {
	for i, w := range s {
		for w != 0 {
			b := bits.TrailingZeros64(w)
			f(i*64 + b)
			w &^= 1 << b
		}
	}
}

// uses lists what an instruction keeps alive: what it reads, the variable
// a SetPlace with a path changes, and the root of each borrowed value among
// them.
func (c *counter) uses(in Instr) []*Value {
	vs := append([]*Value(nil), readsOf(in)...)
	if s, ok := in.(*SetPlace); ok && len(s.Path) > 0 {
		vs = append(vs, s.Var)
	}
	return c.withRoots(vs)
}

func (c *counter) withRoots(vs []*Value) []*Value {
	n := len(vs)
	for _, v := range vs[:n] {
		if v != nil && c.info(v).cls == clsBorrowed && c.info(v).root != nil {
			vs = append(vs, c.info(v).root)
		}
	}
	return vs
}

func (c *counter) termUses(t Term) []*Value {
	switch x := t.(type) {
	case *Branch:
		return c.withRoots([]*Value{x.Cond})
	case *Return:
		if x.Value != nil {
			return c.withRoots([]*Value{x.Value})
		}
	}
	return nil
}

func (c *counter) liveness() {
	n := c.f.values + 1
	c.exit = newBitset(n)
	for i, p := range c.f.Params {
		if c.mut[i] && c.counts(p) {
			c.exit.add(p.ID)
		}
	}
	c.in, c.out = map[*Block]bitset{}, map[*Block]bitset{}
	for _, b := range c.f.Blocks {
		c.in[b], c.out[b] = newBitset(n), newBitset(n)
	}
	gen := func(s bitset, vs []*Value) {
		for _, v := range vs {
			if v != nil && c.counts(v) {
				s.add(v.ID)
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for i := len(c.f.Blocks) - 1; i >= 0; i-- {
			b := c.f.Blocks[i]
			out := c.out[b]
			if _, ret := b.Term.(*Return); ret {
				out.union(c.exit)
			}
			for _, s := range succs(b) {
				out.union(c.in[s])
			}
			live := out.copy()
			gen(live, c.termUses(b.Term))
			for j := len(b.Instrs) - 1; j >= 0; j-- {
				in := b.Instrs[j]
				if d := defOf(in); d != nil {
					live.del(d.ID)
				}
				gen(live, c.uses(in))
			}
			if c.in[b].union(live) {
				changed = true
			}
		}
	}
	c.after = map[*Block][]bitset{}
	for _, b := range c.f.Blocks {
		live := c.out[b].copy()
		gen(live, c.termUses(b.Term))
		after := make([]bitset, len(b.Instrs))
		for j := len(b.Instrs) - 1; j >= 0; j-- {
			after[j] = live.copy()
			in := b.Instrs[j]
			if d := defOf(in); d != nil {
				live.del(d.ID)
			}
			gen(live, c.uses(in))
		}
		c.after[b] = after
	}
}

// ---------------------------------------------------------------- takes and conflicts

// findTakes finds the field reads whose record is rebuilt with a new value
// for the field later in the block, before anything else reads it: the
// field can be moved out instead of borrowed, so that an update of it is in
// place. `p = { p with items: p.items.add(x) }` and
// `p.items = p.items.add(x)` are the shapes.
func (c *counter) findTakes() {
	for _, b := range c.f.Blocks {
		for i, in := range b.Instrs {
			f, ok := in.(*Field)
			if !ok || c.takes[f] {
				continue
			}
			d, s := f.Dst, f.Src
			di := c.info(d)
			if di.cls != clsBorrowed || di.root != s || len(di.path) != 1 || !c.owns(s) {
				continue
			}
			// Nothing else borrowed from the field may be live.
			clash := false
			c.after[b][i].each(func(id int) {
				vi := &c.inf[id]
				if id != d.ID && vi.cls == clsBorrowed && vi.root == s && len(vi.path) > 0 && vi.path[0] == f.Name {
					clash = true
				}
			})
			if clash || !c.refilled(b, i, f) {
				continue
			}
			c.takes[f] = true
			c.forced[d] = true
		}
	}
}

// refilled reports whether the record the Field at b.Instrs[i] reads is
// rebuilt with a new value for the field later in the block (an update in
// place, or a SetPlace of the field), with nothing reading the record in
// between but other fields.
func (c *counter) refilled(b *Block, i int, f *Field) bool {
	s := f.Src
	for j := i + 1; j < len(b.Instrs); j++ {
		switch x := b.Instrs[j].(type) {
		case *At:
			continue
		case *Field:
			if x.Src == s {
				if x.Name == f.Name {
					return false
				}
				continue
			}
		case *Update:
			if x.Src == s {
				refills := false
				for _, n := range x.Names {
					refills = refills || n == f.Name
				}
				for _, v := range x.Fields {
					refills = refills && v != s
				}
				return refills && !c.after[b][j].has(s.ID)
			}
		case *SetPlace:
			if x.Var == s {
				return len(x.Path) == 1 && x.Path[0].Index == nil && x.Path[0].Field == f.Name
			}
		}
		in := b.Instrs[j]
		if defOf(in) == s {
			return false
		}
		reads := readsOf(in)
		if x, ok := in.(*SetPlace); ok {
			reads = append(reads, x.Var)
		}
		for _, v := range reads {
			if v == s {
				return false
			}
			// Another value borrowed from the field would read the hole.
			if vi := c.info(v); v != f.Dst && vi.cls == clsBorrowed && vi.root == s && len(vi.path) > 0 && vi.path[0] == f.Name {
				return false
			}
		}
	}
	return false
}

// findConflicts makes owned each borrowed value whose root changes while
// the value is still used: the variable set again, a SetPlace of an
// overlapping path, or a `mut` argument.
func (c *counter) findConflicts() {
	for _, b := range c.f.Blocks {
		for i, in := range b.Instrs {
			var root *Value
			var path []Step
			whole := false
			switch x := in.(type) {
			case *SetPlace:
				root, path, whole = x.Var, x.Path, len(x.Path) == 0
			case *Call:
				for _, k := range x.Mut {
					if k < len(x.Args) {
						c.conflict(b, i, x.Args[k], nil, true)
					}
				}
				if d := x.Dst; d != nil {
					c.conflict(b, i, d, nil, true)
				}
				continue
			default:
				if d := defOf(in); d != nil {
					root, whole = d, true
				}
			}
			if root != nil {
				c.conflict(b, i, root, path, whole)
			}
		}
	}
}

func (c *counter) conflict(b *Block, i int, root *Value, path []Step, whole bool) {
	c.after[b][i].each(func(id int) {
		vi := &c.inf[id]
		if vi.cls != clsBorrowed || vi.root != root || c.forced[c.valueByID(id)] {
			return
		}
		if whole || overlaps(vi.path, path) {
			c.forced[c.valueByID(id)] = true
		}
	})
}

// overlaps reports whether a write of path changes what a value borrowed at
// fields reads: one is a prefix of the other, up to the first element.
func overlaps(fields []string, path []Step) bool {
	for i := 0; i < len(fields) && i < len(path); i++ {
		if path[i].Index != nil {
			return true
		}
		if path[i].Field != fields[i] {
			return false
		}
	}
	return true
}

// valueByID finds the value with an ID: a parameter or an instruction's.
func (c *counter) valueByID(id int) *Value {
	if c.byID == nil {
		c.byID = map[int]*Value{}
		for _, p := range c.f.Params {
			c.byID[p.ID] = p
		}
		for _, b := range c.f.Blocks {
			for _, in := range b.Instrs {
				if d := dstOf(in); d != nil {
					c.byID[d.ID] = d
				}
			}
		}
	}
	return c.byID[id]
}

// ---------------------------------------------------------------- what each instruction does

type operand struct {
	v       *Value
	takes   bool // the instruction takes the reference
	inPlace bool // an update's operand, which it takes when this is its last use
}

// operands lists what an instruction reads, and which of it it takes.
func (c *counter) operands(in Instr) []operand {
	var ops []operand
	read := func(vs ...*Value) {
		for _, v := range vs {
			ops = append(ops, operand{v: v})
		}
	}
	take := func(vs ...*Value) {
		for _, v := range vs {
			ops = append(ops, operand{v: v, takes: true})
		}
	}
	switch x := in.(type) {
	case *Move:
		if k := c.info(x.Dst).cls; k == clsBorrowed || k == clsStatic {
			read(x.Src)
		} else {
			take(x.Src)
		}
	case *Call:
		for i, a := range x.Args {
			if i < len(x.Fn.Modes) && x.Fn.Modes[i] == Owned {
				take(a)
			} else {
				read(a)
			}
		}
	case *Builtin:
		if updates(x.Name, x.Args, x.Durable) {
			ops = append(ops, operand{v: x.Args[0], inPlace: true})
			read(x.Args[1:]...)
		} else {
			read(x.Args...)
		}
	case *Op:
		if x.Op == "+" && len(x.Args) == 2 && appendable(x.Args[0].Ty) {
			ops = append(ops, operand{v: x.Args[0], inPlace: true})
			read(x.Args[1])
		} else {
			read(x.Args...)
		}
	case *Update:
		ops = append(ops, operand{v: x.Src, inPlace: true})
		take(x.Fields...)
	case *MakeRecord:
		take(x.Fields...)
	case *MakeVariant:
		take(x.Args...)
	case *MakeTuple:
		take(x.Elems...)
	case *MakeList:
		take(x.Elems...)
	case *MakeSet:
		take(x.Elems...)
	case *MakeMap:
		take(x.Keys...)
		take(x.Values...)
	case *MakeClosure:
		take(x.Captures...)
	case *MakeDyn:
		take(x.Src)
	case *SetPlace:
		take(x.Src)
		if len(x.Path) > 0 {
			read(x.Var)
		}
		for _, s := range x.Path {
			if s.Index != nil {
				read(s.Index)
			}
		}
	default:
		read(readsOf(in)...)
	}
	return ops
}

// updates reports whether a call of a built-in is an update of its first
// argument, which it can change in place: add to a List or Set, insert into
// a Map, remove from a Map or Set, concat onto a String.
func updates(name string, args []*Value, durable bool) bool {
	if durable || len(args) == 0 {
		return false
	}
	k := builtinKind(args[0].Ty)
	switch name {
	case "add":
		return len(args) == 2 && (k == "List" || k == "Set")
	case "insert":
		return len(args) == 3 && k == "Map"
	case "remove":
		return len(args) == 2 && (k == "Map" || k == "Set")
	case "concat":
		return k == "String"
	}
	return false
}

// appendable reports whether `+` on values of t appends to the first.
func appendable(t ir.Ty) bool {
	switch builtinKind(t) {
	case "String", "List", "Set":
		return true
	}
	return false
}

// builtinKind is the built-in type t's values are: "List", "String", …, or
// "" for a declared type.
func builtinKind(t ir.Ty) string {
	for range 50 {
		t = Resolve(t)
		c, ok := t.(*ir.TCon)
		if !ok {
			return ""
		}
		if c.Decl != nil && c.Decl.Newtype() {
			t = VariantFieldTy(t, c.Decl.Variants[0], 0)
			continue
		}
		if c.Decl != nil || c.Entity != nil {
			return ""
		}
		return c.Name
	}
	return ""
}

// plan is what counting puts around one instruction.
type plan struct {
	pre, post []Instr
	inPlace   bool
	// temp: the instruction sets a variable it also reads, whose old value
	// must be dropped between the two; its result goes through a new value.
	temp bool
}

// planFor works out the dups before an instruction, the drops after it and
// whether it updates in place, given what is live after it.
func (c *counter) planFor(in Instr, ops []operand, d *Value, after bitset) plan {
	var p plan
	live := func(v *Value) bool { return v != d && after.has(v.ID) }
	times := func(v *Value) int {
		n := 0
		for _, o := range ops {
			if o.v == v {
				n++
			}
		}
		return n
	}
	// Whether the instruction also reads something borrowed from v, which
	// an update of v in place would change under it.
	lends := func(v *Value) bool {
		for _, o := range ops[1:] {
			if o.v != nil && !o.takes && c.info(o.v).cls == clsBorrowed && c.info(o.v).root == v {
				return true
			}
		}
		return false
	}
	if len(ops) > 0 && ops[0].inPlace {
		v := ops[0].v
		vi := c.info(v)
		switch {
		case c.owns(v) && !live(v) && times(v) == 1 && !lends(v):
			p.inPlace = true
			ops[0].takes = true
		case vi.cls == clsBorrowed && vi.param != nil && !live(v) && !live(vi.param) && times(v) == 1:
			c.want(vi.param) // owned, it could be updated in place
		}
	}
	// How many times the instruction takes each value and reads it; a
	// borrowed value read keeps what it borrows from alive through the
	// instruction too, which may be where that dies.
	var order []*Value
	kt, kr := map[*Value]int{}, map[*Value]int{}
	note := func(v *Value, takes bool) {
		if _, ok := kt[v]; !ok {
			order = append(order, v)
			kt[v], kr[v] = 0, 0
		}
		if takes {
			kt[v]++
		} else {
			kr[v]++
		}
	}
	for _, o := range ops {
		if o.v == nil || !c.counts(o.v) {
			continue
		}
		note(o.v, o.takes)
		if vi := c.info(o.v); vi.cls == clsBorrowed && vi.root != nil {
			note(vi.root, false)
		}
	}
	for _, v := range order {
		kt, kr := kt[v], kr[v]
		vi := c.info(v)
		switch vi.cls {
		case clsOwned, clsMut:
			moved := !live(v) && kr == 0 && kt > 0
			dups := kt
			if moved {
				dups--
			}
			for range dups {
				p.pre = append(p.pre, &Dup{V: v})
			}
			if !live(v) && !moved {
				if v == d {
					p.temp = true
				} else {
					p.post = append(p.post, &Drop{V: v})
				}
			}
		case clsBorrowed:
			if kt > 0 && vi.param != nil {
				c.want(vi.param)
			}
			for range kt {
				p.pre = append(p.pre, &Dup{V: v})
			}
		}
	}
	if d != nil && c.owns(d) && !after.has(d.ID) {
		p.post = append(p.post, &Drop{V: d})
	}
	return p
}

func (c *counter) want(p *Value) {
	if c.wanted == nil {
		c.wanted = map[*Value]bool{}
	}
	c.wanted[p] = true
}

// wantOwned lists the borrowed parameters that f's body keeps, returns or
// would update in place: they should be owned.
func (c *counter) wantOwned() []int {
	c.wanted = nil
	for _, b := range c.f.Blocks {
		for i, in := range b.Instrs {
			c.planFor(in, c.operands(in), defOf(in), c.after[b][i])
		}
		if r, ok := b.Term.(*Return); ok && r.Value != nil {
			c.planFor(nil, []operand{{v: r.Value, takes: true}}, nil, c.exit)
		}
	}
	var out []int
	for i, p := range c.f.Params {
		if c.wanted[p] && c.f.Kind != KindLambda {
			out = append(out, i)
		}
	}
	return out
}

// ---------------------------------------------------------------- rewriting

// rewrite puts the dups and drops into f, marks its updates in place and
// its fields taken, and drops on the way into each block what is no longer
// live there.
func (c *counter) rewrite() {
	f := c.f
	blocks := append([]*Block(nil), f.Blocks...)
	// What dies on the way into a block, by predecessor.
	type edge struct{ from, to *Block }
	dies := map[edge][]*Value{}
	for _, b := range blocks {
		br, ok := b.Term.(*Branch)
		if !ok || br.Then == br.Else {
			continue
		}
		for _, s := range []*Block{br.Then, br.Else} {
			if _, stop := s.Term.(*Stop); stop {
				continue // a fault leaks what is live (7.12)
			}
			var vs []*Value
			c.out[b].each(func(id int) {
				v := c.valueByID(id)
				if v != nil && c.owns(v) && !c.in[s].has(id) {
					vs = append(vs, v)
				}
			})
			if len(vs) > 0 {
				dies[edge{b, s}] = vs
			}
		}
	}
	for _, b := range blocks {
		var out []Instr
		for i, in := range b.Instrs {
			if fl, ok := in.(*Field); ok && c.takes[fl] {
				fl.Take = true
			}
			d := defOf(in)
			ops := c.operands(in)
			p := c.planFor(in, ops, d, c.after[b][i])
			if p.inPlace {
				switch x := in.(type) {
				case *Builtin:
					x.InPlace = true
				case *Op:
					x.InPlace = true
				case *Update:
					x.InPlace = true
				}
			}
			if m, ok := in.(*Move); ok {
				k := c.info(m.Dst).cls
				m.Borrow = k == clsBorrowed || k == clsStatic
			}
			out = append(out, p.pre...)
			if p.temp {
				t := c.newValue(d.Ty)
				setDst(in, t)
				out = append(out, in, &Drop{V: d}, &Move{Dst: d, Src: t})
			} else {
				out = append(out, in)
			}
			// A value read from elsewhere that must be owned is dup'ed
			// where it is made.
			if d != nil && c.info(d).cls == clsOwned && borrowingDef(in) {
				out = append(out, &Dup{V: d})
			}
			out = append(out, p.post...)
		}
		if r, ok := b.Term.(*Return); ok && r.Value != nil {
			// The value returned, dup'ed if it is borrowed, and then what
			// it borrowed from, which dies here.
			p := c.planFor(nil, []operand{{v: r.Value, takes: true}}, nil, c.exit)
			out = append(out, p.pre...)
			out = append(out, p.post...)
		}
		b.Instrs = out
	}
	// Owned parameters the function never uses are dropped where it starts.
	entry := f.Blocks[0]
	if len(c.preds[entry]) > 0 {
		start := &Block{ID: len(f.Blocks), Term: &Jump{To: entry}}
		f.Blocks = append([]*Block{start}, f.Blocks...)
		entry = start
	}
	var first []Instr
	for _, p := range f.Params {
		if c.owns(p) && !c.in[blocks[0]].has(p.ID) {
			first = append(first, &Drop{V: p})
		}
	}
	entry.Instrs = append(first, entry.Instrs...)
	for _, b := range blocks {
		br, ok := b.Term.(*Branch)
		if !ok {
			continue
		}
		for _, to := range []**Block{&br.Then, &br.Else} {
			vs := dies[edge{b, *to}]
			if len(vs) == 0 {
				continue
			}
			drops := make([]Instr, len(vs))
			for i, v := range vs {
				drops[i] = &Drop{V: v}
			}
			if len(c.preds[*to]) == 1 {
				(*to).Instrs = append(drops, (*to).Instrs...)
				continue
			}
			mid := &Block{ID: len(f.Blocks), Instrs: drops, Term: &Jump{To: *to}}
			f.Blocks = append(f.Blocks, mid)
			*to = mid
		}
	}
	for i, b := range f.Blocks {
		b.ID = i
	}
}

// borrowingDef reports whether an instruction's result is borrowed from
// something else: a field read that doesn't take the field, a tuple's
// element, a variant's field, a module-level let, or a Move that borrows.
func borrowingDef(in Instr) bool {
	switch x := in.(type) {
	case *Field:
		return !x.Take
	case *TupleGet, *Payload, *Global:
		return true
	case *Move:
		return x.Borrow
	}
	return false
}

func (c *counter) newValue(t ir.Ty) *Value {
	c.f.values++
	v := &Value{ID: c.f.values, Ty: t}
	c.inf = append(c.inf, vinfo{cls: clsOwned, defs: 1})
	return v
}

// setDst makes an instruction's result go into v.
func setDst(in Instr, v *Value) {
	switch x := in.(type) {
	case *Const:
		x.Dst = v
	case *Move:
		x.Dst = v
	case *Call:
		x.Dst = v
	case *Builtin:
		x.Dst = v
	case *Runtime:
		x.Dst = v
	case *CallValue:
		x.Dst = v
	case *CallDyn:
		x.Dst = v
	case *Op:
		x.Dst = v
	case *Derived:
		x.Dst = v
	case *MakeRecord:
		x.Dst = v
	case *MakeVariant:
		x.Dst = v
	case *MakeTuple:
		x.Dst = v
	case *MakeList:
		x.Dst = v
	case *MakeMap:
		x.Dst = v
	case *MakeSet:
		x.Dst = v
	case *MakeClosure:
		x.Dst = v
	case *MakeDyn:
		x.Dst = v
	case *Field:
		x.Dst = v
	case *TupleGet:
		x.Dst = v
	case *Payload:
		x.Dst = v
	case *IsVariant:
		x.Dst = v
	case *Update:
		x.Dst = v
	case *Global:
		x.Dst = v
	case *SetPlace:
		x.Var = v
	default:
		panic(fmt.Sprintf("lir: %T sets no value", in))
	}
}
