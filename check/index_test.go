package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"eyelang/ir"
)

// indexProgram is a program of two modules, for the index's tests.
var indexProgram = map[string]string{
	"eye.toml": "",
	"main.eye": `import shapes.{Square}

type Color = | Red | Green(level: Int)

entity Counter(id: String) {
  state count: Int = 0

  fn bump() {
    count += 1
    log(count)
  }

  fn log(n: Int) {
    print(n)
  }
}

fn area(s: shapes.Shape) -> Int {
  let n = shapes.size(s)
  match s {
    Square(side) => side * n
  }
}

fn pick(c: Color) -> Option(Int) = match c {
  Red => None
  Green(level) => Some(level)
}
`,
	"shapes/shapes.eye": `pub type Shape = | Square(Int)

pub fn size(s: Shape) -> Int = 2
`,
}

func writeProgram(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, src := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestIndex checks what names resolve to. Each case names a use, by the
// text before it on its line and its own text, and where its declaration is,
// as file:line:column, or "builtin print" for a built-in.
func TestIndex(t *testing.T) {
	dir := writeProgram(t, indexProgram)
	res := Check(dir, Options{})
	for _, d := range res.Diagnostics {
		t.Fatal(d)
	}
	cases := []struct {
		file, before, name, want string
	}{
		{"main.eye", "import shapes.{", "Square", "shapes/shapes.eye:1:20"},
		{"main.eye", "import ", "shapes", "shapes/shapes.eye:1:1"},
		{"main.eye", "    ", "count", "main.eye:6:9"},                         // a state field
		{"main.eye", "    log(", "count", "main.eye:6:9"},                     // read
		{"main.eye", "    ", "log", "main.eye:13:6"},                          // an entity's own method
		{"main.eye", "    print(", "n", "main.eye:13:10"},                     // a parameter
		{"main.eye", "    ", "print", "builtin print"},                        // a built-in
		{"main.eye", "fn area(s: shapes.", "Shape", "shapes/shapes.eye:1:10"}, // a qualified type
		{"main.eye", "  let n = shapes.", "size", "shapes/shapes.eye:3:8"},
		{"main.eye", "  let n = shapes.size(", "s", "main.eye:18:9"},
		{"main.eye", "    ", "Square", "shapes/shapes.eye:1:20"}, // a variant pattern
		{"main.eye", "    Square(side) => side * ", "n", "main.eye:19:7"},
		{"main.eye", "    Square(side) => ", "side", "main.eye:21:12"},
		{"main.eye", "fn pick(c: ", "Color", "main.eye:3:6"},
		{"main.eye", "  ", "Red", "main.eye:3:16"},
		{"main.eye", "  Green(level) => Some(", "level", "main.eye:27:9"},
		{"main.eye", "  Green(level) => ", "Some", "prelude.eye:"},
	}
	for _, c := range cases {
		f := res.Program.Modules[len(res.Program.Modules)-1].Files[0]
		for _, m := range res.Program.Modules {
			for _, mf := range m.Files {
				if strings.HasSuffix(filepath.ToSlash(mf.Path), "/"+c.file) {
					f = mf
				}
			}
		}
		offset := -1
		for _, line := range strings.SplitAfter(string(f.Src), "\n") {
			if strings.HasPrefix(line, c.before+c.name) {
				offset = strings.Index(string(f.Src), line) + len(c.before)
				break
			}
		}
		if offset < 0 {
			t.Errorf("%s: no line starts with %q", c.file, c.before+c.name)
			continue
		}
		use, ok := res.Index.At(f.Path, offset+1)
		if !ok {
			t.Errorf("%s %q: not in the index", c.before, c.name)
			continue
		}
		if got := use.At.Text(); got != c.name {
			t.Errorf("%s %q: the use spans %q", c.before, c.name, got)
		}
		var got string
		if b, ok := use.Target.(ir.Builtin); ok {
			got = "builtin " + b.Name
		} else if at, ok := Definition(use.Target); ok {
			got = strings.TrimPrefix(filepath.ToSlash(strings.TrimPrefix(at.String(), dir)), "/")
			if strings.HasPrefix(at.File.Path, "<prelude>") {
				got = "prelude.eye:"
			}
		}
		if !strings.HasPrefix(got, c.want) || (c.want != "prelude.eye:" && got != c.want) {
			t.Errorf("%s %q goes to %s, want %s", c.before, c.name, got, c.want)
		}
	}
}

// TestOverlay checks that an unsaved file's text wins over the disk's.
func TestOverlay(t *testing.T) {
	dir := writeProgram(t, map[string]string{"main.eye": "fn f() = g()\n"})
	path := filepath.Join(dir, "main.eye")
	if res := Check(dir, Options{}); len(res.Diagnostics) == 0 {
		t.Fatal("g should be undefined on disk")
	}
	res := Check(dir, Options{Overlay: map[string][]byte{path: []byte("fn f() = g()\nfn g() = 1\n")}})
	for _, d := range res.Diagnostics {
		t.Error(d)
	}
}

func TestRoot(t *testing.T) {
	dir := writeProgram(t, map[string]string{
		"lab/eye.toml":          "",
		"lab/digest/digest.eye": "",
		"loose/a.eye":           "",
	})
	cases := map[string]string{
		"lab/digest/digest.eye": "lab",
		"lab/digest":            "lab",
		"lab":                   "lab",
		"loose/a.eye":           "loose",
		"loose":                 "loose",
		"lab/digest/new.eye":    "lab", // not saved yet
	}
	for path, want := range cases {
		got := Root(filepath.Join(dir, filepath.FromSlash(path)))
		if rel, _ := filepath.Rel(dir, got); filepath.ToSlash(rel) != want {
			t.Errorf("Root(%s) = %s, want %s", path, rel, want)
		}
	}
}

func TestEmbedded(t *testing.T) {
	for _, name := range []string{"<prelude>", "<std ai>"} {
		repoPath, src, ok := Embedded(name)
		if !ok {
			t.Fatalf("%s isn't embedded", name)
		}
		on, err := os.ReadFile(repoPath)
		if err != nil || string(on) != string(src) {
			t.Errorf("%s: %s should hold the embedded source", name, repoPath)
		}
	}
	if _, _, ok := Embedded("<std nothing>"); ok {
		t.Error("<std nothing> shouldn't be embedded")
	}
}
