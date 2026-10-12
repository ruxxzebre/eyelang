package cgen

import (
	"fmt"
	"strings"

	"eyelang/ir"
	"eyelang/lir"
)

// The built-in functions and operators, specialised by the types of their
// arguments. What the interpreter does with each (interp/builtins.go,
// strings.go, offsets.go, fixed.go) is what the C does.

// params names the parameters of the built-ins that take an argument by
// name or leave one out, in order.
var params = map[string][]string{
	"parseInt":   {"s", "base"},
	"indexOf":    {"s", "part", "from"},
	"after":      {"s", "i", "by"},
	"before":     {"s", "i", "by"},
	"shiftLeft":  {"a", "by"},
	"shiftRight": {"a", "by"},
	"join":       {"xs", "sep"},
}

// bargs are a built-in's arguments in its parameters' order; nil for one
// left out.
func bargs(in *lir.Builtin) []*lir.Value {
	ps, ok := params[in.Name]
	if !ok {
		return in.Args
	}
	out := make([]*lir.Value, len(ps))
	next := 0
	for i, a := range in.Args {
		name := ""
		if i < len(in.Names) {
			name = in.Names[i]
		}
		k := -1
		if name == "" {
			for next < len(out) && out[next] != nil {
				next++
			}
			k = next
		} else {
			for j, p := range ps {
				if p == name {
					k = j
				}
			}
		}
		if k >= 0 && k < len(out) {
			out[k] = a
		}
	}
	return out
}

// kind is the built-in type a value's type is represented as: "List",
// "Map", "String", "Int", …, or "" for a declared type.
func kind(v *lir.Value) string {
	if v == nil {
		return ""
	}
	if c, ok := rep(v.Ty).(*ir.TCon); ok && c.Decl != nil {
		// the prelude's Option and Result; a program's own modules have a
		// directory
		if (c.Name == "Option" || c.Name == "Result") && (c.Decl.Module == nil || c.Decl.Module.Dir == "") {
			return c.Name
		}
		return ""
	}
	return scalarName(v.Ty)
}

func (e *fn) unsupported(what string) {
	e.errorf("the C back end doesn't support %s yet", what)
}

func (e *fn) builtin(in *lir.Builtin) {
	if in.Durable {
		e.errorf("the C back end doesn't compile durable steps (`!`) yet: jobs run in the Go runner")
		return
	}
	a := bargs(in)
	arg := func(i int) *lir.Value {
		if i < len(a) && a[i] != nil && !e.absent(a[i]) {
			return a[i]
		}
		return nil
	}
	v := func(i int) string { return e.v(arg(i)) }
	p := func(i int) string { return "&" + e.v(arg(i)) } // an element, by its address
	has := func(i int) string {
		if arg(i) != nil {
			return "true"
		}
		return "false"
	}
	dst := in.Dst
	rt := func() string { return e.g.desc(dst.Ty) }
	out := func() string { return "&" + e.v(dst) }
	k0 := kind(arg(0))
	set := func(expr string) { e.assign(dst, expr) }
	stmt := func(format string, args ...any) { e.printf(format, args...) }
	unit := func() {
		if dst != nil {
			e.zero(dst)
		}
	}
	name := in.Name

	if in.InPlace {
		// The update's last use of its first argument: it takes it, and
		// changes it in place when it holds the only reference.
		switch {
		case name == "concat":
			set(fmt.Sprintf("eyo_concat(%s, %d, %s)", e.v(in.Args[0]), len(in.Args)-1, e.array(in.Args[1:])))
		case name == "add" && k0 == "List":
			set(fmt.Sprintf("eyo_list_add(%s, %s)", v(0), p(1)))
		case name == "add" && k0 == "Set":
			set(fmt.Sprintf("eyo_set_add(%s, %s)", v(0), p(1)))
		case name == "insert" && k0 == "Map":
			set(fmt.Sprintf("eyo_map_insert(%s, %s, %s)", v(0), p(1), p(2)))
		case name == "remove" && (k0 == "Map" || k0 == "Set"):
			set(fmt.Sprintf("eyo_map_remove(%s, %s)", v(0), p(1)))
		default:
			e.unsupported("the update " + name + " of " + arg(0).Ty.String() + " in place")
		}
		return
	}
	if w, ok := widths[k0]; ok {
		if e.fixed(in, w.bits, w.signed, a) {
			return
		}
	}
	if strings.HasSuffix(name, ".from") {
		if w, ok := widths[strings.TrimSuffix(name, ".from")]; ok {
			stmt("eyb_fixed_from(%s, %d, %v, %s, %s);", v(0), w.bits, w.signed, out(), rt())
			return
		}
	}
	if strings.HasPrefix(name, "log.") {
		stmt("ey_log(%s, %d, %s);", cstring(strings.TrimPrefix(name, "log.")), len(in.Args), e.array(in.Args))
		unit()
		return
	}
	if (k0 == "Iter" || name == "zip" && kind(arg(1)) == "Iter") && e.iterBuiltin(in, arg) {
		return
	}
	// The time methods (times.go).
	if e.timeBuiltin(in) {
		return
	}
	switch name {
	case "expect":
		switch len(in.Args) {
		case 4: // the condition, its text, and the left side's text and value
			stmt("if (!%s) ey_expect_failed(%s, %s, %s, %s, &%s);", v(0), e.at, v(1), v(2), e.g.desc(in.Args[3].Ty), v(3))
		case 2:
			stmt("if (!%s) ey_expect_failed(%s, %s, 0, NULL, NULL);", v(0), e.at, v(1))
		default:
			stmt("if (!%s) ey_fault(%s, \"expectation failed\");", v(0), e.at)
		}
		unit()
	case "unreachable":
		if arg(0) == nil {
			stmt("ey_fault(%s, \"unreachable\");", e.at)
		} else {
			stmt("ey_unreachable(%s, ey_show(%s, %s));", e.at, e.g.desc(arg(0).Ty), p(0))
		}
		unit()
	case "print":
		stmt("ey_print(%d, %s);", len(in.Args), e.array(in.Args))
		unit()
	case "concat":
		set(fmt.Sprintf("ey_concat(%d, %s)", len(in.Args), e.array(in.Args)))
	case "absent":
		i, ok := e.param[in.Args[0]]
		if !ok || !hasDefaults(e.f) {
			e.errorf("absent() of what isn't a parameter with a default")
			return
		}
		set(fmt.Sprintf("(absent >> %d) & 1", i))
	case "now":
		set("ey_now()")
	case "env.get":
		stmt("eyb_env_get(%s, %s, %s);", v(0), out(), rt())
	case "inZone":
		c := e.consts[in.Args[0]]
		inst, ok := c.Value.(ir.Instant)
		if c == nil || !ok {
			e.unsupported("a time in a zone that isn't written out")
			return
		}
		set(e.instant(inst, e.v(in.Args[1])))
	case "range":
		// The range keeps its ends, as words.
		flags := e.rangeFlags(in.Args[0], in.Args[1], in.Args[2])
		s, en := "0", "0"
		if !e.absent(in.Args[0]) {
			s = e.word(in.Args[0])
			stmt("%s", e.g.dupStmt(in.Args[0].Ty, e.v(in.Args[0])))
		}
		if !e.absent(in.Args[1]) {
			en = e.word(in.Args[1])
			stmt("%s", e.g.dupStmt(in.Args[1].Ty, e.v(in.Args[1])))
		}
		set(fmt.Sprintf("ey_range(%s, %s, %s, %s)", rt(), s, en, flags))
	case "iter":
		switch {
		case k0 != "Range" && k0 != "List" && k0 != "Set" && k0 != "Map" && k0 != "String" && k0 != "Bytes" && k0 != "Iter":
			e.unsupported("walking " + arg(0).Ty.String())
		case e.loops[dst]:
			set(fmt.Sprintf("ey_loop_new(%s, %s)", v(0), e.elemDesc(dst.Ty)))
		default:
			set(fmt.Sprintf("ey_lazy_of(%s, %s)", v(0), e.elemDesc(dst.Ty)))
		}
	case "iter.more":
		set(fmt.Sprintf("ey_loop_more(%s)", v(0)))
	case "iter.next":
		stmt("ey_loop_next(%s, %s);", v(0), out())
	case "chars":
		set(fmt.Sprintf("ey_lazy_of(%s, &ey_t_char)", v(0)))
	case "div", "rem":
		stmt("eyb_int_div_rem(%s, %s, %v, %s, %s);", v(0), v(1), name == "rem", out(), rt())
	case "average":
		e.seq(in, 0, func(c string) string { return fmt.Sprintf("eyb_seq_average(%s, %s, %s);", c, out(), rt()) })
	case "parallelMap":
		// One thread runs a program (compilation.md, section 2): the
		// elements are mapped in order, which gives the same list.
		e.seq(in, 0, func(c string) string { return e.setTo(dst, fmt.Sprintf("eyb_seq_map(%s, %s, %s)", c, v(1), rt())) })
	case "size", "count":
		switch {
		case k0 == "String" && name == "size":
			set(fmt.Sprintf("eyb_str_size(%s)", v(0)))
		case name == "count" && len(in.Args) == 2:
			e.seq(in, 0, func(c string) string { return e.setTo(dst, fmt.Sprintf("eyb_seq_test(%s, %s, 2)", c, v(1))) })
		case k0 == "List" || k0 == "Set" || k0 == "Map" || k0 == "Bytes":
			set(fmt.Sprintf("EY_SMALL(ey_coll_size(%s))", v(0)))
		default:
			e.unsupported(name + " of " + arg(0).Ty.String())
		}
	case "isEmpty":
		switch k0 {
		case "String":
			set(fmt.Sprintf("EY_STR(%s)->len == 0", v(0)))
		case "List", "Set", "Map", "Bytes":
			set(fmt.Sprintf("ey_coll_size(%s) == 0", v(0)))
		default:
			e.unsupported("isEmpty of " + arg(0).Ty.String())
		}
	case "at":
		switch k0 {
		case "List":
			stmt("eyb_list_at(%s, %s, %s, %s);", e.at, v(0), v(1), out())
		case "Bytes":
			set(fmt.Sprintf("eyb_bytes_at(%s, %s, %s)", e.at, v(0), v(1)))
		default:
			e.unsupported("list patterns on " + arg(0).Ty.String())
		}
	case "index", "get":
		switch k0 {
		case "List":
			stmt("eyb_list_index(%s, %s, %s, %s);", v(0), v(1), out(), rt())
		case "Bytes":
			stmt("eyb_bytes_index(%s, %s, %s, %s);", v(0), v(1), out(), rt())
		case "Map":
			stmt("eyb_map_get(%s, %s, %s, %s);", v(0), p(1), out(), rt())
		default:
			e.unsupported("indexing " + arg(0).Ty.String())
		}
	case "slice":
		if len(in.Args) == 2 && k0 == "String" {
			stmt("eyb_str_slice(%s, %s, %s, %s);", v(0), v(1), out(), rt())
			return
		}
		if len(in.Args) != 4 {
			e.unsupported("slice with these arguments")
			return
		}
		flags := e.rangeFlags(in.Args[1], in.Args[2], in.Args[3])
		s, en := "0", "0"
		if !e.absent(in.Args[1]) {
			s = e.v(in.Args[1])
		}
		if !e.absent(in.Args[2]) {
			en = e.v(in.Args[2])
		}
		switch k0 {
		case "List":
			set(fmt.Sprintf("eyb_list_slice(%s, %s, %s, %s)", v(0), s, en, flags))
		case "Bytes":
			set(fmt.Sprintf("eyb_bytes_slice(%s, %s, %s, %s)", v(0), s, en, flags))
		default:
			e.unsupported("slicing " + arg(0).Ty.String())
		}
	case "add":
		switch k0 {
		case "List":
			set(fmt.Sprintf("eyb_list_add(%s, %s)", v(0), p(1)))
		case "Set":
			set(fmt.Sprintf("eyb_set_add(%s, %s)", v(0), p(1)))
		default:
			e.unsupported("add to " + arg(0).Ty.String())
		}
	case "remove":
		if k0 == "Set" || k0 == "Map" {
			set(fmt.Sprintf("eyb_map_remove(%s, %s)", v(0), p(1)))
		} else {
			e.unsupported("remove from " + arg(0).Ty.String())
		}
	case "contains":
		switch k0 {
		case "Set", "Map":
			set(fmt.Sprintf("eyb_map_contains(%s, %s)", v(0), p(1)))
		case "List":
			set(fmt.Sprintf("eyb_list_contains(%s, %s)", v(0), p(1)))
		case "String":
			set(fmt.Sprintf("eyb_str_contains(%s, %s)", v(0), v(1)))
		case "Bytes":
			set(fmt.Sprintf("eyb_bytes_contains(%s, %s)", v(0), v(1)))
		default:
			e.unsupported("contains on " + arg(0).Ty.String())
		}
	case "insert":
		if k0 == "Map" {
			set(fmt.Sprintf("eyb_map_insert(%s, %s, %s)", v(0), p(1), p(2)))
		} else {
			e.unsupported("insert into " + arg(0).Ty.String())
		}
	case "first", "last":
		e.seq(in, 0, func(c string) string {
			return fmt.Sprintf("eyb_seq_end(%s, %v, %s, %s);", c, name == "last", out(), rt())
		})
	case "map":
		switch k0 {
		case "Option", "Result":
			e.mapVariant(in)
		default:
			e.seq(in, 0, func(c string) string { return e.setTo(dst, fmt.Sprintf("eyb_seq_map(%s, %s, %s)", c, v(1), rt())) })
		}
	case "filter":
		e.seq(in, 0, func(c string) string { return e.setTo(dst, fmt.Sprintf("eyb_seq_filter(%s, %s, %s)", c, v(1), rt())) })
	case "find":
		e.seq(in, 0, func(c string) string { return fmt.Sprintf("eyb_seq_find(%s, %s, %s, %s);", c, v(1), out(), rt()) })
	case "any", "all":
		how := 0
		if name == "all" {
			how = 1
		}
		e.seq(in, 0, func(c string) string {
			return e.setTo(dst, fmt.Sprintf("(bool)eyb_seq_test(%s, %s, %d)", c, v(1), how))
		})
	case "reduce":
		e.seq(in, 0, func(c string) string {
			return fmt.Sprintf("eyb_seq_reduce(%s, %s, %s, %s, %s);", c, p(1), v(2), out(), rt())
		})
	case "sum":
		e.seq(in, 0, func(c string) string { return e.setTo(dst, e.fromWord(dst.Ty, fmt.Sprintf("eyb_seq_sum(%s)", c))) })
	case "enumerate":
		e.seq(in, 0, func(c string) string { return e.setTo(dst, fmt.Sprintf("eyb_seq_enumerate(%s, %s)", c, rt())) })
	case "take", "drop":
		if k0 == "String" {
			set(fmt.Sprintf("eyb_str_%s(%s, %s, %s)", name, e.at, v(0), v(1)))
			return
		}
		e.seq(in, 0, func(c string) string {
			return e.setTo(dst, fmt.Sprintf("eyb_seq_take_drop(%s, %s, %s, %v, %s)", e.at, c, v(1), name == "take", rt()))
		})
	case "reverse":
		e.seq(in, 0, func(c string) string { return e.setTo(dst, fmt.Sprintf("eyb_seq_reverse(%s, %s)", c, rt())) })
	case "sort":
		e.seq(in, 0, func(c string) string { return e.setTo(dst, fmt.Sprintf("eyb_seq_sort(%s, %s)", c, rt())) })
	case "keys", "values", "entries":
		if k0 != "Map" {
			if name == "entries" {
				e.seq(in, 0, func(c string) string { return e.setTo(dst, fmt.Sprintf("eyb_seq_convert(%s, %s)", c, rt())) })
				return
			}
			e.unsupported(name + " of " + arg(0).Ty.String())
			return
		}
		which := map[string]int{"keys": 0, "values": 1, "entries": 2}[name]
		set(fmt.Sprintf("eyb_map_list(%s, %d, %s)", v(0), which, rt()))
	case "toList", "toSet":
		e.seq(in, 0, func(c string) string { return e.setTo(dst, fmt.Sprintf("eyb_seq_convert(%s, %s)", c, rt())) })
	case "join":
		sep := "0"
		if arg(1) != nil {
			sep = v(1)
		}
		e.seq(in, 0, func(c string) string { return e.setTo(dst, fmt.Sprintf("eyb_seq_join(%s, %s)", c, sep)) })
	case "flatMap":
		e.seq(in, 0, func(c string) string { return e.setTo(dst, fmt.Sprintf("eyb_seq_flat_map(%s, %s, %s)", c, v(1), rt())) })
	case "zip":
		if kind(arg(1)) != "List" && kind(arg(1)) != "Set" {
			e.unsupported("zip with " + arg(1).Ty.String())
			return
		}
		e.seq(in, 0, func(c string) string { return e.setTo(dst, fmt.Sprintf("eyb_seq_zip(%s, %s, %s)", c, v(1), rt())) })
	case "min", "max":
		op := ">"
		if name == "max" {
			op = "<"
		}
		e.dupped(dst, fmt.Sprintf("ey_cmp(%s, %s, %s) %s 0 ? %s : %s", e.g.desc(arg(0).Ty), p(0), p(1), op, v(1), v(0)))
	case "abs":
		switch k0 {
		case "Int":
			set(fmt.Sprintf("ey_int_lt(%s, EY_SMALL(0)) ? ey_int_neg(%s) : (ey_dup(%s), %s)", v(0), v(0), v(0), v(0)))
		case "Float":
			set(fmt.Sprintf("fabs(%s)", v(0)))
		default:
			e.unsupported("abs of " + arg(0).Ty.String())
		}
	case "mapErr":
		if k0 != "Result" {
			e.unsupported("mapErr on " + arg(0).Ty.String())
			return
		}
		e.mapErr(in)
	case "nonzero":
		if k0 != "Int" {
			e.unsupported("nonzero of " + arg(0).Ty.String())
			return
		}
		stmt("if (ey_int_eq(%s, EY_SMALL(0))) ey_none(%s, %s); else { ey_dup(%s); ey_some(%s, %s, %s); }", v(0), rt(), out(), v(0), rt(), out(), p(0))
	case "isOk", "isErr":
		tag := 0
		if name == "isErr" {
			tag = 1
		}
		set(e.isVariant(arg(0), tag))
	case "lines", "words", "split":
		fn := map[string]string{"lines": "eyb_str_lines", "words": "eyb_str_words", "split": "eyb_str_split"}[name]
		if name == "split" {
			set(fmt.Sprintf("%s(%s, %s, %s)", fn, v(0), v(1), rt()))
		} else {
			set(fmt.Sprintf("%s(%s, %s)", fn, v(0), rt()))
		}
	case "trim":
		set(fmt.Sprintf("eyb_str_trim(%s)", v(0)))
	case "startsWith":
		set(fmt.Sprintf("eyb_str_starts_with(%s, %s)", v(0), v(1)))
	case "endsWith":
		set(fmt.Sprintf("eyb_str_ends_with(%s, %s)", v(0), v(1)))
	case "replace":
		set(fmt.Sprintf("eyb_str_replace(%s, %s, %s)", v(0), v(1), v(2)))
	case "lower", "upper":
		set(fmt.Sprintf("eyb_str_case(%s, %v)", v(0), name == "upper"))
	case "bytes":
		set(fmt.Sprintf("eyb_str_bytes(%s)", v(0)))
	case "toText":
		stmt("eyb_bytes_to_text(%s, %s, %s);", v(0), out(), rt())
	case "toBytes":
		set(fmt.Sprintf("eyb_list_to_bytes(%s)", v(0)))
	case "code":
		set(fmt.Sprintf("EY_SMALL((int64_t)%s)", v(0)))
	case "fromCode":
		stmt("eyb_from_code(%s, %s, %s);", v(0), out(), rt())
	case "parseInt":
		base := "0"
		if arg(1) != nil {
			base = v(1)
		}
		stmt("eyb_parse_int(%s, %s, %s, %s, %s, %s);", e.at, v(0), base, has(1), out(), rt())
	case "parseFloat":
		stmt("eyb_parse_float(%s, %s, %s);", v(0), out(), rt())
	case "round", "floor", "ceil":
		set(fmt.Sprintf("%s(%s)", name, v(0)))
	case "toInt":
		switch k0 {
		case "Float":
			stmt("eyb_float_to_int(%s, %s, %s);", v(0), out(), rt())
		case "Int":
			e.dupped(dst, v(0))
		default:
			e.unsupported("toInt of " + arg(0).Ty.String())
		}
	case "toFloat":
		set(fmt.Sprintf("ey_int_to_f64(%s)", v(0)))
	case "start":
		set("EY_SMALL(0)")
	case "end":
		set(fmt.Sprintf("EY_SMALL(EY_STR(%s)->len)", v(0)))
	case "indexOf":
		from := "0"
		if arg(2) != nil {
			from = v(2)
		}
		stmt("eyb_str_index_of(%s, %s, %s, %s, %s, %s);", v(0), v(1), from, has(2), out(), rt())
	case "after", "before":
		by := "0"
		if arg(2) != nil {
			by = v(2)
		}
		stmt("eyb_str_step(%s, %s, %s, %s, %s, %v, %s, %s);", e.at, v(0), v(1), by, has(2), name == "after", out(), rt())
	case "charAt":
		stmt("eyb_str_char_at(%s, %s, %s, %s);", v(0), v(1), out(), rt())
	default:
		e.unsupported("the built-in " + name)
	}
}

// setTo is the C that sets dst to expr.
func (e *fn) setTo(dst *lir.Value, expr string) string {
	if dst == nil {
		return fmt.Sprintf("(void)(%s);", expr)
	}
	return fmt.Sprintf("%s = %s;", e.v(dst), expr)
}

// elemDesc is the descriptor of the elements of a collection or an Iter
// of type t.
func (e *fn) elemDesc(t ir.Ty) string { return e.g.desc(lir.ElemOf(rep(t))) }

// iterBuiltin writes a built-in on an Iter that doesn't collect it first:
// the steps that make a longer pipeline, and the functions that stop as
// soon as they know. It reports whether name is one; every other function
// collects the Iter into a list (seq).
func (e *fn) iterBuiltin(in *lir.Builtin, arg func(int) *lir.Value) bool {
	dst := in.Dst
	v := func(i int) string { return e.v(arg(i)) }
	rt := func() string { return e.g.desc(dst.Ty) }
	out := "&" + e.v(dst)
	step := func(op, fn, n, other string) {
		e.assign(dst, fmt.Sprintf("ey_lazy_then(%s, %s, %s, %s, %s, %s)", v(0), op, fn, n, other, e.elemDesc(dst.Ty)))
	}
	count := func() string {
		return fmt.Sprintf("ey_count(%s, %s)", e.at, v(1))
	}
	switch in.Name {
	case "map":
		step("EY_STEP_MAP", v(1), "0", "0")
	case "filter":
		step("EY_STEP_FILTER", v(1), "0", "0")
	case "flatMap":
		step("EY_STEP_FLATMAP", v(1), "0", "0")
	case "take":
		step("EY_STEP_TAKE", "0", count(), "0")
	case "drop":
		step("EY_STEP_DROP", "0", count(), "0")
	case "enumerate":
		step("EY_STEP_ENUMERATE", "0", "0", "0")
	case "zip":
		// The first may be a collection: its Iter comes first.
		e.printf("{ ey_v z_ = ey_lazy_of(%s, %s);", v(0), e.elemDesc(arg(0).Ty))
		e.assign(dst, fmt.Sprintf("ey_lazy_then(z_, EY_STEP_ZIP, 0, 0, %s, %s)", v(1), e.elemDesc(dst.Ty)))
		e.printf("ey_drop(z_); }")
	case "first":
		e.printf("eyb_iter_first(%s, %s, %s);", v(0), out, rt())
	case "find":
		e.printf("eyb_iter_find(%s, %s, %s, %s);", v(0), v(1), out, rt())
	case "any", "all":
		how := 0
		if in.Name == "all" {
			how = 1
		}
		e.assign(dst, fmt.Sprintf("eyb_iter_test(%s, %s, %d)", v(0), v(1), how))
	case "contains":
		e.assign(dst, fmt.Sprintf("eyb_iter_contains(%s, &%s)", v(0), v(1)))
	case "isEmpty":
		e.assign(dst, fmt.Sprintf("eyb_iter_is_empty(%s)", v(0)))
	case "size":
		e.seq(in, 0, func(c string) string { return e.setTo(dst, fmt.Sprintf("EY_SMALL(ey_coll_size(%s))", c)) })
	case "count":
		if len(in.Args) == 1 {
			e.seq(in, 0, func(c string) string { return e.setTo(dst, fmt.Sprintf("EY_SMALL(ey_coll_size(%s))", c)) })
			return true
		}
		return false
	default:
		return false
	}
	return true
}

// rangeFlags are a range's flags: inclusive, and which ends it leaves out.
func (e *fn) rangeFlags(start, end, incl *lir.Value) string {
	parts := []string{fmt.Sprintf("(%s ? EY_RANGE_INCLUSIVE : 0)", e.v(incl))}
	if e.absent(start) {
		parts = append(parts, "EY_RANGE_NO_START")
	}
	if e.absent(end) {
		parts = append(parts, "EY_RANGE_NO_END")
	}
	return strings.Join(parts, " | ")
}

// seq calls a collection function on argument i as a List or a Set: a Map
// becomes its entries, and a range of Ints its Ints, first. call gives the
// C statement of the call on the collection c.
func (e *fn) seq(in *lir.Builtin, i int, call func(c string) string) {
	c := in.Args[i]
	switch kind(c) {
	case "List", "Set":
		e.printf("%s", call(e.v(c)))
	case "Map":
		mt := rep(c.Ty).(*ir.TCon)
		entries := &ir.TCon{Name: "List", Args: []ir.Ty{&ir.TTuple{Elems: mt.Args}}}
		e.printf("{ ey_v s_ = eyb_map_list(%s, 2, %s);", e.v(c), e.g.desc(entries))
		e.printf("%s", call("s_"))
		e.printf("ey_drop(s_); }")
	case "Range":
		ints := &ir.TCon{Name: "List", Args: []ir.Ty{&ir.TCon{Name: "Int"}}}
		e.printf("{ ey_v s_ = eyb_range_list(%s, %s);", e.v(c), e.g.desc(ints))
		e.printf("%s", call("s_"))
		e.printf("ey_drop(s_); }")
	case "String", "Iter", "Bytes":
		// walked into a list of its elements first
		list := &ir.TCon{Name: "List", Args: []ir.Ty{lir.ElemOf(rep(c.Ty))}}
		e.printf("{ ey_v s_ = ey_lazy_list(%s, %s);", e.v(c), e.g.desc(list))
		e.printf("%s", call("s_"))
		e.printf("ey_drop(s_); }")
	default:
		e.unsupported(in.Name + " on " + c.Ty.String())
	}
}

// mapVariant is map on an Option or a Result: the function applied to
// what Some or Ok holds; an Err's error is copied.
func (e *fn) mapVariant(in *lir.Builtin) {
	x, f := in.Args[0], in.Args[1]
	c, _ := rep(in.Dst.Ty).(*ir.TCon)
	ok0 := lir.VariantFieldTy(c, c.Decl.Variants[0], 0)
	e.printf("if (%s) {", e.isVariant(x, 0))
	e.printf(" %s r_; ey_call(%s, &r_, (void *[]){&%s});", e.g.ctype(ok0), e.v(f), e.payload(x, 0, 0))
	e.printf(" ey_variant_make(%s, &%s, 0, (const void *[]){&r_});", e.g.desc(in.Dst.Ty), e.v(in.Dst))
	e.printf("} else {")
	if c.Name == "Result" {
		errT := lir.VariantFieldTy(c, c.Decl.Variants[1], 0)
		e.printf(" %s e_ = %s; %s", e.g.ctype(errT), e.payload(x, 1, 0), e.g.dupStmt(errT, "e_"))
		e.printf(" ey_variant_make(%s, &%s, 1, (const void *[]){&e_});", e.g.desc(in.Dst.Ty), e.v(in.Dst))
	} else {
		e.printf(" ey_none(%s, &%s);", e.g.desc(in.Dst.Ty), e.v(in.Dst))
	}
	e.printf("}")
}

// mapErr is mapErr on a Result: the function applied to its error, and Ok
// copied.
func (e *fn) mapErr(in *lir.Builtin) {
	x, f := in.Args[0], in.Args[1]
	c, _ := rep(in.Dst.Ty).(*ir.TCon)
	okT := lir.VariantFieldTy(c, c.Decl.Variants[0], 0)
	errT := lir.VariantFieldTy(c, c.Decl.Variants[1], 0)
	d := e.g.desc(in.Dst.Ty)
	e.printf("if (%s) {", e.isVariant(x, 0))
	e.printf(" %s o_ = %s; %s", e.g.ctype(okT), e.payload(x, 0, 0), e.g.dupStmt(okT, "o_"))
	e.printf(" ey_variant_make(%s, &%s, 0, (const void *[]){&o_});", d, e.v(in.Dst))
	e.printf("} else {")
	e.printf(" %s r_; ey_call(%s, &r_, (void *[]){&%s});", e.g.ctype(errT), e.v(f), e.payload(x, 1, 0))
	e.printf(" ey_variant_make(%s, &%s, 1, (const void *[]){&r_});", d, e.v(in.Dst))
	e.printf("}")
}

var fixedOps = map[string]string{
	"wrappingAdd": "EYF_WADD", "wrappingSub": "EYF_WSUB", "wrappingMul": "EYF_WMUL",
	"and": "EYF_AND", "or": "EYF_OR", "xor": "EYF_XOR", "not": "EYF_NOT",
	"shiftLeft": "EYF_SHL", "shiftRight": "EYF_SHR",
}

// fixed writes a method of the fixed-width integers on a receiver of the
// width given, and reports whether name is one.
func (e *fn) fixed(in *lir.Builtin, bits int, signed bool, a []*lir.Value) bool {
	b := "0"
	if len(a) > 1 && a[1] != nil {
		b = e.word(a[1])
	}
	if op, ok := fixedOps[in.Name]; ok {
		e.assign(in.Dst, e.fromWord(in.Dst.Ty, fmt.Sprintf("eyb_fixed(%s, %s, %d, %v, %s, %s)", e.at, op, bits, signed, e.word(a[0]), b)))
		return true
	}
	switch in.Name {
	case "checkedAdd", "checkedSub", "checkedMul":
		op := map[string]int{"checkedAdd": 0, "checkedSub": 1, "checkedMul": 2}[in.Name]
		e.printf("eyb_fixed_checked(%d, %d, %v, %s, %s, &%s, %s);", op, bits, signed, e.word(a[0]), b, e.v(in.Dst), e.g.desc(in.Dst.Ty))
		return true
	case "toInt":
		e.assign(in.Dst, fmt.Sprintf("eyb_fixed_to_int(%d, %v, %s)", bits, signed, e.word(a[0])))
		return true
	}
	return false
}

// runtime writes a call into the runtime: the host functions of fs and
// process, and functions as values. Entities, jobs, views, contracts and
// events are the Go runner's (7.6).
func (e *fn) runtime(x *lir.Runtime) {
	switch x.Op {
	case "extern.call":
		f, _ := x.Target.(*ir.Func)
		if f == nil || f.Host == "" {
			e.unsupported("calling foreign code (@library)")
			return
		}
		e.host(x, f)
	case "builtin.value":
		name, _ := x.Target.(string)
		e.builtinValue(x.Dst, name)
	case "variant.constructor":
		v, _ := x.Target.(*ir.Variant)
		e.constructorValue(x.Dst, v)
	case "derived.value":
		name, _ := x.Target.(string)
		e.derivedValue(x.Dst, name)
	default:
		what := map[string]string{
			"entity": "entities", "job": "jobs", "view": "views", "op": "contracts' operations",
			"event": "events", "run": "runs of jobs", "fakes": "fakes", "task": "tasks",
			"test": "`with` in tests", "module": "modules as values", "type": "types as values",
			"fn": "foreign functions as values", "derived": "derived functions as values",
		}[strings.SplitN(x.Op, ".", 2)[0]]
		if what == "" {
			what = x.Op
		}
		e.errorf("the C back end doesn't compile %s yet; `eye run` and `eye test` run this program", what)
	}
}

// host calls a host function of the standard modules fs and process.
func (e *fn) host(x *lir.Runtime, f *ir.Func) {
	args := map[string]*lir.Value{}
	next := 0
	for i, a := range x.Args {
		name := ""
		if i < len(x.Names) {
			name = x.Names[i]
		}
		if name == "" && next < len(f.Params) {
			name = f.Params[next].Name
			next++
		}
		args[name] = a
	}
	v := func(name string) string { return e.v(args[name]) }
	rt := e.g.desc(x.Dst.Ty)
	out := "&" + e.v(x.Dst)
	switch f.Host {
	case "fs.readText":
		e.printf("eyb_fs_read_text(%s, %s, %s);", v("path"), out, rt)
	case "fs.readBytes":
		e.printf("eyb_fs_read_bytes(%s, %s, %s);", v("path"), out, rt)
	case "fs.writeText", "fs.appendText":
		e.printf("eyb_fs_write_text(%s, %s, %v, %s, %s);", v("path"), v("text"), f.Host == "fs.appendText", out, rt)
	case "fs.writeBytes":
		e.printf("eyb_fs_write_text(%s, %s, false, %s, %s);", v("path"), v("bytes"), out, rt)
	case "fs.exists":
		e.assign(x.Dst, fmt.Sprintf("eyb_fs_exists(%s)", v("path")))
	case "fs.list":
		e.printf("eyb_fs_list(%s, %s, %s);", v("dir"), out, rt)
	case "fs.makeDir":
		e.printf("eyb_fs_make_dir(%s, %s, %s);", v("path"), out, rt)
	case "fs.remove":
		e.printf("eyb_fs_remove(%s, %s, %s);", v("path"), out, rt)
	case "process.run":
		// The defaults: no arguments, no input, the program's directory.
		str := &ir.TCon{Name: "String"}
		strs := &ir.TCon{Name: "List", Args: []ir.Ty{str}}
		optStrT := &ir.TCon{Name: "Option", Decl: e.g.prog.Prelude.Types["Option"], Args: []ir.Ty{str}}
		optStr := e.g.desc(optStrT)
		e.printf("{ ey_v l_ = %s; %s d_;", "ey_list_new("+e.g.desc(strs)+", 0, NULL)", e.g.ctype(optStrT))
		list, input, dir := "l_", e.fs.str("", "&ey_t_string"), "&d_"
		if a, ok := args["args"]; ok {
			list = e.v(a)
		}
		if a, ok := args["input"]; ok {
			input = e.v(a)
		}
		if a, ok := args["dir"]; ok {
			dir = "&" + e.v(a)
		} else {
			e.printf(" ey_none(%s, &d_);", optStr)
		}
		e.printf(" eyb_process_run(%s, %s, %s, %s, %s, %s, %s);", v("program"), list, input, dir, optStr, out, rt)
		e.printf(" ey_drop(l_); ey_drop_at(%s, &d_); }", optStr)
	default:
		e.unsupported("the host function " + f.Host)
	}
}

// wrapper adds a closure's code, which calls what body writes with the
// closure's arguments, a[i], as values of the types params gives, and
// gives the value body leaves in the result's variable.
func (e *fn) wrapper(key string, ft *ir.TFun, body func(sub *fn, args []*lir.Value, r *lir.Value)) string {
	if w, ok := e.g.wrapKeys[key]; ok {
		return w
	}
	w := fmt.Sprintf("ey_w%d", len(e.g.wrapKeys))
	e.g.wrapKeys[key] = w
	// The wrapper is a small function of its own, written by the same
	// rules as a call.
	sub := newFn(e.g, e.fs, e.f)
	sub.at, sub.pos = e.at, e.pos
	var args []*lir.Value
	var b strings.Builder
	for i, p := range ft.Params {
		pv := &lir.Value{ID: 1000000 + i, Ty: p}
		args = append(args, pv)
		fmt.Fprintf(&b, "\t%s v%d = *(%s *)a[%d];\n", e.g.ctype(p), pv.ID, e.g.ctype(p), i)
	}
	r := &lir.Value{ID: 2000000, Ty: ft.Ret}
	body(sub, args, r)
	fmt.Fprintf(&b, "\t%s v%d = {0};\n\t%s\n\t*(%s *)out = v%d;\n", e.g.ctype(ft.Ret), r.ID, sub.pending.String(), e.g.ctype(ft.Ret), r.ID)
	e.g.wrappers = append(e.g.wrappers, fmt.Sprintf("void %s_tr(ey_fn *self, void *out, void *const *a) {\n\t(void)self;\n\t(void)a;\n%s}\n", w, b.String()))
	e.g.wrapProtos = append(e.g.wrapProtos, fmt.Sprintf("void %s_tr(ey_fn *self, void *out, void *const *a);\n", w))
	return w
}

// builtinValue is a closure that calls a built-in: `xs |> map(Byte.from)`.
func (e *fn) builtinValue(dst *lir.Value, name string) {
	ft, ok := dst.Ty.(*ir.TFun)
	if !ok {
		e.unsupported("the built-in " + name + " as a value")
		return
	}
	w := e.wrapper("builtin "+name+" "+lir.TypeKey(ft), ft, func(sub *fn, args []*lir.Value, r *lir.Value) {
		sub.builtin(&lir.Builtin{Dst: r, Name: name, Args: args, Names: make([]string, len(args))})
	})
	e.assign(dst, fmt.Sprintf("EY_V(ey_fn_new(%s, %s_tr))", e.g.desc(ft), w))
}

// derivedValue is a closure over an enum's derived function:
// `names |> map(Priority.parse)`.
func (e *fn) derivedValue(dst *lir.Value, name string) {
	ft, ok := dst.Ty.(*ir.TFun)
	if !ok {
		e.unsupported(name + " as a value")
		return
	}
	var method string
	switch {
	case strings.HasSuffix(name, ".parse") && len(ft.Params) == 1:
		method = "parse"
	case strings.HasSuffix(name, ".variants") && len(ft.Params) == 0:
		method = "variants"
	default:
		e.unsupported(name + " as a value")
		return
	}
	w := e.wrapper("derived "+name+" "+lir.TypeKey(ft), ft, func(sub *fn, args []*lir.Value, r *lir.Value) {
		sub.derived(&lir.Derived{Dst: r, Method: method, Args: args})
	})
	e.assign(dst, fmt.Sprintf("EY_V(ey_fn_new(%s, %s_tr))", e.g.desc(ft), w))
}

// constructorValue is a closure that makes a variant: `xs |> map(Some)`.
// It is lent its arguments, so it dups them for the value it makes.
func (e *fn) constructorValue(dst *lir.Value, v *ir.Variant) {
	ft, ok := dst.Ty.(*ir.TFun)
	if !ok || v == nil {
		e.unsupported("this constructor as a value")
		return
	}
	w := e.wrapper("variant "+v.Name+" "+lir.TypeKey(ft), ft, func(sub *fn, args []*lir.Value, r *lir.Value) {
		for _, a := range args {
			if st := e.g.dupStmt(a.Ty, sub.v(a)); st != "" {
				sub.printf("%s", st)
			}
		}
		sub.makeVariant(&lir.MakeVariant{Dst: r, Variant: v, Args: args})
	})
	e.assign(dst, fmt.Sprintf("EY_V(ey_fn_new(%s, %s_tr))", e.g.desc(ft), w))
}
