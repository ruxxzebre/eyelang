# SPECS.md section 7, example 1.
type Counter = { hits: Int, last: Option(Instant) }

fn record(mut c: Counter, at: Instant) {      # may change the caller's counter
  c.hits += 1
  c.last = Some(at)
}

impl Counter {
  fn reset(mut self) { self.hits = 0 }
}

fn tally(times: List(Instant)) -> Counter {
  var counter = { hits: 0, last: None }
  for t in times {
    record(mut counter, t)                    # `mut` at the call: this call may change counter
  }
  let snapshot = counter                      # a copy; later changes to counter don't reach it
  counter.reset()                             # a `mut self` method needs a `var` receiver
  snapshot
}
