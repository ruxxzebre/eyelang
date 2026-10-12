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
		"keyword":  at(4, 3),  // let
		"int":      at(2, 44), // -> Int
		"let":      at(5, 7),  // n in Some(n)
		"param":    at(3, 8),  // b in b.side
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
		"function": {"```eye\npub fn size(s: Shape) -> Int\n```", "The size of a shape. See [`Box.side`](", "shapes.eye#L15)"},
		"type":     {"pub type Shape =\n  | Square(Int)\n  | Dot\n```", "A shape on the plane."},
		"variant":  {"Square(Int)", "Square, by its side.", "A variant of `Shape`."},
		"field":    {"side: Int", "How long a side is.", "A field of `Box`."},
		"module":   {"```eye\nimport shapes\n```", "Shapes and their sizes."},
		"builtin":  {"fn print(", "standard output"},
		"prelude":  {"Some(a)", "A variant of `Option`."},
		"newtype":  {"pub type Name = | Name(String)", "Names a shape."},
		"keyword":  {"`let`: Binds a name"},
		"int":      {"```eye\ntype Int\n```", "A whole number of any size"},
		"let":      {"```eye\nn: Int\n```"},
		"param":    {"```eye\nb: Box\n```"},
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

// TestHoverShowsInferredEffects shows a private function's effects as the
// checker inferred them, which its signature doesn't state; a public
// function's signature has its uses clause already.
func TestHoverShowsInferredEffects(t *testing.T) {
	dir := t.TempDir()
	main := `fn stamp() -> Instant = now()

fn twice(n: Int) -> Int = n * 2

pub fn label() -> String uses {clock} = "{stamp()} {twice(2)}"
`
	if err := os.WriteFile(filepath.Join(dir, "main.eye"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "eye.toml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	uri := pathURI(filepath.Join(dir, "main.eye"))
	s := newSession(t)
	s.open(uri, main)
	at := func(line, char int) int {
		return s.request("textDocument/hover", map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"position":     map[string]any{"line": line, "character": char},
		})
	}
	ids := map[string]int{"effects": at(4, 45), "pure": at(4, 55), "public": at(4, 9)}
	msgs := s.run()
	for name, want := range map[string]string{
		"effects": "Its effects, as inferred: `uses {clock}`.",
		"pure":    "Its effects, as inferred: none.",
	} {
		var h *hoverResult
		response(t, msgs, ids[name], &h)
		if h == nil || !strings.Contains(h.Contents.Value, want) {
			t.Errorf("hover on %s lacks %q: %+v", name, want, h)
		}
	}
	var h *hoverResult
	response(t, msgs, ids["public"], &h)
	if h == nil || !strings.Contains(h.Contents.Value, "uses {clock}") || strings.Contains(h.Contents.Value, "as inferred") {
		t.Errorf("hover on a public function: %+v", h)
	}
}
