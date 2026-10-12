package interp

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"eyelang/internal/cron"
	"eyelang/ir"
)

// ScheduleVal is a Schedule: when a job with @on(…) starts.
//
//	daily(07:40, in: dublin)             every day at 07:40 on that zone's clock
//	weekdays(07:40, in: dublin)          the same, Monday to Friday
//	cron("*/20 7-9 * * *", in: dublin)   when the crontab line says, on that zone's clock
//	every(15 min)                        at every multiple of 15 minutes since midnight UTC
//
// The schedules on a zone's clock take two policies (SPECS.md §20). A time
// that a daylight-saving change skips fires as far after the change as it
// was meant to be (onGap: Shift, the default) or not at all (Skip). A time
// that a change repeats fires the first time it comes (onOverlap: First, the
// default), the second time (Second), or both times (Both).
type ScheduleVal struct {
	Kind    string // daily, weekdays, cron, every
	At      ir.TimeOfDay
	Cron    string
	Zone    string
	Every   ir.Duration
	Gap     string // Shift or Skip
	Overlap string // First, Second or Both
	spec    *cron.Spec
	times   []ir.TimeOfDay // the times of day it fires at, earliest first
	loc     *time.Location
}

// location is the schedule's zone; every runs on UTC.
func (s *ScheduleVal) location() *time.Location {
	if s.loc == nil {
		s.loc = time.UTC
		if loc, err := loadLocation(s.Zone); err == nil && s.Zone != "" {
			s.loc = loc
		}
	}
	return s.loc
}

func (s *ScheduleVal) String() string {
	var b strings.Builder
	switch s.Kind {
	case "every":
		return "every(" + showDuration(s.Every) + ")"
	case "cron":
		b.WriteString("cron(" + strconv.Quote(s.Cron))
	default:
		b.WriteString(s.Kind + "(" + show(s.At))
	}
	b.WriteString(", in: " + strconv.Quote(s.Zone))
	if s.Gap != "Shift" {
		b.WriteString(", onGap: " + s.Gap)
	}
	if s.Overlap != "First" {
		b.WriteString(", onOverlap: " + s.Overlap)
	}
	return b.String() + ")"
}

// onDay reports whether the schedule fires on a date of its zone's calendar.
func (s *ScheduleVal) onDay(y int, m time.Month, d int) bool {
	switch s.Kind {
	case "weekdays":
		wd := time.Date(y, m, d, 12, 0, 0, 0, time.UTC).Weekday()
		return wd != time.Saturday && wd != time.Sunday
	case "cron":
		return s.spec.Day(y, m, d)
	}
	return true
}

// maxShift is more than any daylight-saving change moves a clock. A time of
// day further than this before the time searched from can't fire after it.
const maxShift = 3 * time.Hour

// next returns the first firing after t.
func (s *ScheduleVal) next(t time.Time) time.Time {
	if s.Kind == "every" {
		d := time.Duration(s.Every)
		return t.Truncate(d).Add(d)
	}
	loc := s.location()
	from := wallOf(t, loc)
	// A cron line can name a day that comes once in eight years: 29
	// February, when the year 2100 isn't a leap year.
	days := 8
	if s.Kind == "cron" {
		days = 8*366 + 2
	}
	var best, bestWall time.Time
	bestDay := 0
	first := time.Date(from.Year(), from.Month(), from.Day()-1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < days; i++ {
		// A time shifted past midnight can come after the next day's
		// first, so the day after the first one that fires is looked at too.
		if !best.IsZero() && i > bestDay+1 {
			break
		}
		y, m, d := first.AddDate(0, 0, i).Date()
		if !s.onDay(y, m, d) {
			continue
		}
		for _, tod := range s.times {
			wall := time.Date(y, m, d, tod.Hour, tod.Minute, tod.Second, 0, time.UTC)
			if wall.Before(from.Add(-maxShift)) {
				continue
			}
			if !best.IsZero() && wall.After(bestWall.Add(maxShift)) {
				break
			}
			for _, at := range s.instants(wall, loc) {
				if at.After(t) && (best.IsZero() || at.Before(best)) {
					if best.IsZero() {
						bestDay = i
					}
					best, bestWall = at, wall
				}
			}
		}
	}
	if best.IsZero() {
		return t.Add(24 * time.Hour) // unreachable: cron.Parse refuses a line that never fires
	}
	return best
}

// date is the day a firing belongs to, on the schedule's clock: what a
// scheduled job that takes a Date is given.
func (s *ScheduleVal) date(at time.Time) ir.Date {
	y, m, d := at.In(s.location()).Date()
	return ir.Date{Year: y, Month: int(m), Day: d}
}

// wallOf is what the clock in loc shows at t, written as a UTC time so that
// wall times compare and add without a zone.
func wallOf(t time.Time, loc *time.Location) time.Time {
	l := t.In(loc)
	return time.Date(l.Year(), l.Month(), l.Day(), l.Hour(), l.Minute(), l.Second(), l.Nanosecond(), time.UTC)
}

// instants returns when the clock in loc shows wall, as the schedule's
// policies pick: usually one instant; none for a time a daylight-saving
// change skips, with onGap: Skip; two for a time a change repeats, with
// onOverlap: Both.
func (s *ScheduleVal) instants(wall time.Time, loc *time.Location) []time.Time {
	var found []time.Time
	for _, probe := range []time.Duration{-12 * time.Hour, 12 * time.Hour} {
		_, offset := wall.Add(probe).In(loc).Zone()
		at := wall.Add(-time.Duration(offset) * time.Second)
		if wallOf(at, loc).Equal(wall) && (len(found) == 0 || !found[0].Equal(at)) {
			found = append(found, at)
		}
	}
	switch len(found) {
	case 0:
		if s.Gap == "Skip" {
			return nil
		}
		// Shift: read the time with the offset from before the change.
		_, offset := wall.Add(-12 * time.Hour).In(loc).Zone()
		return []time.Time{wall.Add(-time.Duration(offset) * time.Second)}
	case 2:
		sort.Slice(found, func(i, j int) bool { return found[i].Before(found[j]) })
		switch s.Overlap {
		case "Second":
			return found[1:]
		case "Both":
			return found
		}
		return found[:1]
	}
	return found
}

// scheduleBuiltins are daily, weekdays, cron and every, which make
// Schedules.
func scheduleBuiltins(add func(string, builtinFunc)) {
	clock := func(kind string) {
		add(kind, func(in *Interp, args []Value, named map[string]Value, at ir.Pos) Value {
			t, ok1 := ir.TimeOfDay{}, false
			if len(args) == 1 {
				t, ok1 = args[0].(ir.TimeOfDay)
			}
			zone, ok2 := named["in"].(string)
			if !ok1 || !ok2 {
				in.faultf(at, "%s takes a time of day and a zone: %s(07:40, in: \"Europe/Dublin\")", kind, kind)
			}
			return zoned(in, &ScheduleVal{Kind: kind, At: t, Zone: zone, times: []ir.TimeOfDay{t}}, named, at)
		})
	}
	clock("daily")
	clock("weekdays")
	add("cron", func(in *Interp, args []Value, named map[string]Value, at ir.Pos) Value {
		line, ok1 := "", false
		if len(args) == 1 {
			line, ok1 = args[0].(string)
		}
		zone, ok2 := named["in"].(string)
		if !ok1 || !ok2 {
			in.faultf(at, "cron takes the five fields of a crontab line and a zone: cron(\"30 7 * * mon-fri\", in: \"Europe/Dublin\")")
		}
		spec, err := cron.Parse(line)
		if err != nil {
			in.faultf(at, "%v", err)
		}
		s := &ScheduleVal{Kind: "cron", Cron: line, Zone: zone, spec: spec}
		for _, t := range spec.Times() {
			s.times = append(s.times, ir.TimeOfDay{Hour: t.Hour, Minute: t.Minute})
		}
		return zoned(in, s, named, at)
	})
	add("every", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		d, ok := ir.Duration(0), false
		if len(args) == 1 {
			d, ok = args[0].(ir.Duration)
		}
		if !ok || d <= 0 {
			in.faultf(at, "every takes how often: every(15 min)")
		}
		return &ScheduleVal{Kind: "every", Every: d}
	})
}

// zoned checks a schedule's zone and sets its policies for daylight-saving
// changes: Shift and First, unless onGap: and onOverlap: say otherwise.
func zoned(in *Interp, s *ScheduleVal, named map[string]Value, at ir.Pos) *ScheduleVal {
	if _, err := loadLocation(s.Zone); err != nil {
		in.faultf(at, "unknown time zone %q", s.Zone)
	}
	s.Gap, s.Overlap = "Shift", "First"
	if v, ok := named["onGap"].(*Variant); ok {
		s.Gap = v.V.Name
	}
	if v, ok := named["onOverlap"].(*Variant); ok {
		s.Overlap = v.V.Name
	}
	return s
}

// schedule is a job started by a schedule, and when it last fired.
type schedule struct {
	job  *ir.Job
	val  *ScheduleVal
	last time.Time
}

func firedKey(j *ir.Job) string { return "fired/" + jobName(j) }

// schedules returns the program's schedules, computing them the first time.
// A schedule that has never fired counts from now, so starting a program
// doesn't fire what came before it.
func (rt *runtime) schedules() []*schedule {
	if rt.scheduled != nil {
		return rt.scheduled
	}
	rt.scheduled = []*schedule{}
	var fresh []MetaWrite
	for _, m := range rt.in.prog.Modules {
		if rt.module != nil && m != rt.module {
			continue
		}
		var jobs []*ir.Job
		for _, j := range m.Jobs {
			jobs = append(jobs, j)
		}
		sort.Slice(jobs, func(a, b int) bool { return jobs[a].Name < jobs[b].Name })
		for _, j := range jobs {
			for _, t := range j.Triggers {
				if t.Kind != "schedule" {
					continue
				}
				v, _ := rt.in.callFunc(t.Schedule, nil, nil, nil, j.Pos)
				val, ok := v.(*ScheduleVal)
				if !ok {
					rt.in.faultf(j.Pos, "@on takes a Schedule, such as daily(07:40, in: zone)")
				}
				s := &schedule{job: j, val: val, last: rt.now}
				if data, ok := rt.store.Meta(firedKey(j)); ok {
					if ns, err := strconv.ParseInt(string(data), 10, 64); err == nil {
						s.last = time.Unix(0, ns).In(rt.now.Location())
					}
				} else {
					fresh = append(fresh, MetaWrite{Key: firedKey(j), Data: []byte(strconv.FormatInt(rt.now.UnixNano(), 10))})
				}
				rt.scheduled = append(rt.scheduled, s)
			}
		}
	}
	if len(fresh) > 0 {
		rt.store.Commit(&Commit{Meta: fresh})
	}
	return rt.scheduled
}

// fire starts a scheduled job for its firing at at. The run and the record
// of the firing are saved together, so a restart neither repeats nor skips
// it.
func (rt *runtime) fire(s *schedule, at time.Time) {
	var args []Value
	if len(s.job.Func.Params) == 1 {
		args = []Value{s.val.date(at)}
	}
	fired := MetaWrite{Key: firedKey(s.job), Data: []byte(strconv.FormatInt(at.UnixNano(), 10))}
	run := rt.newRun(s.job, args, nil, s.job.Pos)
	if c := s.job.Concurrency; c != nil && rt.index.running(jobName(s.job), run.Group) >= c.N {
		// A firing that finds the limit reached is skipped, not queued:
		// the next one will do the work (SPECS.md §15).
		rt.nextRun--
		rt.store.Commit(&Commit{Meta: []MetaWrite{fired}})
		s.last = at
		return
	}
	rt.store.Commit(&Commit{
		Runs: []RunWrite{{ID: run.ID, Data: run.encode()}},
		Meta: []MetaWrite{fired},
	})
	s.last = at
	rt.queue = append(rt.queue, run.ID)
}

// catchUp fires each schedule that was due while the program wasn't running,
// once, for the latest firing it missed.
func (rt *runtime) catchUp() {
	for _, s := range rt.schedules() {
		due := s.val.next(s.last)
		if due.After(rt.now) {
			continue
		}
		for {
			n := s.val.next(due)
			if n.After(rt.now) {
				break
			}
			due = n
		}
		rt.fire(s, due)
	}
	rt.drain()
}

// nextFiring returns the schedule that fires first, and when.
func (rt *runtime) nextFiring() (*schedule, time.Time) {
	var first *schedule
	var when time.Time
	for _, s := range rt.schedules() {
		n := s.val.next(s.last)
		if first == nil || n.Before(when) {
			first, when = s, n
		}
	}
	return first, when
}
