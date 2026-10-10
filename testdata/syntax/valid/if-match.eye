fn choose(n: Int, x: Option(Int)) -> Int {
  let a = if n < 2 { n } else { fib(n - 1) + fib(n - 2) }
  let b = if a > 0 { 1 } else if a < 0 { -1 } else { 0 }
  let c = match x { Some(v) => v, None => 0 }
  let d = match x {
    Some(v) if v > 10 => {
      let doubled = v * 2
      doubled
    }
    Some(v) => v
    None => if n > 0 { n } else { 0 }
  }
  let e = match x { Some(u) => ({ value: u }), None => {} }
  if w.players == {} { 1 } else { 2 }
}
