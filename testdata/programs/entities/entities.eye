# Entities: one writer at a time, and ending one with a person's approval.
# interp/entities_test.go shows what a fault part way through a call rolls
# back, which a test here can't watch without failing.

entity Counter(id: String) {
  state n: Int = 0

  ## Counts one more, and gives the count this transaction saw.
  pub fn bump() -> Int {
    n += 1
    n
  }
}

## Waits, then bumps the counter.
fn bumpAfter(d: Duration) -> Int {
  sleep(d)
  Counter("c").bump()
}

test "one writer at a time: tasks that call a method at once lose no change" {
  let seen = [3, 1, 2, 1, 3, 2, 1, 2, 3, 1] |> parallelMap(fn n: bumpAfter(1 min * n))
  expect(Counter("c").n == 10)
  # Each call saw the count the one before it left: no two ran at once.
  expect(seen |> toSet |> size == 10)
  expect(Counter("c").read().version == 10)
}

# A call tree that comes back to an entity shares its state: Ping.ping
# counts one, Pong.pong calls Ping.bump, which counts another, and Ping
# commits once with both.

entity Ping(id: String) {
  state n: Int = 0
  state log: List(String) = []

  ## Counts one, has Pong come back to bump, and gives the count after.
  pub fn ping() -> Int {
    n += 1
    let before = n
    Pong(id).pong()
    log += ["{before} before pong, {n} after"]
    n
  }

  pub fn bump() { n += 1 }
}

entity Pong(id: String) {
  state saw: Int = 0

  pub fn pong() {
    saw = Ping(id).n
    Ping(id).bump()
  }
}

test "a call that comes back to an entity sees and keeps its changes" {
  expect(Ping("x").ping() == 2)
  expect(Ping("x").n == 2)
  expect(Ping("x").log == ["1 before pong, 2 after"])
  expect(Ping("x").read().version == 1)
  expect(Pong("x").saw == 1)
}

entity Player(id: String) {
  state name: String = ""

  pub fn rename(to: String) { name = to }

  pub fn retire(ok: Approval) { end(ok) }

  ## Ends the player, and changes it after: the change is dropped with it.
  pub fn retireAndRename(ok: Approval) {
    end(ok)
    name = "gone"
  }
}

view named = Player.all |> filter(fn p: p.name != "")

entity Departures(id: String) {
  state seen: List(String) = []

  pub fn add(who: String) { seen += [who] }
}

## Notes each player who leaves the view: renamed to "", or ended. An ended
## player can't be read, but its identity can.
@on(leaves(named))
job farewell(p: Ref(Player)) { Departures("all").add!(p.id) }

job retireWhenApproved(id: String) {
  match approval!("Retire {id}?", from: "operator", within: 1 day) {
    Approved(ok) => Player(id).retire!(ok)
    Declined => log.info("{id} stays")
    Expired => log.info("nobody answered for {id}")
  }
}

job retireAndRenameWhenApproved(id: String) {
  let Approved(ok) = approval!("Retire {id}?", from: "operator", within: 1 day) else { return }
  Player(id).retireAndRename!(ok)
}

test "an approved end removes the entity, which leaves its views" {
  Player("ann").rename("Ann")
  Player("bob").rename("Bob")
  retireWhenApproved.start("ann")
  expect(Player.all |> size == 2)
  approve("Retire ann?")
  expect(Player.all |> map(fn p: p.id) == ["bob"])
  expect(named |> map(fn p: p.id) == ["bob"])
  expect(Departures("all").seen == ["ann"])
}

test "a declined end leaves the entity as it was" {
  Player("ann").rename("Ann")
  retireWhenApproved.start("ann")
  decline("Retire ann?")
  expect(Player("ann").name == "Ann")
  expect(Player.all |> size == 1)
}

test "an ended entity's reference stays a value with its identity" {
  let ann = Player("ann")
  ann.rename("Ann")
  retireWhenApproved.start("ann")
  approve("Retire ann?")
  expect(ann.id == "ann")
  expect([ann] |> map(fn p: p.id) == ["ann"])
  expect(Player.all |> size == 0)
}

test "what a method changes after end(ok) is dropped" {
  Player("ann").rename("Ann")
  retireAndRenameWhenApproved.start("ann")
  approve("Retire ann?")
  expect(Player.all |> size == 0)
  expect(named |> size == 0)
  expect(Departures("all").seen == ["ann"])
}
