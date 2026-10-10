package interp

import (
	"encoding/json"
	"sort"
	"strings"
)

// Store keeps entities, job runs and events as bytes. Programs don't change
// when the store does: tests use the in-memory store, and `eye run` a
// journal (OpenJournal) that survives restarts.
type Store interface {
	// Load returns an entity's saved state and version.
	Load(key string) (state []byte, version int64, ok bool)
	// Keys lists the saved entities of one entity type, oldest first.
	Keys(entity string) []string
	// Run returns a saved job run.
	Run(id int64) ([]byte, bool)
	// Runs lists the saved runs, oldest first.
	Runs() []int64
	// Events lists the published events, oldest first.
	Events() []EventRecord
	// Meta returns what the runtime keeps besides entities, runs and
	// events: when each schedule last fired, the event keys already seen,
	// the commands whose reply was lost.
	Meta(key string) ([]byte, bool)
	// MetaKeys lists the meta keys that start with prefix, in order.
	MetaKeys(prefix string) []string
	// Commit applies a transaction's writes all at once.
	Commit(c *Commit)
}

// Commit is what one transaction writes.
type Commit struct {
	Writes []Write       `json:",omitempty"`
	Runs   []RunWrite    `json:",omitempty"`
	Events []EventRecord `json:",omitempty"`
	Meta   []MetaWrite   `json:",omitempty"`
	// DropEvents drops this many of the oldest events, which no run can
	// still wait for.
	DropEvents int `json:",omitempty"`
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

// EventRecord is a published event: its name and its value as bytes.
type EventRecord struct {
	Name string
	Data []byte
}

// MetaWrite sets a meta key, or removes it when Data is nil.
type MetaWrite struct {
	Key  string
	Data []byte `json:",omitempty"`
}

// ---------------------------------------------------------------- the index of waiting runs

// indexed is a store with an index of the runs that wait, so that finding
// the next one to wake doesn't read every run the store has kept.
type indexed struct {
	Store
	waiting map[int64]waitingRun
	// open holds the unfinished runs, for @concurrency: those that have
	// begun (ready or sleeping), and those queued for a free place.
	open map[int64]openRun
	// keys finds a run by the key it was started with: job/key → run.
	keys map[string]int64
}

type openRun struct {
	job    string
	group  string // the value of the job's @concurrency by: parameter
	queued bool
	begun  bool // past @concurrency and @limit
}

type waitingRun struct {
	wakeAt int64
	event  string
}

func indexStore(s Store) *indexed {
	ix := &indexed{Store: s, waiting: map[int64]waitingRun{}, open: map[int64]openRun{}, keys: map[string]int64{}}
	for _, id := range s.Runs() {
		data, _ := s.Run(id)
		ix.note(id, data)
	}
	return ix
}

func (ix *indexed) Commit(c *Commit) {
	ix.Store.Commit(c)
	for _, r := range c.Runs {
		ix.note(r.ID, r.Data)
	}
}

func (ix *indexed) note(id int64, data []byte) {
	var r struct {
		Job       string
		Status    string
		WakeAt    int64
		WaitEvent string
		Group     string
		Key       string
		Begun     int64
	}
	if json.Unmarshal(data, &r) != nil {
		return
	}
	if r.Status == "sleeping" {
		ix.waiting[id] = waitingRun{r.WakeAt, r.WaitEvent}
	} else {
		delete(ix.waiting, id)
	}
	switch r.Status {
	case "ready", "sleeping", "queued":
		ix.open[id] = openRun{job: r.Job, group: r.Group, queued: r.Status == "queued", begun: r.Begun > 0}
	default:
		delete(ix.open, id)
	}
	if r.Key != "" {
		ix.keys[r.Job+"/"+r.Key] = id
	}
}

// running counts a job's unfinished runs in a group that have begun.
func (ix *indexed) running(job, group string) int {
	n := 0
	for _, r := range ix.open {
		if r.job == job && r.group == group && r.begun {
			n++
		}
	}
	return n
}

// firstQueued returns the earliest run of a job's group that waits for a
// place, if any.
func (ix *indexed) firstQueued(job, group string) (int64, bool) {
	first := int64(-1)
	for id, r := range ix.open {
		if r.job == job && r.group == group && r.queued && (first < 0 || id < first) {
			first = id
		}
	}
	return first, first >= 0
}

// sleeping lists the waiting runs, in the order they were started.
func (ix *indexed) sleeping() []int64 {
	ids := make([]int64, 0, len(ix.waiting))
	for id := range ix.waiting {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// ---------------------------------------------------------------- the in-memory store

type memStore struct {
	rows   map[string]memRow
	kinds  map[string][]string
	runs   map[int64][]byte
	order  []int64
	events []EventRecord
	meta   map[string][]byte
}

type memRow struct {
	state   []byte
	version int64
}

func newMemStore() *memStore {
	return &memStore{rows: map[string]memRow{}, kinds: map[string][]string{}, runs: map[int64][]byte{}, meta: map[string][]byte{}}
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

func (s *memStore) Events() []EventRecord { return s.events }

func (s *memStore) Meta(key string) ([]byte, bool) {
	d, ok := s.meta[key]
	return d, ok
}

func (s *memStore) MetaKeys(prefix string) []string {
	var keys []string
	for k := range s.meta {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

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
	s.events = append(s.events[min(c.DropEvents, len(s.events)):], c.Events...)
	for _, m := range c.Meta {
		if m.Data == nil {
			delete(s.meta, m.Key)
		} else {
			s.meta[m.Key] = m.Data
		}
	}
}

// snapshot is everything the store holds, as one commit.
func (s *memStore) snapshot() *Commit {
	c := &Commit{Events: s.events}
	var entities []string
	for e := range s.kinds {
		entities = append(entities, e)
	}
	sort.Strings(entities)
	for _, e := range entities {
		for _, key := range s.kinds[e] {
			r := s.rows[key]
			c.Writes = append(c.Writes, Write{Key: key, Entity: e, State: r.state, Version: r.version})
		}
	}
	for _, id := range s.order {
		c.Runs = append(c.Runs, RunWrite{ID: id, Data: s.runs[id]})
	}
	for _, k := range s.MetaKeys("") {
		c.Meta = append(c.Meta, MetaWrite{Key: k, Data: s.meta[k]})
	}
	return c
}
