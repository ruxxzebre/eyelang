package interp

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

const tally = `
entity Counter(id: String) {
  state n: Int = 0

  pub fn add(k: Int) { n += k }
}

job count(id: String) {
  log.info("counting {id}")
  Counter(id).add!(2)
  sleep!(10 ms)
  log.info("woke")
}

pub fn main() uses {entity Counter, start count} {
  Counter("a").add(1)
  count.start("a", key: "a")
}

test "a call, a start, and a run that sleeps" {
  Counter("a").add(1)
  count.start("a")
  advance(5 min)
  expect(Counter("a").n == 3)
}
`

// crossingsOf writes the runs' counts as "name: kind=n …" lines.
func crossingsOf(runs []*RunCrossings) string {
	var b strings.Builder
	for _, r := range runs {
		b.WriteString(r.Name + ":")
		for _, k := range r.Kinds() {
			fmt.Fprintf(&b, " %s=%d", k, r.Counts[k])
		}
		b.WriteString("\n")
	}
	return b.String()
}

// TestCrossingsOfATest counts a test's crossings, its body's apart from
// its job's run, which crosses again when it wakes. The run encodes its
// entity call's state and step, two checkpoints, the sleep's step and its
// result; it decodes its argument on each of its two turns, the counter's
// state, the checkpoint it wakes at, and the sleep's step, which it goes
// over again, but doesn't call add or log again.
func TestCrossingsOfATest(t *testing.T) {
	prog := load(t, tally)
	rs, _, _ := RunTestsWith(prog, TestOptions{Crossings: true})
	if len(rs) != 1 || rs[0].Failure != "" {
		t.Fatalf("results = %+v", rs)
	}
	got := crossingsOf(rs[0].Crossings)
	want := "" +
		"the test: entity.call=1 entity.field=1 entity.ref=2 job.start=1 entity.method=1 encode=2 decode=1\n" +
		"job count (run 1): entity.call=1 entity.ref=1 step=1 log.info=2 run.step=2 entity.method=1 encode=6 decode=5\n"
	if got != want {
		t.Errorf("crossings:\n%s\nwant:\n%s", got, want)
	}
}

// TestCrossingsOfEyeRun counts main's crossings and the job run's, as eye
// run --crossings does, and that nothing is counted without it.
func TestCrossingsOfEyeRun(t *testing.T) {
	prog := load(t, tally)
	var out, errs bytes.Buffer
	c := NewCrossings("the runner")
	if code := Run(context.Background(), prog, RunOptions{Stdout: &out, Stderr: &errs, Crossings: c}); code != 0 {
		t.Fatalf("Run exited %d: %s", code, errs.String())
	}
	got := crossingsOf(c.Runs())
	want := "" +
		"main: entity.call=1 entity.ref=1 job.start=1 entity.method=1 encode=2\n" +
		"job count (run 1): entity.call=1 entity.ref=1 step=1 log.info=2 run.step=2 entity.method=1 encode=6 decode=5\n"
	if got != want {
		t.Errorf("crossings:\n%s\nwant:\n%s", got, want)
	}
	var b strings.Builder
	WriteCrossings(&b, c.Runs())
	if !strings.Contains(b.String(), "all runs: 25 crossings, 875ns at 35 ns") {
		t.Errorf("table:\n%s", b.String())
	}

	rs := RunTests(prog, "")
	if len(rs) != 1 || rs[0].Crossings != nil {
		t.Errorf("crossings counted without TestOptions.Crossings: %+v", rs)
	}
}
