type Pair(a, b) = { first: a, second: b }
type Tree(a) = | Leaf | Node(left: Tree(a), value: a, right: Tree(a))

fn swap(p: Pair(a, b)) -> Pair(b, a) = { first: p.second, second: p.first }

fn insert(t: Tree(a), x: a) -> Tree(a) where a: Ord = match t {
  Leaf => Node(left: Leaf, value: x, right: Leaf)
  Node(left, value, right) if x < value => Node(left: insert(left, x), value, right)
  Node(left, value, right) => Node(left, value, right: insert(right, x))
}

fn largest(xs: List(a)) -> Option(a) where a: Ord {
  var best = first(xs)?
  for x in xs {
    if x > best { best = x }
  }
  Some(best)
}

fn describe(m: Map(k, v)) -> String where k: Show + Ord, v: Show =
  m |> entries |> map(fn (key, value): "{key}={value}") |> join(", ")

trait Container(a) {
  fn get(self, i: Int) -> Option(a)
  fn size(self) -> Int
}
