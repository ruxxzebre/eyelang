package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTourResolves(t *testing.T) {
	prog, diags := Program("../testdata/programs/tour")
	for _, d := range diags {
		t.Error(d)
	}
	if prog.Root == nil || len(prog.Root.Tests) != 3 {
		t.Fatalf("the tour's module should have 3 tests")
	}
}

// TestDiagnostics checks what the checker reports for one-file programs.
// Each case is a program and the messages it must produce, as
// "line:column: message".
func TestDiagnostics(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{"undefined names", `fn f() = g(x)
fn h() = Missing(1)`, []string{"1:10: undefined name g", "1:12: undefined name x", "2:10: undefined name Missing"}},
		{"duplicate declarations", `fn f() = 1
fn f() = 2
type T = | A
type U = | A`, []string{"2:4: f is declared twice in this module", "4:6: A is declared twice in this module"}},
		{"a let can't change", `fn f(n: Int) {
  let a = 1
  a = 2
  n += 1
}`, []string{"3:3: a can't change", "4:3: n can't change"}},
		{"var and mut can change", `fn f(mut n: Int) {
  var a = 1
  a = 2
  n += a
}`, nil},
		{"closures don't capture vars", `fn f() {
  var a = 1
  let g = fn: a + 1
}`, []string{"3:15: a closure can't capture a, a var"}},
		{"waiting calls only in jobs", `entity E(id: Int) {
  state n: Int = 0
  pub fn m() { sleep!(1 min) }
}
test "t" { sleep!(1 min) }
pub fn main() { sleep!(1 min) }`, []string{
			"3:16: an entity method is a transaction and can't wait",
			"5:12: a test can't make waiting calls",
			"6:17: `!` marks a durable step",
		}},
		{"durable steps in jobs need !", `job j() { sleep(1 min) }`, []string{"1:11: in a job, sleep is a durable step: write sleep!(…)"}},
		{"break outside a loop", `fn f() { break }`, []string{"1:10: break outside a loop"}},
		{"state can change in entity methods, the identity can't", `entity E(id: Int) {
  state n: Int = 0
  pub fn m() {
    n += 1
    id = 2
  }
}`, []string{"5:5: id is the entity's identity and can't change"}},
		{"with only in tests", `fn f() { with clock(2026-10-10 12:00 in "UTC") }`, []string{"1:10: with applies a test handler"}},
		{"missing module", `import nowhere`, []string{"1:8: no module nowhere"}},
		{"views start from an entity set", `view v = [1, 2]`, []string{"1:1: a view is a pipeline that starts from an entity set"}},
		{"syntax errors", `fn f( {`, []string{"1:7: expected"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "main.eye"), []byte(c.src+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			_, diags := Program(dir)
			var got []string
			for _, d := range diags {
				got = append(got, strings.TrimPrefix(d.String(), filepath.Join(dir, "main.eye")+":"))
			}
			if len(c.want) == 0 && len(got) > 0 {
				t.Errorf("unexpected diagnostics:\n%s", strings.Join(got, "\n"))
			}
			for _, w := range c.want {
				found := false
				for _, g := range got {
					if strings.HasPrefix(g, w) {
						found = true
					}
				}
				if !found {
					t.Errorf("missing %q in:\n%s", w, strings.Join(got, "\n"))
				}
			}
		})
	}
}

func TestImportsAcrossModules(t *testing.T) {
	dir := t.TempDir()
	write := func(path, src string) {
		p := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main.eye", "import lib.shapes\nimport lib.shapes.{Circle}\nlet a = shapes.area(Circle(1.0))\nlet b = shapes.missing\n")
	write("lib/shapes/shapes.eye", "pub type Shape = | Circle(Float)\npub fn area(s: Shape) -> Float = 1.0\n")
	_, diags := Program(dir)
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "module shapes has no missing") {
		t.Errorf("diagnostics: %v", diags)
	}
}
