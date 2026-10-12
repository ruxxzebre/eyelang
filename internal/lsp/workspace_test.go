package lsp

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestWorkspace answers an editor's requests for a program that is only in
// memory, as the online playground's editor makes them.
func TestWorkspace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "nowhere")
	main := filepath.Join(root, "main.eye")
	shapes := filepath.Join(root, "shapes", "shapes.eye")
	w := NewWorkspace(root)
	w.Set(filepath.Join(root, "eye.toml"), nil)
	w.Set(shapes, []byte("## The area of a square.\npub fn area(side: Int) -> Int = side * side\n"))
	w.Set(main, []byte("import shapes\n\npub fn main() uses {stdio} {\n  let n = shapes.area(3)\n  print(\"{n}\")\n}\n"))

	if diags := w.Check(); len(diags) != 0 {
		t.Fatalf("Check() = %v; want nothing", diags)
	}

	// Hover on `area` in main.eye shows the doc comment from shapes.
	if h, ok := w.Hover(main, Position{Line: 3, Character: 18}); !ok || !strings.Contains(h, "The area of a square.") {
		t.Errorf("Hover(area) = %q, %v", h, ok)
	}
	// Definition goes to the other file.
	locs := w.Definition(main, Position{Line: 3, Character: 18})
	if len(locs) != 1 || locs[0].Path != shapes || locs[0].Range.Start != (Position{Line: 1, Character: 7}) {
		t.Errorf("Definition(area) = %+v; want shapes.eye:2:8", locs)
	}
	// Completion after `shapes.` offers area.
	w.Set(main, []byte("import shapes\n\npub fn main() uses {stdio} {\n  let n = shapes.\n}\n"))
	var labels []string
	for _, c := range w.Complete(main, Position{Line: 3, Character: 17}) {
		labels = append(labels, c.Label+":"+c.Kind)
	}
	if !slices.Contains(labels, "area:function") {
		t.Errorf("Complete(shapes.) = %v; want area:function among them", labels)
	}

	// A type error is reported in the file it is in, at its span, and so
	// is a module nothing imports.
	w.Set(main, []byte("import shapes\n\npub fn main() uses {stdio} {\n  print(shapes.area(\"3\"))\n}\n"))
	w.Set(filepath.Join(root, "extra", "extra.eye"), []byte("fn f() -> Int = \"x\"\n"))
	diags := w.Check()
	var got []string
	for _, d := range diags {
		got = append(got, w.Rel(d.Path)+":"+d.Message)
	}
	if len(diags) != 2 || diags[0].Path != filepath.Join(root, "extra", "extra.eye") {
		t.Fatalf("Check() = %v", got)
	}
	if d := diags[1]; d.Path != main || d.Range.Start != (Position{Line: 3, Character: 20}) || !strings.Contains(d.Message, "expected Int, found String") {
		t.Errorf("Check()[1] = %+v", d)
	}

	// Formatting gives the canonical text, and refuses a file that doesn't parse.
	w.Set(main, []byte("fn  shout( s:String )->String=s+\"!\"\n"))
	if out, err := w.Format(main); err != nil || string(out) != "fn shout(s: String) -> String = s + \"!\"\n" {
		t.Errorf("Format() = %q, %v", out, err)
	}
	w.Set(main, []byte("fn (\n"))
	if _, err := w.Format(main); err == nil {
		t.Error("Format() of a file with syntax errors succeeded")
	}
}

// TestWorkspacePrograms checks a workspace of two programs: a directory with
// an eye.toml of its own is a program, which finds the modules of the
// directory above through its modules, and whose problems are its own.
func TestWorkspacePrograms(t *testing.T) {
	root := filepath.Join(t.TempDir(), "nowhere")
	at := func(p string) string { return filepath.Join(root, filepath.FromSlash(p)) }
	w := NewWorkspace(root)
	w.Set(at("eye.toml"), nil)
	w.Set(at("shapes/shapes.eye"), []byte("## The area of a square.\npub fn area(side: Int) -> Int = side * side\n"))
	w.Set(at("main.eye"), []byte("import shapes\n\npub fn main() uses {stdio} { print(\"{shapes.area(2)}\") }\n"))
	w.Set(at("other/eye.toml"), []byte("modules = [\"..\"]\n"))
	w.Set(at("other/other.eye"), []byte("import shapes\n\nfn twice() -> Int = shapes.area(3) * 2\n"))
	if got := w.Programs(); len(got) != 2 || got[0] != root || got[1] != at("other") {
		t.Errorf("Programs() = %v", got)
	}
	if diags := w.Check(); len(diags) != 0 {
		t.Fatalf("Check() = %v; want nothing", diags)
	}
	if h, ok := w.Hover(at("other/other.eye"), Position{Line: 2, Character: 28}); !ok || !strings.Contains(h, "The area of a square.") {
		t.Errorf("Hover(area) in the other program = %q, %v", h, ok)
	}
	w.Set(at("other/other.eye"), []byte("import shapes\n\nfn twice() -> Int = shapes.area(\"3\") * 2\n"))
	if diags := w.Check(); len(diags) != 1 || diags[0].Path != at("other/other.eye") {
		t.Errorf("Check() = %v; want one problem in other/other.eye", diags)
	}
}
