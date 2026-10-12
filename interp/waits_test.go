package interp

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"eyelang/ir"
)

// The runs of testdata/programs/waiting, stopped part way by a restart:
// each resumes from the journal and ends as it would have without one.

const waiting = `
entity Notes(id: String) {
  state lines: List(String) = []

  pub fn add(line: String) { lines += [line] }
}

job ask(what: String) -> Decision { approval!(what, from: "operator", within: 3 days) }

job timer(d: Duration) -> Decision {
  sleep!(d)
  Expired
}

job ship(build: String) {
  # The run keeps a checkpoint here, with runs, and resumes from it.
  let runs = [ask.start!("Ship {build}?"), timer.start!(1 h)]
  let line = match firstDone!(runs) {
    Completed(Approved(_)) => "{build} shipped"
    Completed(_) => "{build} timed out"
    Failed(fault) => "{build}: {fault}"
  }
  Notes("ship").add!(line)
}

job brew(name: String, d: Duration) -> String {
  sleep!(d)
  name
}

job breakfast() {
  let outcomes = allDone!([brew.start!("tea", 3 min), brew.start!("coffee", 5 min)])
  Notes("breakfast").add!("{outcomes}")
}

job deploy(name: String, takes: Duration) {
  lock!("prod", for: 30 min, fn {
    Notes("prod").add!("{name} starts")
    sleep!(takes)
    Notes("prod").add!("{name} ends")
  })
}
`

// journaled runs a program on a journal that it can close and open again,
// as eye run does when it stops and starts.
type journaled struct {
	t    *testing.T
	prog *ir.Program
	path string
	in   *Interp
	j    *Journal
}

func newJournaled(t *testing.T, src string) *journaled {
	t.Helper()
	r := &journaled{t: t, prog: load(t, src), path: filepath.Join(t.TempDir(), ".eye", "journal")}
	r.open(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC))
	t.Cleanup(func() { r.j.Close() })
	return r
}

func (r *journaled) open(now time.Time) {
	r.t.Helper()
	j, err := OpenJournal(r.path)
	if err != nil {
		r.t.Fatal(err)
	}
	r.in, r.j = New(r.prog, &strings.Builder{}, &strings.Builder{}), j
	r.in.rt.useStore(j)
	r.in.rt.now = now
	if err := r.in.rt.checkResumePoints(); err != nil {
		r.t.Fatal(err)
	}
	r.in.rt.restore()
}

// restart closes the journal and opens it in a new interpreter.
func (r *journaled) restart() {
	now := r.in.rt.now
	r.j.Close()
	r.open(now)
}

func (r *journaled) start(job string, args ...Value) {
	r.in.rt.startNow(r.prog.Root.Jobs[job], args, nil, ir.Pos{})
}

func (r *journaled) advance(d time.Duration) { r.in.rt.advance(ir.Duration(d)) }

// notes returns the lines of Notes(id).
func (r *journaled) notes(id string) string {
	state, _, _ := r.in.rt.load(&EntityRef{Entity: r.prog.Root.Entities["Notes"], ID: id}, ir.Pos{})
	lines, _ := state.get("lines")
	return show(lines)
}

// runs returns how each run is, as eye runs shows it: "ship sleeping
// waiting for …", "ask failed Killed".
func (r *journaled) runs() map[int64]RunInfo {
	out := map[int64]RunInfo{}
	for _, info := range Runs(r.prog, r.j) {
		out[info.ID] = info
	}
	return out
}

// approvalID returns the id of the waiting approval request that asks what.
func (r *journaled) approvalID(what string) string {
	r.t.Helper()
	for _, a := range r.in.rt.waitingApprovals() {
		if a.what == what {
			return a.id
		}
	}
	r.t.Fatalf("no run waits for the approval %q", what)
	return ""
}

func (r *journaled) run(id int64) *jobRun {
	data, _ := r.in.rt.store.Run(id)
	return decodeRun(data)
}

func TestFirstDoneResumesAfterARestart(t *testing.T) {
	t.Run("the approval comes first", func(t *testing.T) {
		r := newJournaled(t, waiting)
		r.start("ship", "v2")
		r.advance(10 * time.Minute)
		if w := r.runs()[1].Waits; w != "the first of runs #2, #3" {
			t.Errorf("eye runs says ship waits for %q", w)
		}
		if ship := r.run(1); ship.Checkpoint == nil || len(ship.Log) != 0 {
			t.Errorf("ship waits with checkpoint %+v and %d steps since, want it at firstDone! with none", ship.Checkpoint, len(ship.Log))
		}
		r.restart()
		r.in.rt.answer(r.approvalID("Ship v2?"), true, "operator")
		if got := r.notes("ship"); got != `["v2 shipped"]` {
			t.Errorf("after the restart and the approval, the notes are %s", got)
		}
		if timer := r.run(3); timer.Status != "failed" || timer.FaultKind != "Killed" {
			t.Errorf("the timer is %s (%s), want it stopped", timer.Status, timer.FaultKind)
		}
		r.advance(2 * time.Hour)
		if got := r.notes("ship"); got != `["v2 shipped"]` {
			t.Errorf("once the hour passed, the notes are %s", got)
		}
	})
	t.Run("the timeout comes first", func(t *testing.T) {
		r := newJournaled(t, waiting)
		r.start("ship", "v3")
		r.advance(30 * time.Minute)
		asked := r.approvalID("Ship v3?")
		r.restart()
		r.advance(30 * time.Minute)
		if got := r.notes("ship"); got != `["v3 timed out"]` {
			t.Errorf("after the restart and the hour, the notes are %s", got)
		}
		if ask := r.run(2); ask.Status != "failed" || ask.FaultKind != "Killed" {
			t.Errorf("the question is %s (%s), want it stopped", ask.Status, ask.FaultKind)
		}
		r.restart()
		r.in.rt.answer(asked, true, "operator")
		if got := r.notes("ship"); got != `["v3 timed out"]` {
			t.Errorf("an answer to the stopped question changed the notes to %s", got)
		}
	})
}

func TestAllDoneResumesAfterARestart(t *testing.T) {
	r := newJournaled(t, waiting)
	r.start("breakfast")
	r.advance(4 * time.Minute)
	if got := r.notes("breakfast"); got != "[]" {
		t.Fatalf("before the coffee is ready, the notes are %s", got)
	}
	if w := r.runs()[1].Waits; w != "run #3" {
		t.Errorf("eye runs says breakfast waits for %q, want the coffee's run", w)
	}
	r.restart()
	r.advance(time.Minute)
	if got := r.notes("breakfast"); got != `["[Completed(\"tea\"), Completed(\"coffee\")]"]` {
		t.Errorf("after the restart, the notes are %s", got)
	}
}

func TestALockSurvivesARestart(t *testing.T) {
	t.Run("the holder gives it back", func(t *testing.T) {
		r := newJournaled(t, waiting)
		r.start("deploy", "a", ir.Duration(10*time.Minute))
		r.start("deploy", "b", ir.Duration(10*time.Minute))
		r.advance(5 * time.Minute)
		if l, held := r.in.rt.lockHolder("prod"); !held || l.Owner != 1 {
			t.Fatalf("the lock is held by %+v (%v), want run 1", l, held)
		}
		if w := r.runs()[2].Waits; w != `the lock "prod"` {
			t.Errorf("eye runs says the second deploy waits for %q", w)
		}
		r.restart()
		if l, held := r.in.rt.lockHolder("prod"); !held || l.Owner != 1 {
			t.Fatalf("after the restart the lock is held by %+v (%v), want run 1", l, held)
		}
		r.advance(time.Hour)
		if got := r.notes("prod"); got != `["a starts", "a ends", "b starts", "b ends"]` {
			t.Errorf("the deploys ran as %s", got)
		}
		if _, held := r.in.rt.lockHolder("prod"); held {
			t.Error("the lock is still held once both deploys finished")
		}
	})
	t.Run("it runs out while its holder waits", func(t *testing.T) {
		r := newJournaled(t, waiting)
		r.start("deploy", "stuck", ir.Duration(2*time.Hour))
		r.start("deploy", "next", ir.Duration(5*time.Minute))
		r.advance(10 * time.Minute)
		r.restart()
		r.advance(20 * time.Minute)
		stuck := r.run(1)
		if stuck.Status != "failed" || stuck.FaultKind != "LockExpired" || stuck.FaultArg != "prod" {
			t.Errorf("the holder is %s with %s(%s), want it failed with LockExpired(prod)", stuck.Status, stuck.FaultKind, stuck.FaultArg)
		}
		if got := r.notes("prod"); got != `["stuck starts", "next starts"]` {
			t.Errorf("when the lock ran out the notes were %s", got)
		}
		r.restart()
		r.advance(time.Hour)
		if got := r.notes("prod"); got != `["stuck starts", "next starts", "next ends"]` {
			t.Errorf("the deploys ran as %s", got)
		}
		if _, held := r.in.rt.lockHolder("prod"); held {
			t.Error("the lock is still held once the deploy finished")
		}
	})
}
