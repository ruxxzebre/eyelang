package interp

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"eyelang/ir"
)

func TestJournalSurvivesReopening(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".eye", "journal")
	j, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	j.Commit(&Commit{Writes: []Write{{Key: "m.E/1", Entity: "m.E", State: []byte("a"), Version: 1}}})
	j.Commit(&Commit{
		Writes: []Write{{Key: "m.E/1", Entity: "m.E", State: []byte("b"), Version: 2}},
		Runs:   []RunWrite{{ID: 1, Data: []byte(`{"ID":1}`)}},
		Events: []EventRecord{{Name: "m.ev", Data: []byte("x")}},
		Meta:   []MetaWrite{{Key: "fired/m.job", Data: []byte("7")}},
	})

	if _, err := OpenJournal(path); err == nil || !strings.Contains(err.Error(), "in use") {
		t.Errorf("a second writer opened the journal: %v", err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}

	// A crash in the middle of a commit leaves a line without its end; that
	// commit never counted.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"Writes":[{"Key":"m.E/1","Entity":"m.E","State":"Yw==","Ver`); err != nil {
		t.Fatal(err)
	}
	f.Close()

	again, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if r, _ := again.Load("m.E/1"); string(r.State) != "b" || r.Version != 2 {
		t.Errorf("entity = %q v%d, want \"b\" v2", r.State, r.Version)
	}
	if ids := again.Runs(); len(ids) != 1 || ids[0] != 1 {
		t.Errorf("runs = %v", ids)
	}
	if evs := again.Events(); len(evs) != 1 || evs[0].Name != "m.ev" {
		t.Errorf("events = %v", evs)
	}
	if d, ok := again.Meta("fired/m.job"); !ok || string(d) != "7" {
		t.Errorf("meta = %q, %v", d, ok)
	}
	// Opening rewrote the journal as one commit.
	data, _ := os.ReadFile(path)
	if lines := strings.Count(string(data), "\n"); lines != 1 {
		t.Errorf("the reopened journal has %d lines, want 1", lines)
	}
}

// fullDisk is a journal file whose writes fail, after writing part of what
// they were given, while full is set: as when the disk fills up.
type fullDisk struct {
	*os.File
	full bool
}

func (f *fullDisk) Write(p []byte) (int, error) {
	if f.full {
		n, _ := f.File.Write(p[:len(p)/2])
		return n, syscall.ENOSPC
	}
	return f.File.Write(p)
}

func TestJournalRefusesACommitItCantWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal")
	j, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if _, err := os.Stat(path + ".reserve"); err != nil {
		t.Fatalf("no reserve file: %v", err)
	}
	disk := &fullDisk{File: j.f.(*os.File)}
	j.f = disk
	j.Commit(&Commit{Writes: []Write{{Key: "m.E/1", Entity: "m.E", State: []byte("a"), Version: 1}}})

	disk.full = true
	err = guard(func() {
		j.Commit(&Commit{Writes: []Write{{Key: "m.E/1", Entity: "m.E", State: []byte("b"), Version: 2}}})
	})
	if err == nil || !errors.Is(err, syscall.ENOSPC) {
		t.Fatalf("commit on a full disk: %v, want ENOSPC", err)
	}
	if r, _ := j.Load("m.E/1"); string(r.State) != "a" || r.Version != 1 {
		t.Errorf("after the failed commit the entity is %q v%d, want \"a\" v1", r.State, r.Version)
	}
	if _, err := os.Stat(path + ".reserve"); err == nil {
		t.Error("the reserve wasn't released for compaction")
	}

	// Space comes back: the journal works, and has no trace of the failed
	// commit.
	disk = &fullDisk{File: j.f.(*os.File)}
	j.f = disk
	j.Commit(&Commit{Writes: []Write{{Key: "m.E/1", Entity: "m.E", State: []byte("c"), Version: 2}}})
	mem, err := replay(path)
	if err != nil {
		t.Fatal(err)
	}
	if r, _ := mem.Load("m.E/1"); string(r.State) != "c" || r.Version != 2 {
		t.Errorf("replayed entity = %q v%d, want \"c\" v2", r.State, r.Version)
	}
}

func TestJournalStaysAsLongAsItsState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal")
	j, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	state := []byte(strings.Repeat("x", 32<<10))
	for v := int64(1); v <= 200; v++ { // 6 MiB of history for 32 KiB of state
		j.Commit(&Commit{Writes: []Write{{Key: "m.E/1", Entity: "m.E", State: state, Version: v}}})
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > 2<<20 {
		t.Errorf("the journal is %d bytes for 32 KiB of state", info.Size())
	}
}

// failing is a store whose commits fail while full is set.
type failing struct {
	Store
	full bool
}

func (s *failing) Commit(c *Commit) {
	if s.full {
		panic(&CommitError{Path: "test", Err: syscall.ENOSPC})
	}
	s.Store.Commit(c)
}

func TestRunWaitsAtItsCheckpointWhileTheDiskIsFull(t *testing.T) {
	prog := load(t, `
entity Counter(id: Int) {
  state n: Int = 0
  pub fn bump() { n += 1 }
}

job count(id: Int) {
  sleep!(1 min)
  Counter(id).bump!()
}
`)
	store := &failing{Store: newMemStore()}
	in := New(prog, &strings.Builder{}, &strings.Builder{})
	in.rt.useStore(store)
	in.rt.startNow(prog.Root.Jobs["count"], []Value{int64(7)}, nil, ir.Pos{})
	ref := &EntityRef{Entity: prog.Root.Entities["Counter"], ID: int64(7)}
	status := func() string {
		data, _ := store.Run(1)
		return decodeRun(data).Status
	}

	store.full = true
	if err := guard(func() { in.rt.advance(ir.Duration(2 * time.Minute)) }); err == nil {
		t.Fatal("waking the run on a full disk didn't fail")
	}
	if status() != "sleeping" {
		t.Errorf("run is %s, want sleeping: still at its checkpoint", status())
	}
	if state, _, _ := in.rt.load(ref, ir.Pos{}); show(state) != "{ id: 7, n: 0 }" {
		t.Errorf("counter = %s, want untouched", show(state))
	}

	store.full = false
	in.rt.advance(0)
	if status() != "done" {
		t.Errorf("run is %s once there is space, want done", status())
	}
	if state, _, _ := in.rt.load(ref, ir.Pos{}); show(state) != "{ id: 7, n: 1 }" {
		t.Errorf("counter = %s, want bumped once", show(state))
	}
}

func TestPruneKeepsWhatCanStillBeUsed(t *testing.T) {
	prog := load(t, `
import bus

job await() -> Option(Int) {
  next!(bus.pings, where: fn p: p == 3, within: 1 day)
}
`, map[string]string{"bus/bus.eye": `
contract Bus {
  event pings: Int on "bus.pings"
}
`})
	var pings *ir.Event
	for _, m := range prog.Modules {
		if m.Path == "bus" {
			pings = m.Contracts["Bus"].Events["pings"]
		}
	}
	in := New(prog, &strings.Builder{}, &strings.Builder{})
	for i := 1; i <= 2; i++ {
		in.rt.publish(pings, int64(i))
	}
	in.rt.startNow(prog.Root.Jobs["await"], nil, nil, ir.Pos{}) // waits; sees events after 2
	for i := 4; i <= 5; i++ {
		in.rt.publish(pings, int64(i))
	}
	in.rt.prune()
	if in.rt.eventBase != 2 || len(in.rt.events) != 2 {
		t.Errorf("after pruning: base %d, %d events; want the 2 the waiting run can't see dropped", in.rt.eventBase, len(in.rt.events))
	}
	in.rt.publish(pings, int64(3))
	data, _ := in.rt.store.Run(1)
	if run := decodeRun(data); run.Status != "done" {
		t.Fatalf("the waiting run is %s after its event, want done", run.Status)
	}

	in.rt.now = in.rt.now.Add(8 * 24 * time.Hour)
	in.rt.prune()
	data, _ = in.rt.store.Run(1)
	if run := decodeRun(data); !run.Summarized || run.Log != nil || run.Steps != 1 {
		t.Errorf("a week after it finished the run keeps %d steps (summarized %v, %d counted)", len(run.Log), run.Summarized, run.Steps)
	}
	if len(in.rt.events) != 0 {
		t.Errorf("%d events kept with no run left to see them", len(in.rt.events))
	}
}

func TestALongRunIsReported(t *testing.T) {
	prog := load(t, `
job watch() {
  while true { sleep!(45 s) }
}
`)
	defer func(n int) { longRun = n }(longRun)
	longRun = 50
	var log strings.Builder
	in := New(prog, &log, &log)
	in.rt.startNow(prog.Root.Jobs["watch"], nil, nil, ir.Pos{})
	in.rt.advance(ir.Duration(time.Hour)) // 80 sleeps
	if n := strings.Count(log.String(), "warning: run #1 of .watch has taken 50 steps"); n != 1 {
		t.Errorf("warned %d times, want once:\n%s", n, log.String())
	}
}

func TestMissedFiringRunsOnceOnStart(t *testing.T) {
	prog := load(t, `
entity Seen(name: String) {
  state days: List(Date) = []
  pub fn add(d: Date) { days += [d] }
}

@on(daily(07:40, in: "Europe/Dublin"))
job morning(day: Date) {
  Seen("x").add!(day)
}
`)
	store := newMemStore()
	// The program last fired on 10 October, and was down until the 13th
	// at noon: three mornings were missed.
	last := time.Date(2026, 10, 10, 6, 40, 0, 0, time.UTC)
	store.Commit(&Commit{Meta: []MetaWrite{{Key: "fired/.morning", Data: []byte(itoa(last.UnixNano()))}}})
	in := New(prog, &strings.Builder{}, &strings.Builder{})
	in.rt.useStore(store)
	in.rt.now = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	in.rt.restore()
	in.rt.catchUp()
	state, _, _ := in.rt.load(&EntityRef{Entity: prog.Root.Entities["Seen"], ID: "x"}, ir.Pos{})
	if days, _ := state.get("days"); show(days) != "[2026-10-13]" {
		t.Errorf("days = %s, want only the latest missed morning, [2026-10-13]", show(days))
	}
	_, when := in.rt.nextFiring()
	if want := time.Date(2026, 10, 14, 6, 40, 0, 0, time.UTC); !when.Equal(want) {
		t.Errorf("next firing %v, want %v", when, want)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
