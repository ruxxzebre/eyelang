package lsp

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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
	overlay := map[string][]byte{}
	for uri, d := range s.docs {
		if p, ok := uriPath(uri); ok && strings.HasPrefix(p, root+string(filepath.Separator)) {
			overlay[p] = d.text
		}
	}
	return check.Check(root, check.Options{Overlay: overlay, Module: filepath.Dir(path)})
}

// targetLocation returns where a use's target is declared.
func (s *server) targetLocation(target any) (location, bool) {
	if b, ok := target.(ir.Builtin); ok {
		src, ok := interp.BuiltinSource(b.Name)
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
