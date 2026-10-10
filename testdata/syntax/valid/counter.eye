type Counter = { hits: Int, last: Option(Instant) }

fn record(mut c: Counter, at: Instant) {
  c.hits += 1
  c.last = Some(at)
}

impl Counter {
  fn reset(mut self) { self.hits = 0 }
  fn hits(self) -> Int = self.hits
}

fn tally(times: List(Instant)) -> Counter {
  var counter = { hits: 0, last: None }
  for t in times {
    record(mut counter, t)
  }
  let snapshot = counter
  counter.reset()
  snapshot
}
