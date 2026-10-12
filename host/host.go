// Package host embeds Eyelang in a Go program, which hosts it. It loads and
// checks a program, provides the functions the program's @host extern fns
// name, and runs the program as `eye run` does, or its tests as `eye test`
// does (SPECS.md §18).
//
//	p, err := host.Load("desk")
//	if err != nil { … }
//	err = p.Register("lab.desk", func(text string) error { … })
//	code := p.Run(ctx, host.RunOptions{Stdout: os.Stdout, Stderr: os.Stderr})
//
// The program declares each host function with its Eyelang signature:
//
//	@host("lab.desk")
//	extern fn showOnDesk(text: String) -> Result((), String)
//
// and Register checks the Go function against it: see interp.Hosts.Register
// for which Go type holds which Eyelang type. A program's @library extern
// fns are C functions, which need the C loader: import eyelang/host/clib for
// it. On Linux that makes the binary dynamically linked.
package host

import (
	"context"
	"io"
	"os"
	"strings"

	"eyelang/check"
	"eyelang/interp"
	"eyelang/ir"
)

// Program is a checked Eyelang program, with the host functions registered
// for it.
type Program struct {
	prog  *ir.Program
	hosts *interp.Hosts
}

// CheckError is a program that doesn't check: each of Diagnostics is one
// error, as "file:line:column: message".
type CheckError struct {
	Diagnostics []string
}

func (e *CheckError) Error() string { return strings.Join(e.Diagnostics, "\n") }

// Load reads and checks the program in dir, with the modules it imports.
// A program that doesn't check is a *CheckError.
func Load(dir string) (*Program, error) {
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		e := &CheckError{}
		for _, d := range diags {
			e.Diagnostics = append(e.Diagnostics, d.String())
		}
		return nil, e
	}
	return &Program{prog: prog, hosts: interp.NewHosts(prog)}, nil
}

// Register provides fn for the program's extern fns declared @host(name).
// It returns an error, and registers nothing, when fn's Go signature
// doesn't match the declaration, or when no extern fn is declared with
// that name.
func (p *Program) Register(name string, fn any) error {
	return p.hosts.Register(name, fn)
}

// RunOptions say how Run runs the program.
type RunOptions struct {
	// Args are main's arguments; the first is the program's name, as the
	// directory is for `eye run`.
	Args []string
	// Stdout takes what the program prints and Stderr its logs and
	// errors; nil is os.Stdout and os.Stderr.
	Stdout, Stderr io.Writer
	// Store keeps entities, runs and events; nil keeps them in memory, so
	// nothing survives the process. interp.OpenJournal opens the file
	// `eye run` keeps them in.
	Store interp.Store
}

// Run runs the program as `eye run` does: main first, then its jobs and
// schedules on the system clock until nothing is left to happen or ctx is
// done. Before main starts it reports each @host extern fn nothing was
// registered for, and each @library one whose library or symbol can't be
// loaded, and returns 1. The result is the exit status `eye run` would
// give.
func (p *Program) Run(ctx context.Context, opts RunOptions) int {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	return interp.Run(ctx, p.prog, interp.RunOptions{
		Args: opts.Args, Stdout: opts.Stdout, Stderr: opts.Stderr, Store: opts.Store, Hosts: p.hosts,
	})
}

// TestResult is the outcome of one test block.
type TestResult = interp.TestResult

// Test runs the test blocks of the program's own modules whose names
// contain filter, as `eye test` does, each with a fresh in-memory store and
// a clock that moves only when the test advances it. A test that calls an
// extern fn whose code can't be found fails at the call.
func (p *Program) Test(filter string) []TestResult {
	return interp.RunTestsWith(p.prog, filter, p.hosts)
}
