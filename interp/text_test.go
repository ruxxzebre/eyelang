package interp

import (
	"testing"
)

// TestAppendTextKeepsStrings checks that appending in place never changes a
// string read from a textBuf, nor an older textBuf, whichever of them is
// appended to next.
func TestAppendTextKeepsStrings(t *testing.T) {
	a := appendText("", "ab")
	s := a.string()
	b := appendText(a, "cd") // in place: a uses all of the array
	c := appendText(a, "XY") // copies: b holds a's next bytes
	d := appendText(b, "ef") // in place after b
	e := appendText(c, "!")  // in place after c, on c's own array
	f := appendText(b, "gh") // copies: d holds b's next bytes
	for _, check := range []struct {
		got, want string
	}{
		{s, "ab"}, {a.string(), "ab"}, {b.string(), "abcd"}, {c.string(), "abXY"},
		{d.string(), "abcdef"}, {e.string(), "abXY!"}, {f.string(), "abcdgh"},
	} {
		if check.got != check.want {
			t.Errorf("got %q, want %q", check.got, check.want)
		}
	}
	if &b.b[0] != &a.b[0] || &c.b[0] == &a.b[0] {
		t.Errorf("b should share a's array and c should not")
	}
}

// TestStringAppendKeepsValues runs programs that build strings with += and
// keep copies along the way: in a function, through a mut parameter, in a
// closure, as map keys, and in a job whose checkpoint saves the string.
func TestStringAppendKeepsValues(t *testing.T) {
	prog := load(t, `
fn shout(mut s: String) { s += "!" }

entity Log(id: Int) {
  state lines: List(String) = []
  pub fn add(line: String) { lines += [line] }
}

job build() {
  var out = "start"
  for i in 0..3 {
    out += ",{i}"
    sleep!(1 min)
    Log(1).add!(out)
  }
}

test "a copy of a string never sees a later +=" {
  var s = "a"
  s += "b"
  let old = s
  s += "c"
  var t = old
  t += "X"
  s = s + "d"
  t = t + "Y"
  expect(old == "ab" && s == "abcd" && t == "abXY")
  let now = s
  let keep = fn: old + now
  s += "e"
  expect(keep() == "ababcd" && s == "abcde")
  let none: Map(String, Int) = {}
  let m = none |> insert(old, 1) |> insert(s, 2)
  s += "f"
  expect(m["ab"] == Some(1) && m["abcde"] == Some(2) && m |> size == 2)
  expect(s |> size == 6 && s |> split("c") == ["ab", "def"] && "{s}!" == "abcdef!")
  var w = old
  shout(mut w)
  shout(mut w)
  w += "?"
  expect(w == "ab!!?" && old == "ab")
}

test "a string built across checkpoints keeps what it had" {
  build.start()
  advance(5 min)
  expect(Log(1).lines == ["start,0", "start,0,1", "start,0,1,2"])
}
`)
	for _, r := range RunTests(prog, "") {
		if r.Failure != "" {
			t.Errorf("%s: %s\n%s", r.Name, r.Failure, r.Log)
		}
	}
}
