// Package check turns a program's source into the resolved form in package
// ir. It loads the program's module and the modules it imports, resolves
// every name, and reports what it finds wrong: syntax errors, undefined or
// duplicate names, waiting calls where nothing can wait, and assignments to
// what can't change. Once every name resolves, it infers and checks types
// (infer.go), exhaustive matches (exhaust.go) and effects (effects.go).
package check

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"eyelang/internal/cron"
	"eyelang/ir"
	"eyelang/syntax"
)

//go:embed prelude.eye
var preludeSrc []byte

// storeSrc is the in-memory store the runtime keeps a program's state in,
// written in Eyelang. It is part of every program, and no program imports it.
//
//go:embed runtime/store.eye
var storeSrc []byte

// std holds the standard modules written in Eyelang, one file each:
// `import ai` loads std/ai.eye.
//
//go:embed std/*.eye
var std embed.FS

// Diagnostic is a problem found in a program.
type Diagnostic struct {
	Pos     ir.Pos
	Message string
}

func (d Diagnostic) String() string { return d.Pos.String() + ": " + d.Message }

// Program checks the program in dir: the module made of the .eye files in
// dir, and the modules it imports, which are directories below dir
// (`import lab.notify` is lab/notify). It returns the program even when there
// are diagnostics, so that tools can use what resolved.
func Program(dir string) (*ir.Program, []Diagnostic) {
	c := &checker{root: dir, modules: map[string]*module{}}
	return c.program("")
}

// program checks the module at path, "" for the program's own, and what it
// imports.
func (c *checker) program(path string) (*ir.Program, []Diagnostic) {
	c.prog = &ir.Program{}
	c.readConfig()
	prelude := c.parse("prelude", "", []*ir.File{c.parseFile("<prelude>", preludeSrc)})
	c.prog.Prelude = prelude.mod
	c.prelude = prelude
	c.loadBuiltins()
	c.prog.Store = c.parse("runtime.store", "", []*ir.File{c.parseFile("<runtime store>", storeSrc)}).mod
	root := c.load(path, filepath.Join(append([]string{c.root}, strings.Split(path, ".")...)...))
	if root == nil {
		return c.prog, c.diags
	}
	c.prog.Root = root.mod
	c.loadAll()
	for _, m := range c.order {
		c.lowerModule(m)
	}
	c.prog.Modules = append([]*ir.Module{prelude.mod}, c.prog.Modules...)
	// Types are checked once every name resolves, so that one undefined
	// name doesn't bring a crowd of type errors with it. A syntax error
	// doesn't stop them: the declaration it is in was skipped, and a use of
	// what it declares may be any type. A type whose declaration was
	// skipped does stop them where it is written, since nothing about its
	// values could be checked.
	if len(c.diags) == c.syntaxErrors && len(c.brokenTypes) == 0 {
		c.checkTypes()
	}
	sort.SliceStable(c.diags, func(i, j int) bool {
		a, b := c.diags[i].Pos, c.diags[j].Pos
		if a.File == nil || b.File == nil || a.File != b.File {
			return false
		}
		return a.Offset < b.Offset
	})
	return c.prog, c.diags
}

type checker struct {
	root    string
	prog    *ir.Program
	prelude *module
	modules map[string]*module // by import path
	order   []*module          // dependencies before the modules that import them
	diags   []Diagnostic
	ts      *typeState
	nextVar int
	// variantChoices are the variant names written alone that several
	// imported modules declare, with the candidates, until the type
	// expected settles each (fnCheck.chooseVariant).
	variantChoices map[*ir.Ref][]*ir.Variant
	overlay        map[string][]byte       // unsaved files, by path
	parsed         map[string]*ir.File     // immutable syntax shared by a formatting session
	index          *Index                  // nil unless a tool asked for it
	all            bool                    // load every module below root, for tools
	names          names                   // where names are written, until types resolve them
	dyns           []dynUse                // each dyn Trait written, checked once traits are known
	typePos        map[*ir.TypeExpr]ir.Pos // where each type as written is
	public         map[any]bool            // declarations marked pub that have no Pub field of their own
	opaque         map[*ir.Type]bool       // types declared `pub opaque`

	// importRoots are the directories imports are looked for below: the
	// program's own, then those its eye.toml lists in modules.
	importRoots []string

	// signatures are the public functions whose uses clause eye fmt rewrites.
	signatures []Signature

	// syntaxErrors is how many of diags are syntax errors, and brokenTypes
	// the types written that name a declaration skipped for one: each may
	// be any type, as its first use decides.
	syntaxErrors int
	brokenTypes  map[*ir.TypeExpr]bool

	// builtins are the signatures and effects ir/builtin.eye declares.
	builtins builtinDecls
}

// dynUse is a `dyn Trait` as written.
type dynUse struct {
	trait *ir.Trait
	args  int // the type arguments given to the trait
	at    ir.Pos
}

// module is a module being checked, with its namespaces.
type module struct {
	mod     *ir.Module
	values  map[string]any // lower-case names: functions, globals, jobs, views, imported modules
	types   map[string]any // upper-case names: types, variants, entities, contracts
	imports []*module
	decls   []func() // lowering of bodies, run once every module is collected
	loading bool
	// broken holds the names of the declarations skipped for a syntax
	// error in them, so that a use of one isn't also reported as undefined.
	broken map[string]bool
}

// errorf reports a problem at pos, once: a type written once and used at
// many calls is reported where it is written, not at each call.
func (c *checker) errorf(pos ir.Pos, format string, args ...any) {
	d := Diagnostic{Pos: pos, Message: fmt.Sprintf(format, args...)}
	for _, old := range c.diags {
		if old == d {
			return
		}
	}
	c.diags = append(c.diags, d)
}

func parseFile(path string, src []byte) *ir.File {
	return &ir.File{Path: path, Src: src, Tree: syntax.Parse(src)}
}

// load reads the module at path from dir, and the modules it imports.
func (c *checker) load(path, dir string) *module {
	entries, err := os.ReadDir(dir)
	if err != nil && !c.overlayHasDir(dir) {
		c.errorf(ir.Pos{}, "reading %s: %v", dir, err)
		return nil
	}
	var files []*ir.File
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".eye") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		src, err := c.readFile(p)
		if err != nil {
			c.errorf(ir.Pos{}, "%v", err)
			continue
		}
		files = append(files, c.parseFile(p, src))
	}
	// A file in the overlay that isn't on disk yet, such as an editor's new
	// buffer or the REPL's session, is part of its directory's module too.
	var unsaved []string
	for p := range c.overlay {
		if filepath.Dir(p) == filepath.Clean(dir) && strings.HasSuffix(p, ".eye") {
			if _, err := os.Stat(p); err != nil {
				unsaved = append(unsaved, p)
			}
		}
	}
	sort.Strings(unsaved)
	for _, p := range unsaved {
		files = append(files, c.parseFile(p, c.overlay[p]))
	}
	if len(files) == 0 {
		c.errorf(ir.Pos{}, "%s has no .eye files", dir)
		return nil
	}
	return c.parse(path, dir, files)
}

// parse collects the declarations of a module's files.
func (c *checker) parse(path, dir string, files []*ir.File) *module {
	if c.public == nil {
		c.public, c.opaque = map[any]bool{}, map[*ir.Type]bool{}
	}
	name := path
	if i := strings.LastIndex(path, "."); i >= 0 {
		name = path[i+1:]
	}
	m := &module{
		mod: &ir.Module{
			Path: path, Name: name, Dir: dir, Files: files,
			Types: map[string]*ir.Type{}, Variants: map[string]*ir.Variant{}, Funcs: map[string]*ir.Func{},
			Entities: map[string]*ir.Entity{}, Jobs: map[string]*ir.Job{}, Views: map[string]*ir.View{},
			Contracts: map[string]*ir.Contract{}, Traits: map[string]*ir.Trait{},
		},
		values:  map[string]any{},
		types:   map[string]any{},
		loading: true,
		broken:  map[string]bool{},
	}
	c.modules[path] = m
	// A declaration with a syntax error in it is skipped, and the rest of
	// its file is checked, so that an editor keeps its errors, hover and
	// completion while one declaration is half typed.
	var whole []*syntax.Node
	var wholeFile []*ir.File
	for _, f := range files {
		before := len(c.diags)
		for _, d := range f.Tree.Diagnostics {
			c.errorf(ir.Pos{File: f, Offset: d.Span.Start}, "%s", d.Message)
		}
		c.syntaxErrors += len(c.diags) - before
		for _, d := range f.Tree.Root.Children {
			if !c.skipBroken(m, f, d) {
				whole, wholeFile = append(whole, d), append(wholeFile, f)
			}
		}
	}
	// Imports first, so that their names are known when bodies are lowered.
	for i, d := range whole {
		if d.Kind == syntax.ImportDeclaration {
			c.importDecl(m, wholeFile[i], d)
		}
	}
	// Impls last, so that the type, trait or contract an impl names is
	// known wherever in the module it is declared.
	for _, impls := range []bool{false, true} {
		for i, d := range whole {
			if (d.Kind == syntax.ImplDeclaration) == impls {
				c.collect(m, wholeFile[i], d)
			}
		}
	}
	m.loading = false
	if path != "prelude" {
		c.order = append(c.order, m)
		c.prog.Modules = append(c.prog.Modules, m.mod)
	} else {
		c.order = append([]*module{m}, c.order...)
	}
	return m
}

// skipBroken reports whether a top-level declaration has a syntax error in
// it, which its file's diagnostics say, and then records the name it
// declares, when that can be read.
func (c *checker) skipBroken(m *module, f *ir.File, d *syntax.Node) bool {
	broken := d.Kind == syntax.ErrorNode
	for _, diag := range f.Tree.Diagnostics {
		if d.Span.Start <= diag.Span.Start && diag.Span.Start <= d.Span.End {
			broken = true
		}
	}
	if !broken {
		return false
	}
	name := d.ChildByField(syntax.FieldName)
	if d.Kind == syntax.LetStatement {
		name = d.ChildByField(syntax.FieldPattern)
	}
	if name != nil && name.IsToken() {
		m.broken[text(f, name)] = true
	}
	return true
}

func text(f *ir.File, n *syntax.Node) string { return n.Text(f.Src) }

func pos(f *ir.File, n *syntax.Node) ir.Pos {
	return ir.Pos{File: f, Offset: n.Span.Start, End: n.Span.End}
}

func (c *checker) importDecl(m *module, f *ir.File, d *syntax.Node) {
	pathNode := d.ChildByField(syntax.FieldPath)
	var parts []string
	for _, p := range pathNode.NamedChildren() {
		parts = append(parts, text(f, p))
	}
	path := strings.Join(parts, ".")
	name := parts[len(parts)-1]
	if _, ok := ir.BuiltinModules[path]; ok {
		return // standard modules are always in scope
	}
	imp := c.modules[path]
	if src, err := std.ReadFile("std/" + path + ".eye"); imp == nil && err == nil {
		imp = c.parse(path, "", []*ir.File{c.parseFile("<std "+path+">", src)})
	}
	if imp == nil {
		// The first root that has the module's directory wins.
		dir := ""
		var searched []string
		for _, root := range c.importRoots {
			d := filepath.Join(append([]string{root}, parts...)...)
			if c.isDir(d) {
				dir = d
				break
			}
			searched = append(searched, d)
		}
		if dir == "" {
			c.errorf(pos(f, pathNode), "no module %s: there is no directory %s", path, orList(searched))
			return
		}
		imp = c.load(path, dir)
		if imp == nil {
			return
		}
	} else if imp.loading {
		c.errorf(pos(f, pathNode), "module %s imports itself, directly or through other modules", path)
		return
	}
	// Every file that uses a module imports it; the module's files share
	// the imports.
	c.use(pos(f, pathNode.NamedChildren()[len(parts)-1]), imp.mod)
	if existing, ok := m.values[name]; !ok || existing != imp.mod {
		m.imports = append(m.imports, imp)
		c.define(m, f, pathNode, name, imp.mod)
	}
	// `import m.{a, B}` also brings those names in unqualified.
	for _, item := range d.ChildrenByField(syntax.FieldItem) {
		n := text(f, item)
		var decl any
		if item.Kind == syntax.TypeIdent {
			decl = imp.types[n]
		} else {
			decl = imp.values[n]
		}
		if decl == nil {
			if imp.broken[n] {
				m.broken[n] = true // it has a syntax error, reported in its own file
				continue
			}
			c.errorf(pos(f, item), "module %s has no %s", path, n)
			continue
		}
		c.use(pos(f, item), decl)
		if msg := c.privateUse(m.mod, decl, n); msg != "" {
			c.errorf(pos(f, item), "%s", msg)
			continue
		}
		if m.types[n] == decl || m.values[n] == decl {
			continue
		}
		c.define(m, f, item, n, decl)
	}
}

// define adds a name to a module's namespace.
func (c *checker) define(m *module, f *ir.File, at *syntax.Node, name string, decl any) {
	space := m.values
	if name != "" && name[0] >= 'A' && name[0] <= 'Z' {
		space = m.types
	}
	if _, dup := space[name]; dup {
		c.errorf(pos(f, at), "%s is declared twice in this module", name)
		return
	}
	space[name] = decl
}

// collect adds a top-level declaration to its module and queues the lowering
// of its body.
func (c *checker) collect(m *module, f *ir.File, d *syntax.Node) {
	nameNode := d.ChildByField(syntax.FieldName)
	c.checkAttributes(f, d)
	if nameNode != nil && hasToken(f, d, "pub") && d.Kind != syntax.TypeDeclaration {
		// After the declaration is defined: its entity, job, view, trait or
		// contract is public. Functions and module-level lets say so
		// themselves.
		defer func() {
			n := text(f, nameNode)
			for _, space := range []map[string]any{m.types, m.values} {
				if decl, ok := space[n]; ok {
					c.public[decl] = true
				}
			}
		}()
	}
	switch d.Kind {
	case syntax.ImportDeclaration, syntax.EOF:
	case syntax.LetStatement:
		pat := d.ChildByField(syntax.FieldPattern)
		if pat.Kind != syntax.Ident {
			c.errorf(pos(f, pat), "a module-level let binds one name")
			return
		}
		g := &ir.Global{Name: text(f, pat), Module: m.mod, Index: len(m.mod.Globals), Pub: hasToken(f, d, "pub"), Pos: pos(f, d)}
		m.mod.Globals = append(m.mod.Globals, g)
		c.define(m, f, pat, g.Name, g)
		m.decls = append(m.decls, func() {
			g.Type = c.typeExpr(m, f, d.ChildByField(syntax.FieldType), nil)
			l := c.newFunc(m, f, "let "+g.Name, pos(f, d), ctxGlobal)
			l.fn.Body = l.expr(d.ChildByField(syntax.FieldValue))
			g.Init = l.done()
		})
	case syntax.TypeDeclaration:
		c.typeDecl(m, f, d)
	case syntax.FunctionDeclaration:
		fn := &ir.Func{Name: text(f, nameNode), Module: m.mod, Pub: hasToken(f, d, "pub"), Pos: pos(f, d),
			Doc: docComment(f, d)}
		m.mod.Funcs[fn.Name] = fn
		c.define(m, f, nameNode, fn.Name, fn)
		if fn.Name == "main" {
			m.mod.Main = fn
		}
		fn.Budget = budget(f, d)
		m.decls = append(m.decls, func() { c.lowerFunc(m, f, d, fn, ctxPlain, nil, nil) })
	case syntax.ExternTypeDeclaration:
		// A type foreign code owns: no variants, fields or alias, so only
		// extern functions make or take its values (SPECS.md §18).
		t := &ir.Type{Name: text(f, nameNode), Module: m.mod, Methods: map[string]*ir.Func{}, Pos: pos(f, d),
			Doc: docComment(f, d), Extern: true}
		if tp := d.ChildByField(syntax.FieldTypeParameters); tp != nil {
			for _, p := range tp.NamedChildren() {
				t.Params = append(t.Params, text(f, p))
			}
		}
		m.mod.Types[t.Name] = t
		c.define(m, f, nameNode, t.Name, t)
	case syntax.ExternDeclaration:
		fn := &ir.Func{Name: text(f, nameNode), Module: m.mod, Pub: hasToken(f, d, "pub"), Pos: pos(f, d), Extern: true}
		if args, _, ok := attribute(f, d, "host"); ok && len(args) == 1 {
			fn.Host = c.stringArg(f, args[0])
		}
		if args, _, ok := attribute(f, d, "library"); ok && len(args) == 1 {
			fn.Library = c.stringArg(f, args[0])
		}
		m.mod.Funcs[fn.Name] = fn
		c.define(m, f, nameNode, fn.Name, fn)
		// Its parameters and defaults; it has no body.
		m.decls = append(m.decls, func() { c.lowerFunc(m, f, d, fn, ctxPlain, nil, nil) })
	case syntax.EntityDeclaration:
		c.entityDecl(m, f, d)
	case syntax.JobDeclaration:
		job := &ir.Job{Name: text(f, nameNode), Module: m.mod, Pos: pos(f, d)}
		job.Func = &ir.Func{Name: job.Name, Module: m.mod, Pub: hasToken(f, d, "pub"), Pos: pos(f, d), Budget: budget(f, d)}
		m.mod.Jobs[job.Name] = job
		c.define(m, f, nameNode, job.Name, job)
		m.decls = append(m.decls, func() {
			c.lowerFunc(m, f, d, job.Func, ctxJob, nil, nil)
			c.triggers(m, f, d, job)
			c.jobLimits(m, f, d, job)
		})
	case syntax.ViewDeclaration:
		v := &ir.View{Name: text(f, nameNode), Module: m.mod, Pos: pos(f, d)}
		m.mod.Views[v.Name] = v
		c.define(m, f, nameNode, v.Name, v)
		m.decls = append(m.decls, func() {
			l := c.newFunc(m, f, "view "+v.Name, pos(f, d), ctxView)
			l.fn.Body = l.expr(d.ChildByField(syntax.FieldValue))
			v.Func = l.done()
			v.Entity = viewEntity(v.Func.Body)
			if v.Entity == nil {
				c.errorf(pos(f, d), "a view is a pipeline that starts from an entity set, such as World.all")
			}
		})
	case syntax.ContractDeclaration:
		c.contractDecl(m, f, d)
	case syntax.ImplDeclaration:
		c.implDecl(m, f, d)
	case syntax.TraitDeclaration:
		c.traitDecl(m, f, d)
	case syntax.TestDeclaration:
		name := strings.Trim(text(f, nameNode), `"`)
		t := &ir.Test{Name: name, Module: m.mod, Pos: pos(f, d)}
		m.mod.Tests = append(m.mod.Tests, t)
		m.decls = append(m.decls, func() {
			l := c.newFunc(m, f, "test "+name, pos(f, d), ctxTest)
			l.fn.Body = l.block(d.ChildByField(syntax.FieldBody))
			t.Func = l.done()
		})
	default:
		c.errorf(pos(f, d), "unexpected %s at the top level", d.Kind)
	}
}

// viewEntity finds the entity set a view's pipeline starts from.
func viewEntity(e ir.Expr) *ir.Entity {
	for {
		switch x := e.(type) {
		case *ir.Field:
			if r, ok := x.Value.(*ir.Ref); ok {
				if ent, ok := r.Decl.(*ir.Entity); ok && x.Name == "all" {
					return ent
				}
			}
			return nil
		case *ir.Call:
			if len(x.Args) == 0 {
				return nil
			}
			e = x.Args[0].Value
		default:
			return nil
		}
	}
}

func hasToken(f *ir.File, d *syntax.Node, tok string) bool {
	for _, c := range d.Children {
		if !c.Kind.IsNamed() && text(f, c) == tok {
			return true
		}
	}
	return false
}

func (c *checker) typeDecl(m *module, f *ir.File, d *syntax.Node) {
	nameNode := d.ChildByField(syntax.FieldName)
	t := &ir.Type{Name: text(f, nameNode), Module: m.mod, Methods: map[string]*ir.Func{}, Pos: pos(f, d),
		Doc: docComment(f, d)}
	if tp := d.ChildByField(syntax.FieldTypeParameters); tp != nil {
		for _, p := range tp.NamedChildren() {
			t.Params = append(t.Params, text(f, p))
		}
	}
	m.mod.Types[t.Name] = t
	c.public[t] = hasToken(f, d, "pub")
	c.opaque[t] = hasToken(f, d, "opaque")
	value := d.ChildByField(syntax.FieldValue)
	if value.Kind == syntax.VariantList {
		// A variant's `##` comment sits above its `|`.
		doc := ""
		for _, vn := range value.Children {
			if vn.Kind != syntax.Variant {
				doc = docComment(f, vn)
				continue
			}
			if own := docComment(f, vn); own != "" {
				doc = own
			}
			v := &ir.Variant{Name: text(f, vn.ChildByField(syntax.FieldName)), Type: t, Doc: doc}
			doc = ""
			t.Variants = append(t.Variants, v)
			m.mod.Variants[v.Name] = v
		}
	}
	// A type and its own single variant may share a name: `type Id = | Id(String)`.
	if len(t.Variants) != 1 || t.Variants[0].Name != t.Name {
		c.define(m, f, nameNode, t.Name, t)
	}
	for _, v := range t.Variants {
		c.define(m, f, nameNode, v.Name, v)
	}
	m.decls = append([]func(){func() { c.typeBody(m, f, t, value) }}, m.decls...)
}

// typeBody resolves the field types of a type declaration.
func (c *checker) typeBody(m *module, f *ir.File, t *ir.Type, value *syntax.Node) {
	switch value.Kind {
	case syntax.VariantList:
		for i, vn := range value.NamedChildren() {
			payload := vn.ChildByField(syntax.FieldPayload)
			if payload == nil {
				continue
			}
			for _, field := range payload.NamedChildren() {
				if field.Kind == syntax.FieldDeclaration {
					t.Variants[i].Fields = append(t.Variants[i].Fields, ir.VariantField{
						Name: text(f, field.ChildByField(syntax.FieldName)),
						Type: c.typeExpr(m, f, field.ChildByField(syntax.FieldType), t.Params),
					})
				} else {
					t.Variants[i].Fields = append(t.Variants[i].Fields, ir.VariantField{Type: c.typeExpr(m, f, field, t.Params)})
				}
			}
		}
	case syntax.RecordType:
		for _, field := range value.NamedChildren() {
			name := text(f, field.ChildByField(syntax.FieldName))
			t.Fields = append(t.Fields, name)
			if doc := docComment(f, field); doc != "" {
				if t.FieldDocs == nil {
					t.FieldDocs = map[string]string{}
				}
				t.FieldDocs[name] = doc
			}
		}
		t.Alias = c.typeExpr(m, f, value, t.Params)
	default:
		t.Alias = c.typeExpr(m, f, value, t.Params)
		if via := aliasCycle(t.Alias, t, map[*ir.Type]bool{}); via != nil {
			if t.Alias.Decl == t && len(t.Alias.Args) > 0 {
				c.errorf(pos(f, value), "type %s = %s is an alias of itself: a type with one variant starts with |, as type %s = | %s",
					t.Name, text(f, value), t.Name, text(f, value))
			} else {
				names := make([]string, len(via))
				for i, d := range via {
					names[i] = d.Name
				}
				c.errorf(pos(f, value), "type %s is an alias of itself (%s): an alias names another type; a type that holds itself needs variants, as type %s = | …",
					t.Name, strings.Join(append(append([]string{t.Name}, names...), t.Name), " → "), t.Name)
			}
			t.Alias = &ir.TypeExpr{Name: "?"}
		}
	}
	why := func(v string) string {
		params := strings.Join(append(append([]string{}, t.Params...), v), ", ")
		return fmt.Sprintf("%s doesn't declare it: write type %s(%s) = …", t.Name, t.Name, params)
	}
	c.strayVars(t.Alias, t.Params, why)
	for _, v := range t.Variants {
		for _, field := range v.Fields {
			c.strayVars(field.Type, t.Params, why)
		}
	}
}

// aliasCycle reports whether te names t, directly or through other aliases,
// such as `type A = List(B)` with `type B = A`. It returns the aliases the
// cycle goes through, empty when te names t itself, and nil when it doesn't.
func aliasCycle(te *ir.TypeExpr, t *ir.Type, seen map[*ir.Type]bool) []*ir.Type {
	if te == nil {
		return nil
	}
	if te.Decl == t {
		return []*ir.Type{}
	}
	if d := te.Decl; d != nil && d.Alias != nil && !seen[d] {
		seen[d] = true
		if via := aliasCycle(d.Alias, t, seen); via != nil {
			return append([]*ir.Type{d}, via...)
		}
	}
	for _, a := range append(append([]*ir.TypeExpr{}, te.Args...), te.Result) {
		if via := aliasCycle(a, t, seen); via != nil {
			return via
		}
	}
	return nil
}

// strayVars reports each type variable in te that isn't among allowed,
// saying why with why(name): a type variable stands for a type only where
// something declares it.
func (c *checker) strayVars(te *ir.TypeExpr, allowed []string, why func(string) string) {
	if te == nil {
		return
	}
	if te.Decl == nil && te.Entity == nil && te.Trait == nil && isVarName(te.Name) && !slices.Contains(allowed, te.Name) &&
		te.Name != "record" && te.Name != "tuple" && te.Name != "fn" && te.Name != "dyn" {
		c.errorf(c.typePos[te], "%s is a type variable, and %s", te.Name, why(te.Name))
	}
	for _, a := range te.Args {
		c.strayVars(a, allowed, why)
	}
	c.strayVars(te.Result, allowed, why)
}

// builtinTypes are the type names every module knows.
var builtinTypes = map[string]bool{
	"Int": true, "Float": true, "Bool": true, "Char": true, "String": true, "Byte": true, "Bytes": true,
	"Int32": true, "Int64": true, "UInt32": true, "UInt64": true, "NonZero": true, "Instant": true,
	"Date": true, "Time": true, "Duration": true, "Zone": true, "Schedule": true, "Size": true,
	"List": true, "Map": true, "Set": true, "Iter": true, "Ref": true, "Run": true, "Key": true,
	"Secret": true, "Approval": true, "Version": true, "Range": true,
	"Event": true, "TaskGroup": true, "Sender": true, "Receiver": true, "Handler": true, "Fakes": true,
}

// brokenType records that t names a declaration skipped for a syntax error.
func (c *checker) brokenType(t *ir.TypeExpr) {
	if c.brokenTypes == nil {
		c.brokenTypes = map[*ir.TypeExpr]bool{}
	}
	c.brokenTypes[t] = true
}

// typeExpr resolves a type as written, and keeps where it is written for
// the type checker's messages.
func (c *checker) typeExpr(m *module, f *ir.File, n *syntax.Node, vars []string) *ir.TypeExpr {
	if n == nil {
		return nil
	}
	t := c.typeExprAt(m, f, n, vars)
	if c.typePos == nil {
		c.typePos = map[*ir.TypeExpr]ir.Pos{}
	}
	if _, ok := c.typePos[t]; !ok {
		c.typePos[t] = pos(f, n)
	}
	return t
}

func (c *checker) typeExprAt(m *module, f *ir.File, n *syntax.Node, vars []string) *ir.TypeExpr {
	switch n.Kind {
	case syntax.TypeIdent:
		name := text(f, n)
		t := &ir.TypeExpr{Name: name}
		d := c.lookupTypeName(m, name)
		if _, ok := d.(ambiguous); !ok {
			c.use(pos(f, n), d)
		}
		if d == nil && builtinTypes[name] {
			c.use(pos(f, n), BuiltinType{Name: name})
		}
		switch d := d.(type) {
		case *ir.Type:
			t.Decl = d
		case *ir.Variant:
			t.Decl = d.Type // a newtype named like its variant
		case *ir.Entity:
			t.Entity = d
		case *ir.Trait:
			t.Trait = d
		case *ir.Contract:
		default:
			if v, ok := c.lookupType(m, name).(*ir.Variant); ok && !builtinTypes[name] {
				c.errorf(pos(f, n), "%s is a variant of %s, not a type: write %s", name, v.Type.Name, v.Type.Name)
			} else if m.broken[name] {
				c.brokenType(t)
			} else if !builtinTypes[name] {
				c.errorf(pos(f, n), "undefined type %s", name)
			}
		}
		return t
	case syntax.TypeVariable, syntax.Ident:
		return &ir.TypeExpr{Name: text(f, n)}
	case syntax.GenericType:
		t := c.typeExpr(m, f, n.ChildByField(syntax.FieldName), vars)
		for _, a := range n.ChildByField(syntax.FieldArguments).NamedChildren() {
			t.Args = append(t.Args, c.typeExpr(m, f, a, vars))
		}
		if t.Name == "Ref" && len(t.Args) == 1 {
			t.Entity = t.Args[0].Entity
		}
		return t
	case syntax.QualifiedType:
		mod := text(f, n.ChildByField(syntax.FieldModule))
		name := text(f, n.ChildByField(syntax.FieldName))
		t := &ir.TypeExpr{Name: name}
		imp, _ := m.values[mod].(*ir.Module)
		if imp == nil {
			c.errorf(pos(f, n), "undefined module %s", mod)
			return t
		}
		c.use(pos(f, n.ChildByField(syntax.FieldModule)), imp)
		c.use(pos(f, n.ChildByField(syntax.FieldName)), c.modules[imp.Path].types[name])
		target := c.modules[imp.Path].types[name]
		if v, ok := target.(*ir.Variant); ok {
			target = v.Type // a newtype named like its variant
		}
		if msg := c.privateUse(m.mod, target, name); msg != "" {
			c.errorf(pos(f, n.ChildByField(syntax.FieldName)), "%s", msg)
		}
		switch d := c.modules[imp.Path].types[name].(type) {
		case *ir.Type:
			t.Decl = d
		case *ir.Variant:
			t.Decl = d.Type
		case *ir.Entity:
			t.Entity = d
		case *ir.Trait:
			t.Trait = d
		default:
			if c.modules[imp.Path].broken[name] {
				c.brokenType(t)
			} else {
				c.errorf(pos(f, n), "module %s has no type %s", mod, name)
			}
		}
		return t
	case syntax.UnitType:
		return &ir.TypeExpr{Name: "()"}
	case syntax.TupleType:
		t := &ir.TypeExpr{Name: "tuple"}
		for _, a := range n.NamedChildren() {
			t.Args = append(t.Args, c.typeExpr(m, f, a, vars))
		}
		return t
	case syntax.RecordType:
		t := &ir.TypeExpr{Name: "record"}
		for _, field := range n.NamedChildren() {
			t.Fields = append(t.Fields, text(f, field.ChildByField(syntax.FieldName)))
			t.Args = append(t.Args, c.typeExpr(m, f, field.ChildByField(syntax.FieldType), vars))
		}
		return t
	case syntax.FunctionType:
		t := &ir.TypeExpr{Name: "fn"}
		for _, p := range n.ChildByField(syntax.FieldParameters).NamedChildren() {
			t.Args = append(t.Args, c.typeExpr(m, f, p, vars))
		}
		t.Result = c.typeExpr(m, f, n.ChildByField(syntax.FieldReturnType), vars)
		t.Uses, t.HasUses = usesOf(f, n)
		return t
	case syntax.DynType:
		traitNode := n.ChildByField(syntax.FieldTrait)
		inner := c.typeExpr(m, f, traitNode, vars)
		if inner.Trait == nil {
			if inner.Decl != nil || inner.Entity != nil || builtinTypes[inner.Name] {
				c.errorf(pos(f, traitNode), "dyn takes a trait, and %s is a type: write %s, or dyn and a trait it implements", inner.Name, inner.Name)
			}
			return &ir.TypeExpr{Name: "dyn"}
		}
		c.dyns = append(c.dyns, dynUse{inner.Trait, len(inner.Args), pos(f, n)})
		return &ir.TypeExpr{Name: "dyn", Args: []*ir.TypeExpr{inner}}
	}
	return &ir.TypeExpr{Name: "?"}
}

func (c *checker) entityDecl(m *module, f *ir.File, d *syntax.Node) {
	nameNode := d.ChildByField(syntax.FieldName)
	e := &ir.Entity{Name: text(f, nameNode), Module: m.mod, Methods: map[string]*ir.Func{}, Pos: pos(f, d)}
	m.mod.Entities[e.Name] = e
	c.define(m, f, nameNode, e.Name, e)
	params := d.ChildByField(syntax.FieldParameters).NamedChildren()
	if len(params) != 1 {
		c.errorf(pos(f, d), "an entity has one identity parameter, such as (id: WorldId)")
	} else {
		e.ID = &ir.Param{Name: text(f, params[0].ChildByField(syntax.FieldName))}
	}
	body := d.ChildByField(syntax.FieldBody)
	m.decls = append(m.decls, func() { c.entityVersions(m, f, d, e) })
	for _, member := range body.NamedChildren() {
		switch member.Kind {
		case syntax.StateDeclaration:
			sn := member.ChildByField(syntax.FieldName)
			st := &ir.State{Name: text(f, sn)}
			if st.Name == "version" {
				// A snapshot's version counts transactions (SPECS.md §14),
				// and a state field of that name would be hidden by it.
				c.errorf(pos(f, sn), "a snapshot's version counts the transactions that changed %s, so a state field can't be named version: name it revision, or what it counts", e.Name)
			}
			e.States = append(e.States, st)
		case syntax.FunctionDeclaration:
			mn := member.ChildByField(syntax.FieldName)
			fn := &ir.Func{Name: text(f, mn), Module: m.mod, Entity: e, HasSelf: true, MutSelf: true,
				Pub: hasToken(f, member, "pub"), Pos: pos(f, member)}
			if _, dup := e.Methods[fn.Name]; dup {
				c.errorf(pos(f, mn), "%s.%s is declared twice", e.Name, fn.Name)
			}
			e.Methods[fn.Name] = fn
		}
	}
	m.decls = append(m.decls, func() {
		if e.ID != nil {
			e.ID.Type = c.typeExpr(m, f, params[0].ChildByField(syntax.FieldType), nil)
		}
		i := 0
		for _, member := range body.NamedChildren() {
			switch member.Kind {
			case syntax.StateDeclaration:
				st := e.States[i]
				i++
				st.Type = c.typeExpr(m, f, member.ChildByField(syntax.FieldType), nil)
				c.strayVars(st.Type, nil, func(string) string {
					return "an entity's state is of one type, which it names: write that type"
				})
				if def := member.ChildByField(syntax.FieldDefault); def != nil {
					l := c.newFunc(m, f, e.Name+"."+st.Name, pos(f, member), ctxGlobal)
					l.fn.Body = l.expr(def)
					st.Default = l.done()
				}
			case syntax.FunctionDeclaration:
				fn := e.Methods[text(f, member.ChildByField(syntax.FieldName))]
				c.lowerFunc(m, f, member, fn, ctxEntityMethod, e, nil)
			}
		}
	})
}

func (c *checker) contractDecl(m *module, f *ir.File, d *syntax.Node) {
	nameNode := d.ChildByField(syntax.FieldName)
	ct := &ir.Contract{Name: text(f, nameNode), Module: m.mod, Ops: map[string]*ir.Op{}, Events: map[string]*ir.Event{}, Doc: docComment(f, d)}
	m.mod.Contracts[ct.Name] = ct
	c.define(m, f, nameNode, ct.Name, ct)
	c.contractPolicy(m, f, d, ct)
	if _, named, ok := attribute(f, d, "http"); ok {
		if n := named["names"]; n != nil {
			if text(f, n) != "snake" {
				c.errorf(pos(f, n), "names: takes snake, for snake_case names on the wire")
			}
			ct.HTTP.Snake = true
		}
	}
	if _, named, ok := attribute(f, d, "auth"); ok {
		switch {
		case named["bearer"] != nil:
			ct.HTTP.AuthHeader, ct.HTTP.AuthBearer = "Authorization", true
			ct.HTTP.AuthSecret = c.stringArg(f, named["bearer"])
		case named["header"] != nil && named["secret"] != nil:
			ct.HTTP.AuthHeader = c.stringArg(f, named["header"])
			ct.HTTP.AuthSecret = c.stringArg(f, named["secret"])
		default:
			c.errorf(pos(f, d), `@auth takes bearer: "secret", or header: "X-Name", secret: "secret"`)
		}
	}
	names := map[any]*syntax.Node{} // an op's @reconcile argument, an event's @key argument
	for _, member := range d.ChildByField(syntax.FieldBody).NamedChildren() {
		if member.Kind == syntax.EventDeclaration {
			ev := &ir.Event{
				Name:     text(f, member.ChildByField(syntax.FieldName)),
				Contract: ct,
				Topic:    strings.Trim(text(f, member.ChildByField(syntax.FieldTopic)), `"`),
			}
			if args := attributeArgs(f, member, "key"); len(args) == 1 {
				ev.Key = text(f, args[0])
				names[ev] = args[0]
			}
			ct.Events[ev.Name] = ev
			member := member
			m.decls = append(m.decls, func() {
				ev.Type = c.typeExpr(m, f, member.ChildByField(syntax.FieldType), nil)
				c.strayVars(ev.Type, nil, func(string) string {
					return "an event's values cross the wire, so its type is named in full"
				})
			})
			continue
		}
		if member.Kind != syntax.CommandDeclaration && member.Kind != syntax.QueryDeclaration {
			continue
		}
		mn := member.ChildByField(syntax.FieldName)
		op := &ir.Op{Name: text(f, mn), Contract: ct, Command: member.Kind == syntax.CommandDeclaration, Pos: pos(f, member), Doc: docComment(f, member)}
		if args := attributeArgs(f, member, "reconcile"); len(args) == 1 {
			op.Reconcile = text(f, args[0])
			names[op] = args[0]
		}
		op.Fresh = op.Command && hasAttribute(f, member, "fresh")
		op.Destructive = op.Command && hasAttribute(f, member, "destructive")
		op.Key = hasAttribute(f, member, "key")
		if _, named, ok := attribute(f, member, "key"); ok {
			op.KeyHeader = c.stringArg(f, named["header"])
			op.KeyField = c.stringArg(f, named["field"])
		}
		if args, named, ok := attribute(f, member, "http"); ok {
			op.HTTP = c.opHTTP(f, member, args, named)
		}
		ct.Ops[op.Name] = op
		member := member
		m.decls = append(m.decls, func() {
			for _, p := range member.ChildByField(syntax.FieldParameters).NamedChildren() {
				param := &ir.Param{
					Name: text(f, p.ChildByField(syntax.FieldName)),
					Type: c.typeExpr(m, f, p.ChildByField(syntax.FieldType), nil),
				}
				if def := p.ChildByField(syntax.FieldDefault); def != nil {
					l := c.newFunc(m, f, ct.Name+"."+op.Name, pos(f, p), ctxGlobal)
					param.Default = l.expr(def)
				}
				op.Params = append(op.Params, param)
			}
			op.Return = c.typeExpr(m, f, member.ChildByField(syntax.FieldReturnType), nil)
			c.commandParams(f, member, op)
			wire := func(string) string {
				return "an operation's values cross the wire, so their types are named in full"
			}
			for _, p := range op.Params {
				c.strayVars(p.Type, nil, wire)
			}
			c.strayVars(op.Return, nil, wire)
		})
	}
	// What its attributes name exists, once its types are known.
	m.decls = append(m.decls, func() { c.contractNames(f, ct, names) })
	// A module that holds one contract offers its operations as its own:
	// `notify.send(…)` for `contract Notify { command send … }`.
	m.decls = append([]func(){func() {
		if len(m.mod.Contracts) != 1 {
			return
		}
		for name, op := range ct.Ops {
			if _, taken := m.values[name]; !taken {
				m.values[name] = op
			}
		}
		for name, ev := range ct.Events {
			if _, taken := m.values[name]; !taken {
				m.values[name] = ev
			}
		}
	}}, m.decls...)
}

// commandParams adds the parameters that @fresh and @destructive give a
// command, after the ones it declares (SPECS.md §17): `basedOn: Version`,
// the version of the state the command was based on, then `approval:
// Approval`. A @fresh command may also answer Stale or Expired, which join
// the variants of its result type.
func (c *checker) commandParams(f *ir.File, member *syntax.Node, op *ir.Op) {
	add := func(attr, name, typ string) {
		for _, p := range member.ChildByField(syntax.FieldParameters).NamedChildren() {
			if n := p.ChildByField(syntax.FieldName); text(f, n) == name {
				c.errorf(pos(f, n), "@%s adds the parameter %s, so %s can't declare one of that name", attr, name, op.Name)
				return
			}
		}
		op.Params = append(op.Params, &ir.Param{Name: name, Type: &ir.TypeExpr{Name: typ}})
	}
	if op.Fresh {
		add("fresh", "basedOn", "Version")
		ret := member.ChildByField(syntax.FieldReturnType)
		switch d := op.Return; {
		case d == nil:
		case d.Decl == nil || len(d.Decl.Variants) == 0:
			c.errorf(pos(f, ret), "@fresh adds Stale and Expired to %s's outcomes, so it returns a variant type, and %s isn't one", op.Name, text(f, ret))
		default:
			for _, v := range d.Decl.Variants {
				if v.Name == "Stale" || v.Name == "Expired" {
					c.errorf(pos(f, ret), "@fresh adds Stale and Expired to %s's outcomes, and %s already has a variant %s", op.Name, d.Decl.Name, v.Name)
				}
			}
		}
	}
	if op.Destructive {
		add("destructive", "approval", "Approval")
	}
}

// attributeArgs returns the arguments of a declaration's attribute @name.
// contractNames checks what a contract's attributes name: a command's
// @reconcile names a query of the contract that finds the command's result
// by its key, or the command itself, which the service answers again for
// the same key (SPECS.md §17); and an event's @key names a field of its
// type.
func (c *checker) contractNames(f *ir.File, ct *ir.Contract, names map[any]*syntax.Node) {
	for decl, n := range names {
		switch d := decl.(type) {
		case *ir.Op:
			q := ct.Ops[d.Reconcile]
			switch {
			case q == d:
			case q == nil:
				c.errorf(pos(f, n), "@reconcile names a query of %s, or the command itself, and %s has no operation %s", ct.Name, ct.Name, d.Reconcile)
			case q.Command:
				c.errorf(pos(f, n), "@reconcile names a query, or the command itself, and %s is another command", q.Name)
			case len(q.Params) != 1 || q.Params[0].Type == nil || q.Params[0].Type.Name != "Key" || q.Return == nil || q.Return.Name != "Option":
				c.errorf(pos(f, n), "@reconcile names a query that finds a command's result by its key: query %s(key: Key) -> Option(…)", q.Name)
			}
		case *ir.Event:
			if t := d.Type; t != nil && t.Decl != nil && t.Decl.Alias != nil && t.Decl.Alias.Name == "record" && !slices.Contains(t.Decl.Alias.Fields, d.Key) {
				c.errorf(pos(f, n), "@key names the field that tells %s's events apart, and %s has no field %s", d.Name, t.Decl.Name, d.Key)
			}
		}
	}
}

func attributeArgs(f *ir.File, d *syntax.Node, name string) []*syntax.Node {
	for _, attr := range d.Children {
		if attr.Kind == syntax.Attribute && text(f, attr.ChildByField(syntax.FieldName)) == name {
			if args := attr.ChildByField(syntax.FieldArguments); args != nil {
				return args.NamedChildren()
			}
		}
	}
	return nil
}

// attribute returns the positional and named arguments of a declaration's
// attribute @name, and whether it has one.
func attribute(f *ir.File, d *syntax.Node, name string) ([]*syntax.Node, map[string]*syntax.Node, bool) {
	for _, attr := range d.Children {
		if attr.Kind != syntax.Attribute || text(f, attr.ChildByField(syntax.FieldName)) != name {
			continue
		}
		var pos []*syntax.Node
		named := map[string]*syntax.Node{}
		if args := attr.ChildByField(syntax.FieldArguments); args != nil {
			for _, a := range args.NamedChildren() {
				if a.Kind == syntax.NamedArgument {
					named[text(f, a.ChildByField(syntax.FieldName))] = a.ChildByField(syntax.FieldValue)
				} else {
					pos = append(pos, a)
				}
			}
		}
		return pos, named, true
	}
	return nil, nil, false
}

// stringArg is the text of an attribute's string argument, or "" without one.
func (c *checker) stringArg(f *ir.File, n *syntax.Node) string {
	if n == nil {
		return ""
	}
	if n.Kind != syntax.String || len(n.NamedChildren()) > 1 {
		c.errorf(pos(f, n), "a string without interpolation goes here")
		return ""
	}
	return unescape(strings.Trim(text(f, n), `"`))
}

// opHTTP reads @http("METHOD /path", status: {200: Variant, …}) on an
// operation.
func (c *checker) opHTTP(f *ir.File, d *syntax.Node, args []*syntax.Node, named map[string]*syntax.Node) *ir.OpHTTP {
	if len(args) != 1 {
		c.errorf(pos(f, d), `@http takes the method and path: @http("POST /v1/send")`)
		return nil
	}
	method, path, ok := strings.Cut(c.stringArg(f, args[0]), " ")
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
	default:
		ok = false
	}
	if !ok || !strings.HasPrefix(path, "/") {
		c.errorf(pos(f, args[0]), `@http's first argument is a method and a path, such as "POST /v1/send"`)
		return nil
	}
	h := &ir.OpHTTP{Method: method, Path: path}
	if st := named["status"]; st != nil {
		if st.Kind != syntax.Map {
			c.errorf(pos(f, st), "status: takes a map from status codes to variants: {200: Accepted, 202: Pending}")
			return h
		}
		h.Status = map[int]string{}
		for _, e := range st.NamedChildren() {
			k, v := e.ChildByField(syntax.FieldKey), e.ChildByField(syntax.FieldValue)
			code, err := strconv.Atoi(text(f, k))
			if err != nil || v.Kind != syntax.TypeIdent {
				c.errorf(pos(f, e), "each status code maps to a variant's name: 202: Pending")
				continue
			}
			h.Status[code] = text(f, v)
		}
	}
	return h
}

// docComment returns the `##` lines directly above a node.
func docComment(f *ir.File, n *syntax.Node) string { return syntax.DocComment(n, f.Src) }

func hasAttribute(f *ir.File, d *syntax.Node, name string) bool {
	for _, attr := range d.Children {
		if attr.Kind == syntax.Attribute && text(f, attr.ChildByField(syntax.FieldName)) == name {
			return true
		}
	}
	return false
}

// contractNamed returns the contract a type expression names, if it names one.
func (c *checker) contractNamed(m *module, f *ir.File, n *syntax.Node) *ir.Contract {
	switch n.Kind {
	case syntax.TypeIdent:
		ct, _ := c.lookupTypeName(m, text(f, n)).(*ir.Contract)
		return ct
	case syntax.QualifiedType:
		imp, _ := m.values[text(f, n.ChildByField(syntax.FieldModule))].(*ir.Module)
		if imp == nil {
			return nil
		}
		ct, _ := c.modules[imp.Path].types[text(f, n.ChildByField(syntax.FieldName))].(*ir.Contract)
		return ct
	}
	return nil
}

// serve lowers `impl Contract { … }`: this program serves the contract, with
// one function per operation. A function takes the operation's parameters,
// with their types and defaults from the contract, and `key` last for a
// command with @key. It runs as one transaction, like an entity method.
func (c *checker) serve(m *module, f *ir.File, d *syntax.Node, ct *ir.Contract) {
	c.servedPort(m, f, d, ct)
	served := map[string]bool{}
	for _, member := range d.ChildByField(syntax.FieldBody).NamedChildren() {
		if member.Kind != syntax.FunctionDeclaration {
			continue
		}
		mn := member.ChildByField(syntax.FieldName)
		name := text(f, mn)
		op := ct.Ops[name]
		if op == nil {
			c.errorf(pos(f, mn), "contract %s has no operation %s", ct.Name, name)
			continue
		}
		if served[name] {
			c.errorf(pos(f, mn), "%s.%s is served twice", ct.Name, name)
			continue
		}
		served[name] = true
		c.use(pos(f, mn), op)
		fn := &ir.Func{Name: ct.Name + "." + name, Module: m.mod, Pos: pos(f, member)}
		op.Impl = fn
		member := member
		m.decls = append(m.decls, func() {
			// The contract's module is lowered first, so its parameters are known.
			var want, got []string
			for _, p := range op.Params {
				want = append(want, p.Name)
			}
			if op.Key {
				want = append(want, "key")
			}
			for _, p := range member.ChildByField(syntax.FieldParameters).NamedChildren() {
				got = append(got, text(f, p.ChildByField(syntax.FieldName)))
			}
			if strings.Join(got, ", ") != strings.Join(want, ", ") {
				c.errorf(pos(f, mn), "%s takes (%s), as contract %s declares it", name, strings.Join(want, ", "), ct.Name)
			}
			c.lowerFunc(m, f, member, fn, ctxOperation, nil, nil)
			for i, p := range fn.Params {
				if i < len(op.Params) && p.Default == nil {
					p.Default = op.Params[i].Default
				}
			}
		})
	}
	var missing []string
	for name := range ct.Ops {
		if !served[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		c.errorf(pos(f, d), "a program that serves %s serves every operation; this one doesn't serve %s", ct.Name, strings.Join(missing, ", "))
	}
}

// triggers reads a job's @on(enters(view)) and @on(leaves(view)) attributes.
func (c *checker) triggers(m *module, f *ir.File, d *syntax.Node, job *ir.Job) {
	for _, attr := range d.Children {
		if attr.Kind != syntax.Attribute || text(f, attr.ChildByField(syntax.FieldName)) != "on" {
			continue
		}
		args := attr.ChildByField(syntax.FieldArguments)
		if args == nil || len(args.NamedChildren()) != 1 {
			c.errorf(pos(f, attr), "@on takes one trigger, such as enters(view)")
			continue
		}
		call := args.NamedChildren()[0]
		if call.Kind == syntax.CallExpression {
			kind := text(f, call.ChildByField(syntax.FieldFunction))
			if kind == "enters" || kind == "leaves" || kind == "event" {
				// To the code that reads it, below.
				target := GoText{File: "check.go", Text: `(kind == "enters" || kind == "leaves") &&`}
				if kind == "event" {
					target.Text = `if kind == "event" &&`
				}
				c.use(pos(f, call.ChildByField(syntax.FieldFunction)), target)
			}
			viewArgs := call.ChildByField(syntax.FieldArguments).NamedChildren()
			if (kind == "enters" || kind == "leaves") && len(viewArgs) == 1 && viewArgs[0].Kind == syntax.Ident {
				v, ok := c.lookupValue(m, text(f, viewArgs[0])).(*ir.View)
				if !ok {
					c.errorf(pos(f, viewArgs[0]), "%s(…) takes a view", kind)
					continue
				}
				c.use(pos(f, viewArgs[0]), v)
				job.Triggers = append(job.Triggers, ir.Trigger{Kind: kind, View: v})
				continue
			}
			if kind == "event" && len(viewArgs) == 1 && viewArgs[0].Kind == syntax.FieldExpression {
				ev := c.eventNamed(m, f, viewArgs[0])
				if ev == nil {
					c.errorf(pos(f, viewArgs[0]), "event(…) takes a contract's event, such as telegram.updates")
					continue
				}
				job.Triggers = append(job.Triggers, ir.Trigger{Kind: kind, Event: ev})
				continue
			}
		}
		if call.Kind == syntax.CallExpression {
			switch name := text(f, call.ChildByField(syntax.FieldFunction)); name {
			case "daily", "weekdays", "every", "cron":
				if name == "cron" && !c.cronLine(f, call) {
					continue
				}
				// A schedule is computed once, when the program starts.
				l := c.newFunc(m, f, "schedule of "+job.Name, pos(f, call), ctxGlobal)
				l.fn.Body = l.expr(call)
				job.Triggers = append(job.Triggers, ir.Trigger{Kind: "schedule", Schedule: l.done()})
				continue
			}
		}
		c.errorf(pos(f, call), "@on takes a schedule, such as daily(07:40, in: zone), or enters(view), leaves(view) or event(contract.event)")
	}
}

// cronLine reads the line of `cron("…", in: zone)` when it is written out,
// so that its errors show at the string rather than when the program
// starts. It reports whether the line is good, or isn't written out.
func (c *checker) cronLine(f *ir.File, call *syntax.Node) bool {
	args := call.ChildByField(syntax.FieldArguments).NamedChildren()
	if len(args) == 0 || args[0].Kind != syntax.String || len(args[0].NamedChildren()) > 1 {
		return true
	}
	if _, err := cron.Parse(unescape(strings.Trim(text(f, args[0]), `"`))); err != nil {
		c.errorf(pos(f, args[0]), "%v", err)
		return false
	}
	return true
}

// eventNamed resolves `module.event` to a contract's event.
func (c *checker) eventNamed(m *module, f *ir.File, n *syntax.Node) *ir.Event {
	recv := n.ChildByField(syntax.FieldValue)
	if recv.Kind != syntax.Ident {
		return nil
	}
	imp, _ := m.values[text(f, recv)].(*ir.Module)
	if imp == nil {
		return nil
	}
	ev, _ := c.modules[imp.Path].values[text(f, n.ChildByField(syntax.FieldField))].(*ir.Event)
	if ev != nil {
		c.use(pos(f, recv), imp)
		c.use(pos(f, n.ChildByField(syntax.FieldField)), ev)
	}
	return ev
}

// lookupValue finds a lower-case name at module level.
func (c *checker) lookupValue(m *module, name string) any {
	if d, ok := m.values[name]; ok {
		return d
	}
	if d, ok := c.prelude.values[name]; ok {
		return d
	}
	if _, ok := ir.BuiltinModules[name]; ok {
		return ir.Builtin{Name: name}
	}
	for _, b := range ir.Builtins {
		if b == name {
			return ir.Builtin{Name: name}
		}
	}
	return nil
}

// lookupType finds an upper-case name: this module's, the prelude's, or a
// variant of an imported module when only one of them has that name.
func (c *checker) lookupType(m *module, name string) any {
	return c.lookupUpper(m, name, func(any) bool { return true })
}

// lookupTypeName finds a name written where a type goes. A variant isn't a
// type there unless it shares its type's name, `type Id = | Id(String)`, so
// the variant Int of `type Kind = Name | Int` leaves Int the built-in type
// in this module's types (SPECS.md §5).
func (c *checker) lookupTypeName(m *module, name string) any {
	return c.lookupUpper(m, name, namesType)
}

// namesType says whether d, found under its own name, stands for a type.
func namesType(d any) bool {
	v, ok := d.(*ir.Variant)
	return !ok || v.Name == v.Type.Name
}

// lookupUpper is lookupType, keeping only the declarations keep accepts.
func (c *checker) lookupUpper(m *module, name string, keep func(any) bool) any {
	if d, ok := m.types[name]; ok && keep(d) {
		return d
	}
	if d, ok := c.prelude.types[name]; ok && keep(d) {
		return d
	}
	var found []*ir.Variant
	var hidden any
	for _, imp := range m.imports {
		if v, ok := imp.types[name].(*ir.Variant); ok && keep(v) {
			if c.privateUse(m.mod, v, name) != "" {
				if hidden == nil {
					hidden = v // where it is used says it is private
				}
				continue
			}
			found = append(found, v)
		}
	}
	switch len(found) {
	case 0:
		return hidden
	case 1:
		return found[0]
	}
	return ambiguous{name, found}
}

// ambiguous is a variant name that several imported modules declare. Where
// a value is expected, the type expected there says which it is (SPECS.md
// §13); choices are the candidates.
type ambiguous struct {
	name    string
	choices []*ir.Variant
}

func (c *checker) lowerModule(m *module) {
	for i := 0; i < len(m.decls); i++ {
		m.decls[i]()
	}
	m.decls = nil
}
