# Memory: @memory, @priority and reserve! (SPECS.md §15). In tests the runner
# has 1 GiB.

## What happened, in order.
entity Log(id: String) {
  state lines: List(String) = []

  pub fn add(line: String) { lines += [line] }
}

## Holds 800 MiB for an hour.
job hog() {
  reserve!(800 MiB, fn {
    Log("mem").add!("hog holds")
    sleep!(1 h)
  })
  Log("mem").add!("hog done")
}

@memory(512 MiB)
@priority(background)
job sweep() {
  Log("mem").add!("sweep")
}

@memory(512 MiB)
@priority(interactive)
job answer() {
  Log("mem").add!("answer")
}

test "a run waits for memory, and a higher priority goes first" {
  hog.start()
  sweep.start()
  answer.start()
  expect(Log("mem").lines == ["hog holds"])
  advance(1 h)
  expect(Log("mem").lines == ["hog holds", "hog done", "answer", "sweep"])
}

## Two of these can't hold their memory at once.
job pair(name: String) {
  let n = reserve!(600 MiB, fn {
    Log("pair").add!("{name} in")
    sleep!(10 min)
    Log("pair").add!("{name} out")
    1
  })
  Log("pair").add!("{name} gave back {n}")
}

test "reserve! waits until the memory is given back" {
  pair.start("a")
  pair.start("b")
  expect(Log("pair").lines == ["a in"])
  advance(10 min)
  expect(Log("pair").lines == ["a in", "a out", "a gave back 1", "b in"])
  advance(10 min)
  expect(Log("pair").lines == ["a in", "a out", "a gave back 1", "b in", "b out", "b gave back 1"])
}

## Keeps a step for every minute it waits: more than 1 KiB of saved state.
@memory(1 KiB)
job greedy() {
  for i in 0..100 {
    Log("greedy").add!("{now!()}")
  }
}

## More than the runner has.
@memory(2 GiB)
job huge() {}

job watch() {
  for r in [greedy.start!(), huge.start!()] {
    match r.join!() {
      Completed(_) => Log("watch").add!("completed")
      Failed(fault) => Log("watch").add!("{fault}")
    }
  }
}

test "a run that holds more than its @memory ends as OutOfMemory" {
  watch.start()
  expect(Log("watch").lines == ["OutOfMemory", "OutOfMemory"])
}
