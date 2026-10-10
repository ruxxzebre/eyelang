let a = match x {
  Some(u) => { u with seen: true }
  Named(n) => { name: n,  # a comment
    age: 3 }
  Pair(a, b) => { a, b }
  One(v) => { v }
  Run(v) => {
    log(v)
    v
  }
  None => {}
}
