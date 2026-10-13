package interp

import (
	"errors"
	"fmt"

	"eyelang/ir"
)

// Compiled code and the runner hand each other values as saved bytes
// (ROADMAP 7.6, 7.9): the C back end writes an encoder and a decoder for
// each type that crosses, to the bytes encode and decode use (cgen/save.go).
// These give the runner, and cgen's tests, the interpreter's side.

// Encode saves v as bytes, as entity state and a job's steps are saved.
func (in *Interp) Encode(v Value) ([]byte, error) { return tryEncode(in, v) }

// Decode reads bytes that Encode, or compiled code, saved.
func (in *Interp) Decode(data []byte) (Value, error) { return tryDecode(in, data) }

// Global is the value of the module-level let name of the program's root
// module.
func (in *Interp) Global(name string) (v Value, err error) {
	for _, g := range in.prog.Root.Globals {
		if g.Name != name {
			continue
		}
		defer func() {
			if r := recover(); r != nil {
				f, ok := r.(*Fault)
				if !ok {
					panic(r)
				}
				v, err = nil, f
			}
		}()
		return in.global(g, ir.Pos{}), nil
	}
	return nil, fmt.Errorf("the program has no let %s", name)
}

// HostCall answers host function name for compiled code (cgen's
// runtime/host.c): args is the saved record of the arguments the call
// gives, by parameter name, and the answer is the saved result. Arguments
// left out take their defaults, as in a call the interpreter makes. A fault
// in the function is an error with the fault's message.
func (in *Interp) HostCall(name string, args []byte) (result []byte, err error) {
	var fn *ir.Func
	for _, m := range in.prog.Modules {
		for _, f := range m.Funcs {
			if f.Host == name {
				fn = f
			}
		}
	}
	if fn == nil {
		return nil, fmt.Errorf("the program declares no host function %s", name)
	}
	v, err := in.Decode(args)
	if err != nil {
		return nil, err
	}
	rec, ok := v.(*Record)
	if !ok {
		return nil, fmt.Errorf("the arguments of %s aren't a record", name)
	}
	vals := make([]argVal, len(rec.Names))
	for i, n := range rec.Names {
		vals[i] = argVal{name: n, v: rec.Vals[i]}
	}
	defer func() {
		if r := recover(); r != nil {
			f, ok := r.(*Fault)
			if !ok {
				panic(r)
			}
			result, err = nil, errors.New(f.Msg)
		}
	}()
	return in.encode(in.callHost(fn, vals, fn.Pos), fn.Pos), nil
}
