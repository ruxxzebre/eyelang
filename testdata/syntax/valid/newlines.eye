fn layout(worlds: List(World), a: Int, b: Int) -> Int {
  let ready = worlds
    |> filter(fn w: w.phase == Running)
    |> size

  let sum = a
    + b
    * 2

  let checked = a > 0
    && b > 0
    || a == b

  let chained = text
    .trim()
    .lower()

  let fallback = maybe
    ?? 0

  var total = 0
  total
    += 1
  total
    -= 1

  if ready > 0 {
    ready
  }
  else if sum > 0 {
    sum
  }
  else {
    0
  }
}

fn call(
  a: Int,
  b: Int,
) -> Int = add(
  a,
  b,
)
