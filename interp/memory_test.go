package interp

import (
	"io"
	"testing"

	"eyelang/check"
)

// TestStoreInEyelang runs the tests of the store written in Eyelang, and
// checks that the runtime keeps a program's state in it, and in a journal
// once the program runs.
func TestStoreInEyelang(t *testing.T) {
	prog, diags := check.Program("../testdata/programs/jobs")
	for _, d := range diags {
		t.Fatal(d)
	}
	if len(prog.Store.Tests) == 0 {
		t.Fatal("the store has no tests")
	}
	for _, test := range prog.Store.Tests {
		if r := runTest(prog, test); r.Failure != "" {
			t.Errorf("%s: %s", r.Name, r.Failure)
		}
	}
	in := New(prog, io.Discard, io.Discard)
	if _, ok := in.rt.index.Store.(*eyeStore); !ok {
		t.Errorf("the runtime's store is %T, want the store written in Eyelang", in.rt.index.Store)
	}

	j, err := OpenJournal(t.TempDir() + "/journal")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	j.Commit(&Commit{Writes: []Write{{Key: "x.E/1", Entity: "x.E", State: []byte("a"), Version: 1, Schema: 1}}})
	in.rt.useStore(j)
	if _, ok := j.memory.(*eyeStore); !ok {
		t.Errorf("the journal keeps its state in %T, want the store written in Eyelang", j.memory)
	}
	if r, ok := j.Load("x.E/1"); !ok || string(r.State) != "a" || r.Version != 1 || r.Schema != 1 {
		t.Errorf("after the move the row is %+v, %v", r, ok)
	}
}
