# Approvals reach the operator: each approval! a run waits for becomes a
# question on Notify with two buttons, and the first press answers it.

import approvals
import home.{operator}
import notify.{Choice, QuestionId}

let yes: Choice = { id: "approve", label: "Approve" }
let no: Choice = { id: "decline", label: "Decline" }

## Asks the operator about an approval a run waits for, and passes the first
## press on as the answer. A question nobody presses ends with the request,
## which the run then takes as Expired.
@on(event(approvals.requested))
job ask(r: approvals.Request) {
  let left = r.until - now!()
  let asked = notify.ask!(
    event: "approvals",
    to: operator,
    text: r.what,
    choices: [yes, no],
    within: left,
    key: "approval-{r.id}",
  ).reconcile!()
  let question = match asked {
    Asking(q) => q
    NotAsked(_) => return
  }
  let Some(press) = next!(notify.answered, where: fn a: a.question == question, within: left) else {
    return
  }
  approvals.answered.publish!({ what: r.what, approved: press.choice == yes.id, by: "{press.by}" })
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

test "an approval is asked on Notify, and a press on Approve answers it" {
  restart.start()
  expect(Outcome("router").decided == None)
  notify.answered.publish({
    question: QuestionId("approval-1-0"),
    choice: "approve",
    by: operator,
    at: now(),
  })
  expect(Outcome("router").decided == Some(true))
}

test "a question nobody presses ends as Expired" {
  restart.start()
  advance(2 h)
  expect(Outcome("router").decided == Some(false))
}
