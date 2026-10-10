package interp

import (
	"sort"
	"strconv"
	"time"

	"eyelang/ir"
)

// ScheduleVal is a Schedule: when a job with @on(…) starts.
//
//	daily(07:40, in: dublin)     every day at 07:40 on that zone's clock
//	weekdays(07:40, in: dublin)  the same, Monday to Friday
//	every(15 min)                at every multiple of 15 minutes since midnight UTC
//
// A time that a daylight-saving change skips fires as far after the change
// as it was meant to be after midnight (onGap: shift); a time that a change
// repeats fires the first time it comes (onOverlap: first).
type ScheduleVal struct {
	Kind  string // daily, weekdays, every
	At    ir.TimeOfDay
	Zone  string
	Every ir.Duration
	loc   *time.Location
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
	if s.Kind == "every" {
		return "every(" + showDuration(s.Every) + ")"
	}
	return s.Kind + "(" + show(s.At) + ", in: " + strconv.Quote(s.Zone) + ")"
}

// next returns the first firing after t.
func (s *ScheduleVal) next(t time.Time) time.Time {
	if s.Kind == "every" {
		d := time.Duration(s.Every)
		return t.Truncate(d).Add(d)
	}
	loc := s.location()
	day := t.In(loc)
	for i := 0; i < 8; i++ {
		y, m, d := day.AddDate(0, 0, i).Date()
		at := wallClock(y, m, d, s.At, loc)
		if s.Kind == "weekdays" {
			if wd := at.In(loc).Weekday(); wd == time.Saturday || wd == time.Sunday {
				continue
			}
		}
		if at.After(t) {
			return at
		}
	}
	return t.Add(24 * time.Hour) // unreachable: a week always has a weekday
}

// date is the day a firing belongs to, on the schedule's clock: what a
// scheduled job that takes a Date is given.
func (s *ScheduleVal) date(at time.Time) ir.Date {
	y, m, d := at.In(s.location()).Date()
	return ir.Date{Year: y, Month: int(m), Day: d}
}

// wallClock returns the instant the clock in loc shows t on that day. A time
// a daylight-saving change skips is shifted forward by the change; a time it
// repeats is the first of the two.
func wallClock(y int, m time.Month, d int, t ir.TimeOfDay, loc *time.Location) time.Time {
	wall := time.Date(y, m, d, t.Hour, t.Minute, t.Second, 0, time.UTC)
	var found []time.Time
	for _, probe := range []time.Duration{-12 * time.Hour, 12 * time.Hour} {
		_, offset := wall.Add(probe).In(loc).Zone()
		at := wall.Add(-time.Duration(offset) * time.Second)
		local := at.In(loc)
		if local.Hour() == t.Hour && local.Minute() == t.Minute && local.Day() == d {
			found = append(found, at)
		}
	}
	if len(found) == 0 {
		// Skipped: read the time with the offset from before the change.
		_, offset := wall.Add(-12 * time.Hour).In(loc).Zone()
		return wall.Add(-time.Duration(offset) * time.Second)
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Before(found[j]) })
	return found[0]
}

// scheduleBuiltins are daily, weekdays and every, which make Schedules.
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
			if _, err := loadLocation(zone); err != nil {
				in.faultf(at, "unknown time zone %q", zone)
			}
			return &ScheduleVal{Kind: kind, At: t, Zone: zone}
		})
	}
	clock("daily")
	clock("weekdays")
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
				v, _, _ := rt.in.callFunc(t.Schedule, nil, nil, nil, j.Pos)
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
	run := rt.newRun(s.job, args, nil, s.job.Pos)
	rt.store.Commit(&Commit{
		Runs: []RunWrite{{ID: run.ID, Data: run.encode()}},
		Meta: []MetaWrite{{Key: firedKey(s.job), Data: []byte(strconv.FormatInt(at.UnixNano(), 10))}},
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
