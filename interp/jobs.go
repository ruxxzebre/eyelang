package interp

import (
	"fmt"
	"strings"
	"time"

	"eyelang/ir"
)

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
	switch c := w.Call.(type) {
	case *ir.Call:
		callee := in.eval(f, c.Func)
		args := in.evalArgs(f, c.Args)
		switch fn := callee.(type) {
		case *Closure:
			if fn.Fn.Host != "" || !fn.Fn.Waits {
				// A host function (ai.generate, http.get), or a function
				// that reaches outside without waiting, is a step: its
				// result is recorded, so a resumed run doesn't call it again,
				// and what its model calls spent counts against @budget again.
				if run.cursor < len(run.Log) {
					s := run.Log[run.cursor]
					if v, ok := in.replay(run, w.Pos); ok {
						in.charge(spend{s.Tokens, s.USD})
						return v
					}
				}
				in.call = fn.Fn.Host
				if in.call == "" {
					in.call = fn.Fn.Name
				}
				prev, prevSpend := in.want, in.stepSpend
				in.want, in.stepSpend = wanted(c), &spend{}
				v, _ := in.apply(fn, args, c.Pos)
				spent := *in.stepSpend
				in.want, in.stepSpend = prev, prevSpend
				return in.recordSpend(run, v, spent, c.Pos)
			}
			// A call to a function that itself waits isn't a step; the
			// steps are the waiting calls inside it.
			r, _ := in.apply(fn, args, c.Pos)
			return r
		case *BuiltinFn:
			switch fn.Name {
			case "poll":
				return in.poll(run, args, c.Pos)
			case "next":
				return in.next(run, args, c.Pos)
			case "approval":
				return in.approval(run, args, c.Pos)
			}
		}
		if v, ok := in.replay(run, w.Pos); ok {
			return v
		}
		in.call = callName(callee)
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
		v, _ := in.apply(callee, args, c.Pos)
		return in.record(run, v, c.Pos)
	case *ir.MethodCall:
		recv := in.eval(f, c.Recv)
		args := in.evalArgs(f, c.Args)
		if r, ok := recv.(*RunVal); ok && c.Name == "join" {
			return in.join(run, r, c.Pos)
		}
		if v, ok := in.replay(run, w.Pos); ok {
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
// recorded it. A timer that hasn't run out parks the run again.
func (in *Interp) replay(run *jobRun, at ir.Pos) (Value, bool) {
	if run.cursor >= len(run.Log) {
		return nil, false
	}
	s := &run.Log[run.cursor]
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
	run.Log = append(run.Log, step{Call: in.call, Value: in.encode(v, at), Seq: in.rt.seq(), Tokens: s.Tokens, USD: s.USD})
	run.cursor++
	in.rt.save(run)
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
		if r, _ := in.apply(pos[0], nil, at); r == true {
			return true
		}
		if i >= int64(within/every) {
			return false
		}
		if _, ok := in.replay(run, at); !ok {
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
			if ok, _ := in.apply(where, []argVal{{v: v}}, at); ok != true {
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

// approvalEvent is the event that answers approval requests. Until approvals
// reach people (SPECS.md open question 17), tests answer them with approve
// and decline.
const approvalEvent = "approval"

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
	pre := in.prog.Prelude.Variants
	match := func(v Value) (Value, bool) {
		answer, ok := v.(*Record)
		if !ok {
			return nil, false
		}
		text, _ := answer.get("what")
		if s, _ := text.(string); !strings.Contains(what, s) {
			return nil, false
		}
		if approved, _ := answer.get("approved"); approved == true {
			by, _ := answer.get("by")
			return &Variant{V: pre["Approved"], Vals: []Value{&ApprovalVal{What: what, By: show(by)}}}, true
		}
		return &Variant{V: pre["Declined"]}, true
	}
	expired := &Variant{V: pre["Expired"]}
	if run.cursor < len(run.Log) {
		s := &run.Log[run.cursor]
		if s.Event == "" {
			run.cursor++
			return in.decode(s.Value, at)
		}
		return in.resolveWait(run, s, match, expired, at)
	}
	return in.resolveWait(run, in.waitStep(run, approvalEvent, "approval: "+what, within), match, expired, at)
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
