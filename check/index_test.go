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

// TestTypeAt checks that the index gives the type of the expression that
// ends at an offset, as completion after a `.` needs it.
func TestTypeAt(t *testing.T) {
	src := `type User = { name: String, home: Place }
type Place = { city: String }
entity Counter(id: String) {
  state count: Int = 0
}
fn f(u: User, xs: List(Int)) {
  let c = Counter("a")
  let n = u.name
  u
  u.home
  c
  n
  xs
  xs |> size
}
`
	dir := writeProgram(t, map[string]string{"main.eye": src})
	path := filepath.Join(dir, "main.eye")
	res := Check(dir, Options{})
	for _, d := range res.Diagnostics {
		t.Fatal(d)
	}
	// end returns the offset where the nth (from 1) occurrence of text ends.
	end := func(text string, nth int) int {
		at := -1
		for range nth {
			at += 1 + strings.Index(src[at+1:], text)
		}
		return at + len(text)
	}
	name := func(decl any) string {
		switch d := decl.(type) {
		case *ir.Type:
			return d.Name
		case *ir.Entity:
			return d.Name
		case BuiltinType:
			return d.Name
		}
		return "?"
	}
	for _, c := range []struct {
		text string
		nth  int
		want string
	}{
		{"  u\n", 1, "User"},    // a parameter
		{"u.home", 1, "Place"},  // a field chain: u.home, not home
		{"  c\n", 1, "Counter"}, // Ref(Counter) is Counter
		{"  n\n", 1, "String"},  // a let, typed by inference
		{"  xs\n", 1, "List"},
		{"xs |> size", 1, "Int"},
	} {
		offset := end(strings.TrimRight(c.text, "\n"), c.nth)
		decl, ok := res.Index.TypeAt(path, offset)
		if !ok || name(decl) != c.want {
			t.Errorf("TypeAt before %q = %v, %v; want %s", c.text, decl, ok, c.want)
		}
	}
	if s, ok := res.Index.TypeText(path, end("  xs |> size", 1)-len("xs |> size"), end("xs |> size", 1)); !ok || s != "Int" {
		t.Errorf("TypeText(xs |> size) = %q, %v; want Int", s, ok)
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

// TestOverlayOnly checks a program that is nowhere on disk, as the online
// playground holds one: its modules, imports and eye.toml are all overlay.
func TestOverlayOnly(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nowhere")
	at := func(p string) string { return filepath.Join(dir, filepath.FromSlash(p)) }
	overlay := map[string][]byte{
		at("eye.toml"):           []byte("modules = [\"vendor\"]\n"),
		at("main.eye"):           []byte("import shapes\nimport units\n\npub fn main() uses {stdio} { print(\"{shapes.area(2)} {units.cm(3)}\") }\n"),
		at("shapes/shapes.eye"):  []byte("pub fn area(side: Int) -> Int = side * side\n"),
		at("vendor/units/u.eye"): []byte("pub fn cm(n: Int) -> String = \"{n} cm\"\n"),
	}
	res := Check(dir, Options{Overlay: overlay})
	for _, d := range res.Diagnostics {
		t.Error(d)
	}
	if res.Program == nil || res.Program.Root == nil || len(res.Program.Root.Files) != 1 {
		t.Fatalf("the program's own module wasn't loaded: %+v", res.Program)
	}

	// Asked for all of them, the checker loads the modules nothing imports,
	// but not those in a hidden directory.
	overlay[at("extra/extra.eye")] = []byte("fn f() -> Int = \"x\"\n")
	overlay[at("lib/.cache/c.eye")] = []byte("fn g() -> Int = \"x\"\n")
	res = Check(dir, Options{Overlay: overlay, All: true})
	if len(res.Diagnostics) != 1 || res.Diagnostics[0].Pos.File.Path != at("extra/extra.eye") {
		t.Errorf("diagnostics = %v; want one in extra/extra.eye", res.Diagnostics)
	}
	delete(overlay, at("extra/extra.eye"))
	delete(overlay, at("lib/.cache/c.eye"))

	// A directory with an eye.toml of its own is another program, not a
	// module of this one.
	overlay[at("other/eye.toml")] = nil
	overlay[at("other/sub/o.eye")] = []byte("fn g() -> Int = \"x\"\n")
	if res = Check(dir, Options{Overlay: overlay, All: true}); len(res.Diagnostics) != 0 {
		t.Errorf("diagnostics = %v; want none from the other program", res.Diagnostics)
	}
	delete(overlay, at("other/eye.toml"))
	delete(overlay, at("other/sub/o.eye"))

	// A module the overlay doesn't hold is still missing.
	overlay[at("main.eye")] = []byte("import nothere\n")
	res = Check(dir, Options{Overlay: overlay})
	if len(res.Diagnostics) != 1 || !strings.Contains(res.Diagnostics[0].Message, "no module nothere") {
		t.Errorf("diagnostics = %v; want one saying there is no module nothere", res.Diagnostics)
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
