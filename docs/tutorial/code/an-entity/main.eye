# Tutorial 4: an entity. A shop's shelves, one entity per item.

# region error
type StockError = | OutOfStock(left: Int)
# endregion error

# region shelf
entity Shelf(item: String) {
  state count: Int = 0
  state held: Map(String, Int) = {}

  pub fn restock(n: Int) { count += n }

  pub fn hold(order: String, n: Int) -> Result((), StockError) {
    if n > count { return Err(OutOfStock(left: count)) }
    count -= n
    held = held |> insert(order, n)
    Ok(())
  }

  pub fn release(order: String) {
    let n = held[order] ?? 0
    count += n
    held = held |> remove(order)
  }
}
# endregion shelf

# region test-shelf
test "an entity keeps its state between calls" {
  let apples = Shelf("apples")
  expect(apples.count == 0)
  apples.restock(5)
  expect(apples.hold("order-1", 3) == Ok(()))
  expect(apples.count == 2)
  expect(apples.hold("order-2", 3) == Err(OutOfStock(left: 2)))
  apples.release("order-1")
  expect(apples.count == 5)
}
# endregion test-shelf

# region identity
test "the same id is the same entity" {
  Shelf("pears").restock(4)
  expect(Shelf("pears").count == 4)
  expect(Shelf("plums").count == 0)
}
# endregion identity

# region refs
type Line = { shelf: Ref(Shelf), n: Int }

fn holdAll(order: String, lines: List(Line)) -> List(String) {
  var short = []
  for line in lines {
    match line.shelf.hold(order, line.n) {
      Ok(_) => ()
      Err(OutOfStock(_)) => short += [line.shelf.item]
    }
  }
  short
}
# endregion refs

# region test-refs
test "a Ref names an entity, and can be stored" {
  Shelf("milk").restock(2)
  Shelf("bread").restock(1)
  let lines: List(Line) = [
    { shelf: Shelf("milk"), n: 1 },
    { shelf: Shelf("bread"), n: 2 },
  ]
  expect(holdAll("order-7", lines) == ["bread"])
  expect(Shelf("milk").count == 1)
  let snapshot = Shelf("milk").read()
  expect(snapshot.held == { "order-7": 1 })
  expect(snapshot.version == 2)
}
# endregion test-refs

# region all
test "Shelf.all lists every shelf" {
  Shelf("tea").restock(3)
  Shelf("jam").restock(0)
  let stocked = Shelf.all |> filter(fn s: s.count > 0) |> map(fn s: s.item)
  expect(stocked == ["tea"])
}
# endregion all
