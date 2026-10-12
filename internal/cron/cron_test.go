package cron

import (
	"fmt"
	"testing"
	"time"
)

func TestTimes(t *testing.T) {
	for _, c := range []struct{ line, want string }{
		{"30 7 * * *", "[{7 30}]"},
		{"*/20 7-8 * * *", "[{7 0} {7 20} {7 40} {8 0} {8 20} {8 40}]"},
		{"0,30 9 * * *", "[{9 0} {9 30}]"},
		{"15 8-18/5 * * *", "[{8 15} {13 15} {18 15}]"},
		{"5-7/2 0 * * *", "[{0 5} {0 7}]"},
	} {
		s, err := Parse(c.line)
		if err != nil {
			t.Fatalf("%q: %v", c.line, err)
		}
		if got := fmt.Sprint(s.Times()); got != c.want {
			t.Errorf("%q fires at %s, want %s", c.line, got, c.want)
		}
	}
	s, _ := Parse("* * * * *")
	if n := len(s.Times()); n != 24*60 {
		t.Errorf("* * * * * fires %d times a day, want 1440", n)
	}
}

func TestDays(t *testing.T) {
	// October 2026: the 1st is a Thursday, the 12th a Monday.
	for _, c := range []struct {
		line string
		days []int // the days of October 2026 it fires on, of the first 14
	}{
		{"0 0 * * *", []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14}},
		{"0 0 * * mon-fri", []int{1, 2, 5, 6, 7, 8, 9, 12, 13, 14}},
		{"0 0 * * 1-5", []int{1, 2, 5, 6, 7, 8, 9, 12, 13, 14}},
		{"0 0 * * SAT,sun", []int{3, 4, 10, 11}},
		{"0 0 * * 0", []int{4, 11}},
		{"0 0 * * 7", []int{4, 11}},
		{"0 0 * * 5-7", []int{2, 3, 4, 9, 10, 11}},
		{"0 0 1,13 * *", []int{1, 13}},
		{"0 0 */5 * *", []int{1, 6, 11}},
		{"0 0 13 * fri", []int{2, 9, 13}}, // either day field: the 13th, and Fridays
		{"0 0 */5 * tue", []int{6}},       // a field starting with * narrows: both must match
		{"0 0 * oct *", []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14}},
		{"0 0 * 1-9,11,12 *", nil},
	} {
		s, err := Parse(c.line)
		if err != nil {
			t.Fatalf("%q: %v", c.line, err)
		}
		var got []int
		for d := 1; d <= 14; d++ {
			if s.Day(2026, time.October, d) {
				got = append(got, d)
			}
		}
		if fmt.Sprint(got) != fmt.Sprint(c.days) {
			t.Errorf("%q fires on %v, want %v", c.line, got, c.days)
		}
	}
}

func TestErrors(t *testing.T) {
	for _, c := range []struct{ line, want string }{
		{"0 7 * *", `a cron line has five fields (minute, hour, day of month, month and day of week), and "0 7 * *" has 4`},
		{"0 0 7 * * *", `a cron line has five fields (minute, hour, day of month, month and day of week), and "0 0 7 * * *" has 6`},
		{"@daily", `"@daily" isn't a cron line: write daily(00:00, in: zone) or every(1 h) instead of a macro`},
		{"60 * * * *", "minute 60 is out of range: it goes from 0 to 59"},
		{"0 24 * * *", "hour 24 is out of range: it goes from 0 to 23"},
		{"0 0 0 * *", "day of month 0 is out of range: it goes from 1 to 31"},
		{"0 0 * 13 *", "month 13 is out of range: it goes from 1 to 12"},
		{"0 0 * * 8", "day of week 8 is out of range: it goes from 0 to 7"},
		{"0 0 * * monday", `day of week "monday" isn't a number or a name such as mon`},
		{"x 0 * * *", `minute "x" isn't a number`},
		{"0 9-5 * * *", `the hour range "9-5" goes backwards: write the smaller end first`},
		{"*/0 * * * *", `the step in minute "*/0" must be a whole number above 0`},
		{"5/15 * * * *", `a step needs a range or *: write 5-59/15 or */15, not "5/15"`},
		{"0 0 L * *", `day of month "L" isn't supported: cron here takes numbers, names, ranges, lists and steps, without L, W, # or ?`},
		{"0 0 15W * *", `day of month "15W" isn't supported: cron here takes numbers, names, ranges, lists and steps, without L, W, # or ?`},
		{"0 0 * * 1#2", `day of week "1#2" isn't supported: cron here takes numbers, names, ranges, lists and steps, without L, W, # or ?`},
		{"0 0 ? * 1", `day of month "?" isn't supported: cron here takes numbers, names, ranges, lists and steps, without L, W, # or ?`},
		{"0 0 1,,2 * *", "a day of month is missing in a range or list"},
		{"0 0 30 2 *", `the cron line "0 0 30 2 *" never fires: none of the months it names has the day of month it names`},
		{"0 0 31 apr,jun *", `the cron line "0 0 31 apr,jun *" never fires: none of the months it names has the day of month it names`},
	} {
		_, err := Parse(c.line)
		if err == nil {
			t.Errorf("%q parsed, want the error %s", c.line, c.want)
			continue
		}
		if err.Error() != c.want {
			t.Errorf("%q:\n got %s\nwant %s", c.line, err, c.want)
		}
	}
	// The 29th of February comes in leap years, and a weekday field picks
	// days of its own.
	for _, line := range []string{"0 0 29 2 *", "0 0 30 2 mon"} {
		if _, err := Parse(line); err != nil {
			t.Errorf("%q: %v", line, err)
		}
	}
}
