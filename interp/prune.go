package interp

import (
	"fmt"
	"strconv"
	"time"
)

// ---------------------------------------------------------------- event numbers

// seq is the number of events published so far, dropped ones included: the
// sequence number of the latest event.
func (rt *runtime) seq() int64 { return rt.eventBase + int64(len(rt.events)) }

// eventsAfter returns the events published after sequence number since.
// Events no run could still see may have been dropped.
func (rt *runtime) eventsAfter(since int64) []event {
	i := since - rt.eventBase
	if i < 0 {
		i = 0
	}
	if i > int64(len(rt.events)) {
		return nil
	}
	return rt.events[i:]
}

// ---------------------------------------------------------------- a commit that fails

// retryAfter is how long the runtime waits before trying again the runs whose
// commit failed.
var retryAfter = 10 * time.Second

// guard runs f, and returns the CommitError it panicked with, if any. Nothing
// the failed commit held took effect.
func guard(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			ce, ok := r.(*CommitError)
			if !ok {
				panic(r)
			}
			err = ce
		}
	}()
	f()
	return nil
}

// blocked notes a run whose step couldn't be saved. The store still holds
// the run as it was at its last checkpoint, and it is tried again later.
func (rt *runtime) blocked(id int64, err *CommitError) {
	rt.retry = append(rt.retry, id)
	if rt.commitErr == nil {
		rt.commitErr = err
	}
}

// retryBlocked queues again the runs whose commit failed.
func (rt *runtime) retryBlocked() {
	rt.queue = append(rt.queue, rt.retry...)
	rt.retry, rt.commitErr = nil, nil
	rt.drain()
}

// ---------------------------------------------------------------- pruning

// How long what the runtime keeps for others is kept.
var (
	// keepFinished is how long a finished run keeps its steps; after that
	// only a summary is kept: its job, arguments, outcome and times.
	keepFinished = 7 * 24 * time.Hour
	// keepSeen is how long an event's @key is remembered, so that the same
	// event delivered again starts nothing.
	keepSeen = 30 * 24 * time.Hour
)

// longRun is the number of steps past which a run is reported: a job that
// loops forever keeps its whole history while it lives.
var longRun = 1000

// prune drops what nothing can use any more, in one commit:
//   - events older than what every unfinished run can still wait for;
//   - event keys seen longer ago than keepSeen;
//   - the steps of runs that finished longer ago than keepFinished, which
//     keep a summary.
//
// Waiting runs and commands whose reply was lost are never pruned.
func (rt *runtime) prune() {
	c := &Commit{}
	oldest := rt.seq()
	for _, id := range rt.store.Runs() {
		data, _ := rt.store.Run(id)
		run := decodeRun(data)
		switch run.Status {
		case "ready", "sleeping":
			if s := run.sees(); s < oldest {
				oldest = s
			}
		case "done", "failed":
			if !run.Summarized && run.Ended != 0 && rt.now.Sub(time.Unix(0, run.Ended)) > keepFinished {
				run.summarize()
				c.Runs = append(c.Runs, RunWrite{ID: run.ID, Data: run.encode()})
			}
		}
	}
	if drop := oldest - rt.eventBase; drop > 0 {
		c.DropEvents = int(drop)
		c.Meta = append(c.Meta, MetaWrite{Key: "events/dropped", Data: []byte(strconv.FormatInt(oldest, 10))})
	}
	for _, k := range rt.store.MetaKeys("seen/") {
		data, _ := rt.store.Meta(k)
		if ns, err := strconv.ParseInt(string(data), 10, 64); err == nil && rt.now.Sub(time.Unix(0, ns)) > keepSeen {
			c.Meta = append(c.Meta, MetaWrite{Key: k})
		}
	}
	if len(c.Runs) == 0 && c.DropEvents == 0 && len(c.Meta) == 0 {
		return
	}
	rt.store.Commit(c)
	if c.DropEvents > 0 {
		rt.events = rt.events[c.DropEvents:]
		rt.eventBase = oldest
	}
	for _, m := range c.Meta {
		if m.Data == nil {
			delete(rt.seen, m.Key[len("seen/"):])
		}
	}
}

// sees returns the sequence number after which this run can still be waiting
// for events: its waiting step's, or its last step's.
func (r *jobRun) sees() int64 {
	since := r.Since
	for _, s := range r.Log {
		if s.Event != "" {
			return s.Since
		}
		since = s.Seq
	}
	return since
}

// summarize replaces a finished run's steps with their count.
func (r *jobRun) summarize() {
	r.Steps, r.Log, r.Summarized = len(r.Log), nil, true
}

// warnLong tells once about a run that has taken many steps.
func (rt *runtime) warnLong(run *jobRun) {
	if len(run.Log) < longRun || rt.warned[run.ID] {
		return
	}
	rt.warned[run.ID] = true
	fmt.Fprintf(rt.in.log, "warning: run #%d of %s has taken %d steps; a job that never ends keeps every step it took (SPECS.md, open question 9)\n",
		run.ID, run.Job, len(run.Log))
}
