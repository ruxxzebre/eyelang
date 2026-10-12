# Jobs wait on the test's clock and resume by replaying their recorded steps.

type Phase = Idle | Brewing | Done

entity Kettle(id: String) {
  state phase: Phase = Idle
  state brews: Int = 0

  pub fn brew() uses {start boil} {
    if phase == Idle {
      phase = Brewing
      boil.start(id, after: 1 min)
    }
  }

  pub fn finish() {
    phase = Done
    brews += 1
  }

  pub fn reset() { phase = Idle }
}

view finished = Kettle.all |> filter(fn k: k.phase == Done)

job boil(id: String) {
  log.info("boiling {id}")
  sleep!(3 min)
  log.info("steeping {id}")
  sleep!(4 min)
  Kettle(id).finish!()
}

@on(enters(finished))
job pour(k: Ref(Kettle)) {
  let kettle = k.read!()
  log.info("pouring {kettle.id} after {kettle.brews} brews")
  k.reset!()
}

test "a job waits for its start, then for each sleep" {
  let k = Kettle("office")
  k.brew()
  expect(k.phase == Brewing)
  advance(1 min)
  expect(k.phase == Brewing)
  advance(3 min)
  expect(k.phase == Brewing)
  advance(4 min)
  expect(k.brews == 1)
  expect(k.phase == Idle)  # finished, then poured: the view started pour
}

test "views notice each entity that enters them" {
  let a = Kettle("a")
  let b = Kettle("b")
  a.brew()
  advance(2 min)
  b.brew()
  advance(1 h)
  expect(a.brews == 1 && b.brews == 1)
  expect(Kettle.all |> filter(fn k: k.phase == Idle) |> size == 2)
}

test "a second brew while brewing starts nothing new" {
  let k = Kettle("x")
  k.brew()
  k.brew()
  advance(1 h)
  expect(k.brews == 1)
}

entity Feed(name: String) {
  state up: Bool = false
  state polls: Int = 0

  pub fn check() -> Bool {
    polls += 1
    up
  }

  pub fn start() { up = true }
}

job waitForFeed(name: String) {
  if poll!(every: 1 min, within: 10 min, fn { Feed(name).check!() }) {
    log.info("{name} is up")
  } else {
    log.info("gave up on {name}")
  }
}

test "poll! checks until the check passes, then stops" {
  waitForFeed.start("news")
  advance(3 min)
  Feed("news").start()
  advance(1 h)
  expect(Feed("news").polls == 5)
}

test "poll! gives up when its time runs out" {
  waitForFeed.start("dead")
  advance(1 h)
  expect(Feed("dead").polls == 11)
}

# A function that reads the clock without waiting is one step when a job
# calls it with `!`: a resumed run sees the time it saw the first time.
fn stamp() -> Instant = now()

entity Stamps(id: String) {
  state seen: List(Instant) = []

  pub fn add(at: Instant) { seen += [at] }
}

job stamped() {
  let at = stamp!()
  sleep!(10 min)
  Stamps("s").add!(at)
}

test "a step that reads the clock is recorded once" {
  let start = now()
  stamped.start()
  advance(20 min)
  expect(Stamps("s").seen == [start])
}

# A child run is started once, though its parent is replayed each time it
# wakes: the start is one of the parent's steps.
entity Children(id: String) {
  state started: Int = 0

  pub fn add() { started += 1 }
}

job child() {
  Children("all").add!()
}

job parent() {
  child.start!()
  sleep!(5 min)
  sleep!(5 min)
}

test "a child run starts once, however often its parent is replayed" {
  parent.start()
  advance(20 min)
  expect(Children("all").started == 1)
}

# A test can't wait, so it reads a run's outcome with outcome(): None while
# the run is still going, then how it ended.
job double(n: Int) -> Int {
  sleep!(10 min)
  n * 2
}

@deadline(5 min)
job late() -> Int {
  sleep!(1 h)
  0
}

test "a test reads a run's outcome once the clock has moved past its end" {
  let run = double.start(21)
  expect(run.outcome() == None)
  advance(9 min)
  expect(run.outcome() == None)
  advance(1 min)
  expect(run.outcome() == Some(Completed(42)))
  let stopped = late.start()
  advance(5 min)
  expect(stopped.outcome() == Some(Failed(DeadlinePassed)))
}

# `at:` starts a run at an instant, and `07:45 in zone` is the next instant
# the zone's clock shows 07:45: today if it hasn't yet, else tomorrow.
let dublin: Zone = "Europe/Dublin"

job stampAt() { Stamps("at").add!(now!()) }

job wakeAt() {
  sleepUntil!(07:45 in dublin)
  Stamps("wake").add!(now!())
}

test "a run starts at the instant at: names" {
  with clock(2026-10-12 09:00 in dublin)
  stampAt.start(at: 2026-10-12 10:30 in dublin)
  advance(1 h)
  expect(Stamps("at").seen == [])
  advance(1 h)
  expect(Stamps("at").seen == [2026-10-12 10:30 in dublin])
}

test "a time of day in a zone is the next instant its clock shows it" {
  with clock(2026-10-12 09:00 in dublin)
  expect(07:45 in dublin == 2026-10-13 07:45 in dublin)
  expect(10:00 in dublin == 2026-10-12 10:00 in dublin)
  wakeAt.start()
  advance(1 day)
  expect(Stamps("wake").seen == [2026-10-13 07:45 in dublin])
}

test "a start already due runs with the call" {
  with clock(2026-10-12 09:00 in dublin)
  stampAt.start(at: 2026-10-12 08:00 in dublin)
  stampAt.start(after: 0 s)
  stampAt.start()
  expect(Stamps("at").seen == [2026-10-12 09:00 in dublin, 2026-10-12 09:00 in dublin, 2026-10-12 09:00 in dublin])
}
