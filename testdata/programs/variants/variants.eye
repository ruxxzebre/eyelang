# Two imported modules both declare Low, High and Busy. Written alone, such
# a name is the variant of the type expected where it is written, as in a
# pattern it is the matched value's (SPECS.md §13).

import effort
import priority

type Job = { priority: priority.Priority, effort: effort.Effort }

fn hard() -> effort.Effort = High

fn later() -> effort.Reply = Busy(secs: 3)

fn label(p: priority.Priority) -> String = match p {
  Low => "low"
  Normal => "normal"
  High => "high"
}

test "an argument, a typed let and a return say which" {
  expect(effort.think(Some(High)) == effort.High)
  expect(priority.send(High) == priority.High)
  let e: effort.Effort = Low
  expect(e == effort.Low)
  expect(hard() == effort.High)
}

test "a record field and a later use say which" {
  let job: Job = { priority: Low, effort: High }
  expect(label(job.priority) == "low")
  let urgent = High
  expect(label(urgent) == "high")
}

test "a variant with fields is the one the call's type expects" {
  expect(later() == effort.Busy(secs: 3))
  let s: priority.Status = Busy(secs: 5)
  expect(s == priority.Busy(secs: 5))
}
