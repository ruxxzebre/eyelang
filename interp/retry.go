package interp

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"eyelang/ir"
)

// Retrying a failed run (SPECS.md §15). `eye runs --retry ID` starts a run
// that failed again, once whatever made it fail has been fixed:
//
//   - The run replays the steps it recorded before the fault, as a run that
//     resumes after a restart does, and goes on from where it stopped. A
//     call that went through returns its recorded result and isn't made
//     again, so a charge isn't sent twice.
//   - It keeps its id, its arguments and its key, so a parent's join! and a
//     keyed start still find it. Its outcome event from before, run/<id>,
//     stays in the log; join waits only for events after it starts waiting.
//   - Its @deadline counts from when it runs again, as from a start, and it
//     waits for a place under @concurrency and @limit again.
//   - Only the faults a retry can cure are retried: Unavailable and
//     DeadlinePassed. A run that failed inside lock! or reserve!, which gave
//     the lock or the memory back as it ended, or that keeps only a summary,
//     can't be replayed.
//
// A store no runner holds is changed in place (Retry), and the next eye run
// carries the runs on; a store eye run holds gets the request through the
// runner's inbox (inbox.go), and the runner starts the runs at once.

// Retried is a run that a retry starts again.
type Retried struct {
	ID  int64  `json:"id"`
	Job string `json:"job"`
	// Steps is how many recorded steps it replays.
	Steps int `json:"steps"`
}

// retryableFaults are the faults a retry can cure, by FaultKind.
var retryableFaults = map[string]bool{"Unavailable": true, "DeadlinePassed": true}

// unretryable says why a run can't be started again, or "" if it can.
func (rt *runtime) unretryable(r *jobRun) string {
	name := fmt.Sprintf("run %d (%s)", r.ID, strings.TrimPrefix(r.Job, "."))
	if r.Status != "failed" {
		return fmt.Sprintf("%s hasn't failed: it is %s, and only a failed run is retried", name, r.Status)
	}
	outcome := "Failed(" + faultName(r) + ")"
	switch {
	case !retryableFaults[r.FaultKind]:
		why := map[string]string{
			"Killed":      "it was stopped on purpose, by firstDone! or by a person",
			"OverBudget":  "its @budget would stop it again",
			"OutOfMemory": "its @memory doesn't change with a retry",
			"LockExpired": "the lock it held ran out, and it would carry on without it",
		}[r.FaultKind]
		if why == "" {
			why = "a bug in the program or its configuration, such as unreachable(…) or a refused request, which a retry would meet again"
		}
		return fmt.Sprintf("%s ended as %s: %s. Only Unavailable and DeadlinePassed are retried", name, outcome, why)
	case rt.in.jobs[r.Job] == nil:
		return fmt.Sprintf("%s is a run of %s, which the program no longer has", name, strings.TrimPrefix(r.Job, "."))
	case r.Summarized:
		return fmt.Sprintf("%s ended more than %s ago, so only its summary is kept, not the steps a retry replays", name, showDuration(ir.Duration(keepFinished)))
	case len(r.Locks) > 0:
		return fmt.Sprintf("%s failed inside lock!(%q) and gave the lock back as it ended, so a retry would replay into the block without it", name, r.Locks[0].Name)
	case reservedAtEnd(r):
		return fmt.Sprintf("%s failed inside reserve!(…) and gave the memory back as it ended, so a retry would replay into the block without it", name)
	}
	return ""
}

// faultName is the name of the Fault variant a failed run ended with.
func faultName(r *jobRun) string {
	if r.FaultKind == "" {
		return "Unreachable"
	}
	return r.FaultKind
}

// reservedAtEnd reports whether a run ended inside reserve!: it took a
// reserve step that no release step followed. A checkpoint is taken only at
// the statements of a job's body, never inside reserve!'s function, so the
// steps since the checkpoint hold both.
func reservedAtEnd(r *jobRun) bool {
	n := 0
	for _, s := range r.Log {
		switch {
		case strings.HasPrefix(s.Call, "reserve "):
			n++
		case strings.HasPrefix(s.Call, "release "):
			n--
		}
	}
	return n > 0
}

// retryable picks the runs to retry: those with the ids given, each of
// which must be one a retry can cure, or with all set, every failed run
// that is.
func (rt *runtime) retryable(ids []int64, all bool) ([]*jobRun, error) {
	var out []*jobRun
	if all {
		rt.index.each(func(id int64, h *runHandle) {
			if h.status != failedRun {
				return
			}
			data, ok := rt.store.Run(id)
			if !ok {
				return
			}
			r := decodeRun(data)
			if rt.unretryable(r) == "" {
				out = append(out, r)
			}
		})
		return out, nil
	}
	for _, id := range ids {
		data, ok := rt.store.Run(id)
		if !ok {
			return nil, fmt.Errorf("there is no run %d", id)
		}
		r := decodeRun(data)
		if why := rt.unretryable(r); why != "" {
			return nil, fmt.Errorf("%s", why)
		}
		out = append(out, r)
	}
	return out, nil
}

// again makes failed runs ready to run again, in one commit, and queues
// them. Each keeps its id, arguments, key and recorded steps.
func (rt *runtime) again(runs []*jobRun) []Retried {
	c := &Commit{}
	var out []Retried
	for _, r := range runs {
		r.Status = "ready"
		r.Fault, r.FaultKind, r.FaultArg, r.Result = "", "", "", nil
		r.Ended, r.Begun, r.WakeAt, r.WaitEvent = 0, 0, 0, ""
		r.DeadlineAt = 0 // worked out again when it runs (runtime.step)
		r.Retries++
		c.Runs = append(c.Runs, RunWrite{ID: r.ID, Data: r.encode()})
		out = append(out, Retried{ID: r.ID, Job: strings.TrimPrefix(r.Job, "."), Steps: r.taken()})
	}
	if len(runs) == 0 {
		return nil
	}
	rt.store.Commit(c)
	for _, r := range runs {
		rt.queue = append(rt.queue, r.ID)
	}
	return out
}

// inboxRetry carries out a retry from the runner's inbox, in the file at
// path: it starts the runs again, removes the file once that is on disk, and
// runs them until each finishes or waits.
func (rt *runtime) inboxRetry(r InboxRetry, path string, report func(string)) {
	runs, err := rt.retryable(r.Retry, r.All)
	if err != nil {
		report(err.Error() + "; the retry in " + path + " is dropped")
	}
	done := rt.again(runs)
	rt.durable()
	_ = os.Remove(path)
	for _, d := range done {
		report(fmt.Sprintf("run %d (%s) starts again, replaying %s", d.ID, d.Job, stepsText(d.Steps)))
	}
	rt.drain()
}

// stepsText is a number of recorded steps in words.
func stepsText(n int) string {
	if n == 1 {
		return "its 1 recorded step"
	}
	return fmt.Sprintf("its %d recorded steps", n)
}

// retryRuntime opens a runtime on a store for Retry and RetryPlan, on the
// system clock. It indexes the store's runs and restores nothing else, since
// restoring runs the runs that are ready, and only eye run runs them.
func retryRuntime(prog *ir.Program, store Store) *runtime {
	in := New(prog, io.Discard, io.Discard)
	in.rt.real = true
	in.rt.now = time.Now()
	in.rt.useStore(store)
	return in.rt
}

// RetryPlan says which runs a retry would start again, without changing the
// store: the runs with the ids given, or with all set every failed run a
// retry can cure. An id it can't retry is an error that says why.
func RetryPlan(prog *ir.Program, store Store, ids []int64, all bool) ([]Retried, error) {
	rt := retryRuntime(prog, store)
	runs, err := rt.retryable(ids, all)
	if err != nil {
		return nil, err
	}
	var out []Retried
	for _, r := range runs {
		out = append(out, Retried{ID: r.ID, Job: strings.TrimPrefix(r.Job, "."), Steps: r.taken()})
	}
	return out, nil
}

// Retry starts failed runs again in a store that no runner holds, as eye
// runs --retry does: it makes them ready, and the next eye run carries them
// on. It picks the runs as RetryPlan does, and retries none if an id can't
// be retried.
func Retry(prog *ir.Program, store Store, ids []int64, all bool) ([]Retried, error) {
	rt := retryRuntime(prog, store)
	runs, err := rt.retryable(ids, all)
	if err != nil {
		return nil, err
	}
	var out []Retried
	if err := guard(func() {
		out = rt.again(runs)
		rt.durable()
	}); err != nil {
		return nil, err
	}
	return out, nil
}
