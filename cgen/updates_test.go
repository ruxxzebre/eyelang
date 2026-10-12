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
// section 2). The interpreter runs each in linear time. The C back end
// decides counting while it prints C and updates in place only six exact
// `v = op(v, …)` shapes, so each step of these loops copies the whole
// list or map, and the loop is quadratic.
//
// quadratic records what TestSlowLoops measures, not what is wanted: the
// three slow loops are known quadratic until 7.5 (counting in the low IR)
// makes them linear. When a change makes one linear, the test fails and
// says to set quadratic: false for it here. From then on the test keeps it
// linear, as it keeps "add", the in-place append that shows the timing
// tells the two apart.
var slowLoops = []struct {
	loop         string // the loop's name on updates' command line
	small, large int    // the two sizes timed; large is 4 × small
	quadratic    bool
}{
	{"add", 250_000, 1_000_000, false}, // xs = xs.add(i), in place since 7.4: the control
	{"append", 2_500, 10_000, true},    // xs = push(xs, i), fn push(xs, x) = xs.add(x)
	{"insert", 1_000, 4_000, true},     // let next = m.insert(i, i); m = next
	{"with", 2_500, 10_000, true},      // p = { p with items: p.items.add(i) }
}

// TestSlowLoops builds testdata/updates' main at -O2, times each slow loop
// at two sizes, and works out how its time grows with the size: as size^k,
// with k near 1 for a linear loop and near 2 for a quadratic one. Each time
// is the fastest of three runs, and k = 1.5 divides the two, which leaves
// room for the noise of a shared machine. testdata/updates' tests check the
// loops' results on both engines; this test checks only their growth.
//
// Recorded on 2026-10-12 on a 4-core Xeon at 2.1 GHz, native at -O2: append
// 37 ms at 5k, 260 ms at 10k, 1.1 s at 20k; insert 0.49 s at 5k, 2.4 s at
// 10k, 10.6 s at 20k; with 45 ms at 5k, 248 ms at 10k, 1.07 s at 20k. The
// interpreter takes a few ms for each at these sizes.
func TestSlowLoops(t *testing.T) {
	needZig(t)
	if HostTarget() == "" {
		t.Skip("eye build doesn't target this machine")
	}
	exe := build(t, load(t, "testdata/updates"), "", false, false)
	fastest := func(loop string, n int) time.Duration {
		best := time.Duration(math.MaxInt64)
		for range 3 {
			start := time.Now()
			out, err := exec.Command(exe, loop, fmt.Sprint(n)).CombinedOutput()
			d := time.Since(start)
			if err != nil {
				t.Fatalf("updates %s %d: %v\n%s", loop, n, err, out)
			}
			if got := strings.TrimSpace(string(out)); got != fmt.Sprint(n) {
				t.Fatalf("updates %s %d printed %q, not %d", loop, n, got, n)
			}
			best = min(best, d)
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
