# A program that serves Shelves and calls it. A @fresh command gets the version its caller read,
# and answers Stale when the shelf changed after it, or Expired when the shelf is gone. A
# @destructive command takes the Approval that approval! gives.

import shelves.{Cleared, Burned}

entity Rack(id: String) {
  state items: List(String) = []
  state gone: Bool = false

  pub fn put(item: String) { items += [item] }

  pub fn empty() -> Int {
    let n = items |> size
    items = []
    n
  }

  pub fn burn() -> Int {
    let n = items |> size
    items = []
    gone = true
    n
  }
}

entity Log(name: String) {
  state lines: List(String) = []

  pub fn add(line: String) { lines += [line] }
}

impl shelves.Shelves {
  fn shelf(id) {
    let rack = Rack(id).read()
    { id, items: rack.items, version: rack.version }
  }

  fn clear(id, basedOn) {
    let rack = Rack(id).read()
    if rack.gone { return Expired }
    if rack.version != basedOn { return Stale }
    Cleared(count: Rack(id).empty())
  }

  fn burn(id, approval) = Burned(count: Rack(id).burn())
}

## Clears a shelf from what it read, and reads it again when it changed in between.
job tidy(id: String) {
  var reads = 0
  while true {
    reads += 1
    let shelf = shelves.shelf!(id)
    sleep!(1 min)
    match shelves.clear!(id, basedOn: shelf.version) {
      Cleared(n) => {
        Log(id).add!("cleared {n} after {reads} reads")
        return
      }
      Stale => ()
      Expired => {
        Log(id).add!("gone")
        return
      }
      Uncertain(_) => {
        Log(id).add!("uncertain")
        return
      }
    }
  }
}

## Burns a shelf down once a person approves.
job demolish(id: String) {
  match approval!("Burn shelf {id}?", from: "operator", within: 1 h) {
    Approved(ok) => {
      let Burned(n) = shelves.burn!(id, approval: ok) else { return }
      Log(id).add!("burned {n}")
    }
    Declined | Expired => Log(id).add!("kept")
  }
}

test "a fresh command acts on the shelf as it was read" {
  Rack("a").put("apple")
  let shelf = shelves.shelf("a")
  expect(shelves.clear("a", basedOn: shelf.version) == Cleared(count: 1))
  expect(Rack("a").items == [])
}

test "a fresh command based on an older version is Stale, and changes nothing" {
  Rack("a").put("apple")
  let shelf = shelves.shelf("a")
  Rack("a").put("pear")
  expect(shelves.clear("a", basedOn: shelf.version) == Stale)
  expect(Rack("a").items == ["apple", "pear"])
}

test "a fresh command on a shelf that is gone is Expired" {
  let shelf = shelves.shelf("a")
  let _ = Rack("a").burn()
  expect(shelves.clear("a", basedOn: shelf.version) == Expired)
}

test "a job reads the shelf again after Stale" {
  Rack("b").put("plum")
  tidy.start("b")
  Rack("b").put("fig")
  advance(1 min)
  expect(Log("b").lines == [])
  advance(1 min)
  expect(Log("b").lines == ["cleared 2 after 2 reads"])
}

test "a destructive command runs with the approval a person gave" {
  Rack("c").put("crate")
  demolish.start("c")
  expect(!Rack("c").gone)
  approve("Burn shelf c")
  expect(Rack("c").gone && Log("c").lines == ["burned 1"])
}

test "a declined approval burns nothing" {
  demolish.start("c")
  decline("Burn shelf c")
  expect(!Rack("c").gone && Log("c").lines == ["kept"])
}

test "a fake receives basedOn, and can answer Stale" {
  let fake = fakes(shelves, from: "recordings/")
  with fake
  let shelf = shelves.shelf("d")
  expect(shelf.version == 7)
  expect(shelves.clear("d", basedOn: shelf.version) == Stale)
  expect(fake.calls(shelves.clear) == [{ id: "d", basedOn: 7 }])
}
