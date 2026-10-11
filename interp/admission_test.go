package interp

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"eyelang/check"
	"eyelang/ir"
)

// What reserve! granted is kept in the store, so a run that holds memory
// across a restart still holds it after, and gives it back when it is done.
func TestReservationsSurviveARestart(t *testing.T) {
	dir := t.TempDir()
	src := `
job hog() {
  reserve!(800 MiB, fn {
    sleep!(1 h)
  })
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.eye"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	path := filepath.Join(dir, ".eye", "journal")
	open := func() (*Interp, *Journal) {
		j, err := OpenJournal(path)
		if err != nil {
			t.Fatal(err)
		}
		in := New(prog, io.Discard, io.Discard)
		in.rt.useStore(j)
		in.rt.restore()
		return in, j
	}

	in, j := open()
	in.rt.start(in.jobs[".hog"], nil, nil, ir.Pos{})
	if got := in.rt.held(); got != 800<<20 {
		t.Fatalf("held %d bytes after the grant, want 800 MiB", got)
	}
	j.Close()

	in, j = open()
	defer j.Close()
	if got := in.rt.held(); got != 800<<20 {
		t.Errorf("held %d bytes after the restart, want 800 MiB", got)
	}
	runs := Runs(prog, j)
	if len(runs) != 1 || runs[0].Holds != "800 MiB" {
		t.Errorf("eye runs shows %+v, want the run holding 800 MiB", runs)
	}
	in.rt.advance(ir.Duration(2 * 60 * 60 * 1e9))
	if got := in.rt.held(); got != 0 {
		t.Errorf("held %d bytes once the run finished, want none", got)
	}
	if _, ok := j.Meta(reservedKey(1)); ok {
		t.Error("the store still keeps the finished run's reservation")
	}
}
