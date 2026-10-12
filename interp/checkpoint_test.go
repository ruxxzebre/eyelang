package interp

import (
	"strings"
	"testing"
	"time"

	"eyelang/ir"
)

// TestResumeAfterAChangeElsewhere pauses a run at a checkpoint, changes code
// the run isn't paused in, and resumes it: it finishes, from its variables
// and the statement it stopped at.
func TestResumeAfterAChangeElsewhere(t *testing.T) {
	v1 := `
entity Out(id: String) {
  state lines: List(String) = []
  pub fn add(s: String) { lines += [s] }
}

job wait(name: String) {
  let start = now!()
  sleep!(1 h)
  Out("x").add!("{name} from {start}")
}

fn other() -> Int = 1
`
	store, at := pausedRun(t, v1)
	v2 := strings.Replace(v1, "fn other() -> Int = 1", "fn other() -> Int = 2", 1)
	in := resumeIn(t, v2, store, at)
	in.rt.advance(ir.Duration(2 * time.Hour))
	data, _ := in.rt.store.Run(1)
	run := decodeRun(data)
	if run.Status != "done" {
		t.Fatalf("the run is %s: %s", run.Status, run.Fault)
	}
	state, _, _ := in.rt.load(&EntityRef{Entity: in.prog.Root.Entities["Out"], ID: "x"}, ir.Pos{})
	if lines, _ := state.get("lines"); !strings.Contains(show(lines), "a from 2026-") {
		t.Errorf("lines = %s, want the line the job wrote from its saved start", show(lines))
	}
}

// TestAChangedJobFaultsItsPausedRun changes the statement a run is paused at:
// the run faults instead of resuming at the wrong place, and the runner's
// check before it starts names the job.
func TestAChangedJobFaultsItsPausedRun(t *testing.T) {
	v1 := `
job wait(name: String) {
  let start = now!()
  sleep!(1 h)
  print("{name} from {start}")
}
`
	store, at := pausedRun(t, v1)
	v2 := strings.Replace(v1, "sleep!(1 h)", "sleep!(2 h)", 1)
	in := resumeIn(t, v2, store, at)
	err := in.rt.checkResumePoints()
	if err == nil || !strings.Contains(err.Error(), "wait (1 paused run)") {
		t.Errorf("the check before the runner starts said %v, want it to name wait and its 1 paused run", err)
	}
	in.rt.advance(ir.Duration(3 * time.Hour))
	data, _ := in.rt.store.Run(1)
	run := decodeRun(data)
	if run.Status != "failed" || !strings.Contains(run.Fault, "the code of wait changed while this run was paused") {
		t.Errorf("the run is %s with %q, want it failed because its code changed", run.Status, run.Fault)
	}
}

// TestAChangedHelperFaultsItsReplay pauses a run inside a function its job
// calls, where no checkpoint is taken, and swaps two of the function's
// steps. The resumed run replays its steps since the checkpoint, finds a
// step recorded for another call, and faults rather than hand the call that
// step's result (C.10); eye runs shows the fault.
func TestAChangedHelperFaultsItsReplay(t *testing.T) {
	v1 := `
entity Out(id: String) {
  state lines: List(String) = []
  pub fn add(s: String) { lines += [s] }
}

fn steps(name: String) {
  Out("first").add!(name)
  sleep!(1 h)
  Out("second").add!(name)
}

job wait(name: String) { steps!(name) }
`
	store, at := pausedRun(t, v1)
	v2 := strings.Replace(v1, `Out("first").add!(name)
  sleep!(1 h)`, `sleep!(1 h)
  Out("first").add!(name)`, 1)
	in := resumeIn(t, v2, store, at)
	in.rt.advance(ir.Duration(2 * time.Hour))
	data, _ := in.rt.store.Run(1)
	run := decodeRun(data)
	if want := `the code of wait changed while this run was paused: its step 1 called Out(first).add, and the code now calls sleep there`; run.Status != "failed" || !strings.Contains(run.Fault, want) {
		t.Fatalf("the run is %s with %q, want it failed with %q", run.Status, run.Fault, want)
	}
	var shown strings.Builder
	FormatRuns(&shown, Runs(in.prog, in.rt.store), false, at)
	if !strings.Contains(shown.String(), "failed: ") || !strings.Contains(shown.String(), "changed while this run was paused") {
		t.Errorf("eye runs doesn't show the fault:\n%s", shown.String())
	}
}

// TestADeadIterDoesntStopCheckpoints keeps an Iter in a variable the job no
// longer reads: checkpoints leave it out, since the checker refuses one read
// after a `!`, and the run still keeps few steps.
func TestADeadIterDoesntStopCheckpoints(t *testing.T) {
	prog := load(t, `
job count() -> Int {
  let doubled = [1, 2] |> iter() |> map(fn x: x * 2)
  let k = doubled |> toList() |> sum
  sleep!(1 min)
  var n = k
  while n < 50 {
    sleep!(1 min)
    n += 1
  }
  n
}
`)
	in := New(prog, &strings.Builder{}, &strings.Builder{})
	in.rt.startNow(prog.Root.Jobs["count"], nil, nil, ir.Pos{})
	in.rt.advance(ir.Duration(30 * time.Minute))
	data, _ := in.rt.store.Run(1)
	run := decodeRun(data)
	if run.Checkpoint == nil || len(run.Checkpoint.Unsaved) != 1 || len(run.Log) > 2 {
		t.Fatalf("halfway, the run has checkpoint %+v and %d steps since", run.Checkpoint, len(run.Log))
	}
	in.rt.advance(ir.Duration(time.Hour))
	data, _ = in.rt.store.Run(1)
	run = decodeRun(data)
	if run.Status != "done" || show(in.decode(run.Result, ir.Pos{})) != "50" {
		t.Errorf("the run is %s with %s, want done with 50", run.Status, run.Fault)
	}
}

// pausedRun starts a run of src's job wait("a") and lets it pause at its
// sleep. It returns the store and the time.
func pausedRun(t *testing.T, src string) (Store, time.Time) {
	t.Helper()
	prog := load(t, src)
	in := New(prog, &strings.Builder{}, &strings.Builder{})
	in.rt.startNow(prog.Root.Jobs["wait"], []Value{"a"}, nil, ir.Pos{})
	in.rt.advance(ir.Duration(time.Minute))
	data, _ := in.rt.store.Run(1)
	if run := decodeRun(data); run.Status != "sleeping" {
		t.Fatalf("the run is %s, want it sleeping", run.Status)
	}
	return in.rt.store, in.rt.now
}

// resumeIn loads src over store, as a runner started on changed code.
func resumeIn(t *testing.T, src string, store Store, now time.Time) *Interp {
	t.Helper()
	prog := load(t, src)
	in := New(prog, &strings.Builder{}, &strings.Builder{})
	in.rt.useStore(store)
	in.rt.now = now
	in.rt.restore()
	return in
}

// TestAClosureResumesFromItsCheckpoint holds a lambda and a named function
// across a `!` and resumes the run in a new interpreter: the checkpoint
// saved each as its code's name and what it captured (SPECS.md §15).
func TestAClosureResumesFromItsCheckpoint(t *testing.T) {
	src := `
entity Out(id: String) {
  state lines: List(String) = []
  pub fn add(s: String) { lines += [s] }
}

fn double(n: Int) -> Int = n * 2

job wait(name: String) {
  let n = 3
  let add = fn x: x + n
  let twice = double
  let start = now!()
  sleep!(1 h)
  Out("x").add!("{name} {add(1)} {twice(5)}")
}
`
	store, at := pausedRun(t, src)
	data, _ := store.Run(1)
	if run := decodeRun(data); run.Checkpoint == nil || len(run.Checkpoint.Unsaved) != 0 {
		t.Fatalf("the paused run has checkpoint %+v, want one that saved every variable", run.Checkpoint)
	}
	in := resumeIn(t, src, store, at)
	in.rt.advance(ir.Duration(2 * time.Hour))
	data, _ = in.rt.store.Run(1)
	if run := decodeRun(data); run.Status != "done" {
		t.Fatalf("the run is %s: %s", run.Status, run.Fault)
	}
	state, _, _ := in.rt.load(&EntityRef{Entity: in.prog.Root.Entities["Out"], ID: "x"}, ir.Pos{})
	if lines, _ := state.get("lines"); show(lines) != `["a 4 10"]` {
		t.Errorf("lines = %s, want the line the saved functions computed", show(lines))
	}
}

// TestAChangedLambdaFaultsItsPausedRun changes a lambda a paused run holds:
// the run faults when it resumes rather than call other code.
func TestAChangedLambdaFaultsItsPausedRun(t *testing.T) {
	v1 := `
job wait(name: String) {
  let add = fn x: x + 1
  let start = now!()
  sleep!(1 h)
  print("{name} {add(1)}")
}
`
	store, at := pausedRun(t, v1)
	v2 := strings.Replace(v1, "fn x: x + 1", "fn x: x + 2", 1)
	in := resumeIn(t, v2, store, at)
	in.rt.advance(ir.Duration(2 * time.Hour))
	data, _ := in.rt.store.Run(1)
	if run := decodeRun(data); run.Status != "failed" || !strings.Contains(run.Fault, "holds a function whose code changed") {
		t.Errorf("the run is %s with %q, want it failed because the lambda it holds changed", run.Status, run.Fault)
	}
}
