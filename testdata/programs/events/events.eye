# Events start jobs, and jobs wait for them with next!.

import bus.{Ping}

entity Inbox(name: String) {
  state got: List(String) = []
  state started: Int = 0

  pub fn add(text: String) { got += [text] }

  pub fn count() { started += 1 }
}

## One run per event; a repeat of the same key starts nothing.
@on(event(bus.pings))
job receive(p: Ping) {
  Inbox("all").count!()
}

## Sends, then waits for the ping that answers it.
job await(text: String) {
  bus.send!(text, key: "send-{text}").reconcile!()
  let found = next!(bus.pings, where: fn p: p.text == text, within: 10 min)
  match found {
    Some(p) => Inbox("await").add!("{text}: {p.id}")
    None => Inbox("await").add!("{text}: none")
  }
}

## Sleeps first: a ping that comes while it sleeps still counts.
job later(text: String) {
  sleep!(5 min)
  let found = next!(bus.pings, where: fn p: p.text == text, within: 10 min)
  Inbox("later").add!("{text}: {found |> map(fn p: p.id) ?? 0}")
}

test "an event starts one run per key" {
  bus.pings.publish({ id: 1, text: "a" })
  bus.pings.publish({ id: 1, text: "a" })
  expect(Inbox("all").started == 1)
  bus.pings.publish({ id: 2, text: "b" })
  expect(Inbox("all").started == 2)
}

test "next! wakes on the event it waits for, after a chained call" {
  with fakes(bus, from: "recordings/")
  await.start("a")
  bus.pings.publish({ id: 7, text: "other" })
  expect(Inbox("await").got == [])
  bus.pings.publish({ id: 8, text: "a" })
  expect(Inbox("await").got == ["a: 8"])
}

test "next! gives None when its time runs out" {
  with fakes(bus, from: "recordings/")
  await.start("b")
  advance(11 min)
  expect(Inbox("await").got == ["b: none"])
}

test "next! sees events published since the run's previous step" {
  later.start("c")
  advance(2 min)
  bus.pings.publish({ id: 3, text: "c" })
  advance(5 min)
  expect(Inbox("later").got == ["c: 3"])
}
