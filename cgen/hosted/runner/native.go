//go:build eyehosted

// Package runner is the Go runner a program with entities and jobs is
// built into (compilation.md, "Hosting compiled programs (7.6)"): the
// interpreter's runtime, with the store, the scheduler, steps and their
// replay, running main, the tests, the entity methods and the jobs the
// compiled program hosts through the C entry points of
// cgen/hosted/eye_host.h, and answering its callbacks.
//
// It is built only into such a program, with cgo and the eyehosted tag;
// eye's own build, which has no cgo, never compiles it.
package runner

/*
#cgo CFLAGS: -I${SRCDIR}/..
#include <stdlib.h>
#include "eye_host.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"eyelang/interp"
)

// Program is a compiled program, as the runner calls it: it implements
// interp.Native.
type Program struct {
	entities    map[string]entity
	entityNames []string // by index
	jobs        map[string]job
	tests       map[string]int32
	hosts       []host
	events      []event
	hasMain     bool
	mainArgs    int32
	mainResult  int32
	keyType     int32
	runType     int32
}

type entity struct {
	index                   int32
	idType, state, snapshot int32
	methods                 map[string]method
	names                   []string // the methods' names, by index
}

type method struct {
	index  int32
	params []int32
	result int32
}

type job struct {
	index         int32
	params        []int32
	frame, result int32
}

type host struct {
	name   string
	params []int32
	result int32
}

type event struct {
	name string
	typ  int32
}

// Load reads the compiled program's tables (eye_program).
func Load() (*Program, error) {
	p := C.eye_program()
	if p == nil {
		return nil, errors.New("the compiled program has no eye_program")
	}
	if p.version != C.EYE_HOST_VERSION {
		return nil, fmt.Errorf("the compiled program is for version %d of the hosting interface, and this runner is for %d: build it with the same eye", p.version, C.EYE_HOST_VERSION)
	}
	prog := &Program{
		entities: map[string]entity{},
		jobs:     map[string]job{},
		tests:    map[string]int32{},
		hasMain:  p.has_main != 0, mainArgs: int32(p.main_args_type), mainResult: int32(p.main_result_type),
		keyType: int32(p.key_type), runType: int32(p.run_type),
	}
	for i, e := range unsafe.Slice(p.entities, int(p.nentities)) {
		ent := entity{index: int32(i), idType: int32(e.id_type), state: int32(e.state_type), snapshot: int32(e.snapshot_type), methods: map[string]method{}}
		for j, m := range unsafe.Slice(e.methods, int(e.nmethods)) {
			name := C.GoString(m.name)
			ent.methods[name] = method{index: int32(j), params: ints(m.param_types, m.nparams), result: int32(m.result_type)}
			ent.names = append(ent.names, name)
		}
		name := C.GoString(e.name)
		prog.entities[name] = ent
		prog.entityNames = append(prog.entityNames, name)
	}
	for i, j := range unsafe.Slice(p.jobs, int(p.njobs)) {
		prog.jobs[C.GoString(j.name)] = job{index: int32(i), params: ints(j.param_types, j.nparams), frame: int32(j.frame_type), result: int32(j.result_type)}
	}
	for i, t := range unsafe.Slice(p.tests, int(p.ntests)) {
		prog.tests[C.GoString(t)] = int32(i)
	}
	for _, h := range unsafe.Slice(p.hosts, int(p.nhosts)) {
		prog.hosts = append(prog.hosts, host{name: C.GoString(h.name), params: ints(h.param_types, h.nparams), result: int32(h.result_type)})
	}
	for _, e := range unsafe.Slice(p.events, int(p.nevents)) {
		prog.events = append(prog.events, event{name: C.GoString(e.name), typ: int32(e._type)})
	}
	return prog, nil
}

func ints(p *C.int32_t, n C.int32_t) []int32 {
	if n == 0 {
		return nil
	}
	out := make([]int32, n)
	for i, x := range unsafe.Slice(p, int(n)) {
		out[i] = int32(x)
	}
	return out
}

func (p *Program) HostsMain() bool              { return p.hasMain }
func (p *Program) HostsTest(name string) bool   { _, ok := p.tests[name]; return ok }
func (p *Program) HostsJob(name string) bool    { _, ok := p.jobs[name]; return ok }
func (p *Program) HostsMethod(e, m string) bool { _, ok := p.entities[e].methods[m]; return ok }
func (p *Program) entityByIndex(i int32) (string, entity, bool) {
	if i < 0 || int(i) >= len(p.entityNames) {
		return "", entity{}, false
	}
	name := p.entityNames[i]
	return name, p.entities[name], true
}

// ---- calls, and the runner each one is given

// A call is one call into the compiled program: the eye_rt it is given
// names it, so that a callback finds the runner it calls back into, nested
// calls included. err is what stopped a callback.
type call struct {
	p   *Program
	h   interp.NativeHost
	err error
}

var (
	callsMu sync.Mutex
	calls   = map[*C.eye_rt]*call{}
)

// enter calls f with a fresh eye_rt for h, and says why it stopped when it
// returns EYE_FAULT: a callback's error, or the compiled code's own.
func (p *Program) enter(h interp.NativeHost, f func(rt *C.eye_rt) C.eye_status) (C.eye_status, error) {
	rt := (*C.eye_rt)(C.malloc(1))
	c := &call{p: p, h: h}
	callsMu.Lock()
	calls[rt] = c
	callsMu.Unlock()
	defer func() {
		callsMu.Lock()
		delete(calls, rt)
		callsMu.Unlock()
		C.free(unsafe.Pointer(rt))
	}()
	status := f(rt)
	if status == C.EYE_FAULT {
		if c.err != nil {
			return status, c.err
		}
		return status, faultOf(rt)
	}
	return status, nil
}

func callOf(rt *C.eye_rt) *call {
	callsMu.Lock()
	defer callsMu.Unlock()
	return calls[rt]
}

func faultOf(rt *C.eye_rt) error {
	msg := C.eye_fault(rt)
	if msg == nil {
		return errors.New("the compiled program faulted")
	}
	return errors.New(C.GoString(msg))
}

// ---- the entry points

// RunMain runs main.
func (p *Program) RunMain(h interp.NativeHost, args []byte) ([]byte, error) {
	var out []byte
	_, err := p.enter(h, func(rt *C.eye_rt) C.eye_status {
		var vals handles
		defer func() { vals.release() }() // what is appended later too
		a, err := decode(rt, p.mainArgs, args)
		if err != nil {
			return fail(rt, err)
		}
		vals = append(vals, a)
		var res C.eye_val
		if s := C.eye_run_main(rt, a, &res); s != C.EYE_OK {
			return s
		}
		vals = append(vals, res)
		if out, err = encode(rt, p.mainResult, res); err != nil {
			return fail(rt, err)
		}
		return C.EYE_OK
	})
	return out, err
}

// RunTest runs a test.
func (p *Program) RunTest(h interp.NativeHost, name string) error {
	i, ok := p.tests[name]
	if !ok {
		return fmt.Errorf("the compiled program has no test %q", name)
	}
	_, err := p.enter(h, func(rt *C.eye_rt) C.eye_status {
		return C.eye_run_test(rt, C.int32_t(i))
	})
	return err
}

// CallMethod runs an entity's method on its state.
func (p *Program) CallMethod(h interp.NativeHost, entity, method string, state []byte, args [][]byte) (newState, result []byte, err error) {
	e := p.entities[entity]
	m, ok := e.methods[method]
	if !ok {
		return nil, nil, fmt.Errorf("the compiled program has no method %s.%s", entity, method)
	}
	_, err = p.enter(h, func(rt *C.eye_rt) C.eye_status {
		var vals handles
		defer func() { vals.release() }() // what is appended later too
		st, err := decode(rt, e.state, state)
		if err != nil {
			return fail(rt, err)
		}
		vals = append(vals, st)
		argv, err := decodeArgs(rt, &vals, m.params, args)
		if err != nil {
			return fail(rt, err)
		}
		defer C.free(argv)
		var res C.eye_val
		if s := C.eye_call_method(rt, C.int32_t(e.index), C.int32_t(m.index), &vals[0], (*C.eye_val)(argv), C.int32_t(len(args)), &res); s != C.EYE_OK {
			return s
		}
		vals = append(vals, res)
		if newState, err = encode(rt, e.state, vals[0]); err != nil {
			return fail(rt, err)
		}
		if result, err = encode(rt, m.result, res); err != nil {
			return fail(rt, err)
		}
		return C.EYE_OK
	})
	return newState, result, err
}

// StepJob runs a job to its next step or its end.
func (p *Program) StepJob(h interp.NativeHost, name string, from *interp.NativeStep, args [][]byte, resumed []byte) (interp.NativeStep, error) {
	j, ok := p.jobs[name]
	if !ok {
		return interp.NativeStep{}, fmt.Errorf("the compiled program has no job %s", name)
	}
	var out interp.NativeStep
	_, err := p.enter(h, func(rt *C.eye_rt) C.eye_status {
		var vals handles
		defer func() { vals.release() }() // what is appended later too
		var frame C.eye_val
		rs := C.eye_val(C.EYE_NONE)
		if from == nil {
			argv, err := decodeArgs(rt, &vals, j.params, args)
			if err != nil {
				return fail(rt, err)
			}
			s := C.eye_job_frame(rt, C.int32_t(j.index), (*C.eye_val)(argv), C.int32_t(len(args)), &frame)
			C.free(argv)
			if s != C.EYE_OK {
				return s
			}
		} else {
			f, err := decode(rt, j.frame, from.Frame)
			if err != nil {
				return fail(rt, err)
			}
			frame = f
			if resumed != nil {
				if rs, err = decode(rt, from.ResultType, resumed); err != nil {
					C.eye_release(frame)
					return fail(rt, err)
				}
				vals = append(vals, rs)
			}
		}
		var step C.eye_step
		var result C.eye_val
		s := C.eye_step_job(rt, C.int32_t(j.index), &frame, rs, &step, &result)
		// The frame owns the step's handles: it is released once they are read.
		defer C.eye_release(frame)
		var err error
		switch s {
		case C.EYE_OK:
			vals = append(vals, result)
			out.Done = true
			out.Result, err = encode(rt, j.result, result)
		case C.EYE_WAITING:
			out, err = p.step(rt, j, frame, &step)
		default:
			return s
		}
		if err != nil {
			return fail(rt, err)
		}
		return C.EYE_OK
	})
	return out, err
}

// step reads the step a job stopped at, with its frame.
func (p *Program) step(rt *C.eye_rt, j job, frame C.eye_val, s *C.eye_step) (interp.NativeStep, error) {
	f, err := encode(rt, j.frame, frame)
	if err != nil {
		return interp.NativeStep{}, err
	}
	out := interp.NativeStep{Frame: f, ResultType: int32(s.result_type)}
	args := unsafe.Slice(s.args, int(s.nargs))
	var params []int32
	switch s.kind {
	case C.EYE_STEP_SLEEP:
		out.Sleep = time.Duration(s.sleep_ns)
		return out, nil
	case C.EYE_STEP_CALL:
		name, e, ok := p.entityByIndex(int32(s.entity))
		if !ok || int(s.method) >= len(e.names) {
			return out, fmt.Errorf("the compiled program calls entity %d's method %d, which it doesn't declare", s.entity, s.method)
		}
		out.Entity, out.Method = name, e.names[s.method]
		params = e.methods[out.Method].params
		if out.ID, err = encode(rt, e.idType, s.id); err != nil {
			return out, err
		}
	case C.EYE_STEP_HOST:
		if int(s.fn) >= len(p.hosts) || s.fn < 0 {
			return out, fmt.Errorf("the compiled program calls host function %d, which it doesn't declare", s.fn)
		}
		out.Host, params = p.hosts[s.fn].name, p.hosts[s.fn].params
	default:
		return out, fmt.Errorf("the compiled program takes a step of kind %d, which this runner doesn't know", s.kind)
	}
	if out.Args, err = encodeArgs(rt, params, args); err != nil {
		return out, err
	}
	return out, nil
}

// ---- values

// handles are values the runner made for one call, released after it.
type handles []C.eye_val

func (h handles) release() {
	for _, v := range h {
		if v != C.EYE_NONE {
			C.eye_release(v)
		}
	}
}

// fail keeps err as why the call stopped, for enter to report.
func fail(rt *C.eye_rt, err error) C.eye_status {
	if c := callOf(rt); c != nil && c.err == nil {
		c.err = err
	}
	return C.EYE_FAULT
}

// decodeArgs makes the arguments' values, in a C array the caller frees.
func decodeArgs(rt *C.eye_rt, vals *handles, types []int32, args [][]byte) (unsafe.Pointer, error) {
	if len(args) != len(types) {
		return nil, fmt.Errorf("%d arguments for %d parameters", len(args), len(types))
	}
	argv := C.malloc(C.size_t(max(len(args), 1)) * C.size_t(unsafe.Sizeof(C.eye_val(0))))
	slots := unsafe.Slice((*C.eye_val)(argv), max(len(args), 1))
	for i, a := range args {
		v, err := decode(rt, types[i], a)
		if err != nil {
			C.free(argv)
			return nil, err
		}
		*vals = append(*vals, v)
		slots[i] = v
	}
	return argv, nil
}

// encodeArgs writes the encodings of values the compiled program owns.
func encodeArgs(rt *C.eye_rt, types []int32, args []C.eye_val) ([][]byte, error) {
	if len(args) != len(types) {
		return nil, fmt.Errorf("%d arguments for %d parameters", len(args), len(types))
	}
	var out [][]byte
	for i, a := range args {
		b, err := encode(rt, types[i], a)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

// decode makes the value an encoding holds. The bytes are copied into C's
// memory, which the call may keep pointers into for its length.
func decode(rt *C.eye_rt, typ int32, b []byte) (C.eye_val, error) {
	buf := C.CBytes(b)
	defer C.free(buf)
	var v C.eye_val
	if C.eye_decode(rt, C.int32_t(typ), C.eye_bytes{p: (*C.uint8_t)(buf), n: C.size_t(len(b))}, &v) != C.EYE_OK {
		return 0, faultOf(rt)
	}
	return v, nil
}

// encode writes a value's encoding.
func encode(rt *C.eye_rt, typ int32, v C.eye_val) ([]byte, error) {
	var out C.eye_buf
	if C.eye_encode(rt, C.int32_t(typ), v, &out) != C.EYE_OK {
		return nil, faultOf(rt)
	}
	defer C.free(unsafe.Pointer(out.p))
	return C.GoBytes(unsafe.Pointer(out.p), C.int(out.n)), nil
}
