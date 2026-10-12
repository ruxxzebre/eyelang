package lir

import (
	"os"
	"path/filepath"
	"testing"

	"eyelang/check"
)

// TestLayouts checks the representation chosen for each kind of type
// (docs/internals/compilation.md, "Values laid out by type"): enums as a
// byte, small records and tuples inline, Option without a box, large and
// recursive types boxed, strings as views.
func TestLayouts(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("eye.toml", "")
	write("main.eye", `type Kind = Ident | Number | Plus | Minus
type Token = { kind: Kind, start: Int, end: Int, spaceBefore: Bool, newlineBefore: Bool, missing: Bool }
type Particle = { x: Int, y: Int, vx: Int, vy: Int, bounces: Int }
type Point = { x: Float, y: Float }
type Big = { a: String, b: String, c: String }
type Tree = Leaf | Node(Tree, Tree)
type Shape = Circle(r: Float) | Rect(w: Float, h: Float)
type Id = | Id(String)
type Link = { name: String, next: Option(Link) }

fn uses(k: Kind, t: Token, p: Particle, q: Point, b: Big, tr: Tree, s: Shape, id: Id, l: Link,
        oi: Option(Int), ol: Option(List(Int)), os: Option(String), ok: Option(Kind), ot: Option(Tree),
        ob: Option(Bool), of: Option(Float), oo: Option(Option(Int)), r: Result(Int, String),
        pair: (Kind, Int), xs: List(Token), m: Map(String, Int), rg: Range(Int), at: Instant, n: Byte) -> Int = 0

pub fn main() -> Int = 0
`)
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	f := prog.Root.Funcs["uses"]
	if f == nil {
		t.Fatal("no function uses")
	}
	low, errs := Lower(prog)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	var params []*Value
	for _, fn := range low.Funcs {
		if fn.Src == f {
			params = fn.Params
		}
	}
	want := map[string]string{
		"k":    "enum uint8_t 1/1 niche@0",
		"t":    "struct 24/8 counted {kind@16 start@0 end@8 spaceBefore@17 newlineBefore@18 missing@19} niche@0",
		"p":    "struct 40/8 counted {x@0 y@8 vx@16 vy@24 bounces@32} niche@0",
		"q":    "struct 16/8 {x@0 y@8}",
		"b":    "box 8/8 counted {a@16 b@40 c@64} niche@0",
		"tr":   "box 8/8 counted recursive tag box [Leaf=1 | Node{0@16 1@24}=1] niche@0",
		"s":    "struct 24/8 tag field@0/1 [Circle{r@8}=0 | Rect{w@8 h@16}=1] niche@0",
		"id":   "string 24/8 counted {p@0 len@8 own@16} niche@0",
		"l":    "box 8/8 counted recursive {name@16 next@40} niche@0",
		"oi":   "struct 8/8 counted tag niche@0/8 [Some{0@0} | None=0] niche@0",
		"ol":   "struct 8/8 counted tag niche@0/8 [Some{0@0} | None=0] niche@0",
		"os":   "struct 24/8 counted tag niche@0/8 [Some{0@0} | None=0] niche@0",
		"ok":   "struct 1/1 tag niche@0/1 [Some{0@0} | None=4] niche@0",
		"ot":   "struct 8/8 counted tag niche@0/8 [Some{0@0} | None=0] niche@0",
		"ob":   "struct 1/1 tag niche@0/1 [Some{0@0} | None=2] niche@0",
		"of":   "struct 16/8 tag field@0/1 [Some{0@8}=0 | None=1] niche@0",
		"oo":   "struct 8/8 counted tag niche@0/8 [Some{0@0} | None=2] niche@0",
		"r":    "struct 32/8 counted tag field@0/1 [Ok{0@8}=0 | Err{0@8}=1] niche@0",
		"pair": "struct 16/8 counted {0@8 1@0} niche@0",
		"xs":   "object 8/8 counted niche@0",
		"m":    "object 8/8 counted niche@0",
		"rg":   "struct 24/8 counted {start@0 end@8 flags@16} niche@0",
		"at":   "struct 16/8 {ns@0 zone@8}",
		"n":    "scalar uint8_t 1/1",
	}
	ls := NewLayouts()
	for _, p := range params {
		w, ok := want[p.Name]
		if !ok {
			t.Errorf("no layout expected for %s", p.Name)
			continue
		}
		if got := ls.Of(p.Ty).String(); got != w {
			t.Errorf("%s: %s is laid out as\n\t%s\nwant\n\t%s", p.Name, p.Ty, got, w)
		}
		// One word, dup'ed and dropped as a word.
		word := map[string]bool{"b": true, "tr": true, "l": true, "oi": true, "ol": true, "ot": true, "oo": true, "xs": true, "m": true}[p.Name]
		if got := ls.Of(p.Ty).Word(); got != word {
			t.Errorf("%s: %s is a word: %v, want %v", p.Name, p.Ty, got, word)
		}
	}
}
