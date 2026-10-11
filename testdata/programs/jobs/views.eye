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
