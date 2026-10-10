# The core language, tested with itself: values, functions and control flow.

type Shape =
  | Circle(radius: Float)
  | Rect(w: Float, h: Float)
  | Dot

type Counter = { hits: Int, last: Option(Int) }

type ParseError = | NotANumber(String)

type CountError =
  | Usage
  | BadNumber(ParseError)

impl Shape {
  fn area(self) -> Float = match self {
    Circle(r) => 3.0 * r * r
    Rect(w, h) => w * h
    Dot => 0.0
  }
}

impl Counter {
  fn reset(mut self) { self.hits = 0 }
}

fn fib(n: Int) -> Int = if n < 2 then n else fib(n - 1) + fib(n - 2)

fn page(size: Int = 50, after: Option(Int) = None) -> String =
  "{size} after {after ?? 0}"

fn record(mut c: Counter, at: Int) {
  c.hits += 1
  c.last = Some(at)
}

fn parse(s: String) -> Result(Int, ParseError) = match parseInt(s) {
  Ok(n) => Ok(n)
  Err(_) => Err(NotANumber(s))
}

fn count(text: String) -> Result(Int, CountError) {
  if text == "" { return Err(Usage) }
  let n = parse(text)?
  Ok(n * 2)
}

fn firstEven(xs: List(Int)) -> Option(Int) {
  let found = xs |> find(fn x: x % 2 == 0)?
  Some(found * 10)
}

test "integers are exact and never overflow" {
  expect(fib(20) == 6765)
  expect(9223372036854775807 + 1 == 9223372036854775808)
  expect(2 * 9223372036854775807 - 9223372036854775807 == 9223372036854775807)
  expect(-7 / 2 == -4)
  expect(-7 % 2 == 1)
  expect(1_000_000 == 1000000)
}

test "strings interpolate any value" {
  let n = 3
  let names = ["a", "b"]
  expect("{n} items: {names}" == "3 items: [\"a\", \"b\"]")
  expect("{Circle(radius: 1.5)}" == "Circle(radius: 1.5)")
  expect("{20 min}, {16 MiB}, {2026-10-10}" == "20 min, 16 MiB, 2026-10-10")
  expect("\{literal\}" |> size == 9)
}

test "records are values" {
  let a = { name: "Ann", age: 3 }
  let b = { a with age: 4 }
  expect(a.age == 3)
  expect(b.age == 4)
  expect(a != b)
  expect({ age: 4, name: "Ann" } == b)
}

test "mutation is local and visible" {
  var c = { hits: 0, last: None }
  record(mut c, 7)
  record(mut c, 9)
  expect(c.hits == 2)
  expect(c.last == Some(9))
  let snapshot = c
  c.reset()
  expect(c.hits == 0)
  expect(snapshot.hits == 2)
  var total = 0
  for i in 1..=4 { total += i }
  expect(total == 10)
}

test "match takes variants, guards, alternatives and lists apart" {
  let shapes = [Circle(radius: 1.0), Rect(w: 2.0, h: 3.0), Dot]
  expect(shapes |> map(fn s: s.area()) |> sum == 9.0)
  let label = fn s: match s {
    Rect(w, h) if w == h => "square"
    Rect(..) => "rectangle"
    Circle(_) | Dot => "round"
  }
  expect(label(Rect(w: 2.0, h: 2.0)) == "square")
  expect(label(Rect(w: 1.0, h: 2.0)) == "rectangle")
  expect(label(Dot) == "round")
  let describe = fn xs: match xs {
    [] => "empty"
    [only] => "one: {only}"
    [first, ..rest] => "{first} and {rest |> size} more"
  }
  expect(describe([]) == "empty")
  expect(describe([1]) == "one: 1")
  expect(describe([1, 2, 3]) == "1 and 2 more")
  let digit = fn c: match c {
    '0'..='9' => true
    _ => false
  }
  expect(digit('7') && !digit('x'))
}

test "errors are values, and ? passes them on" {
  expect(count("21") == Ok(42))
  expect(count("") == Err(Usage))
  expect(count("x") == Err(BadNumber(NotANumber("x"))))
  expect(firstEven([1, 3, 4]) == Some(40))
  expect(firstEven([1, 3]) == None)
  let [head, ..] = [5, 6] else { return }
  expect(head == 5)
}

test "pipes put the value first, or where _ is" {
  let sub = fn (a, b): a - b
  expect(2 |> sub(10, _) == 8)
  expect(10 |> sub(2) == 8)
  let total = [1, 2, 3, 4]
    |> filter(fn x: x % 2 == 0)
    |> map(fn x: x * x)
    |> sum
  expect(total == 20)
  expect(page() == "50 after 0")
  expect(page(after: Some(7), size: 10) == "10 after 7")
}

test "closures capture values" {
  let base = 10
  let add = fn x: x + base
  let twice = fn f: fn x: f(f(x))
  expect(twice(add)(1) == 21)
  let pairs = { "a": 1, "b": 2 } |> entries |> filter(fn (_, v): v > 1)
  expect(pairs == [("b", 2)])
}

test "collections" {
  let s = {} |> add(1) |> add(2) |> add(1)
  expect(s |> size == 2)
  expect(s |> contains(2))
  expect(s |> remove(1) == {} |> add(2))
  let m = { "one": 1 } |> insert("two", 2)
  expect(m |> get("two") == Some(2))
  expect(m["three"] == None)
  let xs = [10, 20, 30, 40]
  expect(xs[1] == Some(20))
  expect(xs[1..3] == [20, 30])
  expect(xs[..2] == [10, 20])
  expect(xs |> enumerate |> map(fn (i, x): i * x) |> sum == 200)
  expect([3, 1, 2] |> sort == [1, 2, 3])
  expect("a,b" |> split(",") |> join("+") == "a+b")
}

test "loops stop and skip" {
  var seen = []
  var i = 0
  while true {
    i += 1
    if i % 2 == 0 then continue
    if i > 7 then break
    seen = seen |> add(i)
  }
  expect(seen == [1, 3, 5, 7])
}

test "time" {
  let start = 2026-10-10 12:00 in "Europe/Dublin"
  let later = start + 90 min
  expect(later - start == 90 min)
  expect(later > start)
  expect("{later}" == "2026-10-10 13:30 in \"Europe/Dublin\"")
  expect(2026-10-10 + 1 day == 2026-10-11)
}
