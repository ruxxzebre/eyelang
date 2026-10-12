package interp

import (
	"slices"
	"sort"
	"strings"
)

// Store keeps entities, job runs and events as bytes. Programs don't change
// when the store does: tests use the in-memory store, and `eye run` a
// journal (OpenJournal) that survives restarts.
type Store interface {
	// Load returns an entity's saved row.
	Load(key string) (Row, bool)
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

// Row is an entity as saved: its state, how many transactions have changed
// it, and the @version of the entity's declaration it was saved with.
type Row struct {
	State   []byte
	Version int64
	Schema  int
	// Ended marks what is left of an entity that `end(ok)` ended: State
	// says when, and whose approval ended it (endRecord).
	Ended bool
}

// Write saves an entity's state.
type Write struct {
	Key, Entity string
	State       []byte
	Version     int64
	// Schema is the entity's @version when the state was written; 0, in
	// journals written before entities had versions, is 1.
	Schema int `json:",omitempty"`
	// Ended says `end(ok)` ended the entity: State is then an endRecord,
	// and the row is kept so that the identity stays ended.
	Ended bool `json:",omitempty"`
}

// RunWrite saves a job run.
type RunWrite struct {
	ID   int64
	Data []byte
	// summary is what the runtime keeps of the run in memory, worked out
	// from Data by the index (handles.go); nil if no index saw the commit.
	summary *runSummary
}

// summaryOfWrite is r's summary, worked out now if no index has.
func summaryOfWrite(r RunWrite) (runSummary, bool) {
	if r.summary != nil {
		return *r.summary, true
	}
	return summaryOf(r.Data)
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

// ---------------------------------------------------------------- the Go in-memory store

// memStore is a store written in Go. The runtime keeps a program's state in
// the store written in Eyelang (eyeStore); this one reads a journal before
// there is a program to run it, for `eye runs` and while a journal opens.

type memStore struct {
	rows   map[string]memRow
	kinds  map[string][]string
	runs   map[int64][]byte
	events []EventRecord
	meta   map[string][]byte
}

type memRow = Row

func newMemStore() *memStore {
	return &memStore{rows: map[string]memRow{}, kinds: map[string][]string{}, runs: map[int64][]byte{}, meta: map[string][]byte{}}
}

func (s *memStore) Load(key string) (Row, bool) {
	r, ok := s.rows[key]
	return r, ok
}

func (s *memStore) Keys(entity string) []string { return s.kinds[entity] }

func (s *memStore) Run(id int64) ([]byte, bool) {
	d, ok := s.runs[id]
	return d, ok
}

// Runs lists the runs by number, which is the order they started in.
func (s *memStore) Runs() []int64 {
	ids := make([]int64, 0, len(s.runs))
	for id := range s.runs {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// forget drops the records of runs ids, which a journal keeps on disk.
func (s *memStore) forget(ids []int64) {
	for _, id := range ids {
		delete(s.runs, id)
	}
}

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
		s.rows[w.Key] = Row{State: w.State, Version: w.Version, Schema: w.Schema, Ended: w.Ended}
	}
	for _, r := range c.Runs {
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
			c.Writes = append(c.Writes, Write{Key: key, Entity: e, State: r.State, Version: r.Version, Schema: r.Schema, Ended: r.Ended})
		}
	}
	for _, id := range s.Runs() {
		c.Runs = append(c.Runs, RunWrite{ID: id, Data: s.runs[id]})
	}
	for _, k := range s.MetaKeys("") {
		c.Meta = append(c.Meta, MetaWrite{Key: k, Data: s.meta[k]})
	}
	return c
}
