# Structured concurrency on a test's clock: tasks that wait overlap, so a
# call takes as long as its longest task, and every task has finished or been
# cancelled when the call returns.

entity Log(name: String) {
  state lines: List(String) = []

  pub fn add(line: String) { lines += [line] }
}

## Waits, writes down that it finished, and gives back its name.
fn after(d: Duration, name: String) -> String {
  sleep(d)
  Log("done").add(name)
  name
}

test "parallelMap keeps the elements' order and overlaps the waits" {
  let start = now()
  let names = [3, 1, 2] |> parallelMap(fn n: after(1 min * n, "{n}"))
  expect(names == ["3", "1", "2"])
  expect(now() - start == 3 min)
  expect(Log("done").lines == ["1", "2", "3"])
}

test "parallelMap runs at most limit tasks at once" {
  let start = now()
  let names = [1, 1, 1] |> parallelMap(fn n: after(1 min, "x"), limit: 2)
  expect(names |> size == 3)
  expect(now() - start == 2 min)
}

test "both runs two functions at once" {
  let start = now()
  let (a, b) = both(fn { after(2 min, "a") }, fn { after(5 min, "b") })
  expect(a == "a" && b == "b")
  expect(now() - start == 5 min)
}

test "race gives the first to finish and cancels the rest at their wait" {
  let start = now()
  let winner = race([fn { after(5 min, "slow") }, fn { after(1 min, "fast") }])
  expect(winner == "fast")
  expect(now() - start == 1 min)
  expect(Log("done").lines == ["fast"])
}

test "timeout gives None and cancels what runs too long" {
  let start = now()
  expect(timeout(2 min, fn { after(1 min, "quick") }) == Some("quick"))
  expect(timeout(2 min, fn { after(10 min, "late") }) == None)
  expect(now() - start == 3 min)
  expect(Log("done").lines == ["quick"])
}

test "tasks inside tasks share one clock" {
  let start = now()
  let (inner, other) = both(
    fn { [1, 2] |> parallelMap(fn n: after(1 min * n, "in{n}")) },
    fn { after(90 s, "out") },
  )
  expect(inner == ["in1", "in2"] && other == "out")
  expect(now() - start == 2 min)
  expect(Log("done").lines == ["in1", "out", "in2"])
}
