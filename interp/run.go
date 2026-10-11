package interp

import (
	"bytes"
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
	var results []TestResult
	for _, m := range prog.Modules {
		if m != prog.Root && m.Dir == "" {
			continue // the prelude and the standard modules
		}
		for _, t := range m.Tests {
			if !strings.Contains(t.Name, filter) {
				continue
			}
			r := runTest(prog, t)
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

func runTest(prog *ir.Program, t *ir.Test) (res TestResult) {
	var log bytes.Buffer
	in := New(prog, &log, &log)
	if newTestStore != nil {
		in.rt.useStore(newTestStore())
	}
	in.rt.module = t.Module // only the test's own module's schedules fire
	start := time.Now()
	res = TestResult{Name: t.Name, Pos: t.Pos}
	defer func() {
		res.Duration = time.Since(start)
		res.Log = log.String()
		if r := recover(); r != nil {
			f, ok := r.(*Fault)
			if !ok {
				panic(r)
			}
			res.Failure = f.Error()
		}
		if res.Failure == "" && len(in.rt.failures) > 0 {
			res.Failure = in.rt.failures[0].Error()
		}
	}()
	in.callFunc(t.Func, nil, nil, nil, t.Pos)
	return res
}
