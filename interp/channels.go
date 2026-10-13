package interp

import (
	"context"
	"sync"

	"eyelang/ir"
)

// Task groups and channels (SPECS.md §13). taskGroup(fn group { … }) runs
// its function as the group's first task; group.spawn(fn { … }) adds
// another, and the group returns only when every task has finished. If one
// fails, the others are cancelled at their next wait and the group fails
// with it.
//
// channel(capacity: n) gives a Sender and a Receiver of one channel. put
// parks while the channel is full and take while it is empty; close ends
// it, and a for loop over the Receiver reads until it is closed and empty.
// A group whose tasks all wait for channels that no task can change any
// more would wait forever: it fails instead, at the taskGroup call.

// GroupVal is a running task group, as its function and tasks see it.
type GroupVal struct {
	v *vsched // on a test's clock
	r *rgroup // on the system clock
}

// chanState is a channel: the values put and not yet taken, at most cap.
type chanState struct {
	buf     []Value
	cap     int
	closed  bool
	waiters []*vtask      // tasks blocked on it, on a test's clock
	changed chan struct{} // closed when it changes, on the system clock
	rwait   []*rwaiter    // tasks waiting for changed
}

// rwaiter is a task waiting for a channel on the system clock. It counts
// as waiting until the channel changes, not until it runs again, so a
// deadlock isn't found while it waits for the lock. g is its task group,
// if it is in one; ctx is its task's; stop wakes it when it is stuck
// outside a group, which stuck then says.
type rwaiter struct {
	g     *rgroup
	gl    *gil
	ctx   context.Context
	stop  chan struct{}
	stuck bool
	woken bool
}

func (w *rwaiter) wake() {
	if !w.woken {
		w.woken = true
		w.gl.active++
		delete(w.gl.waiting, w)
	}
}

// wait counts a task that starts waiting for a channel, and looks for a
// deadlock.
func (gl *gil) wait(w *rwaiter) {
	if gl.waiting == nil {
		gl.waiting = map[*rwaiter]bool{}
	}
	gl.waiting[w] = true
	gl.active--
	gl.checkDeadlock()
}

// checkDeadlock fails what waits for channels when no task can go on: each
// waits for a channel, or for its own tasks to finish, so none will ever
// put into or close a channel again. Code outside tasks only waits for
// them, so the tasks are all there is. A group with a task stuck this way
// fails at the taskGroup call (SPECS.md §13), and a task outside a group,
// as parallelMap and both start, faults where it waits. A task that was
// cancelled will run again to finish, so nothing fails while one is left.
func (gl *gil) checkDeadlock() {
	if gl.active > 0 || len(gl.waiting) == 0 {
		return
	}
	for w := range gl.waiting {
		if w.ctx != nil && w.ctx.Err() != nil {
			return
		}
	}
	for w := range gl.waiting {
		if g := w.g; g != nil {
			if g.fault == nil {
				g.fault = &Fault{Pos: g.at, Msg: "every task here waits for a channel that no task will put into or close"}
				g.cancel()
			}
			continue
		}
		w.stuck = true
		close(w.stop)
		w.wake()
	}
}

// SenderVal and ReceiverVal are a channel's two ends.
type (
	SenderVal   struct{ c *chanState }
	ReceiverVal struct{ c *chanState }
)

// rgroup is a task group on the system clock: its tasks are goroutines
// that take turns holding the interpreter's lock.
type rgroup struct {
	root   *Interp
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	at     ir.Pos
	fault  any
	live   int // tasks not finished
	// owned is set when a task started the group: it waits for the
	// group's tasks, and can go on once the last one finishes.
	owned bool
	done  bool
}

// changed wakes whoever waits for the channel.
func (c *chanState) notify(in *Interp) {
	waiters := c.waiters
	c.waiters = nil
	for _, w := range waiters {
		w.ready(in.rt.now)
	}
	for _, w := range c.rwait {
		w.wake()
	}
	c.rwait = nil
	if c.changed != nil {
		close(c.changed)
		c.changed = nil
	}
}

// wait parks the running task until the channel changes.
func (in *Interp) waitFor(c *chanState, at ir.Pos) {
	t := in.currentTask()
	deadlock := func() {
		in.faultf(at, "this waits for a channel that no task will put into or close")
	}
	if !in.rt.real {
		if t == nil || t.v == nil {
			deadlock()
		}
		c.waiters = append(c.waiters, t.v)
		t.v.park()
		return
	}
	if in.g == nil || t == nil {
		deadlock()
	}
	if c.changed == nil {
		c.changed = make(chan struct{})
	}
	sig := c.changed
	w := &rwaiter{g: t.grp, gl: in.g, ctx: t.ctx, stop: make(chan struct{})}
	c.rwait = append(c.rwait, w)
	defer w.wake() // when cancelled
	in.g.wait(w)
	in.blocking(func(ctx context.Context) {
		select {
		case <-sig:
		case <-ctx.Done():
		case <-w.stop:
		}
	})
	if w.stuck {
		deadlock()
	}
}

func (in *Interp) put(c *chanState, v Value, at ir.Pos) {
	for {
		if c.closed {
			in.faultf(at, "put on a channel that is closed")
		}
		if len(c.buf) < c.cap {
			c.buf = append(c.buf, v)
			c.notify(in)
			return
		}
		in.waitFor(c, at)
	}
}

// take returns the next value, or false when the channel is closed and
// empty.
func (in *Interp) take(c *chanState, at ir.Pos) (Value, bool) {
	for {
		if len(c.buf) > 0 {
			v := c.buf[0]
			c.buf = c.buf[1:]
			c.notify(in)
			return v, true
		}
		if c.closed {
			return nil, false
		}
		in.waitFor(c, at)
	}
}

// taskGroup runs f with a new group, and returns what f returns once every
// task of the group has finished.
func (in *Interp) taskGroup(f Value, at ir.Pos) Value {
	if !in.rt.real {
		s := in.newVsched(0, false, at)
		group := &GroupVal{v: s}
		s.add(func(c *Interp) Value { return c.callValue(f, at, group) })
		results, _ := s.run(0)
		return results[0].value
	}
	if in.g == nil {
		in.g = &gil{}
		in.g.mu.Lock() // the code running now holds it from here on
	}
	parent := in.currentTask()
	ctx := in.taskContext(parent)
	g := &rgroup{root: in, at: at, owned: parent != nil}
	g.ctx, g.cancel = context.WithCancel(ctx)
	defer g.cancel()
	group := &GroupVal{r: g}
	var result Value
	g.spawn(func(c *Interp) Value {
		result = c.callValue(f, at, group)
		return result
	})
	// Wait without holding the lock, so the tasks can run. A task that
	// waits for its group can't go on until the group's last task ends.
	if g.owned {
		in.g.active--
	}
	in.g.mu.Unlock()
	g.wg.Wait()
	in.g.mu.Lock()
	in.g.current = parent
	g.done = true
	if g.fault != nil {
		panic(g.fault)
	}
	if ctx.Err() != nil {
		panic(cancelSignal{})
	}
	return result
}

// spawn starts a task in the group on the system clock. It is called with
// the interpreter's lock held, and the task runs when the caller lets go.
func (g *rgroup) spawn(fn func(*Interp) Value) {
	g.live++
	g.root.g.active++
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		t := &taskCtx{ctx: g.ctx, grp: g}
		g.root.g.mu.Lock()
		g.root.g.current = t
		r := g.root.child(t).runTask(fn)
		g.live--
		g.root.g.active--
		if g.live == 0 && g.owned {
			g.root.g.active++ // the task that waits for the group goes on
		}
		if r.fault != nil && g.fault == nil {
			g.fault = r.fault
			g.cancel()
		}
		g.root.g.checkDeadlock()
		g.root.g.mu.Unlock()
	}()
}

// spawn adds a task to the group.
func (in *Interp) spawn(group *GroupVal, f Value, at ir.Pos) {
	fn := func(c *Interp) Value { return c.callValue(f, at) }
	switch {
	case group.v != nil:
		if group.v.finished {
			in.faultf(at, "this group has finished, so it can't spawn more tasks")
		}
		group.v.add(fn)
	case group.r != nil:
		if group.r.done {
			in.faultf(at, "this group has finished, so it can't spawn more tasks")
		}
		group.r.spawn(fn)
	}
}

// channelBuiltins adds taskGroup and channel.
func channelBuiltins(add func(string, builtinFunc)) {
	add("taskGroup", func(in *Interp, args []Value, _ map[string]Value, at ir.Pos) Value {
		in.need(args, 1, "taskGroup", at)
		return in.taskGroup(args[0], at)
	})
	add("channel", func(in *Interp, _ []Value, named map[string]Value, at ir.Pos) Value {
		n, ok := named["capacity"].(int64)
		if !ok || n < 1 {
			in.faultf(at, "channel's capacity: is how many values it holds before put waits, at least 1")
		}
		c := &chanState{cap: int(n)}
		return &Tuple{Elems: []Value{&SenderVal{c}, &ReceiverVal{c}}}
	})
}

// channelMethod runs spawn, put, close and take.
func (in *Interp) channelMethod(recv Value, name string, args []argVal, at ir.Pos) (Value, bool) {
	pos, _ := split(args)
	switch r := recv.(type) {
	case *GroupVal:
		if name == "spawn" && len(pos) == 1 {
			in.spawn(r, pos[0], at)
			return ir.Unit{}, true
		}
	case *SenderVal:
		switch {
		case name == "put" && len(pos) == 1:
			in.put(r.c, pos[0], at)
			return ir.Unit{}, true
		case name == "close" && len(pos) == 0:
			if !r.c.closed {
				r.c.closed = true
				r.c.notify(in)
			}
			return ir.Unit{}, true
		}
	case *ReceiverVal:
		if name == "take" && len(pos) == 0 {
			if v, ok := in.take(r.c, at); ok {
				return in.some(v), true
			}
			return in.none(), true
		}
	}
	return nil, false
}
