// Package clib gives the interpreter its C loader, so that a program's
// @library extern fns can be called (SPECS.md §18). Import it for its
// effect:
//
//	import _ "eyelang/host/clib"
//
// It loads libraries at run time without cgo (internal/ffi). On Linux a
// binary that imports it is linked dynamically, since loading a shared
// library needs the system's dynamic linker; that is why the interpreter
// doesn't import it itself, and why the released eye, a static binary on
// Linux, has it only when built with -tags clib.
package clib

import (
	"eyelang/internal/ffi"
	"eyelang/interp"
)

func init() { interp.UseCLoader(ffi.Load) }
