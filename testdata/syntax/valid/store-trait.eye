# The Store trait and an in-memory store written in Eyelang.

pub trait Store {
  fn get(self, key: Key) -> Option(Row)
  fn put(mut self, key: Key, row: Row)
  fn remove(mut self, key: Key)
  fn due(self, now: Instant) -> List(RunId)
}

pub opaque type MemoryStore = { rows: Map(Key, Row), runs: Map(RunId, Instant) }

impl MemoryStore {
  pub fn new() -> MemoryStore = { rows: {}, runs: {} }
}

impl Store for MemoryStore {
  fn get(self, key: Key) -> Option(Row) = self.rows |> get(key)
  fn put(mut self, key: Key, row: Row) { self.rows = self.rows |> insert(key, row) }
  fn remove(mut self, key: Key) { self.rows = self.rows |> delete(key) }
  fn due(self, now: Instant) -> List(RunId) =
    self.runs |> entries |> filter(fn (_, at): at <= now) |> map(fn (id, _): id)
}
