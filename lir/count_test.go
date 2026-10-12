package lir

import (
	"strings"
	"testing"

	"eyelang/check"
)

func countedProgram(t *testing.T, dir string) *Program {
	t.Helper()
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	low, errs := Lower(prog)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	Count(low)
	if problems := append(Verify(low), CheckCounts(low)...); len(problems) > 0 {
		t.Fatal(strings.Join(problems, "\n"))
	}
	return low
}

// TestCheckCountsCatches takes each Dup and Drop that Count put into the
// counting program out in turn, and checks that CheckCounts then reports a
// problem: a value used after its last reference went, or one left held.
func TestCheckCountsCatches(t *testing.T) {
	low := countedProgram(t, "../testdata/ir/counting")
	tried := 0
	for _, f := range low.Funcs {
		for _, b := range f.Blocks {
			for i := 0; i < len(b.Instrs); i++ {
				in := b.Instrs[i]
				switch in.(type) {
				case *Dup, *Drop:
				default:
					continue
				}
				saved := b.Instrs
				b.Instrs = append(append([]Instr(nil), saved[:i]...), saved[i+1:]...)
				if len(checkFunc(f, low.Layouts)) == 0 {
					t.Errorf("%s: b%d without `%s` still checks", f.Name, b.ID, instrString(in))
				}
				b.Instrs = saved
				tried++
			}
		}
	}
	if tried < 40 {
		t.Errorf("tried only %d dups and drops", tried)
	}
}

// TestSlowLoopsUpdateInPlace checks that the native plan's slow loops
// (cgen/testdata/updates) update in place: the append through a helper
// passes the list on to a parameter that owns it, the insert through a
// `let` is in place, and `{ p with items: p.items.add(i) }` takes the field
// out of p and updates both in place.
func TestSlowLoopsUpdateInPlace(t *testing.T) {
	low := countedProgram(t, "../cgen/testdata/updates")
	want := map[string][]string{
		"push":     {"inplace builtin add(%1.xs, %2.x)"},
		"appended": {"call push(%3.xs, %8.i)"},
		"inserted": {"inplace builtin insert(%3.m, %8.i, %8.i)"},
		"bagged":   {"take %5.p.items", "inplace builtin add(%14, %10.i)", "inplace {%5.p with count: %13, items: %15}"},
	}
	for _, f := range low.Funcs {
		lines, ok := want[f.Name]
		if !ok {
			continue
		}
		delete(want, f.Name)
		var b strings.Builder
		FprintFunc(&b, f)
		for _, l := range lines {
			if !strings.Contains(b.String(), l) {
				t.Errorf("%s has no `%s`:\n%s", f.Name, l, b.String())
			}
		}
		if f.Name == "push" && f.Modes[0] != Owned {
			t.Errorf("push borrows its list")
		}
	}
	for name := range want {
		t.Errorf("no function %s", name)
	}
}
