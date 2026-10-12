package cgen

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"eyelang/ir"
	"eyelang/lir"
)

// fn writes one function instance.
type fn struct {
	g       *gen
	fs      *fileState
	f       *lir.Func
	b       strings.Builder
	pending strings.Builder
	at      string // where the code is, as a C string
	pos     ir.Pos
	mut     map[*lir.Value]bool
	param   map[*lir.Value]int
	consts  map[*lir.Value]*lir.Const
	defs    map[*lir.Value]int // how many instructions set each value
	// loops are the values a for loop walks with iter.more and iter.next:
	// the iter that makes one starts a walk, not an Iter value.
	loops map[*lir.Value]bool
}

func newFn(g *gen, fs *fileState, f *lir.Func) *fn {
	return &fn{g: g, fs: fs, f: f, mut: map[*lir.Value]bool{}, param: map[*lir.Value]int{}, consts: map[*lir.Value]*lir.Const{}, defs: map[*lir.Value]int{}, loops: map[*lir.Value]bool{}}
}

// function writes f, and what calls it as a closure, a dyn method or a
// global.
func (g *gen) function(fs *fileState, f *lir.Func) string {
	e := newFn(g, fs, f)
	e.at = cstring(posString(f.Src.Pos))
	mut := mutParams(f)
	for i, p := range f.Params {
		e.param[p] = i
		if mut[i] {
			e.mut[p] = true
		}
	}
	// The values it makes, which are its C locals.
	seen := map[*lir.Value]bool{}
	var locals []*lir.Value
	for _, b := range f.Blocks {
		for _, in := range b.Instrs {
			if c, ok := in.(*lir.Const); ok {
				e.consts[c.Dst] = c
			}
			if b, ok := in.(*lir.Builtin); ok && b.Name == "iter.more" && len(b.Args) == 1 {
				e.loops[b.Args[0]] = true
			}
			if d := lir.Dst(in); d != nil && !seen[d] {
				if _, isParam := e.param[d]; !isParam {
					seen[d] = true
					locals = append(locals, d)
				}
			}
			if c, ok := in.(*lir.Call); ok {
				for _, i := range c.Mut {
					e.defs[c.Args[i]]++
				}
			}
			if d := lir.Dst(in); d != nil {
				e.defs[d]++
			}
		}
	}
	fmt.Fprintf(&e.b, "/* %s %s */\n", f.Kind, strings.ReplaceAll(f.Name, "*/", "* /"))
	e.pos = f.Src.Pos
	e.line(e.pos)
	fmt.Fprintf(&e.b, "%s {\n", g.signature(f))
	// Every local starts empty: zero, which drops to nothing. The low IR
	// says where each value is dup'ed and dropped (lir.Count).
	for _, v := range locals {
		fmt.Fprintf(&e.b, "\t%s v%d = {0};\n", g.ctype(v.Ty), v.ID)
	}
	if hasDefaults(f) {
		e.b.WriteString("\t(void)absent;\n")
	}
	for _, b := range f.Blocks {
		fmt.Fprintf(&e.b, "b%d:;\n", b.ID)
		for _, in := range b.Instrs {
			e.instr(in)
			e.flush()
		}
		e.term(b.Term)
		e.flush()
	}
	e.b.WriteString("}\n\n")
	e.extras()
	return e.b.String()
}

// extras writes the trampoline that calls f as a closure, the wrapper that
// calls it from a dyn table, and the accessor of a global it computes.
func (e *fn) extras() {
	g, f := e.g, e.f
	name := g.names[f]
	// A closure and a dyn table lend their arguments, by their addresses;
	// one a parameter owns is dup'ed for it.
	call := func(first []string, from string, skip int) string {
		args := append([]string(nil), first...)
		for i := skip; i < len(f.Params); i++ {
			args = append(args, fmt.Sprintf("(*(%s *)%s[%d])", g.ctype(f.Params[i].Ty), from, i-skip))
		}
		var dups strings.Builder
		for i, a := range args {
			if i < len(f.Modes) && f.Modes[i] == lir.Owned {
				if st := g.dupStmt(f.Params[i].Ty, a); st != "" {
					fmt.Fprintf(&dups, "\t%s\n", st)
				}
			}
		}
		if hasDefaults(f) {
			args = append(args, "0")
		}
		return fmt.Sprintf("%s\t*(%s *)out = %s(%s);\n", dups.String(), g.ctype(g.resultTy(f)), name, strings.Join(args, ", "))
	}
	if g.tramps[f] {
		n := captures(f)
		var caps []string
		self := ""
		if cl := g.closureStruct(f); cl != "" {
			self = fmt.Sprintf("\t%s *cl = (%s *)self;\n", cl, cl)
			for i := 0; i < n; i++ {
				caps = append(caps, fmt.Sprintf("cl->c%d", i))
			}
		}
		fmt.Fprintf(&e.b, "void %s_tr(ey_fn *self, void *out, void *const *a) {\n\t(void)self;\n\t(void)a;\n%s%s}\n\n", name, self, call(caps, "a", n))
	}
	if g.methods[f] {
		self := fmt.Sprintf("(*(%s *)self)", g.ctype(f.Params[0].Ty))
		fmt.Fprintf(&e.b, "void %s_dm(void *self, void *out, void *const *a) {\n\t(void)a;\n%s}\n\n", name, call([]string{self}, "a", 1))
	}
	if f.Kind == lir.KindGlobal {
		rt := g.resultTy(f)
		keep := ""
		if g.counted(rt) {
			keep = fmt.Sprintf("\n\t\tey_keep_global(&value, %s);", g.desc(rt))
		}
		ct := g.ctype(rt)
		fmt.Fprintf(&e.b, "%s %s_g(void) {\n\tstatic bool done;\n\tstatic %s value;\n\tif (!done) {\n\t\tvalue = %s();\n\t\tdone = true;%s\n\t}\n\treturn value;\n}\n\n", ct, name, ct, name, keep)
	}
}

func (e *fn) line(p ir.Pos) {
	if p.File == nil {
		return
	}
	at := p.File.Tree.Lines.Position(p.Offset)
	fmt.Fprintf(&e.b, "#line %d %s\n", at.Line, cstring(p.File.Path))
}

// printf adds to the C of the instruction being written, which flush
// writes as one line under a #line directive for its position.
func (e *fn) printf(format string, args ...any) {
	if e.pending.Len() > 0 {
		e.pending.WriteByte(' ')
	}
	fmt.Fprintf(&e.pending, format, args...)
}

func (e *fn) flush() {
	if e.pending.Len() == 0 {
		return
	}
	e.line(e.pos)
	e.b.WriteByte('\t')
	e.b.WriteString(e.pending.String())
	e.b.WriteByte('\n')
	e.pending.Reset()
}

func (e *fn) errorf(format string, args ...any) {
	e.g.errorf(e.pos, format, args...)
}

// v is the C expression of a value: its variable, or what a `mut`
// parameter points at.
func (e *fn) v(x *lir.Value) string {
	if x == nil {
		return "0"
	}
	if e.mut[x] {
		return fmt.Sprintf("(*v%d)", x.ID)
	}
	return fmt.Sprintf("v%d", x.ID)
}

// ct is the C type of a value.
func (e *fn) ct(x *lir.Value) string { return e.g.ctype(x.Ty) }

// word is a scalar's, an enum's or a word's value as a word, which runtime
// functions over words take: signed scalars sign-extended, a Float's bits.
func (e *fn) word(x *lir.Value) string {
	switch {
	case e.g.isWord(x.Ty):
		return e.v(x)
	case scalarName(x.Ty) == "Float":
		return fmt.Sprintf("ey_f2v(%s)", e.v(x))
	}
	switch scalarName(x.Ty) {
	case "Int32", "Int64", "Duration", "Size", "Date", "Time":
		return fmt.Sprintf("(ey_v)(int64_t)%s", e.v(x))
	}
	return fmt.Sprintf("(ey_v)%s", e.v(x))
}

// fromWord is a word a runtime function gives as a value of type t.
func (e *fn) fromWord(t ir.Ty, w string) string {
	switch {
	case e.g.isWord(t):
		return w
	case scalarName(t) == "Float":
		return fmt.Sprintf("ey_v2f(%s)", w)
	}
	return fmt.Sprintf("(%s)%s", e.g.ctype(t), w)
}

// smallConst reports whether x is an Int constant in the small range,
// which nothing but its Const sets, and its value.
func (e *fn) smallConst(x *lir.Value) (int64, bool) {
	c, ok := e.consts[x]
	if !ok || e.defs[x] != 1 || kind(x) != "Int" {
		return 0, false
	}
	var n int64
	switch v := c.Value.(type) {
	case int64:
		n = v
	case *big.Int:
		if !v.IsInt64() {
			return 0, false
		}
		n = v.Int64()
	default:
		return 0, false
	}
	return n, n >= -(1<<62) && n < 1<<62
}

// absent reports whether x is the Absent a call passes for a parameter it
// leaves out.
func (e *fn) absent(x *lir.Value) bool {
	c, ok := e.consts[x]
	if !ok {
		return false
	}
	_, is := c.Value.(lir.Absent)
	return is
}

// assign sets dst to expr. The low IR has dropped or passed on what dst
// held before (lir.Count), so dst is empty here; what expr gives, an owned
// result or a value borrowed from elsewhere, the low IR says.
func (e *fn) assign(dst *lir.Value, expr string) {
	if dst == nil {
		e.printf("(void)(%s);", expr)
		return
	}
	e.printf("%s = %s;", e.v(dst), expr)
}

// dupped sets dst to a part of an argument, which a built-in gives as its
// own result (min, max): a reference of its own.
func (e *fn) dupped(dst *lir.Value, expr string) {
	e.assign(dst, expr)
	if st := e.g.dupStmt(dst.Ty, e.v(dst)); st != "" {
		e.printf("%s", st)
	}
}

// zero sets dst to all-zero bits: nothing, or a left-out argument.
func (e *fn) zero(dst *lir.Value) {
	e.printf("memset(&%s, 0, sizeof %s);", e.v(dst), e.v(dst))
}

// array is a C compound literal of words, or NULL for none.
func (e *fn) array(vs []*lir.Value) string {
	if len(vs) == 0 {
		return "NULL"
	}
	parts := make([]string, len(vs))
	for i, x := range vs {
		parts[i] = e.v(x)
	}
	return "(ey_v[]){" + strings.Join(parts, ", ") + "}"
}

// ptrs is a C compound literal of the values' addresses, or NULL for none.
func (e *fn) ptrs(vs []*lir.Value) string {
	if len(vs) == 0 {
		return "NULL"
	}
	parts := make([]string, len(vs))
	for i, x := range vs {
		parts[i] = "&" + e.v(x)
	}
	return "(void *[]){" + strings.Join(parts, ", ") + "}"
}

func (e *fn) instr(in lir.Instr) {
	switch x := in.(type) {
	case *lir.At:
		e.pos = x.Pos
		e.at = cstring(posString(x.Pos))
	case *lir.Const:
		e.constant(x)
	case *lir.Move:
		e.assign(x.Dst, e.v(x.Src))
	case *lir.Dup:
		if st := e.g.dupStmt(x.V.Ty, e.v(x.V)); st != "" {
			e.printf("%s", st)
		}
	case *lir.Drop:
		if st := e.g.dropStmt(x.V.Ty, e.v(x.V)); st != "" {
			e.printf("%s", st)
		}
	case *lir.Call:
		e.call(x)
	case *lir.Builtin:
		e.builtin(x)
	case *lir.Runtime:
		e.runtime(x)
	case *lir.CallValue:
		if x.Durable {
			e.errorf("the C back end doesn't compile durable steps (`!`) yet: jobs run in the Go runner")
		}
		e.printf("ey_call(%s, &%s, %s);", e.v(x.Callee), e.v(x.Dst), e.ptrs(x.Args))
	case *lir.CallDyn:
		k := -1
		for i, m := range x.Trait.Order {
			if m == x.Method {
				k = i
			}
		}
		if k < 0 {
			e.errorf("%s has no method %s", x.Trait.Name, x.Method)
			return
		}
		e.printf("EY_DYNP(%s)->methods[%d](EY_DYN_VAL(%s), &%s, %s);", e.v(x.Recv), k, e.v(x.Recv), e.v(x.Dst), e.ptrs(x.Args))
	case *lir.Op:
		e.op(x)
	case *lir.Derived:
		e.derived(x)
	case *lir.MakeRecord:
		e.makeRecord(x)
	case *lir.MakeVariant:
		e.makeVariant(x)
	case *lir.MakeTuple:
		e.makeFields(x.Dst, x.Elems, nil)
	case *lir.MakeList:
		// The new collection takes its elements.
		e.assign(x.Dst, fmt.Sprintf("ey_list_from(%s, %d, %s)", e.g.desc(x.Dst.Ty), len(x.Elems), e.ptrs(x.Elems)))
	case *lir.MakeSet:
		e.assign(x.Dst, fmt.Sprintf("ey_set_from(%s, %d, %s)", e.g.desc(x.Dst.Ty), len(x.Elems), e.ptrs(x.Elems)))
	case *lir.MakeMap:
		e.assign(x.Dst, fmt.Sprintf("ey_map_from(%s, %d, %s, %s)", e.g.desc(x.Dst.Ty), len(x.Keys), e.ptrs(x.Keys), e.ptrs(x.Values)))
	case *lir.MakeClosure:
		e.closure(x.Dst, x.Fn, x.Captures)
	case *lir.MakeDyn:
		name, ok := e.g.dynTab[x.Table]
		if !ok {
			name = fmt.Sprintf("ey_dt%d", len(e.g.dynTab))
			e.g.dynTab[x.Table] = name
		}
		e.printf("{ ey_dyn *d_ = ey_alloc(sizeof(ey_dyn) + sizeof(%s), &%s_d); d_->methods = %s; *(%s *)EY_DYN_VAL(d_) = %s; %s = EY_V(d_); }",
			e.ct(x.Src), name, name, e.ct(x.Src), e.v(x.Src), e.v(x.Dst))
	case *lir.Field:
		if kind(x.Src) == "Range" {
			e.rangeEnd(x)
			return
		}
		e.field(x.Dst, x.Src, e.fieldIndex(x.Src.Ty, x.Name), x.Take)
	case *lir.TupleGet:
		e.field(x.Dst, x.Src, x.Index, false)
	case *lir.Payload:
		if newtype(x.Variant) {
			e.assign(x.Dst, e.v(x.Src))
			return
		}
		e.assign(x.Dst, e.payload(x.Src, variantIndex(x.Variant), x.Index))
	case *lir.IsVariant:
		if newtype(x.Variant) {
			e.assign(x.Dst, "1")
			return
		}
		e.assign(x.Dst, e.isVariant(x.Src, variantIndex(x.Variant)))
	case *lir.Update:
		e.update(x)
	case *lir.SetPlace:
		e.setPlace(x)
	case *lir.Global:
		e.assign(x.Dst, e.g.names[x.Init]+"_g()")
	default:
		e.errorf("the C back end can't compile %T yet", in)
	}
}

func (e *fn) term(t lir.Term) {
	switch x := t.(type) {
	case *lir.Jump:
		e.printf("goto b%d;", x.To.ID)
	case *lir.Branch:
		e.printf("if (%s) goto b%d;", e.v(x.Cond), x.Then.ID)
		e.printf("goto b%d;", x.Else.ID)
	case *lir.Return:
		// The returned value is owned: the low IR dup'ed it if it had to.
		if x.Value == nil {
			e.printf("return 0;")
			return
		}
		e.printf("return %s;", e.v(x.Value))
	case *lir.Stop:
		e.printf("ey_fault(%s, %s);", e.at, cstring(x.Why))
	default:
		e.printf("ey_fault(%s, \"the end of code that gives no value\");", e.at)
	}
}

// ---------------------------------------------------------------- values

func (e *fn) constant(c *lir.Const) {
	t := c.Dst.Ty
	name := scalarName(t)
	switch v := c.Value.(type) {
	case lir.Absent, ir.Unit:
		e.zero(c.Dst)
	case bool:
		if v {
			e.assign(c.Dst, "1")
		} else {
			e.assign(c.Dst, "0")
		}
	case int32:
		e.assign(c.Dst, fmt.Sprintf("%d", v))
	case int64:
		e.number(c.Dst, name, big.NewInt(v))
	case *big.Int:
		e.number(c.Dst, name, v)
	case float64:
		e.assign(c.Dst, floatLit(v))
	case string:
		// Any string the program writes may name a zone: the program
		// carries the table of each that does (zoneTables).
		e.g.zones[v] = true
		e.assign(c.Dst, e.fs.str(v, "&ey_t_string"))
	case ir.Duration:
		e.assign(c.Dst, fmt.Sprintf("INT64_C(%d)", int64(v)))
	case ir.Size:
		e.assign(c.Dst, fmt.Sprintf("INT64_C(%d)", int64(v)))
	case ir.Date:
		e.assign(c.Dst, fmt.Sprintf("INT64_C(%d)", daysFromCivil(v.Year, v.Month, v.Day)))
	case ir.TimeOfDay:
		e.assign(c.Dst, fmt.Sprintf("%d", v.Hour*3600+v.Minute*60+v.Second))
	case ir.Instant:
		e.assign(c.Dst, e.instant(v, e.fs.str(v.Zone, "&ey_t_string")))
	default:
		e.errorf("the C back end can't compile a constant of type %T yet", c.Value)
	}
}

// instant is an Instant written as a date and time in the zone given.
func (e *fn) instant(v ir.Instant, zone string) string {
	if v.Zone != "" {
		e.g.zones[v.Zone] = true
	}
	secs := v.Time.Hour*3600 + v.Time.Minute*60 + v.Time.Second
	if v.Date == (ir.Date{}) {
		return fmt.Sprintf("ey_instant_next(%s, %d, %s)", e.at, secs, zone)
	}
	return fmt.Sprintf("ey_instant_local(%s, INT64_C(%d), %d, %s)", e.at, daysFromCivil(v.Date.Year, v.Date.Month, v.Date.Day), secs, zone)
}

// number writes an integer constant as the type it has.
func (e *fn) number(dst *lir.Value, name string, n *big.Int) {
	switch name {
	case "Float":
		f, _ := new(big.Float).SetInt(n).Float64()
		e.assign(dst, floatLit(f))
	case "Byte", "Int32", "Int64", "UInt32", "UInt64", "Char", "Duration", "Size":
		// Two's complement bits; a value outside the type, such as the
		// 2147483648 that -2147483648 negates, keeps its 64 bits.
		bits := new(big.Int).And(n, new(big.Int).SetUint64(math.MaxUint64)).Uint64()
		e.assign(dst, fmt.Sprintf("(%s)UINT64_C(%d)", e.ct(dst), bits))
	default:
		e.assign(dst, e.fs.bigInt(n))
	}
}

func floatLit(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return "HUGE_VAL"
	case math.IsInf(f, -1):
		return "(-HUGE_VAL)"
	case math.IsNaN(f):
		return "NAN"
	}
	return "(" + strconv.FormatFloat(f, 'x', -1, 64) + ")"
}

// daysFromCivil is the days from 1970-01-01 to a date, as the runtime's
// ey_days_from_civil counts them.
func daysFromCivil(y, m, d int) int64 {
	if m <= 2 {
		y--
	}
	era := y
	if era < 0 {
		era -= 399
	}
	era /= 400
	yoe := y - era*400
	mm := m - 3
	if m <= 2 {
		mm = m + 9
	}
	doy := (153*mm+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return int64(era*146097 + doe - 719468)
}

func newtype(v *ir.Variant) bool { return v.Type != nil && v.Type.Newtype() }

func variantIndex(v *ir.Variant) int {
	if v.Type == nil {
		return 0
	}
	for i, w := range v.Type.Variants {
		if w == v {
			return i
		}
	}
	return 0
}

func fieldType(t ir.Ty, name string) ir.Ty {
	names, types, _ := lir.RecordFields(lir.Resolve(t))
	for i, n := range names {
		if n == name {
			return types[i]
		}
	}
	return nil
}

// fieldIndex is the place of a record's field in its declared order.
func (e *fn) fieldIndex(t ir.Ty, name string) int {
	names, _, ok := lir.RecordFields(lir.Resolve(t))
	if !ok {
		e.errorf("the C back end can't find the fields of %s", t)
		return 0
	}
	for i, n := range names {
		if n == name {
			return i
		}
	}
	e.errorf("%s has no field %s", t, name)
	return 0
}

// fieldTys are the types of a record's or tuple's fields, in order.
func fieldTys(t ir.Ty) []ir.Ty {
	t = rep(t)
	if tt, ok := t.(*ir.TTuple); ok {
		return tt.Elems
	}
	_, types, _ := lir.RecordFields(t)
	return types
}

// fieldLv is the C lvalue of field i of the record or tuple in src: in the
// struct, or in its box.
func (e *fn) fieldLv(src string, t ir.Ty, i int) string {
	if e.g.layout(t).Repr == lir.ReprBox {
		return fmt.Sprintf("((%s *)(uintptr_t)%s)->f%d", e.g.boxOf(t, -1).name, src, i)
	}
	return fmt.Sprintf("%s.f%d", src, i)
}

// field reads field i of the record or tuple in src: borrowed, or taken out
// of a record about to be rebuilt, which is left empty when it is inline
// or unique.
func (e *fn) field(dst, src *lir.Value, i int, take bool) {
	lv := e.fieldLv(e.v(src), src.Ty, i)
	if !take {
		e.assign(dst, lv)
		return
	}
	if e.g.layout(src.Ty).Repr != lir.ReprBox {
		e.printf("%s = %s; memset(&%s, 0, sizeof %s);", e.v(dst), lv, lv, lv)
		return
	}
	e.printf("%s = %s; if (EY_OBJ(%s)->rc == 1) memset(&%s, 0, sizeof %s); else { %s }",
		e.v(dst), lv, e.v(src), lv, lv, e.g.dupStmt(dst.Ty, e.v(dst)))
}

// makeFields sets dst to a new record or tuple, with the fields given in
// the order of order (the field each is), which it takes.
func (e *fn) makeFields(dst *lir.Value, fields []*lir.Value, order []int) {
	idx := func(i int) int {
		if order != nil {
			return order[i]
		}
		return i
	}
	if e.g.layout(dst.Ty).Repr == lir.ReprBox {
		b := e.g.boxOf(dst.Ty, -1).name
		e.printf("{ %s *b_ = ey_alloc(sizeof(%s), %s);", b, b, e.g.desc(dst.Ty))
		for i, f := range fields {
			e.printf(" b_->f%d = %s;", idx(i), e.v(f))
		}
		e.printf(" %s = EY_V(b_); }", e.v(dst))
		return
	}
	parts := make([]string, len(fields))
	for i, f := range fields {
		parts[i] = fmt.Sprintf(".f%d = %s", idx(i), e.v(f))
	}
	e.assign(dst, fmt.Sprintf("(%s){%s}", e.ct(dst), strings.Join(parts, ", ")))
}

func (e *fn) makeRecord(x *lir.MakeRecord) {
	names, _, ok := lir.RecordFields(lir.Resolve(x.Dst.Ty))
	if !ok {
		e.errorf("the C back end doesn't compile records of no declared type yet")
		return
	}
	order := make([]int, len(x.Names))
	for i, n := range x.Names {
		order[i] = e.fieldIndex(x.Dst.Ty, n)
	}
	if len(x.Names) != len(names) {
		e.errorf("a record of type %s needs all %d of its fields", x.Dst.Ty, len(names))
		return
	}
	e.makeFields(x.Dst, x.Fields, order)
}

// update writes `{ r with … }`: in place, it takes its record and changes
// it when it is unique; otherwise it borrows it and copies it. Either way
// it takes the new fields and drops the ones they replace.
func (e *fn) update(x *lir.Update) {
	t := x.Src.Ty
	replaced := map[int]*lir.Value{}
	for i, n := range x.Names {
		replaced[e.fieldIndex(t, n)] = x.Fields[i]
	}
	if e.g.layout(t).Repr == lir.ReprBox {
		b := e.g.boxOf(t, -1).name
		e.printf("{ ey_v u_ = %s;", e.v(x.Src))
		if !x.InPlace {
			e.printf(" ey_dup(u_);")
		}
		e.printf(" ey_box_own(&u_);")
		for i, ft := range fieldTys(t) {
			v, ok := replaced[i]
			if !ok {
				continue
			}
			lv := fmt.Sprintf("((%s *)(uintptr_t)u_)->f%d", b, i)
			if st := e.g.dropStmt(ft, lv); st != "" {
				e.printf(" %s", st)
			}
			e.printf(" %s = %s;", lv, e.v(v))
		}
		e.printf(" %s = u_; }", e.v(x.Dst))
		return
	}
	d := e.v(x.Dst)
	e.printf("%s = %s;", d, e.v(x.Src))
	for i, ft := range fieldTys(t) {
		lv := fmt.Sprintf("%s.f%d", d, i)
		v, ok := replaced[i]
		switch {
		case !ok && !x.InPlace:
			if st := e.g.dupStmt(ft, lv); st != "" {
				e.printf(" %s", st)
			}
		case ok:
			if x.InPlace {
				if st := e.g.dropStmt(ft, lv); st != "" {
					e.printf(" %s", st)
				}
			}
			e.printf(" %s = %s;", lv, e.v(v))
		}
	}
}

// isVariant is the C test that the variant-type value in x is variant k.
func (e *fn) isVariant(x *lir.Value, k int) string {
	l := e.g.layout(x.Ty)
	src := e.v(x)
	switch {
	case l.Repr == lir.ReprEnum:
		return fmt.Sprintf("%s == %d", src, k)
	case l.Tag.Kind == lir.TagNone:
		return "1"
	case l.Tag.Kind == lir.TagBox:
		v := l.Variants[k]
		if len(v.Fields) == 0 {
			return fmt.Sprintf("%s == UINT64_C(%d)", src, v.Value)
		}
		others := 0
		for _, w := range l.Variants {
			if len(w.Fields) > 0 {
				others++
			}
		}
		if others == 1 {
			return fmt.Sprintf("ey_is_obj(%s)", src)
		}
		return fmt.Sprintf("(ey_is_obj(%s) && EY_OBJ(%s)->tag == %d)", src, src, k)
	case l.Tag.Kind == lir.TagNiche:
		return "(" + nicheIs(l, k, "&"+src) + ")"
	}
	return fmt.Sprintf("%s.tag == %d", src, k)
}

// payload is the C lvalue of field i of variant k of the value in x.
func (e *fn) payload(x *lir.Value, k, i int) string {
	l := e.g.layout(x.Ty)
	src := e.v(x)
	switch {
	case l.Tag.Kind == lir.TagBox:
		return fmt.Sprintf("((%s *)(uintptr_t)%s)->f%d", e.g.boxOf(x.Ty, k).name, src, i)
	case l.Word():
		return src // the payload is the word itself
	}
	return src + "." + fieldPath(l, k, i)
}

func (e *fn) makeVariant(x *lir.MakeVariant) {
	if newtype(x.Variant) {
		e.assign(x.Dst, e.v(x.Args[0]))
		return
	}
	k := variantIndex(x.Variant)
	l := e.g.layout(x.Dst.Ty)
	d := e.v(x.Dst)
	switch {
	case l.Repr == lir.ReprEnum:
		e.assign(x.Dst, fmt.Sprint(k))
	case l.Tag.Kind == lir.TagBox:
		if len(x.Args) == 0 {
			e.assign(x.Dst, fmt.Sprintf("UINT64_C(%d)", l.Variants[k].Value))
			return
		}
		b := e.g.boxOf(x.Dst.Ty, k).name
		e.printf("{ %s *b_ = ey_alloc(sizeof(%s), %s); b_->h.tag = %d;", b, b, e.g.desc(x.Dst.Ty), k)
		for i, a := range x.Args {
			e.printf(" b_->f%d = %s;", i, e.v(a))
		}
		e.printf(" %s = EY_V(b_); }", d)
	case l.Word():
		if len(x.Args) == 0 {
			e.assign(x.Dst, fmt.Sprintf("UINT64_C(%d)", l.Variants[k].Value))
		} else {
			e.assign(x.Dst, e.v(x.Args[0]))
		}
	case l.Tag.Kind == lir.TagNone || (l.Tag.Kind == lir.TagNiche && len(x.Args) > 0):
		parts := make([]string, len(x.Args))
		for i, a := range x.Args {
			parts[i] = fmt.Sprintf(".f%d = %s", i, e.v(a))
		}
		e.assign(x.Dst, fmt.Sprintf("(%s){%s}", e.ct(x.Dst), strings.Join(parts, ", ")))
	case l.Tag.Kind == lir.TagNiche:
		e.zero(x.Dst)
		e.printf("EY_AT(uint%d_t, &%s, %d) = UINT64_C(%d);", l.Tag.Size*8, d, l.Tag.Offset, l.Variants[k].Value)
	default:
		e.zero(x.Dst)
		e.printf("%s.tag = %d;", d, k)
		for i, a := range x.Args {
			e.printf(" %s.%s = %s;", d, fieldPath(l, k, i), e.v(a))
		}
	}
}

// closure sets dst to a closure of fn with its captures, which it takes.
func (e *fn) closure(dst *lir.Value, f *lir.Func, caps []*lir.Value) {
	cl := e.g.closureStruct(f)
	if cl == "" {
		e.assign(dst, fmt.Sprintf("EY_V(ey_fn_new(%s, %s_tr))", e.g.closureDesc(f), e.g.names[f]))
		return
	}
	e.printf("{ %s *c_ = (%s *)ey_fn_new(%s, %s_tr);", cl, cl, e.g.closureDesc(f), e.g.names[f])
	for i, c := range caps {
		e.printf(" c_->c%d = %s;", i, e.v(c))
	}
	e.printf(" %s = EY_V(c_); }", e.v(dst))
}

// setPlace writes a variable, or a field or element inside it, with the
// value it takes: each box and list on the path is made uniquely owned
// first, copied if it is shared, and what the place held is dropped.
func (e *fn) setPlace(x *lir.SetPlace) {
	if len(x.Path) == 0 {
		e.assign(x.Var, e.v(x.Src))
		return
	}
	t := x.Var.Ty
	e.printf("{ void *p_ = &%s;", e.v(x.Var))
	for _, s := range x.Path {
		if s.Index != nil {
			if kind(&lir.Value{Ty: t}) == "Map" {
				// A map's value is the last step (SPECS.md §7): insert or
				// replace it, in place when the map is unique. The map
				// keeps references of its own.
				e.printf(" *(ey_v *)p_ = eyo_map_insert(*(ey_v *)p_, &%s, &%s);", e.v(s.Index), e.v(x.Src))
				if st := e.g.dropStmt(x.Src.Ty, e.v(x.Src)); st != "" {
					e.printf(" %s", st)
				}
				e.printf(" }")
				return
			}
			e.printf(" p_ = eyo_list_slot(%s, (ey_v *)p_, %s);", e.at, e.v(s.Index))
			t = lir.ElemOf(rep(t))
			continue
		}
		k := e.fieldIndex(t, s.Field)
		if e.g.layout(t).Repr == lir.ReprBox {
			e.printf(" ey_box_own((ey_v *)p_); p_ = &((%s *)(uintptr_t)*(ey_v *)p_)->f%d;", e.g.boxOf(t, -1).name, k)
		} else {
			e.printf(" p_ = &((%s *)p_)->f%d;", e.g.ctype(t), k)
		}
		t = fieldType(t, s.Field)
	}
	lv := fmt.Sprintf("(*(%s *)p_)", e.g.ctype(t))
	if st := e.g.dropStmt(t, lv); st != "" {
		e.printf(" %s", st)
	}
	e.printf(" %s = %s; }", lv, e.v(x.Src))
}

// call calls an instance: `mut` arguments by their variable's address, and
// a mask of the arguments left out when the callee has defaults.
func (e *fn) call(x *lir.Call) {
	if x.Durable {
		e.errorf("the C back end doesn't compile durable steps (`!`) yet: jobs run in the Go runner")
	}
	callee := x.Fn
	mut := mutParams(callee)
	var args []string
	var mask uint64
	for i, a := range x.Args {
		switch {
		case i < len(mut) && mut[i]:
			if e.mut[a] {
				args = append(args, fmt.Sprintf("v%d", a.ID))
			} else {
				args = append(args, fmt.Sprintf("&v%d", a.ID))
			}
		case e.absent(a):
			mask |= 1 << uint(i)
			args = append(args, e.v(a)) // zero
		default:
			args = append(args, e.v(a))
		}
	}
	if hasDefaults(callee) {
		args = append(args, fmt.Sprintf("UINT64_C(%d)", mask))
	}
	e.assign(x.Dst, fmt.Sprintf("%s(%s)", e.g.names[callee], strings.Join(args, ", ")))
}

// derived calls a derived trait's method: on a scalar or an enum by C's
// own operators, and on anything else in the runtime, from the type's
// descriptor.
func (e *fn) derived(x *lir.Derived) {
	if x.Method == "variants" {
		c, _ := rep(x.Dst.Ty).(*ir.TCon)
		if c == nil || len(c.Args) != 1 {
			e.errorf("variants() gives a list")
			return
		}
		e.assign(x.Dst, fmt.Sprintf("eyb_enum_variants(%s, %s)", e.g.desc(c.Args[0]), e.g.desc(x.Dst.Ty)))
		return
	}
	if len(x.Args) == 0 {
		e.errorf("derived %s has no value to work on", x.Method)
		return
	}
	a := x.Args[0]
	d := e.g.desc(a.Ty)
	enum := e.g.layout(a.Ty).Repr == lir.ReprEnum
	switch x.Method {
	case "show":
		if d == "&ey_t_int" {
			e.assign(x.Dst, fmt.Sprintf("ey_show_int(%s)", e.v(a)))
			return
		}
		e.assign(x.Dst, fmt.Sprintf("ey_show(%s, &%s)", d, e.v(a)))
	case "eq":
		if enum {
			e.assign(x.Dst, fmt.Sprintf("%s == %s", e.v(a), e.v(x.Args[1])))
			return
		}
		e.assign(x.Dst, fmt.Sprintf("ey_eq(%s, &%s, &%s)", d, e.v(a), e.v(x.Args[1])))
	case "cmp":
		if enum {
			e.assign(x.Dst, fmt.Sprintf("EY_SMALL((%s > %s) - (%s < %s))", e.v(a), e.v(x.Args[1]), e.v(a), e.v(x.Args[1])))
			return
		}
		e.assign(x.Dst, fmt.Sprintf("EY_SMALL(ey_cmp(%s, &%s, &%s))", d, e.v(a), e.v(x.Args[1])))
	case "hash":
		e.assign(x.Dst, fmt.Sprintf("EY_SMALL((int64_t)(ey_hash(%s, &%s) >> 2))", d, e.v(a)))
	case "parse":
		e.printf("eyb_enum_parse(%s, &%s, %s);", e.v(a), e.v(x.Dst), e.g.desc(x.Dst.Ty))
	case "index":
		// The variant's place in T.variants(), its number.
		if enum {
			e.assign(x.Dst, fmt.Sprintf("EY_SMALL((int64_t)%s)", e.v(a)))
			return
		}
		e.assign(x.Dst, fmt.Sprintf("EY_SMALL((int64_t)ey_variant_of(%s, &%s))", d, e.v(a)))
	default:
		e.errorf("the C back end doesn't derive %s yet", x.Method)
	}
}
