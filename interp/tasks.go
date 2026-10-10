package interp

import (
	"context"
	"sync"
	"time"

	"eyelang/ir"
)

// Structured concurrency (SPECS.md §13): parallelMap, both, race and
// timeout start tasks that all finish, or are cancelled, before the call
// returns. Tasks share only immutable values; the checker keeps vars and mut
// parameters out of closures.
//
// On the system clock (eye run), each task is a goroutine with its own copy
// of the interpreter's per-call state. The interpreter's shared state (the
// store, globals, output) is guarded by one lock, which a task gives up
// while it waits for the network or a timer, so tasks that wait overlap. A
// task that loses a race, or outlives a timeout, is cancelled at its next
// wait.
//
// On a test's clock, tasks take turns: one runs at a time until it waits,
// and the one that wakes first goes next, the clock moving to its wake time.
// Waits still overlap in the clock's time, and every run of a test is the
// same.

// taskCtx is what a running task is: cancelled through ctx on the system
// clock, and run by a scheduler on a test's.
type taskCtx struct {
	ctx context.Context
	v   *vtask
}

// vtask is a task on a test's clock. Tasks are goroutines, but only one runs
// at a time: a task that waits hands control back to its scheduler, which
// resumes the task that wakes first (the one started first, on a tie) and
// moves the clock to its wake time, so every run of a test is the same.
type vtask struct {
	index    int
	sched    chan *vtask // where it hands control back: it waits or finished
	resume   chan bool   // the scheduler's go-ahead; true cancels the task
	wake     time.Time
	started  bool
	sleeping bool
	done     bool
	result   taskResult
}

// cancelSignal unwinds a task that was cancelled.
type cancelSignal struct{}

// gil guards the interpreter's shared state while tasks run on goroutines.
// Whoever holds it is the only goroutine running Eyelang code; current is
// that goroutine's task.
type gil struct {
	mu      sync.Mutex
	current *taskCtx
}

// task returns the task the running code belongs to, if any.
func (in *Interp) currentTask() *taskCtx {
	if in.task != nil {
		return in.task
	}
	if in.g != nil {
		return in.g.current
	}
	return nil
}

// blocking runs f, which waits for something outside the interpreter, with
// the task's context; other tasks run meanwhile. A task cancelled while it
// waited stops here.
func (in *Interp) blocking(f func(ctx context.Context)) {
	t := in.currentTask()
	ctx := context.Background()
	if t != nil && t.ctx != nil {
		ctx = t.ctx
	}
	if in.g != nil {
		in.g.mu.Unlock()
		f(ctx)
		in.g.mu.Lock()
		in.g.current = t
	} else {
		f(ctx)
	}
	if ctx.Err() != nil {
		panic(cancelSignal{})
	}
}

// pause waits d: on the system clock for real, and on a test's clock by
// moving it, unless the task's deadline comes first, which cancels it.
func (in *Interp) pause(d time.Duration) {
	if in.rt.real {
		in.blocking(func(ctx context.Context) {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-ctx.Done():
			}
		})
		return
	}
	if t := in.currentTask(); t != nil && t.v != nil {
		v := t.v
		v.wake, v.sleeping = in.rt.now.Add(d), true
		v.sched <- v
		if <-v.resume {
			panic(cancelSignal{})
		}
		return
	}
	in.rt.advance(ir.Duration(d))
}

// taskResult is what one task ended with.
type taskResult struct {
	value     Value
	fault     any // a *Fault or other panic, passed on to the caller
	cancelled bool
}

// runTasks runs fns as tasks, at most limit at once, and returns what each
// ended with, in order, and which finished first. With first, the first task
// to finish cancels the rest; with within, tasks still running then are
// cancelled. In a job, a task can only compute: a durable step in it couldn't
// be recorded, so it fails (see durable).
func (in *Interp) runTasks(fns []func(*Interp) Value, limit int, first bool, within time.Duration, at ir.Pos) ([]taskResult, int) {
	if limit <= 0 || limit > len(fns) {
		limit = len(fns)
	}
	if len(fns) == 0 {
		return nil, -1
	}
	if in.rt.real {
		return in.runParallel(fns, limit, first, within, at)
	}
	return in.runVirtual(fns, limit, first, within)
}

// child is the interpreter as a task sees it: the same program and shared
// state, with its own per-call state.
func (in *Interp) child(t *taskCtx) *Interp {
	c := *in
	c.task, c.run, c.want, c.call = t, nil, nil, ""
	return &c
}

// runTask runs one task's function, catching how it ends.
func (in *Interp) runTask(fn func(*Interp) Value) (r taskResult) {
	defer func() {
		if p := recover(); p != nil {
			if _, ok := p.(cancelSignal); ok {
				r.cancelled = true
				return
			}
			r.fault = p
		}
	}()
	r.value = fn(in)
	return r
}

// runVirtual runs tasks on a test's clock: one at a time, each until it
// waits or finishes, always resuming the one that wakes first. The caller
// waits through its own pause, so tasks inside tasks share one timeline.
func (in *Interp) runVirtual(fns []func(*Interp) Value, limit int, first bool, within time.Duration) ([]taskResult, int) {
	sched := make(chan *vtask)
	tasks := make([]*vtask, len(fns))
	for i := range fns {
		tasks[i] = &vtask{index: i, sched: sched, resume: make(chan bool)}
	}
	var deadline time.Time
	if within > 0 {
		deadline = in.rt.now.Add(within)
	}
	winner, running, next := -1, 0, 0
	stopping := false
	var fault any
	// settle takes control back from the task that ran: it waits or finished.
	settle := func() {
		v := <-sched
		if !v.done {
			return
		}
		running--
		switch r := v.result; {
		case r.fault != nil:
			if fault == nil {
				fault = r.fault
			}
			stopping = true
		case !r.cancelled && winner < 0:
			winner = v.index
			stopping = stopping || first
		}
	}
	for {
		for !stopping && next < len(fns) && running < limit {
			v, fn := tasks[next], fns[next]
			child := in.child(&taskCtx{v: v})
			v.started = true
			running++
			next++
			go func() {
				v.result = child.runTask(fn)
				v.done, v.sleeping = true, false
				sched <- v
			}()
			settle()
		}
		if stopping {
			// Cancel the rest: each wakes at its wait and unwinds.
			for _, v := range tasks {
				if v.started && !v.done {
					v.sleeping = false
					v.resume <- true
					settle()
				}
			}
			for _, v := range tasks[next:] {
				v.result.cancelled = true
			}
			break
		}
		var soonest *vtask
		for _, v := range tasks {
			if v.started && !v.done && v.sleeping && (soonest == nil || v.wake.Before(soonest.wake)) {
				soonest = v
			}
		}
		if soonest == nil {
			break
		}
		if !deadline.IsZero() && soonest.wake.After(deadline) {
			if d := deadline.Sub(in.rt.now); d > 0 {
				in.pause(d)
			}
			stopping = true
			continue
		}
		if d := soonest.wake.Sub(in.rt.now); d > 0 {
			in.pause(d)
		}
		soonest.sleeping = false
		soonest.resume <- false
		settle()
	}
	if fault != nil {
		panic(fault)
	}
	results := make([]taskResult, len(tasks))
	for i, v := range tasks {
		results[i] = v.result
	}
	return results, winner
}

// runParallel runs tasks on goroutines on the system clock.
func (in *Interp) runParallel(fns []func(*Interp) Value, limit int, first bool, within time.Duration, at ir.Pos) ([]taskResult, int) {
	root := in
	if in.g == nil {
		in.g = &gil{}
		in.g.mu.Lock() // the code running now holds it from here on
	}
	ctx, cancel := context.WithCancel(context.Background())
	if t := in.currentTask(); t != nil && t.ctx != nil {
		ctx, cancel = context.WithCancel(t.ctx)
	}
	if within > 0 {
		var c2 context.CancelFunc
		ctx, c2 = context.WithTimeout(ctx, within)
		defer c2()
	}
	defer cancel()
	results := make([]taskResult, len(fns))
	done := make(chan int, len(fns))
	slots := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i, fn := range fns {
		wg.Add(1)
		go func(i int, fn func(*Interp) Value) {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			t := &taskCtx{ctx: ctx}
			root.g.mu.Lock()
			root.g.current = t
			results[i] = root.child(t).runTask(fn)
			root.g.mu.Unlock()
			done <- i
		}(i, fn)
	}
	// Wait without holding the lock, so the tasks can run; the first to
	// finish cancels the rest when first is set, and a fault cancels all.
	parent := in.currentTask()
	in.g.mu.Unlock()
	winner := -1
	for range fns {
		i := <-done
		r := results[i]
		if winner < 0 && r.fault == nil && !r.cancelled {
			winner = i
		}
		if r.fault != nil || (first && !r.cancelled) {
			cancel()
		}
	}
	wg.Wait()
	in.g.mu.Lock()
	in.g.current = parent
	for _, r := range results {
		if r.fault != nil {
			panic(r.fault)
		}
	}
	return results, winner
}

// taskBuiltins adds parallelMap, both, race and timeout.
func taskBuiltins(add func(string, builtinFunc)) {
	call := func(f Value, at ir.Pos, args ...Value) func(*Interp) Value {
		return func(in *Interp) Value { return in.callValue(f, at, args...) }
	}
	add("parallelMap", func(in *Interp, args []Value, named map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "parallelMap", at)
		limit := 0
		if l, ok := named["limit"]; ok {
			n, ok := l.(int64)
			if !ok || n < 1 {
				in.faultf(at, "parallelMap's limit: is how many tasks run at once, at least 1")
			}
			limit = int(n)
		}
		elems := in.elems(args[0], at)
		fns := make([]func(*Interp) Value, len(elems))
		for i, e := range elems {
			fns[i] = call(args[1], at, e)
		}
		results, _ := in.runTasks(fns, limit, false, 0, at)
		out := make([]Value, len(results))
		for i, r := range results {
			out[i] = r.value
		}
		return &List{Elems: out}
	})
	add("both", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "both", at)
		results, _ := in.runTasks([]func(*Interp) Value{call(args[0], at), call(args[1], at)}, 0, false, 0, at)
		return &Tuple{Elems: []Value{results[0].value, results[1].value}}
	})
	add("race", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "race", at)
		var fns []func(*Interp) Value
		for _, f := range in.elems(args[0], at) {
			fns = append(fns, call(f, at))
		}
		if len(fns) == 0 {
			in.faultf(at, "race needs at least one function to run")
		}
		results, winner := in.runTasks(fns, 0, true, 0, at)
		if winner < 0 {
			in.faultf(at, "every task of the race was cancelled")
		}
		return results[winner].value
	})
	add("timeout", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 2, "timeout", at)
		d, ok := args[0].(ir.Duration)
		if !ok || d <= 0 {
			in.faultf(at, "timeout takes how long to wait, such as 5 s, then the function")
		}
		results, _ := in.runTasks([]func(*Interp) Value{call(args[1], at)}, 0, false, time.Duration(d), at)
		if results[0].cancelled {
			return in.none()
		}
		return in.some(results[0].value)
	})
}
