package interp

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"sync"

	"eyelang/ir"
)

// Checkpoints (ROADMAP 7.3): a job run resumes from the statement it was in,
// not by replaying every step it took since it started.
//
// At the start of each statement of a job's body, nested statements of
// while and for loops, ifs and matches included, the run saves a checkpoint
// when it has taken steps since the last one: the job's local variables, the
// path from the body to the statement, and where each loop on that path is.
// The steps before it are dropped, since what they returned is in the
// variables. A run that wakes restores the variables, goes down the path to
// the statement, and runs on from there, replaying only the steps taken
// since the checkpoint. So a run that never ends, as a watcher that loops
// every minute, keeps a record the size of its variables and of one turn of
// its loop, however long it runs.
//
// A variable that holds what can't be saved, as a function, isn't kept: the
// checker refuses one live across a `!`, so nothing reads it after the
// checkpoint. A loop over what can't be saved takes no checkpoints inside;
// the run replays from the one before it. A checkpoint names the statement
// it resumes at by a hash of its
// typed IR, so a run whose job changed under it faults instead of resuming
// at the wrong place (SPECS.md §15).

// checkpoint is a job run's saved place.
type checkpoint struct {
	Path  []pathElem
	Slots [][]byte // the job's local variables, encoded; nil for one not set yet
	// Unsaved are the variables that held what can't be saved, which
	// nothing reads after the checkpoint.
	Unsaved []int  `json:",omitempty"`
	Code    string // the hash of the statement it resumes at
	// Steps is how many steps the run took before it, and Spent what their
	// model calls spent, which counts against the job's @budget.
	Steps int   `json:",omitempty"`
	Spent spend `json:",omitzero"`
}

// pathElem is one step on the path from a job's body to a statement: the
// statement of a block ('b'), the branch of an if ('i', 0 for then), the arm
// of a match ('m'), the body of a while loop ('w'), or the element of a for
// loop ('f' over the encoded elements in Elems, 'r' over a range from Lo to
// Hi). I is the index.
type pathElem struct {
	Kind   byte
	I      int64
	Elems  []byte `json:",omitempty"`
	Lo, Hi int64  `json:",omitempty"`
}

// checkpointing is what a job run's own frame keeps while it runs: the path
// to the statement running, and the expression that may run as a statement
// next.
type checkpointing struct {
	run  *jobRun
	path []pathElem
	stmt ir.Expr
	code string // while resuming: the hash the statement resumed at must have
}

// runJob runs a job's body for a run: from the start, or from its
// checkpoint.
func (in *Interp) runJob(run *jobRun, job *ir.Job, args []argVal) Value {
	fn := job.Func
	fr := &frame{fn: fn, slots: make([]Value, fn.Slots), ck: &checkpointing{run: run}}
	ck := run.Checkpoint
	body := func() Value {
		fr.ck.stmt = fn.Body
		return in.runBody(fr)
	}
	if ck != nil {
		in.restore(fr, ck, job)
		body = func() Value { return in.resumeBody(fr, ck) }
	} else {
		in.bind(fn, fr, args, job.Pos)
	}
	if fn.Budget == nil {
		return body()
	}
	return in.underBudget(fn, func() Value {
		if ck != nil {
			in.charge(ck.Spent) // what the steps before the checkpoint spent
		}
		return body()
	})
}

// restore sets the job's variables to those its checkpoint saved.
func (in *Interp) restore(fr *frame, ck *checkpoint, job *ir.Job) {
	if len(ck.Slots) != len(fr.slots) {
		in.changed(job.Pos, job)
	}
	for i, b := range ck.Slots {
		if b != nil {
			fr.slots[i] = in.decode(b, job.Pos)
		}
	}
	for _, i := range ck.Unsaved {
		if i >= 0 && i < len(fr.slots) {
			fr.slots[i] = unsaved{}
		}
	}
}

// unsaved stands in for a variable whose value a checkpoint couldn't save.
type unsaved struct{}

func (in *Interp) changed(at ir.Pos, job *ir.Job) {
	in.faultf(at, "the code of %s changed while this run was paused, so it can't resume where it stopped: end the run, or put the code back", job.Name)
}

// resumeBody runs a job's body from its checkpoint, as runBody runs it from
// the start.
func (in *Interp) resumeBody(fr *frame, ck *checkpoint) (v Value) {
	defer func() {
		if r := recover(); r != nil {
			if rs, ok := r.(returnSignal); ok {
				v = rs.v
				return
			}
			panic(r)
		}
	}()
	fr.ck.code = ck.Code
	return returned(fr, in.resumeAt(fr, fr.fn.Body, ck.Path))
}

// resumeAt goes down path from e to the statement a checkpoint saved, runs
// it, and runs on from there as e would have.
func (in *Interp) resumeAt(f *frame, e ir.Expr, path []pathElem) Value {
	if len(path) == 0 {
		if stmtHash(e) != f.ck.code {
			in.changed(e.Position(), in.jobOf(f.fn))
		}
		f.ck.stmt = e
		return in.eval(f, e)
	}
	p, rest := path[0], path[1:]
	bad := func() Value {
		in.changed(e.Position(), in.jobOf(f.fn))
		return nil
	}
	switch x := e.(type) {
	case *ir.Block:
		if p.Kind != 'b' || p.I < 0 || p.I >= int64(len(x.Exprs)) {
			return bad()
		}
		return in.stmts(f, x, int(p.I), rest, true)
	case *ir.If:
		switch {
		case p.Kind == 'i' && p.I == 0:
			return in.enter(f, p, x.Then, rest, true)
		case p.Kind == 'i' && p.I == 1 && x.Else != nil:
			return in.enter(f, p, x.Else, rest, true)
		}
	case *ir.Match:
		if p.Kind == 'm' && p.I >= 0 && p.I < int64(len(x.Arms)) {
			return in.enter(f, p, x.Arms[p.I].Body, rest, true)
		}
	case *ir.While:
		if p.Kind == 'w' {
			if !in.loopEnter(f, p, x.Body, rest, true, in.ctlOf(f).catch[x]) {
				in.whileStmt(f, x)
			}
			return ir.Unit{}
		}
	case *ir.For:
		if p.Kind == 'f' || p.Kind == 'r' {
			in.forResume(f, x, p, rest)
			return ir.Unit{}
		}
	}
	return bad()
}

// checkResumePoints finds runs that wait at a checkpoint whose statement
// the program no longer has, as the same code at the same place. `eye run`
// refuses to start on them, rather than fault each when it wakes (SPECS.md
// §15).
func (rt *runtime) checkResumePoints() error {
	count := map[string]int{}
	var jobs []string
	// From the runs' handles, which say where each resumes; the runs paused
	// at one statement share the answer.
	ok := map[[2]uint32]bool{}
	rt.index.each(func(_ int64, h *runHandle) {
		if h.resume == 0 || h.status.finished() {
			return
		}
		name := rt.index.jobOf(h)
		at := [2]uint32{h.job, h.resume}
		fine, seen := ok[at]
		if !seen {
			job := rt.in.jobs[name]
			ck, read := resumePointCheckpoint(rt.index.names[h.resume])
			fine = job != nil && read && resumable(job, ck)
			ok[at] = fine
		}
		if fine {
			return
		}
		if count[name] == 0 {
			jobs = append(jobs, name)
		}
		count[name]++
	})
	if len(jobs) == 0 {
		return nil
	}
	sort.Strings(jobs)
	var problems []string
	for _, j := range jobs {
		runs := fmt.Sprintf("%d paused runs", count[j])
		if count[j] == 1 {
			runs = "1 paused run"
		}
		problems = append(problems, fmt.Sprintf("%s (%s)", strings.TrimPrefix(j, "."), runs))
	}
	return fmt.Errorf("the code of a job changed where its runs are paused, so they can't resume: %s; put the code back, or remove those runs from the store", strings.Join(problems, ", "))
}

// resumable reports whether a checkpoint's path still leads to the same
// statement in a job's body, and the job keeps as many variables.
func resumable(job *ir.Job, ck *checkpoint) bool {
	if len(ck.Slots) != job.Func.Slots {
		return false
	}
	e := job.Func.Body
	for _, p := range ck.Path {
		switch x := e.(type) {
		case *ir.Block:
			if p.Kind != 'b' || p.I < 0 || p.I >= int64(len(x.Exprs)) {
				return false
			}
			e = x.Exprs[p.I]
		case *ir.If:
			switch {
			case p.Kind == 'i' && p.I == 0:
				e = x.Then
			case p.Kind == 'i' && p.I == 1 && x.Else != nil:
				e = x.Else
			default:
				return false
			}
		case *ir.Match:
			if p.Kind != 'm' || p.I < 0 || p.I >= int64(len(x.Arms)) {
				return false
			}
			e = x.Arms[p.I].Body
		case *ir.While:
			if p.Kind != 'w' {
				return false
			}
			e = x.Body
		case *ir.For:
			if p.Kind != 'f' && p.Kind != 'r' {
				return false
			}
			e = x.Body
		default:
			return false
		}
	}
	return stmtHash(e) == ck.Code
}

// jobOf finds the job a body belongs to, for messages.
func (in *Interp) jobOf(fn *ir.Func) *ir.Job {
	for _, j := range in.jobs {
		if j.Func == fn {
			return j
		}
	}
	return &ir.Job{Name: fn.Name, Pos: fn.Pos}
}

// stmt runs e as a statement of a job's body, if it holds statements: a
// block, an if, a match or a loop. ok is false for anything else, which runs
// as an expression.
func (in *Interp) stmt(f *frame, e ir.Expr) (v Value, ok bool) {
	switch x := e.(type) {
	case *ir.Block:
		return in.stmts(f, x, 0, nil, false), true
	case *ir.If:
		if in.truth(in.eval(f, x.Cond), x.Pos) {
			return in.enter(f, pathElem{Kind: 'i', I: 0}, x.Then, nil, false), true
		}
		if x.Else != nil {
			return in.enter(f, pathElem{Kind: 'i', I: 1}, x.Else, nil, false), true
		}
		return ir.Unit{}, true
	case *ir.Match:
		v := in.eval(f, x.Value)
		if f.ctl != ctlNone {
			return nil, true
		}
		for i, arm := range x.Arms {
			if in.match(f, arm.Pattern, v) && (arm.Guard == nil || in.truth(in.eval(f, arm.Guard), x.Pos)) {
				return in.enter(f, pathElem{Kind: 'm', I: int64(i)}, arm.Body, nil, false), true
			}
		}
		in.faultf(x.Pos, "no arm matches %s", show(v))
	case *ir.While:
		in.whileStmt(f, x)
		return ir.Unit{}, true
	case *ir.For:
		in.forStmt(f, x)
		return ir.Unit{}, true
	}
	return nil, false
}

// stmts runs a block's statements from the one at from, taking a checkpoint
// before each. resume says the statement at from is where a checkpoint
// resumes, down rest.
func (in *Interp) stmts(f *frame, x *ir.Block, from int, rest []pathElem, resume bool) Value {
	var v Value = ir.Unit{}
	n := len(f.ck.path)
	for i := from; i < len(x.Exprs); i++ {
		s := x.Exprs[i]
		f.ck.path = append(f.ck.path[:n], pathElem{Kind: 'b', I: int64(i)})
		if resume && i == from {
			v = in.resumeAt(f, s, rest)
		} else {
			in.checkpoint(f, s)
			f.ck.stmt = s
			v = in.eval(f, s)
		}
		if f.ctl != ctlNone {
			f.ck.path = f.ck.path[:n]
			return nil
		}
	}
	f.ck.path = f.ck.path[:n]
	if !x.Value {
		return ir.Unit{}
	}
	return v
}

// enter runs body, which holds statements, one step further down the path.
func (in *Interp) enter(f *frame, p pathElem, body ir.Expr, rest []pathElem, resume bool) Value {
	n := len(f.ck.path)
	f.ck.path = append(f.ck.path[:n], p)
	var v Value
	if resume {
		v = in.resumeAt(f, body, rest)
	} else {
		f.ck.stmt = body
		v = in.eval(f, body)
	}
	f.ck.path = f.ck.path[:n]
	return v
}

// loopEnter runs one turn of a loop's body as enter does, and reports
// whether the loop should stop. catch runs it under a recover, as loopBody
// does.
func (in *Interp) loopEnter(f *frame, p pathElem, body ir.Expr, rest []pathElem, resume, catch bool) bool {
	if catch {
		n := len(f.ck.path)
		stop := caught(func() { in.enter(f, p, body, rest, resume) })
		f.ck.path = f.ck.path[:n] // a break or continue by a panic leaves from further down
		if stop {
			return true
		}
	} else {
		in.enter(f, p, body, rest, resume)
	}
	return loopFlag(f)
}

func (in *Interp) whileStmt(f *frame, x *ir.While) {
	catch := in.ctlOf(f).catch[x]
	for in.truth(in.eval(f, x.Cond), x.Pos) {
		if in.loopEnter(f, pathElem{Kind: 'w'}, x.Body, nil, false, catch) {
			return
		}
	}
}

// forStmt runs a for loop of a job's body, keeping where it is, so that a
// checkpoint inside can resume there. A loop over what can't be saved, or
// over a channel, runs as an expression: a checkpoint is then taken only
// before it.
func (in *Interp) forStmt(f *frame, x *ir.For) {
	iter := in.eval(f, x.Iter)
	if r, ok := iter.(*RangeVal); ok {
		lo, ok1 := r.Start.(int64)
		hi, ok2 := r.End.(int64)
		if ok1 && ok2 {
			if r.Inclusive {
				hi++
			}
			in.forRange(f, x, lo, hi, lo, nil, false)
			return
		}
	}
	elems, ok := in.loopElems(iter, x.Pos)
	if !ok {
		in.forOver(f, x, iter)
		return
	}
	data, ok := in.tryEncode(&List{Elems: elems})
	if !ok {
		in.forOver(f, x, iter)
		return
	}
	in.forElems(f, x, elems, data, 0, nil, false)
}

// forResume resumes a for loop at the element a checkpoint saved.
func (in *Interp) forResume(f *frame, x *ir.For, p pathElem, rest []pathElem) {
	if p.Kind == 'r' {
		in.forRange(f, x, p.Lo, p.Hi, p.I, rest, true)
		return
	}
	l, ok := in.decode(p.Elems, x.Pos).(*List)
	if !ok {
		in.changed(x.Pos, in.jobOf(f.fn))
	}
	in.forElems(f, x, l.Elems, p.Elems, int(p.I), rest, true)
}

func (in *Interp) forElems(f *frame, x *ir.For, elems []Value, data []byte, from int, rest []pathElem, resume bool) {
	catch := in.ctlOf(f).catch[x]
	for i := from; i < len(elems); i++ {
		p := pathElem{Kind: 'f', I: int64(i), Elems: data}
		resuming := resume && i == from
		if !resuming && !in.match(f, x.Pattern, elems[i]) {
			in.faultf(x.Pos, "%s doesn't match the loop's pattern", show(elems[i]))
		}
		if in.loopEnter(f, p, x.Body, rest, resuming, catch) {
			return
		}
	}
}

func (in *Interp) forRange(f *frame, x *ir.For, lo, hi, from int64, rest []pathElem, resume bool) {
	catch := in.ctlOf(f).catch[x]
	for i := from; i < hi; i++ {
		p := pathElem{Kind: 'r', I: i, Lo: lo, Hi: hi}
		resuming := resume && i == from
		if !resuming && !in.match(f, x.Pattern, i) {
			in.faultf(x.Pos, "%d doesn't match the loop's pattern", i)
		}
		if in.loopEnter(f, p, x.Body, rest, resuming, catch) {
			return
		}
	}
}

// loopElems lists what a for loop walks, for the kinds a checkpoint can keep.
func (in *Interp) loopElems(iter Value, at ir.Pos) ([]Value, bool) {
	switch it := iter.(type) {
	case *List:
		return it.Elems, true
	case *Set:
		return it.Elems, true
	case *Map:
		out, vals := make([]Value, len(it.Keys)), it.values()
		for i, k := range it.Keys {
			out[i] = &Tuple{Elems: []Value{k, vals[i]}}
		}
		return out, true
	case *EntitySet:
		return in.rt.all(it.Entity, at).Elems, true
	case Empty:
		return nil, true
	}
	return nil, false
}

// checkpoint saves the run's place before statement s, if the run took steps
// since its last checkpoint and everything it would keep can be saved.
func (in *Interp) checkpoint(f *frame, s ir.Expr) {
	run := f.ck.run
	if run != in.run || in.task != nil || len(run.Log) == 0 || run.cursor < len(run.Log) {
		return
	}
	slots := make([][]byte, len(f.slots))
	var lost []int
	for i, v := range f.slots {
		if v == nil {
			continue
		}
		b, ok := in.tryEncode(plain(v))
		if !ok {
			// A function or a Secret: no value that can't be saved is live
			// across a `!` (SPECS.md §15), and the run took a step since its
			// last checkpoint, so nothing reads this one again.
			lost = append(lost, i)
			continue
		}
		slots[i] = b
	}
	ck := &checkpoint{Path: append([]pathElem(nil), f.ck.path...), Slots: slots, Unsaved: lost, Code: stmtHash(s)}
	if prev := run.Checkpoint; prev != nil {
		ck.Steps, ck.Spent = prev.Steps, prev.Spent
	}
	for _, st := range run.Log {
		ck.Steps++
		ck.Spent.add(st.spent())
		if st.Seq != 0 {
			run.Since = st.Seq // next! looks at events after the last step
		}
	}
	run.Trail = append(run.Trail, run.Log...)
	if n := len(run.Trail); n > trailSize {
		run.Trail = append([]step(nil), run.Trail[n-trailSize:]...)
	}
	run.Checkpoint, run.Log, run.cursor = ck, nil, 0
}

// trailSize is how many of the steps a checkpoint drops a run keeps for eye
// runs.
const trailSize = 20

// tryEncode saves v, or reports that it can't be saved.
func (in *Interp) tryEncode(v Value) (data []byte, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, fault := r.(*Fault); !fault {
				panic(r)
			}
			data, ok = nil, false
		}
	}()
	return in.encode(v, ir.Pos{}), true
}

var stmtHashes sync.Map // ir.Expr → string

// stmtHash names a statement by its typed IR, so that a checkpoint notices
// when the code it would resume at changed.
func stmtHash(e ir.Expr) string {
	if h, ok := stmtHashes.Load(e); ok {
		return h.(string)
	}
	var b bytes.Buffer
	ir.FprintExpr(&b, e)
	sum := fnv.New64a()
	sum.Write(b.Bytes())
	h := fmt.Sprintf("%016x", sum.Sum64())
	stmtHashes.Store(e, h)
	return h
}
