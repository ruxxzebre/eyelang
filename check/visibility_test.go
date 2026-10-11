package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// lib is a module the boundary tests import: public and private names, an
// opaque record and an opaque newtype.
const lib = `pub type Shown = | Red | Blue
type Hidden = | Secretly(Int)
pub opaque type Box = { inner: Int }
pub opaque type Id = | Id(String)
fn helper() -> Int = 1
pub fn make() -> Box = { inner: 1 }
let hidden = 2
pub entity Counter(id: Int) {
  state n: Int = 0
  fn bump() { n += 1 }
  pub fn add() { n += 1 }
}
entity Private(id: Int) {}
impl Box {
  fn peek(self) -> Int = self.inner
  pub fn get(self) -> Int = self.inner
}
`

// TestModuleBoundaries checks that another module uses only what a module
// marks pub, and nothing inside a pub opaque type (SPECS.md §5, §13). Names
// are checked as they resolve, and fields, variants and methods as types
// are checked, so each group is its own program.
func TestModuleBoundaries(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{"private names", `import lib
import lib.{helper}
fn a() -> Int = lib.helper() + lib.hidden
fn b(h: lib.Hidden) -> Int = 1
fn c() -> Int = Secretly(1)
fn d() = lib.Private(1)
fn ok() -> lib.Shown = Red`, []string{
			"2:13: helper is private to module lib: declare it pub there to use it here",
			"3:21: helper is private to module lib",
			"3:36: hidden is private to module lib",
			"4:13: Hidden is private to module lib",
			"5:17: Secretly is private to module lib",
			"6:14: Private is private to module lib",
		}},
		{"an opaque type's variants", `import lib
fn f() -> lib.Id = Id("b")
fn g(i: lib.Id) -> String = match i { Id(s) => s }`, []string{
			"2:20: Id is a variant of Id, which module lib declares opaque: other modules make and match its values only through its functions",
			"3:39: Id is a variant of Id, which module lib declares opaque",
		}},
		{"an opaque type's fields and private methods", `import lib
fn d() -> lib.Box = { inner: 2 }
fn e(x: lib.Box) -> Int = x.inner + x.peek() + x.get()
fn h(x: lib.Box) -> lib.Box = { x with inner: 3 }
fn k() = lib.Counter(1).bump()
fn m(x: lib.Box) -> Int = match x { { inner } => inner }
fn ok() -> Int = lib.make().get()`, []string{
			"2:21: Box is opaque outside module lib, so its fields can't be used here: use its functions and methods",
			"3:27: Box is opaque outside module lib, so its field inner can't be used here",
			"3:37: peek is private to module lib: declare it pub there to use it here",
			"4:33: Box is opaque outside module lib, so its fields can't be used here",
			"5:10: bump is private to module lib",
			"6:27: Box is opaque outside module lib, so its fields can't be used here",
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for path, src := range map[string]string{"main.eye": c.src + "\n", "lib/lib.eye": lib} {
				p := filepath.Join(dir, path)
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			_, diags := Program(dir)
			var got []string
			for _, d := range diags {
				got = append(got, strings.TrimPrefix(d.String(), filepath.Join(dir, "main.eye")+":"))
			}
			for _, w := range c.want {
				found := false
				for _, g := range got {
					found = found || strings.HasPrefix(g, w)
				}
				if !found {
					t.Errorf("missing %q in:\n%s", w, strings.Join(got, "\n"))
				}
			}
			if len(got) != len(c.want) {
				t.Errorf("want %d diagnostics, got:\n%s", len(c.want), strings.Join(got, "\n"))
			}
		})
	}
}
