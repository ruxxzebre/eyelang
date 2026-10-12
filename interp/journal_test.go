package interp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"eyelang/ir"
)

// failingSync is a journal file whose syncs fail while fail is set.
type failingSync struct {
	*os.File
	fail bool
}

func (f *failingSync) Sync() error {
	if f.fail {
		return syscall.EIO
	}
	return f.File.Sync()
}

// syncs is how many times j was synced.
func syncs(j *Journal) int64 {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.syncs
}

// counter is a program with a job that loops forever, a step and a sleep
// each time around, and a job that sleeps once. The journal in
// testdata/journal-2026-10-11 was written by the runtime of that day, which
// kept each run's record as base64 and in full in every commit, from this
// program: tick and once started with keys "a" and "b", then 5 min 30 s on
// the test clock.
const counter = `
entity Counter(id: String) {
  state n: Int = 0

  pub fn bump() -> Int {
    n += 1
    n
  }
}

job tick(id: String) {
  while true {
    Counter(id).bump!()
    sleep!(1 min)
  }
}

job once(id: String) {
  sleep!(1 h)
  Counter(id).bump!()
}
`

func counterValue(t *testing.T, in *Interp, id string) Value {
	t.Helper()
	state, _, _ := in.rt.load(&EntityRef{Entity: in.prog.Root.Entities["Counter"], ID: id}, ir.Pos{})
	n, _ := state.get("n")
	return n
}

// A journal written before run records were kept as JSON, with their whole
// trail in every commit, opens and runs on, and is written in the new form.
func TestOldJournalOpensAndRunsOn(t *testing.T) {
	old, err := os.ReadFile(filepath.Join("testdata", "journal-2026-10-11"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(old, []byte(`"Data":"eyJ`)) {
		t.Fatal("the test journal doesn't hold run records as base64")
	}
	path := filepath.Join(t.TempDir(), "journal")
	if err := os.WriteFile(path, old, 0o644); err != nil {
		t.Fatal(err)
	}
	j, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	prog := checkSource(t, counter)
	in := New(prog, io.Discard, io.Discard)
	in.rt.useStore(j)
	in.rt.restore()
	if n := counterValue(t, in, "a"); n != int64(6) {
		t.Fatalf("after reopening, counter a = %v, want 6", n)
	}
	if data, _ := j.Run(1); len(decodeRun(data).Trail) == 0 {
		t.Fatal("the looping run lost its trail")
	}

	in.rt.advance(ir.Duration(time.Hour))
	if n := counterValue(t, in, "a"); n != int64(61) {
		t.Errorf("an hour later, counter a = %v, want 61", n)
	}
	if n := counterValue(t, in, "b"); n != int64(1) {
		t.Errorf("an hour later, counter b = %v, want 1", n)
	}
	want, _ := j.Run(1)
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(`"Data":"eyJ`)) {
		t.Error("the journal still holds run records as base64")
	}
	if !bytes.Contains(data, []byte(`"Trail":{"Keep":`)) {
		t.Error("no commit wrote only the steps a trail gained")
	}
	mem, err := replay(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := mem.Run(1); !bytes.Equal(got, want) {
		t.Errorf("the replayed record differs from the one written:\n got %s\nwant %s", got, want)
	}
}

// sizes is a journal that also measures each commit as the runtime before
// 2026-10-12 wrote it: as JSON, with each run's record as base64, whole.
type sizes struct {
	*Journal
	commits, before int
}

func (s *sizes) Commit(c *Commit) {
	line, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	s.commits++
	s.before += len(line) + 1
	s.Journal.Commit(c)
}

// A looping job's commits are several times smaller than when each held
// the run's record as base64 with its whole trail: about 1.9 KB each
// before, about 0.45 KB once a trail that moved on was written as the
// steps it gained, and about 0.64 KB now that the commit that puts a run
// to sleep writes its record whole, to be read back from there once the
// record has left memory.
func TestALoopingJobsCommitsAreSmall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal")
	j, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	prog := checkSource(t, counter)
	in := New(prog, io.Discard, io.Discard)
	store := &sizes{Journal: j}
	in.rt.useStore(store)
	in.rt.startNow(prog.Root.Jobs["tick"], []Value{"a"}, map[string]Value{"key": "a"}, ir.Pos{})
	info, _ := os.Stat(path)
	start, commits, before := info.Size(), store.commits, store.before
	in.rt.advance(ir.Duration(2 * time.Hour))
	info, _ = os.Stat(path)
	n := store.commits - commits
	now := float64(info.Size()-start) / float64(n)
	was := float64(store.before-before) / float64(n)
	t.Logf("%d commits: %.0f bytes each, %.0f before", n, now, was)
	if n < 300 {
		t.Fatalf("%d commits in two hours, want at least 300", n)
	}
	if now*2.5 > was {
		t.Errorf("a commit takes %.0f bytes, and took %.0f: want it at least two and a half times smaller", now, was)
	}
}

// starter is a program whose main starts 5,000 keyed runs, each of which
// sleeps for an hour: the burst of commits eye run makes when a program
// starts its runs.
const starter = `
job sleeper(id: Int) {
  sleep!(1 h)
}

pub fn main(args: List(String)) uses {start sleeper, stdio} {
  for i in 0..5000 {
    sleeper.start(i, key: "{i}")
  }
  print("started")
}
`

// startOnJournal runs main of starter on a journal in dir, and returns how
// long it took and how many syncs it made.
func startOnJournal(tb testing.TB, dir string) (time.Duration, int64) {
	tb.Helper()
	prog := checkSource(tb, starter)
	j, err := OpenJournal(filepath.Join(dir, "journal"))
	if err != nil {
		tb.Fatal(err)
	}
	defer j.Close()
	opened := syncs(j)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // run main, then stop instead of waiting for the runs
	// What main prints comes after every commit before it is synced.
	out := writerFunc(func(p []byte) {
		j.mu.Lock()
		defer j.mu.Unlock()
		if j.synced != j.written {
			tb.Errorf("main printed with %d of %d commits synced", j.synced, j.written)
		}
	})
	start := time.Now()
	Run(ctx, prog, RunOptions{Stdout: out, Stderr: io.Discard, Store: j})
	took := time.Since(start)
	if n := len(j.Runs()); n != 5000 {
		tb.Fatalf("the journal holds %d runs, want 5000", n)
	}
	return took, syncs(j) - opened
}

type writerFunc func(p []byte)

func (w writerFunc) Write(p []byte) (int, error) {
	w(p)
	return len(p), nil
}

// The 15,000 commits of main starting 5,000 runs share a sync, made before
// main prints. Each synced alone, they took about 4 s.
func TestStartsShareASync(t *testing.T) {
	if testing.Short() {
		t.Skip("starts 5,000 runs")
	}
	took, n := startOnJournal(t, t.TempDir())
	t.Logf("5,000 starts: %v, %d syncs", took, n)
	// The journal compacts a few times as it grows; the starts add one.
	if n > 10 {
		t.Errorf("starting the runs took %d syncs, want them shared", n)
	}
}

// BenchmarkStartRunsOnJournal starts 5,000 runs from main on a journal:
// 3.9 s before commits shared their syncs, 0.36 s after.
func BenchmarkStartRunsOnJournal(b *testing.B) {
	for b.Loop() {
		startOnJournal(b, b.TempDir())
	}
}

// Callers that sync at the same time share one fsync.
func TestSyncsAtOnceShareOne(t *testing.T) {
	j, err := OpenJournal(filepath.Join(t.TempDir(), "journal"))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	j.syncLater()
	j.Commit(&Commit{Meta: []MetaWrite{{Key: "k", Data: []byte("1")}}})
	before := syncs(j)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if err := j.Sync(); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if n := syncs(j) - before; n != 1 {
		t.Errorf("8 callers made %d syncs, want 1", n)
	}
}

// A sync that fails leaves memory ahead of the disk: the journal refuses
// every commit after it, as fatal, until the program starts again.
func TestAFailedSyncStopsTheJournal(t *testing.T) {
	j, err := OpenJournal(filepath.Join(t.TempDir(), "journal"))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	j.syncLater()
	f := &failingSync{File: j.f.(*os.File)}
	j.f = f
	j.Commit(&Commit{Meta: []MetaWrite{{Key: "k", Data: []byte("1")}}})
	f.fail = true
	var ce *CommitError
	if err := j.Sync(); !errors.As(err, &ce) || !ce.Fatal || !errors.Is(err, syscall.EIO) {
		t.Fatalf("a failed sync gave %v, want a fatal CommitError", err)
	}
	err = guard(func() { j.Commit(&Commit{Meta: []MetaWrite{{Key: "k", Data: []byte("2")}}}) })
	if !errors.As(err, &ce) || !ce.Fatal {
		t.Errorf("a commit after a failed sync gave %v, want a fatal CommitError", err)
	}
}

// A crash can leave zeros where commits written after the last sync never
// reached the disk; replay stops there, since nothing outside saw them.
func TestReplayStopsAtZeros(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal")
	lines := `{"Meta":[{"Key":"a","Data":"MQ=="}]}` + "\n" +
		strings.Repeat("\x00", 40) + "\n" +
		`{"Meta":[{"Key":"b","Data":"Mg=="}]}` + "\n"
	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	mem, err := replay(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := mem.Meta("a"); !ok {
		t.Error("the commit before the zeros was lost")
	}
	if _, ok := mem.Meta("b"); ok {
		t.Error("the commit after the zeros was kept")
	}

	// Any other line that can't be read is still an error.
	bad := strings.Replace(lines, strings.Repeat("\x00", 40), "{oops", 1)
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := replay(path); err == nil || !strings.Contains(err.Error(), "journal:2") {
		t.Errorf("a broken line in the middle gave %v, want an error naming line 2", err)
	}
}

func TestCutMember(t *testing.T) {
	for _, c := range []struct{ obj, rest, val string }{
		{`{"A":1,"Trail":[2],"B":3}`, `{"A":1,"B":3}`, `[2]`},
		{`{"Trail":[2],"B":3}`, `{"B":3}`, `[2]`},
		{`{"A":1,"Trail":[2]}`, `{"A":1}`, `[2]`},
		{`{"Trail":[2]}`, `{}`, `[2]`},
		{`{ "A" : 1 , "Trail" : [ 2 ] }`, `{ "A" : 1 }`, `[ 2 ]`},
	} {
		rest, val, ok := cutMember([]byte(c.obj), "Trail")
		if !ok || string(rest) != c.rest || string(val) != c.val {
			t.Errorf("cutMember(%s) = %s, %s, %v; want %s, %s", c.obj, rest, val, ok, c.rest, c.val)
		}
	}
	if _, _, ok := cutMember([]byte(`{"A":{"Trail":1}}`), "Trail"); ok {
		t.Error("cutMember found a member inside another")
	}
}
