package check

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

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
}

// Result is a checked program, its diagnostics, and an index of its names.
type Result struct {
	Program     *ir.Program
	Diagnostics []Diagnostic
	Index       *Index
}

// Check checks the program in dir, as Program does, and also indexes what
// each name refers to, for tools such as the language server.
func Check(dir string, opts Options) *Result {
	c := &checker{root: dir, modules: map[string]*module{}, overlay: opts.Overlay, index: &Index{}}
	path := ""
	if opts.Module != "" {
		rel, err := filepath.Rel(dir, opts.Module)
		if err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			path = strings.ReplaceAll(filepath.ToSlash(rel), "/", ".")
		}
	}
	prog, diags := c.program(path)
	return &Result{Program: prog, Diagnostics: diags, Index: c.index}
}

// Index records, for each name in a program's source, what it refers to.
type Index struct {
	Uses []Use
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
	if c.index != nil && at.File != nil && target != nil {
		c.index.Uses = append(c.index.Uses, Use{At: at, Target: target})
	}
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

// memberDefinition finds a record field, or an entity's state field or
// identity.
func memberDefinition(m Member) (ir.Pos, bool) {
	var at ir.Pos
	switch o := m.Of.(type) {
	case *ir.Type:
		at = o.Pos
	case *ir.Entity:
		at = o.Pos
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
