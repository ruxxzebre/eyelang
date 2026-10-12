//go:build eyehosted

// Package hosted is the Go side of a program `eye build --runner` builds
// (compilation.md, "Hosting compiled programs"): the binary's main calls
// Main, which calls the program's C through cgo. eye embeds this source and
// writes it into the module it builds, as the package eyelang/cgen/hosted;
// the eyehosted tag keeps it out of eye's own build, which has no cgo.
//
// Today it runs the program's main, or its tests in a --test build. The
// runner's other entry points and the callbacks compiled code makes into
// it (entity calls, steps, host functions) come with 7.6.
package hosted

/*
#cgo LDFLAGS: ${SRCDIR}/libeyeprogram.a -lm
#include <stdlib.h>

// eye_main is the program's C main, renamed when it is compiled for the
// runner: it runs main, or the tests of a --test build, with these
// arguments, and returns the exit status.
int eye_main(int argc, char **argv);
*/
import "C"

import (
	"os"
	"unsafe"
)

// Main runs the program with the process's arguments and returns its exit
// status.
func Main() int {
	argv := make([]*C.char, len(os.Args)+1)
	for i, a := range os.Args {
		argv[i] = C.CString(a)
	}
	defer func() {
		for _, a := range argv[:len(os.Args)] {
			C.free(unsafe.Pointer(a))
		}
	}()
	return int(C.eye_main(C.int(len(os.Args)), &argv[0]))
}

// Exit ends the process as a C main returning would: through C's exit,
// which flushes the program's buffered output and runs what it registered
// with atexit. Go's os.Exit does neither.
func Exit(status int) {
	C.exit(C.int(status))
}
