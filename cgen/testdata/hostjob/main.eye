# A program whose job and entity methods run compiled, in the runner, while
# the runner keeps its store, its clock and its steps. Its tests run in the
# interpreter and call them: TestHostedJobs compares them with `eye test`.

type Item = { name: String, count: Int }

entity Tally(id: String) {
  state total: Int = 0
  state names: List(String) = []

  pub fn add(item: Item) -> Int {
    total += item.count
    names = names + [item.name]
    total
  }
}

## Waits, then counts one item: a function that stops, called from a job.
fn counted(item: Item, id: String) -> Int uses {wait, entity Tally} {
  sleep!(1 min)
  Tally(id).add!(item)
}

job tally(id: String, items: List(Item)) -> String {
  var sum = 0
  for item in items {
    sum = counted!(item, id)
    log.info("counted {item.name}")
  }
  let seen = Tally(id).read!().names
  "{sum} from {seen}"
}

test "a job that stops in a loop and in a function it calls" {
  let items = [{ name: "a", count: 2 }, { name: "b", count: 3 }]
  let run = tally.start("t", items, key: "t")
  advance(10 min)
  expect(run.outcome() == Some(Completed("5 from [\"a\", \"b\"]")))
  expect(Tally("t").total == 5)
}

test "an entity method called directly" {
  expect(Tally("d").add({ name: "x", count: 7 }) == 7)
  expect(Tally("d").names == ["x"])
}

fn depth(n: Int) -> Int = if n == 0 then 0 else 1 + depth(n - 1)

## Walks a set and a map across steps, and goes deep on the big stack.
job walks(items: Set(String), counts: Map(String, Int)) -> String {
  var seen = 0
  for item in items {
    sleep!(1 min)
    seen += item.size()
  }
  for (name, count) in counts {
    sleep!(1 min)
    seen += name.size() + count
  }
  "{seen} after {depth(50000)}"
}

job shows(id: String) -> String {
  let ref = Tally(id)
  sleep!(1 min)
  "{ref}"
}

test "a set and a map are walked across steps, and recursion goes deep" {
  let counts: Map(String, Int) = { "d": 4 }
  let run = walks.start(toSet(["ab", "c"]), counts, key: "w")
  advance(10 min)
  expect(run.outcome() == Some(Completed("8 after 50000")))
}

test "a Ref shows as it does in the interpreter" {
  let run = shows.start("s", key: "s")
  advance(10 min)
  expect(run.outcome() == Some(Completed("Tally(\"s\")")))
}
