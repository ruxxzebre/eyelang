package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"eyelang/check"
	"eyelang/interp"
)

// eye approve answers an approval a run of eye run waits for: the answer
// goes through the program's inbox, and the run goes on (SPECS.md §15).
func TestApproveAnswersARunningProgram(t *testing.T) {
	dir := program(t, map[string]string{"main.eye": `
job restart() -> String {
  match approval!("Restart the router?", from: "operator", within: 1 h) {
    Approved(_) => "restarted"
    Declined => "left alone"
    Expired => "nobody answered"
  }
}

pub fn main() {
  restart.start(key: "router")
}
`})
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	runs := func() []interp.RunInfo {
		store, err := interp.ReadJournal(filepath.Join(dir, ".eye", "journal"))
		if err != nil {
			return nil
		}
		return interp.Runs(prog, store)
	}

	var stdout, stderr bytes.Buffer
	cmd, out := eyeProcess(t, "run", dir)
	defer func() { _ = cmd.Process.Kill() }()
	waiting := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline) && !waiting; time.Sleep(20 * time.Millisecond) {
		rs := runs()
		waiting = len(rs) == 1 && rs[0].Approval == "Restart the router?"
	}
	if !waiting {
		t.Fatalf("the run never waited for its approval; eye run said:\n%s", out)
	}

	if code := run([]string{"approve", "--by", "ann", dir, "router"}, &stdout, &stderr); code != 0 {
		t.Fatalf("eye approve: exit %d, %s", code, stderr.String())
	}
	if want := "approved run 1 (restart): Restart the router?"; !strings.Contains(stdout.String(), want) {
		t.Errorf("eye approve said %q, want %q", stdout.String(), want)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("eye run: %v\n%s", err, out)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("eye run didn't finish after the answer; it said:\n%s", out)
	}
	rs := runs()
	if len(rs) != 1 || rs[0].Status != "done" || rs[0].Result != `"restarted"` {
		t.Errorf("the run after the answer: %+v", rs)
	}

	// With nothing left waiting, an answer would only sit in the inbox.
	stderr.Reset()
	if code := run([]string{"decline", dir, "router"}, &stdout, &stderr); code != exitErrors ||
		!strings.Contains(stderr.String(), "no run waits for an approval") {
		t.Errorf("decline with nothing waiting: exit %d, %s", code, stderr.String())
	}
}

// Around eye approve, under eye run: an approval nobody answers expires on
// the wall clock; one still waiting survives a restart of the runner with
// its deadline; and eye decline answers it after the restart.
func TestApprovalWaitsAcrossRestartsAndExpires(t *testing.T) {
	dir := program(t, map[string]string{"main.eye": `
job restart(what: String, within: Duration) -> String {
  match approval!("Restart the {what}?", from: "operator", within: within) {
    Approved(_) => "restarted"
    Declined => "left alone"
    Expired => "nobody answered"
  }
}

pub fn main() {
  restart.start("router", 1 h, key: "router")
  restart.start("printer", 2 s, key: "printer")
}
`})
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	runs := func() map[string]interp.RunInfo {
		store, err := interp.ReadJournal(filepath.Join(dir, ".eye", "journal"))
		if err != nil {
			return nil
		}
		out := map[string]interp.RunInfo{}
		for _, r := range interp.Runs(prog, store) {
			out[r.Args[0]] = r
		}
		return out
	}
	until := func(what string, ok func(map[string]interp.RunInfo) bool, out *bytes.Buffer) map[string]interp.RunInfo {
		t.Helper()
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
			if rs := runs(); ok(rs) {
				return rs
			}
		}
		t.Fatalf("%s didn't happen; runs: %+v\neye run said:\n%s", what, runs(), out)
		return nil
	}

	cmd, out := eyeProcess(t, "run", dir)
	defer func() { _ = cmd.Process.Kill() }()
	rs := until("both runs waiting", func(rs map[string]interp.RunInfo) bool {
		return rs[`"router"`].Approval == "Restart the router?" && rs[`"printer"`].Approval == "Restart the printer?"
	}, out)
	deadline := rs[`"router"`].WakeAt
	if deadline == nil || time.Until(*deadline) < 59*time.Minute {
		t.Errorf("the router's approval waits until %v, want about an hour from now", deadline)
	}
	until("the printer's approval expiring", func(rs map[string]interp.RunInfo) bool {
		return rs[`"printer"`].Status == "done" && rs[`"printer"`].Result == `"nobody answered"`
	}, out)

	// The runner stops; the router's run waits on in the journal.
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("eye run after an interrupt: %v\n%s", err, out)
	}
	cmd, out = eyeProcess(t, "run", dir)
	defer func() { _ = cmd.Process.Kill() }()
	rs = until("the router's run waiting again", func(rs map[string]interp.RunInfo) bool {
		return rs[`"router"`].Approval == "Restart the router?"
	}, out)
	if len(rs) != 2 || rs[`"router"`].WakeAt == nil || !rs[`"router"`].WakeAt.Equal(*deadline) {
		t.Errorf("after the restart: %+v, want the router's run waiting until %v and no new run", rs, deadline)
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"decline", dir, "router"}, &stdout, &stderr); code != 0 {
		t.Fatalf("eye decline: exit %d, %s", code, stderr.String())
	}
	if want := "declined run 1 (restart): Restart the router?"; !strings.Contains(stdout.String(), want) {
		t.Errorf("eye decline said %q, want %q", stdout.String(), want)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("eye run: %v\n%s", err, out)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("eye run didn't finish after the answer; it said:\n%s", out)
	}
	if r := runs()[`"router"`]; r.Status != "done" || r.Result != `"left alone"` {
		t.Errorf("the router's run after eye decline: %+v", r)
	}
}
