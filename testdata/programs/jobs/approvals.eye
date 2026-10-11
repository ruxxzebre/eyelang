# A run waits at approval! until a person answers or its time runs out.

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

test "an approval waits until someone approves" {
  ask.start("router")
  advance(30 min)
  expect(Ask("router").outcome == "waiting")
  approve("Restart router?", by: "ann")
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

test "an answer after the approval expired changes nothing" {
  ask.start("router")
  advance(2 h)
  approve("Restart router")
  expect(Ask("router").outcome == "expired")
}

test "an answer given before a run asks doesn't answer it" {
  approve("Restart router")
  ask.start("router")
  expect(Ask("router").outcome == "waiting")
  advance(1 h)
  expect(Ask("router").outcome == "expired")
}

test "an answer goes only to approvals whose text contains it" {
  ask.start("router")
  ask.start("switch")
  approve("Restart switch")
  expect(Ask("switch").outcome == "approved")
  expect(Ask("router").outcome == "waiting")
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
