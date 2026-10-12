package interp

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"
	"time"

	"eyelang/ir"
)

// A waiting run's record leaves memory (SPECS.md §15): the journal and the
// Postgres store keep only its handle in memory, read the record back when
// it wakes, and read no waiting record when they open again.

// reminders is a program whose runs each wait a day.
const reminders = `
job remind(id: Int) {
  sleep!(24 h)
}
`

// liveHeap is the bytes the heap holds once the garbage is collected.
func liveHeap() int64 {
	goruntime.GC()
	goruntime.GC()
	var m goruntime.MemStats
	goruntime.ReadMemStats(&m)
	return int64(m.HeapAlloc)
}

// startReminders starts n runs of remind, keyed, on store, and returns the
// interpreter and the bytes of heap each run added.
func startReminders(tb testing.TB, prog *ir.Program, store Store, n int) (*Interp, int64) {
	tb.Helper()
	in := New(prog, io.Discard, io.Discard)
	if store != nil {
		in.rt.useStore(store)
	}
	before := liveHeap()
	job := prog.Root.Jobs["remind"]
	for i := range n {
		in.rt.startNow(job, []Value{int64(i)}, map[string]Value{"key": int64(i)}, ir.Pos{})
	}
	if got := in.rt.index.waiting(); got != n {
		tb.Fatalf("%d runs wait, want %d", got, n)
	}
	return in, (liveHeap() - before) / int64(n)
}

// waitingRuns is how many waiting runs the memory test starts.
func waitingRuns() int {
	if testing.Short() {
		return 2_000
	}
	return 20_000
}

// 20,000 runs that each sleep for a day held about 690 bytes of heap each on
// the journal while it kept every record in memory. They hold about 510
// bytes each on the in-memory store, which still does, and about 135 bytes
// on the journal, which keeps only their handles. Opening the journal again
// reads none of their records; each is read once when its run wakes.
func TestWaitingRunsLeaveMemory(t *testing.T) {
	n := waitingRuns()
	prog := checkSource(t, reminders)
	_, inMemory := startReminders(t, prog, nil, n)

	path := filepath.Join(t.TempDir(), "journal")
	j, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	_, onJournal := startReminders(t, prog, j, n)
	t.Logf("%d waiting runs: %d B of heap each with their records in memory, %d B on the journal", n, inMemory, onJournal)
	if onJournal*3 > inMemory || onJournal > 200 {
		t.Errorf("a waiting run takes %d B on the journal and %d B in memory: want under 200 B, and a third", onJournal, inMemory)
	}
	if held := j.memory.Runs(); len(held) != 0 {
		t.Errorf("the journal holds %d records in memory, want none", len(held))
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	again, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	in := New(prog, io.Discard, io.Discard)
	in.rt.useStore(again)
	if err := in.rt.checkResumePoints(); err != nil {
		t.Fatal(err)
	}
	in.rt.restore()
	t.Logf("reopening the journal with %d waiting runs took %v", n, time.Since(start))
	if again.loads != 0 {
		t.Errorf("reopening read %d waiting runs' records, want none", again.loads)
	}
	if held := again.memory.Runs(); len(held) != 0 {
		t.Errorf("the reopened journal holds %d records in memory, want none", len(held))
	}
	if got := in.rt.index.waiting(); got != n {
		t.Fatalf("%d runs wait after reopening, want %d", got, n)
	}

	in.rt.advance(ir.Duration(25 * time.Hour))
	if again.loads != int64(n) {
		t.Errorf("waking %d runs read %d records, want each once", n, again.loads)
	}
	if got := in.rt.index.waiting(); got != 0 {
		t.Errorf("%d runs still wait", got)
	}
	if held := again.memory.Runs(); len(held) != 0 {
		t.Errorf("the finished runs left %d records in memory, want none", len(held))
	}
	data, ok := again.Run(int64(n / 2))
	if !ok || decodeRun(data).Status != "done" {
		t.Errorf("run %d reads back as %s", n/2, data)
	}
}

// An event wakes only the runs waiting for it, and reads only their
// records.
func TestAnEventReadsOnlyTheRecordsItWakes(t *testing.T) {
	prog := load(t, `
import bus

job remind(id: Int) {
  sleep!(24 h)
}

job await(id: Int) -> Option(Int) {
  next!(bus.pings, where: fn p: p == id, within: 2 days)
}
`, map[string]string{"bus/bus.eye": `
contract Bus {
  event pings: Int on "bus.pings"
}
`})
	j, err := OpenJournal(filepath.Join(t.TempDir(), "journal"))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	in := New(prog, io.Discard, io.Discard)
	in.rt.useStore(j)
	for i := range 100 {
		in.rt.startNow(prog.Root.Jobs["remind"], []Value{int64(i)}, nil, ir.Pos{})
	}
	for i := range 3 {
		in.rt.startNow(prog.Root.Jobs["await"], []Value{int64(i)}, nil, ir.Pos{})
	}
	var pings *ir.Event
	for _, m := range prog.Modules {
		if m.Path == "bus" {
			pings = m.Contracts["Bus"].Events["pings"]
		}
	}
	before := j.loads
	in.rt.publish(pings, int64(1))
	if got := j.loads - before; got != 3 {
		t.Errorf("the event read %d records, want the 3 of the runs it woke", got)
	}
	if got := in.rt.index.waiting(); got != 102 {
		t.Errorf("%d runs wait, want the 100 that sleep and the 2 the event isn't for", got)
	}
}

// A journal written before records left memory, whose sleeping runs were
// written as the steps their trails gained, opens: the records come
// together, and opening it moves them out of memory.
func TestJournalWithTrailStepsOpens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal")
	lines := `{"Runs":[{"ID":1,"Data":{"ID":1,"Job":".j","Status":"ready","Trail":[{"Call":"a"}]}}]}
{"Runs":[{"ID":1,"Data":{"ID":1,"Job":".j","Status":"sleeping","WakeAt":5},"Trail":{"Keep":1,"Add":[{"Call":"b"}]}}]}
`
	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	j, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	data, ok := j.Run(1)
	if !ok {
		t.Fatal("run 1 is gone")
	}
	if r := decodeRun(data); r.Status != "sleeping" || len(r.Trail) != 2 || r.Trail[1].Call != "b" {
		t.Errorf("run 1 reads back as %s", data)
	}
	if held := j.memory.Runs(); len(held) != 0 {
		t.Errorf("the journal holds %d records in memory, want none", len(held))
	}
	if sums := j.takeSummaries(); sums[1].Status != "sleeping" || sums[1].WakeAt != 5 {
		t.Errorf("run 1's summary is %+v", sums[1])
	}
}

// On Postgres too, a waiting run keeps only its handle in memory, and
// opening the store again reads no waiting run's record.
func TestWaitingRunsLeaveMemoryOnPostgres(t *testing.T) {
	url := testPostgres(t)
	n := 2_000 // each start is a commit, a round trip to the database
	prog := checkSource(t, reminders)
	p := openPostgres(t, url)
	_, perRun := startReminders(t, prog, p, n)
	t.Logf("%d waiting runs: %d B of heap each on Postgres", n, perRun)
	if perRun > 200 {
		t.Errorf("a waiting run takes %d B on Postgres, want under 200 B", perRun)
	}
	if held := p.memory.Runs(); len(held) != 0 {
		t.Errorf("the store holds %d records in memory, want none", len(held))
	}
	p.Close()

	start := time.Now()
	again := openPostgres(t, url)
	in := New(prog, io.Discard, io.Discard)
	in.rt.useStore(again)
	if err := in.rt.checkResumePoints(); err != nil {
		t.Fatal(err)
	}
	in.rt.restore()
	t.Logf("reopening the store with %d waiting runs took %v", n, time.Since(start))
	if again.loads != 0 {
		t.Errorf("reopening read %d waiting runs' records, want none", again.loads)
	}
	if held := again.memory.Runs(); len(held) != 0 {
		t.Errorf("the reopened store holds %d records in memory, want none", len(held))
	}
	if got := in.rt.index.waiting(); got != n {
		t.Fatalf("%d runs wait after reopening, want %d", got, n)
	}
	in.rt.advance(ir.Duration(25 * time.Hour))
	if again.loads != int64(n) {
		t.Errorf("waking %d runs read %d records, want each once", n, again.loads)
	}
	if got := in.rt.index.waiting(); got != 0 {
		t.Errorf("%d runs still wait", got)
	}
}

// A run's summary says where its checkpoint resumes as resumable reads it,
// counting the variables without decoding them.
func TestSummaryOfARecord(t *testing.T) {
	for in, want := range map[string]elements{`null`: 0, `[]`: 0, `[ ]`: 0, `["a,b","c\"]"]`: 2, `[null,"x",{"a":[1,2]}]`: 3} {
		var n elements
		if err := n.UnmarshalJSON([]byte(in)); err != nil || n != want {
			t.Errorf("%s has %d elements, want %d", in, n, want)
		}
	}
	run := &jobRun{ID: 3, Job: ".j", Status: "sleeping", WakeAt: 9, WaitEvent: "m.ev", Key: "k", Since: 2,
		Checkpoint: &checkpoint{Path: []pathElem{{Kind: 'b', I: 2}, {Kind: 'f', I: 7, Elems: []byte("x")}, {Kind: 'm', I: 1}},
			Slots: [][]byte{[]byte("a"), nil, []byte(`"q,"`)}, Code: "abc"},
		Log: []step{{Seq: 4}, {Event: "m.ev", Since: 4, Seq: 5}}}
	sum, ok := summaryOf(run.encode())
	if !ok {
		t.Fatal("the record can't be summarized")
	}
	if want := summaryOfRun(run); sum != want {
		t.Errorf("summary %+v, want %+v", sum, want)
	}
	if sum.Resume != "3 b2.f.m1. abc" || sum.Sees != 4 {
		t.Errorf("resume point %q, sees %d", sum.Resume, sum.Sees)
	}
	ck, ok := resumePointCheckpoint(sum.Resume)
	if !ok || len(ck.Slots) != 3 || ck.Code != "abc" || len(ck.Path) != 3 || ck.Path[0].Kind != 'b' || ck.Path[0].I != 2 || ck.Path[1].Kind != 'f' {
		t.Errorf("the resume point reads back as %+v", ck)
	}
}

// A Postgres store written before runs had summaries opens: opening it works
// each run's summary out once, and then reads only the ready runs' records.
// A record the database can't give back is a *CommitError the runtime tries
// again, and the next read connects again.
func TestPostgresStoreFromBeforeSummariesOpens(t *testing.T) {
	url := testPostgres(t)
	prog := checkSource(t, reminders)
	p := openPostgres(t, url)
	startReminders(t, prog, p, 20)
	ctx := context.Background()
	if _, err := p.conn.Exec(ctx, `ALTER TABLE eye_runs DROP COLUMN summary, DROP COLUMN ready; UPDATE eye_store SET schema = 2`); err != nil {
		t.Fatal(err)
	}
	p.Close()

	again := openPostgres(t, url)
	in := New(prog, io.Discard, io.Discard)
	in.rt.useStore(again)
	in.rt.restore()
	if again.loads != 0 {
		t.Errorf("opening read %d waiting runs' records, want none", again.loads)
	}
	if got := in.rt.index.waiting(); got != 20 {
		t.Fatalf("%d runs wait, want 20", got)
	}
	var version int
	if err := again.conn.QueryRow(ctx, `SELECT schema FROM eye_store`).Scan(&version); err != nil || version != pgSchema {
		t.Errorf("the tables are at version %d (%v), want %d", version, err, pgSchema)
	}

	again.conn.Close(ctx) // as when the database goes away
	err := guard(func() { again.Run(5) })
	var ce *CommitError
	if !errors.As(err, &ce) || !ce.Read || ce.Fatal {
		t.Fatalf("reading a record with no connection: %v, want a read the runtime tries again", err)
	}
	if data, ok := again.Run(5); !ok || decodeRun(data).Status != "sleeping" {
		t.Errorf("run 5 reads back as %s", data)
	}
}
