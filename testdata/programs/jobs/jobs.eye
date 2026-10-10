# Jobs wait on the test's clock and resume by replaying their recorded steps.

type Phase = Idle | Brewing | Done

entity Kettle(id: String) {
  state phase: Phase = Idle
  state brews: Int = 0

  pub fn brew() {
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
