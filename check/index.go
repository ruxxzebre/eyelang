package check

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"eyelang/ir"
	"eyelang/syntax"
)

// Options change how Check reads a program.
type Options struct {
	// Overlay holds the text of files that differ from the disk, by path:
	// an editor's unsaved buffers.
	Overlay map[string][]byte
	// Module is the directory of the module to check, below the program's
	// directory, with what it imports. Empty means the program's own module.
	Module string
	// All checks every module below the program's directory too, not
	// only those the module imports: what finding references needs.
	All bool
}

// Result is a checked program, its diagnostics, and an index of its names.
type Result struct {
	Program     *ir.Program
	Diagnostics []Diagnostic
	Index       *Index
	c           *checker
}

// Check checks the program in dir, as Program does, and also indexes what
// each name refers to, for tools such as the language server.
func Check(dir string, opts Options) *Result {
	c := &checker{root: dir, modules: map[string]*module{}, overlay: opts.Overlay, index: &Index{}, all: opts.All}
	path := ""
	if opts.Module != "" {
		rel, err := filepath.Rel(dir, opts.Module)
		if err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			path = strings.ReplaceAll(filepath.ToSlash(rel), "/", ".")
		}
	}
	prog, diags := c.program(path)
	c.indexDeclarations()
	return &Result{Program: prog, Diagnostics: diags, Index: c.index, c: c}
}

// loadAll loads every module below the program's directory that isn't
// loaded yet, when the checker was asked for all of them. Directories whose
// names start with a dot, such as .eye and .git, are left out.
func (c *checker) loadAll() {
	if !c.all {
		return
	}
	_ = filepath.WalkDir(c.root, func(dir string, e os.DirEntry, err error) error {
		if err != nil || !e.IsDir() {
			return nil
		}
		if dir != c.root && strings.HasPrefix(e.Name(), ".") {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(c.root, dir)
		if err != nil {
			return nil
		}
		path := strings.ReplaceAll(filepath.ToSlash(rel), "/", ".")
		if rel == "." {
			path = "" // the program's own module
		}
		if c.modules[path] != nil || !hasEyeFiles(dir) {
			return nil
		}
		c.load(path, dir)
		return nil
	})
}

func hasEyeFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".eye") {
			return true
		}
	}
	return false
}

// indexDeclarations records each declaration's own name as a use of it, so
// that the name a declaration declares leads to it as its uses do.
func (c *checker) indexDeclarations() {
	if c.index == nil {
		return
	}
	c.indexRecords()
	add := func(target any) {
		if at, ok := Definition(target); ok && at.File != nil && at.End > at.Offset {
			c.use(at, target)
		}
	}
	for _, m := range c.prog.Modules {
		for _, g := range m.Globals {
			add(g)
		}
		for _, t := range m.Types {
			if len(t.Variants) == 1 && t.Variants[0].Name == t.Name {
				// `type Id = | Id(String)`: both names are the variant.
				if at, ok := variantName(t.Variants[0]); ok {
					c.use(at, t.Variants[0])
				}
			} else {
				add(t)
			}
			for _, v := range t.Variants {
				add(v)
				for _, f := range v.Fields {
					if f.Name != "" {
						add(Member{Of: v, Name: f.Name})
					}
				}
			}
			for _, f := range t.Fields {
				add(Member{Of: t, Name: f})
			}
			for _, fn := range t.Methods {
				add(fn)
			}
		}
		for _, fn := range m.Funcs {
			add(fn)
		}
		for _, e := range m.Entities {
			add(e)
			if e.ID != nil {
				add(Member{Of: e, Name: e.ID.Name})
			}
			for _, st := range e.States {
				add(Member{Of: e, Name: st.Name})
			}
			for _, fn := range e.Methods {
				add(fn)
			}
		}
		for _, j := range m.Jobs {
			add(j)
		}
		for _, tr := range m.Traits {
			add(tr)
			for _, fn := range tr.Methods {
				add(fn)
			}
		}
		for _, f := range m.Files {
			c.indexTypeVariables(f)
		}
		c.indexContracts(m)
	}
	for _, m := range c.prog.Modules {
		for _, f := range m.Files {
			c.indexAttributes(f)
		}
		for _, v := range m.Views {
			add(v)
		}
		for _, ct := range m.Contracts {
			add(ct)
			for _, op := range ct.Ops {
				add(op)
				for _, p := range op.Params {
					add(Member{Of: op, Name: p.Name})
				}
			}
			for _, ev := range ct.Events {
				add(ev)
			}
		}
	}
}

// References returns the uses of target, its declaration's name among them.
func (ix *Index) References(target any) []ir.Pos {
	var out []ir.Pos
	seen := map[ir.Pos]bool{}
	for _, u := range ix.Uses {
		if u.Target == target && !seen[u.At] {
			seen[u.At] = true
			out = append(out, u.At)
		}
	}
	return out
}

// BuiltinType is a type the checker knows without a declaration, such as
// Int, String or Zone.
type BuiltinType struct{ Name string }

// GoFile returns the path, as the compiler saw it, of a Go file of eye's
// source, by its path in the repository, such as "check/types.go".
func GoFile(rel string) string {
	_, self, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filepath.Dir(self)), filepath.FromSlash(rel))
}

// Index records, for each name in a program's source, what it refers to.
type Index struct {
	Uses  []Use
	types []typed // each checked expression's type, for TypeAt
}

// typed is an expression's type as checked. Its type variables are bound
// as checking goes on, so it is read once checking is done.
type typed struct {
	at ir.Pos
	t  typ
}

// Use is a name in the source and what it refers to.
type Use struct {
	At ir.Pos // the name
	// Target is a declaration (*ir.Func, *ir.Type, *ir.Variant, *ir.Entity,
	// *ir.Job, *ir.View, *ir.Contract, *ir.Op, *ir.Event, *ir.Global), an
	// imported *ir.Module, an ir.Builtin, a Local or a Member.
	Target any
}

// Local is a local variable or parameter, by where it is bound.
type Local struct{ At ir.Pos }

// Member is a field or state of a declaration, by name: a record field of an
// *ir.Type, or a state field or the identity of an *ir.Entity.
type Member struct {
	Of   any
	Name string
}

// At returns what the name at offset in the file at path refers to.
func (ix *Index) At(path string, offset int) (Use, bool) {
	for _, u := range ix.Uses {
		if u.At.File != nil && u.At.File.Path == path && u.At.Offset <= offset && offset <= u.At.End {
			return u, true
		}
	}
	return Use{}, false
}

// use records that the name at `at` refers to target.
func (c *checker) use(at ir.Pos, target any) {
	if l, ok := target.(Local); ok && l.At.File == nil {
		return
	}
	if c.index != nil && at.File != nil && target != nil {
		c.index.Uses = append(c.index.Uses, Use{At: at, Target: target})
	}
}

// typed records the type of the expression at at, for tools.
func (c *checker) typed(at ir.Pos, t typ) {
	if c.index != nil && at.File != nil && at.End > at.Offset {
		c.index.types = append(c.index.types, typed{at: at, t: t})
	}
}

// TypeAt returns the declaration of the type of the expression whose text
// ends at offset in the file at path, such as the receiver before a `.`:
// an *ir.Type, an *ir.Entity (a Ref(E) is E) or a BuiltinType. ok is false
// when no expression ends there, or its type is unknown or a type variable.
// Of the expressions that end at offset, the one that starts first counts,
// so the receiver of `a.b.` is a.b.
func (ix *Index) TypeAt(path string, offset int) (any, bool) {
	var best *typed
	for i := range ix.types {
		e := &ix.types[i]
		if e.at.End == offset && e.at.File.Path == path && (best == nil || e.at.Offset < best.at.Offset) {
			best = e
		}
	}
	if best == nil {
		return nil, false
	}
	return typeDecl(best.t)
}

// TypeText returns how the type of the expression spanning start to end is
// written, such as `List(String)`, for hover.
func (ix *Index) TypeText(path string, start, end int) (string, bool) {
	for _, e := range ix.types {
		if e.at.Offset == start && e.at.End == end && e.at.File.Path == path {
			switch t := prune(e.t).(type) {
			case tunknown, *tvar:
				return "", false
			default:
				return show(defaultLiteral(t)), true
			}
		}
	}
	return "", false
}

// typeDecl is the declaration a type names.
func typeDecl(t typ) (any, bool) {
	c, ok := prune(t).(*tcon)
	if !ok {
		return nil, false
	}
	switch {
	case c.entity != nil:
		return c.entity, true
	case c.name == "Ref" && c.decl == nil && len(c.args) == 1:
		return typeDecl(c.args[0])
	case c.decl != nil:
		return c.decl, true
	case builtinTypes[c.name]:
		return BuiltinType{Name: c.name}, true
	}
	return nil, false
}

// readFile reads a source file, from the overlay when it has the file.
func (c *checker) readFile(path string) ([]byte, error) {
	if src, ok := c.overlay[path]; ok {
		return src, nil
	}
	return os.ReadFile(path)
}

// Definition returns where target is declared: the span of its name. It has
// none for a built-in.
func Definition(target any) (ir.Pos, bool) {
	switch d := target.(type) {
	case Local:
		return d.At, d.At.File != nil
	case *ir.Module:
		if len(d.Files) == 0 {
			return ir.Pos{}, false
		}
		return ir.Pos{File: d.Files[0]}, true
	case *ir.Func:
		return named(d.Pos)
	case *ir.Type:
		return named(d.Pos)
	case ir.EnumFn:
		return named(d.Type.Pos) // derived: the type declares it
	case *ir.Trait:
		return named(d.Pos)
	case *ir.Entity:
		return named(d.Pos)
	case *ir.Job:
		return named(d.Pos)
	case *ir.View:
		return named(d.Pos)
	case *ir.Op:
		return named(d.Pos)
	case *ir.Test:
		return named(d.Pos)
	case *ir.Global:
		decl := nodeOf(d.Pos)
		if decl == nil {
			return d.Pos, d.Pos.File != nil
		}
		return nodePos(d.Pos.File, decl.ChildByField(syntax.FieldPattern), decl), true
	case *ir.Variant:
		if len(d.Type.Variants) == 1 && d.Type.Variants[0] == d && d.Type.Name == d.Name {
			return named(d.Type.Pos)
		}
		return variantName(d)
	case *ir.Contract:
		f, decl := contractNode(d)
		if decl == nil {
			return ir.Pos{}, false
		}
		return nodePos(f, decl.ChildByField(syntax.FieldName), decl), true
	case *ir.Event:
		f, decl := contractNode(d.Contract)
		if decl == nil {
			return ir.Pos{}, false
		}
		if n := memberNamed(f, decl.ChildByField(syntax.FieldBody), d.Name); n != nil {
			return nodePos(f, n.ChildByField(syntax.FieldName), n), true
		}
		return nodePos(f, decl.ChildByField(syntax.FieldName), decl), true
	case Member:
		return memberDefinition(d)
	}
	return ir.Pos{}, false
}

// variantName is the name of a variant where it is declared.
func variantName(d *ir.Variant) (ir.Pos, bool) {
	decl := nodeOf(d.Type.Pos)
	if decl == nil {
		return ir.Pos{}, false
	}
	if v := decl.ChildByField(syntax.FieldValue); v != nil && v.Kind == syntax.VariantList {
		if n := memberNamed(d.Type.Pos.File, v, d.Name); n != nil {
			return nodePos(d.Type.Pos.File, n.ChildByField(syntax.FieldName), n), true
		}
	}
	return named(d.Type.Pos)
}

// memberDefinition finds a record field, or an entity's state field or
// identity.
func memberDefinition(m Member) (ir.Pos, bool) {
	var at ir.Pos
	switch o := m.Of.(type) {
	case *ir.Type:
		at = o.Pos
	case *ir.Entity:
		at = o.Pos
	case *ir.Op:
		at = o.Pos
	case *ir.Variant:
		if vn, ok := variantName(o); ok {
			// The variant's node holds its payload: from its name up.
			if decl := nodeOf(o.Type.Pos); decl != nil {
				if v := decl.ChildByField(syntax.FieldValue); v != nil {
					if n := memberNamed(o.Type.Pos.File, v, o.Name); n != nil {
						at = pos(o.Type.Pos.File, n)
						break
					}
				}
			}
			return vn, true
		}
		return ir.Pos{}, false
	default:
		return ir.Pos{}, false
	}
	decl := nodeOf(at)
	if decl == nil {
		return ir.Pos{}, false
	}
	var found *syntax.Node
	decl.Walk(func(n *syntax.Node) bool {
		if found != nil {
			return false
		}
		switch n.Kind {
		case syntax.FieldDeclaration, syntax.StateDeclaration, syntax.Parameter:
			if name := n.ChildByField(syntax.FieldName); name != nil && name.Text(at.File.Src) == m.Name {
				found = name
				return false
			}
		case syntax.Block:
			return false // not into method bodies
		}
		return true
	})
	if found == nil {
		return named(at)
	}
	return nodePos(at.File, found, decl), true
}

// named returns the name of the declaration at `at`, or the declaration
// itself when it has no name field.
func named(at ir.Pos) (ir.Pos, bool) {
	if at.File == nil {
		return ir.Pos{}, false
	}
	decl := nodeOf(at)
	if decl == nil {
		return at, true
	}
	return nodePos(at.File, decl.ChildByField(syntax.FieldName), decl), true
}

// nodePos is the position of n, or of fallback when n is nil.
func nodePos(f *ir.File, n, fallback *syntax.Node) ir.Pos {
	if n == nil {
		n = fallback
	}
	return pos(f, n)
}

// nodeOf finds the syntax node a declaration's position spans.
func nodeOf(at ir.Pos) *syntax.Node {
	if at.File == nil || at.File.Tree == nil {
		return nil
	}
	var found *syntax.Node
	at.File.Tree.Root.Walk(func(n *syntax.Node) bool {
		if found != nil || n.Span.End < at.Offset || n.Span.Start > at.End {
			return false
		}
		if n.Span.Start == at.Offset && n.Span.End == at.End && !n.IsToken() {
			found = n
			return false
		}
		return true
	})
	return found
}

// memberNamed finds the child of n whose name field is name.
func memberNamed(f *ir.File, n *syntax.Node, name string) *syntax.Node {
	if n == nil {
		return nil
	}
	for _, c := range n.NamedChildren() {
		if nn := c.ChildByField(syntax.FieldName); nn != nil && nn.Text(f.Src) == name {
			return c
		}
	}
	return nil
}

// contractNode finds a contract's declaration in its module's files.
func contractNode(ct *ir.Contract) (*ir.File, *syntax.Node) {
	if ct == nil || ct.Module == nil {
		return nil, nil
	}
	for _, f := range ct.Module.Files {
		for _, d := range f.Tree.Root.Children {
			if d.Kind == syntax.ContractDeclaration {
				if n := d.ChildByField(syntax.FieldName); n != nil && n.Text(f.Src) == ct.Name {
					return f, d
				}
			}
		}
	}
	return nil, nil
}

// Root returns the directory of the program that the file or directory at
// path belongs to: the nearest directory, from path upward, that holds an
// eye.toml. Without one, a file belongs to the program in its own
// directory, and a directory is a program of its own.
func Root(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	start := abs
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		start = filepath.Dir(abs)
	}
	for dir := start; ; {
		if info, err := os.Stat(filepath.Join(dir, "eye.toml")); err == nil && !info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return start
		}
		dir = parent
	}
}

// Embedded returns the source of a file the checker carries inside it,
// "<prelude>" or "<std ai>", and where that file is in the repository the
// checker was built from. The file may not be there: a binary can outlive
// its source.
func Embedded(path string) (repoPath string, src []byte, ok bool) {
	_, self, _, _ := runtime.Caller(0)
	dir := filepath.Dir(self)
	switch path {
	case "<prelude>":
		return filepath.Join(dir, "prelude.eye"), preludeSrc, true
	case "<runtime store>":
		return filepath.Join(dir, "runtime", "store.eye"), storeSrc, true
	case "<builtin>":
		return GoFile("ir/builtin.eye"), ir.BuiltinSource, true
	}
	if name, found := strings.CutPrefix(path, "<std "); found {
		name = strings.TrimSuffix(name, ">")
		src, err := std.ReadFile("std/" + name + ".eye")
		if err != nil {
			return "", nil, false
		}
		return filepath.Join(dir, "std", name+".eye"), src, true
	}
	return "", nil, false
}

// builtinFile is ir/builtin.eye, parsed once.
var builtinFile = sync.OnceValue(func() *ir.File {
	return parseFile("<builtin>", ir.BuiltinSource)
})

// BuiltinDeclaration returns where ir/builtin.eye declares the built-in
// function name (print, log.info), or the built-in type when isType is set:
// the name in its documented declaration.
func BuiltinDeclaration(name string, isType bool) (ir.Pos, bool) {
	f := builtinFile()
	for _, d := range f.Tree.Root.Children {
		n := d.ChildByField(syntax.FieldName)
		if n == nil || isType != (d.Kind == syntax.ExternTypeDeclaration) {
			continue
		}
		if d.Kind == syntax.ExternDeclaration {
			if args := attributeArgs(f, d, "host"); len(args) == 1 && strings.Trim(text(f, args[0]), `"`) == name {
				return pos(f, n), true
			}
		}
		if text(f, n) == name {
			return pos(f, n), true
		}
	}
	return ir.Pos{}, false
}

// GoText is a place in eye's Go source, found by its text: where a built-in
// attribute, trigger or method is declared. File is relative to the
// repository, as "check/attributes.go".
type GoText struct{ File, Text string }

// method is where the type checker handles a method it knows itself, such
// as `run.join()` or `date.format(…)`.
func method(name string) GoText {
	return GoText{File: "check/infer.go", Text: `e.Name == "` + name + `"`}
}

// names keeps where the names in an expression are written, from lowering
// until the type checker knows what they refer to.
type names struct {
	name   map[ir.Expr]ir.Pos            // of a Field or MethodCall
	args   map[ir.Expr]map[string]ir.Pos // named arguments of a call
	fields map[ir.Expr][]ir.Pos          // of a Record or RecordUpdate, by index
	place  map[ir.Pos][]ir.Pos           // of an assignment target's path, by its position
}

// nameAt records where the name of a Field or MethodCall is written.
func (c *checker) nameAt(e ir.Expr, at ir.Pos) {
	if c.index == nil {
		return
	}
	if c.names.name == nil {
		c.names.name = map[ir.Expr]ir.Pos{}
	}
	c.names.name[e] = at
}

// argsAt records where a call's named arguments are named.
func (c *checker) argsAt(e ir.Expr, f *ir.File, args *syntax.Node) {
	if c.index == nil || args == nil {
		return
	}
	for _, a := range args.NamedChildren() {
		if a.Kind != syntax.NamedArgument {
			continue
		}
		if c.names.args == nil {
			c.names.args = map[ir.Expr]map[string]ir.Pos{}
		}
		if c.names.args[e] == nil {
			c.names.args[e] = map[string]ir.Pos{}
		}
		name := a.ChildByField(syntax.FieldName)
		c.names.args[e][text(f, name)] = pos(f, name)
	}
}

// fieldsAt records where a record literal's or update's field names are.
func (c *checker) fieldsAt(e ir.Expr, at []ir.Pos) {
	if c.index == nil {
		return
	}
	if c.names.fields == nil {
		c.names.fields = map[ir.Expr][]ir.Pos{}
	}
	c.names.fields[e] = at
}

// useName records what the name of a Field or MethodCall refers to.
func (c *checker) useName(e ir.Expr, target any) {
	if at, ok := c.names.name[e]; ok {
		c.use(at, target)
	}
	if call, ok := e.(*ir.MethodCall); ok {
		// The typed IR keeps what the call reaches (7.1).
		switch t := target.(type) {
		case *ir.Func, ir.Builtin, ir.EnumFn:
			call.Target = t
		case GoText:
			call.Target = ir.RuntimeMethod{Name: call.Name}
		}
	}
}

// useMethod records the method a MethodCall calls, and what its named
// arguments name.
func (c *checker) useMethod(e ir.Expr, m *ir.Func) {
	c.useName(e, m)
	c.useArgs(e, m)
}

// useArgs records the parameters a call's named arguments name.
func (c *checker) useArgs(e ir.Expr, callee any) {
	for name, at := range c.names.args[e] {
		if p := paramOf(callee, name); p != nil {
			c.use(at, p)
		}
	}
}

// paramOf is the target a named argument of a call of callee names.
func paramOf(callee any, name string) any {
	switch d := callee.(type) {
	case *ir.Job:
		if p := paramOf(d.Func, name); p != nil {
			return p
		}
		return method("start") // after:, at: and key:, which start adds
	case *ir.Func:
		decl := nodeOf(d.Pos)
		if decl == nil {
			return nil
		}
		if ps := decl.ChildByField(syntax.FieldParameters); ps != nil {
			for _, p := range ps.NamedChildren() {
				if n := p.ChildByField(syntax.FieldName); n != nil && text(d.Pos.File, n) == name {
					return Local{At: pos(d.Pos.File, n)}
				}
			}
		}
	case *ir.Op:
		return Member{Of: d, Name: name}
	case *ir.Variant:
		return Member{Of: d, Name: name}
	case *ir.Entity:
		if d.ID != nil && d.ID.Name == name {
			return Member{Of: d, Name: name}
		}
	case ir.Builtin, GoText:
		return d // its parameters are the Go function's
	}
	return nil
}

// placeAt records where the names in an assignment's target are written:
// each field of its path, the state field first when it starts at one.
func (c *checker) placeAt(p ir.Pos, names []ir.Pos) {
	if c.index == nil {
		return
	}
	if c.names.place == nil {
		c.names.place = map[ir.Pos][]ir.Pos{}
	}
	c.names.place[p] = names
}

// usePlace records what the i-th name of the target at p refers to.
func (c *checker) usePlace(p ir.Pos, i int, target any) {
	if at := c.names.place[p]; i < len(at) {
		c.use(at[i], target)
	}
}

// useField records the field a Field reads, from the type of its value.
func (c *checker) useField(e ir.Expr, t typ, name string) {
	if owner := c.memberOwner(t, name); owner != nil {
		c.useName(e, Member{Of: owner, Name: name})
	} else if x, ok := prune(t).(*tcon); ok && x.name == "Approval" && x.decl == nil {
		// ok.what and ok.by lead to Approval's declaration, which says
		// what they hold.
		c.useName(e, BuiltinType{Name: "Approval"})
	}
}

// useFields records the fields a record literal or update names.
func (c *checker) useFields(e ir.Expr, t typ, names []string) {
	at := c.names.fields[e]
	for i, name := range names {
		if i < len(at) && at[i].File != nil {
			if owner := c.memberOwner(t, name); owner != nil {
				c.use(at[i], Member{Of: owner, Name: name})
			}
		}
	}
}

// memberOwner is the declaration that declares the field name of a value of
// type t: an entity, a record type, or the one variant of a type.
func (c *checker) memberOwner(t typ, name string) any {
	x, ok := prune(t).(*tcon)
	if !ok {
		return nil
	}
	switch {
	case x.entity != nil:
		return x.entity
	case x.name == "Ref" && x.decl == nil && len(x.args) == 1:
		if s, ok := prune(x.args[0]).(*tcon); ok && s.entity != nil {
			return s.entity
		}
	case x.decl != nil && x.decl.Alias != nil:
		return x.decl
	case x.decl != nil && len(x.decl.Variants) == 1:
		return x.decl.Variants[0]
	}
	return nil
}

// indexRecords records the fields of record literals whose type the checker
// found.
func (c *checker) indexRecords() {
	for e, at := range c.names.fields {
		r, ok := e.(*ir.Record)
		if !ok || r.Type == nil {
			continue
		}
		for i, name := range r.Names {
			if i < len(at) && at[i].File != nil {
				c.use(at[i], Member{Of: r.Type, Name: name})
			}
		}
	}
}

// indexTypeVariables records the type variables of each declaration: the
// first place a name appears, or its entry in the type parameters, declares
// it, and the others use it. A trait's or an impl's variables are seen in
// their methods.
func (c *checker) indexTypeVariables(f *ir.File) {
	var walk func(n *syntax.Node, scopes []map[string]ir.Pos)
	declare := func(scopes []map[string]ir.Pos, n *syntax.Node) {
		name := text(f, n)
		for i := len(scopes) - 1; i >= 0; i-- {
			if at, ok := scopes[i][name]; ok {
				c.use(pos(f, n), Local{At: at})
				return
			}
		}
		at := pos(f, n)
		scopes[len(scopes)-1][name] = at
		c.use(at, Local{At: at})
	}
	walk = func(n *syntax.Node, scopes []map[string]ir.Pos) {
		switch n.Kind {
		case syntax.TypeDeclaration, syntax.TraitDeclaration, syntax.ImplDeclaration, syntax.FunctionDeclaration,
			syntax.FunctionSignature, syntax.ExternDeclaration:
			scopes = append(scopes, map[string]ir.Pos{})
		case syntax.Block:
			return // bodies hold no type variables of their own
		case syntax.TypeVariable:
			inner := n
			if kids := n.NamedChildren(); len(kids) == 1 && kids[0].Kind == syntax.Ident {
				inner = kids[0]
			}
			if len(scopes) > 0 {
				declare(scopes, inner)
			}
			return
		case syntax.TypeParameters:
			for _, p := range n.NamedChildren() {
				if p.Kind == syntax.Ident && len(scopes) > 0 {
					declare(scopes, p)
				}
			}
			return
		case syntax.Requirement:
			if name := n.ChildByField(syntax.FieldName); name != nil && name.Kind == syntax.Ident && len(scopes) > 0 {
				declare(scopes, name)
			}
		}
		for _, k := range n.Children {
			walk(k, scopes)
		}
	}
	walk(f.Tree.Root, nil)
}

// indexContracts records the names a contract's attributes give:
// @reconcile(query), @key(field) on an event, and the variants of
// @http(status: {…}).
func (c *checker) indexContracts(m *ir.Module) {
	mod := c.modules[m.Path]
	for _, ct := range m.Contracts {
		f, decl := contractNode(ct)
		if decl == nil || mod == nil {
			continue
		}
		for _, member := range decl.ChildByField(syntax.FieldBody).NamedChildren() {
			name := text(f, member.ChildByField(syntax.FieldName))
			if args := attributeArgs(f, member, "reconcile"); len(args) == 1 {
				if q := ct.Ops[text(f, args[0])]; q != nil {
					c.use(pos(f, args[0]), q)
				}
			}
			if ev := ct.Events[name]; ev != nil && ev.Type != nil && ev.Type.Decl != nil {
				if args := attributeArgs(f, member, "key"); len(args) == 1 {
					c.use(pos(f, args[0]), Member{Of: ev.Type.Decl, Name: text(f, args[0])})
				}
			}
			if _, named, ok := attribute(f, member, "http"); ok && named["status"] != nil && named["status"].Kind == syntax.Map {
				for _, e := range named["status"].NamedChildren() {
					if v := e.ChildByField(syntax.FieldValue); v != nil && v.Kind == syntax.TypeIdent {
						if d, ok := c.lookupType(mod, text(f, v)).(*ir.Variant); ok {
							c.use(pos(f, v), d)
						}
					}
				}
			}
		}
	}
}

// indexAttributes leads the names in a built-in attribute's arguments that
// name nothing in the program, such as `snake` in @http(names: snake), to
// the attribute's declaration.
func (c *checker) indexAttributes(f *ir.File) {
	f.Tree.Root.Walk(func(n *syntax.Node) bool {
		if n.Kind != syntax.Attribute {
			return true
		}
		name := text(f, n.ChildByField(syntax.FieldName))
		args := n.ChildByField(syntax.FieldArguments)
		if _, known := attributes[name]; !known || args == nil {
			return false
		}
		spec := GoText{File: "check/attributes.go", Text: `"` + name + `": {`}
		for _, a := range args.NamedChildren() {
			if a.Kind == syntax.NamedArgument {
				a = a.ChildByField(syntax.FieldValue)
			}
			if a == nil || a.Kind != syntax.Ident {
				continue
			}
			if _, done := c.index.At(f.Path, a.Span.Start); !done {
				c.use(pos(f, a), spec)
			}
		}
		return false
	})
}

// moduleAt is the module of the file at path.
func (r *Result) moduleAt(path string) *module {
	if r.c == nil {
		return nil
	}
	for _, mod := range r.c.modules {
		for _, f := range mod.mod.Files {
			if f.Path == path {
				return mod
			}
		}
	}
	return nil
}

// FuncsOn returns, by name, the functions `x.f(…)` can call on the
// expression whose text ends at offset in the file at path, as the receiver
// before a `.` (SPECS.md §8): the functions declared with fn in scope there
// whose first parameter takes the expression's type, each an *ir.Func, and
// the built-ins that take it, each an ir.Builtin. A function of the program
// hides a built-in of its name. It returns nothing when the type isn't
// known. The methods and fields of the type aren't among them.
func (r *Result) FuncsOn(path string, offset int) map[string]any {
	m := r.moduleAt(path)
	if m == nil || r.Index == nil {
		return nil
	}
	var best *typed
	for i := range r.Index.types {
		e := &r.Index.types[i]
		if e.at.End == offset && e.at.File.Path == path && (best == nil || e.at.Offset < best.at.Offset) {
			best = e
		}
	}
	if best == nil {
		return nil
	}
	t := prune(best.t)
	switch v := defaultLiteral(t).(type) {
	case tunknown:
		return nil
	case *tvar:
		if !v.rigid {
			return nil
		}
	}
	c := r.c
	out := map[string]any{}
	for name, v := range m.values {
		if fn, ok := v.(*ir.Func); ok && !fn.HasSelf && fn.Receiver == nil && fn.Entity == nil && c.takesFirst(fn, t) {
			out[name] = fn
		}
	}
	builtin := func(name string) {
		if _, taken := m.values[name]; !taken {
			out[name] = ir.Builtin{Name: name}
		}
	}
	for name, fn := range c.builtins.funcs {
		if !strings.Contains(name, ".") && c.takesFirst(fn, t) {
			builtin(name)
		}
	}
	// The collection functions and the fixed-width methods are typed by
	// rules rather than signatures: they take the kinds their rules say.
	kind, _ := (&fnCheck{c: c}).collection(t)
	fixed := false
	if k, ok := defaultLiteral(t).(*tcon); ok && k.decl == nil {
		fixed = fixedRanges[k.name] != nil
	}
	for name := range c.builtins.rules {
		if _, op := fixedOps[name]; op {
			if fixed {
				builtin(name)
			}
			continue
		}
		kinds, ok := collectionKinds[name]
		if !ok {
			if !goRuleCollection[name] {
				continue
			}
			kinds = elementKinds
		}
		if kind != "" && slices.Contains(kinds, kind) {
			builtin(name)
		}
	}
	return out
}

// Lookup finds what name means in the module of the file at path, as code
// there would: a declaration of the module, an imported one (`notify.send`,
// `notify.Delivery`), the prelude's, or a built-in. Tools use it for names
// written in comments.
func (r *Result) Lookup(path, name string) any {
	m := r.moduleAt(path)
	if m == nil {
		return nil
	}
	if first, rest, ok := strings.Cut(name, "."); ok {
		switch d := r.c.lookupValue(m, first).(type) {
		case *ir.Module:
			imp := r.c.modules[d.Path]
			if imp == nil {
				return nil
			}
			if v, ok := imp.values[rest]; ok {
				return v
			}
			return imp.types[rest]
		case ir.Builtin:
			return ir.Builtin{Name: name}
		}
		if t, ok := r.c.lookupTypeName(m, first).(*ir.Type); ok {
			if fn, ok := t.Methods[rest]; ok {
				return fn
			}
		}
		return nil
	}
	if name != "" && name[0] >= 'A' && name[0] <= 'Z' {
		d := r.c.lookupType(m, name)
		if _, ok := d.(ambiguous); ok || d == nil {
			if builtinTypes[name] {
				return BuiltinType{Name: name}
			}
			return nil
		}
		return d
	}
	return r.c.lookupValue(m, name)
}
