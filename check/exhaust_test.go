package check

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestExhaustiveRecursiveFields checks matches on types with several
// recursive fields. Splitting a field that no row tests used to make the
// search exponential: the tree match took 9.4 s and 7 GB. Fields bound by
// name are decided by the default matrix, so each match takes milliseconds.
func TestExhaustiveRecursiveFields(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{"a tree match that covers both variants", `type Tree = Leaf | Node(left: Tree, value: Int, right: Tree)
fn sum(t: Tree) -> Int = match t {
  Leaf => 0
  Node(left, value, right) => sum(left) + value + sum(right)
}`, nil},
		{"a node's fields bound by name or skipped", `type Tree = Leaf | Node(left: Tree, value: Int, right: Tree)
fn size(t: Tree) -> Int = match t {
  Node(l, _, r) => size(l) + 1 + size(r)
  _ => 0
}`, nil},
		{"three recursive fields, inside a tuple", `type T = Nil | Three(a: T, b: T, c: T)
fn f(p: (T, T)) -> Int = match p {
  (Nil, _) => 0
  (Three(a, b, c), Nil) => 1
  (Three(_, _, _), Three(x, y, z)) => 2
}`, nil},
		{"a missing variant with recursive fields is named", `type Tree = Leaf | Node(left: Tree, value: Int, right: Tree)
fn f(t: Tree) -> Int = match t {
  Leaf => 0
}`, []string{"2:24: this match doesn't handle Node(_, _, _): add an arm for it, or `_ => …`"}},
		{"a missing case below a tested field is named", `type Tree = Leaf | Node(left: Tree, value: Int, right: Tree)
fn f(t: Tree) -> Int = match t {
  Node(Node(_, _, _), _, Leaf) => 0
  Leaf => 1
}`, []string{"2:24: this match doesn't handle Node(Leaf, _, Leaf): add an arm for it, or `_ => …`"}},
		{"an untested column beside a tested one", `type Tree = Leaf | Node(left: Tree, value: Int, right: Tree)
fn f(t: Tree, b: Bool) -> Int = match (t, b) {
  (_, true) => 0
}`, []string{"2:33: this match doesn't handle (Leaf, false): add an arm for it, or `_ => …`"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			main := filepath.Join(dir, "main.eye")
			if err := os.WriteFile(main, []byte(tc.src+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			done := make(chan []string, 1)
			go func() {
				_, diags := Program(dir)
				var got []string
				for _, d := range diags {
					got = append(got, strings.TrimPrefix(d.String(), main+":"))
				}
				done <- got
			}()
			select {
			case got := <-done:
				if !slices.Equal(got, tc.want) {
					t.Errorf("got %q\nwant %q", got, tc.want)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("checking the match took more than 5 s: the exhaustiveness search grows with the type's recursion")
			}
		})
	}
}
