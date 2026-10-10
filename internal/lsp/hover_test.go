package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHover shows the signature and the doc comment of what a name refers to:
// a function in another module, a type, a variant, a record field, a module,
// a built-in function and a prelude type.
func TestHover(t *testing.T) {
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
	write("shapes/shapes.eye", `## Shapes and their sizes.

## A shape on the plane.
pub type Shape =
  ## Square, by its side.
  | Square(Int)
  | Dot

## Names a shape.
pub type Name = | Name(String)

## A box.
pub type Box = {
  ## How long a side is.
  side: Int,
}

## The size of a shape. See [Box.side].
pub fn size(s: Shape) -> Int = match s {
  Square(n) => n
  Dot => 0
}
`)
	main := "import shapes\n\nfn area(s: shapes.Shape, b: shapes.Box) -> Int {\n  print(b.side)\n  let n = shapes.size(shapes.Square(2))\n  Some(n) ?? 0\n}\n\nlet unnamed = shapes.Name(\"none\")\n"
	write("app/main.eye", main)
	uri := pathURI(filepath.Join(dir, "app", "main.eye"))

	s := newSession(t)
	s.open(uri, main)
	at := func(line, char int) int {
		return s.request("textDocument/hover", map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"position":     map[string]any{"line": line, "character": char},
		})
	}
	ids := map[string]int{
		"function": at(4, 18), // shapes.size
		"type":     at(2, 20), // shapes.Shape
		"variant":  at(4, 30), // shapes.Square
		"field":    at(3, 11), // b.side
		"module":   at(0, 8),  // import shapes
		"builtin":  at(3, 3),  // print
		"prelude":  at(5, 3),  // Some
		"nothing":  at(1, 0),
		"newtype":  at(8, 23), // shapes.Name("none")
	}
	msgs := s.run()
	get := func(name string) string {
		var h *hoverResult
		response(t, msgs, ids[name], &h)
		if h == nil {
			return ""
		}
		return h.Contents.Value
	}
	for name, want := range map[string][]string{
		"function": {"```eye\npub fn size(s: Shape) -> Int\n```", "The size of a shape. See `Box.side`."},
		"type":     {"pub type Shape =\n  | Square(Int)\n  | Dot\n```", "A shape on the plane."},
		"variant":  {"Square(Int)", "Square, by its side.", "A variant of `Shape`."},
		"field":    {"side: Int", "How long a side is.", "A field of `Box`."},
		"module":   {"```eye\nimport shapes\n```", "Shapes and their sizes."},
		"builtin":  {"fn print(", "standard output"},
		"prelude":  {"Some(a)", "A variant of `Option`."},
		"newtype":  {"pub type Name = | Name(String)", "Names a shape."},
	} {
		got := get(name)
		for _, w := range want {
			if !strings.Contains(got, w) {
				t.Errorf("hover on %s lacks %q:\n%s", name, w, got)
			}
		}
	}
	if got := get("nothing"); got != "" {
		t.Errorf("an empty line has a hover: %s", got)
	}
}
