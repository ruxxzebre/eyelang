package interp

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// deepSrc recurses ten million calls deep in a test and in a job: far more
// than the Go stack under the interpreter holds.
const deepSrc = `fn count(n: Int) -> Int = if n == 0 then 0 else 1 + count(n - 1)

job deep() -> Int { count(10_000_000) }

job other() { sleep!(2 s) }

pub fn main() uses {start deep, start other} {
  other.start(key: "other")
  deep.start(key: "deep")
}

test "deep" { expect(count(10_000_000) == 10_000_000) }

test "a run that recursed too deep" {
  deep.start()
  expect(count(90_000) == 90_000)
}

test "shallow" { expect(count(90_000) == 90_000) }
`

// TestRecursionTooDeepIsAFault checks that recursion deeper than maxDepth
// fails the test it is in, or the run, instead of overflowing the stack
// and ending the process (SPECS.md §15), and that the calls a fault unwinds
// don't count against what runs after it.
func TestRecursionTooDeepIsAFault(t *testing.T) {
	prog := load(t, deepSrc)
	rs := RunTests(prog, "")
	const msg = "recursion too deep: count was called with 100000 calls already unfinished"
	if !strings.Contains(rs[0].Failure, msg) {
		t.Errorf("deep: %q, want it to contain %q", rs[0].Failure, msg)
	}
	if f := rs[1].Failure; !strings.Contains(f, "job deep: "+msg) || strings.Count(f, msg) != 1 {
		t.Errorf("a run that recursed too deep: %q, want the job's fault, once", f)
	}
	if rs[2].Failure != "" {
		t.Errorf("shallow: %s", rs[2].Failure)
	}
}

// TestRecursionTooDeepFailsTheRun checks that under eye run, a run that
// recurses too deeply is kept as failed, the other runs carry on, and a
// restart doesn't run it again.
func TestRecursionTooDeepFailsTheRun(t *testing.T) {
	prog := load(t, deepSrc)
	path := filepath.Join(t.TempDir(), "journal")
	for start := range 2 {
		j, err := OpenJournal(path)
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		code := Run(context.Background(), prog, RunOptions{Stdout: &out, Stderr: &out, Store: j})
		j.Close()
		if start == 0 && (code != 1 || !strings.Contains(out.String(), "job deep: recursion too deep")) {
			t.Fatalf("eye run ended with %d:\n%s", code, out.String())
		}
		if start == 1 && (code != 0 || strings.Contains(out.String(), "recursion")) {
			t.Fatalf("the restart ended with %d:\n%s", code, out.String())
		}
	}
	j, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	for id, want := range map[int64]string{1: "done", 2: "failed"} {
		data, ok := j.Run(id)
		if !ok {
			t.Fatalf("no run %d", id)
		}
		if got := decodeRun(data).Status; got != want {
			t.Errorf("run %d is %s, want %s", id, got, want)
		}
	}
}
