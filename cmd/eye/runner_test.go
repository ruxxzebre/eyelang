package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"eyelang/check"
	"eyelang/interp"
)

// TestMain lets the test binary stand in for the eye command, so a test can
// run eye as a process of its own and kill it.
func TestMain(m *testing.M) {
	if os.Getenv("EYE_AS_COMMAND") == "1" {
		os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

// eyeProcess starts eye as a process of its own.
func eyeProcess(t *testing.T, args ...string) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "EYE_AS_COMMAND=1")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return cmd, &out
}

// eye run stops cleanly on Ctrl-C, leaving its runs where they were.
func TestRunStopsOnInterrupt(t *testing.T) {
	dir := program(t, map[string]string{"main.eye": `
job nap() {
  sleep!(1 h)
}

pub fn main() {
  nap.start()
}
`})
	cmd, out := eyeProcess(t, "run", dir)
	journal := filepath.Join(dir, ".eye", "journal")
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if store, err := interp.ReadJournal(journal); err == nil && len(store.Runs()) == 1 {
			break
		}
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("eye run on Ctrl-C: %v\n%s", err, out)
	}
}

// The exit test of the durable runner: a job sleeps, the process is killed
// in the middle of the sleep, and when eye run starts again the job finishes
// when its sleep was meant to end.
func TestRunSurvivesKill(t *testing.T) {
	dir := program(t, map[string]string{"main.eye": `
entity Alarm(name: String) {
  state armed: Bool = false

  ## Starts ring once, however often main runs.
  pub fn arm() {
    if !armed {
      armed = true
      ring.start(name)
    }
  }
}

job ring(name: String) -> String {
  sleep!(3 s)
  "{name} rang"
}

pub fn main() uses {entity Alarm} {
  Alarm("kitchen").arm()
}
`})
	journal := filepath.Join(dir, ".eye", "journal")
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	runs := func() []interp.RunInfo {
		store, err := interp.ReadJournal(journal)
		if err != nil {
			return nil
		}
		return interp.Runs(prog, store)
	}

	first, out := eyeProcess(t, "run", dir)
	var wake time.Time
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if rs := runs(); len(rs) == 1 && rs[0].Status == "sleeping" {
			wake = *rs[0].WakeAt
			break
		}
	}
	if wake.IsZero() {
		_ = first.Process.Kill()
		t.Fatalf("the job never went to sleep; eye run said:\n%s", out)
	}
	time.Sleep(time.Until(wake) / 3)
	if err := first.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = first.Wait()
	if rs := runs(); rs[0].Status != "sleeping" {
		t.Fatalf("after the kill the run is %s, want sleeping", rs[0].Status)
	}

	second, out := eyeProcess(t, "run", dir)
	done := make(chan error, 1)
	go func() { done <- second.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("eye run after the restart: %v\n%s", err, out)
		}
	case <-time.After(10 * time.Second):
		_ = second.Process.Kill()
		t.Fatalf("eye run didn't finish after the restart:\n%s", out)
	}
	if late := time.Since(wake); late < 0 || late > 1500*time.Millisecond {
		t.Errorf("the job finished %v after its wake time, want on time", late)
	}
	rs := runs()
	if len(rs) != 1 || rs[0].Status != "done" || rs[0].Result != `"kitchen rang"` {
		t.Fatalf("runs after the restart: %+v", rs)
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"runs", dir, "1"}, &stdout, &stderr); code != 0 {
		t.Fatalf("eye runs: exit %d: %s", code, stderr.String())
	}
	for _, want := range []string{`#1 ring("kitchen")  done: "kitchen rang"`, "1. sleep, slept until"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("eye runs doesn't show %q:\n%s", want, stdout.String())
		}
	}
}
