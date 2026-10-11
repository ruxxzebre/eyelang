# Guide: testing. A morning digest: the items that came in overnight, sent to
# one person through Notify at 07:30, Irish time.

import notify
import format.{headline}

let dublin: Zone = "Europe/Dublin"

# region inbox
## What came in since the last digest, and what was sent.
entity Inbox(owner: String) {
  state items: List(String) = []
  state sent: List(String) = []

  pub fn add(item: String) { items += [item] }

  pub fn delivered(id: String) {
    sent += [id]
    items = []
  }
}
# endregion inbox

# region digest
## Sends the morning digest, if anything came in.
@on(daily(07:30, in: dublin))
job digest(day: Date) {
  let inbox = Inbox("ann").read!()
  if inbox.items == [] { return }
  let text = "{headline(day, inbox.items |> size)}\n{inbox.items |> join("\n")}"
  match notify.send!(to: "ann", text:, key: "digest-{day}").reconcile!() {
    Accepted(id) => Inbox("ann").delivered!(id)
    Refused(error) => log.warn("Notify refused the digest for {day}: {error}")
  }
}
# endregion digest

# region test-digest
test "the digest goes out at 07:30" {
  with fakes(notify, from: "recordings/")
  with clock(2026-10-12 06:00 in dublin)
  Inbox("ann").add("Rail strike called off")
  advance(89 min)
  expect(Inbox("ann").sent == [])
  advance(1 min)
  expect(Inbox("ann").sent == ["m-1"])
  expect(Inbox("ann").items == [])
}
# endregion test-digest

# region test-fresh
test "each test starts with an empty store" {
  expect(Inbox("ann").items == [] && Inbox("ann").sent == [])
}
# endregion test-fresh

# region test-lost
test "a lost reply is settled by asking Notify, not by sending again" {
  with fakes(notify, from: "recordings/")
  with clock(2026-10-13 07:00 in dublin)
  Inbox("ann").add("Storm warning for Tuesday")
  advance(30 min)
  expect(Inbox("ann").sent == ["m-2"])
}
# endregion test-lost

# region test-refused
test "a refused digest keeps the items for tomorrow" {
  with fakes(notify, from: "recordings/")
  with clock(2026-10-14 07:00 in dublin)
  Inbox("ann").add("Bin day moved")
  advance(1 h)
  expect(Inbox("ann").sent == [] && Inbox("ann").items == ["Bin day moved"])
}
# endregion test-refused

# region deadline
## Waits for the feed's morning fetch, which sometimes never ends.
@deadline(1 h)
job fetch(feed: String) -> Int {
  sleep!(2 h)
  0
}

## Fetches a feed as a child run, and keeps how the run ended.
job refresh(feed: String) {
  let outcome = fetch.start!(feed).join!()
  Feed(feed).ended!(outcome)
}

entity Feed(name: String) {
  state last: Option(Outcome(Int)) = None

  pub fn ended(o: Outcome(Int)) { last = Some(o) }
}
# endregion deadline

# region test-deadline
test "a run that passes its deadline ends as an outcome" {
  refresh.start("ireland")
  advance(59 min)
  expect(Feed("ireland").last == None)
  advance(1 min)
  expect(Feed("ireland").last == Some(Failed(DeadlinePassed)))
}
# endregion test-deadline
