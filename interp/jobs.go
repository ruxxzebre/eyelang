package interp

import (
	"time"

	"eyelang/ir"
)

// durable runs a waiting call in a job. A step the run already took returns
// its recorded result; a new step happens, and its result is recorded before
// the job goes on.
func (in *Interp) durable(f *frame, w *ir.Waiting) Value {
	run := in.run
	if run == nil {
		in.faultf(w.Pos, "a waiting call ran outside a job")
	}
	// A call to a function that itself waits isn't a step; the steps are
	// the waiting calls inside it.
	if c, ok := w.Call.(*ir.Call); ok {
		if callee, ok := in.eval(f, c.Func).(*Closure); ok {
			r, _ := in.apply(callee, in.evalArgs(f, c.Args), c.Pos)
			return r
		}
	}
	if v, ok := in.replay(run, w.Pos); ok {
		return v
	}
	switch c := w.Call.(type) {
	case *ir.Call:
		callee := in.eval(f, c.Func)
		args := in.evalArgs(f, c.Args)
		if b, ok := callee.(*BuiltinFn); ok && b.Name == "sleep" {
			pos, _ := split(args)
			d, ok := pos[0].(ir.Duration)
			if len(pos) != 1 || !ok {
				in.faultf(c.Pos, "sleep!(…) takes a Duration, such as 20 min")
			}
			return in.sleep(run, in.rt.now.Add(time.Duration(d)))
		}
		v, _ := in.apply(callee, args, c.Pos)
		return in.record(run, v, c.Pos)
	case *ir.MethodCall:
		recv := in.eval(f, c.Recv)
		args := in.evalArgs(f, c.Args)
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
		s.Timer, s.WakeAt, s.Value = false, 0, in.encode(ir.Unit{}, at)
		run.cursor++
		in.rt.save(run)
		return ir.Unit{}, true
	}
	run.cursor++
	return in.decode(s.Value, at), true
}

func (in *Interp) record(run *jobRun, v Value, at ir.Pos) Value {
	run.Log = append(run.Log, step{Value: in.encode(v, at)})
	run.cursor++
	in.rt.save(run)
	return v
}

// sleep records a timer and parks the run until the clock reaches wake.
func (in *Interp) sleep(run *jobRun, wake time.Time) Value {
	if !wake.After(in.rt.now) {
		return in.record(run, ir.Unit{}, ir.Pos{})
	}
	run.Log = append(run.Log, step{Timer: true, WakeAt: wake.UnixNano()})
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
	run.Log = append(run.Log, step{Value: in.encode(result, at)})
	run.cursor++
	tx.run = run
	rt.tx = nil
	rt.commit(tx)
	return result
}
