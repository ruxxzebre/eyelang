# A model's tools (SPECS.md §19): functions passed to ai.generate as tools:.
# The model calls them, the runtime runs them and sends back their results,
# and a @destructive one runs only after a person approves that one call.
# The recordings in recordings/ stand in for the model.

import ai

## An issue in the tracker.
type Issue = { id: Int, title: String }

## What a triage decides about a report.
type Triage = { duplicateOf: Option(Int), labels: List(String) }

## What the tools did, and how the triage ended.
entity Trail(name: String) {
  state lines: List(String) = []

  pub fn add(line: String) { lines += [line] }
}

## Finds open issues whose title mentions a phrase.
fn searchIssues(phrase: String, limit: Int = 10) -> List(Issue) uses {entity Trail} {
  Trail("tools").add("searched {phrase}")
  [{ id: 7, title: "Crash on {phrase}" }, { id: 9, title: "Slow {phrase}" }] |> take(limit)
}

## Adds a label to an issue.
@destructive
fn label(issue: Int, name: String) uses {entity Trail} {
  Trail("tools").add("labelled {issue} {name}, as {approval.by} approved")
}

job triage(report: String) {
  let answer: Result(Triage, ai.AiError) = ai.generate!(
    model: "claude-opus-5-5",
    prompt: report,
    tools: [searchIssues, label],
    approve: fn call: approval!(call, from: "maintainer", within: 1 day),
  )
  match answer {
    Ok(t) => Trail("tools").add!("labels {t.labels}")
    Err(e) => Trail("tools").add!("failed: {e}")
  }
}

test "the model calls two tools, and the destructive one waits for approve" {
  with fakes(ai, from: "recordings/")
  triage.start("It crashes on save")
  expect(Trail("tools").lines == ["searched save"])
  approve("label(issue: 7, name: \"duplicate\")")
  expect(Trail("tools").lines == ["searched save", "labelled 7 duplicate, as maintainer approved", "labels [\"duplicate\"]"])
}

test "a call the person declines doesn't run, and the model goes on" {
  with fakes(ai, from: "recordings/")
  triage.start("It crashes on save")
  decline("label(issue: 7")
  expect(Trail("tools").lines == ["searched save", "labels [\"duplicate\"]"])
}

test "a call nobody approves in time doesn't run" {
  with fakes(ai, from: "recordings/")
  triage.start("It crashes on save")
  advance(1 day)
  expect(Trail("tools").lines == ["searched save", "labels [\"duplicate\"]"])
}
