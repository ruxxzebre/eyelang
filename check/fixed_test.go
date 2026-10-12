package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFixedWidthErrors checks the errors of SPECS.md §5's fixed-width
// integers and character literals as Bytes: each is reported once, at the
// exact span of what is wrong.
func TestFixedWidthErrors(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string // "line:column: message prefix", then the span's text after " @ "
	}{
		{"a literal too large for its type", `fn f() -> Byte = 300`,
			[]string{"1:18: 300 doesn't fit in a Byte, which holds 0 to 255 @ 300"}},
		{"a negative literal too small for its type", `fn f() {
  let ok: Int32 = -2147483648
  let low: Int32 = -2147483649
  let u: UInt32 = -1
}`, []string{
			"3:20: -2147483649 doesn't fit in an Int32, which holds -2147483648 to 2147483647 @ -2147483649",
			"4:19: -1 doesn't fit in a UInt32, which holds 0 to 4294967295 @ -1",
		}},
		{"a literal that becomes a fixed-width type later", `fn g(b: UInt64) = b
fn f() {
  let n = 18446744073709551616
  g(n)
  g(18446744073709551615)
}`, []string{"3:11: 18446744073709551616 doesn't fit in a UInt64, which holds 0 to 18446744073709551615 @ 18446744073709551616"}},
		{"a literal argument of a fixed-width method", `fn f(b: Byte) -> Byte = b.wrappingAdd(256)`,
			[]string{"1:39: 256 doesn't fit in a Byte, which holds 0 to 255 @ 256"}},
		{"a literal in a pattern", `fn f(b: Byte) -> Int = match b {
  300 => 1
  _ => 0
}`, []string{"2:3: 300 doesn't fit in a Byte, which holds 0 to 255 @ 300"}},
		{"a character outside ASCII where a Byte is expected", `fn f() -> Byte = 'é'
fn g(src: Bytes) -> Bool = src[0] == Some('ü')`, []string{
			"1:18: 'é' isn't ASCII, so it can't be a Byte: it is 2 bytes in UTF-8 @ 'é'",
			"2:43: 'ü' isn't ASCII, so it can't be a Byte: it is 2 bytes in UTF-8 @ 'ü'",
		}},
		{"a character outside ASCII in a pattern over Bytes", `fn f(src: Bytes) -> Int = match src {
  ['é', ..] => 1
  _ => 0
}`, []string{"2:4: 'é' isn't ASCII, so it can't be a Byte: it is 2 bytes in UTF-8 @ 'é'"}},
		{"arithmetic operators on fixed-width integers", `fn f(a: Byte, b: Int32) {
  let x = a + a
  let y = 1 * b
  let z = a / nonzero(2) ?? 1
}`, []string{
			"2:11: Byte has no +, since the result may not fit: write a.wrappingAdd(b) to wrap around, or a.checkedAdd(b) for None when it doesn't fit @ a + a",
			"3:11: Int32 has no *, since the result may not fit: write a.wrappingMul(b)",
			"4:11: Byte has no /: convert to an Int with .toInt(), which can't overflow",
		}},
		{"fixed-width methods on other types", `fn f(n: Int, ok: Bool) {
  let a = n.wrappingAdd(1)
  let b = n.xor(1)
  let c = ok.and(true)
  let d = "x".toInt()
}`, []string{
			"2:11: wrappingAdd takes a fixed-width integer (Byte, Int32, Int64, UInt32 or UInt64), and this is Int: an Int never overflows, so use +, - and * @ n",
			"3:11: xor takes a fixed-width integer (Byte, Int32, Int64, UInt32 or UInt64), and this is Int: an Int has no fixed width; convert it first, as UInt64.from(n)",
			"4:11: and takes a fixed-width integer (Byte, Int32, Int64, UInt32 or UInt64), and this is Bool: for Bools, write a && b",
			"5:11: toInt takes a Float or a fixed-width integer (Byte, Int32, Int64, UInt32 or UInt64), and this is String",
		}},
		{"both sides of a fixed-width method have one type", `fn f(a: Byte, b: Int32, n: Int) {
  let x = a.wrappingAdd(b)
  let y = a.shiftLeft(a)
  let z = Int32.from(a)
}`, []string{
			"2:25: expected Byte, found Int32",
			"3:23: expected Int, found Byte",
			"4:22: expected Int, found Byte",
		}},
		{"a built-in type's function that doesn't exist", `fn f() = Int32.parse("1")`,
			[]string{"1:10: Int32 has no function parse @ Int32"}},
		{"a well-typed use of each", `fn f(src: Bytes, i: Int) -> Option(Int) {
  let b = src[i]?
  let w: Int64 = -9223372036854775808
  let u: UInt64 = 18446744073709551615
  if b == '-' { return None }
  let x = b.wrappingAdd(1).xor(127).not().shiftLeft(2).toInt()
  let y = Byte.from(x)? |> checkedSub(1)
  let z = y?
  Some(z.toInt() + w.toInt() + u.toInt())
}`, nil},
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
