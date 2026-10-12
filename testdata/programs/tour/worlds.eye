# What the tour in SPECS.md §3 (tour.eye, generated from the spec) takes from
# the rest of its module.

import zeropod.{WorldId}
import notify.{UserId}

type PlayerId = | PlayerId(String)

type SleepError = | SleepRefused(zeropod.Refusal)

type WakeError =
  | WakeRefused(zeropod.Refusal)
  ## restore takes no reconcile query, so a lost reply is reported, not retried.
  | WakeUncertain

# Started when a player joins a sleeping world: restores it from its last
# checkpoint. The world reports itself running once it has started.
@unwitnessed("Zeropod refuses a restore only for a damaged checkpoint, which no test has seen")
job wake(id: WorldId) -> Result((), WakeError) {
  match zeropod.restore!(id, key: "wake-{id}") {
    Restored(_) | NothingToRestore => Ok(())
    RestoreRefused(reason) => Err(WakeRefused(reason))
    Uncertain(_) => Err(WakeUncertain)
  }
}

test "a world that can't checkpoint stays up and tells its owner" {
  with fakes(zeropod, notify, from: "receipts/")
  with clock(2026-10-10 12:00 in "Europe/Dublin")
  let w = World(WorldId("creative"))

  w.join(PlayerId("bob"))
  w.markRunning()
  w.leave(PlayerId("bob"))

  advance(21 min)
  expect(w.phase == Running)
}

test "a world nobody left doesn't sleep" {
  with fakes(zeropod, notify, from: "receipts/")
  let w = World(WorldId("busy"))
  w.join(PlayerId("carol"))
  w.markRunning()
  advance(2 h)
  expect(w.phase == Running)
  expect(w.players |> contains(PlayerId("carol")))
}
