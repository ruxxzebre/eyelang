package interp

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"eyelang/ir"
)

// Memory (SPECS.md §15). A runner has a memory limit: --memory-limit for
// eye run, the host's memory without one, and 1 GiB in tests and the REPL so
// that they don't depend on the host. What is held against it is what
// reserve! has granted: a run holds its @memory only while it executes, and
// the interpreter executes one run at a time, so a waiting run holds nothing.
//
//   - A run with @memory executes only when its @memory fits beside what is
//     reserved, and when no run of a higher @priority waits for memory;
//     otherwise it waits at the checkpoint it reached until memory is
//     released. So under pressure lower priorities pause, and none overtakes
//     a higher one.
//   - reserve!(size, fn) waits the same way until size fits, holds it while
//     fn runs, even across fn's own waits, and gives it back when fn returns.
//     The grant and the release are steps of the run, and what is reserved is
//     kept in the store (meta reserved/<run>), so a restart neither forgets
//     a reservation nor grants one twice.
//   - What the runtime keeps for a run, its saved state with every step's
//     result, may not grow past its @memory: such a run ends as
//     Failed(OutOfMemory).

// memoryEvent wakes the runs that wait for memory.
const memoryEvent = "memory"

// testMemory is the limit tests and the REPL run with.
const testMemory = 1 << 30

// forever is how long a run waits for something that has no deadline.
const forever = ir.Duration(100 * 365 * 24 * time.Hour)

// held is the memory reserve! has granted, to every run.
func (rt *runtime) held() int64 {
	var n int64
	for _, s := range rt.reserved {
		n += s
	}
	return n
}

// outranked reports whether a run of a higher priority than p, other than
// self, waits for memory.
func (rt *runtime) outranked(p ir.Priority, self int64) bool {
	for id := range rt.index.byEvent[memoryEvent] {
		if id == self {
			continue
		}
		if h := rt.index.handle(id); h != nil {
			if j := rt.in.jobs[rt.index.jobOf(h)]; j != nil && j.Priority > p {
				return true
			}
		}
	}
	return false
}

// wakeMemory wakes the runs waiting for memory, highest priority first and
// then in the order they started, so that the first to ask again is the one
// that should get it.
func (rt *runtime) wakeMemory() {
	ids := rt.index.waitingFor(memoryEvent)
	prio := func(id int64) ir.Priority {
		if h := rt.index.handle(id); h != nil {
			if j := rt.in.jobs[rt.index.jobOf(h)]; j != nil {
				return j.Priority
			}
		}
		return ir.Normal
	}
	sort.Slice(ids, func(i, j int) bool {
		if pi, pj := prio(ids[i]), prio(ids[j]); pi != pj {
			return pi > pj
		}
		return ids[i] < ids[j]
	})
	for _, id := range ids {
		data, _ := rt.store.Run(id)
		run := decodeRun(data)
		if run.Status == "sleeping" && run.WaitEvent == memoryEvent {
			run.Status = "ready"
			rt.save(run)
			rt.queue = append(rt.queue, run.ID)
		}
	}
}

// admitMemory lets a run with @memory execute, or makes it wait for memory.
// A run whose @memory is more than the runner has ends as OutOfMemory.
func (in *Interp) admitMemory(run *jobRun, job *ir.Job) {
	rt := in.rt
	waited := run.WaitEvent == memoryEvent
	if job.Memory == 0 || rt.memLimit == 0 {
		return
	}
	need := int64(job.Memory)
	if need > rt.memLimit {
		runFault(job.Pos, "OutOfMemory", "%s needs @memory(%s), and this runner has %s", job.Name, showSize(ir.Size(need)), showSize(ir.Size(rt.memLimit)))
	}
	if need+rt.held() > rt.memLimit || rt.outranked(job.Priority, run.ID) {
		in.waitForMemory(run)
	}
	if waited {
		run.WaitEvent = ""
		rt.wakeMemory() // a lower priority may have waited behind this one
	}
}

// waitForMemory parks the run until memory is released. It takes no step:
// once woken, the run replays to where it stopped and asks again.
func (in *Interp) waitForMemory(run *jobRun) {
	run.WaitEvent = memoryEvent
	run.WakeAt = in.rt.now.Add(time.Duration(forever)).UnixNano()
	panic(parkSignal{})
}

// reserve runs fn with size bytes of memory granted to the run, waiting
// until they can be.
func (in *Interp) reserve(run *jobRun, args []argVal, at ir.Pos) Value {
	pos, _ := split(args)
	var size ir.Size
	ok := len(pos) == 2
	if ok {
		size, ok = pos[0].(ir.Size)
	}
	if !ok {
		in.faultf(at, "reserve!(size, fn { … }) takes a Size, such as 256 MiB, and a function")
	}
	rt := in.rt
	job := in.jobs[run.Job]
	call := "reserve " + showSize(size)
	if run.cursor < len(run.Log) {
		run.cursor++ // granted before: what is reserved is in rt.reserved
	} else {
		if rt.memLimit > 0 {
			if int64(size) > rt.memLimit {
				runFault(at, "OutOfMemory", "reserve!(%s) asks for more than this runner's %s", showSize(size), showSize(ir.Size(rt.memLimit)))
			}
			if int64(size)+rt.held() > rt.memLimit || rt.outranked(job.Priority, run.ID) {
				in.waitForMemory(run)
			}
		}
		in.changeReserved(run, int64(size), call, at)
	}
	v := in.apply(pos[1], nil, at)
	if run.cursor < len(run.Log) {
		run.cursor++ // released before
		return v
	}
	in.changeReserved(run, -int64(size), "release "+showSize(size), at)
	rt.wakeMemory()
	return v
}

// changeReserved changes what the run has reserved, as a step of the run
// saved in the same commit as the reservation.
func (in *Interp) changeReserved(run *jobRun, by int64, call string, at ir.Pos) {
	rt := in.rt
	total := rt.reserved[run.ID] + by
	run.Log = append(run.Log, step{Call: call, Value: in.encode(ir.Unit{}, at), Seq: rt.seq()})
	run.cursor++
	c := &Commit{Runs: []RunWrite{{ID: run.ID, Data: run.encode()}}}
	key := reservedKey(run.ID)
	if total > 0 {
		c.Meta = []MetaWrite{{Key: key, Data: []byte(strconv.FormatInt(total, 10))}}
	} else {
		c.Meta = []MetaWrite{{Key: key}}
	}
	rt.store.Commit(c)
	if total > 0 {
		rt.reserved[run.ID] = total
	} else {
		delete(rt.reserved, run.ID)
	}
}

func reservedKey(id int64) string { return "reserved/" + strconv.FormatInt(id, 10) }

// dropReserved gives back what a run that ended still had reserved, in c,
// the commit that ends it.
func (rt *runtime) dropReserved(run *jobRun, c *Commit) bool {
	if rt.reserved[run.ID] == 0 {
		return false
	}
	c.Meta = append(c.Meta, MetaWrite{Key: reservedKey(run.ID)})
	return true
}

// restoreReserved reads what runs had reserved when the program stopped.
func (rt *runtime) restoreReserved() {
	for _, k := range rt.store.MetaKeys("reserved/") {
		id, err := strconv.ParseInt(strings.TrimPrefix(k, "reserved/"), 10, 64)
		data, _ := rt.store.Meta(k)
		n, err2 := strconv.ParseInt(string(data), 10, 64)
		if err == nil && err2 == nil && n > 0 {
			rt.reserved[id] = n
		}
	}
}

// checkHeld ends a run as OutOfMemory if what the runtime keeps for it has
// grown past its @memory. size is the run's saved state in bytes.
func (in *Interp) checkHeld(run *jobRun, size int, at ir.Pos) {
	job := in.jobs[run.Job]
	if job == nil || job.Memory == 0 || int64(size) <= int64(job.Memory) {
		return
	}
	runFault(at, "OutOfMemory", "%s holds %s of saved state, more than its @memory(%s)", job.Name, showSize(ir.Size(size)), showSize(job.Memory))
}

// Reservations lists what each run has reserved, for eye runs.
func reservations(store Store) map[int64]int64 {
	out := map[int64]int64{}
	keys := store.MetaKeys("reserved/")
	sort.Strings(keys)
	for _, k := range keys {
		id, err := strconv.ParseInt(strings.TrimPrefix(k, "reserved/"), 10, 64)
		data, _ := store.Meta(k)
		n, err2 := strconv.ParseInt(string(data), 10, 64)
		if err == nil && err2 == nil {
			out[id] = n
		}
	}
	return out
}
