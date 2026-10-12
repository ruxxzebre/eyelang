package check

import (
	"strings"
	"testing"
)

// TestProblem checks `?` into a Problem and `.context(…)` (SPECS.md §10).
func TestProblem(t *testing.T) {
	ok := `type ParseError = | NotANumber(String)
fn parse(s: String) -> Result(Int, ParseError) = Err(NotANumber(s))
fn count(s: String) -> Result(Int, Problem) {
  let a = parseInt(s)?
  let b = parse(s)?
  let c = parse(s).context("reading {s}")?
  let d = count(s).context("again")?
  let e = count(s)?
  Ok(a + b + c + d + e)
}
pub fn main() -> Result((), Problem) uses {stdio} {
  let n = count("1").context("counting")?
  print("{n}")
  Ok(())
}`
	if got := diagnose(t, ok); len(got) > 0 {
		t.Errorf("unexpected diagnostics:\n%s", strings.Join(got, "\n"))
	}

	bad := `fn secretly() -> Result(Int, Secret(String)) = Err(secret("token"))
fn f() -> Result(Int, Problem) = Ok(secretly()?)
fn g() -> Result(Int, Problem) = Ok(secretly().context("reading")?)
fn h() -> Result(Int, String) = Ok(parseInt("1").context("reading")?)`
	want := []string{
		"2:47: ? into a Problem needs Show, and Secret(String) has no Show",
		"3:37: context needs e: Show, and e is Secret(String) here",
		"4:68: ? can't pass on a Problem from a function whose error is String",
	}
	got := diagnose(t, bad)
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
