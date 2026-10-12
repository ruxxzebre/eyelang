## The in-memory store: everything a running program keeps, as one immutable
## value. Each commit is a pure function from one store to the next, and the
## runtime holds the only reference to the current one.

# SPECS.md §15.
# The runtime reaches it through the Store interface in interp/store.go. A
# journal (interp/journal.go) wraps it to keep it across restarts. Saved
# values are bytes; until the interpreter has Bytes values, they travel as
# Strings, which hold any bytes.

## An entity's saved state, and how many transactions have changed it. An
## entity that ended keeps a row with ended true, whose state says how it
## ended, so that its identity stays ended.
pub type Row = { state: String, version: Int, schema: Int, ended: Bool }

## A published event: its name and its value as bytes.
pub type EventRecord = { name: String, data: String }

## An entity's new state, as a transaction writes it, or, when ended is true,
## what is kept of an entity that ended.
pub type Write = {
  key: String,
  entity: String,
  state: String,
  version: Int,
  schema: Int,
  ended: Bool,
}

## A job run as saved.
pub type RunWrite = { id: Int, data: String }

## A meta key's new value; None removes the key.
pub type MetaWrite = { key: String, data: Option(String) }

## What one transaction writes, all at once.
pub type Commit = {
  writes: List(Write),
  runs: List(RunWrite),
  events: List(EventRecord),
  meta: List(MetaWrite),
  ## How many of the oldest events to drop, which no run can still wait for.
  dropEvents: Int,
}

## Everything the store holds. Entity keys keep the order they were first
## saved in, which listings follow; runs are listed by number, which is the
## order they started in.
##
## Runs are kept in buckets of 64 by their number, and rows by entity type, so
## that a commit, which rewrites a run at every step, replaces one small map
## and not a map of every run. A store whose only reference the runtime holds
## would be updated in place anyway once compiled; the interpreter copies.
pub opaque type MemoryStore = {
  rows: Map(String, Map(String, Row)),
  kinds: Map(String, List(String)),
  entities: List(String),
  runs: Map(Int, Map(Int, String)),
  events: List(EventRecord),
  meta: Map(String, String),
}

## What a store does, whichever keeps the data: the in-memory store below, or
## a database. Its own type is s, which a commit returns.
pub trait Store(s) {
  ## An entity's saved row.
  fn load(self, entity: String, key: String) -> Option(Row)

  ## The saved entities of one entity type, oldest first.
  fn keys(self, entity: String) -> List(String)

  ## The entity types with saved entities, in the order they first appeared.
  fn entities(self) -> List(String)

  ## A saved job run.
  fn run(self, id: Int) -> Option(String)

  ## The saved runs, oldest first.
  fn runs(self) -> List(Int)

  ## The published events, oldest first.
  fn events(self) -> List(EventRecord)

  ## A meta key's value.
  fn meta(self, key: String) -> Option(String)

  ## The meta keys that start with prefix, in order.
  fn metaKeys(self, prefix: String) -> List(String)

  ## The store after a transaction: every write of the commit, or none.
  fn commit(self, c: Commit) -> s
}

impl MemoryStore {
  ## A store that holds nothing.
  pub fn empty() -> MemoryStore = {
    rows: {},
    kinds: {},
    entities: [],
    runs: {},
    events: [],
    meta: {},
  }

  ## The store without the records of the runs ids. A journal or a database
  ## that wraps this store keeps a run's record itself while the run waits or
  ## after it ends, and only a ready run's record here (SPECS.md §15).
  pub fn forget(self, ids: List(Int)) -> MemoryStore {
    var s = self
    for id in ids {
      let runs = s.runs[bucket(id)] ?? {}
      if runs |> contains(id) {
        let left = runs |> remove(id)
        s.runs = if left |> size == 0 {
          s.runs |> remove(bucket(id))
        } else {
          s.runs |> insert(bucket(id), left)
        }
      }
    }
    s
  }
}

impl Store(MemoryStore) for MemoryStore {
  ## An entity's saved row.
  fn load(self, entity: String, key: String) -> Option(Row) =
    match self.rows[entity] {
      Some(rows) => rows[key]
      None => None
    }

  ## The saved entities of one entity type, oldest first.
  fn keys(self, entity: String) -> List(String) = self.kinds[entity] ?? []

  ## The entity types with saved entities, in the order they first appeared.
  fn entities(self) -> List(String) = self.entities

  ## A saved job run.
  fn run(self, id: Int) -> Option(String) =
    match self.runs[bucket(id)] {
      Some(runs) => runs[id]
      None => None
    }

  ## The saved runs, oldest first.
  fn runs(self) -> List(Int) =
    self.runs |> keys |> sort |> flatMap(fn b: (self.runs[b] ?? {}) |> keys |> sort)

  ## The published events, oldest first.
  fn events(self) -> List(EventRecord) = self.events

  ## A meta key's value.
  fn meta(self, key: String) -> Option(String) = self.meta[key]

  ## The meta keys that start with prefix, in order.
  fn metaKeys(self, prefix: String) -> List(String) =
    self.meta |> keys |> filter(fn k: k |> startsWith(prefix)) |> sort

  ## The store after a transaction: every write of the commit, or none, since
  ## the commit is one value applied in one step.
  fn commit(self, c: Commit) -> MemoryStore {
    var s = self
    for w in c.writes {
      let rows = s.rows[w.entity] ?? {}
      if !(rows |> contains(w.key)) {
        if !(s.kinds |> contains(w.entity)) { s.entities += [w.entity] }
        s.kinds = s.kinds |> insert(w.entity, (s.kinds[w.entity] ?? []) + [w.key])
      }
      let row: Row = { state: w.state, version: w.version, schema: w.schema, ended: w.ended }
      s.rows = s.rows |> insert(w.entity, rows |> insert(w.key, row))
    }
    for r in c.runs {
      let runs = s.runs[bucket(r.id)] ?? {}
      s.runs = s.runs |> insert(bucket(r.id), runs |> insert(r.id, r.data))
    }
    # Most commits neither drop nor add events, and leave the list alone: a
    # list from drop has no spare room, so + would copy every event kept.
    if c.dropEvents > 0 { s.events = s.events |> drop(c.dropEvents) }
    if c.events |> size > 0 { s.events += c.events }
    for m in c.meta {
      s.meta = match m.data {
        Some(data) => s.meta |> insert(m.key, data)
        None => s.meta |> remove(m.key)
      }
    }
    s
  }
}

## The bucket a run is kept in.
fn bucket(id: Int) -> Int = id / 64

test "a commit applies every write, and listings keep their order" {
  let c: Commit = {
    writes: [
      { key: "World/a", entity: "World", state: "1", version: 1, schema: 1, ended: false },
      { key: "World/b", entity: "World", state: "2", version: 1, schema: 1, ended: false },
    ],
    runs: [{ id: 2, data: "r2" }, { id: 1, data: "r1" }],
    events: [{ name: "e", data: "x" }],
    meta: [{ key: "fired/j", data: Some("10") }],
    dropEvents: 0,
  }
  let s = MemoryStore.empty().commit(c)
  expect(s.keys("World") == ["World/a", "World/b"])
  expect(s.runs() == [1, 2])
  expect(s.load("World", "World/b") |> map(fn r: r.state) == Some("2"))
  expect(s.run(2) == Some("r2") && s.run(99) == None)
  expect(s.metaKeys("fired/") == ["fired/j"])
  let later: Commit = {
    writes: [{ key: "World/a", entity: "World", state: "3", version: 2, schema: 1, ended: false }],
    runs: [],
    events: [{ name: "f", data: "y" }],
    meta: [{ key: "fired/j", data: None }],
    dropEvents: 1,
  }
  let t = s.commit(later)
  expect(t.keys("World") == ["World/a", "World/b"])
  expect(t.load("World", "World/a") |> map(fn r: r.version) == Some(2))
  expect(t.events() |> map(fn e: e.name) == ["f"])
  expect(t.metaKeys("") == [])
  expect(s.load("World", "World/a") |> map(fn r: r.state) == Some("1"))
}

test "an ended entity keeps its row, marked ended" {
  let c: Commit = {
    writes: [
      { key: "World/a", entity: "World", state: "1", version: 1, schema: 1, ended: false },
      { key: "World/b", entity: "World", state: "2", version: 1, schema: 1, ended: false },
    ],
    runs: [],
    events: [],
    meta: [],
    dropEvents: 0,
  }
  let s = MemoryStore.empty().commit(c)
  let end: Commit = {
    writes: [{ key: "World/a", entity: "World", state: "ended", version: 1, schema: 1, ended: true }],
    runs: [],
    events: [],
    meta: [],
    dropEvents: 0,
  }
  let t = s.commit(end)
  expect(t.load("World", "World/a") |> map(fn r: r.ended) == Some(true))
  expect(t.load("World", "World/b") |> map(fn r: r.ended) == Some(false))
  expect(t.keys("World") == ["World/a", "World/b"])
}

test "a forgotten run's record leaves the store, and the other runs stay" {
  let c: Commit = {
    writes: [],
    runs: [{ id: 1, data: "r1" }, { id: 2, data: "r2" }, { id: 70, data: "r70" }],
    events: [],
    meta: [],
    dropEvents: 0,
  }
  let s = MemoryStore.empty().commit(c).forget([2, 70, 99])
  expect(s.runs() == [1])
  expect(s.run(2) == None && s.run(70) == None && s.run(1) == Some("r1"))
}
