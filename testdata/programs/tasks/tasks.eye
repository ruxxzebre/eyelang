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

## Puts 1 to n into a channel, writing down each put once it is done, then
## closes the channel.
fn produce(send: Sender(Int), n: Int) {
  for i in 1..=n {
    send.put(i)
    Log("flow").add("put {i}")
  }
  send.close()
}

test "a channel of capacity 1 holds one value: each put waits for the take before it" {
  let total = taskGroup(fn group {
    let (send, receive) = channel(capacity: 1)
    group.spawn(fn { produce(send, 3) })
    var sum = 0
    for n in receive {
      Log("flow").add("took {n}")
      sum += n
    }
    sum
  })
  expect(total == 6)
  expect(Log("flow").lines == ["put 1", "took 1", "put 2", "took 2", "put 3", "took 3"])
}

test "a group waits for every task it spawned, and tasks spawn more" {
  let start = now()
  taskGroup(fn group {
    group.spawn(fn { after(2 min, "a") })
    group.spawn(fn {
      after(1 min, "b")
      group.spawn(fn { after(3 min, "c") })
    })
  })
  expect(now() - start == 4 min)
  expect(Log("done").lines == ["b", "a", "c"])
}

test "take gives None once the channel is closed and empty" {
  let (send, receive) = channel(capacity: 2)
  send.put("x")
  send.close()
  expect(receive.take() == Some("x"))
  expect(receive.take() == None)
}

test "waits overlap in a group, as in parallelMap" {
  let start = now()
  let names = taskGroup(fn group {
    let (send, receive) = channel(capacity: 8)
    for n in [3, 1, 2] {
      group.spawn(fn { send.put(after(1 min * n, "{n}")) })
    }
    [receive.take(), receive.take(), receive.take()]
  })
  expect(names == [Some("1"), Some("2"), Some("3")])
  expect(now() - start == 3 min)
}
