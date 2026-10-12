# A view read by name is the list of its entities now.

entity Lamp(room: String) {
  state on: Bool = false
  state watts: Int = 0

  pub fn switch(w: Int) {
    on = w > 0
    watts = w
  }
}

view lit = Lamp.all |> filter(fn l: l.on)

test "a view is a list that loops, pipes and counts" {
  Lamp("hall").switch(40)
  Lamp("desk").switch(0)
  Lamp("porch").switch(60)
  var total = 0
  for l in lit {
    total += l.watts
  }
  expect(total == 100)
  expect(lit |> size == 2)
  expect(lit |> map(fn l: l.room) |> sort == ["hall", "porch"])
  Lamp("hall").switch(0)
  expect(lit |> map(fn l: l.room) == ["porch"])
}

# enters(view) starts a run for each entity that begins to match, and
# leaves(view) one for each that stops: each time, not only the first.

entity Switches(room: String) {
  state moves: List(String) = []

  pub fn add(move: String) { moves += [move] }
}

@on(enters(lit))
job lightUp(l: Ref(Lamp)) {
  Switches(l.room).add!("on")
}

@on(leaves(lit))
job lightDown(l: Ref(Lamp)) {
  Switches(l.room).add!("off")
}

test "leaves starts a run when an entity stops matching" {
  let porch = Lamp("porch")
  porch.switch(60)
  porch.switch(0)
  expect(Switches("porch").moves == ["on", "off"])
}

test "an entity that never matched doesn't leave" {
  Lamp("desk").switch(0)
  expect(Switches("desk").moves == [])
}

test "an entity that enters, leaves and enters again starts a run each time" {
  let hall = Lamp("hall")
  hall.switch(40)
  hall.switch(60)  # still lit: it doesn't enter again
  hall.switch(0)
  hall.switch(25)
  expect(Switches("hall").moves == ["on", "off", "on"])
  hall.switch(0)
  expect(Switches("hall").moves == ["on", "off", "on", "off"])
}
