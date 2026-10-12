package interp

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"eyelang/check"
)

// runOneTest checks src, which has one test, and runs it on a test's clock.
func runOneTest(t *testing.T, src string) TestResult {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.eye"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		t.Fatalf("%v", diags)
	}
	return RunTests(prog, "")[0]
}

// On a test's clock, a task that fails cancels the rest of its group at
// their next wait, and the group fails with it.
func TestAFailingTaskCancelsItsGroup(t *testing.T) {
	r := runOneTest(t, `
test "t" {
  taskGroup(fn group {
    group.spawn(fn {
      sleep(1 min)
      unreachable("broken")
    })
    group.spawn(fn {
      sleep(5 min)
      print("late")
    })
    print("body done")
  })
  print("after the group")
}
`)
	if !strings.Contains(r.Failure, "broken") {
		t.Errorf("failure %q, want the task's", r.Failure)
	}
	if strings.Contains(r.Log, "late") || strings.Contains(r.Log, "after the group") {
		t.Errorf("printed %q: the other task, or the code after the group, ran", r.Log)
	}
	if !strings.Contains(r.Log, "body done") {
		t.Errorf("printed %q: the group's own function didn't run", r.Log)
	}
}

// A group whose tasks all wait for channels that nobody can put into or
// close fails, at the group, instead of waiting forever.
func TestAGroupStuckOnChannelsFails(t *testing.T) {
	for _, src := range []string{
		// The producer forgets to close, so the reader waits for more.
		`
test "t" {
  taskGroup(fn group {
    let (send, receive) = channel(capacity: 4)
    group.spawn(fn { send.put(1) })
    for n in receive { print(n) }
  })
}
`,
		// Inside another task, the group waits for its owner's siblings,
		// which finish without helping.
		`
test "t" {
  let (a, b) = both(fn {
    taskGroup(fn group {
      let (send, receive) = channel(capacity: 1)
      receive.take()
    })
  }, fn { sleep(1 min) })
}
`,
	} {
		r := runOneTest(t, src)
		if !strings.Contains(r.Failure, "waits for a channel that no task will put into or close") {
			t.Errorf("%s\nfailure %q", src, r.Failure)
		}
	}
}

// A task in one group can wake a task in a group nested in another task:
// the nested group waits for it rather than failing.
func TestANestedGroupWaitsForTasksOutside(t *testing.T) {
	r := runOneTest(t, `
test "t" {
  let (send, receive) = channel(capacity: 1)
  let (got, _) = both(fn {
    taskGroup(fn group { receive.take() })
  }, fn {
    sleep(1 min)
    send.put("hello")
  })
  expect(got == Some("hello"))
}
`)
	if r.Failure != "" {
		t.Errorf("failure %q", r.Failure)
	}
}

// On the system clock, groups and channels run on goroutines: a producer
// and a reader take turns over a channel, a failing task ends the group at
// once, and a reader nobody will close the channel for is reported.
func TestGroupsOnTheSystemClock(t *testing.T) {
	for _, c := range []struct{ src, out, err string }{
		{`
pub fn main() uses {stdio, clock} {
  let total = taskGroup(fn group {
    let (send, receive) = channel(capacity: 1)
    group.spawn(fn {
      for i in 1..=100 { send.put(i) }
      send.close()
    })
    var sum = 0
    for n in receive { sum += n }
    sum
  })
  print(total)
}
`, "5050", ""},
		{`
pub fn main() uses {stdio, clock, wait} {
  taskGroup(fn group {
    group.spawn(fn {
      sleep(10 ms)
      unreachable("broken")
    })
    group.spawn(fn { sleep(5 s) })
  })
}
`, "", "broken"},
		{`
pub fn main() uses {stdio, clock} {
  taskGroup(fn group {
    let (send, receive) = channel(capacity: 1)
    group.spawn(fn { send.put(1) })
    for n in receive { print(n) }
  })
}
`, "1", "waits for a channel that no task will put into or close"},
		// A group inside another task fails once the tasks outside it have
		// finished without helping (#138).
		{`
pub fn main() uses {stdio, wait} {
  let (a, b) = both(fn {
    taskGroup(fn group {
      let (send, receive) = channel(capacity: 1)
      receive.take()
    })
  }, fn { sleep(10 ms) })
  print("after")
}
`, "", "main.eye:4:5: every task here waits for a channel that no task will put into or close"},
		// It waits for a task outside that does put into its channel.
		{`
pub fn main() uses {stdio, wait} {
  let (send, receive) = channel(capacity: 1)
  let (got, _) = both(fn {
    taskGroup(fn group { receive.take() })
  }, fn {
    sleep(10 ms)
    send.put("hello")
  })
  print(got ?? "none")
}
`, "hello", ""},
		// A task outside any group, stuck while its siblings wait for their
		// own group, faults where it waits.
		{`
pub fn main() uses {stdio, wait} {
  let (send, receive) = channel(capacity: 1)
  let (got, _) = both(fn { receive.take() }, fn {
    taskGroup(fn group {
      group.spawn(fn { sleep(10 ms) })
    })
  })
  print("after")
}
`, "", "main.eye:4:28: this waits for a channel that no task will put into or close"},
	} {
		prog := loadFiles(t, map[string]string{"main.eye": c.src})
		var out, errs bytes.Buffer
		start := time.Now()
		code := Run(context.Background(), prog, RunOptions{Stdout: &out, Stderr: &errs})
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Errorf("%s\ntook %v", c.src, elapsed)
		}
		if got := strings.TrimSpace(out.String()); got != c.out {
			t.Errorf("%s\nprinted %q, want %q", c.src, got, c.out)
		}
		if c.err == "" && code != 0 || c.err != "" && (code != 1 || !strings.Contains(errs.String(), c.err)) {
			t.Errorf("%s\nexit %d, stderr %q", c.src, code, errs.String())
		}
	}
}
