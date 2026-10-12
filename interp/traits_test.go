package interp

import (
	"strings"
	"testing"
)

// TestShowImplInsideValues checks that an impl of Show is used wherever the
// value is written (SPECS.md §11): inside a list, an Option, a record and a
// tuple, and in expect's messages, not only on its own.
func TestShowImplInsideValues(t *testing.T) {
	prog := load(t, `
type Money = { cents: Int }

impl Show for Money {
  fn show(self) -> String = "€{self.cents / 100}"
}

type Line = { item: String, price: Money }

test "inside" {
  let m: Money = { cents: 250 }
  let l: Line = { item: "tea", price: m }
  expect("{m}" == "€2")
  expect("{[m]}" == "[€2]")
  expect("{Some(m)}" == "Some(€2)")
  expect("{l}" == "\{ item: \"tea\", price: €2 \}")
  expect("{[(1, l)]}" == "[(1, \{ item: \"tea\", price: €2 \})]")
}

test "in expect's message" {
  let m: Money = { cents: 250 }
  expect([m] == [])
}
`)
	rs := RunTests(prog, "")
	if rs[0].Failure != "" {
		t.Errorf("inside: %s", rs[0].Failure)
	}
	if want := "but [m] is [€2]"; !strings.Contains(rs[1].Failure, want) {
		t.Errorf("expect's message is %q, want it to contain %q", rs[1].Failure, want)
	}
}
