package interp

import (
	"bytes"
	"context"
	"fmt"
	goruntime "runtime"
	"strings"
	"testing"

	"eyelang/ir"
)

// Cheap tasks (SPECS.md §13): thousands of waiting tasks cost little memory.
// These tests start 10,000 tasks in one task group, let every one of them
// reach its wait, and measure the memory the program holds then against what
// it held before: the Go heap still live after a collection, and the
// goroutine stacks.
//
// A waiting task is a goroutine parked at its wait. On linux/amd64 with Go
// 1.27 it holds about 9 KiB, on either clock and in a race-detector build
// too: an 8 KiB stack and about 1 KiB of heap (its goroutine, its copy of the
// interpreter's per-call state, its closure, and its place in the scheduler
// and the channel). The stack is the tree-walking interpreter's Go frames
// between the task's start and its wait, about 3 KiB here, which Go rounds
// up to a power of two once it passes 4 KiB less a guard; each level of
// Eyelang code costs one eval frame, and each call of a function about
// 1 KiB more (evalCall, apply, callFunc). The budget leaves a quarter of
// room above that, but not a doubled stack.
const (
	manyTasks   = 10_000
	taskBudget  = 12 << 10               // bytes for one waiting task
	totalBudget = manyTasks * taskBudget // 120,000 KiB, about 117 MiB
)

// manyTasksSrc starts n tasks in a group. Each tells the group's own
// function that it has started, then waits: for an hour on the clock, or on
// the gate, a channel nothing is put into until the group's function opens
// it. Once all n have said so, every task is at its wait or about to be, and
// measure(n) runs; then the gate opens (one task takes the 0, the rest see it
// closed) and the group ends.
const manyTasksSrc = `
@host("test.measure")
extern fn measure(waiting: Int) uses {ffi}

fn waitMany(n: Int, onTimer: Bool) -> Int uses {ffi, wait} {
  taskGroup(fn group {
    let (ready, started) = channel(capacity: n)
    let (open, gate) = channel(capacity: 1)
    for i in 1..=n {
      group.spawn(fn {
        ready.put(i)
        if onTimer {
          sleep(1 h)
        } else {
          let _ = gate.take()
        }
      })
    }
    var count = 0
    while count < n {
      let _ = started.take()
      count += 1
    }
    measure(count)
    open.put(0)
    open.close()
    count
  })
}

test "10,000 tasks wait on a timer" {
  expect(waitMany(10_000, true) == 10_000)
}

test "10,000 tasks wait on a channel" {
  expect(waitMany(10_000, false) == 10_000)
}

pub fn main() uses {ffi, wait, stdio} {
  print(waitMany(10_000, false))
}
`

// heldMemory is what the program holds: the live heap after a collection,
// and the goroutine stacks.
func heldMemory() uint64 {
	goruntime.GC()
	var m goruntime.MemStats
	goruntime.ReadMemStats(&m)
	return m.HeapAlloc + m.StackInuse
}

// measureTasks gives measure(n) a host function that records the memory
// held while the tasks wait, and how many wait.
func measureTasks(t *testing.T) (during *uint64, waiting *int64) {
	t.Helper()
	during, waiting = new(uint64), new(int64)
	hostFuncs["test.measure"] = func(_ *Interp, params map[string]Value, _ ir.Pos) Value {
		*waiting, _ = params["waiting"].(int64)
		*during = heldMemory()
		return ir.Unit{}
	}
	t.Cleanup(func() { delete(hostFuncs, "test.measure") })
	return during, waiting
}

// checkTaskMemory checks the memory held above base while the tasks waited
// against the budget.
func checkTaskMemory(t *testing.T, base, during uint64, waiting int64) {
	t.Helper()
	if waiting != manyTasks {
		t.Fatalf("measured with %d tasks waiting, want %d", waiting, manyTasks)
	}
	var used uint64
	if during > base {
		used = during - base
	}
	per := used / manyTasks
	t.Logf("%d waiting tasks hold %d KiB, %d bytes each (budget %d KiB, %d bytes each)",
		manyTasks, used>>10, per, totalBudget>>10, taskBudget)
	if used > totalBudget {
		t.Errorf("%d waiting tasks hold %d KiB, %d bytes each: over the budget of %d KiB, %d bytes each",
			manyTasks, used>>10, per, totalBudget>>10, taskBudget)
	}
}

// On a test's clock, 10,000 tasks wait for an hour, or for a channel, within
// the budget.
func TestManyTasksWaitOnATestClock(t *testing.T) {
	if testing.Short() {
		t.Skip("starts 10,000 tasks")
	}
	prog := loadFiles(t, map[string]string{"main.eye": manyTasksSrc})
	for _, name := range []string{"on a timer", "on a channel"} {
		t.Run(strings.ReplaceAll(name, " ", "-"), func(t *testing.T) {
			during, waiting := measureTasks(t)
			base := heldMemory()
			results := RunTests(prog, name)
			if len(results) != 1 {
				t.Fatalf("ran %d tests, want 1", len(results))
			}
			if r := results[0]; r.Failure != "" {
				t.Fatalf("%s: %s\n%s", r.Name, r.Failure, r.Log)
			}
			checkTaskMemory(t, base, *during, *waiting)
		})
	}
}

// On the system clock, as under eye run, the tasks are goroutines that take
// turns holding the interpreter's lock; 10,000 of them wait for a channel
// within the budget.
func TestManyTasksWaitOnTheSystemClock(t *testing.T) {
	if testing.Short() {
		t.Skip("starts 10,000 tasks")
	}
	prog := loadFiles(t, map[string]string{"main.eye": manyTasksSrc})
	during, waiting := measureTasks(t)
	var out, errs bytes.Buffer
	base := heldMemory()
	if code := Run(context.Background(), prog, RunOptions{Stdout: &out, Stderr: &errs}); code != 0 {
		t.Fatalf("exit %d: %s", code, errs.String())
	}
	if got := strings.TrimSpace(out.String()); got != fmt.Sprint(manyTasks) {
		t.Errorf("printed %q", got)
	}
	checkTaskMemory(t, base, *during, *waiting)
}
