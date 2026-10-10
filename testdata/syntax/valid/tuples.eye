fn minMax(xs: List(Int)) -> Option((Int, Int)) {
  let [first, ..] = xs else { return None }
  var low = first
  var high = first
  for x in xs {
    if x < low { low = x }
    if x > high { high = x }
  }
  Some((low, high))
}

fn zipped(names: List(String), ages: List(Int)) -> List((String, Int)) =
  zip(names, ages)

fn nested() -> ((Int, Int), (String, Bool,),) = ((1, 2), ("a", true))

fn entries(m: Map(String, Int)) {
  for (key, value) in m {
    log.info("{key}: {value}")
  }
}
