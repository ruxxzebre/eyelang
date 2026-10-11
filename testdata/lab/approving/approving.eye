# Approvals reach the operator: each approval! a run waits for becomes a
# message on Notify that says what is asked, until when, and how to answer.
#
# Answers come only from `eye approve` and `eye decline`, run where the
# program runs. Answering from Notify's buttons waits for answers that name
# the request and count only from the person asked (#71): until then, a
# press could answer any approval with the same text, by anyone.

import approvals
import home.{operator}
import notify

## Tells the operator about an approval a run waits for, and how to answer
## it. A request nobody answers ends at its `until`, which the run then takes
## as Expired.
@on(event(approvals.requested))
job ask(r: approvals.Request) {
  notify.send!(
    event: "approvals",
    title: "Approval needed",
    body: "{r.what}\n\nAnswer before {r.until} with eye approve \"{r.what}\", or eye decline, in the program's directory.",
    to: Some([operator]),
    key: "approval-{r.id}",
  )
}

# ---------------------------------------------------------------- tests

## What the test's run was told.
entity Outcome(id: String) {
  state decided: Option(Bool) = None

  pub fn set(approved: Bool) { decided = Some(approved) }
}

job restart() {
  match approval!("Restart the router?", from: operator, within: 1 h) {
    Approved(_) => Outcome("router").set!(true)
    Declined | Expired => Outcome("router").set!(false)
  }
}

test "an approval is sent to the operator on Notify, and eye approve answers it" {
  let sent = fakes(notify, from: "recordings/")
  with sent
  restart.start()
  expect(Outcome("router").decided == None)
  let [m] = sent.calls(notify.send) else { unreachable("not sent once") }
  expect(m.to == Some([operator]) && m.title == "Approval needed")
  expect(m.body |> contains("eye approve \"Restart the router?\""))
  approve("Restart the router?", by: operator)
  expect(Outcome("router").decided == Some(true))
}

test "a request nobody answers ends as Expired" {
  with fakes(notify, from: "recordings/")
  restart.start()
  advance(2 h)
  expect(Outcome("router").decided == Some(false))
}
