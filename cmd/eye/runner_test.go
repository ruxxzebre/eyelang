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
  nap.start(key: "nap")
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
	survivesKill(t, nil, func(dir string) (interp.Store, error) {
		return interp.ReadJournal(filepath.Join(dir, ".eye", "journal"))
	})
}

// The same on the Postgres store (SPECS.md §15): the runs live in the
// database, and nothing in the program's directory.
func TestRunSurvivesKillOnPostgres(t *testing.T) {
	url := testPostgres(t)
	survivesKill(t, []string{"--store", url}, func(string) (interp.Store, error) {
		return interp.ReadPostgres(url)
	})
}

// survivesKill runs the kill test with eye run's store arguments, reading
// the store with read.
func survivesKill(t *testing.T, storeArgs []string, read func(dir string) (interp.Store, error)) {
	dir := program(t, map[string]string{"main.eye": `
job ring(name: String) -> String {
  sleep!(3 s)
  "{name} rang"
}

## main runs on every start; the key makes it start ring once.
pub fn main() {
  ring.start("kitchen", key: "kitchen")
}
`})
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	runs := func() []interp.RunInfo {
		store, err := read(dir)
		if err != nil {
			return nil
		}
		return interp.Runs(prog, store)
	}
	args := append(append([]string{"run"}, storeArgs...), dir)

	first, out := eyeProcess(t, args...)
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

	second, out := eyeProcess(t, args...)
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
	if code := run(append(append([]string{"runs"}, storeArgs...), dir, "1"), &stdout, &stderr); code != 0 {
		t.Fatalf("eye runs: exit %d: %s", code, stderr.String())
	}
	for _, want := range []string{`#1 ring("kitchen")  done: "kitchen rang"`, "1. sleep, slept until"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("eye runs doesn't show %q:\n%s", want, stdout.String())
		}
	}
}

// eye runs DIR ID STEP shows a step's whole result.
func TestRunsShowsAStep(t *testing.T) {
	dir := program(t, map[string]string{"main.eye": `
job stamp() -> String {
  let at = now!()
  "stamped at {at}"
}

pub fn main() {
  stamp.start(key: "once")
}
`})
	var stdout, stderr bytes.Buffer
	if code := run([]string{"run", dir}, &stdout, &stderr); code != 0 {
		t.Fatalf("eye run: exit %d: %s", code, stderr.String())
	}
	stdout.Reset()
	if code := run([]string{"runs", dir, "1", "1"}, &stdout, &stderr); code != 0 {
		t.Fatalf("eye runs: exit %d: %s", code, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "#1 stamp()  done: ") || !strings.HasPrefix(lines[1], "    1. now → ") ||
		!strings.HasSuffix(lines[1], lines[2]) {
		t.Errorf("eye runs DIR 1 1 shows:\n%s", stdout.String())
	}
	stderr.Reset()
	if code := run([]string{"runs", dir, "1", "2"}, &stdout, &stderr); code == 0 || !strings.Contains(stderr.String(), "run 1 has 1 step, so it has no step 2") {
		t.Errorf("eye runs DIR 1 2: exit %d: %s", code, stderr.String())
	}
}

// main starts its jobs with fixed keys, so once they are done a second eye
// run starts nothing, and says so; eye run --fresh starts over, keeping
// the old journal as journal.old.
func TestRunFreshStartsOver(t *testing.T) {
	dir := program(t, map[string]string{"main.eye": `
job bake(name: String) -> String {
  log.info("baking {name}")
  "{name} is out of the oven"
}

pub fn main() {
  bake.start("rye", key: "rye")
}
`})
	journal := filepath.Join(dir, ".eye", "journal")
	eyeRun := func(args ...string) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := run(append(append([]string{"run"}, args...), dir), &stdout, &stderr); code != 0 {
			t.Fatalf("eye run %v: exit %d: %s", args, code, stderr.String())
		}
		return stdout.String() + stderr.String()
	}
	if out := eyeRun(); !strings.Contains(out, "baking rye") {
		t.Fatalf("the first run didn't bake:\n%s", out)
	}
	out := eyeRun()
	if strings.Contains(out, "baking rye") || !strings.Contains(out, "eye run: nothing to do: main started no new run, every run in the store has finished (1), and no job is scheduled; eye run --fresh starts over with an empty journal") {
		t.Errorf("the second run said:\n%s", out)
	}
	if out := eyeRun("--fresh"); !strings.Contains(out, "baking rye") || strings.Contains(out, "nothing to do") {
		t.Errorf("eye run --fresh said:\n%s", out)
	}
	if _, err := os.Stat(journal + ".old"); err != nil {
		t.Errorf("no old journal: %v", err)
	}
	old, err := interp.ReadJournal(journal + ".old")
	if err != nil || len(old.Runs()) != 1 {
		t.Errorf("the old journal holds %v, %v; want the first run", old, err)
	}
}

// --fresh never empties a database, means nothing with --memory, and moves
// nothing while another eye run holds the journal.
func TestRunFreshRefuses(t *testing.T) {
	dir := program(t, map[string]string{"main.eye": `
job bake() {}

pub fn main() {
  bake.start(key: "once")
}
`})
	for _, c := range []struct {
		args []string
		env  string
		want string
	}{
		{[]string{"--fresh", "--store", "postgres://app@db/app"}, "", "--fresh starts a journal over, and never empties a database; --store names one"},
		{[]string{"--fresh"}, "postgres://app@db/app", "--fresh starts a journal over, and never empties a database; $EYE_STORE names one"},
		{[]string{"--fresh", "--memory"}, "", "--fresh means nothing with --memory"},
	} {
		t.Setenv("EYE_STORE", c.env)
		var stdout, stderr bytes.Buffer
		if code := run(append(append([]string{"run"}, c.args...), dir), &stdout, &stderr); code != exitUsage || !strings.Contains(stderr.String(), c.want) {
			t.Errorf("eye run %v with EYE_STORE=%q: exit %d: %s", c.args, c.env, code, stderr.String())
		}
	}
	t.Setenv("EYE_STORE", "")

	journal := filepath.Join(dir, ".eye", "journal")
	held, err := interp.OpenJournal(journal)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	var stdout, stderr bytes.Buffer
	if code := run([]string{"run", "--fresh", dir}, &stdout, &stderr); code == 0 || !strings.Contains(stderr.String(), "is in use by another eye run") {
		t.Errorf("eye run --fresh on a held journal: exit %d: %s", code, stderr.String())
	}
	if _, err := os.Stat(journal + ".old"); !os.IsNotExist(err) {
		t.Errorf("the held journal was moved: %v", err)
	}
}

// TestRunRefusesAChangedJob pauses a run at its sleep, changes the
// statement it is paused at, and starts eye run again: it refuses to start,
// naming the job and how many runs wait in it (7.3, SPECS.md §15).
func TestRunRefusesAChangedJob(t *testing.T) {
	src := `
job wait(name: String) -> String {
  let start = now!()
  sleep!(1 h)
  "{name} waited from {start}"
}

pub fn main() {
  wait.start("a", key: "a")
}
`
	dir := program(t, map[string]string{"main.eye": src})
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	first, out := eyeProcess(t, "run", dir)
	paused := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline) && !paused; time.Sleep(20 * time.Millisecond) {
		if store, err := interp.ReadJournal(filepath.Join(dir, ".eye", "journal")); err == nil {
			rs := interp.Runs(prog, store)
			paused = len(rs) == 1 && rs[0].Status == "sleeping"
		}
	}
	_ = first.Process.Signal(syscall.SIGKILL)
	_ = first.Wait()
	if !paused {
		t.Fatalf("the run never paused; eye run said:\n%s", out)
	}

	changed := strings.Replace(src, "sleep!(1 h)", "sleep!(2 h)", 1)
	if err := os.WriteFile(filepath.Join(dir, "main.eye"), []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"run", dir}, &stdout, &stderr); code == 0 {
		t.Fatalf("eye run started on a job changed where its run is paused:\n%s%s", stdout.String(), stderr.String())
	}
	if want := "wait (1 paused run)"; !strings.Contains(stderr.String(), want) {
		t.Errorf("eye run said %q, want it to name %q", stderr.String(), want)
	}
}
