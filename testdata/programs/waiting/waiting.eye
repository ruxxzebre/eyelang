# Waiting for several runs with allDone! and firstDone!, and locks with lock!,
# on the test's clock. interp/waits_test.go shows each surviving a restart.

entity Notes(id: String) {
  state lines: List(String) = []

  pub fn add(line: String) { lines += [line] }
}

job ask(what: String) -> Decision { approval!(what, from: "operator", within: 3 days) }

job timer(d: Duration) -> Decision {
  sleep!(d)
  Expired
}

## Ships a build if someone approves it within the hour.
job ship(build: String) {
  let asked = ask.start!("Ship {build}?")
  let answer = firstDone!([asked, timer.start!(1 h)])
  let line = match answer {
    Completed(Approved(_)) => "{build} shipped"
    Completed(_) => "{build} wasn't approved within the hour"
    Failed(fault) => "{build}: {fault}"
  }
  Notes("ship").add!(line)
  # The run that lost was stopped.
  let lost = match asked.join!() {
    Failed(Killed) => "the question was withdrawn"
    _ => "the question was answered"
  }
  Notes("ship").add!(lost)
}

test "firstDone! takes an approval that comes before the timeout" {
  ship.start("v2")
  advance(20 min)
  expect(Notes("ship").lines == [])
  approve("Ship v2?")
  expect(Notes("ship").lines == ["v2 shipped", "the question was answered"])
  advance(2 h)
  expect(Notes("ship").lines |> size == 2)
}

test "firstDone! takes the timeout when nobody answers, and stops the approval" {
  ship.start("v3")
  advance(59 min)
  expect(Notes("ship").lines == [])
  advance(1 min)
  expect(Notes("ship").lines == ["v3 wasn't approved within the hour", "the question was withdrawn"])
  # The question no longer waits, so approve("Ship v3?") would find nothing to answer.
  advance(3 days)
  expect(Notes("ship").lines |> size == 2)
}

job brew(name: String, d: Duration) -> String {
  sleep!(d)
  name
}

@deadline(1 min)
job burn() -> String {
  sleep!(1 h)
  "toast"
}

job breakfast() {
  let outcomes = allDone!([brew.start!("tea", 3 min), brew.start!("coffee", 5 min), burn.start!()])
  let names = outcomes |> map(fn o: match o {
    Completed(name) => name
    Failed(fault) => "{fault}"
  })
  Notes("breakfast").add!(names |> join(", "))
}

test "allDone! waits for every run, and gives their outcomes in the list's order" {
  breakfast.start()
  advance(4 min)
  expect(Notes("breakfast").lines == [])
  advance(1 min)
  expect(Notes("breakfast").lines == ["tea, coffee, DeadlinePassed"])
}

## Deploys while holding the lock "prod", so two deploys never overlap.
job deploy(name: String, takes: Duration) {
  lock!("prod", for: 30 min, fn {
    Notes("prod").add!("{name} starts")
    sleep!(takes)
    Notes("prod").add!("{name} ends")
  })
}

## Deploys as a child run, and writes down how it ended.
job watch(name: String, takes: Duration) {
  let line = match deploy.start!(name, takes).join!() {
    Completed(_) => "{name} deployed"
    Failed(fault) => "{name}: {fault}"
  }
  Notes("outcomes").add!(line)
}

test "a lock lets one run in at a time" {
  deploy.start("a", 10 min)
  deploy.start("b", 10 min)
  advance(15 min)
  expect(Notes("prod").lines == ["a starts", "a ends", "b starts"])
  advance(1 h)
  expect(Notes("prod").lines == ["a starts", "a ends", "b starts", "b ends"])
}

test "a lock that runs out while its holder waits ends the holder, and the next run takes it" {
  watch.start("stuck", 2 h)
  watch.start("next", 5 min)
  advance(29 min)
  expect(Notes("prod").lines == ["stuck starts"])
  advance(1 min)
  expect(Notes("prod").lines == ["stuck starts", "next starts"])
  expect(Notes("outcomes").lines == ["stuck: LockExpired(\"prod\")"])
  advance(3 h)
  expect(Notes("prod").lines == ["stuck starts", "next starts", "next ends"])
  expect(Notes("outcomes").lines == ["stuck: LockExpired(\"prod\")", "next deployed"])
}
