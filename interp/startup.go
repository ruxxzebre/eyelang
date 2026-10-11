package interp

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"eyelang/ir"
)

// startLine is the line eye run logs when it starts a program that has
// jobs, so that a runner's log shows it is alive before any job runs: the
// program, where its state is kept, the memory runs may hold, and its jobs,
// each scheduled one with when it next fires.
//
//	eye run: started /program; store: postgres db:5432/app schema lab; memory: 512 MiB; jobs: digest.command, digest.morning (next 2026-10-12 07:40 Europe/Dublin), digest.write
func (rt *runtime) startLine(program string, store Store, memory string) string {
	var b strings.Builder
	if program == "" {
		program = "."
	}
	b.WriteString("eye run: started " + program)
	b.WriteString("; store: " + storeName(store))
	b.WriteString("; memory: " + memory)
	first := map[*ir.Job]*schedule{}
	for _, s := range rt.schedules() {
		if f := first[s.job]; f == nil || s.val.next(s.last).Before(f.val.next(f.last)) {
			first[s.job] = s
		}
	}
	var jobs []string
	for _, m := range rt.in.prog.Modules {
		for _, j := range m.Jobs {
			name := strings.TrimPrefix(jobName(j), ".")
			if s := first[j]; s != nil {
				name += " (next " + rt.firingTime(s) + ")"
			}
			jobs = append(jobs, name)
		}
	}
	sort.Strings(jobs)
	b.WriteString("; jobs: ")
	b.WriteString(strings.Join(jobs, ", "))
	return b.String()
}

// firingTime shows when a schedule fires next, on its own clock; a firing
// the program missed while it was stopped is due now.
func (rt *runtime) firingTime(s *schedule) string {
	at := s.val.next(s.last)
	if !at.After(rt.now) {
		return "now"
	}
	loc := s.val.location()
	t := at.In(loc)
	layout := "2006-01-02 15:04"
	if t.Second() != 0 {
		layout = "2006-01-02 15:04:05"
	}
	return t.Format(layout) + " " + loc.String()
}

// storeName says where a store keeps its state, for the log: a journal's
// path, or a database's host, name and schema, never its user or password.
func storeName(s Store) string {
	switch d := s.(type) {
	case nil:
		return "in memory only"
	case *Journal:
		return "journal " + d.path
	case *Postgres:
		return d.name
	}
	return "a store"
}

// memoryText shows a runner's memory: a round size as 512 MiB, any other
// as 15.2 GiB.
func memoryText(n int64) string {
	if n <= 0 {
		return "unknown, so runs aren't held back"
	}
	if s := showSize(ir.Size(n)); !strings.HasSuffix(s, " B") {
		return s
	}
	return bytesText(n)
}

// nothingToDo says why a runner stops as soon as it starts: main started no
// new run, since the runs its keys name are finished, and nothing waits or
// is scheduled. A journal can be started over with eye run --fresh.
func (rt *runtime) nothingToDo(w io.Writer, store Store) {
	if len(rt.queue) > 0 || rt.nextSleeper() != nil {
		return
	}
	if s, _ := rt.nextFiring(); s != nil {
		return
	}
	finished := 0
	for _, id := range rt.store.Runs() {
		data, _ := rt.store.Run(id)
		if st := decodeRun(data).Status; st == "done" || st == "failed" {
			finished++
		}
	}
	if finished == 0 {
		return
	}
	line := fmt.Sprintf("eye run: nothing to do: main started no new run, every run in the store has finished (%d), and no job is scheduled", finished)
	if _, ok := store.(*Journal); ok {
		line += "; eye run --fresh starts over with an empty journal"
	}
	fmt.Fprintln(w, line)
}
