# Values laid out by type (ROADMAP 7.13): each kind of layout the C back
# end chooses, made, taken apart, compared, shown, kept in collections and
# closures, and updated in place or copied. The tests check the results on
# both engines.

type Kind = Ident | Number | Plus | Minus

type Token = { kind: Kind, start: Int, end: Int, space: Bool }

type Point = { x: Float, y: Float }

## More than 64 bytes: boxed.
type Big = { a: String, b: String, c: String, d: Int, e: Int, f: Int, g: Int, h: Int }

type Tree = Leaf | Node(Tree, Int, Tree)

type Shape = Circle(r: Float) | Rect(w: Float, h: Float) | Dot

type Bag = { name: String, items: List(Int) }

let origin: Point = { x: 0.0, y: 0.0 }

fn insert(t: Tree, n: Int) -> Tree = match t {
  Leaf => Node(Leaf, n, Leaf)
  Node(l, m, r) => if n < m then Node(insert(l, n), m, r) else Node(l, m, insert(r, n))
}

fn walk(t: Tree) -> List(Int) = match t {
  Leaf => []
  Node(l, n, r) => walk(l) + [n] + walk(r)
}

fn area(s: Shape) -> Float = match s {
  Circle(r) => 3.0 * r * r
  Rect(w, h) => w * h
  Dot => 0.0
}

fn moved(p: Point = origin, by: Float) -> Point = { p with x: p.x + by }

fn grow(mut b: Bag, n: Int) {
  b.items += [n]
  b.name = "{b.name}+"
}

fn kindOf(c: String) -> Option(Kind) = match c {
  "+" => Some(Plus)
  "-" => Some(Minus)
  _ => None
}

trait Named {
  fn label(self) -> String
}

impl Named for Point {
  fn label(self) -> String = "({self.x}, {self.y})"
}

test "enums and Options of them" {
  let kinds = [Ident, Number, Plus]
  expect(kinds[2] == Some(Plus))
  expect(kindOf("-") == Some(Minus) && kindOf("x") == None)
  expect("{kinds}" == "[Ident, Number, Plus]")
  expect(Plus.index() == 2)
}

test "records inline" {
  let t: Token = { kind: Number, start: 3, end: 5, space: true }
  let u = { t with end: 9 }
  expect(t.end == 5 && u.end == 9 && u.kind == Number)
  expect("{u}" == "\{ kind: Number, start: 3, end: 9, space: true \}")
  var tokens: List(Token) = []
  for i in 0..100 {
    tokens += [{ kind: Ident, start: i, end: i + 1, space: false }]
  }
  expect(tokens.size() == 100 && tokens[99] == Some({ kind: Ident, start: 99, end: 100, space: false }))
  let starts = tokens |> map(fn t: t.start) |> sum
  expect(starts == 4950)
}

test "records boxed" {
  var b: Big = { a: "a", b: "b", c: "c", d: 1, e: 2, f: 3, g: 4, h: 5 }
  let c = b
  b.d = 10
  b = { b with a: "z" }
  expect(c.d == 1 && c.a == "a" && b.d == 10 && b.a == "z")
  expect(b != c && c == { a: "a", b: "b", c: "c", d: 1, e: 2, f: 3, g: 4, h: 5 })
  let bigs = [b, c]
  expect(bigs |> map(fn x: x.d) |> sum == 11)
}

test "recursive variants" {
  var t = Leaf
  for n in [5, 2, 8, 1, 9, 3] {
    t = insert(t, n)
  }
  expect(walk(t) == [1, 2, 3, 5, 8, 9])
  let some: Option(Tree) = Some(t)
  expect(match some {
    Some(Node(_, n, _)) => n == 5
    _ => false
  })
  expect("{insert(Leaf, 1)}" == "Node(Leaf, 1, Leaf)")
}

test "variants with a tag" {
  let shapes = [Circle(r: 1.0), Rect(w: 2.0, h: 3.0), Dot]
  expect(shapes |> map(area) == [3.0, 6.0, 0.0])
  expect("{shapes}" == "[Circle(r: 1.0), Rect(w: 2.0, h: 3.0), Dot]")
  expect(shapes[1] == Some(Rect(w: 2.0, h: 3.0)))
}

test "Options in a niche and behind a tag" {
  let a: Option(Float) = Some(1.5)
  let b: Option(Bool) = Some(false)
  let c: Option(Option(Int)) = Some(None)
  let d: Option(Option(Int)) = None
  expect(a == Some(1.5) && b == Some(false) && c != d)
  expect("{c} {d} {b}" == "Some(None) None Some(false)")
  let m = { "one": 1, "two": 2 }
  expect((m["two"] ?? 0) + (m["three"] ?? 0) == 2)
}

test "tuples, sets and maps of structs" {
  let pairs = [(Plus, 1), (Minus, 2)]
  let at: Point = { x: 1.0, y: 2.0 }
  let byKind: Map(Kind, Point) = { (Plus): at }
  expect(byKind[Plus] == Some(at))
  let seen = toSet([(1, "a"), (2, "b"), (1, "a")])
  expect(seen.size() == 2 && seen.contains((2, "b")))
  expect(pairs |> map(fn (k, n): "{k}{n}") == ["Plus1", "Minus2"])
  let far: Point = { x: 2.0, y: 0.0 }
  let near: Point = { x: 1.0, y: 5.0 }
  let sorted = [far, near] |> sort
  expect(sorted[0] == Some(near))
  for (i, p) in [origin, moved(by: 1.0)] |> enumerate {
    expect(p.x == i.toFloat())
  }
}

test "closures and dyn values hold structs" {
  let p: Point = { x: 1.0, y: 2.0 }
  let shift = fn (q: Point): { q with y: q.y + p.y }
  let up: Point = { x: 0.0, y: 2.0 }
  expect(shift(origin) == up)
  let named: List(dyn Named) = [p, origin]
  expect(named |> map(fn n: n.label()) == ["(1.0, 2.0)", "(0.0, 0.0)"])
  let total = [p, p] |> reduce(origin, fn (acc, q): { acc with x: acc.x + q.x, y: acc.y + q.y })
  let want: Point = { x: 2.0, y: 4.0 }
  expect(total == want)
}

test "updates in place and copies" {
  var b: Bag = { name: "b", items: [] }
  let before = b
  for n in 0..5 {
    grow(mut b, n)
  }
  expect(b.items == [0, 1, 2, 3, 4] && b.name == "b+++++")
  expect(before.items == [] && before.name == "b")
  var grid = [[0, 0], [0, 0]]
  grid[1][0] = 7
  expect(grid == [[0, 0], [7, 0]])
  var tokens: List(Token) = [{ kind: Ident, start: 0, end: 1, space: false }]
  tokens[0].end = 4
  expect(tokens[0] == Some({ kind: Ident, start: 0, end: 4, space: false }))
}

test "results of structs" {
  let r: Result(Point, String) = Ok(origin)
  let e: Result(Point, String) = Err("no")
  expect(r.isOk() && e.isErr())
  expect(r.map(fn p: p.x) == Ok(0.0))
  expect(e.mapErr(fn s: s.size()) == Err(2))
}
