package interp

import (
	"bytes"
	"context"
	goruntime "runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"eyelang/internal/pgtest"
)

// lockedBuffer is a buffer that Run writes to while a test reads it.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

// waitFor, docker and ownPostgres are pgtest's, under the names these tests
// use.
func waitFor(t *testing.T, limit time.Duration, what string, cond func() bool) {
	t.Helper()
	pgtest.WaitFor(t, limit, what, cond)
}

func docker(t *testing.T, args ...string) string {
	t.Helper()
	return pgtest.Docker(t, args...)
}

func ownPostgres(t *testing.T) (url, container string) {
	t.Helper()
	return pgtest.Own(t)
}

// A job runs once and finishes when the database goes away in the middle of
// it, whether it stops or stops answering: its commits fail and are tried
// again, and the runner carries on once the database is back, without a
// restart of its own (task 5.3).
func TestRunSurvivesTheDatabaseGoingAway(t *testing.T) {
	base, container := ownPostgres(t)
	defer func(r, p time.Duration) { retryAfter, pgTimeout = r, p }(retryAfter, pgTimeout)
	retryAfter, pgTimeout = 300*time.Millisecond, 2*time.Second

	for _, c := range []struct{ name, down, up string }{
		{"restart", "stop", "start"},  // the connection breaks
		{"pause", "pause", "unpause"}, // the database stops answering
	} {
		t.Run(c.name, func(t *testing.T) {
			prog := load(t, `
entity Counter(id: Int) {
  state n: Int = 0
  pub fn bump() { n += 1 }
}

job work() -> Int {
  Counter(1).bump!()
  sleep!(2 s)
  Counter(1).bump!()
  sleep!(500 ms)
  Counter(1).read!().n
}

pub fn main() uses {start work} {
  work.start(key: "work")
}
`)
			url := newSchema(t, base)
			p := openPostgres(t, url)
			status := func() string {
				s, err := ReadPostgres(url)
				if err != nil {
					return ""
				}
				if data, ok := s.Run(1); ok {
					return decodeRun(data).Status
				}
				return ""
			}
			var log lockedBuffer
			done := make(chan int, 1)
			go func() { done <- Run(context.Background(), prog, RunOptions{Stdout: &log, Stderr: &log, Store: p}) }()
			waitFor(t, 5*time.Second, "the run sleeps", func() bool { return status() == "sleeping" })

			docker(t, c.down, container)
			upped := false
			defer func() {
				if !upped {
					docker(t, c.up, container)
				}
			}()
			// The run wakes while the database is away, and its commit fails.
			waitFor(t, 15*time.Second, "a commit fails", func() bool { return strings.Contains(log.String(), "trying again") })
			docker(t, c.up, container)
			upped = true

			select {
			case code := <-done:
				if code != 0 {
					t.Fatalf("eye run exited %d:\n%s", code, log.String())
				}
			case <-time.After(60 * time.Second):
				t.Fatalf("the run didn't finish once the database was back:\n%s", log.String())
			}
			p.Close()
			s, err := ReadPostgres(url)
			if err != nil {
				t.Fatal(err)
			}
			runs := Runs(prog, s)
			if len(runs) != 1 || runs[0].Status != "done" || runs[0].Result != "2" {
				t.Errorf("runs %+v, want one, done with 2: each bump once\n%s", runs, log.String())
			}
		})
	}
}

// watched is a store that notes each run's status as it is committed, for a
// test to read while Run runs.
type watched struct {
	Store
	mu     sync.Mutex
	status map[int64]string
}

func (s *watched) Commit(c *Commit) {
	s.Store.Commit(c)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range c.Runs {
		s.status[r.ID] = decodeRun(r.Data).Status
	}
}

func (s *watched) count(status string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, st := range s.status {
		if st == status {
			n++
		}
	}
	return n
}

// While runs wait, the runner holds only their saved bytes: no goroutine,
// no interpreter state (SPECS.md §15: a job holds no memory while paused).
func TestWaitingRunsHoldNoGoroutines(t *testing.T) {
	prog := load(t, `
job nap(i: Int) {
  sleep!(1 h)
}

pub fn main() uses {start nap} {
  for i in 1..=1000 { nap.start(i, key: "nap-{i}") }
}
`)
	store := &watched{Store: newMemStore(), status: map[int64]string{}}
	goruntime.GC()
	before := goruntime.NumGoroutine()
	var heapBefore goruntime.MemStats
	goruntime.ReadMemStats(&heapBefore)

	ctx, cancel := context.WithCancel(context.Background())
	var log lockedBuffer
	done := make(chan int, 1)
	go func() { done <- Run(ctx, prog, RunOptions{Stdout: &log, Stderr: &log, Store: store}) }()
	waitFor(t, 30*time.Second, "a thousand runs sleep", func() bool { return store.count("sleeping") == 1000 })

	goruntime.GC()
	during := goruntime.NumGoroutine()
	var heap goruntime.MemStats
	goruntime.ReadMemStats(&heap)
	cancel()
	<-done
	// Run itself takes one goroutine; the runs, none.
	if extra := during - before; extra > 2 {
		t.Errorf("%d goroutines while 1000 runs sleep, %d before: the runs hold goroutines", during, before)
	}
	t.Logf("1000 sleeping runs: %d goroutines over the %d before; heap %d KiB over the %d KiB before",
		during-before, before, (int64(heap.HeapAlloc)-int64(heapBefore.HeapAlloc))/1024, heapBefore.HeapAlloc/1024)
}
