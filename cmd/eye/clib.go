//go:build clib || darwin || windows

package main

// The C loader, for a program's @library extern fns (SPECS.md §18). On
// macOS and Windows it is always there. On Linux it makes eye a dynamically
// linked binary, so the release, which is static and runs on a `scratch`
// image, leaves it out; build with -tags clib for an eye that has it.
import _ "eyelang/host/clib"
