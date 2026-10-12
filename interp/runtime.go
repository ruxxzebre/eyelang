package interp

import (
	"encoding/json"
	"math/rand/v2"
	"sort"
	"strconv"
	"time"

	"eyelang/ir"
)

// runtime holds what a running program keeps beyond its values: entities,
// job runs, views, the clock and the fakes of other services.
type runtime struct {
	in       *Interp
	store    Store
	now      time.Time
	real     bool // the clock is the system's (eye run), not a test's
	queue    []int64
	draining bool
	tx       *txn
	selves   []*txRow // the entities whose methods are running, innermost last
	nextRun  int64
	members  map[*ir.View]map[string]bool
	fakes    map[*ir.Contract]*fakeSet
	failures []*Fault // job runs that faulted
	events   []event  // every event published, in order; an event's sequence number is its index + 1
	seen     map[string]bool
	// rate holds, for each job and @limit group, when its recent runs
	// began, oldest first.
	rate map[string][]int64
	// uncertain holds the commands whose reply was lost, by key.
	uncertain map[string]sentCommand
	// aiReplies are recorded model replies, when a test applies fakes(ai, …).
	aiReplies []recordedReply
	// httpFakes answer http.get, when a test applies fakes(http, …).
	httpFakes *httpFakes
	// live, in a test run with eye test --live, says which modules the
	// real services answer for, and keeps what --record writes (live.go).
	live *liveTest
	// fastRetries makes retries of unreachable services immediate, in tests,
	// and beforeRetry, when set, is called before each wait.
	fastRetries bool
	beforeRetry func(try int)
	// scheduled holds the jobs started by schedules, once computed.
	scheduled []*schedule
	// index is the store's index of waiting runs.
	index *indexed
	// module, in a test, is the test's module: only its schedules fire.
	module *ir.Module
	// eventBase is the number of events dropped: events[0] is event
	// eventBase + 1.
	eventBase int64
	// retry holds the runs whose step couldn't be saved, and commitErr why.
	retry     []int64
	commitErr *CommitError
	// warned holds the runs already reported for their many steps.
	warned map[int64]bool
	// memLimit is the memory the runner has, in bytes (admission.go); 0
	// admits everything. reserved is what reserve! granted, by run.
	memLimit int64
	reserved map[int64]int64
	// sandbox, in a test, gives fs and process an empty directory of the
	// test's own, made in dir when first used; rand draws random numbers
	// (system.go).
	sandbox bool
	dir     string
	rand    *rand.Rand
	// kick wakes eye run's runner when a request to a served contract has
	// committed, so the jobs it started run; stopped turns requests away
	// once the runner has stopped (listen.go).
	kick    chan struct{}
	stopped bool
	// journal is the store when it is a journal, whose commits wait to be
	// synced until the runner does something outside (durable).
	journal *Journal
	// crossings, when set, counts what would cross between compiled code
	// and the runner (crossings.go).
	crossings *Crossings
}

// event is a published event, saved as bytes.
type event struct {
	name string
	data []byte
}

func eventName(e *ir.Event) string { return e.Contract.Module.Path + "." + e.Name }

func newRuntime(in *Interp) *runtime {
	rt := &runtime{
		in: in, now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		members: map[*ir.View]map[string]bool{}, fakes: map[*ir.Contract]*fakeSet{}, nextRun: 1,
		seen: map[string]bool{}, uncertain: map[string]sentCommand{}, rate: map[string][]int64{}, warned: map[int64]bool{},
		memLimit: testMemory, reserved: map[int64]int64{},
	}
	if in.prog.Store != nil {
		rt.useStore(newEyeStore(in))
	} else {
		rt.useStore(newMemStore())
	}
	return rt
}

// useStore makes s the runtime's store. A journal or a Postgres store read
// before the program ran keeps its state in the Go store; it moves to the
// store written in Eyelang here.
func (rt *runtime) useStore(s Store) {
	var mem *memory
	rt.journal = nil
	switch d := s.(type) {
	case *Journal:
		mem = &d.memory
		d.syncLater()
		rt.journal = d
	case *Postgres:
		mem = &d.memory
	}
	if mem != nil && rt.in.prog.Store != nil {
		if _, already := (*mem).(*eyeStore); !already {
			fresh := newEyeStore(rt.in)
			fresh.Commit((*mem).snapshot())
			*mem = fresh
		}
	}
	jobs := rt.in.jobs
	concurrent := func(job string) bool { j := jobs[job]; return j == nil || j.Concurrency != nil }
	limited := func(job string) bool { j := jobs[job]; return j != nil && j.Limit != nil }
	rt.index = indexStore(s, concurrent, limited)
	rt.store = rt.index
}

// durable syncs the commits the journal holds unsynced, with one fsync for
// all of them. The runtime calls it before the runner does anything that
// can be seen outside it: before it calls a service or an extern fn, prints,
// answers a request, removes an answer from the inbox, or waits. So nothing
// outside learns of a commit before it is on disk, and commits made one
// after another in between, such as main starting many runs, share a sync.
// It panics with a fatal *CommitError if the sync fails.
func (rt *runtime) durable() {
	if rt.journal == nil {
		return
	}
	if err := rt.journal.Sync(); err != nil {
		panic(err)
	}
}

// ---------------------------------------------------------------- entities

func entityName(e *ir.Entity) string { return e.Module.Path + "." + e.Name }

func entityKey(ref *EntityRef) string { return entityName(ref.Entity) + "/" + keyOf(ref.ID) }

// txn is a transaction: the entities it has loaded, as it has changed them,
// and the jobs it starts when it commits.
type txn struct {
	rows   map[string]*txRow
	order  []string
	starts []*jobRun
	events []pending // published when the transaction commits
	run    *jobRun   // a job run whose step commits with this transaction
}

type txRow struct {
	ref     *EntityRef
	state   *Record
	orig    string
	version int64
	exists  bool
	ended   *ApprovalVal // end(ok) ended it with this approval: the commit keeps only that
}

// load returns an entity's committed state, or its default state if it has
// never been saved.
func (rt *runtime) load(ref *EntityRef, at ir.Pos) (*Record, int64, bool) {
	row, ok := rt.store.Load(entityKey(ref))
	if ok && row.Ended {
		rt.endedFault(ref, row, at)
	}
	if ok {
		return rt.decodeRow(ref.Entity, row, at), row.Version, true
	}
	return rt.defaults(ref, at), 0, false
}

// endRecord is what is kept of an entity that ended: when, and the approval
// that let it (SPECS.md §14).
type endRecord struct {
	At       int64 // Unix nanoseconds
	What, By string
}

// endedFault stops a call on, or a read of, an entity that ended: its
// identity stays ended, so nothing can bring it back by using it.
func (rt *runtime) endedFault(ref *EntityRef, row Row, at ir.Pos) {
	name := ref.Entity.Name + "(" + showValue(ref.ID) + ")"
	var e endRecord
	if json.Unmarshal(row.State, &e) != nil {
		rt.in.faultf(at, "%s ended, so it can't be read or changed", name)
	}
	when := time.Unix(0, e.At).In(rt.now.Location()).Format("2006-01-02 15:04 MST")
	rt.in.faultf(at, "%s ended at %s, approved by %s (%q), so it can't be read or changed", name, when, e.By, e.What)
}

func (rt *runtime) defaults(ref *EntityRef, at ir.Pos) *Record {
	e := ref.Entity
	r := &Record{Names: []string{e.ID.Name}, Vals: []Value{ref.ID}}
	for _, s := range e.States {
		if s.Default == nil {
			rt.in.faultf(at, "%s.%s has no default, so a new %s can't start", e.Name, s.Name, e.Name)
		}
		v, _ := rt.in.callFunc(s.Default, nil, nil, nil, at)
		r.Names = append(r.Names, s.Name)
		r.Vals = append(r.Vals, coerce(v, s.Type))
	}
	return r
}

// coerce gives `{}` the collection type it was declared with.
func coerce(v Value, t *ir.TypeExpr) Value {
	if _, ok := v.(Empty); !ok || t == nil {
		return v
	}
	switch t.Name {
	case "Set":
		return newSet(nil)
	case "Map":
		return newMap(nil, nil)
	case "List":
		return &List{}
	case "record":
		return &Record{}
	}
	return v
}

// current returns an entity's state as the current transaction sees it.
func (rt *runtime) current(ref *EntityRef, at ir.Pos) *Record {
	state, _ := rt.stateOf(ref, at)
	return state
}

// snapshot is what `read!()` returns: the state with its version. In a
// transaction that has the entity, it is the state in progress, at the
// version it was loaded at.
func (rt *runtime) snapshot(ref *EntityRef, at ir.Pos) *Record {
	return snapshotOf(rt.stateOf(ref, at))
}

// stateOf returns an entity's state and version as the current transaction
// sees them: in progress if a call in the transaction has it, else as
// committed.
func (rt *runtime) stateOf(ref *EntityRef, at ir.Pos) (*Record, int64) {
	if rt.tx != nil {
		if r, ok := rt.tx.rows[entityKey(ref)]; ok {
			if r.ended != nil {
				rt.endedInTx(ref, at)
			}
			return r.state, r.version
		}
	}
	state, version, _ := rt.load(ref, at)
	return state, version
}

// endedInTx stops a call on, or a read of, an entity that a call earlier in
// the transaction ended.
func (rt *runtime) endedInTx(ref *EntityRef, at ir.Pos) {
	rt.in.faultf(at, "%s(%s) ended earlier in this transaction, so it can't be read or changed", ref.Entity.Name, showValue(ref.ID))
}

func snapshotOf(state *Record, version int64) *Record {
	s := &Record{Names: append([]string{}, state.Names[0], "version"), Vals: append([]Value{}, state.Vals[0], version)}
	s.Names = append(s.Names, state.Names[1:]...)
	s.Vals = append(s.Vals, state.Vals[1:]...)
	return s
}

func (rt *runtime) begin() *txn {
	tx := &txn{rows: map[string]*txRow{}}
	rt.tx = tx
	return tx
}

func (tx *txn) row(rt *runtime, ref *EntityRef, at ir.Pos) *txRow {
	key := entityKey(ref)
	if r, ok := tx.rows[key]; ok {
		if r.ended != nil {
			rt.endedInTx(ref, at)
		}
		return r
	}
	state, version, exists := rt.load(ref, at)
	r := &txRow{ref: ref, state: state, orig: keyOf(state), version: version, exists: exists}
	tx.rows[key] = r
	tx.order = append(tx.order, key)
	return r
}

// callEntity calls an entity method as a transaction, or as part of the
// current one.
func (rt *runtime) callEntity(ref *EntityRef, m *ir.Func, args []argVal, at ir.Pos) Value {
	rt.crossed("entity.call")
	if rt.tx != nil {
		return rt.callIn(rt.tx, ref, m, args, at)
	}
	tx := rt.begin()
	defer func() {
		if rt.tx == tx {
			rt.tx = nil // a fault rolls the transaction back
		}
	}()
	result := rt.callIn(tx, ref, m, args, at)
	rt.tx = nil
	rt.commit(tx)
	return result
}

// serve runs an operation of a contract the program serves, as one
// transaction: the entity calls it makes commit together, or not at all.
func (rt *runtime) serve(op *ir.Op, args []argVal, at ir.Pos) Value {
	rt.crossed("op.serve")
	prevRun := rt.in.run
	rt.in.run = nil // serving an operation never waits
	defer func() { rt.in.run = prevRun }()
	if rt.tx != nil {
		r, _ := rt.in.callFunc(op.Impl, nil, nil, args, at)
		return r
	}
	tx := rt.begin()
	defer func() {
		if rt.tx == tx {
			rt.tx = nil // a fault rolls the transaction back
		}
	}()
	r, _ := rt.in.callFunc(op.Impl, nil, nil, args, at)
	rt.tx = nil
	rt.commit(tx)
	return r
}

func (rt *runtime) callIn(tx *txn, ref *EntityRef, m *ir.Func, args []argVal, at ir.Pos) Value {
	rt.crossed("entity.method")
	row := tx.row(rt, ref, at)
	prevRun := rt.in.run
	rt.in.run = nil // entity methods never wait, even when a job calls them
	rt.selves = append(rt.selves, row)
	defer func() {
		rt.in.run = prevRun
		rt.selves = rt.selves[:len(rt.selves)-1]
	}()
	// The method changes row.state as it goes, so a call that comes back to
	// the entity before it returns starts from its changes, and it goes on
	// from the changes that call made (SPECS.md §14).
	return rt.in.callOnRow(m, row, args, at)
}

// end ends the entity whose method is running: `end(ok)`. Its row is
// removed when the transaction commits (SPECS.md §14).
func (rt *runtime) end(args []argVal, at ir.Pos) {
	pos, _ := split(args)
	if len(pos) != 1 {
		rt.in.faultf(at, "end(ok) takes an Approval")
	}
	ok, isApproval := pos[0].(*ApprovalVal)
	if !isApproval {
		rt.in.faultf(at, "end(ok) takes an Approval, and this is %s", kindName(pos[0]))
	}
	if len(rt.selves) == 0 {
		rt.in.faultf(at, "end(ok) ran outside an entity's method")
	}
	rt.selves[len(rt.selves)-1].ended = ok
}

// commit saves a transaction, starts the jobs it started, and updates the
// views over the entities it changed.
func (rt *runtime) commit(tx *txn) {
	c := &Commit{}
	var changed, gone []*txRow
	for _, key := range tx.order {
		r := tx.rows[key]
		if r.ended != nil {
			// The row stays, saying how the entity ended, so that its
			// identity stays ended.
			end, _ := json.Marshal(endRecord{At: rt.now.UnixNano(), What: r.ended.What, By: r.ended.By})
			c.Writes = append(c.Writes, Write{Key: key, Entity: entityName(r.ref.Entity), State: end, Version: r.version + 1, Schema: entityVersion(r.ref.Entity), Ended: true})
			gone = append(gone, r)
			continue
		}
		isChanged := keyOf(r.state) != r.orig
		if !isChanged && r.exists {
			continue
		}
		if isChanged {
			r.version++
			changed = append(changed, r)
		}
		c.Writes = append(c.Writes, Write{Key: key, Entity: entityName(r.ref.Entity), State: rt.in.encode(r.state, ir.Pos{}), Version: r.version, Schema: entityVersion(r.ref.Entity)})
	}
	for _, run := range tx.starts {
		c.Runs = append(c.Runs, RunWrite{ID: run.ID, Data: run.encode()})
	}
	if tx.run != nil {
		c.Runs = append(c.Runs, RunWrite{ID: tx.run.ID, Data: tx.run.encode()})
	}
	rt.store.Commit(c)
	for _, run := range tx.starts {
		if run.Status == "ready" {
			rt.queue = append(rt.queue, run.ID)
		}
	}
	for _, r := range changed {
		rt.updateViews(r.ref, r.state, r.version)
	}
	for _, r := range gone {
		rt.updateViews(r.ref, nil, 0) // it leaves every view
	}
	for _, e := range tx.events {
		rt.deliver(e.event, e.value)
	}
	rt.drain()
}

type pending struct {
	event *ir.Event
	value Value
}

// publish publishes an event: when the current transaction commits, or now.
func (rt *runtime) publish(e *ir.Event, v Value) {
	if rt.tx != nil {
		rt.tx.events = append(rt.tx.events, pending{e, v})
		return
	}
	rt.deliver(e, v)
	rt.drain()
}

// answer records a person's answer to the approval request id and wakes the
// run waiting for it. by is who answered, as text.
func (rt *runtime) answer(id string, approved bool, by string) {
	v := &Record{Names: []string{"id", "approved", "by"}, Vals: []Value{id, approved, by}}
	if e := rt.in.approvalsEvent("answered"); e != nil {
		// The program imports approvals, so jobs may start on an answer.
		rt.publish(e, v)
		return
	}
	rt.record(&Commit{}, approvalEvent, rt.in.encode(v, ir.Pos{}))
	rt.checkAnswer(v)
	rt.wake(approvalEvent)
	rt.drain()
}

// record adds an event to the log, in the store with the rest of c.
func (rt *runtime) record(c *Commit, name string, data []byte) {
	c.Events = append(c.Events, EventRecord{Name: name, Data: data})
	rt.store.Commit(c)
	rt.events = append(rt.events, event{name: name, data: data})
}

// wake makes ready the runs waiting for an event named name.
func (rt *runtime) wake(name string) {
	for _, id := range rt.index.waitingFor(name) {
		data, _ := rt.store.Run(id)
		run := decodeRun(data)
		if run.Status == "sleeping" && waitsFor(run.WaitEvent, name) {
			run.Status = "ready"
			rt.save(run)
			rt.queue = append(rt.queue, run.ID)
		}
	}
}

// deliver records an event, starts the jobs it triggers, and wakes the runs
// waiting for one like it. An event whose @key field was seen before is a
// repeat, and does nothing.
func (rt *runtime) deliver(e *ir.Event, v Value) {
	name := eventName(e)
	c := &Commit{}
	seen := ""
	if e.Key != "" {
		if r, ok := v.(*Record); ok {
			if k, ok := r.get(e.Key); ok {
				seen = name + "/" + keyOf(k)
				if rt.seen[seen] {
					return
				}
				// The key is kept, with when it was seen, for keepSeen.
				c.Meta = append(c.Meta, MetaWrite{Key: "seen/" + seen, Data: []byte(strconv.FormatInt(rt.now.UnixNano(), 10))})
			}
		}
	}
	// The event, the key that marks it seen and the runs it starts are saved
	// together, so a restart neither loses nor repeats them.
	var started []int64
	for _, m := range rt.in.prog.Modules {
		for _, j := range m.Jobs {
			for _, t := range j.Triggers {
				if t.Event == e {
					run := rt.newRun(j, []Value{v}, nil, j.Pos)
					run.Since++ // the event that starts it isn't one it waits for
					c.Runs = append(c.Runs, RunWrite{ID: run.ID, Data: run.encode()})
					started = append(started, run.ID)
				}
			}
		}
	}
	rt.record(c, name, rt.in.encode(v, ir.Pos{}))
	if seen != "" {
		rt.seen[seen] = true
	}
	if name == approvalEvent {
		rt.checkAnswer(v)
	}
	rt.queue = append(rt.queue, started...)
	rt.wake(name)
}

// all returns snapshots of every entity of a type; while a view is checked
// for one entity, just that one.
func (rt *runtime) all(e *ir.Entity, at ir.Pos) *List {
	if s := rt.in.scope; s != nil && s.entity == e {
		return &List{Elems: []Value{s.snapshot}}
	}
	rt.crossed("entity.all")
	l := &List{}
	for _, key := range rt.store.Keys(entityName(e)) {
		row, _ := rt.store.Load(key)
		if row.Ended {
			continue
		}
		l.Elems = append(l.Elems, snapshotOf(rt.decodeRow(e, row, at), row.Version))
	}
	return l
}

// ---------------------------------------------------------------- views

// viewScope limits Entity.all to one entity while a view is checked for it.
type viewScope struct {
	entity   *ir.Entity
	snapshot *Record
}

// viewMembers is a view read as a value: the snapshots of the entities in it
// now, a List(Entity) as the checker types it. Inside another view's check,
// Entity.all still means the one entity being checked.
func (rt *runtime) viewMembers(v *ir.View, at ir.Pos) *List {
	rt.crossed("view.read")
	result, _ := rt.in.callFunc(v.Func, nil, nil, nil, at)
	switch r := result.(type) {
	case *List:
		return r
	case *EntitySet:
		return rt.all(r.Entity, at)
	case *Set:
		return &List{Elems: r.Elems}
	}
	rt.in.faultf(v.Pos, "a view must give a collection of %s, and this one gives %s", v.Entity.Name, kindName(result))
	return nil
}

// updateViews checks the views over an entity that changed, and starts the
// jobs triggered by it entering or leaving them. A nil state is an entity
// that ended, which is in no view.
func (rt *runtime) updateViews(ref *EntityRef, state *Record, version int64) {
	key := entityKey(ref)
	for _, m := range rt.in.prog.Modules {
		for _, v := range sortedViews(m) {
			if v.Entity != ref.Entity {
				continue
			}
			member := state != nil && rt.inView(v, snapshotOf(state, version))
			if rt.members[v] == nil {
				rt.members[v] = map[string]bool{}
			}
			was := rt.members[v][key]
			rt.members[v][key] = member
			if member == was {
				continue
			}
			kind := "leaves"
			if member {
				kind = "enters"
			}
			for _, job := range rt.triggered(v, kind) {
				rt.startNow(job, []Value{ref}, nil, job.Pos)
			}
		}
	}
}

func sortedViews(m *ir.Module) []*ir.View {
	var vs []*ir.View
	for _, v := range m.Views {
		vs = append(vs, v)
	}
	sort.Slice(vs, func(i, j int) bool { return vs[i].Name < vs[j].Name })
	return vs
}

func (rt *runtime) triggered(v *ir.View, kind string) []*ir.Job {
	var jobs []*ir.Job
	for _, m := range rt.in.prog.Modules {
		for _, j := range m.Jobs {
			for _, t := range j.Triggers {
				if t.View == v && t.Kind == kind {
					jobs = append(jobs, j)
				}
			}
		}
	}
	sort.Slice(jobs, func(i, k int) bool { return jobs[i].Name < jobs[k].Name })
	return jobs
}

func (rt *runtime) inView(v *ir.View, snapshot *Record) bool {
	rt.crossed("view.check")
	prev := rt.in.scope
	rt.in.scope = &viewScope{entity: v.Entity, snapshot: snapshot}
	defer func() { rt.in.scope = prev }()
	result, _ := rt.in.callFunc(v.Func, nil, nil, nil, v.Pos)
	switch r := result.(type) {
	case *List:
		return len(r.Elems) > 0
	case *EntitySet:
		return true
	case *Set:
		return len(r.Elems) > 0
	}
	rt.in.faultf(v.Pos, "a view must give a collection of %s, and this one gives %s", v.Entity.Name, kindName(result))
	return false
}

// ---------------------------------------------------------------- jobs

// jobRun is a run of a job, saved as JSON with its values encoded. A run
// resumes from its checkpoint, or from the start if it has none: each
// durable step it took since then returns its recorded result instead of
// happening again (checkpoint.go).
type jobRun struct {
	ID         int64
	Job        string
	Args       [][]byte
	Checkpoint *checkpoint `json:",omitempty"`
	Log        []step      // the steps since the checkpoint
	Status     string      // ready, sleeping, done, failed
	WakeAt     int64       `json:",omitempty"` // Unix nanoseconds, while sleeping
	// WaitEvent is the event a sleeping run waits for in next!; one wakes it.
	WaitEvent string `json:",omitempty"`
	// Since is the sequence number of the last event published before the
	// run started.
	Since  int64  `json:",omitempty"`
	Result []byte `json:",omitempty"`
	Fault  string `json:",omitempty"`
	// FaultKind and FaultArg say which of the prelude's Fault variants the
	// run ended with; empty is Unreachable(Fault).
	FaultKind string `json:",omitempty"`
	FaultArg  string `json:",omitempty"`
	// When the run started and finished, in Unix nanoseconds.
	Started int64 `json:",omitempty"`
	Ended   int64 `json:",omitempty"`
	// A run that finished long ago keeps only a summary: how many steps it
	// took instead of the steps.
	Summarized bool `json:",omitempty"`
	Steps      int  `json:",omitempty"`
	// Spent is what the summarized steps' model calls spent.
	Spent *spend `json:",omitempty"`
	// Key is the key the run was started with: a job starts once per key.
	Key string `json:",omitempty"`
	// Group and RateGroup are the values of the job's @concurrency and
	// @limit by: parameters, which the limits count separately.
	Group     string `json:",omitempty"`
	RateGroup string `json:",omitempty"`
	// Begun is when the run got past @concurrency and @limit, in Unix
	// nanoseconds; 0 while it waits for them.
	Begun int64 `json:",omitempty"`
	// DeadlineAt is when the run ends as Failed(DeadlinePassed), from the
	// job's @deadline.
	DeadlineAt int64 `json:",omitempty"`
	// Locks are the locks the run holds inside lock!, which end it as
	// Failed(LockExpired) if it is still waiting there when they run out.
	Locks []heldLock `json:",omitempty"`
	// Trail is the last few steps a checkpoint dropped, kept for eye runs.
	// It comes last, where the journal puts it back when it writes only the
	// steps a trail gained (journal_line.go).
	Trail  []step `json:",omitempty"`
	cursor int
}

// step is a durable step: a recorded result, a timer still running, or a
// wait for an event. Seq is the number of events published when it was
// taken, so that next! after it sees every event published since.
type step struct {
	Call   string `json:",omitempty"` // what the step called, for eye runs: "notify.send", "sleep"
	Timer  bool   `json:",omitempty"`
	WakeAt int64  `json:",omitempty"`
	Value  []byte `json:",omitempty"`
	Seq    int64  `json:",omitempty"`
	Event  string `json:",omitempty"` // next! is waiting for this event
	Since  int64  `json:",omitempty"` // and looks at events after this one
	// While approval! waits: the request's id, which an answer names, and
	// whom it asks, as text. Steps saved before 2026-10-12 have neither.
	Request string `json:",omitempty"`
	From    string `json:",omitempty"`
	// What the step's model calls spent, which counts against @budget
	// again when the run is replayed: all their tokens, those of the
	// prompts and of the replies (not kept by runtimes before 2026-10-11),
	// and their price.
	Tokens    int64   `json:",omitempty"`
	TokensIn  int64   `json:",omitempty"`
	TokensOut int64   `json:",omitempty"`
	USD       float64 `json:",omitempty"`
	// The models the step asked, and the bytes of prompts sent to them and
	// of replies, for eye runs.
	Model          string `json:",omitempty"`
	Options        string `json:",omitempty"` // how the calls were sent
	Sent, Received int64  `json:",omitempty"`
}

func (s step) spent() spend {
	return spend{Tokens: s.Tokens, In: s.TokensIn, Out: s.TokensOut, USD: s.USD, Model: s.Model, Options: s.Options, Sent: s.Sent, Received: s.Received}
}

// taken is how many steps the run has taken.
func (r *jobRun) taken() int {
	if r.Summarized {
		return r.Steps
	}
	n := len(r.Log)
	if r.Checkpoint != nil {
		n += r.Checkpoint.Steps
	}
	return n
}

// history is the steps eye runs shows: the trail a checkpoint kept, then
// the steps since. The first is step taken() - len(history()) + 1.
func (r *jobRun) history() []step {
	return append(append([]step(nil), r.Trail...), r.Log...)
}

func (r *jobRun) encode() []byte {
	data, err := json.Marshal(r)
	if err != nil {
		panic(err)
	}
	return data
}

func decodeRun(data []byte) *jobRun {
	var r jobRun
	if err := json.Unmarshal(data, &r); err != nil {
		panic(err)
	}
	return &r
}

func jobName(j *ir.Job) string { return j.Module.Path + "." + j.Name }

func (rt *runtime) newRun(job *ir.Job, args []Value, named map[string]Value, at ir.Pos) *jobRun {
	run := &jobRun{ID: rt.nextRun, Job: jobName(job), Status: "ready", Since: rt.seq(), Started: rt.now.UnixNano()}
	rt.nextRun++
	for _, a := range args {
		run.Args = append(run.Args, rt.in.encode(a, at))
	}
	if after, ok := named["after"].(ir.Duration); ok {
		run.Status, run.WakeAt = "sleeping", rt.now.Add(time.Duration(after)).UnixNano()
	}
	if when, ok := named["at"].(time.Time); ok {
		run.Status, run.WakeAt = "sleeping", when.UnixNano()
	}
	if len(args) != len(job.Func.Params) {
		rt.in.faultf(at, "%s takes %d arguments", job.Name, len(job.Func.Params))
	}
	if k, ok := named["key"]; ok {
		run.Key = show(k)
	}
	group := func(by int) string {
		if by < 0 || by >= len(args) {
			return ""
		}
		return keyOf(args[by])
	}
	if c := job.Concurrency; c != nil {
		run.Group = group(c.By)
	}
	if l := job.Limit; l != nil {
		run.RateGroup = group(l.By)
	}
	if job.Deadline != nil {
		switch d := rt.in.evalGlobalFunc(job.Deadline).(type) {
		case ir.Duration:
			run.DeadlineAt = rt.now.Add(time.Duration(d)).UnixNano()
		case time.Time:
			run.DeadlineAt = d.UnixNano()
		}
	}
	return run
}

// keyed returns the run a job was already started with under a key, if
// any: a job starts once per key, so main can start one on every start of
// eye run, and a repeated event or command starts nothing new.
func (rt *runtime) keyed(job *ir.Job, named map[string]Value) (Value, bool) {
	k, ok := named["key"]
	if !ok {
		return nil, false
	}
	key := show(k)
	if id, ok := rt.index.keys[jobName(job)+"/"+key]; ok {
		return &RunVal{ID: id}, true
	}
	if rt.tx != nil {
		for _, run := range rt.tx.starts {
			if run.Job == jobName(job) && run.Key == key {
				return &RunVal{ID: run.ID}, true
			}
		}
	}
	return nil, false
}

// start starts a job: when the current transaction commits, or now.
func (rt *runtime) start(job *ir.Job, args []Value, named map[string]Value, at ir.Pos) Value {
	rt.crossed("job.start")
	if run, ok := rt.keyed(job, named); ok {
		return run
	}
	if rt.tx != nil {
		run := rt.newRun(job, args, named, at)
		rt.tx.starts = append(rt.tx.starts, run)
		return &RunVal{ID: run.ID}
	}
	return rt.startNow(job, args, named, at)
}

func (rt *runtime) startNow(job *ir.Job, args []Value, named map[string]Value, at ir.Pos) Value {
	if run, ok := rt.keyed(job, named); ok {
		return run
	}
	run := rt.newRun(job, args, named, at)
	rt.store.Commit(&Commit{Runs: []RunWrite{{ID: run.ID, Data: run.encode()}}})
	if run.Status == "ready" {
		rt.queue = append(rt.queue, run.ID)
	}
	rt.drain()
	return &RunVal{ID: run.ID}
}

// drain runs ready runs until each finishes or waits.
func (rt *runtime) drain() {
	if rt.draining || rt.tx != nil || rt.in.run != nil {
		return
	}
	rt.draining = true
	defer func() { rt.draining = false }()
	for len(rt.queue) > 0 {
		id := rt.queue[0]
		rt.queue = rt.queue[1:]
		rt.step(id)
	}
}

// step runs a job run from its start, replaying its recorded steps, until it
// finishes, fails or waits.
func (rt *runtime) step(id int64) {
	data, ok := rt.store.Run(id)
	if !ok {
		return
	}
	run := decodeRun(data)
	if run.Status != "ready" {
		return
	}
	if rt.real {
		rt.now = time.Now()
	}
	job := rt.in.jobs[run.Job]
	if rt.crossings != nil {
		defer rt.crossings.enterRun(run.ID, run.Job)()
	}
	var args []argVal
	for _, a := range run.Args {
		args = append(args, argVal{v: rt.in.decode(a, job.Pos)})
	}
	var failed *CommitError
	if run.Begun == 0 && !rt.admit(run, job) {
		// It waits for a place or for the rate to allow it.
		if err := guard(func() { rt.save(run) }); err != nil {
			rt.blocked(id, err.(*CommitError))
		}
		return
	}
	rt.crossed("run.step")
	func() {
		rt.in.run = run
		defer func() {
			rt.in.run = nil
			if r := recover(); r != nil {
				switch x := r.(type) {
				case parkSignal:
					run.Status = "sleeping"
					if run.DeadlineAt > 0 && (run.WakeAt == 0 || run.WakeAt > run.DeadlineAt) {
						run.WakeAt = run.DeadlineAt // to end it then
					}
					for _, h := range run.Locks {
						if run.WakeAt == 0 || run.WakeAt > h.Until {
							run.WakeAt = h.Until // to end it then, if it still holds the lock
						}
					}
				case *Fault:
					run.Status = "failed"
					run.Fault = x.Error()
					run.FaultKind, run.FaultArg = x.Kind, x.Arg
					run.Ended = rt.now.UnixNano()
					if x.Kind == "" {
						// A bug, not an outcome the program asked for (a
						// deadline, a budget): it fails the test it ran in.
						rt.failures = append(rt.failures, &Fault{Pos: x.Pos, Msg: "job " + job.Name + ": " + x.Msg})
					}
				case *CommitError:
					failed = x
				default:
					panic(r)
				}
			}
		}()
		if run.DeadlineAt > 0 && rt.now.UnixNano() >= run.DeadlineAt {
			runFault(job.Pos, "DeadlinePassed", "%s passed its @deadline", job.Name)
		}
		rt.lockExpired(run, job)
		rt.in.admitMemory(run, job)
		result := rt.in.runJob(run, job, args)
		run.Status = "done"
		run.Result = rt.in.encode(result, job.Pos)
		run.Ended = rt.now.UnixNano()
	}()
	if failed == nil {
		// A finished run's outcome is an event, saved with it: runs that
		// join this one wake to it, even after a restart.
		c := &Commit{Runs: []RunWrite{{ID: run.ID, Data: run.encode()}}}
		var after func()
		if ended(run) {
			after = rt.ending(run, c)
		}
		if err := guard(func() { rt.store.Commit(c) }); err != nil {
			failed = err.(*CommitError)
		} else if after != nil {
			after()
		}
		rt.warnLong(run)
	}
	if failed != nil {
		// The store still holds the run at its last checkpoint.
		rt.blocked(id, failed)
	}
}

// admit lets a run begin if the job's @concurrency and @limit allow it. If
// not, the run is queued for a place, or sleeps until the rate allows it.
func (rt *runtime) admit(run *jobRun, job *ir.Job) bool {
	name := jobName(job)
	if c := job.Concurrency; c != nil && rt.index.running(name, run.Group) >= c.N {
		run.Status = "queued"
		return false
	}
	if l := job.Limit; l != nil {
		window := rt.rate[name+"/"+run.RateGroup]
		since := rt.now.Add(-time.Duration(l.Per)).UnixNano()
		for len(window) > 0 && window[0] <= since {
			window = window[1:]
		}
		rt.rate[name+"/"+run.RateGroup] = window
		if len(window) >= l.N {
			run.Status = "sleeping"
			run.WakeAt = window[len(window)-l.N] + int64(l.Per)
			return false
		}
	}
	run.Begun = rt.now.UnixNano()
	if job.Limit != nil {
		key := name + "/" + run.RateGroup
		rt.rate[key] = append(rt.rate[key], run.Begun)
	}
	return true
}

// release gives a finished run's place to the earliest run of its group
// that waits for one.
func (rt *runtime) release(job *ir.Job, finished *jobRun) {
	if job.Concurrency == nil || (finished.Status != "done" && finished.Status != "failed") {
		return
	}
	id, ok := rt.index.firstQueued(jobName(job), finished.Group)
	if !ok {
		return
	}
	data, _ := rt.store.Run(id)
	next := decodeRun(data)
	next.Status = "ready"
	rt.save(next)
	rt.queue = append(rt.queue, id)
}

// fillPlaces starts queued runs where their group has a free place: after a
// restart, a run that finished just before the stop may not have passed its
// place on.
func (rt *runtime) fillPlaces() {
	for id, r := range rt.index.open {
		name := rt.index.names[r.job]
		job := rt.in.jobs[name]
		if !r.queued || job == nil || job.Concurrency == nil {
			continue
		}
		if rt.index.running(name, r.group) >= job.Concurrency.N {
			continue
		}
		if first, ok := rt.index.firstQueued(name, r.group); !ok || first != id {
			continue
		}
		data, _ := rt.store.Run(id)
		next := decodeRun(data)
		next.Status = "ready"
		rt.save(next)
		rt.queue = append(rt.queue, id)
	}
}

// save writes a run after it recorded a step.
// It returns the size of the run as saved.
func (rt *runtime) save(run *jobRun) int {
	data := run.encode()
	rt.store.Commit(&Commit{Runs: []RunWrite{{ID: run.ID, Data: data}}})
	rt.warnLong(run)
	return len(data)
}

// advance moves a test's clock forward, waking each sleeping run when its
// time comes.
func (rt *runtime) advance(d ir.Duration) {
	target := rt.now.Add(time.Duration(d))
	rt.schedules() // schedules count from the clock as it was before it moved
	for rt.wakeNext(target) {
	}
	rt.now = target
}

// wakeNext runs what comes first, up to target: a sleeping run whose time
// has come, or a schedule's firing. The clock moves to that time. It reports
// whether there was one.
func (rt *runtime) wakeNext(target time.Time) bool {
	id, wakeAt, sleeping := rt.index.firstToWake()
	s, firing := rt.nextFiring()
	runDue := sleeping && wakeAt <= target.UnixNano()
	fireDue := s != nil && !firing.After(target)
	if runDue && fireDue && firing.UnixNano() < wakeAt {
		runDue = false
	}
	switch {
	case runDue:
		// Only now is the run's record read.
		data, _ := rt.store.Run(id)
		run := decodeRun(data)
		if wake := time.Unix(0, run.WakeAt).In(rt.now.Location()); wake.After(rt.now) {
			rt.now = wake
		}
		run.Status = "ready"
		rt.save(run)
		rt.queue = append(rt.queue, run.ID)
	case fireDue:
		if firing.After(rt.now) {
			rt.now = firing.In(rt.now.Location())
		}
		rt.fire(s, firing)
	default:
		return false
	}
	rt.drain()
	return true
}

// nextWake returns when the sleeping run that wakes first wakes, if any,
// from its handle: its record stays where it is.
func (rt *runtime) nextWake() (int64, bool) {
	_, wakeAt, ok := rt.index.firstToWake()
	return wakeAt, ok
}
