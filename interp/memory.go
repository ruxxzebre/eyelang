package interp

import (
	"strings"

	"eyelang/ir"
)

// eyeStore is the in-memory store a program's state is kept in: the
// MemoryStore written in Eyelang (check/runtime/store.eye), run by the
// interpreter. It holds the current store value, and a commit replaces it
// with the value the store's commit function returns (SPECS.md §15).
//
// The Go memStore (store.go) remains for one job only: reading a journal
// before there is a program to run, as `eye runs` does and as opening a
// journal does; the runtime then moves what it read into an eyeStore.
type eyeStore struct {
	in    *Interp
	typ   *ir.Type
	value Value // the current MemoryStore; nil until first used
}

// memory is an in-memory store a journal can keep its state in.
type memory interface {
	Store
	// snapshot is everything the store holds, as one commit.
	snapshot() *Commit
}

func newEyeStore(in *Interp) *eyeStore {
	return &eyeStore{in: in, typ: in.prog.Store.Types["MemoryStore"]}
}

// call runs one of MemoryStore's functions, apart from whatever the
// interpreter is in the middle of: a job's run, a view.
func (s *eyeStore) call(name string, self Value, args ...Value) Value {
	in := s.in
	fn := s.typ.Methods[name]
	prevRun, prevScope, prevWant := in.run, in.scope, in.want
	in.run, in.scope, in.want = nil, nil, nil
	defer func() { in.run, in.scope, in.want = prevRun, prevScope, prevWant }()
	var argv []argVal
	for _, a := range args {
		argv = append(argv, argVal{v: a})
	}
	r, _, _ := in.callFunc(fn, self, nil, argv, fn.Pos)
	return r
}

func (s *eyeStore) current() Value {
	if s.value == nil {
		s.value = s.call("empty", nil)
	}
	return s.value
}

// option returns what Some holds, or false for None.
func option(v Value) (Value, bool) {
	o, ok := v.(*Variant)
	if !ok || len(o.Vals) != 1 {
		return nil, false
	}
	return o.Vals[0], true
}

func field(r Value, name string) Value {
	v, _ := r.(*Record).get(name)
	return v
}

func toInt64(v Value) int64 {
	n, _ := v.(int64)
	return n
}

func (s *eyeStore) Load(key string) (Row, bool) {
	// An entity's key is its entity type's name, which has no "/", then
	// "/" and its identity (entityKey).
	entity, _, _ := strings.Cut(key, "/")
	r, ok := option(s.call("load", s.current(), entity, key))
	if !ok {
		return Row{}, false
	}
	return Row{
		State:   []byte(field(r, "state").(string)),
		Version: toInt64(field(r, "version")),
		Schema:  int(toInt64(field(r, "schema"))),
		Ended:   field(r, "ended") == true,
	}, true
}

func (s *eyeStore) strings(v Value) []string {
	var out []string
	for _, e := range v.(*List).Elems {
		out = append(out, e.(string))
	}
	return out
}

func (s *eyeStore) Keys(entity string) []string {
	return s.strings(s.call("keys", s.current(), entity))
}

func (s *eyeStore) Run(id int64) ([]byte, bool) {
	d, ok := option(s.call("run", s.current(), id))
	if !ok {
		return nil, false
	}
	return []byte(d.(string)), true
}

func (s *eyeStore) Runs() []int64 {
	var out []int64
	for _, e := range s.call("runs", s.current()).(*List).Elems {
		out = append(out, toInt64(e))
	}
	return out
}

func (s *eyeStore) Events() []EventRecord {
	var out []EventRecord
	for _, e := range s.call("events", s.current()).(*List).Elems {
		out = append(out, EventRecord{Name: field(e, "name").(string), Data: []byte(field(e, "data").(string))})
	}
	return out
}

func (s *eyeStore) Meta(key string) ([]byte, bool) {
	d, ok := option(s.call("meta", s.current(), key))
	if !ok {
		return nil, false
	}
	return []byte(d.(string)), true
}

func (s *eyeStore) MetaKeys(prefix string) []string {
	return s.strings(s.call("metaKeys", s.current(), prefix))
}

// Commit replaces the store with the one the commit function returns: every
// write of the commit, or, if it faults, none.
func (s *eyeStore) Commit(c *Commit) {
	types := s.typ.Module.Types
	rec := func(t string, names []string, vals ...Value) *Record {
		return &Record{Names: names, Vals: vals, Type: types[t]}
	}
	writes := &List{}
	for _, w := range c.Writes {
		writes.Elems = append(writes.Elems, rec("Write", []string{"key", "entity", "state", "version", "schema", "ended"},
			w.Key, w.Entity, string(w.State), w.Version, int64(w.Schema), w.Ended))
	}
	runs := &List{}
	for _, r := range c.Runs {
		runs.Elems = append(runs.Elems, rec("RunWrite", []string{"id", "data"}, r.ID, string(r.Data)))
	}
	events := &List{}
	for _, e := range c.Events {
		events.Elems = append(events.Elems, rec("EventRecord", []string{"name", "data"}, e.Name, string(e.Data)))
	}
	meta := &List{}
	for _, m := range c.Meta {
		data := s.in.none()
		if m.Data != nil {
			data = s.in.some(string(m.Data))
		}
		meta.Elems = append(meta.Elems, rec("MetaWrite", []string{"key", "data"}, m.Key, data))
	}
	commit := rec("Commit", []string{"writes", "runs", "events", "meta", "dropEvents"},
		writes, runs, events, meta, int64(c.DropEvents))
	s.value = s.call("commit", s.current(), commit)
}

func (s *eyeStore) snapshot() *Commit {
	c := &Commit{Events: s.Events()}
	for _, e := range s.strings(s.call("entities", s.current())) {
		for _, key := range s.Keys(e) {
			r, _ := s.Load(key)
			c.Writes = append(c.Writes, Write{Key: key, Entity: e, State: r.State, Version: r.Version, Schema: r.Schema, Ended: r.Ended})
		}
	}
	for _, id := range s.Runs() {
		data, _ := s.Run(id)
		c.Runs = append(c.Runs, RunWrite{ID: id, Data: data})
	}
	for _, k := range s.MetaKeys("") {
		data, _ := s.Meta(k)
		c.Meta = append(c.Meta, MetaWrite{Key: k, Data: data})
	}
	return c
}
