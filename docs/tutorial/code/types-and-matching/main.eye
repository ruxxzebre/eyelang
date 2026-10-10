# Tutorial 2: types and matching. Tasks on a to-do list.

# region record
type Task = { title: String, done: Bool, priority: Priority }
# endregion record

# region variant
type Priority = Low | Normal | High

type Status =
  | Open
  | Snoozed(until: Date)
  | Done(on: Date)
# endregion variant

# region record-values
test "records are values" {
  let task: Task = { title: "Buy milk", done: false, priority: Normal }
  let finished = { task with done: true }
  expect(task.done == false)
  expect(finished.done == true)
  expect(finished.title == "Buy milk")
  expect(task != finished)
}
# endregion record-values

# region match
fn describe(s: Status) -> String = match s {
  Open => "open"
  Snoozed(until) => "snoozed until {until}"
  Done(on) => "done on {on}"
}
# endregion match

# region test-match
test "match takes a variant apart" {
  expect(describe(Open) == "open")
  expect(describe(Snoozed(until: 2026-10-12)) == "snoozed until 2026-10-12")
  expect(describe(Done(on: 2026-10-10)) == "done on 2026-10-10")
}
# endregion test-match

# region guards
fn urgent(t: Task) -> Bool = match t.priority {
  High if !t.done => true
  _ => false
}

fn label(p: Priority) -> String = match p {
  Low | Normal => "-"
  High => "!"
}
# endregion guards

# region test-guards
test "guards, alternatives and _" {
  let t: Task = { title: "Pay rent", done: false, priority: High }
  expect(urgent(t))
  expect(!urgent({ t with done: true }))
  expect(label(Low) == "-" && label(High) == "!")
  expect(Low < High)
}
# endregion test-guards

# region option
fn findTask(tasks: List(Task), title: String) -> Option(Task) =
  tasks |> find(fn t: t.title == title)

fn priorityOf(tasks: List(Task), title: String) -> String =
  match findTask(tasks, title) {
    Some(t) => label(t.priority)
    None => "no such task"
  }
# endregion option

# region test-option
test "Option says a value may be missing" {
  let tasks: List(Task) = [
    { title: "Buy milk", done: false, priority: Normal },
    { title: "Pay rent", done: false, priority: High },
  ]
  expect(priorityOf(tasks, "Pay rent") == "!")
  expect(priorityOf(tasks, "Walk dog") == "no such task")
  let title = findTask(tasks, "Buy milk") |> map(fn t: t.title)
  expect(title == Some("Buy milk"))
  let missing = findTask(tasks, "Walk dog") |> map(fn t: t.title) ?? "none"
  expect(missing == "none")
}
# endregion test-option

# region impl
impl Task {
  fn new(title: String) -> Task = { title, done: false, priority: Normal }

  fn line(self) -> String {
    let mark = if self.done then "x" else " "
    "[{mark}] {label(self.priority)} {self.title}"
  }

  fn finish(self) -> Task = { self with done: true }
}
# endregion impl

# region test-impl
test "methods" {
  let t = Task.new("Buy milk")
  expect(t.line() == "[ ] - Buy milk")
  expect(t.finish().line() == "[x] - Buy milk")
}
# endregion test-impl
