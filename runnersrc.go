// Package eyelang holds the source of the Go runner, which `eye build
// --runner` builds a program into: the interpreter's runtime with its stores,
// the checker that reads the program the binary carries, and what those need
// (compilation.md, "Hosting compiled programs"). It ties the runner a program
// links to the eye that compiled it. The modules they use are named by go.mod
// and go.sum, and fetched once into the user's cache.
package eyelang

import "embed"

// RunnerSource is the runner's source, tests and all: the build skips those.
//
//go:embed go.mod go.sum
//go:embed check/*.go check/prelude.eye check/runtime/*.eye check/std/*.eye
//go:embed ir/*.go ir/builtin.eye lir/*.go syntax/*.go interp/*.go
//go:embed internal/cron/*.go internal/wire/*.go
//go:embed cgen/hosted/*.go cgen/hosted/eye_host.h cgen/hosted/runner/*.go
var RunnerSource embed.FS
