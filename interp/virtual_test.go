package interp

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// TestVirtualClock runs a program whose job sleeps and whose schedule
// recurs on a clock that skips ahead, as the online playground runs one: the
// job finishes at once, and the schedule stops at the limit with a line
// saying so.
func TestVirtualClock(t *testing.T) {
	prog := loadFiles(t, map[string]string{"main.eye": `
job remind(who: String) uses {stdio, clock, wait} {
  sleep!(15 min)
  print("remind {who} at {now!()}")
}

@on(every(1 h))
job tick() uses {stdio, clock, wait} {
  print("tick at {now!()}")
}

pub fn main() uses {stdio, start remind} {
  print("start")
  remind.start("Ann", key: "ann")
}
`})
	start := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	var out, errs bytes.Buffer
	began := time.Now()
	code := Run(context.Background(), prog, RunOptions{
		Stdout: &out, Stderr: &errs, MemoryLimit: 1 << 30,
		Virtual: &VirtualClock{Start: start, Until: 3 * time.Hour},
	})
	if elapsed := time.Since(began); elapsed > 2*time.Second {
		t.Errorf("took %v; a virtual clock doesn't wait", elapsed)
	}
	if code != 0 {
		t.Fatalf("exit %d, stderr:\n%s", code, errs.String())
	}
	want := "start\n" +
		"remind Ann at 2026-10-12 09:15 in \"UTC\"\n" +
		"tick at 2026-10-12 10:00 in \"UTC\"\n" +
		"tick at 2026-10-12 11:00 in \"UTC\"\n" +
		"tick at 2026-10-12 12:00 in \"UTC\"\n"
	if out.String() != want {
		t.Errorf("printed\n%s\nwant\n%s", out.String(), want)
	}
	if !strings.Contains(errs.String(), "the virtual clock stopped at 2026-10-12 12:00:00 after the clock moved 3h0m0s; schedules go on") {
		t.Errorf("stderr doesn't say where the clock stopped:\n%s", errs.String())
	}

	// The limit on wakes stops a schedule that fires often.
	out.Reset()
	errs.Reset()
	Run(context.Background(), prog, RunOptions{
		Stdout: &out, Stderr: &errs, MemoryLimit: 1 << 30,
		Virtual: &VirtualClock{Start: start, Wakes: 2},
	})
	if n := strings.Count(out.String(), "\n"); n != 3 {
		t.Errorf("printed %d lines after 2 wakes, want 3:\n%s", n, out.String())
	}
	if !strings.Contains(errs.String(), "after 2 wakes") {
		t.Errorf("stderr doesn't name the limit on wakes:\n%s", errs.String())
	}
}
