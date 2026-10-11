package check

import (
	"strings"
	"testing"
)

// TestOffsetHint checks that an Int where a string's Offset goes is an
// error that says where offsets come from (SPECS.md §5).
func TestOffsetHint(t *testing.T) {
	src := `fn f(city: String) {
  let a = city.slice(0..(city |> size))
  let b = city |> indexOf("a", from: 2)
  let c = city.slice(city.start()..city.end())
}`
	hint := "a position in a string is an Offset, not an Int; it comes from s.start()"
	want := []string{
		"2:22: expected Range(Offset), found Range(Int): " + hint,
		"3:38: expected Offset, found Int: " + hint,
	}
	got := diagnose(t, src)
	if len(got) != len(want) {
		t.Errorf("want %d diagnostics, got:\n%s", len(want), strings.Join(got, "\n"))
	}
	for _, w := range want {
		found := false
		for _, g := range got {
			found = found || strings.HasPrefix(g, w)
		}
		if !found {
			t.Errorf("missing %q in:\n%s", w, strings.Join(got, "\n"))
		}
	}
}
