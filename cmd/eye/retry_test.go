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

// slowProgram's run of slow sleeps past its @deadline, so it ends as
// Failed(DeadlinePassed); a retry counts the deadline again, and replays
// the sleep, which has run out by then, so the retried run finishes.
const slowProgram = `
@deadline(1 s)
job slow() uses {stdio, wait} {
  sleep!(1500 ms)
  print("slow is done")
}

pub fn main() uses {start slow} {
  slow.start(key: "slow")
}
`

// runStatus reads a run's status from a store.
func runStatus(t *testing.T, dir string, read func() (interp.Store, error), id int64) string {
	t.Helper()
	st, err := read()
	if err != nil {
		return "unread: " + err.Error()
	}
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	for _, r := range interp.Runs(prog, st) {
		if r.ID == id {
			return r.Status + " " + r.Fault
		}
	}
	return "missing"
}

// eye runs --retry starts a failed run again in a store no eye run holds,
// and the next eye run carries it on (SPECS.md section 15).
func TestRunsRetry(t *testing.T) {
	dir := program(t, map[string]string{"main.eye": slowProgram})
	runsRetry(t, dir, nil, func() (interp.Store, error) {
		return interp.ReadJournal(filepath.Join(dir, ".eye", "journal"))
	})
}

// The same on the Postgres store.
func TestRunsRetryOnPostgres(t *testing.T) {
	url := testPostgres(t)
	dir := program(t, map[string]string{"main.eye": slowProgram})
	runsRetry(t, dir, []string{"--store", url}, func() (interp.Store, error) {
		return interp.ReadPostgres(url)
	})
}

func runsRetry(t *testing.T, dir string, storeArgs []string, read func() (interp.Store, error)) {
	eye := func(args ...string) (int, string, string) {
		var stdout, stderr bytes.Buffer
		cmd := args[0]
		code := run(append(append([]string{cmd}, storeArgs...), args[1:]...), &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}
	eye("run", dir)
	if status := runStatus(t, dir, read, 1); !strings.HasPrefix(status, "failed ") || !strings.Contains(status, "slow passed its @deadline") {
		t.Fatalf("after eye run, run 1 is %s", status)
	}
	if code, out, errOut := eye("runs", "--retry", "1", dir); code != 0 || out != "run 1 (slow) is ready again: eye run "+dir+" carries it on, replaying its 1 recorded step\n" {
		t.Fatalf("eye runs --retry 1: exit %d\n%s%s", code, out, errOut)
	}
	if code, _, errOut := eye("runs", "--retry", "1", dir); code == 0 || !strings.Contains(errOut, "run 1 (slow) hasn't failed: it is ready") {
		t.Errorf("a second retry: exit %d: %s", code, errOut)
	}
	if code, out, errOut := eye("run", dir); code != 0 || !strings.Contains(out, "slow is done") {
		t.Errorf("eye run after the retry: exit %d\n%s%s", code, out, errOut)
	}
	if status := runStatus(t, dir, read, 1); status != "done " {
		t.Errorf("after the retry, run 1 is %s", status)
	}
	if code, out, _ := eye("runs", "--retry-failed", dir); code != 0 || !strings.HasPrefix(out, "no failed run is one a retry can cure") {
		t.Errorf("eye runs --retry-failed with none: exit %d: %s", code, out)
	}
	if code, _, errOut := eye("runs", "--retry", "1", "--retry-failed", dir); code != exitUsage || !strings.Contains(errOut, "go alone") {
		t.Errorf("--retry with --retry-failed: exit %d: %s", code, errOut)
	}
}

// While eye run holds the store, eye runs --retry puts the retry in its
// inbox, and the runner starts the run within a second.
func TestRunsRetryWhileRunning(t *testing.T) {
	dir := program(t, map[string]string{"main.eye": `
@deadline(1 s)
job slow() uses {stdio, wait} {
  sleep!(1500 ms)
  print("slow is done")
}

job nap() {
  sleep!(1 h)
}

pub fn main() uses {start slow, start nap} {
  slow.start(key: "slow")
  nap.start(key: "nap")
}
`})
	read := func() (interp.Store, error) { return interp.ReadJournal(filepath.Join(dir, ".eye", "journal")) }
	cmd, out := eyeProcess(t, "run", dir)
	defer func() {
		_ = cmd.Process.Signal(os.Interrupt)
		_ = cmd.Wait()
	}()
	waitFor := func(want string) string {
		status := ""
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			if status = runStatus(t, dir, read, 1); strings.HasPrefix(status, want) {
				break
			}
		}
		return status
	}
	if status := waitFor("failed"); !strings.HasPrefix(status, "failed") {
		t.Fatalf("run 1 is %s, want failed\n%s", status, out)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"runs", "--retry", "1", dir}, &stdout, &stderr); code != 0 ||
		stdout.String() != "retrying run 1 (slow): eye run, which holds the store, starts it within a second, replaying its 1 recorded step\n" {
		t.Fatalf("eye runs --retry 1: exit %d\n%s%s", code, stdout.String(), stderr.String())
	}
	if status := waitFor("done"); status != "done " {
		t.Errorf("run 1 is %s after the retry, want done\n%s", status, out)
	}
	_ = cmd.Process.Signal(os.Interrupt)
	_ = cmd.Wait()
	if s := out.String(); !strings.Contains(s, "eye run: run 1 (slow) starts again, replaying its 1 recorded step") || !strings.Contains(s, "slow is done") {
		t.Errorf("eye run wrote:\n%s", s)
	}
}
