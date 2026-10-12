package interp

import (
	"strings"
	"testing"
	"time"

	"eyelang/ir"
)

// approve(…) in a test refuses text that more than one waiting approval
// contains, and text that none does, rather than answer the wrong one
// (SPECS.md §15).
func TestApproveInATestNeedsOneApproval(t *testing.T) {
	src := `
job ask(id: String) {
  match approval!("Restart the {id}?", from: "operator", within: 1 h) {
    Approved(_) => ()
    Declined | Expired => ()
  }
}

test "several" {
  ask.start("router")
  ask.start("switch")
  approve("Restart the")
}

test "none" {
  ask.start("router")
  decline("Restart the printer")
}
`
	results := RunTests(load(t, src), "")
	want := map[string]string{
		"several": `approve("Restart the") matches 2 waiting approvals, so it answers none: "Restart the router?" (run #1 of ask), "Restart the switch?" (run #2 of ask)`,
		"none":    `decline("Restart the printer"): no run waits for an approval whose text contains "Restart the printer"`,
	}
	for _, r := range results {
		if !strings.Contains(r.Failure, want[r.Name]) {
			t.Errorf("%s: failure %q, want %q", r.Name, r.Failure, want[r.Name])
		}
	}
}

// An answer from someone other than the person asked is refused and logged
// once, when it is published, however often the run looks at it again.
func TestARefusedAnswerIsLoggedOnce(t *testing.T) {
	src := `
job ask() {
  match approval!("Restart the router?", from: "operator", within: 1 h) {
    Approved(_) => ()
    Declined | Expired => ()
  }
}

test "t" {
  ask.start()
  approve("Restart the router?", by: "ann")
  advance(30 min)
  approve("Restart the router?")
}
`
	r := RunTests(load(t, src), "")[0]
	if r.Failure != "" {
		t.Fatal(r.Failure)
	}
	want := `refused an answer to approval 1-0 ("Restart the router?", run #1 of ask) by ann: only operator, whom it asks, may answer it; the run keeps waiting`
	if n := strings.Count(r.Log, want); n != 1 {
		t.Errorf("logged %d times, want once:\n%s", n, r.Log)
	}
}

// A run that waits for an approval after a checkpoint resumes after a
// restart and is answered by the id it published: the step's number counts
// the steps the checkpoint dropped. The person asked must still answer.
func TestAnApprovalIsAnsweredByIDAfterARestart(t *testing.T) {
	src := `
entity Out(id: String) {
  state lines: List(String) = []
  pub fn add(s: String) { lines += [s] }
}

job wait(name: String) {
  Out("x").add!("one")
  Out("x").add!("two")
  match approval!("Restart the {name}?", from: "operator", within: 1 h) {
    Approved(ok) => Out("x").add!("{ok.by} approved {ok.what}")
    Declined | Expired => Out("x").add!("left alone")
  }
}
`
	prog := load(t, src)
	in := New(prog, &strings.Builder{}, &strings.Builder{})
	in.rt.startNow(prog.Root.Jobs["wait"], []Value{"router"}, nil, ir.Pos{})
	in.rt.advance(ir.Duration(time.Minute))
	before := in.rt.waitingApprovals()
	if len(before) != 1 || before[0].from != "operator" {
		t.Fatalf("waiting before the restart: %+v", before)
	}
	data, _ := in.rt.store.Run(1)
	if run := decodeRun(data); run.Checkpoint == nil || run.Checkpoint.Steps == 0 {
		t.Fatalf("the run kept no checkpoint before its approval: %s", data)
	}

	var log strings.Builder
	again := New(load(t, src), &strings.Builder{}, &log)
	again.rt.useStore(in.rt.store)
	again.rt.now = in.rt.now
	again.rt.restore()
	after := again.rt.waitingApprovals()
	if len(after) != 1 || after[0] != before[0] {
		t.Fatalf("waiting after the restart: %+v, want %+v", after, before)
	}
	if want := "1-2"; after[0].id != want {
		t.Errorf("the request's id is %s, want %s: the run's third step", after[0].id, want)
	}

	again.rt.answer(after[0].id, true, "ann")
	if !strings.Contains(log.String(), "refused an answer to approval 1-2") {
		t.Errorf("the answer from ann wasn't logged as refused: %q", log.String())
	}
	data, _ = again.rt.store.Run(1)
	if run := decodeRun(data); run.Status != "sleeping" {
		t.Fatalf("after ann's answer the run is %s, want it still waiting", run.Status)
	}
	again.rt.answer(after[0].id, true, "operator")
	data, _ = again.rt.store.Run(1)
	if run := decodeRun(data); run.Status != "done" {
		t.Fatalf("after the operator's answer the run is %s: %s", run.Status, run.Fault)
	}
	state, _, _ := again.rt.load(&EntityRef{Entity: again.prog.Root.Entities["Out"], ID: "x"}, ir.Pos{})
	if lines, _ := state.get("lines"); !strings.Contains(show(lines), "operator approved Restart the router?") {
		t.Errorf("lines = %s, want the approval's by and what", show(lines))
	}
}

// A runner whose journal eye approve couldn't read finds the approval an
// inbox answer's text names itself: it drops text that several approvals
// contain, and answers as the person asked when the answer names no one.
func TestTheInboxAnswersOneApprovalByText(t *testing.T) {
	src := `
entity Out(id: String) {
  state lines: List(String) = []
  pub fn add(s: String) { lines += [s] }
}

job ask(name: String) {
  match approval!("Restart the {name}?", from: "operator", within: 1 h) {
    Approved(ok) => Out("x").add!("{ok.by} approved {ok.what}")
    Declined | Expired => Out("x").add!("left {name} alone")
  }
}
`
	prog := load(t, src)
	in := New(prog, &strings.Builder{}, &strings.Builder{})
	in.rt.startNow(prog.Root.Jobs["ask"], []Value{"router"}, nil, ir.Pos{})
	in.rt.startNow(prog.Root.Jobs["ask"], []Value{"switch"}, nil, ir.Pos{})
	dir := t.TempDir()
	var reports []string
	report := func(s string) { reports = append(reports, s) }

	if err := SendAnswer(dir, InboxAnswer{What: "Restart the", Approved: true}); err != nil {
		t.Fatal(err)
	}
	in.rt.readInbox(dir, report)
	if len(reports) != 1 || !strings.Contains(reports[0], `2 approvals contain "Restart the" (1-0, 2-0), and an answer goes to one`) {
		t.Errorf("reports %q", reports)
	}
	if n := len(in.rt.waitingApprovals()); n != 2 {
		t.Errorf("%d approvals wait after an ambiguous answer, want 2", n)
	}

	if err := SendAnswer(dir, InboxAnswer{What: "switch", Approved: true}); err != nil {
		t.Fatal(err)
	}
	in.rt.readInbox(dir, report)
	state, _, _ := in.rt.load(&EntityRef{Entity: prog.Root.Entities["Out"], ID: "x"}, ir.Pos{})
	if lines, _ := state.get("lines"); show(lines) != `["operator approved Restart the switch?"]` {
		t.Errorf("lines = %s, want the switch approved by the operator", show(lines))
	}
}
