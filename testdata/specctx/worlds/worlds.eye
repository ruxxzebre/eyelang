# What SPECS.md §16's view assumes (context=worlds): the tour's worlds, with
# their phase and players.

type WorldId = | WorldId(String)

type PlayerId = | PlayerId(String)

type Phase = Asleep | Waking | Running

entity World(id: WorldId) {
  state phase: Phase = Asleep
  state players: Set(PlayerId) = {}

  pub fn join(p: PlayerId) { players = players |> add(p) }

  pub fn leave(p: PlayerId) { players = players |> remove(p) }

  pub fn markRunning() { phase = Running }
}
