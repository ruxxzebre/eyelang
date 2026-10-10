fn f(x: Int) -> Int {
  let a = x
    |> double
    |> triple
  if a > 0 {
    a
  } else {
    0
  }
}

fn call() = add(
  1,
  2,
)

entity E(id: Id) {
  state n: Int = 0
  pub fn bump() { n += 1 }
}
