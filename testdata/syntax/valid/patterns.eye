fn classify(x: Value) -> Int {
  match x {
    n => 1
  }
}

fn every(v: Value, xs: List(Int), pair: (Int, Option(Int))) -> String {
  let { name, age } = user
  let { name: n, .. } = user
  let [first, ..rest] = xs else { return "empty" }
  let (count, maybe) = pair
  match v {
    Number(0) => "zero"
    Number(-1) => "minus one"
    Number(1..10) => "small"
    Number(n) if n > 100 => "big"
    Number(_) => "number"
    Text("") => "empty text"
    Text(s) => s
    Flag(true) | Flag(false) => "flag"
    Pair(a: 1, ..) => "pair"
    Nothing(()) => "unit"
    List([]) => "empty list"
    List([x, y, ..more]) => "list"
    List([_]) => "one"
    Letter('a'..='z' | 'A'..='Z') => "letter"
    Record({ name, age: 3 }) => name
    Record({ .. }) => "record"
    Nested(Some(Some(x))) => "nested"
    Tuple((1, _)) => "tuple"
    _ => "other"
  }
}
