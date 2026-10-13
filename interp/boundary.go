package interp

import (
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
