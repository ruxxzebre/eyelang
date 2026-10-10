package interp

import (
	"encoding/json"
	"sort"
	"time"

	"eyelang/ir"
)

// Store keeps entities and job runs as bytes. Programs don't change when the
// store does: the in-memory store is the default, and a durable store
// implements the same interface.
type Store interface {
	// Load returns an entity's saved state and version.
	Load(key string) (state []byte, version int64, ok bool)
	// Keys lists the saved entities of one entity type, oldest first.
	Keys(entity string) []string
	// Run returns a saved job run.
	Run(id int64) ([]byte, bool)
	// Runs lists the saved runs, oldest first.
	Runs() []int64
	// Commit applies a transaction's writes all at once.
	Commit(c *Commit)
}

// Commit is what one transaction writes.
type Commit struct {
	Writes []Write
	Runs   []RunWrite
}

// Write saves an entity's state.
type Write struct {
	Key, Entity string
	State       []byte
	Version     int64
}

// RunWrite saves a job run.
type RunWrite struct {
	ID   int64
	Data []byte
}

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
	// uncertain holds the commands whose reply was lost, by key.
	uncertain map[string]sentCommand
}

// event is a published event, saved as bytes.
type event struct {
	name string
	data []byte
}

func eventName(e *ir.Event) string { return e.Contract.Module.Path + "." + e.Name }

func newRuntime(in *Interp) *runtime {
	return &runtime{
		in: in, store: newMemStore(), now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		members: map[*ir.View]map[string]bool{}, fakes: map[*ir.Contract]*fakeSet{}, nextRun: 1,
		seen: map[string]bool{}, uncertain: map[string]sentCommand{},
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
}

// load returns an entity's committed state, or its default state if it has
// never been saved.
func (rt *runtime) load(ref *EntityRef, at ir.Pos) (*Record, int64, bool) {
	data, version, ok := rt.store.Load(entityKey(ref))
	if ok {
		return rt.in.decode(data, at).(*Record), version, true
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
		c.Writes = append(c.Writes, Write{Key: key, Entity: entityName(r.ref.Entity), State: rt.in.encode(r.state, ir.Pos{}), Version: r.version})
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
	rt.events = append(rt.events, event{name: approvalEvent, data: rt.in.encode(v, ir.Pos{})})
	rt.wake(approvalEvent)
	rt.drain()
}

// wake makes ready the runs waiting for an event named name.
func (rt *runtime) wake(name string) {
	for _, id := range rt.store.Runs() {
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
	if e.Key != "" {
		if r, ok := v.(*Record); ok {
			if k, ok := r.get(e.Key); ok {
				id := name + "/" + keyOf(k)
				if rt.seen[id] {
					return
				}
				rt.seen[id] = true
			}
		}
	}
	rt.events = append(rt.events, event{name: name, data: rt.in.encode(v, ir.Pos{})})
	for _, m := range rt.in.prog.Modules {
		for _, j := range m.Jobs {
			for _, t := range j.Triggers {
				if t.Event == e {
					run := rt.newRun(j, []Value{v}, nil, j.Pos)
					rt.store.Commit(&Commit{Runs: []RunWrite{{ID: run.ID, Data: run.encode()}}})
					rt.queue = append(rt.queue, run.ID)
				}
			}
		}
	}
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
		data, version, _ := rt.store.Load(key)
		l.Elems = append(l.Elems, snapshotOf(rt.in.decode(data, at).(*Record), version))
	}
	return l
}

// ---------------------------------------------------------------- views

// viewScope limits Entity.all to one entity while a view is checked for it.
type viewScope struct {
	entity   *ir.Entity
	snapshot *Record
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
// resumes by running the job again from the start: each durable step it
// already took returns its recorded result instead of happening again.
type jobRun struct {
	ID     int64
	Job    string
	Args   [][]byte
	Log    []step
	Status string // ready, sleeping, done, failed
	WakeAt int64  `json:",omitempty"` // Unix nanoseconds, while sleeping
	// WaitEvent is the event a sleeping run waits for in next!; one wakes it.
	WaitEvent string `json:",omitempty"`
	// Since is the sequence number of the last event published before the
	// run started.
	Since  int64  `json:",omitempty"`
	Result []byte `json:",omitempty"`
	Fault  string `json:",omitempty"`
	cursor int
}

// step is a durable step: a recorded result, a timer still running, or a
// wait for an event. Seq is the number of events published when it was
// taken, so that next! after it sees every event published since.
type step struct {
	Timer  bool   `json:",omitempty"`
	WakeAt int64  `json:",omitempty"`
	Value  []byte `json:",omitempty"`
	Seq    int64  `json:",omitempty"`
	Event  string `json:",omitempty"` // next! is waiting for this event
	Since  int64  `json:",omitempty"` // and looks at events after this one
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
	run := &jobRun{ID: rt.nextRun, Job: jobName(job), Status: "ready", Since: int64(len(rt.events))}
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
	return run
}

// start starts a job: when the current transaction commits, or now.
func (rt *runtime) start(job *ir.Job, args []Value, named map[string]Value, at ir.Pos) Value {
	if rt.tx != nil {
		run := rt.newRun(job, args, named, at)
		rt.tx.starts = append(rt.tx.starts, run)
		return &RunVal{ID: run.ID}
	}
	return rt.startNow(job, args, named, at)
}

func (rt *runtime) startNow(job *ir.Job, args []Value, named map[string]Value, at ir.Pos) Value {
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
	job := rt.in.jobs[run.Job]
	var args []argVal
	for _, a := range run.Args {
		args = append(args, argVal{v: rt.in.decode(a, job.Pos)})
	}
	func() {
		rt.in.run = run
		defer func() {
			rt.in.run = nil
			if r := recover(); r != nil {
				switch x := r.(type) {
				case parkSignal:
					run.Status = "sleeping"
				case *Fault:
					run.Status = "failed"
					run.Fault = x.Error()
					rt.failures = append(rt.failures, &Fault{Pos: x.Pos, Msg: "job " + job.Name + ": " + x.Msg})
				default:
					panic(r)
				}
			}
		}()
		result, _, _ := rt.in.callFunc(job.Func, nil, nil, args, job.Pos)
		run.Status = "done"
		run.Result = rt.in.encode(result, job.Pos)
	}()
	rt.store.Commit(&Commit{Runs: []RunWrite{{ID: run.ID, Data: run.encode()}}})
}

// save writes a run after it recorded a step.
func (rt *runtime) save(run *jobRun) {
	rt.store.Commit(&Commit{Runs: []RunWrite{{ID: run.ID, Data: run.encode()}}})
}

// advance moves a test's clock forward, waking each sleeping run when its
// time comes.
func (rt *runtime) advance(d ir.Duration) {
	target := rt.now.Add(time.Duration(d))
	for {
		var next *jobRun
		for _, id := range rt.store.Runs() {
			data, _ := rt.store.Run(id)
			run := decodeRun(data)
			if run.Status != "sleeping" || run.WakeAt > target.UnixNano() {
				continue
			}
			if next == nil || run.WakeAt < next.WakeAt {
				next = run
			}
		}
		if next == nil {
			break
		}
		if wake := time.Unix(0, next.WakeAt).In(rt.now.Location()); wake.After(rt.now) {
			rt.now = wake
		}
		next.Status = "ready"
		rt.save(next)
		rt.queue = append(rt.queue, next.ID)
		rt.drain()
	}
	rt.now = target
}

// ---------------------------------------------------------------- the in-memory store

type memStore struct {
	rows  map[string]memRow
	kinds map[string][]string
	runs  map[int64][]byte
	order []int64
}

type memRow struct {
	state   []byte
	version int64
}

func newMemStore() *memStore {
	return &memStore{rows: map[string]memRow{}, kinds: map[string][]string{}, runs: map[int64][]byte{}}
}

func (s *memStore) Load(key string) ([]byte, int64, bool) {
	r, ok := s.rows[key]
	return r.state, r.version, ok
}

func (s *memStore) Keys(entity string) []string { return s.kinds[entity] }

func (s *memStore) Run(id int64) ([]byte, bool) {
	d, ok := s.runs[id]
	return d, ok
}

func (s *memStore) Runs() []int64 { return s.order }

func (s *memStore) Commit(c *Commit) {
	for _, w := range c.Writes {
		if _, ok := s.rows[w.Key]; !ok {
			s.kinds[w.Entity] = append(s.kinds[w.Entity], w.Key)
		}
		s.rows[w.Key] = memRow{state: w.State, version: w.Version}
	}
	for _, r := range c.Runs {
		if _, ok := s.runs[r.ID]; !ok {
			s.order = append(s.order, r.ID)
		}
		s.runs[r.ID] = r.Data
	}
}
