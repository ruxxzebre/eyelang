package lsp

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"eyelang/check"
	"eyelang/interp"
	"eyelang/ir"
	"eyelang/syntax"
)

// location is an LSP location: a range in a file.
type location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// definition answers textDocument/definition: where the name at p is
// declared. A name declared in Eyelang goes to its declaration, in this
// program, an imported module, the prelude or a standard module. A built-in
// function goes to the Go that implements it, and so does the name of an
// extern fn whose @host the interpreter provides.
func (s *server) definition(d *document, p Position) (any, *responseError) {
	path, ok := uriPath(d.uri)
	if !ok {
		return nil, nil
	}
	offset := d.offset(p)
	res := s.checked(path)
	if use, ok := res.Index.At(path, offset); ok {
		if loc, ok := s.targetLocation(use.Target); ok {
			return []location{loc}, nil
		}
		return nil, nil
	}
	if loc, ok := s.hostLocation(d, offset); ok {
		return []location{loc}, nil
	}
	if word := commentWord(d, offset); word != "" {
		if target := res.Lookup(path, word); target != nil {
			if loc, ok := s.targetLocation(target); ok {
				return []location{loc}, nil
			}
		}
		return nil, nil
	}
	if loc, ok := s.keywordDefinition(d, offset); ok {
		return []location{loc}, nil
	}
	if n := declaredName(d, offset); n != nil {
		// A declaration's own name: editors then offer its references.
		return []location{{URI: d.uri, Range: d.rangeOf(n.Span)}}, nil
	}
	return nil, nil
}

// declaredName returns the name a declaration, parameter or binding declares
// at offset.
func declaredName(d *document, offset int) *syntax.Node {
	var found *syntax.Node
	d.file.Root.Walk(func(n *syntax.Node) bool {
		if found != nil || offset < n.Span.Start || offset > n.Span.End {
			return false
		}
		if name := n.ChildByField(syntax.FieldName); name != nil && name.IsToken() &&
			name.Span.Start <= offset && offset <= name.Span.End {
			found = name
			return false
		}
		if p := n.ChildByField(syntax.FieldPattern); p != nil && p.Kind == syntax.Ident &&
			p.Span.Start <= offset && offset <= p.Span.End {
			found = p
			return false
		}
		return true
	})
	return found
}

// checked checks the module of the file at path, with what it imports from
// the program it belongs to, and the open documents in place of what is on
// disk.
func (s *server) checked(path string) *check.Result {
	root := check.Root(path)
	return check.Check(root, check.Options{Overlay: s.overlay(root), Module: filepath.Dir(path)})
}

// overlay holds the text of the open documents below root.
func (s *server) overlay(root string) map[string][]byte {
	overlay := map[string][]byte{}
	for uri, d := range s.docs {
		if p, ok := uriPath(uri); ok && strings.HasPrefix(p, root+string(filepath.Separator)) {
			overlay[p] = d.text
		}
	}
	return overlay
}

// targetLocation returns where a use's target is declared.
func (s *server) targetLocation(target any) (location, bool) {
	switch t := target.(type) {
	case check.BuiltinType:
		// Where the checker makes the type: `tString = con("String")`,
		// `con("List", t)`, or else its entry among the built-in names.
		for _, g := range []check.GoText{
			{File: "check/types.go", Text: `con("` + t.Name + `"`},
			{File: "check/builtintypes.go", Text: `con("` + t.Name + `"`},
			{File: "check/check.go", Text: `"` + t.Name + `": true`},
		} {
			if loc, ok := s.goTextLocation(g); ok {
				return loc, true
			}
		}
		return location{}, false
	case check.GoText:
		return s.goTextLocation(t)
	case ir.Builtin:
		src, ok := interp.BuiltinSource(t.Name)
		if !ok {
			return location{}, false
		}
		return s.goLocation(src)
	}
	at, ok := check.Definition(target)
	if !ok {
		return location{}, false
	}
	return s.posLocation(at)
}

// goTextLocation finds the first line of a Go file of eye's source that
// holds the text.
func (s *server) goTextLocation(g check.GoText) (location, bool) {
	path, ok := s.sourceFile(check.GoFile(g.File))
	if !ok {
		return location{}, false
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return location{}, false
	}
	for i, line := range strings.Split(string(src), "\n") {
		if c := strings.Index(line, g.Text); c >= 0 {
			start := Position{Line: i, Character: utf16Len([]byte(line[:c]))}
			end := Position{Line: i, Character: start.Character + utf16Len([]byte(g.Text))}
			return location{URI: pathURI(path), Range: Range{Start: start, End: end}}, true
		}
	}
	return location{}, false
}

// references answers textDocument/references: every use of the name at p
// in the program the file belongs to, across all its modules. A name
// declared in the prelude or a standard module, or built in, is looked for
// in every program in the workspace.
func (s *server) references(d *document, p Position, withDecl bool) (any, *responseError) {
	out := []location{}
	path, ok := uriPath(d.uri)
	if !ok {
		return out, nil
	}
	root := check.Root(path)
	res := s.checkAll(root, filepath.Dir(path))
	use, ok := res.Index.At(path, d.offset(p))
	if !ok {
		return out, nil
	}
	key, global := s.targetKey(use.Target)
	results := []*check.Result{res}
	if global {
		for _, r := range s.programs() {
			if r != root {
				results = append(results, s.checkAll(r, ""))
			}
		}
	}
	decl, _ := s.targetLocation(use.Target)
	seen := map[location]bool{}
	for _, res := range results {
		for _, u := range res.Index.Uses {
			if k, _ := s.targetKey(u.Target); k != key {
				continue
			}
			loc, ok := s.posLocation(u.At)
			if !ok || seen[loc] || !withDecl && loc == decl {
				continue
			}
			seen[loc] = true
			out = append(out, loc)
		}
	}
	// What the prelude and std declare, the runtime's Go often makes.
	if at, ok := check.Definition(use.Target); ok && strings.HasPrefix(key, "<") {
		for _, loc := range s.goSites(at.Text()) {
			if !seen[loc] {
				seen[loc] = true
				out = append(out, loc)
			}
		}
	}
	return out, nil
}

// checkAll checks a program, every module of it when it has an eye.toml,
// with module, if given, among them.
func (s *server) checkAll(root, module string) *check.Result {
	opts := check.Options{Overlay: s.overlay(root), Module: module}
	if _, err := os.Stat(filepath.Join(root, "eye.toml")); err == nil {
		opts.All = true // without one, the program is its own directory only
	}
	return check.Check(root, opts)
}

// targetKey names a target the same way in every check of every program:
// by where it is declared, a file eye carries by its name in eye, or by
// what it is when it is built in. global reports whether programs other than
// this one can use it.
func (s *server) targetKey(target any) (key string, global bool) {
	switch t := target.(type) {
	case check.BuiltinType, check.GoText, ir.Builtin:
		return fmt.Sprintf("%T %v", t, t), true
	}
	at, ok := check.Definition(target)
	if !ok || at.File == nil {
		return fmt.Sprintf("%T %p", target, target), false
	}
	path := embeddedName(at.File.Path)
	return fmt.Sprintf("%s:%d", path, at.Offset), strings.HasPrefix(path, "<")
}

// embeddedName is the name eye gives a file it carries, for the copy of it
// in the repository, so that a declaration in check/prelude.eye is the
// prelude's.
func embeddedName(path string) string {
	p := filepath.ToSlash(path)
	switch {
	case strings.HasSuffix(p, "/check/prelude.eye"):
		return "<prelude>"
	case strings.HasSuffix(p, "/check/runtime/store.eye"):
		return "<runtime store>"
	}
	if i := strings.LastIndex(p, "/check/std/"); i >= 0 && strings.HasSuffix(p, ".eye") {
		return "<std " + strings.TrimSuffix(p[i+len("/check/std/"):], ".eye") + ">"
	}
	return path
}

// programs returns the roots of the programs in the workspace: the
// directories that hold an eye.toml, outside hidden directories and
// node_modules.
func (s *server) programs() []string {
	var out []string
	for _, folder := range s.folders {
		_ = filepath.WalkDir(folder, func(p string, e os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if e.IsDir() && p != folder && (strings.HasPrefix(e.Name(), ".") || e.Name() == "node_modules") {
				return filepath.SkipDir
			}
			if !e.IsDir() && e.Name() == "eye.toml" {
				out = append(out, filepath.Dir(p))
			}
			return nil
		})
	}
	return out
}

// hostLocation returns the Go implementation of the extern fn whose name, or
// @host attribute, is at offset.
func (s *server) hostLocation(d *document, offset int) (location, bool) {
	for _, decl := range d.file.Root.Children {
		if decl.Kind != syntax.ExternDeclaration || offset < decl.Span.Start || offset > decl.Span.End {
			continue
		}
		name := decl.ChildByField(syntax.FieldName)
		for _, attr := range decl.Children {
			if attr.Kind != syntax.Attribute || attr.ChildByField(syntax.FieldName).Text(d.text) != "host" {
				continue
			}
			args := attr.ChildByField(syntax.FieldArguments)
			if args == nil || len(args.NamedChildren()) != 1 {
				continue
			}
			arg := args.NamedChildren()[0]
			within := func(n *syntax.Node) bool { return n != nil && n.Span.Start <= offset && offset <= n.Span.End }
			if !within(name) && !within(attr) {
				continue
			}
			if src, ok := interp.HostSource(strings.Trim(arg.Text(d.text), `"`)); ok {
				return s.goLocation(src)
			}
		}
	}
	return location{}, false
}

// posLocation turns a checker position into a location. A file the checker
// carries inside it, such as the prelude, is found in the repository it was
// built from, or else written out to the user's cache.
func (s *server) posLocation(at ir.Pos) (location, bool) {
	if at.File == nil {
		return location{}, false
	}
	path := at.File.Path
	if strings.HasPrefix(path, "<") {
		p, ok := s.embeddedFile(path)
		if !ok {
			return location{}, false
		}
		path = p
	}
	src := &document{text: at.File.Src, file: at.File.Tree}
	r := Range{Start: src.position(at.Offset), End: src.position(max(at.Offset, at.End))}
	return location{URI: pathURI(path), Range: r}, true
}

// embeddedFile returns a file on disk that holds the embedded file name.
func (s *server) embeddedFile(name string) (string, bool) {
	repoPath, src, ok := check.Embedded(name)
	if !ok {
		return "", false
	}
	if p, ok := s.sourceFile(repoPath); ok {
		if on, err := os.ReadFile(p); err == nil && bytes.Equal(on, src) {
			return p, true
		}
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", false
	}
	sum := sha256.Sum256(src)
	path := filepath.Join(cache, "eyelang", "embedded", hex.EncodeToString(sum[:6]), filepath.Base(repoPath))
	if _, err := os.Stat(path); err == nil {
		return path, true
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", false
	}
	if err := os.WriteFile(path, src, 0o444); err != nil {
		return "", false
	}
	return path, true
}

// goLocation is the start of a line of Go, when the file is there to open.
func (s *server) goLocation(src interp.GoSource) (location, bool) {
	path, ok := s.sourceFile(src.File)
	if !ok {
		return location{}, false
	}
	at := Position{Line: src.Line - 1}
	return location{URI: pathURI(path), Range: Range{Start: at, End: at}}, true
}

// sourceFile finds a file of eye's own source. A binary built with -trimpath
// knows it only as eyelang/interp/builtins.go, which is looked up in each
// workspace folder that holds the eyelang module.
func (s *server) sourceFile(path string) (string, bool) {
	if filepath.IsAbs(path) {
		_, err := os.Stat(path)
		return path, err == nil
	}
	rel, ok := strings.CutPrefix(filepath.ToSlash(path), "eyelang/")
	if !ok {
		return "", false
	}
	for _, dir := range s.folders {
		mod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err != nil || !strings.HasPrefix(string(mod), "module eyelang\n") {
			continue
		}
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}

// uriPath returns the path of a file: URI.
func uriPath(uri string) (string, bool) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return "", false
	}
	p := u.Path
	if runtime.GOOS == "windows" {
		p = strings.TrimPrefix(p, "/")
	}
	return filepath.Clean(filepath.FromSlash(p)), true
}

// pathURI returns the file: URI of a path.
func pathURI(path string) string {
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

// implementation answers textDocument/implementation. For a keyword or an
// operator it lists every place eye implements it: the checker, the
// interpreter, the Go parser and the grammar. For a name it is the
// definition.
func (s *server) implementation(d *document, p Position) (any, *responseError) {
	if locs := s.keywordImplementations(d, d.offset(p)); len(locs) > 0 {
		return locs, nil
	}
	// A name the prelude or std declares: where the runtime's Go makes it.
	if path, ok := uriPath(d.uri); ok {
		if use, ok := s.checked(path).Index.At(path, d.offset(p)); ok {
			if at, ok := check.Definition(use.Target); ok && at.File != nil && strings.HasPrefix(embeddedName(at.File.Path), "<") {
				if locs := s.goSites(at.Text()); len(locs) > 0 {
					return locs, nil
				}
			}
		}
	}
	return s.definition(d, p)
}

// declaration answers textDocument/declaration. For a keyword or an operator
// it is the section of SPECS.md that defines it; for a name, the definition.
func (s *server) declaration(d *document, p Position) (any, *responseError) {
	if tok, _ := keywordAt(d, d.offset(p)); tok != nil {
		if _, loc, ok := s.keywordDoc(tok.Text(d.text)); ok && loc.URI != "" {
			return []location{loc}, nil
		}
	}
	return s.definition(d, p)
}

// commentWord returns the name at offset when offset is in a comment: a
// word, or a dotted name such as notify.send, as `None` or [Delivery] write
// it in a doc comment.
func commentWord(d *document, offset int) string {
	inComment := false
	d.file.Root.Walk(func(n *syntax.Node) bool {
		if inComment || offset < n.Span.Start-1 && len(n.Leading) == 0 {
			return !inComment
		}
		for _, t := range n.Leading {
			if t.Kind == syntax.Comment && t.Span.Start <= offset && offset < t.Span.End {
				inComment = true
			}
		}
		return !inComment
	})
	if !inComment {
		return ""
	}
	isName := func(b byte) bool { return isIdentByte(b) || b == '.' }
	start, end := offset, offset
	for start > 0 && isName(d.text[start-1]) {
		start--
	}
	for end < len(d.text) && isName(d.text[end]) {
		end++
	}
	word := strings.Trim(string(d.text[start:end]), ".")
	if word == "" || word[0] >= '0' && word[0] <= '9' {
		return ""
	}
	return word
}
