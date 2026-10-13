//go:build eyehosted

// Package runner is the Go runner a program with entities and jobs is
// built into (compilation.md, "Hosting compiled programs (7.6)"): the
// interpreter's runtime, with the store, the scheduler, steps and their
// replay, running the entity methods and jobs the compiled program hosts
// through the C entry points of cgen/hosted/eye_host.h.
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
	"time"
	"unsafe"

	"eyelang/interp"
)

// Program is a compiled program, as the runner calls it: it implements
// interp.Native.
type Program struct {
	rt       *C.eye_rt
	entities map[string]entity
	byIndex  []string // entity names by index
	jobs     map[string]job
}

type entity struct {
	index         int32
	idType, state int32
	methods       map[string]method
	names         []string // the methods' names, by index
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
		// The program never looks inside eye_rt; it only passes it back.
		rt:       (*C.eye_rt)(C.malloc(1)),
		entities: map[string]entity{},
		jobs:     map[string]job{},
	}
	for i, e := range unsafe.Slice(p.entities, int(p.nentities)) {
		ent := entity{index: int32(i), idType: int32(e.id_type), state: int32(e.state_type), methods: map[string]method{}}
		for j, m := range unsafe.Slice(e.methods, int(e.nmethods)) {
			mm := method{index: int32(j), params: ints(m.param_types, m.nparams), result: int32(m.result_type)}
			name := C.GoString(m.name)
			ent.methods[name] = mm
			ent.names = append(ent.names, name)
		}
		name := C.GoString(e.name)
		prog.entities[name] = ent
		prog.byIndex = append(prog.byIndex, name)
	}
	for i, j := range unsafe.Slice(p.jobs, int(p.njobs)) {
		prog.jobs[C.GoString(j.name)] = job{index: int32(i), params: ints(j.param_types, j.nparams), frame: int32(j.frame_type), result: int32(j.result_type)}
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

// HostsMethod reports whether the compiled program runs an entity's method.
func (p *Program) HostsMethod(entity, method string) bool {
	_, ok := p.entities[entity].methods[method]
	return ok
}

// HostsJob reports whether the compiled program runs a job.
func (p *Program) HostsJob(job string) bool {
	_, ok := p.jobs[job]
	return ok
}

// CallMethod runs an entity's method on its state.
func (p *Program) CallMethod(entity, method string, state []byte, args [][]byte) (newState, result []byte, err error) {
	e := p.entities[entity]
	m, ok := e.methods[method]
	if !ok {
		return nil, nil, fmt.Errorf("the compiled program has no method %s.%s", entity, method)
	}
	var vals handles
	defer vals.release()
	st, err := p.decode(e.state, state)
	if err != nil {
		return nil, nil, err
	}
	vals = append(vals, st)
	argv, err := p.decodeArgs(&vals, m.params, args)
	if err != nil {
		return nil, nil, err
	}
	defer C.free(argv)
	var res C.eye_val
	status := C.eye_call_method(p.rt, C.int32_t(e.index), C.int32_t(m.index), &vals[0], (*C.eye_val)(argv), C.int32_t(len(args)), &res)
	if status != C.EYE_OK {
		return nil, nil, p.fault()
	}
	vals = append(vals, res)
	if newState, err = p.encode(e.state, vals[0]); err != nil {
		return nil, nil, err
	}
	if result, err = p.encode(m.result, res); err != nil {
		return nil, nil, err
	}
	return newState, result, nil
}

// StepJob runs a job to its next step or its end.
func (p *Program) StepJob(name string, from *interp.NativeStep, args [][]byte, resumed []byte) (interp.NativeStep, error) {
	j, ok := p.jobs[name]
	if !ok {
		return interp.NativeStep{}, fmt.Errorf("the compiled program has no job %s", name)
	}
	var vals handles
	defer vals.release()
	var frame C.eye_val
	rs := C.eye_val(C.EYE_NONE)
	if from == nil {
		argv, err := p.decodeArgs(&vals, j.params, args)
		if err != nil {
			return interp.NativeStep{}, err
		}
		status := C.eye_job_frame(p.rt, C.int32_t(j.index), (*C.eye_val)(argv), C.int32_t(len(args)), &frame)
		C.free(argv)
		if status != C.EYE_OK {
			return interp.NativeStep{}, p.fault()
		}
	} else {
		f, err := p.decode(j.frame, from.Frame)
		if err != nil {
			return interp.NativeStep{}, err
		}
		frame = f
		if resumed != nil {
			if rs, err = p.decode(from.ResultType, resumed); err != nil {
				C.eye_release(frame)
				return interp.NativeStep{}, err
			}
			vals = append(vals, rs)
		}
	}
	var step C.eye_step
	var result C.eye_val
	status := C.eye_step_job(p.rt, C.int32_t(j.index), &frame, rs, &step, &result)
	// The frame is the program's again: it is released after the step's
	// handles, which it owns, are read.
	defer C.eye_release(frame)
	switch status {
	case C.EYE_OK:
		vals = append(vals, result)
		out, err := p.encode(j.result, result)
		return interp.NativeStep{Done: true, Result: out}, err
	case C.EYE_WAITING:
		return p.step(j, frame, &step)
	}
	return interp.NativeStep{}, p.fault()
}

// step reads the step a job stopped at, with its frame.
func (p *Program) step(j job, frame C.eye_val, s *C.eye_step) (interp.NativeStep, error) {
	f, err := p.encode(j.frame, frame)
	if err != nil {
		return interp.NativeStep{}, err
	}
	out := interp.NativeStep{Frame: f, ResultType: int32(s.result_type)}
	switch s.kind {
	case C.EYE_STEP_SLEEP:
		out.Sleep = time.Duration(s.sleep_ns)
		return out, nil
	case C.EYE_STEP_CALL:
		if int(s.entity) >= len(p.byIndex) {
			return out, fmt.Errorf("the compiled program calls entity %d, which it doesn't declare", s.entity)
		}
		name := p.byIndex[s.entity]
		e := p.entities[name]
		if int(s.method) >= len(e.names) {
			return out, fmt.Errorf("the compiled program calls method %d of %s, which it doesn't declare", s.method, name)
		}
		out.Entity, out.Method = name, e.names[s.method]
		m := e.methods[out.Method]
		if out.ID, err = p.encode(e.idType, s.id); err != nil {
			return out, err
		}
		args := unsafe.Slice(s.args, int(s.nargs))
		if len(args) != len(m.params) {
			return out, fmt.Errorf("%s.%s takes %d arguments, and the compiled program gives %d", name, out.Method, len(m.params), len(args))
		}
		for i, a := range args {
			b, err := p.encode(m.params[i], a)
			if err != nil {
				return out, err
			}
			out.Args = append(out.Args, b)
		}
		return out, nil
	}
	return out, fmt.Errorf("the compiled program takes a step of kind %d, which this runner doesn't take yet", s.kind)
}

// handles are values the runner made for one call, released after it.
type handles []C.eye_val

func (h handles) release() {
	for _, v := range h {
		if v != C.EYE_NONE {
			C.eye_release(v)
		}
	}
}

// decodeArgs makes the arguments' values, in a C array the caller frees.
func (p *Program) decodeArgs(vals *handles, types []int32, args [][]byte) (unsafe.Pointer, error) {
	if len(args) != len(types) {
		return nil, fmt.Errorf("%d arguments for %d parameters", len(args), len(types))
	}
	argv := C.malloc(C.size_t(max(len(args), 1)) * C.size_t(unsafe.Sizeof(C.eye_val(0))))
	slots := unsafe.Slice((*C.eye_val)(argv), max(len(args), 1))
	for i, a := range args {
		v, err := p.decode(types[i], a)
		if err != nil {
			C.free(argv)
			return nil, err
		}
		*vals = append(*vals, v)
		slots[i] = v
	}
	return argv, nil
}

// decode makes the value an encoding holds. The bytes are copied into C's
// memory, which the call may keep pointers into for its length.
func (p *Program) decode(typ int32, b []byte) (C.eye_val, error) {
	buf := C.CBytes(b)
	defer C.free(buf)
	var v C.eye_val
	if C.eye_decode(p.rt, C.int32_t(typ), C.eye_bytes{p: (*C.uint8_t)(buf), n: C.size_t(len(b))}, &v) != C.EYE_OK {
		return 0, p.fault()
	}
	return v, nil
}

// encode writes a value's encoding.
func (p *Program) encode(typ int32, v C.eye_val) ([]byte, error) {
	var out C.eye_buf
	if C.eye_encode(p.rt, C.int32_t(typ), v, &out) != C.EYE_OK {
		return nil, p.fault()
	}
	defer C.free(unsafe.Pointer(out.p))
	return C.GoBytes(unsafe.Pointer(out.p), C.int(out.n)), nil
}

func (p *Program) fault() error {
	msg := C.eye_fault(p.rt)
	if msg == nil {
		return errors.New("the compiled program faulted")
	}
	return errors.New(C.GoString(msg))
}
