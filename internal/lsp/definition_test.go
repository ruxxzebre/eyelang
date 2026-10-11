package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestDefinition jumps from names in an open, unsaved file: to another
// module, to a local, to the prelude, to the Go behind a built-in, and from an
// extern fn's name to its host function.
func TestDefinition(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("eye.toml", "")
	write("shapes/shapes.eye", "pub type Shape = | Square(Int)\n\npub fn size(s: Shape) -> Int = 2\n")
	// On disk the program doesn't use shapes; the editor's text does.
	write("app/main.eye", "fn main() {}\n")
	main := "import shapes\n\nfn area(s: shapes.Shape) -> Int {\n  let n = shapes.size(s)\n  print(n)\n  Some(n) ?? 0\n}\n"
	uri := pathURI(filepath.Join(dir, "app", "main.eye"))
	// An extern fn the interpreter provides; no program needs to use it.
	ext := "@host(\"http.get\")\npub extern fn get(url: String) -> String\n"
	extURI := pathURI(filepath.Join(dir, "ext", "ext.eye"))

	s := newSession(t)
	s.open(uri, main)
	s.open(extURI, ext)
	at := func(u string, line, char int) int {
		return s.request("textDocument/definition", map[string]any{
			"textDocument": map[string]any{"uri": u},
			"position":     map[string]any{"line": line, "character": char},
		})
	}
	ids := map[string]int{
		"module function": at(uri, 3, 18), // shapes.size
		"qualified type":  at(uri, 2, 20), // shapes.Shape
		"local":           at(uri, 4, 8),  // print(n)
		"declared name":   at(uri, 3, 6),  // let n
		"prelude":         at(uri, 5, 3),  // Some
		"builtin":         at(uri, 4, 3),  // print
		"host function":   at(extURI, 1, 16),
		"nothing":         at(uri, 1, 0),
	}
	msgs := s.run()
	get := func(name string) []location {
		var locs []location
		response(t, msgs, ids[name], &locs)
		return locs
	}
	expect := func(name, file string, line, char int) {
		t.Helper()
		locs := get(name)
		if len(locs) != 1 {
			t.Errorf("%s: %d locations, want 1", name, len(locs))
			return
		}
		got, _ := uriPath(locs[0].URI)
		if !strings.HasSuffix(filepath.ToSlash(got), file) || (line >= 0 && locs[0].Range.Start.Line != line) ||
			(char >= 0 && locs[0].Range.Start.Character != char) {
			t.Errorf("%s: %s:%d:%d, want %s:%d:%d", name, got, locs[0].Range.Start.Line, locs[0].Range.Start.Character, file, line, char)
		}
	}
	expect("module function", "shapes/shapes.eye", 2, 7)
	expect("qualified type", "shapes/shapes.eye", 0, 9)
	expect("local", "app/main.eye", 3, 6)
	expect("declared name", "app/main.eye", 3, 6)
	expect("prelude", "check/prelude.eye", -1, -1)
	expect("builtin", "interp/builtins.go", -1, 0)
	expect("host function", "interp/services.go", -1, 0)
	if locs := get("nothing"); len(locs) != 0 {
		t.Errorf("an empty line has a definition: %v", locs)
	}
}

// TestReferences finds the uses of a function across a program's modules,
// from its declaration, and goes from a method call and a built-in type to
// their declarations.
func TestReferences(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("eye.toml", "")
	write("shapes/shapes.eye", "pub type Box = { side: Int }\n\nimpl Box {\n  pub fn area(self) -> Int = self.side * self.side\n}\n\npub fn unit() -> Box = { side: 1 }\n")
	write("a/a.eye", "import shapes\n\nfn a() -> Int = shapes.unit().area()\n")
	write("b/b.eye", "import shapes\n\nfn b(s: String) -> Int = shapes.unit().side\n")
	shapes := pathURI(filepath.Join(dir, "shapes", "shapes.eye"))
	a := pathURI(filepath.Join(dir, "a", "a.eye"))
	b := pathURI(filepath.Join(dir, "b", "b.eye"))

	s := newSession(t)
	src, _ := os.ReadFile(filepath.Join(dir, "shapes", "shapes.eye"))
	s.open(shapes, string(src))
	src, _ = os.ReadFile(filepath.Join(dir, "a", "a.eye"))
	s.open(a, string(src))
	src, _ = os.ReadFile(filepath.Join(dir, "b", "b.eye"))
	s.open(b, string(src))
	refs := s.request("textDocument/references", map[string]any{
		"textDocument": map[string]any{"uri": shapes},
		"position":     map[string]any{"line": 6, "character": 9}, // pub fn unit
		"context":      map[string]any{"includeDeclaration": false},
	})
	field := s.request("textDocument/references", map[string]any{
		"textDocument": map[string]any{"uri": shapes},
		"position":     map[string]any{"line": 0, "character": 18}, // side
		"context":      map[string]any{"includeDeclaration": true},
	})
	method := s.request("textDocument/definition", map[string]any{
		"textDocument": map[string]any{"uri": a},
		"position":     map[string]any{"line": 2, "character": 31}, // .area()
	})
	builtin := s.request("textDocument/definition", map[string]any{
		"textDocument": map[string]any{"uri": b},
		"position":     map[string]any{"line": 2, "character": 9}, // String
	})
	builtinImpl := s.request("textDocument/implementation", map[string]any{
		"textDocument": map[string]any{"uri": b},
		"position":     map[string]any{"line": 2, "character": 9},
	})
	msgs := s.run()
	where := func(locs []location) []string {
		var out []string
		for _, l := range locs {
			p, _ := uriPath(l.URI)
			rel, err := filepath.Rel(dir, p)
			if err != nil || strings.HasPrefix(rel, "..") {
				rel = filepath.Base(p)
			}
			out = append(out, fmt.Sprintf("%s:%d:%d", filepath.ToSlash(rel), l.Range.Start.Line, l.Range.Start.Character))
		}
		sort.Strings(out)
		return out
	}
	var locs []location
	response(t, msgs, refs, &locs)
	if got, want := strings.Join(where(locs), " "), "a/a.eye:2:23 b/b.eye:2:32"; got != want {
		t.Errorf("references to unit: %s, want %s", got, want)
	}
	response(t, msgs, field, &locs)
	if got, want := strings.Join(where(locs), " "), "b/b.eye:2:39 shapes/shapes.eye:0:17 shapes/shapes.eye:3:34 shapes/shapes.eye:3:46 shapes/shapes.eye:6:25"; got != want {
		t.Errorf("references to side: %s, want %s", got, want)
	}
	response(t, msgs, method, &locs)
	if got, want := strings.Join(where(locs), " "), "shapes/shapes.eye:3:9"; got != want {
		t.Errorf("area() goes to %s, want %s", got, want)
	}
	response(t, msgs, builtin, &locs)
	if got := where(locs); len(got) != 1 || !strings.HasPrefix(got[0], "builtin.eye:") {
		t.Errorf("String goes to %v, want ir/builtin.eye", got)
	}
	response(t, msgs, builtinImpl, &locs)
	if len(locs) < 2 || !strings.HasSuffix(locs[0].URI, "/check/types.go") {
		t.Errorf("String's implementations are %v, want check/types.go first, then the other Go that names it", where(locs))
	}
}
