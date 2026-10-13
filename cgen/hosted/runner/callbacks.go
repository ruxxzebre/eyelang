//go:build eyehosted

package runner

/*
#cgo CFLAGS: -I${SRCDIR}/..
#include "eye_host.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

// The callbacks compiled code makes into the runner (eye_host.h). Each finds
// the call it is made in by its eye_rt, reads its arguments' encodings,
// calls the runner, and gives back the result as a value of the program's.
// What stops the runner, as a fault, is kept on the call, and the callback
// returns EYE_FAULT for the compiled code to return on.

// back runs a callback for the call rt names.
func back(rt *C.eye_rt, f func(c *call) error) C.eye_status {
	c := callOf(rt)
	if c == nil {
		return C.EYE_FAULT // a callback outside any call: nothing to call back into
	}
	if err := f(c); err != nil {
		if c.err == nil {
			c.err = err
		}
		return C.EYE_FAULT
	}
	return C.EYE_OK
}

func argList(args *C.eye_val, n C.int32_t) []C.eye_val {
	if n <= 0 {
		return nil
	}
	return unsafe.Slice(args, int(n))
}

//export eye_go_entity_call
func eye_go_entity_call(rt *C.eye_rt, entity C.int32_t, id C.eye_val, method C.int32_t, args *C.eye_val, nargs C.int32_t, result *C.eye_val) C.eye_status {
	return back(rt, func(c *call) error {
		name, e, ok := c.p.entityByIndex(int32(entity))
		if !ok || int(method) < 0 || int(method) >= len(e.names) {
			return fmt.Errorf("the compiled program calls entity %d's method %d, which it doesn't declare", entity, method)
		}
		m := e.methods[e.names[method]]
		idb, err := encode(rt, e.idType, id)
		if err != nil {
			return err
		}
		argb, err := encodeArgs(rt, m.params, argList(args, nargs))
		if err != nil {
			return err
		}
		out, err := c.h.EntityCall(name, e.names[method], idb, argb)
		if err != nil {
			return err
		}
		v, err := decode(rt, m.result, out)
		*result = v
		return err
	})
}

//export eye_go_entity_read
func eye_go_entity_read(rt *C.eye_rt, entity C.int32_t, id C.eye_val, snapshot *C.eye_val) C.eye_status {
	return back(rt, func(c *call) error {
		name, e, ok := c.p.entityByIndex(int32(entity))
		if !ok {
			return fmt.Errorf("the compiled program reads entity %d, which it doesn't declare", entity)
		}
		idb, err := encode(rt, e.idType, id)
		if err != nil {
			return err
		}
		out, err := c.h.EntityRead(name, idb)
		if err != nil {
			return err
		}
		v, err := decode(rt, e.snapshot, out)
		*snapshot = v
		return err
	})
}

//export eye_go_start
func eye_go_start(rt *C.eye_rt, index C.int32_t, args *C.eye_val, nargs C.int32_t, key C.eye_val, run *C.eye_val) C.eye_status {
	return back(rt, func(c *call) error {
		var name string
		var j job
		for n, x := range c.p.jobs {
			if x.index == int32(index) {
				name, j = n, x
			}
		}
		if name == "" {
			return fmt.Errorf("the compiled program starts job %d, which it doesn't declare", index)
		}
		argb, err := encodeArgs(rt, j.params, argList(args, nargs))
		if err != nil {
			return err
		}
		var keyb []byte
		if key != C.EYE_NONE {
			if keyb, err = encode(rt, c.p.keyType, key); err != nil {
				return err
			}
		}
		out, err := c.h.Start(name, argb, keyb)
		if err != nil {
			return err
		}
		v, err := decode(rt, c.p.runType, out)
		*run = v
		return err
	})
}

//export eye_go_publish
func eye_go_publish(rt *C.eye_rt, ev C.int32_t, value C.eye_val) C.eye_status {
	return back(rt, func(c *call) error {
		if int(ev) < 0 || int(ev) >= len(c.p.events) {
			return fmt.Errorf("the compiled program publishes event %d, which it doesn't declare", ev)
		}
		e := c.p.events[ev]
		b, err := encode(rt, e.typ, value)
		if err != nil {
			return err
		}
		return c.h.Publish(e.name, b)
	})
}

//export eye_go_host
func eye_go_host(rt *C.eye_rt, fn C.int32_t, args *C.eye_val, nargs C.int32_t, result *C.eye_val) C.eye_status {
	return back(rt, func(c *call) error {
		if int(fn) < 0 || int(fn) >= len(c.p.hosts) {
			return errors.New("the compiled program calls a host function it doesn't declare")
		}
		h := c.p.hosts[fn]
		argb, err := encodeArgs(rt, h.params, argList(args, nargs))
		if err != nil {
			return err
		}
		out, err := c.h.Call(h.name, argb)
		if err != nil {
			return err
		}
		v, err := decode(rt, h.result, out)
		*result = v
		return err
	})
}
