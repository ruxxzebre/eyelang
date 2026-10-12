# A morning report. At 08:00 New York time on weekdays, the warehouse team
# hears how many paid orders wait to ship, and which has waited longest.
# The schedule carries its zone, so the report stays at 08:00 when the clocks
# change, and the key on the post means a restart never posts it twice.

import chat

let newYork: Zone = "America/New_York"

# region orders
entity Order(id: Int) {
  state cents: Int = 0
  state paidAt: Option(Instant) = None
  state shipped: Bool = false

  pub fn pay(amount: Int, at: Instant) {
    cents = amount
    paidAt = Some(at)
  }

  pub fn ship() { shipped = true }
}

## Every order that is paid for and not yet shipped. The runtime keeps it up
## to date as orders change, so the report reads it rather than every order.
view waiting = Order.all |> filter(fn o: o.paidAt != None && !o.shipped)
# endregion orders

# region summary
## An order that waits to ship, as the report sees it.
pub type Waiting = { id: Int, cents: Int, since: Instant }

## Writes cents as dollars: 41250 is "$412.50".
pub fn dollars(cents: Int) -> String {
  let rest = cents % 100
  "${cents / 100}.{if rest < 10 then "0{rest}" else "{rest}"}"
}

## The report's text, or None when nothing waits. It is pure: the orders and
## the time come in as arguments, so a test can pass any.
pub fn summary(orders: List(Waiting), at: Instant) -> Option(String) {
  let oldest = orders |> map(fn o: (o.since, o.id)) |> sort() |> first()?
  let (since, id) = oldest
  let total = orders |> map(fn o: o.cents) |> sum()
  let count = if orders |> size == 1 then "1 paid order waits" else "{orders |> size} paid orders wait"
  Some("{count} to ship, {dollars(total)} in all. #{id} has waited longest: {at - since}.")
}
# endregion summary

# region report
## Posts the report on weekday mornings. The key names the day, so however
## often this step runs, the team sees one report a day.
@on(weekdays(08:00, in: newYork))
job report(day: Date) {
  let at = now!()
  let open = waiting.read!() |> map(fn o: { id: o.id, cents: o.cents, since: o.paidAt ?? at })
  let Some(text) = summary(open, at) else { return }
  match chat.post!("#warehouse", text, key: "report-{day}").reconcile!() {
    Posted(_) => ()
    Refused(_) => log.warn("the chat refused the report for {day}")
  }
}
# endregion report

# ---------------------------------------------------------------- tests

# region test-summary
test "the summary counts the orders and finds the oldest" {
  let at = 2026-10-30 12:00 in "UTC"
  let orders = [{ id: 2, cents: 38_750, since: at - 1 day }, { id: 1, cents: 2_500, since: at - 3 days }]
  expect(summary(orders, at) == Some("2 paid orders wait to ship, $412.50 in all. #1 has waited longest: 3 days."))
  expect(summary([], at) == None)
}
# endregion test-summary

# region test-schedule
test "the report comes at 08:00 in New York, on both sides of the clock change" {
  let team = fakes(chat, from: "recordings/")
  with team
  with clock(2026-10-30 11:00 in "UTC")  # Friday, 07:00 in New York
  Order(1182).pay(4_250, now() - 1 day)
  Order(1183).pay(1_999, now())
  Order(1183).ship()
  advance(1 h)  # 08:00 in New York is 12:00 UTC on summer time
  let [friday] = team.calls(chat.post) else { unreachable("no report on Friday") }
  expect(friday.text == "1 paid order waits to ship, $42.50 in all. #1182 has waited longest: 25 h.")
  # No report at the weekend. On Sunday the clocks go back an hour, so
  # Monday's 08:00 is 13:00 UTC.
  advance(3 days)
  expect(team.calls(chat.post) |> size == 1)
  advance(1 h)
  let [_, monday] = team.calls(chat.post) else { unreachable("no report on Monday") }
  expect(monday.key == "report-2026-11-02")
}
# endregion test-schedule

test "nothing waiting, nothing posted" {
  let team = fakes(chat, from: "recordings/")
  with team
  with clock(2026-10-30 11:00 in "UTC")
  advance(1 day)
  expect(team.calls(chat.post) == [])
}
