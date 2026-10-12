package cgen

import (
	"fmt"
	"math"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// slowLoops are the loops of testdata/updates whose update's last use is
// a call, a `let` or a record that still holds the old value: the slow
// cases of the native plan (docs/internals/reviews/native-plan-2026-10-12.md,
// section 2). The interpreter runs each in linear time. Until 7.5 the C
// back end decided counting while it printed C and updated in place only
// six exact `v = op(v, …)` shapes, so each step of these loops copied the
// whole list or map, and the loop was quadratic. Since 7.5 the low IR
// counts precisely (lir.Count): each update is the last use of its
// operand, so it is in place, and every loop is linear.
//
// quadratic records what TestSlowLoops measures, not what is wanted. When
// a change makes a quadratic loop linear, the test fails and says to set
// quadratic: false for it here; from then on the test keeps it linear, as
// it keeps "add", the in-place append that was linear first.
var slowLoops = []struct {
	loop         string // the loop's name on updates' command line
	small, large int    // the two sizes timed; large is 4 × small
	quadratic    bool
}{
	{"add", 1_000_000, 4_000_000, false},    // xs = xs.add(i), in place since 7.4: the control
	{"append", 1_000_000, 4_000_000, false}, // xs = push(xs, i), fn push(xs, x) = xs.add(x); linear since 7.5
	{"insert", 100_000, 400_000, false},     // let next = m.insert(i, i); m = next; linear since 7.5
	{"with", 1_000_000, 4_000_000, false},   // p = { p with items: p.items.add(i) }; linear since 7.5
}

// TestSlowLoops builds testdata/updates' main at -O2, times each slow loop
// at two sizes, and works out how its time grows with the size: as size^k,
// with k near 1 for a linear loop and near 2 for a quadratic one. Each time
// is the least CPU time of three runs, and k = 1.5 divides the two, which
// leaves room for the noise of a shared machine. testdata/updates' tests check the
// loops' results on both engines; this test checks only their growth.
//
// Recorded on 2026-10-12 on a 4-core Xeon at 2.1 GHz, native at -O2. Before
// 7.5: append 37 ms at 5k, 260 ms at 10k, 1.1 s at 20k; insert 0.49 s at
// 5k, 2.4 s at 10k, 10.6 s at 20k; with 45 ms at 5k, 248 ms at 10k, 1.07 s
// at 20k. After: append 8 ms at 1M and 30 ms at 4M; insert 7 ms at 100k and
// 32 ms at 400k; with 12 ms at 1M and 48 ms at 4M.
func TestSlowLoops(t *testing.T) {
	needZig(t)
	if HostTarget() == "" {
		t.Skip("eye build doesn't target this machine")
	}
	exe := build(t, load(t, "testdata/updates"), "", false, false)
	fastest := func(loop string, n int) time.Duration {
		best := time.Duration(math.MaxInt64)
		for range 3 {
			cmd := exec.Command(exe, loop, fmt.Sprint(n))
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("updates %s %d: %v\n%s", loop, n, err, out)
			}
			if got := strings.TrimSpace(string(out)); got != fmt.Sprint(n) {
				t.Fatalf("updates %s %d printed %q, not %d", loop, n, got, n)
			}
			// CPU time, not wall time: under `go test ./...` other
			// packages' tests share the machine, and waiting for a core
			// once made the linear control look like size^1.65.
			best = min(best, cmd.ProcessState.UserTime()+cmd.ProcessState.SystemTime())
		}
		return best
	}
	for _, c := range slowLoops {
		small, large := fastest(c.loop, c.small), fastest(c.loop, c.large)
		k := math.Log(float64(large)/float64(small)) / math.Log(float64(c.large)/float64(c.small))
		t.Logf("%s: %v at %d, %v at %d: grows as size^%.2f", c.loop, small, c.small, large, c.large, k)
		switch {
		case c.quadratic && k < 1.5:
			t.Errorf("the %s loop now grows linearly (size^%.2f): set quadratic: false for it in slowLoops, "+
				"and say in ROADMAP which task made it linear", c.loop, k)
		case !c.quadratic && k >= 1.5:
			t.Errorf("the %s loop grows quadratically again (size^%.2f): each step copies what it updates", c.loop, k)
		}
	}
}
