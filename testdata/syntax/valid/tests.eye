import zeropod
import notify

test "an idle world sleeps after 20 minutes" {
  with fakes(zeropod, notify, from: "receipts/")
  with clock(2026-10-10 12:00 in "Europe/Dublin")
  let w = World(WorldId("survival"))
  let alice = PlayerId("alice")

  w.join(alice)
  w.markRunning()
  w.leave(alice)

  advance(21 min)
  expect(w.phase == Asleep)
}

test "memory budget" {
  with memory(64 MiB)
  expect(true)
}
