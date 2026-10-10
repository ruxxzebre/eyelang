package lsp

import (
	"os"
	"path/filepath"
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
