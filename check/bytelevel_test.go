package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestByteLevelCode checks the rules of byte-level code (SPECS.md open
// question 43, decided): element assignment, hex and binary literals,
// string patterns over Bytes, toBytes and a variant's number. Each error is
// reported once, at the span of what is wrong.
func TestByteLevelCode(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string // "line:column: message prefix", then the span's text after " @ "
	}{
		{"element assignment on a var list, a mut parameter and a map", `type P = { xs: List(Int) }
fn bump(mut xs: List(Int), mut p: P, i: Int) {
  xs[i] += 1
  p.xs[0] = 2
}
fn f() -> Map(String, Int) {
  var m: Map(String, Int) = {}
  m["a"] = 1
  var grid = [[1, 2], [3, 4]]
  grid[0][1] *= 3
  m
}`, nil},
		{"an element of a let can't change", `fn f() {
  let xs = [1, 2]
  xs[0] = 3
}`, []string{"3:3: xs can't change: it is a let or a parameter @ xs"}},
		{"an element of the wrong type", `fn f() {
  var xs = [1, 2]
  xs[0] = "a"
  xs["b"] = 1
}`, []string{"3:11: expected Int, found String", "4:6: expected Int, found String"}},
		{"a compound assignment to a map's value", `fn f() {
  var m = { "a": 1 }
  m["a"] += 1
}`, []string{`3:5: a map may not hold the key, so m[key] += v has nothing to start from: write m[key] = (m[key] ?? 0) + v @ "a"`}},
		{"a field of a map's value", `type P = { x: Int }
fn f() {
  var m: Map(String, P) = {}
  m["a"].x = 1
}`, []string{`4:5: a map's value can't be changed inside @ "a"`}},
		{"Bytes and other values have no elements to assign", `fn f(s: String) {
  var b = s.bytes()
  b[0] = 1
  var n = 5
  n[0] = 1
}`, []string{
			"3:5: Bytes don't change one byte at a time: build a List(Byte), assign its elements, and make Bytes with toBytes(list) @ 0",
			"5:5: Int has no elements to assign @ 0",
		}},
		{"a slice can't be assigned", `fn f() {
  var xs = [1, 2, 3]
  xs[0..2] = [4]
}`, []string{"3:6: a slice can't be assigned @ 0..2"}},
		{"an element can't be passed as mut", `fn g(mut x: Int) { x += 1 }
fn f() {
  var xs = [1]
  g(mut xs[0])
}`, []string{"4:9: mut takes a variable or a field of one, not an element @ xs[0]"}},
		{"hex and binary literals are Ints, and Bytes where one is expected", `fn f() -> Int {
  let b: Byte = 0xFF
  let w: UInt32 = 0xFFFF_FFFF
  0x7F + 0b1010 + b.toInt() + w.toInt()
}`, nil},
		{"a hex literal too large for a Byte", `fn f() -> Byte = 0x1FF`,
			[]string{"1:18: 511 doesn't fit in a Byte, which holds 0 to 255 @ 0x1FF"}},
		{"string patterns over Bytes and a List(Byte)", `fn f(src: Bytes, xs: List(Byte)) -> Int {
  let a = match src {
    "let" | "var" => 1
    ["..=", ..] => 2
    ['.', ..rest] => rest.size()
    _ => 0
  }
  let b = match xs {
    "x" => 1
    _ => 0
  }
  a + b
}`, nil},
		{"a string pattern over Bytes still needs every case", `fn f(src: Bytes) -> Int = match src {
  "let" => 1
  ["a", ..] => 2
}`, []string{"1:27: this match doesn't handle []: add an arm for it"}},
		{"a string pattern over a String stays a String", `fn f(s: String) -> Int = match s {
  "let" => 1
  _ => 0
}`, nil},
		{"toBytes takes a List(Byte)", `fn f() -> Option(String) {
  let ok = toBytes([0xC3, 0xA9]).toText()
  let bad = toBytes(["a"])
  ok
}`, []string{`3:22: expected Byte, found String`}},
		{"a variant's number", `type K = A | B | C
fn f(k: K) -> Int = k.index() + B.index()`, nil},
		{"index() needs a type whose variants have no fields", `type S = Circle(r: Int) | Dot
type R = { x: Int }
fn f(s: S, r: R) -> Int = s.index() + r.index()`, []string{
			"3:27: index() is derived only for a type whose variants have no fields, and its variant Circle has fields",
			"3:39: index() is derived only for a type whose variants have no fields, and R is a record",
		}},
		{"a type's own index method is called instead", `type K = A | B
impl K {
  fn index(self) -> String = "k"
}
fn f(k: K) -> String = k.index()`, nil},
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
				got = append(got, strings.TrimPrefix(d.String(), filepath.Join(dir, "main.eye")+":")+" @ "+d.Pos.Text())
			}
			if len(got) != len(c.want) {
				t.Errorf("got %d diagnostics, want %d:\n%s", len(got), len(c.want), strings.Join(got, "\n"))
			}
			for _, w := range c.want {
				msg, span, hasSpan := strings.Cut(w, " @ ")
				found := false
				for _, g := range got {
					gmsg, gspan, _ := strings.Cut(g, " @ ")
					found = found || strings.HasPrefix(gmsg, msg) && (!hasSpan || gspan == span)
				}
				if !found {
					t.Errorf("missing %q in:\n%s", w, strings.Join(got, "\n"))
				}
			}
		})
	}
}
