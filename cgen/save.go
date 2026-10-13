package cgen

import (
	"fmt"
	"strings"

	"eyelang/ir"
	"eyelang/lir"
)

// Saved values (ROADMAP 7.6, 7.9). Entity state, a job's arguments and
// steps, and every value the Go runner hands to compiled code or takes from
// it cross as bytes in the interpreter's encoding (interp/save.go), so that
// either side reads what the other wrote and a store holds the same bytes
// whoever wrote them. For each type that crosses, the back end writes an
// encoder and a decoder from the low IR's monomorphic type: eys_eN appends
// a value's bytes, eys_dN reads them back (runtime/save.c has what they
// share). A type's name for the runner is SavedName, and program.c lists
// the types the runner may ask for in ey_saved_types.
//
// Encoders follow the type, not the layout: a newtype, which the back end
// holds as its field, is still saved as its variant with one value, as the
// interpreter saves it, and a record by its declared type's name and each
// field's.

// saver is the encoder and decoder of one type.
type saver struct {
	n  int
	ty ir.Ty // aliases followed, newtypes kept
}

func (s *saver) enc() string { return fmt.Sprintf("eys_e%d", s.n) }
func (s *saver) dec() string { return fmt.Sprintf("eys_d%d", s.n) }

// SavedName is the name a saved type has in a program's ey_saved_types,
// which the runner finds its encoder and decoder by: lir.TypeKey's, after
// aliases, such as "List(main.Point)".
func SavedName(t ir.Ty) string { return lir.TypeKey(lir.Resolve(t)) }

// saver is t's encoder and decoder, which saveFuncs writes.
func (g *gen) saver(t ir.Ty) *saver {
	t = lir.Resolve(t)
	key := lir.TypeKey(t)
	if s, ok := g.savers[key]; ok {
		return s
	}
	s := &saver{n: len(g.saveList), ty: t}
	g.savers[key] = s
	g.saveList = append(g.saveList, s)
	return s
}

// savedC writes every saver's encoder and decoder, and the table of the
// types the runner may ask for: those Options.Saved names and the probes'.
func (g *gen) savedC() string {
	var table []*saver
	for _, t := range g.opts.Saved {
		table = append(table, g.saver(t))
	}
	for _, p := range g.probes {
		table = append(table, g.saver(g.resultTy(p.fn)))
	}
	var b strings.Builder
	for i := 0; i < len(g.saveList); i++ { // writing one may ask for more
		b.WriteString(g.saveFunc(g.saveList[i]))
	}
	b.WriteString("const ey_saved ey_saved_types[] = {\n")
	for _, s := range table {
		fmt.Fprintf(&b, "\t{%s, %s, %s, %s},\n", cstring(SavedName(s.ty)), g.desc(s.ty), s.enc(), s.dec())
	}
	if len(table) == 0 {
		b.WriteString("\t{NULL, NULL, NULL, NULL},\n")
	}
	fmt.Fprintf(&b, "};\nconst int ey_nsaved = %d;\n\n", len(table))
	return b.String()
}

// saveProtos declares the encoders and decoders, which host calls use from
// every module's file.
func (g *gen) saveProtos() string {
	var b strings.Builder
	for _, s := range g.saveList {
		fmt.Fprintf(&b, "void %s(ey_buf *b, const void *v);\nbool %s(ey_rd *r, void *out);\n", s.enc(), s.dec())
	}
	return b.String()
}

// typeName and variantName are what the interpreter saves a record's type
// and a variant as (interp/value.go).
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

// saveFunc writes s's encoder and decoder.
func (g *gen) saveFunc(s *saver) string {
	var enc, dec strings.Builder
	t := s.ty
	ct := g.ctype(t)
	in := func(w *strings.Builder, format string, args ...any) {
		fmt.Fprintf(w, "\t"+format+"\n", args...)
	}
	cant := func(why string) {
		g.errorf(ir.Pos{}, "the C back end can't save values of type %s yet: %s", t, why)
	}
	// scalar writes a scalar kept as a C integer: its tag and value.
	scalar := func(tag string) {
		in(&enc, "eys_tagged(b, '%s', (int64_t)*(const %s *)v);", tag, ct)
		in(&dec, "int64_t n;")
		in(&dec, "if (!eys_read_tagged(r, '%s', &n)) return false;", tag)
		in(&dec, "*(%s *)out = (%s)n;", ct, ct)
		in(&dec, "return true;")
	}
	switch x := t.(type) {
	case *ir.TCon:
		switch {
		case x.Entity != nil:
			cant("a Ref is saved by the runner (ROADMAP 7.6)")
		case x.Decl != nil && x.Decl.Newtype():
			v := x.Decl.Variants[0]
			f := g.saver(lir.VariantFieldTy(t, v, 0))
			name := variantName(v)
			in(&enc, "ey_buf_char(b, 'V');")
			in(&enc, "eys_text(b, %s, %d);", cstring(name), len(name))
			in(&enc, "eys_uvarint(b, 1);")
			in(&enc, "%s(b, v);", f.enc())
			in(&dec, "const char *name;\n\tint64_t len;\n\tuint64_t n;")
			in(&dec, "if (!eys_read_variant(r, &name, &len, &n)) return false;")
			in(&dec, "if (!eys_text_is(name, len, %s) || n != 1) return eys_fail(r, \"the saved value isn't of the type it is read as\");", cstring(name))
			in(&dec, "return %s(r, out);", f.dec())
		case x.Decl != nil && len(x.Decl.Variants) > 0:
			g.saveVariants(&enc, &dec, x)
		case x.Decl != nil:
			g.saveRecord(&enc, &dec, x, cant)
		default:
			g.saveBuiltin(&enc, &dec, x, ct, scalar, cant)
		}
	case *ir.TTuple:
		d := g.desc(t)
		in(&enc, "ey_buf_char(b, 'P');")
		in(&enc, "eys_uvarint(b, %d);", len(x.Elems))
		in(&dec, "uint64_t n;")
		in(&dec, "if (!eys_tag(r, 'P') || !eys_read_uvarint(r, &n)) return false;")
		in(&dec, "if (n != %d) return eys_fail(r, \"the saved tuple has another size\");", len(x.Elems))
		var fields []string
		for i, e := range x.Elems {
			f := g.saver(e)
			in(&enc, "%s(b, ey_field(%s, v, %d));", f.enc(), d, i)
			in(&dec, "%s f%d = {0};", g.ctype(e), i)
			in(&dec, "if (!%s(r, &f%d)) return false;", f.dec(), i)
			fields = append(fields, fmt.Sprintf("&f%d", i))
		}
		in(&dec, "ey_tuple_make(%s, out, (const void *[]){%s});", d, strings.Join(fields, ", "))
		in(&dec, "return true;")
	default:
		cant("only data is saved")
	}
	if dec.Len() == 0 {
		in(&dec, "return eys_fail(r, \"the saved value isn't of the type it is read as\");")
	}
	return fmt.Sprintf("/* %s */\nvoid %s(ey_buf *b, const void *v) {\n\t(void)b;\n\t(void)v;\n%s}\nbool %s(ey_rd *r, void *out) {\n\t(void)out;\n%s}\n\n",
		strings.ReplaceAll(lir.TypeKey(t), "*/", "* /"), s.enc(), enc.String(), s.dec(), dec.String())
}

// saveBuiltin writes the encoder and decoder of a built-in type.
func (g *gen) saveBuiltin(enc, dec *strings.Builder, x *ir.TCon, ct string, scalar func(string), cant func(string)) {
	in := func(w *strings.Builder, format string, args ...any) {
		fmt.Fprintf(w, "\t"+format+"\n", args...)
	}
	word := func(write, read string) {
		in(enc, "%s(b, *(const ey_v *)v);", write)
		in(dec, "return %s(r, (ey_v *)out);", read)
	}
	arg := func(i int) *saver { return g.saver(x.Args[i]) }
	switch x.Name {
	case "()":
		in(enc, "ey_buf_char(b, 'u');")
		in(dec, "return eys_tag(r, 'u');")
	case "Bool":
		in(enc, "ey_buf_char(b, *(const bool *)v ? 'T' : 'F');")
		in(dec, "return eys_read_bool(r, (bool *)out);")
	case "Int":
		word("eys_int", "eys_read_int")
	case "Byte", "Int32", "Int64", "UInt32", "UInt64":
		d := g.desc(x)
		in(enc, "eys_fixed(b, %s, v);", d)
		in(dec, "return eys_read_fixed(r, %s, out);", d)
	case "Float":
		in(enc, "eys_float(b, *(const double *)v);")
		in(dec, "return eys_read_float(r, (double *)out);")
	case "Char":
		scalar("c")
	case "Duration":
		scalar("d")
	case "Size":
		scalar("z")
	case "Date", "Time":
		write, read := "eys_date", "eys_read_date"
		if x.Name == "Time" {
			write, read = "eys_time", "eys_read_time"
		}
		in(enc, "%s(b, (int64_t)*(const %s *)v);", write, ct)
		in(dec, "int64_t n;")
		in(dec, "if (!%s(r, &n)) return false;", read)
		in(dec, "*(%s *)out = (%s)n;", ct, ct)
		in(dec, "return true;")
	case "String", "Key", "Zone":
		word("eys_string", "eys_read_string")
	case "Bytes":
		word("eys_bytes", "eys_read_bytes")
	case "Instant":
		word("eys_instant", "eys_read_instant")
	case "List", "Set":
		e := arg(0)
		w := strings.ToLower(x.Name)
		in(enc, "eys_%s(b, *(const ey_v *)v, %s);", w, e.enc())
		in(dec, "return eys_read_%s(r, %s, %s, (ey_v *)out);", w, g.desc(x), e.dec())
	case "Map":
		k, val := arg(0), arg(1)
		in(enc, "eys_map(b, *(const ey_v *)v, %s, %s);", k.enc(), val.enc())
		in(dec, "return eys_read_map(r, %s, %s, %s, (ey_v *)out);", g.desc(x), k.dec(), val.dec())
	case "Range":
		e := arg(0)
		in(enc, "eys_range(b, *(const ey_v *)v, %s, %s);", g.desc(x.Args[0]), e.enc())
		in(dec, "return eys_read_range(r, %s, %s, (ey_v *)out);", g.desc(x), e.dec())
	case "NonZero":
		// an Int that isn't zero, which the interpreter holds as the Int
		e := arg(0)
		in(enc, "%s(b, v);", e.enc())
		in(dec, "return %s(r, out);", e.dec())
	case "Never":
		in(enc, "ey_fault(\"<saving>\", \"a value of type Never can't be saved\");")
	default:
		cant(x.Name + " isn't saved by compiled code")
	}
}

// saveVariants writes the encoder and decoder of a variant type: its
// variant's name, how many values follow, and each.
func (g *gen) saveVariants(enc, dec *strings.Builder, x *ir.TCon) {
	in := func(w *strings.Builder, format string, args ...any) {
		fmt.Fprintf(w, "\t"+format+"\n", args...)
	}
	d := g.desc(x)
	in(enc, "switch (ey_variant_of(%s, v)) {", d)
	in(dec, "const char *name;\n\tint64_t len;\n\tuint64_t n;")
	in(dec, "if (!eys_read_variant(r, &name, &len, &n)) return false;")
	for k, v := range x.Decl.Variants {
		name := variantName(v)
		in(enc, "case %d:", k)
		in(enc, "\tey_buf_char(b, 'V');")
		in(enc, "\teys_text(b, %s, %d);", cstring(name), len(name))
		in(enc, "\teys_uvarint(b, %d);", len(v.Fields))
		in(dec, "if (eys_text_is(name, len, %s)) {", cstring(name))
		in(dec, "\tif (n != %d) return eys_fail(r, \"the saved variant has another number of values\");", len(v.Fields))
		var fields []string
		for j := range v.Fields {
			ft := lir.VariantFieldTy(x, v, j)
			f := g.saver(ft)
			in(enc, "\t%s(b, ey_variant_field(%s, v, %d, %d));", f.enc(), d, k, j)
			in(dec, "\t%s f%d = {0};", g.ctype(ft), j)
			in(dec, "\tif (!%s(r, &f%d)) return false;", f.dec(), j)
			fields = append(fields, fmt.Sprintf("&f%d", j))
		}
		in(enc, "\tbreak;")
		list := "NULL"
		if len(fields) > 0 {
			list = "(const void *[]){" + strings.Join(fields, ", ") + "}"
		}
		in(dec, "\tey_variant_make(%s, out, %d, %s);", d, k, list)
		in(dec, "\treturn true;")
		in(dec, "}")
	}
	in(enc, "}")
	in(dec, "return eys_fail(r, \"the saved value names a variant its type doesn't have\");")
}

// saveRecord writes the encoder and decoder of a record type: its type's
// name, and each field's name and value, in the order the type declares
// them, as the interpreter keeps a record's fields.
func (g *gen) saveRecord(enc, dec *strings.Builder, x *ir.TCon, cant func(string)) {
	in := func(w *strings.Builder, format string, args ...any) {
		fmt.Fprintf(w, "\t"+format+"\n", args...)
	}
	names, types, ok := lir.RecordFields(x)
	if !ok {
		cant("it is neither a record nor a variant type")
		return
	}
	d := g.desc(x)
	tn := typeName(x.Decl)
	in(enc, "ey_buf_char(b, 'N');")
	in(enc, "eys_text(b, %s, %d);", cstring(tn), len(tn))
	in(enc, "eys_uvarint(b, %d);", len(names))
	in(dec, "uint64_t n;")
	in(dec, "if (!eys_read_record(r, &n)) return false;")
	in(dec, "bool got[%d] = {0};", max(len(names), 1))
	var fields []string
	for i, ft := range types {
		in(dec, "%s f%d = {0};", g.ctype(ft), i)
		fields = append(fields, fmt.Sprintf("&f%d", i))
	}
	in(dec, "for (uint64_t i = 0; i < n; i++) {")
	in(dec, "\tconst char *name;\n\t\tint64_t len;")
	in(dec, "\tif (!eys_read_text(r, &name, &len)) return false;")
	for i, name := range names {
		f := g.saver(types[i])
		in(enc, "eys_text(b, %s, %d);", cstring(name), len(name))
		in(enc, "%s(b, ey_field(%s, v, %d));", f.enc(), d, i)
		in(dec, "\tif (eys_text_is(name, len, %s)) {", cstring(name))
		in(dec, "\t\tif (got[%d]) return eys_fail(r, \"the saved record has a field twice\");", i)
		in(dec, "\t\tif (!%s(r, &f%d)) return false;", f.dec(), i)
		in(dec, "\t\tgot[%d] = true;\n\t\t\tcontinue;\n\t\t}", i)
	}
	in(dec, "\treturn eys_fail(r, \"the saved record has a field its type doesn't\");")
	in(dec, "}")
	for i := range names {
		in(dec, "if (!got[%d]) return eys_fail(r, \"the saved record lacks a field its type has\");", i)
	}
	list := "NULL"
	if len(fields) > 0 {
		list = "(const void *[]){" + strings.Join(fields, ", ") + "}"
	}
	in(dec, "ey_tuple_make(%s, out, %s);", d, list)
	in(dec, "return true;")
}

// probeRoot is a module-level let a binary that checks saving writes and
// reads (Options.SaveCheck).
type probeRoot struct {
	name string
	fn   *lir.Func
}

// probeEntry writes the main of a binary that checks saving.
func (g *gen) probeEntry() string {
	var b strings.Builder
	var rows []string
	for i, p := range g.probes {
		t := g.resultTy(p.fn)
		ct := g.ctype(t)
		fmt.Fprintf(&b, "static const void *ey_probe%d(void) {\n\tstatic %s v;\n\tv = %s_g();\n\treturn &v;\n}\n", i, ct, g.names[p.fn])
		rows = append(rows, fmt.Sprintf("\t{%s, %d, ey_probe%d},\n", cstring(p.name), len(g.opts.Saved)+i, i))
	}
	b.WriteString("static const ey_probe ey_probes[] = {\n")
	b.WriteString(strings.Join(rows, ""))
	if len(rows) == 0 {
		b.WriteString("\t{NULL, 0, NULL},\n")
	}
	b.WriteString("};\n")
	fmt.Fprintf(&b, "static void ey_run_probes(void) { ey_save_check(ey_probes, %d); }\n", len(g.probes))
	fmt.Fprintf(&b, "static const ey_program ey_prog = {NULL, 0, ey_run_probes, ey_zone_list, %d};\n", g.nzones)
	return b.String()
}
