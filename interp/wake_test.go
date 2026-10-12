package interp

import (
	"encoding/json"
	"io"
	"slices"
	"testing"
	"time"

	"eyelang/ir"
)

// The cost of waking a run must not grow with the number of runs that wait:
// a service with 10,000 runs asleep wakes each in about the same time as one
// with 100.

const sleepers = `
entity Counter(id: Int) {
  state n: Int = 0

  pub fn bump() -> Int {
    n += 1
    n
  }
}

job sleeper(id: Int) {
  sleep!(1 h)
  Counter(id).bump!()
}
`

// startSleepers checks the program and starts n runs of sleeper, each
// waiting for an hour on the test clock.
func startSleepers(tb testing.TB, n int) *Interp {
	tb.Helper()
	prog := checkSource(tb, sleepers)
	in := New(prog, io.Discard, io.Discard)
	job := prog.Root.Jobs["sleeper"]
	for i := range n {
		in.rt.startNow(job, []Value{int64(i)}, nil, ir.Pos{})
	}
	return in
}

// TestWakingManyRuns starts 10,000 runs that sleep, and wakes them all.
// Each wake used to sort every waiting run's id and copy the store's whole
// event list, which took 25 s; then each commit copied the run's bucket map
// to replace the run in it, which took 1.5 to 2 s; now it takes under one.
func TestWakingManyRuns(t *testing.T) {
	if testing.Short() {
		t.Skip("starts 10,000 runs")
	}
	const n = 10_000
	in := startSleepers(t, n)
	if got := in.rt.index.waiting(); got != n {
		t.Fatalf("%d runs wait, want %d", got, n)
	}
	start := time.Now()
	in.rt.advance(ir.Duration(2 * time.Hour))
	took := time.Since(start)
	t.Logf("woke %d runs in %v, %v per run", n, took, took/n)
	if got := in.rt.index.waiting(); got != 0 {
		t.Errorf("%d runs still wait", got)
	}
	for _, id := range []int64{0, n / 2, n - 1} {
		state, _, _ := in.rt.load(&EntityRef{Entity: in.prog.Root.Entities["Counter"], ID: id}, ir.Pos{})
		if v, _ := state.get("n"); v != int64(1) {
			t.Errorf("Counter(%d).n = %v, want 1", id, v)
		}
	}
	if took > 20*time.Second {
		t.Errorf("waking %d runs took %v: the cost of a wake grows with the runs that wait", n, took)
	}
}

// TestWaitingIndex checks the order the index wakes runs in: by time, then
// in the order they started, as runs change what they wait for or stop
// waiting; and which runs an event wakes.
func TestWaitingIndex(t *testing.T) {
	ix := indexStore(newMemStore(), nil, nil)
	save := func(id int64, status string, wakeAt int64, event string) {
		data, err := json.Marshal(jobRun{ID: id, Job: "m.j", Status: status, WakeAt: wakeAt, WaitEvent: event})
		if err != nil {
			t.Fatal(err)
		}
		ix.Commit(&Commit{Runs: []RunWrite{{ID: id, Data: data}}})
	}
	// order lists the waiting runs in the order they wake, emptying a copy.
	order := func() []int64 {
		cp := indexStore(ix.Store, nil, nil)
		var ids []int64
		for {
			id, _, ok := cp.firstToWake()
			if !ok {
				return ids
			}
			ids = append(ids, id)
			cp.unwait(id, cp.handle(id))
		}
	}
	save(5, "sleeping", 300, "")
	save(3, "sleeping", 100, "")
	save(4, "sleeping", 100, "m.ev")
	save(1, "sleeping", 200, "runs/7,8")
	save(2, "ready", 0, "")
	if got, want := order(), []int64{3, 4, 1, 5}; !slices.Equal(got, want) {
		t.Errorf("wake order %v, want %v", got, want)
	}
	if id, _, _ := ix.firstToWake(); id != 3 {
		t.Errorf("first to wake is %d, want 3", id)
	}
	save(5, "sleeping", 50, "m.ev") // it waits again, for less time
	save(3, "done", 0, "")          // it stops waiting
	save(2, "sleeping", 100, "")
	if got, want := order(), []int64{5, 2, 4, 1}; !slices.Equal(got, want) {
		t.Errorf("after the changes, wake order %v, want %v", got, want)
	}
	for name, want := range map[string][]int64{"m.ev": {4, 5}, "run/8": {1}, "runs/7,8": {1}, "run/9": {}, "": {}} {
		if got := ix.waitingFor(name); !slices.Equal(got, want) {
			t.Errorf("%q wakes %v, want %v", name, got, want)
		}
	}
	save(1, "sleeping", 200, "m.other")
	if got := ix.waitingFor("run/7"); len(got) != 0 {
		t.Errorf("run/7 still wakes %v after run 1 stopped waiting for it", got)
	}
	for _, id := range []int64{1, 2, 4, 5} {
		save(id, "failed", 0, "")
	}
	if len(ix.timers.ids) != 0 || len(ix.byEvent) != 0 {
		t.Errorf("no run waits, and the index holds %v, %v", ix.timers.ids, ix.byEvent)
	}
}

// BenchmarkWake reports the time to wake one run when 1,000 and 10,000 runs
// wait: the two should be about the same.
//
//	go test ./interp -run '^$' -bench Wake
func BenchmarkWake(b *testing.B) {
	for _, n := range []int{1_000, 10_000} {
		b.Run(itoa(int64(n))+" runs", func(b *testing.B) {
			var took time.Duration
			for range b.N {
				b.StopTimer()
				in := startSleepers(b, n)
				b.StartTimer()
				start := time.Now()
				in.rt.advance(ir.Duration(2 * time.Hour))
				took += time.Since(start)
			}
			b.ReportMetric(float64(took.Nanoseconds())/float64(b.N*n), "ns/wake")
		})
	}
}
