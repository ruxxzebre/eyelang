package cgen

import (
	"fmt"
	"sort"
	"strings"

	"eyelang/ir"
	"eyelang/lir"
)

// Values are laid out by type (7.13): each type's C representation comes
// from its layout in the low IR (lir.Layouts; compilation.md section 2,
// "Values laid out by type"). A word (an Int, a box, an object, or a
// variant type held in the spare values of one) is an ey_v; a scalar or an
// enum its C integer or double; a record, tuple or variant type of at most
// 64 bytes a C struct this file defines; a larger or recursive one a box,
// a word that points at an object whose payload this file defines too.

// objects are the built-in types the runtime still holds as its objects
// rather than as their layouts say.
var objects = map[string]bool{"String": true, "Bytes": true, "Key": true, "Zone": true, "Instant": true, "Range": true}

// layout is the layout of t's values.
func (g *gen) layout(t ir.Ty) *lir.Layout { return g.lay.Of(rep(t)) }

// counted reports whether values of t are counted: dup'ed and dropped.
func (g *gen) counted(t ir.Ty) bool {
	if t == nil {
		return false
	}
	return g.lay.Counted(rep(t))
}

// isWord reports whether values of t are one word, dup'ed and dropped as
// one.
func (g *gen) isWord(t ir.Ty) bool { return g.layout(t).Word() }

// ctype is the C type of t's values.
func (g *gen) ctype(t ir.Ty) string {
	if t == nil {
		return "uint8_t"
	}
	l := g.layout(t)
	switch {
	case l.Word():
		return "ey_v"
	case l.Repr == lir.ReprNone:
		return "uint8_t"
	case l.Repr == lir.ReprScalar || l.Repr == lir.ReprEnum:
		return l.Scalar
	case l.Repr == lir.ReprStruct:
		return g.structOf(l).name
	}
	return "ey_v"
}

// cstruct is a C struct the program's types need: an inline value, or a
// box's payload (of a record or tuple, or of one variant).
type cstruct struct {
	name    string
	l       *lir.Layout
	box     bool
	variant int // a box of a variant type: which variant
	deps    []*cstruct
	done    bool
}

// structOf is the struct of an inline value of layout l.
func (g *gen) structOf(l *lir.Layout) *cstruct {
	key := lir.TypeKey(l.Ty)
	if s, ok := g.structs[key]; ok {
		return s
	}
	s := &cstruct{name: fmt.Sprintf("ey_s%d", len(g.structList)), l: l, variant: -1}
	g.structs[key] = s
	g.structList = append(g.structList, s)
	return s
}

// boxOf is the struct of a box's object: of the record or tuple of type t,
// or of its variant k.
func (g *gen) boxOf(t ir.Ty, k int) *cstruct {
	l := g.layout(t)
	key := fmt.Sprintf("box %d %s", k, lir.TypeKey(l.Ty))
	if s, ok := g.structs[key]; ok {
		return s
	}
	name := fmt.Sprintf("ey_b%d", len(g.structList))
	if k >= 0 {
		name += fmt.Sprintf("_v%d", k)
	}
	s := &cstruct{name: name, l: l, box: true, variant: k}
	g.structs[key] = s
	g.structList = append(g.structList, s)
	return s
}

// member is one member of a C struct: its name, C type and offset.
type member struct {
	name, ctype string
	off         int
	ty          ir.Ty
}

func (g *gen) members(slots []lir.Slot) []member {
	ms := make([]member, len(slots))
	for i, s := range slots {
		ms[i] = member{name: fmt.Sprintf("f%d", i), ctype: g.ctype(s.L.Ty), off: s.Offset, ty: s.L.Ty}
	}
	sort.SliceStable(ms, func(a, b int) bool { return ms[a].off < ms[b].off })
	return ms
}

// dataful is the variant with fields of a variant type held in a niche.
func dataful(l *lir.Layout) int {
	for i, v := range l.Variants {
		if len(v.Fields) > 0 {
			return i
		}
	}
	return 0
}

// body writes s's members, which name the structs they hold in s.deps.
func (g *gen) body(s *cstruct) string {
	var b strings.Builder
	write := func(ms []member, indent string) {
		for _, m := range ms {
			fmt.Fprintf(&b, "%s%s %s;\n", indent, m.ctype, m.name)
			if strings.HasPrefix(m.ctype, "ey_s") {
				s.deps = append(s.deps, g.structs[lir.TypeKey(g.layout(m.ty).Ty)])
			}
		}
	}
	l := s.l
	if s.box {
		b.WriteString("\tey_obj h;\n")
		if s.variant >= 0 {
			write(g.members(l.Variants[s.variant].Fields), "\t")
		} else {
			write(g.members(l.Fields), "\t")
		}
		return b.String()
	}
	switch {
	case len(l.Variants) == 0:
		write(g.members(l.Fields), "\t")
	case l.Tag.Kind == lir.TagNone:
		write(g.members(l.Variants[0].Fields), "\t")
	case l.Tag.Kind == lir.TagNiche:
		write(g.members(l.Variants[dataful(l)].Fields), "\t")
	default: // a tag, then a union of the variants with fields
		fmt.Fprintf(&b, "\tuint%d_t tag;\n\tunion {\n", l.Tag.Size*8)
		for k, v := range l.Variants {
			if len(v.Fields) == 0 {
				continue
			}
			b.WriteString("\t\tstruct {\n")
			write(g.members(v.Fields), "\t\t\t")
			fmt.Fprintf(&b, "\t\t} v%d;\n", k)
		}
		b.WriteString("\t} u;\n")
	}
	return b.String()
}

// fieldPath is the C path from a struct of layout l to field i of variant
// k (or of the record or tuple, with k -1).
func fieldPath(l *lir.Layout, k, i int) string {
	switch {
	case k < 0 || l.Tag.Kind == lir.TagNone || l.Tag.Kind == lir.TagNiche || l.Tag.Kind == lir.TagBox:
		return fmt.Sprintf("f%d", i)
	}
	return fmt.Sprintf("u.v%d.f%d", k, i)
}

// structDefs writes the program's structs, each after those it holds, and
// the dup and drop of each counted inline one.
func (g *gen) structDefs() string {
	var defs, fns strings.Builder
	// Bodies first: writing one can name more structs.
	bodies := map[*cstruct]string{}
	for i := 0; i < len(g.structList); i++ {
		s := g.structList[i]
		bodies[s] = g.body(s)
	}
	for _, s := range g.structList {
		fmt.Fprintf(&defs, "typedef struct %s %s;\n", s.name, s.name)
	}
	var visit func(s *cstruct)
	visit = func(s *cstruct) {
		if s == nil || s.done {
			return
		}
		s.done = true
		for _, d := range s.deps {
			visit(d)
		}
		fmt.Fprintf(&defs, "struct %s {\n%s};\n", s.name, bodies[s])
		if !s.box && s.l.Counted {
			g.countFns(&fns, s)
		}
	}
	for _, s := range g.structList {
		visit(s)
	}
	return defs.String() + fns.String()
}

// countFns writes the dup and drop of an inline struct that holds counted
// values.
func (g *gen) countFns(b *strings.Builder, s *cstruct) {
	for _, op := range []string{"dup", "drop"} {
		fmt.Fprintf(b, "static inline void ey_%s_%s(%s *p) {\n", op, s.name, s.name)
		each := func(ms []member, path func(string) string, indent string) {
			for _, m := range ms {
				if st := g.countStmt(op, m.ty, "p->"+path(m.name)); st != "" {
					fmt.Fprintf(b, "%s%s\n", indent, st)
				}
			}
		}
		l := s.l
		plain := func(n string) string { return n }
		switch {
		case len(l.Variants) == 0:
			each(g.members(l.Fields), plain, "\t")
		case l.Tag.Kind == lir.TagNone:
			each(g.members(l.Variants[0].Fields), plain, "\t")
		case l.Tag.Kind == lir.TagNiche:
			fmt.Fprintf(b, "\tif (%s) {\n", nicheIs(l, dataful(l), "p"))
			each(g.members(l.Variants[dataful(l)].Fields), plain, "\t\t")
			b.WriteString("\t}\n")
		default:
			b.WriteString("\tswitch (p->tag) {\n")
			for k, v := range l.Variants {
				if len(v.Fields) == 0 {
					continue
				}
				fmt.Fprintf(b, "\tcase %d:\n", k)
				kk := k
				each(g.members(v.Fields), func(n string) string { return fmt.Sprintf("u.v%d.%s", kk, n) }, "\t\t")
				b.WriteString("\t\tbreak;\n")
			}
			b.WriteString("\t}\n")
		}
		b.WriteString("}\n")
	}
}

// countStmt is the C that dups or drops (op) the value of type t at the
// lvalue lv, or "" for one that isn't counted.
func (g *gen) countStmt(op string, t ir.Ty, lv string) string {
	l := g.layout(t)
	switch {
	case l.Word():
		return fmt.Sprintf("ey_%s(%s);", op, lv)
	case l.Repr == lir.ReprStruct && l.Counted:
		return fmt.Sprintf("ey_%s_%s(&%s);", op, g.structOf(l).name, lv)
	}
	return ""
}

func (g *gen) dropStmt(t ir.Ty, lv string) string { return g.countStmt("drop", t, lv) }
func (g *gen) dupStmt(t ir.Ty, lv string) string  { return g.countStmt("dup", t, lv) }

// nicheIs is the C test that the value at the struct pointer p, of a
// variant type held in a niche, is variant k.
func nicheIs(l *lir.Layout, k int, p string) string {
	at := fmt.Sprintf("EY_AT(uint%d_t, %s, %d)", l.Tag.Size*8, p, l.Tag.Offset)
	if len(l.Variants[k].Fields) == 0 {
		return fmt.Sprintf("%s == UINT64_C(%d)", at, l.Variants[k].Value)
	}
	var parts []string
	for _, v := range l.Variants {
		if len(v.Fields) == 0 {
			parts = append(parts, fmt.Sprintf("%s != UINT64_C(%d)", at, v.Value))
		}
	}
	if len(parts) == 0 {
		return "1"
	}
	return strings.Join(parts, " && ")
}

// ---------------------------------------------------------------- descriptors

var scalarDescs = map[string]string{
	"()": "&ey_t_unit", "Bool": "&ey_t_bool", "Float": "&ey_t_float", "Char": "&ey_t_char",
	"Byte": "&ey_t_byte", "Int32": "&ey_t_int32", "Int64": "&ey_t_int64", "UInt32": "&ey_t_uint32",
	"UInt64": "&ey_t_uint64", "Duration": "&ey_t_duration", "Size": "&ey_t_size", "Date": "&ey_t_date",
	"Time": "&ey_t_time", "Never": "&ey_t_never", "Int": "&ey_t_int", "String": "&ey_t_string",
	"Zone": "&ey_t_string", "Key": "&ey_t_string", "Bytes": "&ey_t_bytes", "Instant": "&ey_t_instant",
	"Iter": "&ey_t_iter",
}

// desc is the C expression of t's descriptor, writing one if needed.
func (g *gen) desc(t ir.Ty) string {
	t = rep(t)
	if n := scalarName(t); n != "" {
		if d, ok := scalarDescs[n]; ok {
			return d
		}
	}
	if t == nil {
		return "&ey_t_unit"
	}
	key := lir.TypeKey(t)
	if d, ok := g.descs[key]; ok {
		return "&" + d.name
	}
	d := &desc{name: fmt.Sprintf("ey_d%d", len(g.descs)), ty: t}
	g.descs[key] = d
	g.descQueue = append(g.descQueue, d)
	return "&" + d.name
}

// define writes a descriptor's definition, with the layout of its type.
func (g *gen) define(d *desc) string {
	var b strings.Builder
	list := func(suffix string, ts []ir.Ty) string {
		if len(ts) == 0 {
			return "NULL"
		}
		parts := make([]string, len(ts))
		for i, t := range ts {
			parts[i] = g.desc(t)
		}
		fmt.Fprintf(&b, "static const ey_type *const %s_%s[] = {%s};\n", d.name, suffix, strings.Join(parts, ", "))
		return d.name + "_" + suffix
	}
	names := func(suffix string, ns []string) string {
		if len(ns) == 0 {
			return "NULL"
		}
		parts := make([]string, len(ns))
		for i, n := range ns {
			parts[i] = cstring(n)
		}
		fmt.Fprintf(&b, "static const char *const %s_%s[] = {%s};\n", d.name, suffix, strings.Join(parts, ", "))
		return d.name + "_" + suffix
	}
	offsets := func(suffix string, offs []string) string {
		if len(offs) == 0 {
			return "NULL"
		}
		fmt.Fprintf(&b, "static const uint32_t %s_%s[] = {%s};\n", d.name, suffix, strings.Join(offs, ", "))
		return d.name + "_" + suffix
	}
	f := map[string]string{} // the descriptor's fields, by name
	l := g.layout(d.ty)
	ct := g.ctype(d.ty)
	f["size"], f["align"] = fmt.Sprintf("sizeof(%s)", ct), fmt.Sprintf("_Alignof(%s)", ct)
	switch {
	case l.Word():
		f["repr"] = "EY_R_WORD"
	case l.Repr == lir.ReprEnum:
		f["repr"] = "EY_R_ENUM"
	case l.Repr == lir.ReprStruct:
		f["repr"] = "EY_R_STRUCT"
		if l.Counted {
			s := g.structOf(l)
			for _, op := range []string{"dup", "drop"} {
				fmt.Fprintf(&b, "static void ey_%s_%s_v(void *p) { ey_%s_%s(p); }\n", op, s.name, op, s.name)
				f[op] = fmt.Sprintf("ey_%s_%s_v", op, s.name)
			}
		}
	default:
		f["repr"] = "EY_R_SCALAR"
	}
	// where field i of a record or tuple is
	fieldOffs := func(n int) []string {
		offs := make([]string, n)
		for i := range offs {
			if l.Repr == lir.ReprBox {
				offs[i] = fmt.Sprintf("offsetof(%s, f%d)", g.boxOf(d.ty, -1).name, i)
			} else {
				offs[i] = fmt.Sprintf("offsetof(%s, f%d)", ct, i)
			}
		}
		return offs
	}
	if l.Repr == lir.ReprBox {
		f["boxed"] = "1"
	}
	switch t := d.ty.(type) {
	case *ir.TCon:
		switch {
		case t.Decl != nil && len(t.Decl.Variants) > 0:
			f["kind"], f["n"], f["name"] = "EY_VARIANT", fmt.Sprint(len(t.Decl.Variants)), cstring(t.Decl.Name)
			f["tag"] = map[lir.TagKind]string{lir.TagNone: "EY_TAG_NONE", lir.TagField: "EY_TAG_FIELD", lir.TagNiche: "EY_TAG_NICHE", lir.TagBox: "EY_TAG_BOX"}[l.Tag.Kind]
			if l.Repr == lir.ReprEnum {
				f["tag"] = "EY_TAG_FIELD"
			}
			f["tagsize"], f["tagoff"] = fmt.Sprint(l.Tag.Size), fmt.Sprint(l.Tag.Offset)
			var vs []string
			for i, v := range t.Decl.Variants {
				var fts []ir.Ty
				var fns, offs []string
				for j, fl := range v.Fields {
					fts = append(fts, lir.VariantFieldTy(t, v, j))
					fns = append(fns, fl.Name)
					switch {
					case l.Repr == lir.ReprBox:
						offs = append(offs, fmt.Sprintf("offsetof(%s, f%d)", g.boxOf(d.ty, i).name, j))
					case l.Word():
						offs = append(offs, "0")
					default:
						offs = append(offs, fmt.Sprintf("offsetof(%s, %s)", ct, fieldPath(l, i, j)))
					}
				}
				objsize := "0"
				if l.Repr == lir.ReprBox && len(v.Fields) > 0 {
					objsize = fmt.Sprintf("sizeof(%s)", g.boxOf(d.ty, i).name)
				}
				fl := list(fmt.Sprintf("v%d_f", i), fts)
				nl := names(fmt.Sprintf("v%d_n", i), fns)
				ol := offsets(fmt.Sprintf("v%d_o", i), offs)
				vs = append(vs, fmt.Sprintf("{%s, %d, %s, %s, %s, UINT64_C(%d), %s}", cstring(v.Name), len(v.Fields), fl, nl, ol, l.Variants[i].Value, objsize))
			}
			fmt.Fprintf(&b, "static const ey_variant %s_vs[] = {%s};\n", d.name, strings.Join(vs, ", "))
			f["variants"] = d.name + "_vs"
		case t.Decl != nil:
			fns, fts, ok := lir.RecordFields(t)
			if !ok {
				g.errorf(t.Decl.Pos, "the C back end can't lay out the type %s", t)
			}
			f["kind"], f["n"], f["name"] = "EY_RECORD", fmt.Sprint(len(fns)), cstring(t.Decl.Name)
			f["args"], f["names"], f["offs"] = list("f", fts), names("n", fns), offsets("o", fieldOffs(len(fns)))
			if l.Repr == lir.ReprBox {
				f["objsize"] = fmt.Sprintf("sizeof(%s)", g.boxOf(d.ty, -1).name)
			}
		case t.Name == "List" || t.Name == "Set" || t.Name == "Map" || t.Name == "Range":
			f["kind"], f["n"] = "EY_"+strings.ToUpper(t.Name), fmt.Sprint(len(t.Args))
			f["args"] = list("a", t.Args)
		default:
			g.errorf(ir.Pos{}, "the C back end doesn't support values of type %s yet", t)
		}
	case *ir.TTuple:
		f["kind"], f["n"] = "EY_TUPLE", fmt.Sprint(len(t.Elems))
		f["args"], f["offs"] = list("a", t.Elems), offsets("o", fieldOffs(len(t.Elems)))
		if l.Repr == lir.ReprBox {
			f["objsize"] = fmt.Sprintf("sizeof(%s)", g.boxOf(d.ty, -1).name)
		}
	case *ir.TFun:
		// a closure over a built-in or a constructor captures nothing
		f["kind"], f["name"], f["objsize"] = "EY_FN", `"fn"`, "sizeof(ey_fn)"
	case *ir.TDyn:
		f["kind"], f["name"] = "EY_DYN", cstring(t.Trait.Name)
	default:
		g.errorf(ir.Pos{}, "the C back end doesn't support values of type %s yet", d.ty)
	}
	fmt.Fprintf(&b, "const ey_type %s = {%s};\n", d.name, initializer(f))
	return b.String()
}

// initializer writes a descriptor's designated initializer from its fields.
func initializer(f map[string]string) string {
	var parts []string
	for _, k := range []string{"kind", "n", "size", "align", "repr", "boxed", "tag", "tagsize", "tagoff", "objsize", "name", "args", "names", "offs", "variants", "drop", "dup"} {
		if v, ok := f[k]; ok {
			parts = append(parts, "."+k+" = "+v)
		}
	}
	return strings.Join(parts, ", ")
}

// closureStruct is the struct of the closures of f: the closure's header
// and its captures, or "" for one that captures nothing.
func (g *gen) closureStruct(f *lir.Func) string {
	if captures(f) == 0 {
		return ""
	}
	return g.names[f] + "_cl"
}
