package cgen

import (
	"fmt"

	"eyelang/lir"
)

// The methods of time values (SPECS.md §20), as the interpreter has them
// (interp/times.go), and the ends of a range. The runtime's side is in
// runtime/time.c.

// named gives a built-in's arguments by name: the receiver and the
// positional ones in order under the names given, then the named ones.
func named(in *lir.Builtin, positional ...string) map[string]*lir.Value {
	out := map[string]*lir.Value{}
	next := 0
	for i, a := range in.Args {
		name := ""
		if i < len(in.Names) {
			name = in.Names[i]
		}
		if name == "" {
			if next >= len(positional) {
				continue
			}
			name = positional[next]
			next++
		}
		out[name] = a
	}
	return out
}

// timeBuiltin writes a method of Instant, Date or Time, or contains on a
// range of times, and reports whether in is one.
func (e *fn) timeBuiltin(in *lir.Builtin) bool {
	if len(in.Args) == 0 || in.Dst == nil {
		return false
	}
	k0 := kind(in.Args[0])
	var a map[string]*lir.Value
	v := func(name string) string {
		x := a[name]
		if x == nil {
			e.errorf("%s takes %s", in.Name, name)
			return "0"
		}
		return e.v(x)
	}
	set := func(format string, args ...any) { e.assign(in.Dst, fmt.Sprintf(format, args...)) }
	switch {
	case k0 == "Instant" && in.Name == "unix":
		a = named(in, "at")
		set("eyb_instant_unix(%s)", v("at"))
	case k0 == "Instant" && in.Name == "date":
		a = named(in, "at")
		set("eyb_instant_date(%s, %s, %s)", e.at, v("at"), v("in"))
	case k0 == "Instant" && in.Name == "time":
		a = named(in, "at")
		set("eyb_instant_time(%s, %s, %s)", e.at, v("at"), v("in"))
	case k0 == "Time" && in.Name == "next":
		a = named(in, "t")
		set("eyb_time_next(%s, %s, %s, %s)", e.at, v("t"), v("in"), v("after"))
	case k0 == "Date" && in.Name == "at":
		a = named(in, "day", "time", "in")
		set("eyb_date_at(%s, %s, %s, %s)", e.at, v("day"), v("time"), v("in"))
	case (k0 == "Date" || k0 == "Time") && in.Name == "format":
		a = named(in, "t", "pattern")
		if k0 == "Date" {
			set("eyb_date_format(%s, %s)", v("t"), v("pattern"))
		} else {
			set("eyb_time_format(%s, %s)", v("t"), v("pattern"))
		}
	case k0 == "Range" && in.Name == "contains" && len(in.Args) == 2 && kind(in.Args[1]) == "Time":
		set("eyb_time_range_contains(%s, %s, %s)", e.at, e.v(in.Args[0]), e.v(in.Args[1]))
	default:
		return false
	}
	return true
}

// rangeEnd reads a range's start or end: borrowed, as a field is.
func (e *fn) rangeEnd(x *lir.Field) {
	e.assign(x.Dst, e.fromWord(x.Dst.Ty, fmt.Sprintf("EY_RANGEP(%s)->%s", e.v(x.Src), x.Name)))
	if x.Take {
		if st := e.g.dupStmt(x.Dst.Ty, e.v(x.Dst)); st != "" {
			e.printf("%s", st)
		}
	}
}
