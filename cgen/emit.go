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
	// loops are the values a for loop walks with iter.more and iter.next:
	// the iter that makes one starts a walk, not an Iter value.
	loops map[*lir.Value]bool
}

// function writes f, and what calls it as a closure, a dyn method or a
// global.
func (g *gen) function(fs *fileState, f *lir.Func) string {
	e := &fn{g: g, fs: fs, f: f, mut: map[*lir.Value]bool{}, param: map[*lir.Value]int{}, consts: map[*lir.Value]*lir.Const{}, loops: map[*lir.Value]bool{}}
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
		}
	}
	fmt.Fprintf(&e.b, "/* %s %s */\n", f.Kind, strings.ReplaceAll(f.Name, "*/", "* /"))
	e.pos = f.Src.Pos
	e.line(e.pos)
	fmt.Fprintf(&e.b, "%s {\n", g.signature(f))
	var owned []*lir.Value
	for _, v := range locals {
		fmt.Fprintf(&e.b, "\tey_v v%d = 0;\n", v.ID)
		if counted(v.Ty) {
			owned = append(owned, v)
		}
	}
	// A parameter is borrowed from the caller; one the function sets, as
	// the code that gives a left-out parameter its default does, takes a
	// reference of its own first, which it drops when it returns.
	set := map[*lir.Value]bool{}
	for _, b := range f.Blocks {
		for _, in := range b.Instrs {
			if d := lir.Dst(in); d != nil {
				set[d] = true
			}
		}
	}
	for _, p := range f.Params {
		if set[p] && !e.mut[p] && counted(p.Ty) {
			fmt.Fprintf(&e.b, "\tey_dup(v%d);\n", p.ID)
			owned = append(owned, p)
		}
	}
	if hasDefaults(f) {
		e.b.WriteString("\t(void)absent;\n")
	}
	reads := map[*lir.Value]int{}
	for _, b := range f.Blocks {
		for _, in := range b.Instrs {
			for _, r := range lir.Reads(in) {
				reads[r]++
			}
		}
		switch t := b.Term.(type) {
		case *lir.Branch:
			reads[t.Cond]++
		case *lir.Return:
			reads[t.Value]++
		}
	}
	for _, b := range f.Blocks {
		fmt.Fprintf(&e.b, "b%d:;\n", b.ID)
		for i := 0; i < len(b.Instrs); i++ {
			j := i + 1 // the next instruction that isn't an At
			for j < len(b.Instrs) {
				if _, at := b.Instrs[j].(*lir.At); !at {
					break
				}
				j++
			}
			if j < len(b.Instrs) && e.inPlace(b.Instrs[i], b.Instrs[j], reads) {
				e.flush()
				for _, at := range b.Instrs[i+1 : j] {
					e.instr(at)
				}
				i = j
				continue
			}
			e.instr(b.Instrs[i])
			e.flush()
		}
		e.term(b.Term, owned)
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
	call := func(first []string, from string, skip int) string {
		args := append([]string(nil), first...)
		for i := skip; i < len(f.Params); i++ {
			args = append(args, fmt.Sprintf("%s[%d]", from, i-skip))
		}
		if hasDefaults(f) {
			args = append(args, "0")
		}
		return fmt.Sprintf("%s(%s)", name, strings.Join(args, ", "))
	}
	if g.tramps[f] {
		n := captures(f)
		var caps []string
		for i := 0; i < n; i++ {
			caps = append(caps, fmt.Sprintf("self->cap[%d]", i))
		}
		fmt.Fprintf(&e.b, "ey_v %s_tr(ey_fn *self, const ey_v *a) {\n\t(void)self;\n\t(void)a;\n\treturn %s;\n}\n\n", name, call(caps, "a", n))
	}
	if g.methods[f] {
		fmt.Fprintf(&e.b, "ey_v %s_dm(ey_v self, const ey_v *a) {\n\t(void)a;\n\treturn %s;\n}\n\n", name, call([]string{"self"}, "a", 1))
	}
	if f.Kind == lir.KindGlobal {
		fmt.Fprintf(&e.b, "ey_v %s_g(void) {\n\tstatic bool done;\n\tstatic ey_v value;\n\tif (!done) {\n\t\tvalue = %s();\n\t\tdone = true;\n\t}\n\treturn value;\n}\n\n", name, name)
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

// assign sets dst to expr, a value the expression owns: the old value of a
// counted dst is dropped.
func (e *fn) assign(dst *lir.Value, expr string) {
	if dst == nil {
		e.printf("(void)(%s);", expr)
		return
	}
	if counted(dst.Ty) {
		e.printf("{ ey_v t_ = %s; ey_drop(%s); %s = t_; }", expr, e.v(dst), e.v(dst))
		return
	}
	e.printf("%s = %s;", e.v(dst), expr)
}

// borrowed sets dst to a value it reads from elsewhere, which it dups.
func (e *fn) borrowed(dst *lir.Value, expr string) {
	if counted(dst.Ty) {
		e.printf("{ ey_v t_ = %s; ey_dup(t_); ey_drop(%s); %s = t_; }", expr, e.v(dst), e.v(dst))
		return
	}
	e.printf("%s = %s;", e.v(dst), expr)
}

// array is a C compound literal of values, or NULL for none.
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

func (e *fn) instr(in lir.Instr) {
	switch x := in.(type) {
	case *lir.At:
		e.pos = x.Pos
		e.at = cstring(posString(x.Pos))
	case *lir.Const:
		e.constant(x)
	case *lir.Move:
		e.borrowed(x.Dst, e.v(x.Src))
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
		e.assign(x.Dst, fmt.Sprintf("ey_call(%s, %s)", e.v(x.Callee), e.array(x.Args)))
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
		e.assign(x.Dst, fmt.Sprintf("EY_DYNP(%s)->methods[%d](EY_DYNP(%s)->val, %s)", e.v(x.Recv), k, e.v(x.Recv), e.array(x.Args)))
	case *lir.Op:
		e.op(x)
	case *lir.Derived:
		e.derived(x)
	case *lir.MakeRecord:
		e.makeRecord(x)
	case *lir.MakeVariant:
		e.makeVariant(x)
	case *lir.MakeTuple:
		e.makeAgg(x.Dst, e.g.desc(x.Dst.Ty), 0, x.Elems, nil)
	case *lir.MakeList:
		e.assign(x.Dst, fmt.Sprintf("ey_list_new(%s, %d, %s)", e.g.desc(x.Dst.Ty), len(x.Elems), e.array(x.Elems)))
	case *lir.MakeSet:
		e.assign(x.Dst, fmt.Sprintf("ey_set_new(%s, %d, %s)", e.g.desc(x.Dst.Ty), len(x.Elems), e.array(x.Elems)))
	case *lir.MakeMap:
		e.assign(x.Dst, fmt.Sprintf("ey_map_new(%s, %d, %s, %s)", e.g.desc(x.Dst.Ty), len(x.Keys), e.array(x.Keys), e.array(x.Values)))
	case *lir.MakeClosure:
		e.closure(x.Dst, x.Fn, x.Captures)
	case *lir.MakeDyn:
		name, ok := e.g.dynTab[x.Table]
		if !ok {
			name = fmt.Sprintf("ey_dt%d", len(e.g.dynTab))
			e.g.dynTab[x.Table] = name
		}
		e.printf("{ ey_dyn *d_ = ey_alloc(sizeof(ey_dyn), &%s_d); d_->methods = %s; d_->val = %s;", name, name, e.v(x.Src))
		if counted(x.Src.Ty) {
			e.printf("  ey_dup(d_->val);")
		}
		e.printf("  ey_drop(%s); %s = EY_V(d_); }", e.v(x.Dst), e.v(x.Dst))
	case *lir.Field:
		i := e.fieldIndex(x.Src.Ty, x.Name)
		e.borrowed(x.Dst, fmt.Sprintf("EY_AGG(%s)->f[%d]", e.v(x.Src), i))
	case *lir.TupleGet:
		e.borrowed(x.Dst, fmt.Sprintf("EY_AGG(%s)->f[%d]", e.v(x.Src), x.Index))
	case *lir.Payload:
		if newtype(x.Variant) {
			e.borrowed(x.Dst, e.v(x.Src))
			return
		}
		e.borrowed(x.Dst, fmt.Sprintf("EY_AGG(%s)->f[%d]", e.v(x.Src), x.Index))
	case *lir.IsVariant:
		if newtype(x.Variant) {
			e.assign(x.Dst, "1")
			return
		}
		e.assign(x.Dst, fmt.Sprintf("EY_AGG(%s)->tag == %d", e.v(x.Src), variantIndex(x.Variant)))
	case *lir.Update:
		e.printf("{ ey_v u_ = %s; ey_dup(u_); ey_agg_own(&u_);", e.v(x.Src))
		for i, n := range x.Names {
			k := e.fieldIndex(x.Src.Ty, n)
			ft := lir.Resolve(fieldType(x.Src.Ty, n))
			if counted(ft) {
				e.printf("  ey_drop(EY_AGG(u_)->f[%d]); ey_dup(%s);", k, e.v(x.Fields[i]))
			}
			e.printf("  EY_AGG(u_)->f[%d] = %s;", k, e.v(x.Fields[i]))
		}
		e.printf("  ey_drop(%s); %s = u_; }", e.v(x.Dst), e.v(x.Dst))
	case *lir.SetPlace:
		e.setPlace(x)
	case *lir.Global:
		e.borrowed(x.Dst, e.g.names[x.Init]+"_g()")
	default:
		e.errorf("the C back end can't compile %T yet", in)
	}
}

// inPlace writes a variable's update by itself, an operation on the
// variable whose result only goes back into it (`s += t`, `xs += [x]`,
// `xs = xs.add(x)`, `m = m.insert(k, v)`, `s = s.add(x)`, `s = "{s}…"`), as
// one call that takes the variable's value and changes it in place when
// the variable holds its only reference. It reports whether in and next
// are such a pair. Counting stays simple (7.5 makes it precise): the
// variable owns a reference, so a count of one means no one else sees the
// change.
func (e *fn) inPlace(in, next lir.Instr, reads map[*lir.Value]int) bool {
	set, ok := next.(*lir.SetPlace)
	if !ok || len(set.Path) > 0 || set.Src != lir.Dst(in) || reads[set.Src] != 1 {
		return false
	}
	v := set.Var
	rs := lir.Reads(in)
	if len(rs) == 0 || rs[0] != v {
		return false
	}
	for _, r := range rs[1:] {
		if r == v { // `xs += xs`: the variable is read after it is taken
			return false
		}
	}
	var call string
	switch x := in.(type) {
	case *lir.Op:
		if x.Op != "+" || len(x.Args) != 2 || x.Args[0] != v {
			return false
		}
		switch kind(v) {
		case "String":
			call = fmt.Sprintf("eyo_str_append(o_, %s)", e.v(x.Args[1]))
		case "List":
			call = fmt.Sprintf("eyo_list_concat(o_, %s)", e.v(x.Args[1]))
		default:
			return false
		}
	case *lir.Builtin:
		if x.Durable || len(x.Args) == 0 || x.Args[0] != v {
			return false
		}
		for _, n := range x.Names {
			if n != "" {
				return false
			}
		}
		switch {
		case x.Name == "concat" && kind(v) == "String":
			call = fmt.Sprintf("eyo_concat(o_, %d, %s)", len(x.Args)-1, e.array(x.Args[1:]))
		case x.Name == "add" && len(x.Args) == 2 && kind(v) == "List":
			call = fmt.Sprintf("eyo_list_add(o_, %s)", e.v(x.Args[1]))
		case x.Name == "add" && len(x.Args) == 2 && kind(v) == "Set":
			call = fmt.Sprintf("eyo_set_add(o_, %s)", e.v(x.Args[1]))
		case x.Name == "insert" && len(x.Args) == 3 && kind(v) == "Map":
			call = fmt.Sprintf("eyo_map_insert(o_, %s, %s)", e.v(x.Args[1]), e.v(x.Args[2]))
		default:
			return false
		}
	default:
		return false
	}
	e.printf("{ ey_v o_ = %s; %s = 0; %s = %s; }", e.v(v), e.v(v), e.v(v), call)
	return true
}

func (e *fn) term(t lir.Term, locals []*lir.Value) {
	switch x := t.(type) {
	case *lir.Jump:
		e.printf("goto b%d;", x.To.ID)
	case *lir.Branch:
		e.printf("if (%s) goto b%d;", e.v(x.Cond), x.Then.ID)
		e.printf("goto b%d;", x.Else.ID)
	case *lir.Return:
		r := "0"
		if x.Value != nil {
			r = e.v(x.Value)
		}
		e.printf("{ ey_v r_ = %s;", r)
		if x.Value != nil && counted(x.Value.Ty) {
			e.printf("  ey_dup(r_);")
		}
		for _, v := range locals {
			e.printf("  ey_drop(v%d);", v.ID)
		}
		e.printf("  return r_; }")
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
		e.assign(c.Dst, "0")
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
		e.assign(c.Dst, fmt.Sprintf("(ey_v)INT64_C(%d)", int64(v)))
	case ir.Size:
		e.assign(c.Dst, fmt.Sprintf("(ey_v)INT64_C(%d)", int64(v)))
	case ir.Date:
		e.assign(c.Dst, fmt.Sprintf("(ey_v)INT64_C(%d)", daysFromCivil(v.Year, v.Month, v.Day)))
	case ir.TimeOfDay:
		e.assign(c.Dst, fmt.Sprintf("(ey_v)%d", v.Hour*3600+v.Minute*60+v.Second))
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
		e.assign(dst, fmt.Sprintf("UINT64_C(%d)", bits))
	default:
		e.assign(dst, e.fs.bigInt(n))
	}
}

func floatLit(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return "ey_f2v(HUGE_VAL)"
	case math.IsInf(f, -1):
		return "ey_f2v(-HUGE_VAL)"
	case math.IsNaN(f):
		return "ey_f2v(NAN)"
	}
	return "ey_f2v(" + strconv.FormatFloat(f, 'x', -1, 64) + ")"
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

// makeAgg sets dst to a new record, variant or tuple, with fields in
// order, each dup'ed.
func (e *fn) makeAgg(dst *lir.Value, desc string, tag int, fields []*lir.Value, order []int) {
	e.printf("{ ey_agg *a_ = ey_agg_new(%s, %d, %d);", desc, tag, len(fields))
	for i, f := range fields {
		k := i
		if order != nil {
			k = order[i]
		}
		if f != nil && counted(f.Ty) {
			e.printf("  ey_dup(%s);", e.v(f))
		}
		e.printf("  a_->f[%d] = %s;", k, e.v(f))
	}
	e.printf("  ey_drop(%s); %s = EY_V(a_); }", e.v(dst), e.v(dst))
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
	e.makeAgg(x.Dst, e.g.desc(x.Dst.Ty), 0, x.Fields, order)
}

func (e *fn) makeVariant(x *lir.MakeVariant) {
	if newtype(x.Variant) {
		e.borrowed(x.Dst, e.v(x.Args[0]))
		return
	}
	e.makeAgg(x.Dst, e.g.desc(x.Dst.Ty), variantIndex(x.Variant), x.Args, nil)
}

// closure sets dst to a closure of fn with its captures.
func (e *fn) closure(dst *lir.Value, f *lir.Func, caps []*lir.Value) {
	e.printf("{ ey_fn *c_ = ey_fn_new(%s, %s_tr, %d);", e.g.closureDesc(f), e.g.names[f], len(caps))
	for i, c := range caps {
		if counted(c.Ty) {
			e.printf("  ey_dup(%s);", e.v(c))
		}
		e.printf("  c_->cap[%d] = %s;", i, e.v(c))
	}
	e.printf("  ey_drop(%s); %s = EY_V(c_); }", e.v(dst), e.v(dst))
}

// setPlace writes a variable, or a field inside it: each record on the
// path is made uniquely owned first, copied if it is shared.
func (e *fn) setPlace(x *lir.SetPlace) {
	if len(x.Path) == 0 {
		e.borrowed(x.Var, e.v(x.Src))
		return
	}
	e.printf("{ ey_v *p_ = &%s;", e.v(x.Var))
	t := x.Var.Ty
	for _, name := range x.Path {
		k := e.fieldIndex(t, name)
		e.printf("  ey_agg_own(p_); p_ = &EY_AGG(*p_)->f[%d];", k)
		t = fieldType(t, name)
	}
	if counted(x.Src.Ty) {
		e.printf("  ey_dup(%s); ey_drop(*p_);", e.v(x.Src))
	}
	e.printf("  *p_ = %s; }", e.v(x.Src))
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
			args = append(args, "0")
		default:
			args = append(args, e.v(a))
		}
	}
	if hasDefaults(callee) {
		args = append(args, fmt.Sprintf("UINT64_C(%d)", mask))
	}
	e.assign(x.Dst, fmt.Sprintf("%s(%s)", e.g.names[callee], strings.Join(args, ", ")))
}

// derived calls a derived trait's method, which the runtime answers from
// the type's descriptor.
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
	switch x.Method {
	case "show":
		e.assign(x.Dst, fmt.Sprintf("ey_show(%s, %s)", d, e.v(a)))
	case "eq":
		e.assign(x.Dst, fmt.Sprintf("ey_eq(%s, %s, %s)", d, e.v(a), e.v(x.Args[1])))
	case "cmp":
		e.assign(x.Dst, fmt.Sprintf("EY_SMALL(ey_cmp(%s, %s, %s))", d, e.v(a), e.v(x.Args[1])))
	case "hash":
		e.assign(x.Dst, fmt.Sprintf("EY_SMALL((int64_t)(ey_hash(%s, %s) >> 2))", d, e.v(a)))
	case "parse":
		e.assign(x.Dst, fmt.Sprintf("eyb_enum_parse(%s, %s)", e.v(a), e.g.desc(x.Dst.Ty)))
	default:
		e.errorf("the C back end doesn't derive %s yet", x.Method)
	}
}
