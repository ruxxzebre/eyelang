package interp

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestASignalStopsMain checks that Ctrl-C or SIGTERM, which end Run's
// context, stop main while it waits, as they stop the runner when it is
// idle (SPECS.md §13): main unwinds at once, what it committed is in the
// journal, and the runner doesn't go on to the jobs.
func TestASignalStopsMain(t *testing.T) {
	for _, c := range []struct{ name, wait string }{
		{"a timer", `sleep(30 s)`},
		{"a task", `let (a, b) = both(fn { sleep(30 s) }, fn { sleep(20 s) })`},
		{"a channel in a nested group", `let (a, b) = both(fn { sleep(30 s) }, fn {
    taskGroup(fn group {
      let (send, receive) = channel(capacity: 1)
      receive.take()
    })
  })`},
	} {
		t.Run(strings.ReplaceAll(c.name, " ", "-"), func(t *testing.T) {
			prog := load(t, `
job ping() { sleep!(1 h) }

pub fn main() uses {stdio, wait, start ping} {
  ping.start(key: "ping")
  print("waiting")
  `+c.wait+`
  print("woke")
}
`)
			path := filepath.Join(t.TempDir(), "journal")
			j, err := OpenJournal(path)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			time.AfterFunc(100*time.Millisecond, cancel)
			var out, errs bytes.Buffer
			start := time.Now()
			code := Run(ctx, prog, RunOptions{Stdout: &out, Stderr: &errs, Store: j})
			j.Close()
			if elapsed := time.Since(start); elapsed > 5*time.Second {
				t.Errorf("Run returned after %v", elapsed)
			}
			if code != 0 || strings.TrimSpace(out.String()) != "waiting" || !strings.Contains(errs.String(), "stopped while main ran") {
				t.Errorf("exit %d, printed %q, stderr %q", code, out.String(), errs.String())
			}
			j, err = OpenJournal(path)
			if err != nil {
				t.Fatal(err)
			}
			defer j.Close()
			if data, ok := j.Run(1); !ok || decodeRun(data).Status != "sleeping" {
				t.Errorf("the run main started isn't in the journal, sleeping")
			}
		})
	}
}
