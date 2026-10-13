package interp

import (
	"fmt"
	"time"

	"eyelang/ir"
)

// Native is a program compiled to native code that the runner executes in
// place of the interpreter (ROADMAP 7.6, compilation.md "Hosting compiled
// programs"). The runner keeps everything else as it is: the store, the
// scheduler, the journal, steps and their replay, checkpoints. Only what
// runs main, a test, an entity's method or a job's code changes.
//
// Every value crosses as its encoding, the one encode writes, so the store
// holds the same bytes whichever runs the code. Declarations are named as
// the runner names them: one of the program's own module by its name
// ("Counter"), one of an imported module with the module's path in front
// ("lab.notify.Message"). Each call is given the runner as a NativeHost,
// for compiled code to call back into.
type Native interface {
	// HostsMain, HostsTest, HostsMethod and HostsJob say whether the
	// compiled program runs main, a test, an entity's method or a job.
	// What it doesn't run, the interpreter does.
	HostsMain() bool
	HostsTest(name string) bool
	HostsMethod(entity, method string) bool
	HostsJob(job string) bool
	// RunMain runs main with its arguments, a List(String), and returns
	// what it returns.
	RunMain(h NativeHost, args []byte) (result []byte, err error)
	// RunTest runs a test.
	RunTest(h NativeHost, name string) error
	// CallMethod runs an entity's method on its state, given and returned
	// encoded, with the method's arguments in the order of its parameters.
	CallMethod(h NativeHost, entity, method string, state []byte, args [][]byte) (newState, result []byte, err error)
	// StepJob runs a job to its next `!` step or to its end: from its
	// arguments when from is nil, and otherwise from the step it stopped
	// at, with resumed, what that step gave.
	StepJob(h NativeHost, job string, from *NativeStep, args [][]byte, resumed []byte) (NativeStep, error)
}

// NativeHost is the runner as compiled code calls back into it, inside the
// call it was given to: an entity call from a method joins the method's
// transaction. An error says why the callback stopped; the compiled code
// returns it on, and the runner then stops as the interpreter would have.
type NativeHost interface {
	// EntityCall calls an entity's method, and EntityRead reads an
	// entity's snapshot: its identity, its version and its state.
	EntityCall(entity, method string, id []byte, args [][]byte) ([]byte, error)
	EntityRead(entity string, id []byte) ([]byte, error)
	// Start starts a job, with a key when key isn't nil, and returns the run.
	Start(job string, args [][]byte, key []byte) ([]byte, error)
	// Publish publishes an event, named as a declaration is: "bus.counted".
	Publish(event string, value []byte) error
	// Call calls a function the runner provides, which doesn't wait: a
	// built-in such as print, expect or log.info, or a host function of the
	// standard library such as env.get, by the name a step records it as.
	Call(fn string, args [][]byte) ([]byte, error)
}

// NativeStep is where a native job stopped: at its end, with its result,
// or at a step for the runner to take, with the frame to resume from. A
// run saves it as its checkpoint: the run's Log holds only the steps taken
// since, which a run that wakes replays.
type NativeStep struct {
	Done   bool   `json:",omitempty"`
	Result []byte `json:",omitempty"`
	Frame  []byte `json:",omitempty"`
	// The step: a sleep for Sleep, a call of an entity's method, or a call
	// of a host function, such as env.get or http.get.
	Sleep  time.Duration `json:",omitempty"`
	Entity string        `json:",omitempty"`
	Method string        `json:",omitempty"`
	Host   string        `json:",omitempty"`
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

// nativeHost is the runner for one call into compiled code. What stops a
// callback, as a fault, a wait or a stop, is kept rather than unwinding
// through the compiled code's frames, which a Go panic must never do; the
// call raises it again once the compiled code has returned.
type nativeHost struct {
	in      *Interp
	at      ir.Pos
	stopped any
}

// native calls into compiled code with f, which is given the runner, and
// carries on what stopped a callback, or faults with what stopped the
// compiled code.
func (in *Interp) nativeCall(at ir.Pos, what string, f func(h NativeHost) error) {
	h := &nativeHost{in: in, at: at}
	err := f(h)
	if h.stopped != nil {
		panic(h.stopped)
	}
	if err != nil {
		in.faultf(at, "%s: %v", what, err)
	}
}

// guard runs a callback, keeping what stops it.
func (h *nativeHost) guard(f func() []byte) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			if h.stopped == nil {
				h.stopped = r
			}
			if f, ok := r.(*Fault); ok {
				err = fmt.Errorf("%s", f.Msg)
			} else {
				err = fmt.Errorf("stopped")
			}
		}
	}()
	return f(), nil
}

func (h *nativeHost) entity(name string) *ir.Entity {
	for _, e := range h.in.entities {
		if nativeName(e.Module, e.Name) == name {
			return e
		}
	}
	h.in.faultf(h.at, "the compiled program names the entity %s, which the program doesn't declare", name)
	return nil
}

func (h *nativeHost) values(args [][]byte) []argVal {
	var out []argVal
	for _, a := range args {
		out = append(out, argVal{v: h.in.decode(a, h.at)})
	}
	return out
}

func (h *nativeHost) EntityCall(entity, method string, id []byte, args [][]byte) ([]byte, error) {
	return h.guard(func() []byte {
		in := h.in
		e := h.entity(entity)
		m, ok := e.Methods[method]
		if !ok {
			in.faultf(h.at, "%s has no method %s", e.Name, method)
		}
		ref := &EntityRef{Entity: e, ID: in.decode(id, h.at)}
		return in.encode(in.rt.callEntity(ref, m, h.values(args), h.at), h.at)
	})
}

func (h *nativeHost) EntityRead(entity string, id []byte) ([]byte, error) {
	return h.guard(func() []byte {
		in := h.in
		ref := &EntityRef{Entity: h.entity(entity), ID: in.decode(id, h.at)}
		return in.encode(in.rt.snapshot(ref, h.at), h.at)
	})
}

func (h *nativeHost) Start(job string, args [][]byte, key []byte) ([]byte, error) {
	return h.guard(func() []byte {
		in := h.in
		var j *ir.Job
		for _, x := range in.jobs {
			if nativeName(x.Module, x.Name) == job {
				j = x
			}
		}
		if j == nil {
			in.faultf(h.at, "the compiled program starts the job %s, which the program doesn't declare", job)
		}
		var pos []Value
		for _, a := range h.values(args) {
			pos = append(pos, a.v)
		}
		var named map[string]Value
		if key != nil {
			named = map[string]Value{"key": in.decode(key, h.at)}
		}
		return in.encode(in.rt.start(j, pos, named, h.at), h.at)
	})
}

func (h *nativeHost) Publish(event string, value []byte) error {
	_, err := h.guard(func() []byte {
		in := h.in
		for _, m := range in.prog.Modules {
			for _, ct := range m.Contracts {
				for _, e := range ct.Events {
					if nativeName(m, e.Name) == event {
						in.rt.publish(e, in.decode(value, h.at))
						return nil
					}
				}
			}
		}
		in.faultf(h.at, "the compiled program publishes %s, which the program doesn't declare", event)
		return nil
	})
	return err
}

func (h *nativeHost) Call(fn string, args [][]byte) ([]byte, error) {
	return h.guard(func() []byte {
		in := h.in
		return in.encode(in.apply(in.hostFunc(fn, h.at), h.values(args), h.at), h.at)
	})
}

// hostFunc is the function the runner provides by the name compiled code
// calls it by: a built-in, or a function of the standard library that its
// host runs, by its host name ("http.get") or as module.name ("env.get").
func (in *Interp) hostFunc(name string, at ir.Pos) Value {
	if b, ok := in.builtins[name]; ok {
		return b
	}
	for _, m := range in.prog.Modules {
		if m.Dir != "" {
			continue // the program's own: compiled code calls those itself
		}
		for _, f := range m.Funcs {
			if f.Host == name || nativeName(m, f.Name) == name {
				return &Closure{Fn: f}
			}
		}
	}
	in.faultf(at, "the compiled program calls %s, which the runner doesn't provide", name)
	return nil
}

// nativeMain runs main natively.
func (in *Interp) nativeMain(main *ir.Func, argv []argVal) Value {
	args := in.encode(&List{}, main.Pos)
	if len(argv) > 0 {
		args = in.encode(argv[0].v, main.Pos)
	}
	var result []byte
	in.nativeCall(main.Pos, "main", func(h NativeHost) error {
		var err error
		result, err = in.native.RunMain(h, args)
		return err
	})
	return in.decode(result, main.Pos)
}

// nativeTest runs a test natively.
func (in *Interp) nativeTest(t *ir.Test) {
	in.nativeCall(t.Pos, "test "+t.Name, func(h NativeHost) error {
		return in.native.RunTest(h, t.Name)
	})
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
	var state, result []byte
	in.nativeCall(at, e.Name+"."+m.Name, func(h NativeHost) error {
		var err error
		state, result, err = in.native.CallMethod(h, nativeName(e.Module, e.Name), m.Name, in.encode(row.state, at), encoded)
		return err
	})
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
		var s NativeStep
		in.nativeCall(job.Pos, job.Name, func(h NativeHost) error {
			var err error
			s, err = in.native.StepJob(h, name, run.Native, encoded, resumed)
			return err
		})
		if s.Done {
			run.Native = nil
			return in.decode(s.Result, job.Pos)
		}
		// The new place replaces the run's checkpoint. The step's commit
		// saves it, so a crash before the step leaves the run where it was,
		// which gives the same step again. The steps since the last place
		// go to the run's trail, as a checkpoint's do, for eye runs.
		for _, st := range run.Log {
			if st.Seq != 0 {
				run.Since = st.Seq // next! looks at events after the last step
			}
		}
		run.Trail = append(run.Trail, run.Log...)
		if n := len(run.Trail); n > trailSize {
			run.Trail = append([]step(nil), run.Trail[n-trailSize:]...)
		}
		run.Native = &s
		run.Checkpoint, run.Log, run.cursor = nil, nil, 0
	}
}

// nativeStep takes the step a native job stopped at, or replays it.
func (in *Interp) nativeStep(run *jobRun, s *NativeStep, at ir.Pos) Value {
	switch {
	case s.Host != "":
		// As durable takes a step of a host function: recorded, so a run
		// that wakes gets what it gave the first time.
		callee := in.hostFunc(s.Host, at)
		in.call = s.Host
		if v, ok := in.replay(run, in.call, at); ok {
			return v
		}
		var args []argVal
		for _, a := range s.Args {
			args = append(args, argVal{v: in.decode(a, at)})
		}
		return in.record(run, in.apply(callee, args, at), at)
	case s.Entity == "":
		in.call = "sleep"
		if v, ok := in.replay(run, in.call, at); ok {
			return v
		}
		return in.sleep(run, in.rt.now.Add(s.Sleep))
	}
	h := &nativeHost{in: in, at: at}
	e := h.entity(s.Entity)
	meth, ok := e.Methods[s.Method]
	if !ok {
		in.faultf(at, "%s has no method %s", e.Name, s.Method)
	}
	ref := &EntityRef{Entity: e, ID: in.decode(s.ID, at)}
	in.call = methodName(ref, s.Method)
	if v, ok := in.replay(run, in.call, at); ok {
		return v
	}
	return in.entityStep(run, ref, meth, h.values(s.Args), at)
}
