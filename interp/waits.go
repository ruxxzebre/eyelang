package interp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"eyelang/ir"
)

// Joining several runs, and locks (SPECS.md §15).
//
//   - allDone!(runs) waits until every run in the list has finished and
//     records their outcomes as one step. firstDone!(runs) waits until one
//     has finished, records its outcome, and stops the others in the same
//     commit, so a restart neither loses the outcome nor leaves a loser
//     running.
//   - lock!(name, for: d, fn) takes the lock as a step, recorded in the
//     commit that saves who holds it and until when (meta lock/<name>), runs
//     fn, and gives the lock back as another step. The run keeps the locks
//     it holds (jobRun.Locks), so it wakes when one runs out and ends as
//     Failed(LockExpired(name)) if it is still inside fn.
//
// While a run waits for runs or for a lock it takes no step: it parks with
// the event it waits for, and once woken it replays to where it stopped and
// looks again, as a run waiting for memory does.

// heldLock is a lock a run holds: its name, and when it runs out in Unix
// nanoseconds.
type heldLock struct {
	Name  string
	Until int64
}

// lockRecord is what the store keeps for a lock someone holds.
type lockRecord struct {
	Owner int64 // the run
	Until int64 // Unix nanoseconds
}

func lockKey(name string) string { return "lock/" + name }

// lockEvent wakes the runs waiting for a lock, when it is given back.
func lockEvent(name string) string { return "lock/" + name }

// lockHolder returns who holds a lock, if anyone has taken it and not given
// it back. A holder whose time has run out may still be listed.
func (rt *runtime) lockHolder(name string) (lockRecord, bool) {
	data, ok := rt.store.Meta(lockKey(name))
	if !ok {
		return lockRecord{}, false
	}
	var l lockRecord
	if json.Unmarshal(data, &l) != nil {
		return lockRecord{}, false
	}
	return l, true
}

// lock runs f holding the named lock, waiting for it first if another run
// holds it.
func (in *Interp) lock(run *jobRun, args []argVal, at ir.Pos) Value {
	pos, named := split(args)
	name, ok := "", false
	var d ir.Duration
	var f Value
	switch len(pos) {
	case 2: // lock!("ci", for: 30 min, fn { … })
		name, ok = pos[0].(string)
		d, _ = named["for"].(ir.Duration)
		f = pos[1]
	case 3: // lock!("ci", 30 min, fn { … })
		name, ok = pos[0].(string)
		d, _ = pos[1].(ir.Duration)
		f = pos[2]
	}
	if !ok || d <= 0 || f == nil {
		in.faultf(at, "lock!(name, for: duration, fn { … }) takes the lock's name, how long to hold it, and a function")
	}
	rt := in.rt
	call := "lock " + strconv.Quote(name)
	if run.cursor < len(run.Log) {
		in.replay(run, call, at) // taken before: the run's Locks has it
	} else {
		for _, h := range run.Locks {
			if h.Name == name {
				in.faultf(at, "this run already holds the lock %q: locks don't nest", name)
			}
		}
		if l, held := rt.lockHolder(name); held && l.Owner != run.ID && l.Until > rt.now.UnixNano() {
			run.WaitEvent = lockEvent(name)
			run.WakeAt = l.Until // it is free then, if its holder hasn't given it back
			panic(parkSignal{})
		}
		run.WaitEvent = ""
		until := rt.now.Add(time.Duration(d)).UnixNano()
		run.Locks = append(run.Locks, heldLock{Name: name, Until: until})
		run.Log = append(run.Log, step{Call: call, Value: in.encode(ir.Unit{}, at), Seq: rt.seq(), WakeAt: until})
		run.cursor++
		data, _ := json.Marshal(lockRecord{Owner: run.ID, Until: until})
		rt.store.Commit(&Commit{
			Runs: []RunWrite{{ID: run.ID, Data: run.encode()}},
			Meta: []MetaWrite{{Key: lockKey(name), Data: data}},
		})
	}
	v := in.apply(f, nil, at)
	unlock := "unlock " + strconv.Quote(name)
	if run.cursor < len(run.Log) {
		in.replay(run, unlock, at) // given back before
		return v
	}
	for i, h := range run.Locks {
		if h.Name == name {
			run.Locks = append(run.Locks[:i:i], run.Locks[i+1:]...)
			break
		}
	}
	run.Log = append(run.Log, step{Call: unlock, Value: in.encode(ir.Unit{}, at), Seq: rt.seq()})
	run.cursor++
	c := &Commit{Runs: []RunWrite{{ID: run.ID, Data: run.encode()}}}
	freed := false
	if l, held := rt.lockHolder(name); held && l.Owner == run.ID {
		c.Meta = []MetaWrite{{Key: lockKey(name)}}
		freed = true
	}
	rt.store.Commit(c)
	if freed {
		rt.wake(lockEvent(name))
	}
	return v
}

// lockExpired ends a run that is still inside lock! when the lock has run
// out, as it wakes.
func (rt *runtime) lockExpired(run *jobRun, job *ir.Job) {
	for _, h := range run.Locks {
		if rt.now.UnixNano() >= h.Until {
			panic(&Fault{Pos: job.Pos, Kind: "LockExpired", Arg: h.Name,
				Msg: fmt.Sprintf("%s was still waiting inside lock!(%q) when the lock ran out", job.Name, h.Name)})
		}
	}
}

// dropLocks gives back, in c, the locks a run that ended still holds, and
// returns their names.
func (rt *runtime) dropLocks(r *jobRun, c *Commit) []string {
	var freed []string
	for _, h := range r.Locks {
		if l, ok := rt.lockHolder(h.Name); ok && l.Owner == r.ID {
			c.Meta = append(c.Meta, MetaWrite{Key: lockKey(h.Name)})
			freed = append(freed, h.Name)
		}
	}
	return freed
}

// runsArg reads the list of runs that allDone! and firstDone! take.
func (in *Interp) runsArg(args []argVal, name string, at ir.Pos) []*RunVal {
	pos, _ := split(args)
	var l *List
	if len(pos) == 1 {
		l, _ = pos[0].(*List)
	}
	if l == nil {
		in.faultf(at, "%s(runs) takes a list of runs, as %s([a, b])", name, name)
	}
	var runs []*RunVal
	for _, e := range l.Elems {
		r, ok := e.(*RunVal)
		if !ok {
			in.faultf(at, "%s(runs) takes a list of runs, and this holds %s", name, kindName(e))
		}
		runs = append(runs, r)
	}
	return runs
}

// runOf reads a run that allDone! or firstDone! waits for.
func (in *Interp) runOf(id int64, at ir.Pos) *jobRun {
	data, ok := in.rt.store.Run(id)
	if !ok {
		in.faultf(at, "run #%d isn't kept in the store", id)
	}
	return decodeRun(data)
}

func ended(r *jobRun) bool { return r.Status == "done" || r.Status == "failed" }

// waitForRuns parks the run until an event that a run ended wakes it: one
// run's (run/<id>), or any of several (runs/<id>,<id>).
func (in *Interp) waitForRuns(run *jobRun, event string) {
	run.WaitEvent = event
	run.WakeAt = in.rt.now.Add(time.Duration(forever)).UnixNano()
	panic(parkSignal{})
}

// waitsFor reports whether a run waiting for wait wakes to the event name.
func waitsFor(wait, name string) bool {
	if wait == name {
		return true
	}
	ids, ok := strings.CutPrefix(wait, "runs/")
	id, isRun := strings.CutPrefix(name, "run/")
	if !ok || !isRun {
		return false
	}
	for _, s := range strings.Split(ids, ",") {
		if s == id {
			return true
		}
	}
	return false
}

// allDone waits until every run in the list has finished, and returns their
// outcomes in the list's order.
func (in *Interp) allDone(run *jobRun, args []argVal, at ir.Pos) Value {
	runs := in.runsArg(args, "allDone!", at)
	if v, ok := in.replay(run, "allDone", at); ok {
		return v
	}
	out := &List{}
	for _, target := range runs {
		r := in.runOf(target.ID, at)
		if !ended(r) {
			in.waitForRuns(run, runEvent(target.ID))
		}
		out.Elems = append(out.Elems, in.outcome(r, at))
	}
	run.WaitEvent = ""
	in.call = "allDone"
	return in.record(run, out, at)
}

// firstDone waits until one of the runs has finished, returns its outcome,
// and stops the others.
func (in *Interp) firstDone(run *jobRun, args []argVal, at ir.Pos) Value {
	runs := in.runsArg(args, "firstDone!", at)
	if len(runs) == 0 {
		in.faultf(at, "firstDone! needs at least one run to wait for")
	}
	if v, ok := in.replay(run, "firstDone", at); ok {
		return v
	}
	rt := in.rt
	var winner *jobRun
	var ids []string
	for _, target := range runs {
		r := in.runOf(target.ID, at)
		if ended(r) && (winner == nil || r.Ended < winner.Ended) {
			winner = r
		}
		ids = append(ids, strconv.FormatInt(target.ID, 10))
	}
	if winner == nil {
		in.waitForRuns(run, "runs/"+strings.Join(ids, ","))
	}
	run.WaitEvent = ""
	v := in.outcome(winner, at)
	run.Log = append(run.Log, step{Call: "firstDone", Value: in.encode(v, at), Seq: rt.seq()})
	run.cursor++
	// The outcome and the stopped runs are saved together.
	c := &Commit{Runs: []RunWrite{{ID: run.ID, Data: run.encode()}}}
	var after []func()
	stopped := map[int64]bool{}
	for _, target := range runs {
		if stopped[target.ID] {
			continue
		}
		if r := in.runOf(target.ID, at); !ended(r) {
			stopped[target.ID] = true
			why := fmt.Sprintf("firstDone! in run #%d stopped it, since run #%d finished first", run.ID, winner.ID)
			after = append(after, rt.stop(r, why, c))
		}
	}
	rt.store.Commit(c)
	for _, f := range after {
		f()
	}
	return v
}

// stop ends a run that hasn't finished as Failed(Killed), in c. It returns
// what to do once c is committed.
func (rt *runtime) stop(r *jobRun, why string, c *Commit) func() {
	r.Status, r.FaultKind, r.Fault, r.Ended = "failed", "Killed", why, rt.now.UnixNano()
	r.WaitEvent = ""
	c.Runs = append(c.Runs, RunWrite{ID: r.ID, Data: r.encode()})
	return rt.ending(r, c)
}

// ending adds to c what a run that finished leaves: its outcome, as the
// event run/<id> that wakes the runs joining it, and the memory and locks it
// still held. It returns what to do once c is committed: wake those runs,
// and pass on its place, its memory and its locks.
func (rt *runtime) ending(r *jobRun, c *Commit) func() {
	job := rt.in.jobs[r.Job]
	var at ir.Pos
	if job != nil {
		at = job.Pos
	}
	name, outcome := runEvent(r.ID), rt.in.encode(rt.in.outcome(r, at), at)
	c.Events = append(c.Events, EventRecord{Name: name, Data: outcome})
	freed := rt.dropReserved(r, c) // a run that ended inside reserve!
	locks := rt.dropLocks(r, c)    // or inside lock!
	return func() {
		rt.events = append(rt.events, event{name: name, data: outcome})
		rt.wake(name)
		if job != nil {
			rt.release(job, r)
		}
		if freed {
			delete(rt.reserved, r.ID)
			rt.wakeMemory()
		}
		for _, l := range locks {
			rt.wake(lockEvent(l))
		}
	}
}
