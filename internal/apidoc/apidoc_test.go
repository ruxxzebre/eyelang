package apidoc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"eyelang/check"
)

func TestPages(t *testing.T) {
	dir := t.TempDir()
	src := `# Shapes and what they cover.

## A shape on the plane.
pub type Shape = | Circle(Float) | Square(Float)

# An ordinary comment, not documentation.

## The area of a shape.
## Squares are exact; circles use 3.0 for pi.
pub fn area(s: Shape) -> Float = match s {
  Circle(r) => 3.0 * r * r
  Square(a) => a * a
}

fn helper() = 1

pub entity Board(id: String) {
  state shapes: List(Shape) = []

  ## Adds a shape to the board.
  pub fn add(s: Shape) { shapes = shapes |> add(s) }
}
`
	if err := os.WriteFile(filepath.Join(dir, "shapes.eye"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	pages := Pages(prog)
	byPath := map[string]string{}
	for _, p := range pages {
		byPath[p.Path] = p.Content
	}
	main := byPath["main.md"]
	for _, want := range []string{
		"Shapes and what they cover.",
		"### Shape\n\n```eye\npub type Shape = | Circle(Float) | Square(Float)\n```\n\nA shape on the plane.",
		"### area\n\n```eye\npub fn area(s: Shape) -> Float\n```\n\nThe area of a shape.\nSquares are exact; circles use 3.0 for pi.",
		"#### Board.add\n\n```eye\npub fn add(s: Shape)\n```\n\nAdds a shape to the board.",
		"state shapes: List(Shape) = []",
	} {
		if !strings.Contains(main, want) {
			t.Errorf("main.md lacks %q:\n%s", want, main)
		}
	}
	if strings.Contains(main, "helper") || strings.Contains(main, "ordinary comment") {
		t.Errorf("main.md documents private or undocumented things:\n%s", main)
	}
	if !strings.Contains(byPath["std/prelude.md"], "### Option(a)") || !strings.Contains(byPath["std/builtins.md"], "### filter") {
		t.Error("the standard library pages are missing")
	}
	if !strings.Contains(byPath["index.md"], "[main](./main)") {
		t.Errorf("the index doesn't link the module:\n%s", byPath["index.md"])
	}

	out := filepath.Join(t.TempDir(), "api")
	if err := Write(out, pages); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "std", "builtins.md")); err != nil {
		t.Error(err)
	}
}
