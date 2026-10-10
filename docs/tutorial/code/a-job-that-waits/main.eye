# Tutorial 5: a job that waits. A hold on stock lasts 15 minutes, unless the
# order is paid for first.

type StockError = | OutOfStock(left: Int)

# region shelf
entity Shelf(item: String) {
  state count: Int = 0
  state held: Map(String, Int) = {}

  pub fn restock(n: Int) { count += n }

  pub fn hold(order: String, n: Int) -> Result((), StockError) {
    if n > count { return Err(OutOfStock(left: count)) }
    count -= n
    held = held |> insert(order, n)
    expire.start(item, order)
    Ok(())
  }

  pub fn sell(order: String) { held = held |> remove(order) }

  pub fn release(order: String) -> Bool {
    let Some(n) = held[order] else { return false }
    count += n
    held = held |> remove(order)
    true
  }
}
# endregion shelf

# region job
job expire(item: String, order: String) {
  sleep!(15 min)
  if Shelf(item).release!(order) {
    log.info("released {order}'s hold on {item} at {now!()}")
  }
}
# endregion job

# region test
test "a hold ends after 15 minutes unless the order is paid" {
  with clock(2026-10-10 12:00 in "Europe/Dublin")
  let apples = Shelf("apples")
  apples.restock(5)
  expect(apples.hold("order-1", 2) == Ok(()))
  expect(apples.hold("order-2", 1) == Ok(()))
  expect(apples.count == 2)

  advance(10 min)
  apples.sell("order-2")
  expect(apples.count == 2)

  advance(5 min)
  expect(apples.count == 4)
  expect(apples.held == {})
}
# endregion test

# region main
pub fn main() uses {entity Shelf} {
  let apples = Shelf("apples")
  if apples.read().version == 0 {
    apples.restock(5)
    let _ = apples.hold("order-1", 2)
  }
}
# endregion main
