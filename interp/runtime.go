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
	// fastRetries makes retries of unreachable services immediate, in tests.
	fastRetries bool
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
	switch d := s.(type) {
	case *Journal:
		mem = &d.memory
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
	rt.index = indexStore(s)
	rt.store = rt.index
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
}

// load returns an entity's committed state, or its default state if it has
// never been saved.
func (rt *runtime) load(ref *EntityRef, at ir.Pos) (*Record, int64, bool) {
	row, ok := rt.store.Load(entityKey(ref))
	if ok {
		return rt.decodeRow(ref.Entity, row, at), row.Version, true
	}
	return rt.defaults(ref, at), 0, false
}

func (rt *runtime) defaults(ref *EntityRef, at ir.Pos) *Record {
	e := ref.Entity
	r := &Record{Names: []string{e.ID.Name}, Vals: []Value{ref.ID}}
	for _, s := range e.States {
		if s.Default == nil {
			rt.in.faultf(at, "%s.%s has no default, so a new %s can't start", e.Name, s.Name, e.Name)
		}
		v, _, _ := rt.in.callFunc(s.Default, nil, nil, nil, at)
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
	if rt.tx != nil {
		if r, ok := rt.tx.rows[entityKey(ref)]; ok {
			return r.state
		}
	}
	state, _, _ := rt.load(ref, at)
	return state
}

// snapshot is what `read!()` returns: the state with its version.
func (rt *runtime) snapshot(ref *EntityRef, at ir.Pos) *Record {
	state, version, _ := rt.load(ref, at)
	return snapshotOf(state, version)
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
	prevRun := rt.in.run
	rt.in.run = nil // serving an operation never waits
	defer func() { rt.in.run = prevRun }()
	if rt.tx != nil {
		r, _, _ := rt.in.callFunc(op.Impl, nil, nil, args, at)
		return r
	}
	tx := rt.begin()
	defer func() {
		if rt.tx == tx {
			rt.tx = nil // a fault rolls the transaction back
		}
	}()
	r, _, _ := rt.in.callFunc(op.Impl, nil, nil, args, at)
	rt.tx = nil
	rt.commit(tx)
	return r
}

func (rt *runtime) callIn(tx *txn, ref *EntityRef, m *ir.Func, args []argVal, at ir.Pos) Value {
	row := tx.row(rt, ref, at)
	prevRun := rt.in.run
	rt.in.run = nil // entity methods never wait, even when a job calls them
	defer func() { rt.in.run = prevRun }()
	result, self, _ := rt.in.callFunc(m, row.state, nil, args, at)
	row.state = self.(*Record)
	return result
}

// commit saves a transaction, starts the jobs it started, and updates the
// views over the entities it changed.
func (rt *runtime) commit(tx *txn) {
	c := &Commit{}
	var changed []*txRow
	for _, key := range tx.order {
		r := tx.rows[key]
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

// answer records a person's answer to an approval request and wakes the runs
// waiting for one.
func (rt *runtime) answer(v *Record) {
	if e := rt.in.approvalsEvent("answered"); e != nil {
		// The program imports approvals, so jobs may start on an answer.
		rt.publish(e, v)
		return
	}
	rt.record(&Commit{}, approvalEvent, rt.in.encode(v, ir.Pos{}))
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
	for _, id := range rt.index.sleeping() {
		if rt.index.waiting[id].event != name {
			continue
		}
		data, _ := rt.store.Run(id)
		run := decodeRun(data)
		if run.Status == "sleeping" && run.WaitEvent == name {
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
	rt.queue = append(rt.queue, started...)
	rt.wake(name)
}

// all returns snapshots of every entity of a type; while a view is checked
// for one entity, just that one.
func (rt *runtime) all(e *ir.Entity, at ir.Pos) *List {
	if s := rt.in.scope; s != nil && s.entity == e {
		return &List{Elems: []Value{s.snapshot}}
	}
	l := &List{}
	for _, key := range rt.store.Keys(entityName(e)) {
		row, _ := rt.store.Load(key)
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
	result, _, _ := rt.in.callFunc(v.Func, nil, nil, nil, at)
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
// jobs triggered by it entering or leaving them.
func (rt *runtime) updateViews(ref *EntityRef, state *Record, version int64) {
	key := entityKey(ref)
	for _, m := range rt.in.prog.Modules {
		for _, v := range sortedViews(m) {
			if v.Entity != ref.Entity {
				continue
			}
			member := rt.inView(v, snapshotOf(state, version))
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
	prev := rt.in.scope
	rt.in.scope = &viewScope{entity: v.Entity, snapshot: snapshot}
	defer func() { rt.in.scope = prev }()
	result, _, _ := rt.in.callFunc(v.Func, nil, nil, nil, v.Pos)
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
	// Trail is the last few steps a checkpoint dropped, kept for eye runs.
	Trail  []step `json:",omitempty"`
	Status string // ready, sleeping, done, failed
	WakeAt int64  `json:",omitempty"` // Unix nanoseconds, while sleeping
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
	cursor     int
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
		var name string
		var outcome []byte
		freed := false
		if run.Status == "done" || run.Status == "failed" {
			name, outcome = runEvent(run.ID), rt.in.encode(rt.in.outcome(run, job.Pos), job.Pos)
			c.Events = []EventRecord{{Name: name, Data: outcome}}
			freed = rt.dropReserved(run, c) // a run that ended inside reserve!
		}
		if err := guard(func() { rt.store.Commit(c) }); err != nil {
			failed = err.(*CommitError)
		} else if name != "" {
			rt.events = append(rt.events, event{name: name, data: outcome})
			rt.wake(name)
			rt.release(job, run)
			if freed {
				delete(rt.reserved, run.ID)
				rt.wakeMemory()
			}
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
		job := rt.in.jobs[r.job]
		if !r.queued || job == nil || job.Concurrency == nil {
			continue
		}
		if rt.index.running(r.job, r.group) >= job.Concurrency.N {
			continue
		}
		if first, ok := rt.index.firstQueued(r.job, r.group); !ok || first != id {
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
	run := rt.nextSleeper()
	s, firing := rt.nextFiring()
	runDue := run != nil && run.WakeAt <= target.UnixNano()
	fireDue := s != nil && !firing.After(target)
	if runDue && fireDue && firing.UnixNano() < run.WakeAt {
		runDue = false
	}
	switch {
	case runDue:
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

// nextSleeper returns the sleeping run that wakes first, if any.
func (rt *runtime) nextSleeper() *jobRun {
	first := int64(-1)
	for _, id := range rt.index.sleeping() {
		if first < 0 || rt.index.waiting[id].wakeAt < rt.index.waiting[first].wakeAt {
			first = id
		}
	}
	if first < 0 {
		return nil
	}
	data, _ := rt.store.Run(first)
	return decodeRun(data)
}
