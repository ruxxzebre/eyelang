package interp

import (
	"bytes"
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"eyelang/ir"
)

// Process fakes answer process.run from recordings, so that a test of a
// program that runs other programs gives the same result on every machine:
// `with fakes(process, from: "recordings/")` reads recordings/process.json.
// Each exchange is one command line, its "key": the program and its
// arguments, separated by spaces, an argument that is empty or holds a
// space, a quote or a backslash written as a quoted string. Its result is
// what process.run returns, in the JSON shape every value travels in:
//
//	{
//	  "contract": "process",
//	  "exchanges": [
//	    { "op": "run", "key": "git pull --ff-only",
//	      "result": { "kind": "Ok", "value": { "status": 0, "stdout": "Already up to date.\n", "stderr": "" } } },
//	    { "op": "run", "key": "deploy \"two words\"",
//	      "result": { "kind": "Err", "value": { "kind": "NotStarted", "program": "deploy", "reason": "no such program" } } }
//	  ]
//	}
//
// An exchange without a key answers every command that no other names. A
// command that none answers fails the test. The commands answered are kept,
// for fakes.calls(process.run).
type processFakes struct {
	path    string
	answers map[string]Value // by command line; "" answers the commands no exchange names
	ran     []*Record        // the calls answered, oldest first: program, args, input, dir
}

// loadProcessFakes reads the recorded answers to process.run at path, each
// checked against its result type.
func (in *Interp) loadProcessFakes(path string, at ir.Pos) *processFakes {
	data, err := os.ReadFile(path)
	if err != nil {
		in.faultf(at, "no recordings for process: %v", err)
	}
	var rec recording
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&rec); err != nil {
		in.faultf(at, "%s: %v", path, err)
	}
	if rec.Contract != "process" {
		in.faultf(at, `%s records %q; recordings for process say "contract": "process"`, path, rec.Contract)
	}
	run := in.stdModule("process", at).Funcs["run"]
	f := &processFakes{path: path, answers: map[string]Value{}}
	for i, e := range rec.Exchanges {
		if e.Op != "run" {
			in.faultf(at, "%s: exchange %d: process has no operation %q to fake, only run", path, i+1, e.Op)
		}
		if _, dup := f.answers[e.Key]; dup {
			in.faultf(at, "%s: exchange %d: %s is answered twice", path, i+1, describeCommand(e.Key))
		}
		v, err := in.recorded(e.Result, run.Return)
		if err != nil {
			in.faultf(at, "%s: exchange %d (%s): %v", path, i+1, describeCommand(e.Key), err)
		}
		f.answers[e.Key] = v
	}
	return f
}

// fakeRun answers process.run from the test's recordings, if it has any.
func (in *Interp) fakeRun(params map[string]Value, program string, args []string, at ir.Pos) (Value, bool) {
	f := in.rt.processFakes
	if f == nil {
		return nil, false
	}
	line := commandLine(program, args)
	v, ok := f.answers[line]
	if !ok {
		v, ok = f.answers[""]
	}
	if !ok {
		in.faultf(at, "%s has no exchange for process.run of %s: add one with \"key\": %s", f.path, line, strconv.Quote(line))
	}
	call := &Record{}
	for _, name := range []string{"program", "args", "input", "dir"} {
		call.Names, call.Vals = append(call.Names, name), append(call.Vals, params[name])
	}
	f.ran = append(f.ran, call)
	return v, true
}

// commandLine writes a command as a recording's key names it.
func commandLine(program string, args []string) string {
	parts := []string{program}
	for _, a := range args {
		if a == "" || strings.ContainsAny(a, " \t\n\"\\") {
			a = strconv.Quote(a)
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, " ")
}

func describeCommand(key string) string {
	if key == "" {
		return "every other command"
	}
	return strconv.Quote(key)
}

// ranCalls is fakes.calls(process.run): the commands the fakes answered.
func (h *fakesHandler) ranCalls(in *Interp, at ir.Pos) Value {
	if h.process == nil {
		in.faultf(at, "these fakes don't answer for process, so they saw no run: write fakes(process, from: …)")
	}
	list := &List{}
	for _, r := range h.process.ran {
		list.Elems = append(list.Elems, r)
	}
	return list
}
