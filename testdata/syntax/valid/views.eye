view idleWorlds = World.all
  |> filter(fn w: w.phase == Running && w.players == {})

pub view busyWorlds = World.all
  |> filter(fn w: w.players |> size > 10)
  |> sortBy(fn w: -(w.players |> size))

@on(leaves(busyWorlds))
job relax(w: Ref(World)) { sleep!(5 min) }

@on(every(15 min))
@limit(4, per: 1 h)
job sweep() { cleanup!() }

@on(event(notify.delivered))
job confirm(e: Delivered) { log.info("delivered {e.key}") }
