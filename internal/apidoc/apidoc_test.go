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
	src := `## Shapes and what they cover. See [area].

import ai

## A shape on the plane.
pub type Shape =
  ## Round, by its radius.
  | Circle(Float)
  | Square(Float)

## A point, made with [Point.at] or [Origin].
pub type Point = {
  ## Across.
  x: Int,
  y: Int,
}

# An ordinary comment, not documentation.

## The area of a [Shape], as a [Float] would hold it. [Square] is exact;
## [Circle] uses 3.0 for pi. Built-ins link too: [filter], [Option], [ai.generate].
## Broken: [Nowhere].
pub fn area(s: Shape) -> Float = match s {
  Circle(r) => 3.0 * r * r
  Square(a) => a * a
}

fn helper() = 1

## Where shapes go.
pub entity Board(id: String) {
  ## Everything added so far.
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
	pages, problems := Pages(prog)
	byPath := map[string]string{}
	for _, p := range pages {
		byPath[p.Path] = p.Content
	}
	main := byPath["main.md"]
	for _, want := range []string{
		"Shapes and what they cover.",
		"Shapes and what they cover. See [`area`](./main#area).",
		"### Shape {#Shape}\n\n```eye\npub type Shape =\n  | Circle(Float)\n  | Square(Float)\n```\n\nA shape on the plane.",
		"- <span id=\"Shape-Circle\"></span>`Circle(Float)`: Round, by its radius.",
		"- <span id=\"Point-x\"></span>`x: Int`: Across.",
		"### area {#area}\n\n```eye\npub fn area(s: Shape) -> Float\n```\n\nThe area of a [`Shape`](./main#Shape)",
		"[`Square`](./main#Shape-Square) is exact;\n[`Circle`](./main#Shape-Circle)",
		"[`filter`](./std/builtins#filter), [`Option`](./std/prelude#Option), [`ai.generate`](./std/ai#generate)",
		"Broken: [Nowhere].",
		"- <span id=\"Board-shapes\"></span>`state shapes: List(Shape) = []`: Everything added so far.",
		"#### Board.add {#Board-add}\n\n```eye\npub fn add(s: Shape)\n```\n\nAdds a shape to the board.",
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
	var got []string
	for _, p := range problems {
		got = append(got, p.Decl+": "+p.Link)
	}
	if want := "Point: Point.at|Point: Origin|area: Float|area: Nowhere"; strings.Join(got, "|") != want {
		t.Errorf("problems = %q, want %q", strings.Join(got, "|"), want)
	}
	if !strings.Contains(byPath["index.md"], "[main](./main) | Shapes and what they cover.") {
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
