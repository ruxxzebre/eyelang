package interp

import (
	"bytes"
	"fmt"
	"io"
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
	for _, t := range prog.Root.Tests {
		if !strings.Contains(t.Name, filter) {
			continue
		}
		results = append(results, runTest(prog, t))
	}
	return results
}

func runTest(prog *ir.Program, t *ir.Test) (res TestResult) {
	var log bytes.Buffer
	in := New(prog, &log, &log)
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

// RunMain runs the program's main function with args, and returns the exit
// status: 0, or 1 if main returned an Err or the program faulted.
func RunMain(prog *ir.Program, args []string, stdout, stderr io.Writer) (code int) {
	main := prog.Root.Main
	if main == nil {
		fmt.Fprintln(stderr, "the program has no main function: add `pub fn main(args: List(String))`")
		return 1
	}
	in := New(prog, stdout, stderr)
	in.rt.real = true
	in.rt.now = time.Now()
	defer func() {
		if r := recover(); r != nil {
			f, ok := r.(*Fault)
			if !ok {
				panic(r)
			}
			fmt.Fprintln(stderr, f.Error())
			code = 1
		}
	}()
	var argv []argVal
	if len(main.Params) > 0 {
		list := &List{}
		for _, a := range args {
			list.Elems = append(list.Elems, a)
		}
		argv = []argVal{{v: list}}
	}
	result, _, _ := in.callFunc(main, nil, nil, argv, main.Pos)
	if e, ok := isVariant(result, "Err"); ok {
		fmt.Fprintln(stderr, show(e.Vals[0]))
		return 1
	}
	for _, f := range in.rt.failures {
		fmt.Fprintln(stderr, f.Error())
		code = 1
	}
	return code
}
