package interp

import (
	"strconv"
	"strings"
	"time"

	"eyelang/ir"
)

// timeMethod calls a method of a time value (SPECS.md §20):
//
//	at.date(in: zone)                 the date the clock in zone shows at that instant
//	at.time(in: zone)                 the time of day it shows
//	t.next(in: zone, after: instant)  the first instant after `after` when the clock shows t
//	day.at(t, in: zone)               the instant that day at time t
//	day.format("%A %-d %B")           the date written out
func (in *Interp) timeMethod(recv Value, name string, args []argVal, at ir.Pos) (Value, bool) {
	pos, named := split(args)
	zone := func() *time.Location {
		z, ok := named["in"].(string)
		if !ok {
			in.faultf(at, "%s takes a zone: %s(in: dublin)", name, name)
		}
		loc, err := time.LoadLocation(z)
		if err != nil {
			in.faultf(at, "unknown time zone %q", z)
		}
		return loc
	}
	switch x := recv.(type) {
	case time.Time:
		switch name {
		case "date":
			t := x.In(zone())
			return ir.Date{Year: t.Year(), Month: int(t.Month()), Day: t.Day()}, true
		case "time":
			t := x.In(zone())
			return ir.TimeOfDay{Hour: t.Hour(), Minute: t.Minute(), Second: t.Second()}, true
		}
	case ir.TimeOfDay:
		if name == "next" {
			after, ok := named["after"].(time.Time)
			if !ok {
				in.faultf(at, "next takes the instant to start from: next(in: zone, after: now)")
			}
			loc := zone()
			a := after.In(loc)
			t := time.Date(a.Year(), a.Month(), a.Day(), x.Hour, x.Minute, x.Second, 0, loc)
			if !t.After(after) {
				t = time.Date(a.Year(), a.Month(), a.Day()+1, x.Hour, x.Minute, x.Second, 0, loc)
			}
			return t, true
		}
	case ir.Date:
		switch name {
		case "at":
			t, ok := pos[0].(ir.TimeOfDay)
			if len(pos) != 1 || !ok {
				in.faultf(at, "at takes a time of day: day.at(07:40, in: zone)")
			}
			return time.Date(x.Year, time.Month(x.Month), x.Day, t.Hour, t.Minute, t.Second, 0, zone()), true
		case "format":
			layout, ok := pos[0].(string)
			if len(pos) != 1 || !ok {
				in.faultf(at, `format takes a pattern, such as "%%A %%-d %%B"`)
			}
			return formatDate(time.Date(x.Year, time.Month(x.Month), x.Day, 0, 0, 0, 0, time.UTC), layout), true
		}
	}
	return nil, false
}

// formatDate writes a date with strftime's directives: %A weekday, %B month,
// %d day, %-d day without padding, %m month number, %Y year, %% a percent sign.
func formatDate(t time.Time, layout string) string {
	var b strings.Builder
	for i := 0; i < len(layout); i++ {
		if layout[i] != '%' || i+1 == len(layout) {
			b.WriteByte(layout[i])
			continue
		}
		i++
		switch layout[i] {
		case 'A':
			b.WriteString(t.Weekday().String())
		case 'B':
			b.WriteString(t.Month().String())
		case 'd':
			b.WriteString(t.Format("02"))
		case 'm':
			b.WriteString(t.Format("01"))
		case 'Y':
			b.WriteString(strconv.Itoa(t.Year()))
		case '-':
			if i+1 < len(layout) && layout[i+1] == 'd' {
				i++
				b.WriteString(strconv.Itoa(t.Day()))
			}
		default:
			b.WriteByte(layout[i])
		}
	}
	return b.String()
}

// inTimeRange reports whether a range of times of day holds t. A range whose
// end comes before its start wraps midnight: 23:00..08:00 holds 23:30 and
// 07:00.
func inTimeRange(r *RangeVal, t ir.TimeOfDay) (bool, bool) {
	start, ok1 := r.Start.(ir.TimeOfDay)
	end, ok2 := r.End.(ir.TimeOfDay)
	if !ok1 || !ok2 {
		return false, false
	}
	secs := func(x ir.TimeOfDay) int { return x.Hour*3600 + x.Minute*60 + x.Second }
	s, e, v := secs(start), secs(end), secs(t)
	afterEnd := v > e || (v == e && !r.Inclusive)
	if s <= e {
		return v >= s && !afterEnd, true
	}
	return v >= s || !afterEnd, true
}
