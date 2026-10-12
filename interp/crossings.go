package interp

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"eyelang/ir"
)

// Crossings (ROADMAP 7.10): once the Go runner hosts compiled programs
// (hosting B′, docs/internals/reviews/native-plan-2026-10-12.md section 5),
// every operation the runner owns is a call between compiled code and Go,
// at about 35 ns each. The interpreter counts those operations as it runs
// them, so that a real run says how many there would be.
//
// Into the runner, from compiled code, are the low IR's Runtime operations:
// entity calls, reads and references, E.all, view reads, job starts,
// contract operations, event publishes, host functions (http.get,
// ai.generate, fs.*, process.run, env.get, log.*), and the `!` steps whose
// operation isn't one of those (now!, sleep!, poll!, join!, …). Into
// compiled code, from the runner, are a run's turns (each time the runner
// resumes a job's state machine), entity methods run on a state, views
// checked for a changed entity, contract operations served, and each
// encode or decode of a saved value (entity states, steps, checkpoints,
// a run's arguments and result).
//
// Each operation counts once: entity.call!(…) is an entity.call, not also a
// step. What the interpreter replays since a run's last checkpoint isn't
// counted, since those operations don't run again; the decodes of the
// replayed results are, as compiled code resuming from its saved frame
// decodes the frame.

// crossingCost is what one crossing costs under B′: 31–39 ns in the survey
// (native plan, section 5).
const crossingCost = 35 * time.Nanosecond

// crossingKinds is the order kinds are shown in: into the runner, then
// into compiled code. Host functions follow the first group, by name.
var crossingKinds = []string{
	"entity.call", "entity.read", "entity.field", "entity.ref", "entity.all",
	"view.read", "job.start", "op.call", "op.reconcile", "event.publish", "step",
	"run.step", "entity.method", "view.check", "op.serve", "encode", "decode",
}

// intoCompiled are the kinds that cross from the runner into compiled code.
var intoCompiled = map[string]bool{
	"run.step": true, "entity.method": true, "view.check": true, "op.serve": true, "encode": true, "decode": true,
}

// Crossings is what one `eye run` or one test counted, by run.
type Crossings struct {
	mu     sync.Mutex
	runs   []*RunCrossings
	cur    *RunCrossings
	byID   map[int64]*RunCrossings
	byName map[string]*RunCrossings
}

// RunCrossings is what one run crossed: main, a test's own body, a job's
// run, or the runner between runs (schedules firing, events delivered).
type RunCrossings struct {
	Name   string
	Counts map[string]int64
}

// NewCrossings starts counting, with name for what runs first: "main",
// "the runner", or "the test".
func NewCrossings(name string) *Crossings {
	c := &Crossings{byID: map[int64]*RunCrossings{}, byName: map[string]*RunCrossings{}}
	c.enter(name)
	return c
}

func (c *Crossings) add(name string) *RunCrossings {
	r := &RunCrossings{Name: name, Counts: map[string]int64{}}
	c.runs = append(c.runs, r)
	return r
}

// Runs are the runs that crossed at least once, in the order they began.
func (c *Crossings) Runs() []*RunCrossings {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []*RunCrossings
	for _, r := range c.runs {
		if r.Total() > 0 {
			out = append(out, r)
		}
	}
	return out
}

func (c *Crossings) count(kind string) {
	c.mu.Lock()
	c.cur.Counts[kind]++
	c.mu.Unlock()
}

// enterRun counts what follows for a job's run, until the function it
// returns is called.
func (c *Crossings) enterRun(id int64, job string) func() {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.byID[id]
	if !ok {
		r = c.add(fmt.Sprintf("job %s (run %d)", strings.TrimPrefix(job, "."), id)) // a job of the program's own module has no path
		c.byID[id] = r
	}
	prev := c.cur
	c.cur = r
	return func() {
		c.mu.Lock()
		c.cur = prev
		c.mu.Unlock()
	}
}

// enter counts what follows, outside job runs, for the named part of the
// run: main, or the runner between runs.
func (c *Crossings) enter(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.byName[name]
	if !ok {
		r = c.add(name)
		c.byName[name] = r
	}
	c.cur = r
}

// crossed counts one crossing of the kind, when crossings are counted. A
// job run going over steps it took before its checkpoint crosses only to
// decode what they returned.
func (rt *runtime) crossed(kind string) {
	if rt.crossings != nil && (kind == "decode" || !rt.in.replaying()) {
		rt.crossings.count(kind)
	}
}

// hostBuiltin reports whether a built-in is a host function under B′:
// env.get and the log functions.
func hostBuiltin(name string) bool { return name == "env.get" || strings.HasPrefix(name, "log.") }

// countBuiltins counts the calls of the built-ins that are host functions.
func (rt *runtime) countBuiltins(table map[string]*BuiltinFn) {
	for name, b := range table {
		if !hostBuiltin(name) {
			continue
		}
		fn, kind := b.Fn, name
		table[name] = &BuiltinFn{Name: b.Name, Fn: func(in *Interp, args []Value, named map[string]Value, at ir.Pos) Value {
			in.rt.crossed(kind)
			return fn(in, args, named, at)
		}}
	}
}

// Total is how many crossings the run made.
func (r *RunCrossings) Total() int64 {
	var n int64
	for _, c := range r.Counts {
		n += c
	}
	return n
}

// Kinds are the kinds the run crossed by, in crossingKinds' order, with
// host functions after the first group.
func (r *RunCrossings) Kinds() []string {
	rank := map[string]int{}
	for i, k := range crossingKinds {
		rank[k] = i
	}
	hosts := rank["run.step"] // host functions go between the two groups
	var ks []string
	for k := range r.Counts {
		ks = append(ks, k)
	}
	pos := func(k string) (int, string) {
		if i, ok := rank[k]; ok {
			if i >= hosts {
				i++
			}
			return i, ""
		}
		return hosts, k
	}
	sort.Slice(ks, func(i, j int) bool {
		a, an := pos(ks[i])
		b, bn := pos(ks[j])
		if a != b {
			return a < b
		}
		return an < bn
	})
	return ks
}

// WriteCrossings writes a table of each run's crossings by kind, a total
// for each, and what they cost at 35 ns each.
func WriteCrossings(w io.Writer, runs []*RunCrossings) {
	if len(runs) == 0 {
		fmt.Fprintln(w, "no crossings")
		return
	}
	width := len("total")
	for _, r := range runs {
		for k := range r.Counts {
			width = max(width, len(k))
		}
	}
	var all int64
	for _, r := range runs {
		fmt.Fprintf(w, "%s\n", r.Name)
		for _, k := range r.Kinds() {
			dir := "into the runner"
			if intoCompiled[k] {
				dir = "into compiled code"
			}
			fmt.Fprintf(w, "  %-*s %8d  %s\n", width, k, r.Counts[k], dir)
		}
		t := r.Total()
		all += t
		fmt.Fprintf(w, "  %-*s %8d  %s at 35 ns\n", width, "total", t, cost(t))
	}
	if len(runs) > 1 {
		fmt.Fprintf(w, "all runs: %d crossings, %s at 35 ns\n", all, cost(all))
	}
}

func cost(n int64) time.Duration { return time.Duration(n) * crossingCost }
