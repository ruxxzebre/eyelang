// Package cron reads the five fields of a crontab line, the dialect that
// Eyelang's cron(…) schedule takes (SPECS.md §20):
//
//	minute        0-59
//	hour          0-23
//	day of month  1-31
//	month         1-12, or jan-dec
//	day of week   0-7, where 0 and 7 are Sunday, or sun-sat
//
// Each field is *, a number or name, a range (mon-fri), a list (1,15) or a
// step (*/15, 8-18/2). When both day fields are restricted, a day matches
// if either does, as in every cron: "0 9 1 * mon" fires on the 1st and on
// every Monday. A field that starts with * counts as unrestricted.
//
// The checker reads a cron string to report its errors at compile time, and
// the interpreter to know when it fires. Daylight-saving changes are the
// interpreter's: this package only says which days and which times of day.
package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Spec is a parsed cron line.
type Spec struct {
	minutes, hours uint64 // bit n set: minute or hour n
	days           uint64 // day of month, bits 1-31
	months         uint64 // bits 1-12
	weekdays       uint64 // bits 0-6, Sunday is 0
	anyDay         bool   // the day-of-month field starts with *
	anyWeekday     bool   // the day-of-week field starts with *
}

// A Time is a time of day a cron line fires at.
type Time struct{ Hour, Minute int }

type field struct {
	name     string
	min, max int
	names    []string // names for min, min+1, …; nil when the field has none
}

var fields = []field{
	{name: "minute", min: 0, max: 59},
	{name: "hour", min: 0, max: 23},
	{name: "day of month", min: 1, max: 31},
	{name: "month", min: 1, max: 12, names: []string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"}},
	{name: "day of week", min: 0, max: 7, names: []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}},
}

// Parse reads a cron line. Its errors say what was expected and what was
// found, for the checker to show at the string.
func Parse(line string) (*Spec, error) {
	if strings.HasPrefix(strings.TrimSpace(line), "@") {
		return nil, fmt.Errorf("%q isn't a cron line: write daily(00:00, in: zone) or every(1 h) instead of a macro", strings.TrimSpace(line))
	}
	parts := strings.Fields(line)
	if len(parts) != len(fields) {
		return nil, fmt.Errorf("a cron line has five fields (minute, hour, day of month, month and day of week), and %q has %d", line, len(parts))
	}
	var sets [5]uint64
	for i, p := range parts {
		set, err := fields[i].parse(p)
		if err != nil {
			return nil, err
		}
		sets[i] = set
	}
	weekdays := sets[4]
	if weekdays&(1<<7) != 0 {
		weekdays = weekdays&^(1<<7) | 1 // 7 is Sunday too
	}
	s := &Spec{
		minutes: sets[0], hours: sets[1], days: sets[2], months: sets[3], weekdays: weekdays,
		anyDay: strings.HasPrefix(parts[2], "*"), anyWeekday: strings.HasPrefix(parts[4], "*"),
	}
	if !s.firesSomeDay() {
		return nil, fmt.Errorf("the cron line %q never fires: none of the months it names has the day of month it names", line)
	}
	return s, nil
}

// firesSomeDay reports whether some date matches: a day of month that a
// month it names has (29 February counts, since leap years have it).
func (s *Spec) firesSomeDay() bool {
	if s.anyDay || !s.anyWeekday {
		return true // the weekday field alone, or either field, picks days
	}
	long := []int{0, 31, 29, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	for m := 1; m <= 12; m++ {
		if s.months&(1<<m) == 0 {
			continue
		}
		for d := 1; d <= long[m]; d++ {
			if s.days&(1<<d) != 0 {
				return true
			}
		}
	}
	return false
}

// Day reports whether the line fires on that date.
func (s *Spec) Day(y int, m time.Month, d int) bool {
	if s.months&(1<<int(m)) == 0 {
		return false
	}
	day := s.days&(1<<d) != 0
	weekday := s.weekdays&(1<<int(time.Date(y, m, d, 12, 0, 0, 0, time.UTC).Weekday())) != 0
	if s.anyDay || s.anyWeekday {
		return day && weekday
	}
	return day || weekday
}

// Times returns the times of day the line fires at, earliest first.
func (s *Spec) Times() []Time {
	var ts []Time
	for h := 0; h < 24; h++ {
		if s.hours&(1<<h) == 0 {
			continue
		}
		for m := 0; m < 60; m++ {
			if s.minutes&(1<<m) != 0 {
				ts = append(ts, Time{h, m})
			}
		}
	}
	return ts
}

// parse reads one field: a comma-separated list of *, n, a-b, */s or a-b/s.
func (f field) parse(text string) (uint64, error) {
	var set uint64
	for _, item := range strings.Split(text, ",") {
		lo, hi, step := f.min, f.max, 1
		rng, stepText, hasStep := strings.Cut(item, "/")
		if hasStep {
			n, err := strconv.Atoi(stepText)
			if err != nil || n <= 0 {
				return 0, fmt.Errorf("the step in %s %q must be a whole number above 0", f.name, item)
			}
			step = n
		}
		switch {
		case rng == "*":
			if f.name == "day of week" {
				hi = 6 // * is each day once, not Sunday twice
			}
		case strings.Contains(rng, "-"):
			a, b, _ := strings.Cut(rng, "-")
			var err error
			if lo, err = f.value(a); err != nil {
				return 0, err
			}
			if hi, err = f.value(b); err != nil {
				return 0, err
			}
			if lo > hi {
				return 0, fmt.Errorf("the %s range %q goes backwards: write the smaller end first", f.name, rng)
			}
		default:
			n, err := f.value(rng)
			if err != nil {
				return 0, err
			}
			if hasStep {
				return 0, fmt.Errorf("a step needs a range or *: write %d-%d/%d or */%d, not %q", n, f.max, step, step, item)
			}
			lo, hi = n, n
		}
		for n := lo; n <= hi; n += step {
			set |= 1 << n
		}
	}
	return set, nil
}

// value reads a number or a name in the field.
func (f field) value(text string) (int, error) {
	for i, name := range f.names {
		if strings.EqualFold(text, name) {
			return f.min + i, nil
		}
	}
	n, err := strconv.Atoi(text)
	if err != nil {
		switch {
		case text == "":
			return 0, fmt.Errorf("a %s is missing in a range or list", f.name)
		case quartz(text):
			return 0, fmt.Errorf("%s %q isn't supported: cron here takes numbers, names, ranges, lists and steps, without L, W, # or ?", f.name, text)
		case f.names != nil:
			return 0, fmt.Errorf("%s %q isn't a number or a name such as %s", f.name, text, f.names[1])
		}
		return 0, fmt.Errorf("%s %q isn't a number", f.name, text)
	}
	if n < f.min || n > f.max {
		return 0, fmt.Errorf("%s %d is out of range: it goes from %d to %d", f.name, n, f.min, f.max)
	}
	return n, nil
}

// quartz reports whether text is one of the extensions some crons take and
// this one doesn't: ?, L, 15W, 5L or 1#2.
func quartz(text string) bool {
	if text == "?" || text == "L" || strings.Contains(text, "#") {
		return true
	}
	last := text[len(text)-1]
	return len(text) > 1 && (last == 'L' || last == 'W') && strings.Trim(text[:len(text)-1], "0123456789") == ""
}
