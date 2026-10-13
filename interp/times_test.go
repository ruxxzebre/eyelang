package interp

import "testing"

// TestFormat writes dates and times of day with strftime's directives
// (SPECS.md §20).
func TestFormat(t *testing.T) {
	prog := load(t, `
test "dates and times written out" {
  expect(2026-10-12.format("%A %-d %B %Y") == "Monday 12 October 2026")
  expect(2026-10-02.format("%d/%m") == "02/10")
  expect(07:40.format("%H:%M") == "07:40")
  expect(07:40:09.format("%-H:%M:%S") == "7:40:09")
  expect(19:05.format("%-I:%M %p") == "7:05 PM")
  expect(00:30.format("%I %p") == "12 AM")
  expect(07:40.format("100%% at %H") == "100% at 07")
}
`)
	rs := RunTests(prog, "")
	if len(rs) != 1 {
		t.Fatalf("%d tests ran, want 1", len(rs))
	}
	for _, r := range rs {
		if r.Failure != "" {
			t.Errorf("%s: %s", r.Name, r.Failure)
		}
	}
}

// TestDurationsAsNumbers turns durations into whole seconds and
// milliseconds, and divides them (SPECS.md §20).
func TestDurationsAsNumbers(t *testing.T) {
	prog := load(t, `
fn averageWait(total: Duration, runs: NonZero(Int)) -> Duration = total / runs

test "a duration as a number, and divided" {
  let late = 0 s - 1 ms
  expect((90 s).seconds() == 90 && (1500 ms).millis() == 1500)
  expect((1999 ms).seconds() == 1 && late.seconds() == -1 && late.millis() == -1)
  expect(2 h / 8 == 15 min)
  match nonzero(4) {
    Some(n) => expect(averageWait(1 h, n) == 15 min)
    None => unreachable("4 is not 0")
  }
  expect(15 min / 1 h == 0.25 && 3 h / 90 min == 2.0)
  expect(1 h / (0 s) > 1000000.0)
}
`)
	rs := RunTests(prog, "")
	if len(rs) != 1 {
		t.Fatalf("%d tests ran, want 1", len(rs))
	}
	for _, r := range rs {
		if r.Failure != "" {
			t.Errorf("%s: %s", r.Name, r.Failure)
		}
	}
}
