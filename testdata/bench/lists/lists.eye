# Lists: map, filter and sum over a list of whole numbers, then sort a list
# of pseudo-random ones.

pub fn main(args: List(String)) uses {stdio} {
  let n = match parseInt(args[1] ?? "") {
    Ok(n) => n
    Err(_) => 1_000_000
  }
  let xs = (0..n) |> toList
  let total = xs |> map(fn x: x * x % 1000) |> filter(fn x: x % 2 == 0) |> sum
  var seed = 42
  var ys: List(Int) = []
  for _ in 0..n {
    seed = (seed * 1103515245 + 12345) % 2147483648
    ys += [seed % 1_000_000]
  }
  let sorted = ys |> sort
  let low = sorted[0] ?? -1
  let middle = sorted[n / 2] ?? -1
  let high = sorted[n - 1] ?? -1
  print("sum {total}; sorted {n}: {low} … {middle} … {high}")
}
