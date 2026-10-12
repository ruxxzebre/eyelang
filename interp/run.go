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
	Skipped  string // why the test was skipped, under eye test --live
	Log      string // what the test and its jobs logged and printed
	Duration time.Duration
}

// RunTests runs the test blocks of the program's own module whose names
// contain filter. Each test gets a fresh store and a clock that only moves
// when the test advances it.
func RunTests(prog *ir.Program, filter string) []TestResult {
	results, _, _ := RunTestsWith(prog, TestOptions{Filter: filter})
	return results
}

// RunTestsWith runs the program's tests as RunTests does, with the host
// functions opts.Hosts registers, and the calls of the live modules opts
// names answered by the real services. A test that calls an extern fn whose
// code can't be found fails at the call. With opts.Record it then writes
// what the live services answered in the tests that passed, and returns the
// files it wrote.
func RunTestsWith(prog *ir.Program, opts TestOptions) ([]TestResult, []Recorded, error) {
	var results []TestResult
	var rec *recorder
	if opts.Record && len(opts.Live) > 0 {
		rec = &recorder{secrets: map[string]string{}}
	}
	for _, m := range prog.Modules {
		if m != prog.Root && m.Dir == "" {
			continue // the prelude and the standard modules
		}
		for _, t := range m.Tests {
			if !strings.Contains(t.Name, opts.Filter) {
				continue
			}
			r := runTest(prog, t, opts, rec)
			if m != prog.Root {
				r.Name = m.Path + ": " + r.Name
			}
			results = append(results, r)
		}
	}
	if rec == nil {
		return results, nil, nil
	}
	written, err := rec.write()
	return results, written, err
}

// newTestStore, when set, gives each test the store it runs on, so the
// runtime's own tests can run a program's tests on every kind of store.
var newTestStore func() Store

func runTest(prog *ir.Program, t *ir.Test, opts TestOptions, rec *recorder) (res TestResult) {
	var log bytes.Buffer
	in := New(prog, &log, &log)
	in.hosts = opts.Hosts
	if newTestStore != nil {
		in.rt.useStore(newTestStore())
	}
	in.rt.module = t.Module // only the test's own module's schedules fire
	in.rt.sandbox = true    // files in a directory of its own, random numbers from a fixed seed
	in.rt.live = newLiveTest(opts)
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
		// A live test that couldn't reach a service it needs is skipped,
		// whatever failed after.
		if lt := in.rt.live; lt != nil && lt.skip != "" {
			res.Skipped, res.Failure = lt.skip, ""
		}
		if rec != nil && res.Failure == "" && res.Skipped == "" {
			rec.add(in.rt.live)
		}
	}()
	in.callFunc(t.Func, nil, nil, nil, t.Pos)
	return res
}
