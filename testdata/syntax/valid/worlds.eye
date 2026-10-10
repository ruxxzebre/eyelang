import zeropod

type Phase = Asleep | Waking | Running

entity World(id: WorldId) {
  state phase: Phase = Asleep
  state players: Set(PlayerId) = {}

  pub fn join(p: PlayerId) {
    players = players |> add(p)
    if phase == Asleep {
      phase = Waking
      wake.start(id)
    }
  }

  pub fn leave(p: PlayerId) { players = players |> remove(p) }
  pub fn markRunning() { phase = Running }
  pub fn markAsleep() { phase = Asleep }
}

view idleWorlds = World.all
  |> filter(fn w: w.phase == Running && w.players == {})

@on(enters(idleWorlds))
@priority(background)
@memory(16 MiB)
job sleepWhenIdle(w: Ref(World)) -> Result((), SleepError) {
  sleep!(20 min)
  let world = w.read!()
  if world.players != {} {
    return Ok(())
  }
  match zeropod.checkpoint!(w.id, key: "sleep-{w.id}-{world.version}").reconcile!() {
    Done(_) => {
      w.markAsleep!()
      Ok(())
    }
    Refused(reason) => Err(SleepRefused(reason))
  }
}

job wake(id: WorldId) {
  zeropod.restore!(id, key: "wake-{id}")
  World(id).markRunning!()
}
