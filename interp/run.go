package interp

import (
	"bytes"
	"os"
	"strings"
	"time"
	_ "time/tzdata" // zones such as Europe/Dublin, whatever the host has installed

	"eyelang/ir"
)

// TestResult is the outcome of one test block.
type TestResult struct {
	Name     string
	Pos      ir.Pos
	Failure  string // empty if the test passed
	Log      string // what the test and its jobs logged and printed
	Duration time.Duration
}

// RunTests runs the test blocks of the program's own module whose names
// contain filter. Each test gets a fresh store and a clock that only moves
// when the test advances it.
func RunTests(prog *ir.Program, filter string) []TestResult {
	return RunTestsWith(prog, filter, nil)
}

// RunTestsWith runs tests as RunTests does, with the host functions a Go
// program that embeds Eyelang registered. A test that calls an extern fn
// whose code can't be found fails at the call.
func RunTestsWith(prog *ir.Program, filter string, hosts *Hosts) []TestResult {
	var results []TestResult
	for _, m := range prog.Modules {
		if m != prog.Root && m.Dir == "" {
			continue // the prelude and the standard modules
		}
		for _, t := range m.Tests {
			if !strings.Contains(t.Name, filter) {
				continue
			}
			r := runTest(prog, t, hosts)
			if m != prog.Root {
				r.Name = m.Path + ": " + r.Name
			}
			results = append(results, r)
		}
	}
	return results
}

// newTestStore, when set, gives each test the store it runs on, so the
// runtime's own tests can run a program's tests on every kind of store.
var newTestStore func() Store

func runTest(prog *ir.Program, t *ir.Test, hosts *Hosts) (res TestResult) {
	var log bytes.Buffer
	in := New(prog, &log, &log)
	in.hosts = hosts
	if newTestStore != nil {
		in.rt.useStore(newTestStore())
	}
	in.rt.module = t.Module // only the test's own module's schedules fire
	in.rt.sandbox = true    // files in a directory of its own, random numbers from a fixed seed
	start := time.Now()
	res = TestResult{Name: t.Name, Pos: t.Pos}
	defer func() {
		if in.rt.dir != "" {
			os.RemoveAll(in.rt.dir)
		}
		res.Duration = time.Since(start)
		res.Log = log.String()
		if r := recover(); r != nil {
			f, ok := r.(*Fault)
			if !ok {
				panic(r)
			}
			res.Failure = f.Error()
		}
		// A job's run that stopped on a bug is usually why an expect after
		// it failed, so it comes first.
		if len(in.rt.failures) > 0 {
			first := in.rt.failures[0].Error()
			if res.Failure != "" && res.Failure != first {
				first += "\n" + res.Failure
			}
			res.Failure = first
		}
	}()
	in.callFunc(t.Func, nil, nil, nil, t.Pos)
	return res
}
