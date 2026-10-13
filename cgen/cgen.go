// Package cgen is the C back end (ROADMAP 7.4): it prints a program's low IR
// (package lir) as C11, one file per module, with #line directives so that
// debuggers and crash reports point at .eye lines, and builds it with the
// zig the repository pins (build.go). The core runtime the C calls is
// written in C too (runtime/), and so is every value's layout: see
// runtime/eye.h and docs/internals/compilation.md, "The C back end as
// built".
//
// It compiles the core language: functions, records, variants, match,
// loops, mutation, closures, strings, collections, lazy Iters, Bytes and
// the fixed-width integers, traits and dyn, Result and Option, and test
// blocks.
// Entities, jobs, views and contracts are hosted by the Go runner (7.6);
// a program that uses them gets an error that says so.
package cgen

import (
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"eyelang/ir"
	"eyelang/lir"
)

// Options says what to generate.
type Options struct {
	// Tests makes a binary that runs the program's own test blocks, as
	// `eye test` does, instead of its main.
	Tests bool
}

// File is one generated C file.
type File struct {
	Name string
	Src  []byte
}

// Error is a part of the program the back end can't compile, at its
// position.
type Error struct {
	Pos ir.Pos
	Msg string
}

func (e *Error) Error() string {
	if e.Pos.File == nil {
		return e.Msg
	}
	return e.Pos.String() + ": " + e.Msg
}

// gen writes the C of one program.
type gen struct {
	prog  *ir.Program
	low   *lir.Program
	opts  Options
	funcs []*lir.Func // the instances reached from the roots, in order
	reach map[*lir.Func]bool
	names map[*lir.Func]string
	// descriptors of types, by lir.TypeKey, in the order they were named
	descs      map[string]*desc
	descQueue  []*desc
	closDesc   map[*lir.Func]string
	dynTab     map[*lir.DynTable]string
	tramps     map[*lir.Func]bool // functions called as closures
	methods    map[*lir.Func]bool // functions called through a dyn table
	wrappers   []string           // the code of closures over built-ins and constructors
	wrapProtos []string
	wrapKeys   map[string]string
	zones      map[string]bool // strings the program writes, which may name zones
	nzones     int
	tests      []testRoot
	main       *lir.Func
	errs       []error
	// how values are laid out (types.go), and the C structs that holds
	lay        *lir.Layouts
	structs    map[string]*cstruct
	structList []*cstruct
	// the impls of Show the derived Show calls, by lir.TypeKey (shows.go)
	shown map[string]*lir.Func
	// the functions that can call themselves, which count their calls (depth.go)
	recursive map[*lir.Func]bool
}

type testRoot struct {
	name string
	fn   *lir.Func
}

// desc is a type's descriptor in the C: its name, and the type it
// describes.
type desc struct {
	name string
	ty   ir.Ty
	show *lir.Func // the impl of Show of a newtype, whose descriptor is its field's with this
}

// Generate writes the C of a checked program from its low IR: one file per
// module, program.h, which every file includes, and program.c, with the
// type descriptors, the time zones it names and its entry point. The
// runtime's files (Runtime) complete the program.
func Generate(prog *ir.Program, low *lir.Program, opts Options) ([]File, error) {
	g := &gen{
		prog: prog, low: low, opts: opts,
		reach: map[*lir.Func]bool{}, names: map[*lir.Func]string{},
		descs: map[string]*desc{}, closDesc: map[*lir.Func]string{}, dynTab: map[*lir.DynTable]string{},
		tramps: map[*lir.Func]bool{}, methods: map[*lir.Func]bool{}, wrapKeys: map[string]string{},
		zones: map[string]bool{}, structs: map[string]*cstruct{},
	}
	g.unsupportedDecls()
	if len(g.errs) > 0 {
		return nil, joinErrors(g.errs)
	}
	// Values are laid out by type (7.13), but for the built-in types the
	// runtime still holds as objects; the low IR says where they are
	// dup'ed and dropped (7.5).
	if low.Layouts == nil {
		low.Layouts = lir.NewLayouts()
		low.Layouts.Objects = objects
	}
	g.lay = low.Layouts
	lir.Count(low)
	g.roots()
	if len(g.errs) > 0 {
		return nil, joinErrors(g.errs)
	}
	g.walk()
	g.findRecursive()
	for i, f := range g.funcs {
		g.names[f] = fmt.Sprintf("ey_f%d_%s", i, ident(f.Name, 40))
	}
	files := g.modules()
	if len(g.errs) > 0 {
		return nil, joinErrors(g.errs)
	}
	program := g.programC()
	header := g.programH()
	out := []File{{Name: "program.h", Src: []byte(header)}, {Name: "program.c", Src: []byte(program)}}
	return append(out, files...), nil
}

// Errors is every error of a program the back end can't compile.
type Errors []error

func (es Errors) Error() string {
	parts := make([]string, len(es))
	for i, e := range es {
		parts[i] = e.Error()
	}
	return strings.Join(parts, "\n")
}

func joinErrors(errs []error) error {
	if len(errs) > 10 {
		errs = append(errs[:10:10], fmt.Errorf("and %d more", len(errs)-10))
	}
	return Errors(errs)
}

func (g *gen) errorf(at ir.Pos, format string, args ...any) {
	g.errs = append(g.errs, &Error{Pos: at, Msg: fmt.Sprintf(format, args...)})
}

// own reports whether a module is the program's own, not the prelude, a
// standard module or the store.
func (g *gen) own(m *ir.Module) bool { return m != nil && (m == g.prog.Root || m.Dir != "") }

// unsupportedDecls reports the program's entities, jobs, views and
// contracts: the Go runner hosts them (7.6), not compiled code.
func (g *gen) unsupportedDecls() {
	found := map[string]bool{}
	var first ir.Pos
	note := func(kind string, at ir.Pos) {
		found[kind] = true
		if first.File == nil || (at.File == first.File && at.Offset < first.Offset) {
			first = at
		}
	}
	for _, m := range g.prog.Modules {
		if !g.own(m) {
			continue
		}
		for _, e := range m.Entities {
			note("entities", e.Pos)
		}
		for _, j := range m.Jobs {
			note("jobs", j.Pos)
		}
		for _, v := range m.Views {
			note("views", v.Pos)
		}
		for range m.Contracts {
			note("contracts", ir.Pos{})
		}
	}
	var kinds []string
	for _, k := range []string{"entities", "jobs", "views", "contracts"} {
		if found[k] {
			kinds = append(kinds, k)
		}
	}
	if len(kinds) == 0 {
		return
	}
	list := kinds[0]
	if n := len(kinds); n > 1 {
		list = strings.Join(kinds[:n-1], ", ") + " and " + kinds[n-1]
	}
	g.errorf(first, "the program declares %s, which the C back end doesn't compile yet (ROADMAP 7.6 hosts them in the Go runner); `eye run` and `eye test` run this program", list)
}

// roots finds the instances the binary starts from: the own modules' tests,
// in the order `eye test` runs them, or main.
func (g *gen) roots() {
	bySrc := map[*ir.Func]*lir.Func{}
	for _, f := range g.low.Funcs {
		if f.Kind == lir.KindTest || (f.Kind == lir.KindFunc && bySrc[f.Src] == nil) {
			bySrc[f.Src] = f
		}
	}
	if g.opts.Tests {
		for _, m := range g.prog.Modules {
			if !g.own(m) {
				continue
			}
			for _, t := range m.Tests {
				f := bySrc[t.Func]
				if f == nil {
					g.errorf(t.Pos, "test %q wasn't lowered", t.Name)
					continue
				}
				name := t.Name
				if m != g.prog.Root {
					name = m.Path + ": " + name
				}
				g.tests = append(g.tests, testRoot{name, f})
			}
		}
		return
	}
	m := g.prog.Root.Main
	if m == nil {
		g.errorf(ir.Pos{}, "the program has no main function: add `pub fn main(args: List(String))`, or build its tests with --test")
		return
	}
	g.main = bySrc[m]
	if g.main == nil {
		g.errorf(m.Pos, "main wasn't lowered")
	}
}

// walk finds every instance the roots reach.
func (g *gen) walk() {
	var queue []*lir.Func
	add := func(f *lir.Func) {
		if f != nil && !g.reach[f] {
			g.reach[f] = true
			queue = append(queue, f)
		}
	}
	for _, t := range g.tests {
		add(t.fn)
	}
	add(g.main)
	add(g.low.MainErrShow)
	if c, ok := rep(resultOf(g.main)).(*ir.TCon); ok && g.low.MainErrShow == nil && c.Name == "Result" && len(c.Args) == 2 {
		g.addShows(c.Args[1], add)
	}
	for len(queue) > 0 {
		f := queue[0]
		queue = queue[1:]
		g.funcs = append(g.funcs, f)
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				switch x := in.(type) {
				case *lir.Call:
					add(x.Fn)
				case *lir.MakeClosure:
					add(x.Fn)
					g.tramps[x.Fn] = true
				case *lir.Global:
					add(x.Init)
				case *lir.MakeDyn:
					for _, m := range x.Table.Methods {
						add(m)
						g.methods[m] = true
					}
				case *lir.Runtime:
					if fn, ok := x.Target.(*lir.Func); ok {
						add(fn)
					}
				case *lir.Derived:
					if x.Method == "show" && len(x.Args) == 1 {
						g.addShows(x.Args[0].Ty, add)
					}
				case *lir.Builtin:
					if x.Name == "expect" && len(x.Args) == 4 {
						g.addShows(x.Args[3].Ty, add)
					}
				}
			}
		}
	}
}

// ---------------------------------------------------------------- names

// ident makes s part of a C identifier.
func ident(s string, max int) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "_") {
				b.WriteByte('_')
			}
		}
		if b.Len() >= max {
			break
		}
	}
	return strings.TrimSuffix(b.String(), "_")
}

// cstring writes s as a C string literal.
func cstring(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '?':
			b.WriteString(`\?`) // no trigraphs
		case c >= 0x20 && c < 0x7f:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "\\%03o", c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func posString(p ir.Pos) string {
	if p.File == nil {
		return "<builtin>"
	}
	return p.String()
}

// ---------------------------------------------------------------- types

// rep is how a type is represented: the built-in type's name its values
// are, after aliases and newtypes.
func rep(t ir.Ty) ir.Ty {
	for i := 0; i < 50; i++ {
		t = lir.Resolve(t)
		c, ok := t.(*ir.TCon)
		if !ok {
			return t
		}
		if c.Decl != nil && c.Decl.Newtype() {
			t = lir.VariantFieldTy(t, c.Decl.Variants[0], 0)
			continue
		}
		if c.Decl == nil && c.Name == "NonZero" && len(c.Args) == 1 {
			t = c.Args[0]
			continue
		}
		return t
	}
	return t
}

// scalarName is the name of the built-in type t is represented as, or ""
// for a declared or composite one.
func scalarName(t ir.Ty) string {
	switch c := rep(t).(type) {
	case *ir.TCon:
		if c.Decl == nil && c.Entity == nil {
			return c.Name
		}
	case ir.TNever:
		return "Never"
	}
	return ""
}

// widths are the fixed-width integer types: bits, and whether signed.
var widths = map[string]struct {
	bits   int
	signed bool
}{
	"Byte": {8, false}, "Int32": {32, true}, "Int64": {64, true}, "UInt32": {32, false}, "UInt64": {64, false},
}

// closureDesc is the descriptor of the closures of f: their captures'
// types, which freeing one needs.
func (g *gen) closureDesc(f *lir.Func) string {
	if d, ok := g.closDesc[f]; ok {
		return "&" + d
	}
	name := fmt.Sprintf("ey_c%d", len(g.closDesc))
	g.closDesc[f] = name
	return "&" + name
}

// captures is how many of f's parameters are captures.
func captures(f *lir.Func) int {
	if f.Kind == lir.KindLambda && f.Src != nil {
		return len(f.Src.Captures)
	}
	return 0
}

// ---------------------------------------------------------------- the program's files

func moduleFile(m *ir.Module) string {
	if m == nil || m.Path == "" {
		return "m_main.c"
	}
	return "m_" + ident(m.Path, 60) + ".c"
}

// modules writes each module's file.
func (g *gen) modules() []File {
	byFile := map[string][]*lir.Func{}
	var order []string
	for _, f := range g.funcs {
		var m *ir.Module
		if f.Src != nil {
			m = f.Src.Module
		}
		name := moduleFile(m)
		if _, ok := byFile[name]; !ok {
			order = append(order, name)
		}
		byFile[name] = append(byFile[name], f)
	}
	var files []File
	for i, name := range order {
		// Each file's constants have names of their own, so that the files
		// can be compiled as one unit with the runtime.
		fs := &fileState{g: g, strs: map[string]string{}, prefix: fmt.Sprintf("ey_m%d", i)}
		var body strings.Builder
		for _, f := range byFile[name] {
			body.WriteString(g.function(fs, f))
		}
		var out strings.Builder
		out.WriteString("/* Generated by eye build from the low IR. Don't edit. */\n#include \"program.h\"\n\n")
		out.WriteString(fs.decls.String())
		out.WriteString("\n")
		out.WriteString(body.String())
		files = append(files, File{Name: name, Src: []byte(out.String())})
	}
	return files
}

// fileState is what one file declares once: its string constants and big
// integers.
type fileState struct {
	g      *gen
	strs   map[string]string
	n      int
	prefix string // of its constants' names
	decls  strings.Builder
}

// str is a static String with the text s, made once per file.
func (fs *fileState) str(s string, ty string) string {
	key := ty + "\x00" + s
	if name, ok := fs.strs[key]; ok {
		return name
	}
	name := fmt.Sprintf("%s_s%d", fs.prefix, fs.n)
	fs.n++
	fs.strs[key] = name
	fmt.Fprintf(&fs.decls, "static struct { ey_obj h; int64_t len, cap; char b[%d]; } %s = {{EY_IMMORTAL, 0, %s}, %d, 0, %s};\n",
		len(s)+1, name, ty, len(s), cstring(s))
	fs.strs[key] = "EY_V(&" + name + ")"
	return fs.strs[key]
}

// bigInt is an Int constant: small in the word, or a static big integer.
func (fs *fileState) bigInt(n *big.Int) string {
	if n.IsInt64() {
		v := n.Int64()
		if v >= -(1<<62) && v < 1<<62 {
			return fmt.Sprintf("EY_SMALL(INT64_C(%d))", v)
		}
	}
	key := "big\x00" + n.String()
	if name, ok := fs.strs[key]; ok {
		return name
	}
	mag := new(big.Int).Abs(n)
	var limbs []string
	for mag.Sign() > 0 {
		limbs = append(limbs, strconv.FormatUint(new(big.Int).And(mag, big.NewInt(0xffffffff)).Uint64(), 10)+"u")
		mag.Rsh(mag, 32)
	}
	name := fmt.Sprintf("%s_k%d", fs.prefix, fs.n)
	fs.n++
	neg := 0
	if n.Sign() < 0 {
		neg = 1
	}
	fmt.Fprintf(&fs.decls, "static struct { ey_obj h; int32_t neg; uint32_t n; uint32_t d[%d]; } %s = {{EY_IMMORTAL, 0, &ey_t_big}, %d, %d, {%s}};\n",
		len(limbs), name, neg, len(limbs), strings.Join(limbs, ", "))
	fs.strs[key] = "EY_V(&" + name + ")"
	return fs.strs[key]
}

// resultTy is the type of what f returns. A global's instance says () for
// its result; what it returns has the global's type.
func (g *gen) resultTy(f *lir.Func) ir.Ty {
	if f.Kind == lir.KindGlobal {
		for _, b := range f.Blocks {
			if r, ok := b.Term.(*lir.Return); ok && r.Value != nil {
				return r.Value.Ty
			}
		}
	}
	return f.Result
}

// signature is f's C prototype: each parameter and the result as its type
// is laid out, a `mut` one as the address of the caller's variable.
func (g *gen) signature(f *lir.Func) string {
	var params []string
	mut := mutParams(f)
	for i, p := range f.Params {
		if mut[i] {
			params = append(params, fmt.Sprintf("%s *v%d", g.ctype(p.Ty), p.ID))
		} else {
			params = append(params, fmt.Sprintf("%s v%d", g.ctype(p.Ty), p.ID))
		}
	}
	if hasDefaults(f) {
		params = append(params, "uint64_t absent")
	}
	if len(params) == 0 {
		params = []string{"void"}
	}
	return fmt.Sprintf("%s %s(%s)", g.ctype(g.resultTy(f)), g.names[f], strings.Join(params, ", "))
}

// mutParams says which of f's parameters are `mut`: passed as the address
// of the caller's variable, whose value the function changes.
func mutParams(f *lir.Func) []bool { return lir.MutParams(f) }

// hasDefaults reports whether f has parameters with defaults: a call that
// leaves them out says so in a mask, bit i for parameter i.
func hasDefaults(f *lir.Func) bool {
	if f.Src == nil || f.Kind == lir.KindLambda {
		return false
	}
	for _, p := range f.Src.Params {
		if p.Default != nil {
			return true
		}
	}
	return false
}

// programH declares what the files share: the structs values are held in,
// the descriptors and every function's prototype.
func (g *gen) programH() string {
	var b strings.Builder
	b.WriteString("/* Generated by eye build from the low IR. Don't edit. */\n")
	b.WriteString("#ifndef EYE_PROGRAM_H\n#define EYE_PROGRAM_H\n#include <math.h>\n#include \"eye.h\"\n#include \"builtins.h\"\n\n")
	// The closures' structs name the values' structs.
	var clos strings.Builder
	for _, f := range g.closures() {
		cl := g.closureStruct(f)
		if cl == "" {
			continue
		}
		fmt.Fprintf(&clos, "typedef struct %s {\n\tey_fn h;\n", cl)
		for i, p := range f.Params[:captures(f)] {
			fmt.Fprintf(&clos, "\t%s c%d;\n", g.ctype(p.Ty), i)
		}
		fmt.Fprintf(&clos, "} %s;\n", cl)
	}
	b.WriteString(g.structDefs())
	b.WriteString(clos.String())
	var names []string
	for _, d := range g.descs {
		names = append(names, d.name)
	}
	for _, name := range g.closDesc {
		names = append(names, name)
	}
	for _, name := range g.dynTab {
		names = append(names, name+"_d")
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(&b, "extern const ey_type %s;\n", n)
	}
	for _, name := range g.dynTab {
		fmt.Fprintf(&b, "extern const ey_method %s[];\n", name)
	}
	b.WriteString("\n")
	for _, f := range g.funcs {
		fmt.Fprintf(&b, "%s;\n", g.signature(f))
		if g.tramps[f] {
			fmt.Fprintf(&b, "void %s_tr(ey_fn *self, void *out, void *const *a);\n", g.names[f])
		}
		if g.methods[f] {
			fmt.Fprintf(&b, "void %s_dm(void *self, void *out, void *const *a);\n", g.names[f])
		}
		if f.Kind == lir.KindGlobal {
			fmt.Fprintf(&b, "%s %s_g(void);\n", g.ctype(g.resultTy(f)), g.names[f])
		}
	}
	for _, p := range g.wrapProtos {
		b.WriteString(p)
	}
	b.WriteString("#endif\n")
	return b.String()
}

// closures are the functions the program makes closures of, in the order
// their descriptors were named.
func (g *gen) closures() []*lir.Func {
	var clos []*lir.Func
	for f := range g.closDesc {
		clos = append(clos, f)
	}
	sort.Slice(clos, func(i, j int) bool { return g.closDesc[clos[i]] < g.closDesc[clos[j]] })
	return clos
}

// programC writes the descriptors, the dyn tables, the zones and the
// entry point.
func (g *gen) programC() string {
	var b strings.Builder
	b.WriteString("/* Generated by eye build from the low IR. Don't edit. */\n#include <stdio.h>\n#include <stdlib.h>\n#include \"program.h\"\n\n")
	var defs strings.Builder
	// What names types comes first, the descriptors last, since each may
	// name more.
	zones := g.zoneTables()
	entry := g.entry()
	for _, f := range g.closures() {
		n := captures(f)
		cl := g.closureStruct(f)
		var caps, offs []string
		for i, p := range f.Params[:n] {
			caps = append(caps, g.desc(p.Ty))
			offs = append(offs, fmt.Sprintf("offsetof(%s, c%d)", cl, i))
		}
		fields := map[string]string{"kind": "EY_FN", "n": fmt.Sprint(n), "size": "8", "align": "8", "repr": "EY_R_WORD", "objsize": "sizeof(ey_fn)"}
		if n > 0 {
			fmt.Fprintf(&defs, "static const ey_type *const %s_a[] = {%s};\n", g.closDesc[f], strings.Join(caps, ", "))
			fmt.Fprintf(&defs, "static const uint32_t %s_o[] = {%s};\n", g.closDesc[f], strings.Join(offs, ", "))
			fields["args"], fields["offs"], fields["objsize"] = g.closDesc[f]+"_a", g.closDesc[f]+"_o", fmt.Sprintf("sizeof(%s)", cl)
		}
		name := f.Name
		if f.Src != nil && f.Kind != lir.KindLambda {
			name = f.Src.Name
		}
		fields["name"] = cstring(name)
		fmt.Fprintf(&defs, "const ey_type %s = {%s};\n", g.closDesc[f], initializer(fields))
	}
	var tabs []*lir.DynTable
	for t := range g.dynTab {
		tabs = append(tabs, t)
	}
	sort.Slice(tabs, func(i, j int) bool { return g.dynTab[tabs[i]] < g.dynTab[tabs[j]] })
	for _, t := range tabs {
		name := g.dynTab[t]
		var ms []string
		for _, m := range t.Trait.Order {
			if f := t.Methods[m]; f != nil {
				ms = append(ms, g.names[f]+"_dm")
			} else {
				ms = append(ms, "NULL")
			}
		}
		if len(ms) == 0 {
			ms = []string{"NULL"}
		}
		fmt.Fprintf(&defs, "const ey_method %s[] = {%s};\n", name, strings.Join(ms, ", "))
		inner := g.desc(t.Type)
		fmt.Fprintf(&defs, "static const ey_type *const %s_a[] = {%s};\n", name, inner)
		fields := map[string]string{"kind": "EY_DYN", "n": "1", "size": "8", "align": "8", "repr": "EY_R_WORD",
			"objsize": fmt.Sprintf("sizeof(ey_dyn) + sizeof(%s)", g.ctype(t.Type)), "name": cstring(t.Trait.Name), "args": name + "_a"}
		fmt.Fprintf(&defs, "const ey_type %s_d = {%s};\n", name, initializer(fields))
	}
	for i := 0; i < len(g.descQueue); i++ {
		defs.WriteString(g.define(g.descQueue[i]))
	}
	b.WriteString(defs.String())
	b.WriteString("\n")
	for _, w := range g.wrappers {
		b.WriteString(w)
	}
	b.WriteString(zones)
	b.WriteString(entry)
	return b.String()
}

// entry writes the tests' table or main's wrapper, and main itself.
func (g *gen) entry() string {
	var b strings.Builder
	if g.opts.Tests {
		for i, t := range g.tests {
			fmt.Fprintf(&b, "static void ey_test%d(void) { (void)%s(); }\n", i, g.names[t.fn])
		}
		b.WriteString("static const ey_test ey_tests[] = {\n")
		for i, t := range g.tests {
			fmt.Fprintf(&b, "\t{%s, ey_test%d},\n", cstring(t.name), i)
		}
		if len(g.tests) == 0 {
			b.WriteString("\t{NULL, NULL},\n")
		}
		b.WriteString("};\n")
		fmt.Fprintf(&b, "static const ey_program ey_prog = {ey_tests, %d, NULL, ey_zone_list, %d};\n", len(g.tests), g.nzones)
	} else {
		m := g.main
		b.WriteString("static void ey_run_main(void) {\n")
		var args []string
		// main owns its arguments' list when it keeps it, and borrows it
		// otherwise, which leaves it to be dropped here.
		dropArgs := false
		if len(m.Params) > 0 {
			fmt.Fprintf(&b, "\tey_v args = ey_args(%s);\n", g.desc(m.Params[0].Ty))
			args = append(args, "args")
			dropArgs = len(m.Modes) == 0 || m.Modes[0] != lir.Owned
		}
		if hasDefaults(m) {
			args = append(args, "0")
		}
		fmt.Fprintf(&b, "\t%s r = %s(%s);\n", g.ctype(m.Result), g.names[m], strings.Join(args, ", "))
		rd := g.desc(m.Result)
		if c, ok := rep(m.Result).(*ir.TCon); ok && c.Name == "Result" && len(c.Args) == 2 {
			et := g.ctype(c.Args[1])
			errv := fmt.Sprintf("(*(%s *)ey_variant_field(%s, &r, 1, 0))", et, rd)
			show := fmt.Sprintf("ey_show(%s, &%s)", g.desc(c.Args[1]), errv)
			if s := g.low.MainErrShow; s != nil && g.reach[s] {
				arg := errv
				if len(s.Modes) > 0 && s.Modes[0] == lir.Owned {
					if st := g.dupStmt(c.Args[1], errv); st != "" {
						fmt.Fprintf(&b, "\tif (ey_variant_of(%s, &r) == 1) { %s }\n", rd, st)
					}
				}
				show = g.names[s] + "(" + arg + ")"
				if hasDefaults(s) {
					show = g.names[s] + "(" + arg + ", 0)"
				}
			}
			fmt.Fprintf(&b, "\tif (ey_variant_of(%s, &r) == 1) {\n\t\tey_v s = %s;\n\t\tey_eprintln(s);\n\t\tey_drop(s);\n\t\tey_drop_at(%s, &r);\n", rd, show, rd)
			if dropArgs {
				b.WriteString("\t\tey_drop(args);\n")
			}
			b.WriteString("\t\texit(1);\n\t}\n")
		}
		// What main made is dropped, so that a count of objects balances.
		fmt.Fprintf(&b, "\tey_drop_at(%s, &r);\n", rd)
		if dropArgs {
			b.WriteString("\tey_drop(args);\n")
		}
		b.WriteString("}\n")
		fmt.Fprintf(&b, "static const ey_program ey_prog = {NULL, 0, ey_run_main, ey_zone_list, %d};\n", g.nzones)
	}
	b.WriteString("int main(int argc, char **argv) { return ey_start(&ey_prog, argc, argv); }\n")
	return b.String()
}
