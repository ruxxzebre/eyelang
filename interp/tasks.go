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
// clock, and run by a scheduler on a test's. grp is the task group it
// belongs to on the system clock, if any.
type taskCtx struct {
	ctx context.Context
	v   *vtask
	grp *rgroup
}

// vtask is a task on a test's clock. Tasks are goroutines, but only one runs
// at a time: a task that waits hands control back to its scheduler, which
// resumes the task that wakes first (the one started first, on a tie) and
// moves the clock to its wake time, so every run of a test is the same.
//
// A task that waits for a channel is blocked rather than sleeping: it has no
// wake time, and becomes ready when another task changes the channel.
type vtask struct {
	index    int
	s        *vsched
	sched    chan *vtask // where it hands control back: it waits or finished
	resume   chan bool   // the scheduler's go-ahead; true cancels the task
	wake     time.Time
	started  bool
	sleeping bool
	blocked  bool
	done     bool
	result   taskResult
}

// ready makes a task that was blocked on a channel ready to run now. The
// task that runs its scheduler may itself be blocked, because every task it
// runs was, or asleep until a later time: it is woken now too, and so on up.
func (v *vtask) ready(now time.Time) {
	if !v.blocked {
		return
	}
	v.blocked, v.sleeping, v.wake = false, true, now
	for o := v.s.owner; o != nil; o = o.s.owner {
		switch {
		case o.blocked:
			o.blocked, o.sleeping, o.wake = false, true, now
		case o.sleeping && o.wake.After(now):
			o.wake = now
		default:
			return
		}
	}
}

// cancelSignal unwinds a task that was cancelled.
type cancelSignal struct{}

// gil guards the interpreter's shared state while tasks run on goroutines.
// Whoever holds it is the only goroutine running Eyelang code; current is
// that goroutine's task.
type gil struct {
	mu      sync.Mutex
	current *taskCtx
	// active counts the tasks that can go on: running, waiting for the
	// lock, or waiting for anything but a channel or their own tasks.
	// waiting are the tasks waiting for a channel (checkDeadlock).
	active  int
	waiting map[*rwaiter]bool
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
	ctx := in.taskContext(t)
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

// taskContext is what cancels the code of task t, or, outside a task,
// main's code under eye run when a signal stops it.
func (in *Interp) taskContext(t *taskCtx) context.Context {
	if t != nil && t.ctx != nil {
		return t.ctx
	}
	if in.rt.signalled != nil {
		return in.rt.signalled
	}
	return context.Background()
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
	s := in.newVsched(limit, first, at)
	for _, fn := range fns {
		s.add(fn)
	}
	return s.run(within)
}

// child is the interpreter as a task sees it: the same program and shared
// state, with its own per-call state.
func (in *Interp) child(t *taskCtx) *Interp {
	c := *in
	c.task, c.run, c.want, c.call, c.free, c.depth = t, nil, nil, "", nil, 0 // a goroutine, with a stack of its own
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

// vsched runs tasks on a test's clock: one at a time, each until it waits
// or finishes, always resuming the one that wakes first. The task that runs
// the scheduler (its owner) waits through its own pause, so tasks inside
// tasks share one timeline. A task group adds tasks while it runs.
type vsched struct {
	in       *Interp
	at       ir.Pos
	ch       chan *vtask
	tasks    []*vtask
	fns      []func(*Interp) Value
	owner    *vtask // the task that runs this scheduler; nil at the top
	limit    int    // tasks running at once; 0 for no limit
	first    bool   // the first to finish cancels the rest
	running  int
	next     int // the first task not started yet
	winner   int
	stopping bool
	finished bool
	fault    any
}

func (in *Interp) newVsched(limit int, first bool, at ir.Pos) *vsched {
	s := &vsched{in: in, at: at, ch: make(chan *vtask), limit: limit, first: first, winner: -1}
	if t := in.currentTask(); t != nil {
		s.owner = t.v
	}
	return s
}

// add adds a task, which starts when the running one waits.
func (s *vsched) add(fn func(*Interp) Value) {
	v := &vtask{index: len(s.tasks), s: s, sched: s.ch, resume: make(chan bool)}
	s.tasks = append(s.tasks, v)
	s.fns = append(s.fns, fn)
}

// settle takes control back from the task that ran: it waits or finished.
func (s *vsched) settle() {
	v := <-s.ch
	if !v.done {
		return
	}
	s.running--
	switch r := v.result; {
	case r.fault != nil:
		if s.fault == nil {
			s.fault = r.fault
		}
		s.stopping = true
	case !r.cancelled && s.winner < 0:
		s.winner = v.index
		s.stopping = s.stopping || s.first
	}
}

// cancel cancels every task that has started and not finished: each wakes
// at its wait and unwinds. Those not started yet never will be.
func (s *vsched) cancel() {
	for _, v := range s.tasks {
		if v.started && !v.done {
			v.sleeping, v.blocked = false, false
			v.resume <- true
			s.settle()
		}
	}
	for _, v := range s.tasks[s.next:] {
		v.result.cancelled = true
	}
	s.next = len(s.tasks)
}

// run runs the tasks until all have finished, or until within, and returns
// what each ended with and which finished first.
func (s *vsched) run(within time.Duration) ([]taskResult, int) {
	in := s.in
	defer func() {
		s.finished = true
		if p := recover(); p != nil {
			// The owner was cancelled while it waited for its tasks: they
			// are cancelled with it.
			s.cancel()
			panic(p)
		}
	}()
	var deadline time.Time
	if within > 0 {
		deadline = in.rt.now.Add(within)
	}
	for {
		for !s.stopping && s.next < len(s.tasks) && (s.limit <= 0 || s.running < s.limit) {
			v, fn := s.tasks[s.next], s.fns[s.next]
			child := in.child(&taskCtx{v: v})
			v.started = true
			s.running++
			s.next++
			go func() {
				v.result = child.runTask(fn)
				v.done, v.sleeping, v.blocked = true, false, false
				s.ch <- v
			}()
			s.settle()
		}
		if s.stopping {
			s.cancel()
			break
		}
		var soonest *vtask
		blocked := false
		for _, v := range s.tasks {
			if !v.started || v.done {
				continue
			}
			blocked = blocked || v.blocked
			if v.sleeping && (soonest == nil || v.wake.Before(soonest.wake)) {
				soonest = v
			}
		}
		if soonest == nil {
			if !blocked {
				break
			}
			if s.owner == nil {
				// Every task waits for a channel, and no task is left to
				// put into one or close it.
				s.fault = &Fault{Pos: s.at, Msg: "every task here waits for a channel that no task will put into or close"}
				s.stopping = true
				continue
			}
			// A task outside may still change the channel: the owner waits
			// for it, blocked too.
			s.owner.park()
			continue
		}
		if !deadline.IsZero() && soonest.wake.After(deadline) {
			if d := deadline.Sub(in.rt.now); d > 0 {
				in.pause(d)
				continue
			}
			s.stopping = true
			continue
		}
		if d := soonest.wake.Sub(in.rt.now); d > 0 {
			// The owner may be woken earlier, when a task outside makes one
			// of these ready: look again then.
			in.pause(d)
			continue
		}
		soonest.sleeping = false
		soonest.resume <- false
		s.settle()
	}
	if s.fault != nil {
		panic(s.fault)
	}
	results := make([]taskResult, len(s.tasks))
	for i, v := range s.tasks {
		results[i] = v.result
	}
	return results, s.winner
}

// park hands control back to the task's scheduler until something makes it
// ready; a cancelled task unwinds.
func (v *vtask) park() {
	v.blocked, v.sleeping = true, false
	v.sched <- v
	if <-v.resume {
		panic(cancelSignal{})
	}
}

// runParallel runs tasks on goroutines on the system clock.
func (in *Interp) runParallel(fns []func(*Interp) Value, limit int, first bool, within time.Duration, at ir.Pos) ([]taskResult, int) {
	root := in
	if in.g == nil {
		in.g = &gil{}
		in.g.mu.Lock() // the code running now holds it from here on
	}
	base := in.taskContext(in.currentTask())
	ctx, cancel := context.WithCancel(base)
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
	// What the tasks count for checkDeadlock, changed with the lock held:
	// each can go on until it ends, and a task that waits for them can go
	// on once the last one has.
	pending, owned := len(fns), false
	root.g.active += len(fns)
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
			root.g.active--
			if pending--; pending == 0 && owned {
				root.g.active++
			}
			root.g.checkDeadlock()
			root.g.mu.Unlock()
			done <- i
		}(i, fn)
	}
	// Wait without holding the lock, so the tasks can run; the first to
	// finish cancels the rest when first is set, and a fault cancels all.
	parent := in.currentTask()
	if parent != nil {
		owned = true
		in.g.active--
	}
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
	if base.Err() != nil {
		panic(cancelSignal{}) // the caller was cancelled, its tasks with it
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
