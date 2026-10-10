fn lambdas(xs: List(Int)) {
  let inc = fn n: n + 1
  let add = fn (a, b): a + b
  let typed = fn (a: Int, b: Int = 2): a * b
  let nothing = fn { 42 }
  let block = fn item {
    let label = item.name
    "<{label}>"
  }
  let unitParams = fn (): ()
  let record = fn u: { u with seen: true }
  let nested = fn a: fn b: a + b
  let greedy = fn x: x |> f ?? g
  xs |> map(fn x: x * 2) |> filter(fn x: x > 2)
}
