fn ranges(xs: List(Int), bytes: Bytes, s: String, n: Int) {
  for i in 0..n { log.info(i) }
  for i in 1..=10 { log.info(i) }
  let window = xs[2..5]
  let head = xs[..n]
  let headInclusive = xs[..=n]
  let tail = bytes[n..]
  let middle = xs[n - 1..n + 1]
  let computed = xs[0..xs |> size]
  let text = s.slice(2..5) ?? ""
  let same = 0..n == 0..n
}

fn grade(score: Int) -> Char = match score {
  90..=100 => 'A'
  80..90 => 'B'
  -100..=-1 => '?'
  _ => 'C'
}
