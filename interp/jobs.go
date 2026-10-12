package interp

import (
	"fmt"
	"strings"
	"time"

	"eyelang/ir"
)

// waitingBuiltins are the built-ins whose waiting calls record their own
// steps, and replay them.
var waitingBuiltins = map[string]bool{"poll": true, "next": true, "approval": true, "reserve": true, "lock": true, "allDone": true, "firstDone": true}

// durable runs a waiting call in a job. A step the run already took returns
// its recorded result; a new step happens, and its result is recorded before
// the job goes on.
func (in *Interp) durable(f *frame, w *ir.Waiting) Value {
	run := in.run
	if run == nil && in.task != nil {
		in.faultf(w.Pos, "a task in a job can only compute: its steps couldn't be recorded, so start a child run for durable work (SPECS.md §13)")
	}
	if run == nil {
		in.faultf(w.Pos, "a waiting call ran outside a job")
	}
	// The callee or receiver and the arguments come first: they may hold
	// waiting calls of their own, as `a!().reconcile!()` does, whose steps
	// come before this one.
	call := w.Call
	if m, ok := call.(*ir.MethodCall); ok {
		switch t := m.Target.(type) {
		case ir.FieldCall:
			call = t.Call // a field that holds a function
		case ir.FuncCall:
			call = t.Call // a function called on its first argument
		}
	}
	switch c := call.(type) {
	case *ir.Call:
		callee := in.eval(f, c.Func)
		args := in.evalArgs(f, c.Args)
		switch fn := callee.(type) {
		case *Closure:
			if fn.Fn.Host == "ai.generate" && hasTools(fn.Fn, args) {
				// Its replies and its tools' results are steps of their
				// own, which it records and replays itself (tools.go).
				return in.generateSteps(run, fn, args, wanted(c), c.Pos)
			}
			if fn.Fn.Host != "" || !fn.Fn.Waits {
				// A host function (ai.generate, http.get), or a function
				// that reaches outside without waiting, is a step: its
				// result is recorded, so a resumed run doesn't call it again,
				// and what its model calls spent counts against @budget again.
				call := fn.Fn.Host
				if call == "" {
					call = fn.Fn.Name
				}
				if run.cursor < len(run.Log) {
					s := run.Log[run.cursor]
					if v, ok := in.replay(run, call, w.Pos); ok {
						in.charge(s.spent())
						return v
					}
				}
				in.call = call
				if fn.Fn.Host == "" {
					in.rt.crossed("step") // a host function counts as itself, in callHost
				}
				prev, prevSpend := in.want, in.stepSpend
				in.want, in.stepSpend = wanted(c), &spend{}
				v := in.apply(fn, args, c.Pos)
				spent := *in.stepSpend
				in.want, in.stepSpend = prev, prevSpend
				return in.recordSpend(run, v, spent, c.Pos)
			}
			// A call to a function that itself waits isn't a step; the
			// steps are the waiting calls inside it.
			r := in.apply(fn, args, c.Pos)
			return r
		case *BuiltinFn:
			if waitingBuiltins[fn.Name] && run.cursor >= len(run.Log) {
				in.rt.crossed("step")
			}
			switch fn.Name {
			case "poll":
				return in.poll(run, args, c.Pos)
			case "next":
				return in.next(run, args, c.Pos)
			case "approval":
				return in.approval(run, args, c.Pos)
			case "reserve":
				return in.reserve(run, args, c.Pos)
			case "lock":
				return in.lock(run, args, c.Pos)
			case "allDone":
				return in.allDone(run, args, c.Pos)
			case "firstDone":
				return in.firstDone(run, args, c.Pos)
			}
		}
		if v, ok := in.replay(run, callName(callee), w.Pos); ok {
			return v
		}
		in.call = callName(callee)
		if b, ok := callee.(*BuiltinFn); ok && !hostBuiltin(b.Name) {
			in.rt.crossed("step") // an operation counts as op.call, in apply, and env.get as itself
		}
		if b, ok := callee.(*BuiltinFn); ok && (b.Name == "sleep" || b.Name == "sleepUntil") {
			pos, _ := split(args)
			if b.Name == "sleep" {
				d, ok := pos[0].(ir.Duration)
				if len(pos) != 1 || !ok {
					in.faultf(c.Pos, "sleep!(…) takes a Duration, such as 20 min")
				}
				return in.sleep(run, in.rt.now.Add(time.Duration(d)))
			}
			at, ok := pos[0].(time.Time)
			if len(pos) != 1 || !ok {
				in.faultf(c.Pos, "sleepUntil!(…) takes an Instant, such as 08:00 in dublin")
			}
			return in.sleep(run, at)
		}
		v := in.apply(callee, args, c.Pos)
		return in.record(run, v, c.Pos)
	case *ir.MethodCall:
		if c.Name == "read" && entitySet(c.Recv) {
			// A view or E.all read as a step: a resumed run gets the
			// snapshots the first read saw.
			name := "read " + c.Recv.Position().Text()
			if v, ok := in.replay(run, name, w.Pos); ok {
				return v
			}
			in.call = name
			return in.record(run, in.snapshots(in.eval(f, c.Recv), c.Pos), c.Pos)
		}
		recv := in.eval(f, c.Recv)
		args := in.evalArgs(f, c.Args)
		if r, ok := recv.(*RunVal); ok && c.Name == "join" {
			if run.cursor >= len(run.Log) {
				in.rt.crossed("step")
			}
			return in.join(run, r, c.Pos)
		}
		if j, ok := recv.(*JobVal); ok && c.Name == "start" && in.rt.tx == nil {
			// startChild replays or records the step itself.
			pos, named := split(args)
			return in.startChild(run, j.Job, pos, named, c.Pos)
		}
		if v, ok := in.replay(run, methodName(recv, c.Name), w.Pos); ok {
			return v
		}
		in.call = methodName(recv, c.Name)
		if ref, ok := recv.(*EntityRef); ok && c.Name != "read" {
			meth, ok := ref.Entity.Methods[c.Name]
			if !ok {
				in.faultf(c.Pos, "%s has no method %s", ref.Entity.Name, c.Name)
			}
			return in.entityStep(run, ref, meth, args, c.Pos)
		}
		v, _ := in.method(recv, c.Name, args, c.Pos, true)
		return in.record(run, v, c.Pos)
	}
	in.faultf(w.Pos, "this waiting call isn't supported yet")
	return nil
}

// startChild starts a child run from a job's run. The start is a step of the
// parent: a replayed parent gets the run it started the first time, rather
// than starting another, and the child is saved in the same commit as the
// step, so a crash can't leave one without the other.
func (in *Interp) startChild(run *jobRun, job *ir.Job, args []Value, named map[string]Value, at ir.Pos) Value {
	if v, ok := in.replay(run, "start "+job.Name, at); ok {
		return v
	}
	rt := in.rt
	rt.crossed("job.start")
	in.call = "start " + job.Name
	if v, ok := rt.keyed(job, named); ok {
		return in.record(run, v, at)
	}
	child := rt.newRun(job, args, named, at)
	v := &RunVal{ID: child.ID}
	run.Log = append(run.Log, step{Call: in.call, Value: in.encode(v, at), Seq: rt.seq()})
	run.cursor++
	rt.store.Commit(&Commit{Runs: []RunWrite{{ID: child.ID, Data: child.encode()}, {ID: run.ID, Data: run.encode()}}})
	if child.Status == "ready" {
		rt.queue = append(rt.queue, child.ID)
	}
	return v
}

// join waits for a run started earlier and returns its outcome:
// Completed(value), or Failed(fault) if it stopped first. A run that ends
// publishes its outcome as an event, run/<id>, which wakes the runs joining it.
func (in *Interp) join(run *jobRun, target *RunVal, at ir.Pos) Value {
	match := func(v Value) (Value, bool) { return v, true }
	if run.cursor < len(run.Log) {
		s := &run.Log[run.cursor]
		if s.Event == "" {
			run.cursor++
			return in.decode(s.Value, at)
		}
		return in.resolveWait(run, s, match, nil, at)
	}
	in.call = fmt.Sprintf("join #%d", target.ID)
	if data, ok := in.rt.store.Run(target.ID); ok {
		if r := decodeRun(data); r.Status == "done" || r.Status == "failed" {
			return in.record(run, in.outcome(r, at), at)
		}
	}
	s := in.waitStep(run, runEvent(target.ID), in.call, ir.Duration(100*365*24*time.Hour))
	return in.resolveWait(run, s, match, nil, at)
}

func runEvent(id int64) string { return fmt.Sprintf("run/%d", id) }

// runOutcome is `run.outcome()` outside a job: Some(outcome) once the run
// has ended, None while it is still going.
func (in *Interp) runOutcome(target *RunVal, at ir.Pos) Value {
	data, ok := in.rt.store.Run(target.ID)
	if !ok {
		in.faultf(at, "run #%d isn't kept in the store", target.ID)
	}
	if r := decodeRun(data); r.Status == "done" || r.Status == "failed" {
		return in.some(in.outcome(r, at))
	}
	return in.none()
}

// outcome is how a finished run ended, as the prelude's Outcome.
func (in *Interp) outcome(r *jobRun, at ir.Pos) Value {
	pre := in.prog.Prelude.Variants
	if r.Status == "done" {
		return &Variant{V: pre["Completed"], Vals: []Value{in.decode(r.Result, at)}}
	}
	kind, arg := r.FaultKind, r.FaultArg
	if kind == "" {
		kind, arg = "Unreachable", r.Fault
	}
	fault := &Variant{V: pre[kind]}
	if fault.V == nil {
		fault = &Variant{V: pre["Unreachable"], Vals: []Value{r.Fault}}
	} else if len(fault.V.Fields) == 1 {
		fault.Vals = []Value{arg}
	}
	return &Variant{V: pre["Failed"], Vals: []Value{fault}}
}

// replay returns the result of the next recorded step, if the run has
// recorded it. A timer that hasn't run out parks the run again. call is
// what the step about to happen calls, named as it is recorded: a step
// recorded for another call means the job's code changed under the run,
// which faults rather than hand the call another's result (SPECS.md §15).
func (in *Interp) replay(run *jobRun, call string, at ir.Pos) (Value, bool) {
	if run.cursor >= len(run.Log) {
		return nil, false
	}
	s := &run.Log[run.cursor]
	if call != "" && s.Call != "" && s.Call != call {
		n := run.cursor + 1
		if run.Checkpoint != nil {
			n += run.Checkpoint.Steps
		}
		in.faultf(at, "the code of %s changed while this run was paused: its step %d called %s, and the code now calls %s there", strings.TrimPrefix(run.Job, "."), n, s.Call, call)
	}
	if s.Timer {
		if in.rt.now.UnixNano() < s.WakeAt {
			run.WakeAt = s.WakeAt
			panic(parkSignal{})
		}
		// The timer has run out; its wake time stays, for eye runs.
		s.Timer, s.Value = false, in.encode(ir.Unit{}, at)
		run.cursor++
		in.rt.save(run)
		return ir.Unit{}, true
	}
	run.cursor++
	return in.decode(s.Value, at), true
}

// callName and methodName name what a step calls, for eye runs.
func callName(callee Value) string {
	switch c := callee.(type) {
	case *BuiltinFn:
		return c.Name
	case *OpVal:
		return c.Op.Contract.Module.Name + "." + c.Op.Name
	}
	return kindName(callee)
}

func methodName(recv Value, name string) string {
	switch r := recv.(type) {
	case *EntityRef:
		return r.Entity.Name + "(" + show(r.ID) + ")." + name
	case *EventVal:
		return r.Event.Contract.Module.Name + "." + r.Event.Name + "." + name
	}
	return name
}

func (in *Interp) record(run *jobRun, v Value, at ir.Pos) Value {
	return in.recordSpend(run, v, spend{}, at)
}

// recordSpend records a step that spent tokens and money on model calls.
func (in *Interp) recordSpend(run *jobRun, v Value, s spend, at ir.Pos) Value {
	run.Log = append(run.Log, step{Call: in.call, Value: in.encode(v, at), Seq: in.rt.seq(), Tokens: s.Tokens, TokensIn: s.In, TokensOut: s.Out, USD: s.USD,
		Model: s.Model, Options: s.Options, Sent: s.Sent, Received: s.Received})
	run.cursor++
	in.checkHeld(run, in.rt.save(run), at)
	return v
}

// poll calls check until it returns true, sleeping between calls, for at
// most within. The calls and the sleeps are the run's steps; the number of
// attempts, not the clock, decides when it gives up, so a resumed run takes
// the same path.
func (in *Interp) poll(run *jobRun, args []argVal, at ir.Pos) Value {
	pos, named := split(args)
	every, ok1 := named["every"].(ir.Duration)
	within, ok2 := named["within"].(ir.Duration)
	if len(pos) != 1 || !ok1 || !ok2 || every <= 0 {
		in.faultf(at, "poll!(every: …, within: …, fn { … }) takes two Durations and a function")
	}
	for i := int64(0); ; i++ {
		if r := in.apply(pos[0], nil, at); r == true {
			return true
		}
		if i >= int64(within/every) {
			return false
		}
		if _, ok := in.replay(run, "poll", at); !ok {
			in.call = "poll"
			in.sleep(run, in.rt.now.Add(time.Duration(every)))
		}
	}
}

// next waits for the first event that matches, counting every one published
// since the run's previous step, so an event that came quickly isn't missed.
// While it waits the run sleeps until `within` runs out, and each event of
// that kind wakes it to look again.
func (in *Interp) next(run *jobRun, args []argVal, at ir.Pos) Value {
	pos, named := split(args)
	var ev *EventVal
	if len(pos) == 1 {
		ev, _ = pos[0].(*EventVal)
	}
	within, ok := named["within"].(ir.Duration)
	if ev == nil || !ok {
		in.faultf(at, "next!(event, where: fn e: …, within: duration) takes an event and how long to wait")
	}
	where := named["where"]
	if run.cursor < len(run.Log) {
		s := &run.Log[run.cursor]
		if s.Event == "" {
			run.cursor++
			return in.decode(s.Value, at)
		}
		return in.resolveWait(run, s, in.matchEvent(where, at), in.none(), at)
	}
	s := in.waitStep(run, eventName(ev.Event), "next", within)
	return in.resolveWait(run, s, in.matchEvent(where, at), in.none(), at)
}

// matchEvent is how next! picks an event: the first for which where is true.
func (in *Interp) matchEvent(where Value, at ir.Pos) func(Value) (Value, bool) {
	return func(v Value) (Value, bool) {
		if where != nil {
			if ok := in.apply(where, []argVal{{v: v}}, at); ok != true {
				return nil, false
			}
		}
		return in.some(v), true
	}
}

// waitStep adds a step that waits for an event named name, counting those
// published since the run's previous step, for at most within.
func (in *Interp) waitStep(run *jobRun, name, call string, within ir.Duration) *step {
	since := run.Since
	if n := len(run.Log); n > 0 {
		since = run.Log[n-1].Seq
	}
	run.Log = append(run.Log, step{
		Call:   call,
		Event:  name,
		Since:  since,
		WakeAt: in.rt.now.Add(time.Duration(within)).UnixNano(),
	})
	return &run.Log[len(run.Log)-1]
}

// approvalEvent is the event that answers approval requests: approvals.answered,
// the standard approvals module's event, whether or not the program imports
// it. Tests publish it with approve and decline, and `eye approve` through
// the runner's inbox.
const approvalEvent = "approvals.answered"

// approvalsEvent returns an event of the standard approvals module, if the
// program imports it and so can hear it: requested, which a run publishes
// when it starts waiting for an approval, or answered.
func (in *Interp) approvalsEvent(name string) *ir.Event {
	for _, m := range in.prog.Modules {
		if m.Path == "approvals" && m.Dir == "" {
			if c := m.Contracts["Approvals"]; c != nil {
				return c.Events[name]
			}
		}
	}
	return nil
}

// approval asks a person to approve what, and waits for the answer:
// Approved(approval), Declined, or Expired once within runs out.
func (in *Interp) approval(run *jobRun, args []argVal, at ir.Pos) Value {
	pos, named := split(args)
	what, ok1 := "", false
	if len(pos) == 1 {
		what, ok1 = pos[0].(string)
	}
	within, ok2 := named["within"].(ir.Duration)
	if !ok1 || !ok2 {
		in.faultf(at, "approval!(what, from: person, within: duration) takes what to approve and how long to wait")
	}
	// from and an answer's by are compared as text, as interpolation shows
	// them: the routing job writes by as "{press.by}".
	from := in.display(named["from"])
	pre := in.prog.Prelude.Variants
	expired := &Variant{V: pre["Expired"]}
	var s *step
	if run.cursor < len(run.Log) {
		s = &run.Log[run.cursor]
		if s.Event == "" {
			run.cursor++
			return in.decode(s.Value, at)
		}
	} else {
		s = in.waitStep(run, approvalEvent, "approval: "+what, within)
		s.Request, s.From = requestID(run, len(run.Log)-1), from
		if e := in.approvalsEvent("requested"); e != nil {
			// The id names the run and the step, so a run that replays this
			// step after a restart publishes nothing new: the event's key was
			// seen.
			in.rt.publish(e, &Record{
				Names: []string{"id", "what", "from", "until"},
				Vals:  []Value{s.Request, what, from, time.Unix(0, s.WakeAt).UTC()},
			})
		}
	}
	id := s.Request
	if id == "" {
		id = requestID(run, run.cursor) // a step saved before ids were kept
	}
	// An answer counts when it names this request and comes from the person
	// asked. One from anyone else was logged when it was published (see
	// checkAnswer), and the run keeps waiting.
	match := func(v Value) (Value, bool) {
		answer, ok := v.(*Record)
		if !ok {
			return nil, false
		}
		if got, _ := answer.get("id"); got != id {
			return nil, false
		}
		if by, _ := answer.get("by"); by != from {
			return nil, false
		}
		if approved, _ := answer.get("approved"); approved == true {
			return &Variant{V: pre["Approved"], Vals: []Value{&ApprovalVal{What: what, By: from}}}, true
		}
		return &Variant{V: pre["Declined"]}, true
	}
	return in.resolveWait(run, s, match, expired, at)
}

// requestID is the id of the approval request that run's step i makes: the
// run and the step, counting those a checkpoint dropped.
func requestID(run *jobRun, i int) string {
	if run.Checkpoint != nil {
		i += run.Checkpoint.Steps
	}
	return fmt.Sprintf("%d-%d", run.ID, i)
}

// asked is an approval request a run waits for: its id, what it asks, and
// whom, as text (empty for a request saved before 2026-10-12).
type asked struct {
	run            int64
	job            string
	id, what, from string
}

// askedBy returns the request run waits for at approval!, if it waits for
// one. A waiting step is the last of the run's steps.
func askedBy(run *jobRun) (asked, bool) {
	n := len(run.Log)
	if (run.Status != "sleeping" && run.Status != "ready") || n == 0 || run.Log[n-1].Event != approvalEvent {
		return asked{}, false
	}
	s := run.Log[n-1]
	a := asked{run: run.ID, job: strings.TrimPrefix(run.Job, "."), id: s.Request, what: strings.TrimPrefix(s.Call, "approval: "), from: s.From}
	if a.id == "" {
		a.id = requestID(run, n-1)
	}
	return a, true
}

// waitingApprovals returns the approval requests the program's runs wait
// for, oldest run first.
func (rt *runtime) waitingApprovals() []asked {
	// Only a ready run's record, or that of one that sleeps until an
	// answer, can hold a request: no other is read.
	var ids []int64
	rt.index.each(func(id int64, h *runHandle) {
		if h.status == readyRun || h.status == sleepingRun && h.event == approvalEvent {
			ids = append(ids, id)
		}
	})
	var out []asked
	for _, id := range ids {
		data, ok := rt.store.Run(id)
		if !ok {
			continue
		}
		if a, ok := askedBy(decodeRun(data)); ok {
			out = append(out, a)
		}
	}
	return out
}

// approvalsWith returns the waiting approval requests whose text contains
// text, as `eye approve TEXT` and approve(TEXT) in tests find them.
func (rt *runtime) approvalsWith(text string) []asked {
	var out []asked
	for _, a := range rt.waitingApprovals() {
		if strings.Contains(a.what, text) {
			out = append(out, a)
		}
	}
	return out
}

// checkAnswer logs an answer that the request it names refuses, because it
// comes from someone other than the person asked. It is called once, when
// the answer is published, rather than each time the run looks at it.
func (rt *runtime) checkAnswer(v Value) {
	answer, ok := v.(*Record)
	if !ok {
		return
	}
	id, _ := answer.get("id")
	by, _ := answer.get("by")
	for _, a := range rt.waitingApprovals() {
		if a.id == id && a.from != "" && by != a.from {
			fmt.Fprintf(rt.in.log, "refused an answer to approval %s (%q, run #%d of %s) by %s: only %s, whom it asks, may answer it; the run keeps waiting\n",
				a.id, a.what, a.run, a.job, show(by), a.from)
		}
	}
}

// resolveWait ends a waiting step with what match makes of the first event
// it accepts, or with timeout once its time is up; otherwise the run sleeps
// until one of them happens.
func (in *Interp) resolveWait(run *jobRun, s *step, match func(Value) (Value, bool), timeout Value, at ir.Pos) Value {
	finish := func(v Value) Value {
		*s = step{Call: s.Call, Value: in.encode(v, at), Seq: in.rt.seq()}
		run.cursor++
		run.WaitEvent = ""
		in.rt.save(run)
		return v
	}
	for _, e := range in.rt.eventsAfter(s.Since) {
		if e.name != s.Event {
			continue
		}
		if v, ok := match(in.decode(e.data, at)); ok {
			return finish(v)
		}
	}
	if in.rt.now.UnixNano() >= s.WakeAt {
		return finish(timeout)
	}
	run.WakeAt = s.WakeAt
	run.WaitEvent = s.Event
	in.rt.save(run)
	panic(parkSignal{})
}

// sleep records a timer and parks the run until the clock reaches wake.
func (in *Interp) sleep(run *jobRun, wake time.Time) Value {
	if !wake.After(in.rt.now) {
		return in.record(run, ir.Unit{}, ir.Pos{})
	}
	run.Log = append(run.Log, step{Call: in.call, Timer: true, WakeAt: wake.UnixNano(), Seq: in.rt.seq()})
	run.WakeAt = wake.UnixNano()
	in.rt.save(run)
	panic(parkSignal{})
}

// entityStep calls an entity method from a job. The method's transaction
// and the recorded step commit together, so the call happens exactly once.
func (in *Interp) entityStep(run *jobRun, ref *EntityRef, meth *ir.Func, args []argVal, at ir.Pos) Value {
	rt := in.rt
	rt.crossed("entity.call")
	tx := rt.begin()
	defer func() {
		if rt.tx == tx {
			rt.tx = nil
		}
	}()
	result := rt.callIn(tx, ref, meth, args, at)
	run.Log = append(run.Log, step{Call: in.call, Value: in.encode(result, at), Seq: in.rt.seq()})
	run.cursor++
	tx.run = run
	rt.tx = nil
	rt.commit(tx)
	return result
}
