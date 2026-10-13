package interp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"eyelang/ir"
)

// payments stands in for a payment service: it counts the charges it takes,
// and answers receipts only once it is up.
type payments struct {
	mu      sync.Mutex
	charges int
	up      bool
}

func (p *payments) serve(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		defer p.mu.Unlock()
		switch {
		case r.Method == "POST" && r.URL.Path == "/charges":
			p.charges++
			w.WriteHeader(http.StatusCreated)
		case !p.up:
			http.Error(w, "down for maintenance", http.StatusServiceUnavailable)
		default:
			_, _ = w.Write([]byte(`"r-1"`))
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PAYMENTS_URL", srv.URL)
	return srv
}

func (p *payments) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.charges
}

func (p *payments) setUp() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.up = true
}

var retryProgram = map[string]string{
	"payments/payments.eye": `
pub type Charged = | Charged

contract Payments {
  @http("POST /charges", status: {201: Charged})
  @key(required)
  command charge(amount: Int) -> Charged

  @http("GET /receipts/:id")
  query receipt(id: String) -> String
}
`,
	"main.eye": `
import payments

## Charges, then fetches the receipt, which fails while the service is
## down, then waits an hour.
job collect(id: String) -> String {
  match payments.charge!(1500, key: "charge-{id}") {
    Charged => ()
    Uncertain(_) => unreachable("the charge's reply was lost")
  }
  let r = payments.receipt!(id)
  sleep!(1 h)
  r
}

## Starts collect, and joins it after ten minutes.
job parent() -> String {
  let run = collect.start!("i-1", key: "i-1")
  sleep!(10 min)
  match run.join!() {
    Completed(r) => "completed {r}"
    Failed(f) => "failed {f}"
  }
}
`,
}

// runState reads a run's status and outcome from the store.
func runState(in *Interp, id int64) (string, string) {
	data, ok := in.rt.store.Run(id)
	if !ok {
		return "missing", ""
	}
	r := decodeRun(data)
	if r.Status != "done" && r.Status != "failed" {
		return r.Status, ""
	}
	return r.Status, show(in.outcome(r, ir.Pos{}))
}

// TestRetryReplaysAndKeepsTheRun is the retry of SPECS.md section 15 on the
// journal: a run that failed as Unavailable starts again from the runner's
// inbox, replays its charge instead of sending it again, keeps its id and
// key, and a parent that joins it afterwards gets the retry's outcome, not
// the failure from before.
func TestRetryReplaysAndKeepsTheRun(t *testing.T) {
	retryInTheRunner(t, func(t *testing.T) Store {
		j, err := OpenJournal(filepath.Join(t.TempDir(), "journal"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { j.Close() })
		return j
	})
}

// The same on the Postgres store.
func TestRetryReplaysAndKeepsTheRunOnPostgres(t *testing.T) {
	url := testPostgres(t)
	retryInTheRunner(t, func(t *testing.T) Store { return openPostgres(t, url) })
}

func retryInTheRunner(t *testing.T, open func(t *testing.T) Store) {
	pay := &payments{}
	pay.serve(t)
	prog := loadFiles(t, retryProgram)
	in := New(prog, io.Discard, io.Discard)
	in.rt.fastRetries = true
	in.rt.useStore(open(t))
	in.rt.restore()

	in.rt.startNow(prog.Root.Jobs["parent"], nil, nil, ir.Pos{})
	if status, outcome := runState(in, 2); outcome != `Failed(Unavailable("payments.receipt"))` {
		t.Fatalf("collect is %s %s, want Failed(Unavailable(…))", status, outcome)
	}
	if n := pay.count(); n != 1 {
		t.Fatalf("%d charges, want 1", n)
	}

	pay.setUp()
	dir := t.TempDir()
	var reports []string
	report := func(s string) { reports = append(reports, s) }
	if err := SendRetry(dir, InboxRetry{Retry: []int64{2}}); err != nil {
		t.Fatal(err)
	}
	in.rt.readInbox(dir, report)
	if len(reports) != 1 || reports[0] != "run 2 (collect) starts again, replaying its 1 recorded step" {
		t.Errorf("reports %q", reports)
	}
	if status, _ := runState(in, 2); status != "sleeping" {
		t.Errorf("the retried run is %s, want sleeping in its hour", status)
	}
	if n := pay.count(); n != 1 {
		t.Errorf("%d charges after the retry, want 1: the charge is replayed, not sent again", n)
	}
	if run := in.rt.start(prog.Root.Jobs["collect"], []Value{"i-1"}, map[string]Value{"key": "i-1"}, ir.Pos{}); run.(*RunVal).ID != 2 {
		t.Errorf("a keyed start gives run %d, want the retried run 2", run.(*RunVal).ID)
	}

	in.rt.advance(ir.Duration(10 * time.Minute))
	if status, outcome := runState(in, 1); status != "sleeping" {
		t.Errorf("after ten minutes the parent is %s %s, want joining the retried run", status, outcome)
	}
	in.rt.advance(ir.Duration(time.Hour))
	if _, outcome := runState(in, 2); outcome != `Completed("r-1")` {
		t.Errorf("collect = %s", outcome)
	}
	if _, outcome := runState(in, 1); outcome != `Completed("completed r-1")` {
		t.Errorf("parent = %s", outcome)
	}

	// A run that has finished isn't retried, nor one that doesn't exist.
	reports = nil
	for _, id := range []int64{2, 9} {
		if err := SendRetry(dir, InboxRetry{Retry: []int64{id}}); err != nil {
			t.Fatal(err)
		}
		in.rt.readInbox(dir, report)
	}
	if len(reports) != 2 || !strings.HasPrefix(reports[0], "run 2 (collect) hasn't failed: it is done") || !strings.HasPrefix(reports[1], "there is no run 9; the retry in") {
		t.Errorf("reports %q", reports)
	}
}

// Retry changes a store that no runner holds, as eye runs --retry does, and
// the next runner carries the run on.
func TestRetryInAStoreNoRunnerHolds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal")
	retryOffline(t, func() (Store, func() error) {
		j, err := OpenJournal(path)
		if err != nil {
			t.Fatal(err)
		}
		return j, j.Close
	}, func() (Store, error) { return ReadJournal(path) })
}

// The same on the Postgres store.
func TestRetryInAStoreNoRunnerHoldsOnPostgres(t *testing.T) {
	url := testPostgres(t)
	retryOffline(t, func() (Store, func() error) {
		p, err := OpenPostgres(url)
		if err != nil {
			t.Fatal(err)
		}
		return p, p.Close
	}, func() (Store, error) { return ReadPostgres(url) })
}

func retryOffline(t *testing.T, open func() (Store, func() error), read func() (Store, error)) {
	pay := &payments{}
	pay.serve(t)
	prog := loadFiles(t, retryProgram)

	st, closeStore := open()
	in := New(prog, io.Discard, io.Discard)
	in.rt.fastRetries = true
	in.rt.useStore(st)
	in.rt.restore()
	in.rt.startNow(prog.Root.Jobs["collect"], []Value{"i-1"}, map[string]Value{"key": "i-1"}, ir.Pos{})
	if err := closeStore(); err != nil {
		t.Fatal(err)
	}

	r, err := read()
	if err != nil {
		t.Fatal(err)
	}
	if plan, err := RetryPlan(prog, r, nil, true); err != nil || len(plan) != 1 || plan[0] != (Retried{ID: 1, Job: "collect", Steps: 1}) {
		t.Errorf("RetryPlan = %+v, %v; want run 1 with its one step", plan, err)
	}
	st, closeStore = open()
	done, err := Retry(prog, st, []int64{1}, false)
	if cerr := closeStore(); err == nil {
		err = cerr
	}
	if err != nil || len(done) != 1 || done[0].ID != 1 {
		t.Fatalf("Retry = %+v, %v", done, err)
	}

	pay.setUp()
	st, closeStore = open()
	defer func() { _ = closeStore() }()
	in = New(prog, io.Discard, io.Discard)
	in.rt.useStore(st)
	in.rt.restore()
	in.rt.drain()
	in.rt.advance(ir.Duration(time.Hour))
	if _, outcome := runState(in, 1); outcome != `Completed("r-1")` || pay.count() != 1 {
		t.Errorf("collect = %s after %d charges, want Completed after 1", outcome, pay.count())
	}
}

// Only Unavailable and DeadlinePassed are retried; a retry of anything else
// is refused with the reason.
func TestRetryRefusesWhatARetryCantCure(t *testing.T) {
	prog := load(t, `
job bug() { unreachable("checked above") }

job slow() -> Int {
  sleep!(1 h)
  1
}

job quick() -> Int { 2 }

job race() -> Int {
  match firstDone!([quick.start!(), slow.start!()]) {
    Completed(n) => n
    Failed(_) => 0
  }
}

@deadline(1 min)
job late() -> Int {
  sleep!(2 min)
  3
}

job guarded() {
  lock!("ci", for: 1 h, fn {
    sleep!(30 min)
  })
}
`)
	in := New(prog, io.Discard, io.Discard)
	start := func(name string) int64 {
		return in.rt.startNow(prog.Root.Jobs[name], nil, nil, ir.Pos{}).(*RunVal).ID
	}
	bug, race, late := start("bug"), start("race"), start("late")
	in.rt.advance(ir.Duration(2 * time.Minute))
	for _, c := range []struct {
		id   int64
		want string
	}{
		{bug, "run 1 (bug) ended as Failed(Unreachable): a bug in the program or its configuration"},
		{race + 2, "run 4 (slow) ended as Failed(Killed): it was stopped on purpose"},
		{race, "run 2 (race) hasn't failed: it is done"},
		{99, "there is no run 99"},
	} {
		if _, err := in.rt.retryable([]int64{c.id}, false); err == nil || !strings.HasPrefix(err.Error(), c.want) {
			t.Errorf("retrying run %d: %v, want %q", c.id, err, c.want)
		}
	}
	if _, outcome := runState(in, late); outcome != "Failed(DeadlinePassed)" {
		t.Fatalf("late = %s", outcome)
	}
	if runs, err := in.rt.retryable(nil, true); err != nil || len(runs) != 1 || runs[0].ID != late {
		t.Errorf("--retry-failed picks %d runs, %v; want late's alone", len(runs), err)
	}
	// late's deadline counts from the retry, and its sleep, which has run
	// out, replays, so it finishes.
	done := in.rt.again([]*jobRun{decodeRun(must(in.rt.store.Run(late)))})
	in.rt.drain()
	if _, outcome := runState(in, late); len(done) != 1 || outcome != "Completed(3)" {
		t.Errorf("late after its retry = %s", outcome)
	}

	// A run that failed inside lock! gave the lock back, so it isn't
	// retried: it would replay into the block without it.
	g := in.rt.startNow(prog.Root.Jobs["guarded"], nil, nil, ir.Pos{}).(*RunVal).ID
	r := decodeRun(must(in.rt.store.Run(g)))
	r.Status, r.FaultKind, r.FaultArg = "failed", "Unavailable", "svc"
	if why := in.rt.unretryable(r); !strings.Contains(why, `failed inside lock!("ci")`) {
		t.Errorf("a run that failed inside lock!: %q", why)
	}
}

func must(data []byte, ok bool) []byte {
	if !ok {
		panic("no such run")
	}
	return data
}
