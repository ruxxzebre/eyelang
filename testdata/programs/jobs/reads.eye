# A job reads an entity or a view only as a recorded step, so a run that
# replays the step gets what the first read saw, even when the entity has
# changed since (SPECS.md §15).

entity Gate(id: String) {
  state open: Bool = false

  pub fn swing() { open = true }
}

view openGates = Gate.all |> filter(fn g: g.open)

entity Sightings(id: String) {
  state seen: List(String) = []

  pub fn add(what: String) { seen += [what] }
}

## Reads the gate, the open gates and every gate, then sleeps. A job calls
## it in one statement, so a run that wakes replays all four steps, and the
## reads give what they gave before the sleep.
fn lookThenSleep(gate: Ref(Gate)) -> String {
  let open = gate.read!().open
  let opened = openGates.read!() |> size
  let gates = Gate.all.read!() |> size
  sleep!(10 min)
  "open: {open}, open gates: {opened}, gates: {gates}"
}

job watchGate(gate: Ref(Gate)) {
  let seen = lookThenSleep!(gate)
  Sightings(gate.id).add!(seen)
}

test "a recorded read keeps its value when the run replays it" {
  let front = Gate("front")
  let run = watchGate.start(front)
  advance(5 min)
  front.swing()
  Gate("back").swing()
  advance(10 min)
  # The gates are open now, but the run kept what it read before.
  expect(front.read().open)
  expect(Sightings("front").seen == ["open: false, open gates: 0, gates: 0"])
}

test "outside a job, a field of a reference is its snapshot's field" {
  let front = Gate("front")
  front.swing()
  expect(front.open == front.read().open)
  expect(openGates.read() == openGates)
}
