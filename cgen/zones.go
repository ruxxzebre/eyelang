package cgen

import (
	"fmt"
	"sort"
	"strings"
	"time"
	_ "time/tzdata" // every zone, whatever the machine that builds has installed
)

// zoneTables writes, for each zone the program names in a string, its
// offsets from UTC and when each starts, from 1900 to 2100, as Go's zone
// database has them. The runtime looks a zone up there by name
// (ey_zone_find); UTC needs no table.
func (g *gen) zoneTables() string {
	var names []string
	var locs []*time.Location
	for n := range g.zones {
		if n == "" || n == "UTC" || n == "Local" || len(n) > 64 || strings.ContainsAny(n, " \n\t{}") {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)
	keep := names[:0]
	for _, n := range names {
		if loc, err := time.LoadLocation(n); err == nil {
			keep = append(keep, n)
			locs = append(locs, loc)
		}
	}
	names = keep
	g.nzones = len(names)
	var b strings.Builder
	var refs []string
	for i, name := range names {
		loc := locs[i]
		first, at, offs := transitions(loc)
		var ats, os []string
		for j := range at {
			ats = append(ats, fmt.Sprintf("INT64_C(%d)", at[j]))
			os = append(os, fmt.Sprint(offs[j]))
		}
		if len(ats) == 0 {
			ats, os = []string{"0"}, []string{fmt.Sprint(first)}
		}
		fmt.Fprintf(&b, "static const int64_t ey_z%d_at[] = {%s};\n", i, strings.Join(ats, ", "))
		fmt.Fprintf(&b, "static const int32_t ey_z%d_off[] = {%s};\n", i, strings.Join(os, ", "))
		fmt.Fprintf(&b, "static const ey_zone ey_z%d = {%s, %d, ey_z%d_at, ey_z%d_off, %d};\n", i, cstring(name), len(ats), i, i, first)
		refs = append(refs, fmt.Sprintf("&ey_z%d", i))
	}
	if len(refs) == 0 {
		refs = []string{"NULL"}
	}
	fmt.Fprintf(&b, "static const ey_zone *const ey_zone_list[] = {%s};\n", strings.Join(refs, ", "))
	return b.String()
}

// transitions finds when loc's offset changes, a day at a time and then to
// the second.
func transitions(loc *time.Location) (first int, at []int64, offs []int) {
	offset := func(unix int64) int {
		_, off := time.Unix(unix, 0).In(loc).Zone()
		return off
	}
	start := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	end := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	first = offset(start)
	cur := first
	const step = 6 * 3600
	for t := start; t < end; t += step {
		next := offset(t + step)
		if next == cur {
			continue
		}
		lo, hi := t, t+step // offset(lo) == cur, offset(hi) == next
		for hi-lo > 1 {
			mid := lo + (hi-lo)/2
			if offset(mid) == cur {
				lo = mid
			} else {
				hi = mid
			}
		}
		at = append(at, hi)
		offs = append(offs, next)
		cur = next
	}
	return first, at, offs
}
