# Approvals reach the operator: each approval! a run waits for becomes a
# question on Notify with Approve and Decline buttons, and the first press
# answers it.
#
# The answer names the request's id, so a press answers that request only,
# and the runtime counts it only when the person who pressed is the one the
# approval asks: a press by anyone else is refused, and the run keeps
# waiting. `eye approve` and `eye decline`, run where the program runs, still
# answer too.

import approvals
import home.{UserId, operator}
import notify.{Choice, QuestionId}

## The buttons under a question, in this order. A button's id and label are
## the variant's name.
type Button = Approve | Decline

let buttons: List(Choice) = Button.variants() |> map(fn b: { id: "{b}", label: "{b}" })

## Asks the operator about an approval a run waits for, and passes the first
## press on. A request nobody answers ends at its `until`, which the run then
## takes as Expired.
@on(event(approvals.requested))
job ask(r: approvals.Request) {
  let left = r.until - now!()
  let asked = notify.ask!(
    event: "approvals",
    to: operator,
    text: "{r.what}\n\nOr answer before {r.until} with eye approve --id {r.id}, or eye decline, in the program's directory.",
    choices: buttons,
    within: left,
    key: "approval-{r.id}",
  ).reconcile!()
  let Asking(question) = asked else { return }
  let Some(press) = next!(notify.answered, where: fn a: a.question == question, within: left) else {
    return
  }
  approvals.answered.publish!({ id: r.id, approved: press.choice == "{Approve}", by: "{press.by}" })
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

test "an approval is asked on Notify with buttons, and the operator's press answers it" {
  let sent = fakes(notify, from: "recordings/")
  with sent
  restart.start()
  expect(Outcome("router").decided == None)
  let [q] = sent.calls(notify.ask) else { unreachable("not asked once") }
  expect(q.to == operator && q.choices == buttons)
  expect(q.text |> startsWith("Restart the router?"))
  notify.answered.publish({ question: QuestionId("q-1"), choice: "Approve", by: operator, at: now() })
  expect(Outcome("router").decided == Some(true))
}

test "the operator's press on Decline declines it" {
  with fakes(notify, from: "recordings/")
  restart.start()
  notify.answered.publish({ question: QuestionId("q-1"), choice: "Decline", by: operator, at: now() })
  expect(Outcome("router").decided == Some(false))
}

test "a press by someone else is refused, and the run keeps waiting" {
  with fakes(notify, from: "recordings/")
  restart.start()
  notify.answered.publish({ question: QuestionId("q-1"), choice: "Approve", by: UserId("mallory"), at: now() })
  expect(Outcome("router").decided == None)
  approve("Restart the router?")
  expect(Outcome("router").decided == Some(true))
}

test "eye approve answers it too" {
  with fakes(notify, from: "recordings/")
  restart.start()
  approve("Restart the router?", by: operator)
  expect(Outcome("router").decided == Some(true))
}

test "a request nobody answers ends as Expired" {
  with fakes(notify, from: "recordings/")
  restart.start()
  advance(2 h)
  expect(Outcome("router").decided == Some(false))
}
