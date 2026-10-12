## Courier pickups: which pickup a parcel packed now can still make.

# region types
## A courier's pickup at the warehouse.
pub type Pickup = {
  ## When the courier arrives, as booked.
  arrives: Instant,
  ## How long before that the parcels must be packed.
  cutoff: Duration,
}

## How a delivery went.
pub type Delivery =
  ## Delivered on the promised day.
  | OnTime
  ## Delivered late, by this many days.
  | Late(Int)
# endregion types

# region next
## The first pickup in `pickups` that a parcel packed at `now` can still
## make, if there is one.
##
## A parcel must be packed before the pickup's cutoff. See [Pickup.cutoff].
pub fn next(pickups: List(Pickup), now: Instant) -> Option(Pickup) =
  pickups |> find(fn p: p.arrives - p.cutoff > now)
# endregion next

test "a pickup past its cutoff is missed" {
  with clock(2026-10-12 16:00 in "UTC")
  let missed = { arrives: now() + 20 min, cutoff: 30 min }
  let later = { arrives: now() + 2 h, cutoff: 30 min }
  expect(next([missed, later], now()) == Some(later))
  expect(next([missed], now()) == None)
}
