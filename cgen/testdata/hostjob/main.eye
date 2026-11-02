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
