package interp

import (
	"time"

	"eyelang/ir"
)

// Native is a program compiled to native code that the runner executes in
// place of the interpreter (ROADMAP 7.6, compilation.md "Hosting compiled
// programs"). The runner keeps everything else as it is: the store, the
// scheduler, the journal, steps and their replay, checkpoints. Only what
// runs an entity's method or a job's code changes.
//
// Every value crosses as its encoding, the one encode writes, so the store
// holds the same bytes whichever runs the code. Declarations are named as
// the runner names them: one of the program's own module by its name
// ("Counter"), one of an imported module with the module's path in front
// ("lab.notify.Message").
type Native interface {
	// HostsMethod and HostsJob say whether the compiled program runs an
	// entity's method or a job. What it doesn't run, the interpreter does.
	HostsMethod(entity, method string) bool
	HostsJob(job string) bool
	// CallMethod runs an entity's method on its state, given and returned
	// encoded, with the method's arguments in the order of its parameters.
	CallMethod(entity, method string, state []byte, args [][]byte) (newState, result []byte, err error)
	// StepJob runs a job to its next `!` step or to its end: from its
	// arguments when from is nil, and otherwise from the step it stopped
	// at, with resumed, what that step gave.
	StepJob(job string, from *NativeStep, args [][]byte, resumed []byte) (NativeStep, error)
}

// NativeStep is where a native job stopped: at its end, with its result,
// or at a step for the runner to take, with the frame to resume from. A
// run saves it as its checkpoint: the run's Log holds only the steps taken
// since, which a run that wakes replays.
type NativeStep struct {
	Done   bool   `json:",omitempty"`
	Result []byte `json:",omitempty"`
	Frame  []byte `json:",omitempty"`
	// The step: a sleep for Sleep, or a call of an entity's method.
	Sleep  time.Duration `json:",omitempty"`
	Entity string        `json:",omitempty"`
	Method string        `json:",omitempty"`
	ID     []byte        `json:",omitempty"`
	Args   [][]byte      `json:",omitempty"`
	// ResultType is the type of what the step gives back, by the compiled
	// program's own numbering, for it to read resumed by.
	ResultType int32 `json:",omitempty"`
}

// nativeName names a declaration as Native does.
func nativeName(m *ir.Module, name string) string {
	if m == nil || m.Path == "" {
		return name
	}
	return m.Path + "." + name
}

// paramValues orders a call's arguments by the function's parameters, with
// the defaults of those left out.
func (in *Interp) paramValues(fn *ir.Func, args []argVal, at ir.Pos) []Value {
	fr := in.newFrame(fn)
	in.bind(fn, fr, args, at)
	out := make([]Value, len(fn.Params))
	for i, p := range fn.Params {
		out[i] = fr.slots[p.Slot]
	}
	in.freeFrame(fr)
	return out
}

// nativeMethod runs an entity's method natively on the row's state.
func (in *Interp) nativeMethod(m *ir.Func, row *txRow, args []argVal, at ir.Pos) Value {
	var encoded [][]byte
	for _, v := range in.paramValues(m, args, at) {
		encoded = append(encoded, in.encode(v, at))
	}
	e := row.ref.Entity
	state, result, err := in.native.CallMethod(nativeName(e.Module, e.Name), m.Name, in.encode(row.state, at), encoded)
	if err != nil {
		in.faultf(at, "%s.%s: %v", e.Name, m.Name, err)
	}
	s, ok := in.decode(state, at).(*Record)
	if !ok {
		in.faultf(at, "%s.%s gave back a state that isn't a record", e.Name, m.Name)
	}
	row.state = s
	return in.decode(result, at)
}

// runNativeJob runs a job natively for a run, from its start or from its
// saved place, taking its steps as the interpreter takes a job's: a step
// the run took before it woke is replayed, and a new one is recorded.
func (in *Interp) runNativeJob(run *jobRun, job *ir.Job, args []argVal) Value {
	name := nativeName(job.Module, job.Name)
	var encoded [][]byte
	if run.Native == nil {
		for _, v := range in.paramValues(job.Func, args, job.Pos) {
			encoded = append(encoded, in.encode(v, job.Pos))
		}
	}
	for {
		var resumed []byte
		if run.Native != nil {
			resumed = in.encode(in.nativeStep(run, run.Native, job.Pos), job.Pos)
		}
		s, err := in.native.StepJob(name, run.Native, encoded, resumed)
		if err != nil {
			in.faultf(job.Pos, "%v", err)
		}
		if s.Done {
			run.Native = nil
			return in.decode(s.Result, job.Pos)
		}
		// The new place replaces the run's checkpoint. The step's commit
		// saves it, so a crash before the step leaves the run where it was,
		// which gives the same step again.
		run.Native = &s
		run.Checkpoint, run.Log, run.cursor = nil, nil, 0
	}
}

// nativeStep takes the step a native job stopped at, or replays it.
func (in *Interp) nativeStep(run *jobRun, s *NativeStep, at ir.Pos) Value {
	if s.Entity == "" {
		in.call = "sleep"
		if v, ok := in.replay(run, in.call, at); ok {
			return v
		}
		return in.sleep(run, in.rt.now.Add(s.Sleep))
	}
	var e *ir.Entity
	for _, x := range in.entities {
		if nativeName(x.Module, x.Name) == s.Entity {
			e = x
		}
	}
	if e == nil {
		in.faultf(at, "the compiled program calls %s, which the program doesn't declare", s.Entity)
	}
	meth, ok := e.Methods[s.Method]
	if !ok {
		in.faultf(at, "%s has no method %s", e.Name, s.Method)
	}
	ref := &EntityRef{Entity: e, ID: in.decode(s.ID, at)}
	in.call = methodName(ref, s.Method)
	if v, ok := in.replay(run, in.call, at); ok {
		return v
	}
	var args []argVal
	for _, a := range s.Args {
		args = append(args, argVal{v: in.decode(a, at)})
	}
	return in.entityStep(run, ref, meth, args, at)
}
