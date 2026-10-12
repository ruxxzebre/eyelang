# SPECS.md section 7, example 2.
fn histogram(data: Bytes) -> List(Int) {
  var counts = (0..256) |> map(fn _: 0)
  for b in data {
    counts[b.toInt()] += 1
  }
  counts
}
