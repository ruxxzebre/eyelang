# SPECS.md section 5, example 7.
# The example is a list of statements, wrapped in a function.
fn example() {
  let squares = (1..1_000_000_000).iter() |> map(fn n: n * n) |> filter(fn n: n % 3 == 1)
  let few = squares |> take(3) |> toList    # [1, 4, 16]: squares four numbers, not a billion
  for (i, c) in "héllo".chars() |> enumerate { print(i, c) }
}
