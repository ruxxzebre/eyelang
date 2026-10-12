# Late orders. A shop's orders, a rule for when one is late, and a job that
# tells the team about late orders every evening.

import chat

let dublin: Zone = "Europe/Dublin"

pub type OrderId = | OrderId(String)

entity Order(id: OrderId) {
  state placed: Option(Instant) = None
  state shipped: Bool = false
  state flagged: Bool = false

  pub fn place(at: Instant) {
    if placed == None { placed = Some(at) }
  }

  pub fn ship() { shipped = true }

  pub fn flag() { flagged = true }
}

# region pure
## Whether an order is late: not shipped two days after it was placed. It
## takes the time as an argument, so it is pure, and a test can pass any time.
pub fn isLate(placed: Instant, shipped: Bool, at: Instant) -> Bool =
  !shipped && at - placed > 2 days
# endregion pure

# region uses
## The orders that are late now.
pub fn lateOrders() -> List(OrderId) uses {clock, entity Order} {
  let at = now()
  Order.all
    |> filter(fn o: match o.placed {
      Some(placed) => isLate(placed, o.shipped, at)
      None => false
    })
    |> map(fn o: o.id)
}
# endregion uses

# region inferred
## The line the team reads. Private and pure.
fn summary(late: List(OrderId)) -> String =
  "{late |> size} orders are late: {late |> map(fn id: "{id}") |> join(", ")}"

## Posts to the team's channel. Private, so its effects are inferred:
## wait and net Chat.post.
@unwitnessed("Chat refuses only when the channel is archived, which no recording has")
fn post(text: String, key: Key) {
  match chat.post!(text, key:).reconcile!() {
    Posted(_) => ()
    Refused(error) => log.warn("the chat refused {key}: {error}")
  }
}

## Tells the team which orders are late. Public, so it states what post does.
pub fn tellLate(late: List(OrderId), key: Key) uses {wait, net Chat.post} {
  if late == [] { return }
  post!(summary(late), key:)
}
# endregion inferred

# region callback
## Calls `f` with each late order. It states only its own effects; a caller
## adds the effects of the function it passes.
pub fn eachLate(f: fn(OrderId)) uses {clock, entity Order} {
  for id in lateOrders() { f(id) }
}

## Prints the late orders. The print is in the lambda, so stdio is this
## function's effect, not eachLate's.
pub fn printLate() uses {clock, entity Order, stdio} {
  eachLate(fn id: print("late: {id}"))
}
# endregion callback

# region job
## Every evening, flags the late orders and tells the team. Each `!` is a
## step whose result is saved, so a restart repeats none of them.
@on(daily(18:00, in: dublin))
job evening(day: Date) {
  let late = lateOrders!()
  for id in late { Order(id).flag!() }
  tellLate!(late, key: "late-{day}")
}
# endregion job

# ---------------------------------------------------------------- tests

# region tests
test "an order is late two days after it was placed, until it ships" {
  let placed = 2026-10-12 10:00 in dublin
  expect(!isLate(placed, false, placed + 47 h))
  expect(isLate(placed, false, placed + 49 h))
  expect(!isLate(placed, true, placed + 49 h))
}

test "every evening the team hears about late orders" {
  with fakes(chat, from: "recordings/")
  with clock(2026-10-12 09:00 in dublin)
  Order(OrderId("o-1")).place(now())
  Order(OrderId("o-2")).place(now())
  Order(OrderId("o-2")).ship()
  advance(2 days)  # the evening of the 12th and the 13th find nothing late
  expect(!Order(OrderId("o-1")).flagged)
  advance(9 h)  # 18:00 on the 14th
  expect(Order(OrderId("o-1")).flagged)
  expect(!Order(OrderId("o-2")).flagged)
  expect(lateOrders() == [OrderId("o-1")])
  printLate()  # prints "late: o-1"
}
# endregion tests
