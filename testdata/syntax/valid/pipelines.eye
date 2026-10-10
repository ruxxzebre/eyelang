fn report(urls: List(Url)) -> String {
  let sizes = urls |> map(fetchSize) |> filter(fn s: s > 0)
  let diff = 2 |> sub(10, _)
  let total = sizes
    |> map(fn s: s * 2)
    |> fold(0, fn (acc, s): acc + s)
  let label = total |> format(_, unit: "bytes")
  let biggest = sizes |> sortBy(fn s: -s) |> take(3)
  "{label}: {biggest}, {diff}"
}
