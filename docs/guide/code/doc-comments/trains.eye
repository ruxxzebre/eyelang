## Train reminders: when to leave for the next train home.

# region types
## A train between two stations.
pub type Departure = {
  ## When it leaves, as timetabled.
  leaves: Instant,
  ## How late it is, as the operator last reported.
  delay: Duration,
}

## Whether a trip ended well.
pub type Trip =
  ## Arrived on time.
  | OnTime
  ## Arrived late, by this many minutes.
  | Late(Int)
# endregion types

# region next
## The first departure in `trains` that leaves after `now`, if there is one.
##
## Trains that are late count from when they will actually leave. See
## [Departure.delay].
pub fn next(trains: List(Departure), now: Instant) -> Option(Departure) =
  trains |> find(fn d: d.leaves + d.delay > now)
# endregion next

test "a late train still counts" {
  with clock(2026-10-10 18:00 in "Europe/Dublin")
  let gone = { leaves: now() - 5 min, delay: 0 min }
  let late = { leaves: now() - 5 min, delay: 10 min }
  expect(next([gone, late], now()) == Some(late))
  expect(next([gone], now()) == None)
}
