package cgen

import (
	"strings"
	"testing"
)

// TestConstantDivision checks that / and % by a literal call the inline
// division by a constant, and that a variable divisor doesn't.
func TestConstantDivision(t *testing.T) {
	files, err := generate(t, "testdata/numbers", Options{Tests: true})
	if err != nil {
		t.Fatal(err)
	}
	c := files["m_main.c"]
	for _, want := range []string{"ey_int_div_k(", "ey_int_mod_k(", "INT64_C(1000))", "ey_int_div(", "ey_int_mod("} {
		if !strings.Contains(c, want) {
			t.Errorf("the C has no %s", want)
		}
	}
	for _, line := range strings.Split(c, "\n") {
		if strings.Contains(line, "_k(") && strings.Contains(line, "INT64_C(-1))") {
			t.Errorf("a division by -1, which can overflow, is inline: %s", strings.TrimSpace(line))
		}
	}
}
