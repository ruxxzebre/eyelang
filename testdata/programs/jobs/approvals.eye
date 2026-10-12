# A run waits at approval! until the person asked answers or its time runs
# out. An answer names its request's id, and counts only from that person.

import approvals

entity Ask(id: String) {
  state outcome: String = "waiting"
  state times: Int = 0

  pub fn set(o: String) { outcome = o }
  pub fn again() { times += 1 }
}

job ask(id: String) {
  let o = match approval!("Restart {id}?", from: "operator", within: 1 h) {
    Approved(_) => "approved"
    Declined => "declined"
    Expired => "expired"
  }
  Ask(id).set!(o)
}

## Asks again each time nobody answers, as the lab's access sync does.
job askUntilAnswered(id: String) {
  while true {
    Ask(id).again!()
    match approval!("Rotate {id}?", from: "operator", within: 1 day) {
      Approved(_) => {
        Ask(id).set!("approved")
        return
      }
      Declined => {
        Ask(id).set!("declined")
        return
      }
      Expired => sleep!(1 h)
    }
  }
}

## The requests published, in order, as a routing job would see them.
entity Requests(name: String) {
  state ids: List(String) = []
  state from: List(String) = []

  pub fn add(r: approvals.Request) {
    ids += [r.id]
    from += [r.from]
  }
}

@on(event(approvals.requested))
job heard(r: approvals.Request) {
  Requests("all").add!(r)
}

## Writes down what was approved and by whom, as an audit log would.
entity Audit(name: String) {
  state lines: List(String) = []

  pub fn add(line: String) { lines += [line] }
}

job retire(name: String) {
  match approval!("Retire {name}?", from: "operator", within: 1 h) {
    Approved(ok) => Audit("log").add!("{ok.by} approved: {ok.what}")
    Declined | Expired => Audit("log").add!("kept {name}")
  }
}

test "an approval waits until the person asked approves" {
  ask.start("router")
  advance(30 min)
  expect(Ask("router").outcome == "waiting")
  approve("Restart router?")
  expect(Ask("router").outcome == "approved")
}

test "an answer from someone other than the person asked is refused, and the run keeps waiting" {
  ask.start("router")
  approve("Restart router?", by: "ann")
  expect(Ask("router").outcome == "waiting")
  approve("Restart router?", by: "operator")
  expect(Ask("router").outcome == "approved")
}

test "a declined approval is Declined" {
  ask.start("router")
  decline("Restart router")
  expect(Ask("router").outcome == "declined")
}

test "an approval nobody answers is Expired once its time is up, not before" {
  ask.start("router")
  advance(59 min)
  expect(Ask("router").outcome == "waiting")
  advance(1 min)
  expect(Ask("router").outcome == "expired")
}

test "a request names whom it asks, as text" {
  ask.start("router")
  expect(Requests("all").from == ["operator"])
}

test "an answer after the approval expired changes nothing" {
  ask.start("router")
  advance(2 h)
  let [id] = Requests("all").ids else { unreachable("asked once") }
  approvals.answered.publish({ id, approved: true, by: "operator" })
  expect(Ask("router").outcome == "expired")
}

test "an answer goes only to the approval whose text contains it" {
  ask.start("router")
  ask.start("switch")
  approve("Restart switch")
  expect(Ask("switch").outcome == "approved")
  expect(Ask("router").outcome == "waiting")
}

test "two runs that ask the same thing are answered one by one, by id" {
  ask.start("router")
  ask.start("router")
  let [first, second] = Requests("all").ids else { unreachable("asked twice") }
  expect(first != second)
  approvals.answered.publish({ id: second, approved: false, by: "operator" })
  expect(Ask("router").outcome == "declined")
  approvals.answered.publish({ id: first, approved: true, by: "operator" })
  expect(Ask("router").outcome == "approved")
}

test "an answer by id from someone else is refused" {
  ask.start("router")
  let [id] = Requests("all").ids else { unreachable("asked once") }
  approvals.answered.publish({ id, approved: true, by: "ann" })
  expect(Ask("router").outcome == "waiting")
  approvals.answered.publish({ id, approved: false, by: "operator" })
  expect(Ask("router").outcome == "declined")
}

test "a run that asks again after Expired waits for the new approval" {
  askUntilAnswered.start("keys")
  advance(1 day)
  expect(Ask("keys").outcome == "waiting" && Ask("keys").times == 1)
  advance(1 h)
  expect(Ask("keys").times == 2)
  approve("Rotate keys")
  expect(Ask("keys").outcome == "approved" && Ask("keys").times == 2)
}

test "an Approval says what was approved and by whom" {
  retire.start("printer")
  approve("Retire printer?")
  expect(Audit("log").lines == ["operator approved: Retire printer?"])
}
