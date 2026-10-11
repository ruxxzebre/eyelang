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
	"sort"
	"strconv"
	"strings"

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
	prelude := c.parse("prelude", "", []*ir.File{parseFile("<prelude>", preludeSrc)})
	c.prog.Prelude = prelude.mod
	c.prelude = prelude
	c.loadBuiltins()
	c.prog.Store = c.parse("runtime.store", "", []*ir.File{parseFile("<runtime store>", storeSrc)}).mod
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
	// name doesn't bring a crowd of type errors with it.
	if len(c.diags) == 0 {
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
	overlay map[string][]byte       // unsaved files, by path
	index   *Index                  // nil unless a tool asked for it
	all     bool                    // load every module below root, for tools
	names   names                   // where names are written, until types resolve them
	dyns    []dynUse                // each dyn Trait written, checked once traits are known
	typePos map[*ir.TypeExpr]ir.Pos // where each type as written is

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
	if err != nil {
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
		files = append(files, parseFile(p, src))
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
		files = append(files, parseFile(p, c.overlay[p]))
	}
	if len(files) == 0 {
		c.errorf(ir.Pos{}, "%s has no .eye files", dir)
		return nil
	}
	return c.parse(path, dir, files)
}

// parse collects the declarations of a module's files.
func (c *checker) parse(path, dir string, files []*ir.File) *module {
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
	}
	c.modules[path] = m
	for _, f := range files {
		for _, d := range f.Tree.Diagnostics {
			c.errorf(ir.Pos{File: f, Offset: d.Span.Start}, "%s", d.Message)
		}
	}
	// Imports first, so that their names are known when bodies are lowered.
	for _, f := range files {
		if len(f.Tree.Diagnostics) > 0 {
			continue
		}
		for _, d := range f.Tree.Root.Children {
			if d.Kind == syntax.ImportDeclaration {
				c.importDecl(m, f, d)
			}
		}
	}
	for _, f := range files {
		if len(f.Tree.Diagnostics) > 0 {
			continue
		}
		for _, d := range f.Tree.Root.Children {
			c.collect(m, f, d)
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
		imp = c.parse(path, "", []*ir.File{parseFile("<std "+path+">", src)})
	}
	if imp == nil {
		dir := filepath.Join(append([]string{c.root}, parts...)...)
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			c.errorf(pos(f, pathNode), "no module %s: there is no directory %s", path, dir)
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
			c.errorf(pos(f, item), "module %s has no %s", path, n)
			continue
		}
		c.use(pos(f, item), decl)
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
		fn := &ir.Func{Name: text(f, nameNode), Module: m.mod, Pub: hasToken(f, d, "pub"), Pos: pos(f, d)}
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
	}
}

// builtinTypes are the type names every module knows.
var builtinTypes = map[string]bool{
	"Int": true, "Float": true, "Bool": true, "Char": true, "String": true, "Byte": true, "Bytes": true,
	"Int32": true, "Int64": true, "UInt32": true, "UInt64": true, "NonZero": true, "Instant": true,
	"Date": true, "Time": true, "Duration": true, "Zone": true, "Schedule": true, "Size": true,
	"List": true, "Map": true, "Set": true, "Iter": true, "Ref": true, "Run": true, "Key": true,
	"Secret": true, "Approval": true, "Version": true, "Handle": true, "Range": true,
	"Event": true, "TaskGroup": true, "Sender": true, "Receiver": true, "Handler": true,
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
		d := c.lookupType(m, name)
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
			if !builtinTypes[name] {
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
			c.errorf(pos(f, n), "module %s has no type %s", mod, name)
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
			st := &ir.State{Name: text(f, member.ChildByField(syntax.FieldName))}
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
	ct := &ir.Contract{Name: text(f, nameNode), Module: m.mod, Ops: map[string]*ir.Op{}, Events: map[string]*ir.Event{}}
	m.mod.Contracts[ct.Name] = ct
	c.define(m, f, nameNode, ct.Name, ct)
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
	for _, member := range d.ChildByField(syntax.FieldBody).NamedChildren() {
		if member.Kind == syntax.EventDeclaration {
			ev := &ir.Event{
				Name:     text(f, member.ChildByField(syntax.FieldName)),
				Contract: ct,
				Topic:    strings.Trim(text(f, member.ChildByField(syntax.FieldTopic)), `"`),
			}
			if args := attributeArgs(f, member, "key"); len(args) == 1 {
				ev.Key = text(f, args[0])
			}
			ct.Events[ev.Name] = ev
			member := member
			m.decls = append(m.decls, func() { ev.Type = c.typeExpr(m, f, member.ChildByField(syntax.FieldType), nil) })
			continue
		}
		if member.Kind != syntax.CommandDeclaration && member.Kind != syntax.QueryDeclaration {
			continue
		}
		mn := member.ChildByField(syntax.FieldName)
		op := &ir.Op{Name: text(f, mn), Contract: ct, Command: member.Kind == syntax.CommandDeclaration, Pos: pos(f, member)}
		if args := attributeArgs(f, member, "reconcile"); len(args) == 1 {
			op.Reconcile = text(f, args[0])
		}
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
		})
	}
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

// attributeArgs returns the arguments of a declaration's attribute @name.
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
		ct, _ := c.lookupType(m, text(f, n)).(*ir.Contract)
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
			switch text(f, call.ChildByField(syntax.FieldFunction)) {
			case "daily", "weekdays", "every":
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
	if d, ok := m.types[name]; ok {
		return d
	}
	if d, ok := c.prelude.types[name]; ok {
		return d
	}
	var found any
	for _, imp := range m.imports {
		if v, ok := imp.types[name].(*ir.Variant); ok {
			if found != nil {
				return ambiguous{name}
			}
			found = v
		}
	}
	return found
}

// ambiguous is a variant name that several imported modules declare.
type ambiguous struct{ name string }

func (c *checker) lowerModule(m *module) {
	for i := 0; i < len(m.decls); i++ {
		m.decls[i]()
	}
	m.decls = nil
}
