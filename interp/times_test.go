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
