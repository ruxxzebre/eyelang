package check

import (
	"slices"
	"strings"
	"testing"
)

// TestTryReturnsFromItsFunction checks that `?` can return what it passes
// on (SPECS.md §10): the None of an Option needs a function, lambda or job
// that returns an Option, and the error of a Result one that returns a
// Result. A lambda's `?` returns from the lambda, so the lambda's own result
// decides. Each case lists every diagnostic the program has, exactly.
func TestTryReturnsFromItsFunction(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{"a function that returns an Int", `fn g(x: Option(Int)) -> Int = x? + 1`, []string{
			"1:32: ? on an Option returns its None from g, but g returns Int: give a default with ?? value, match the None, or return an Option",
		}},
		{"a function that returns nothing", `fn g(x: Option(Int)) {
  let n = x?
  print("{n}")
}`, []string{
			"2:12: ? on an Option returns its None from g, but g returns (): give a default with ?? value, match the None, or return an Option",
		}},
		{"an inferred function decides its result from its body", `fn g(x: Option(Int)) = x? + 1
fn h(x: Option(Int)) = Some(x? + 1)`, []string{
			"1:25: ? on an Option returns its None from g, but g returns Int: give a default with ?? value, match the None, or return an Option",
		}},
		{"a lambda returns from itself", `fn g(xs: List(Option(Int))) -> Option(List(Int)) {
  let ns = xs |> map(fn x: x? + 1)
  let ms = xs |> map(fn x: Some(x? + 1))
  Some(ns)
}`, []string{
			"2:29: ? on an Option returns its None from this lambda, but this lambda returns Int: give a default with ?? value, match the None, or return an Option",
		}},
		{"a job", `job sync(x: Option(Int)) {
  let n = x?
  print("{n}")
}`, []string{
			"2:12: ? on an Option returns its None from job sync, but job sync returns (): give a default with ?? value, match the None, or return an Option",
		}},
		{"a Result in a lambda returns from the lambda too", `fn g(xs: List(String)) -> Result(List(Int), String) {
  let ns = xs |> map(fn s: parseInt(s)? + 1)
  let ms = xs |> map(fn s: Ok(parseInt(s)? + 1))
  Ok(ns)
}`, []string{
			"2:39: ? on a Result needs a function that returns a Result, and this lambda returns Int",
		}},
		{"what is fine", `fn second(xs: List(Int)) -> Option(Int) = Some(xs[1]? * 2)
fn total(x: Option(Int)) -> Int = (x ?? 0) + 1
fn read(s: String) = Ok(parseInt(s)? * 2)
fn all(xs: List(Option(Int))) -> List(Option(Int)) = xs |> map(fn x: Some(x? * 2))`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := diagnose(t, c.src)
			if !slices.Equal(got, c.want) {
				t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(c.want, "\n"))
			}
		})
	}
}
