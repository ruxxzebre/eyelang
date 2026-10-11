package main

import (
	"bytes"
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
