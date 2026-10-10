## The in-memory store: everything a running program keeps, as one immutable
## value. Each commit is a pure function from one store to the next, and the
## runtime holds the only reference to the current one.

# SPECS.md §15.
# The runtime reaches it through the Store interface in interp/store.go. A
# journal (interp/journal.go) wraps it to keep it across restarts. Saved
# values are bytes; until the interpreter has Bytes values, they travel as
# Strings, which hold any bytes.

## An entity's saved state, and how many transactions have changed it.
pub type Row = { state: String, version: Int, schema: Int }

## A published event: its name and its value as bytes.
pub type EventRecord = { name: String, data: String }

## An entity's new state, as a transaction writes it.
pub type Write = { key: String, entity: String, state: String, version: Int, schema: Int }

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

## Everything the store holds. Entity keys and runs keep the order they were
## first saved in, which listings follow.
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
  order: List(Int),
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
    order: [],
    events: [],
    meta: {},
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
  fn runs(self) -> List(Int) = self.order

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
      let row: Row = { state: w.state, version: w.version, schema: w.schema }
      s.rows = s.rows |> insert(w.entity, rows |> insert(w.key, row))
    }
    for r in c.runs {
      let runs = s.runs[bucket(r.id)] ?? {}
      if !(runs |> contains(r.id)) { s.order += [r.id] }
      s.runs = s.runs |> insert(bucket(r.id), runs |> insert(r.id, r.data))
    }
    s.events = (s.events |> drop(c.dropEvents)) + c.events
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
      { key: "World/a", entity: "World", state: "1", version: 1, schema: 1 },
      { key: "World/b", entity: "World", state: "2", version: 1, schema: 1 },
    ],
    runs: [{ id: 2, data: "r2" }, { id: 1, data: "r1" }],
    events: [{ name: "e", data: "x" }],
    meta: [{ key: "fired/j", data: Some("10") }],
    dropEvents: 0,
  }
  let s = MemoryStore.empty().commit(c)
  expect(s.keys("World") == ["World/a", "World/b"])
  expect(s.runs() == [2, 1])
  expect(s.load("World", "World/b") |> map(fn r: r.state) == Some("2"))
  expect(s.run(2) == Some("r2") && s.run(99) == None)
  expect(s.metaKeys("fired/") == ["fired/j"])
  let later: Commit = {
    writes: [{ key: "World/a", entity: "World", state: "3", version: 2, schema: 1 }],
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
