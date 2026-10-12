# Leaving early from anywhere: return, break, continue and ? in statements
# and inside expressions, and records changed in place only where no one
# else can see it.

type Point = { x: Int, y: Int }

fn sign(n: Int) -> Int {
  if n < 0 { return -1 }
  if n == 0 { return 0 }
  1
}

# A return inside an expression, not a statement.
fn plusOne(n: Option(Int)) -> Int = 1 + (n ?? return 0)

fn firstBig(xs: List(Int)) -> Int {
  for x in xs {
    for y in [x, x * 10] {
      if y > 100 { return y }
    }
  }
  -1
}

fn half(n: Int) -> Option(Int) = if n % 2 == 0 then Some(n / 2) else None

# ? as a statement's value, as an argument and inside an expression.
fn quarter(n: Int) -> Option(Int) {
  let h = half(n)?
  Some(half(h)? + 0)
}

fn quarterArg(n: Int) -> Option(Int) = Some(1 * half(half(n)?)?)

fn pick(n: Int) -> String {
  let label = match n {
    0 => return "zero"
    1 => "one"
    _ => "many"
  }
  "{label}!"
}

fn sumPoint(p: Point) -> Int = p.x + p.y

test "return leaves from a statement or an expression" {
  expect(sign(-5) == -1 && sign(0) == 0 && sign(9) == 1)
  expect(plusOne(Some(2)) == 3 && plusOne(None) == 0)
  expect(firstBig([1, 5, 12, 40]) == 120)
  expect(firstBig([1]) == -1)
  expect(pick(0) == "zero" && pick(1) == "one!" && pick(7) == "many!")
}

test "? passes None on from anywhere" {
  expect(quarter(8) == Some(2))
  expect(quarter(6) == None)
  expect(quarter(5) == None)
  expect(quarterArg(8) == Some(2))
  expect(quarterArg(6) == None)
}

test "break and continue leave from a statement or an expression" {
  var seen = []
  for i in 0..10 {
    let odd = if i % 2 == 1 then true else continue
    if i > 6 { break }
    seen += [i * 10 + (if odd then 1 else break)]
  }
  expect(seen == [11, 31, 51])
  var n = 0
  while true {
    n += 1
    let small = n < 4 || (if n > 5 then break else false)
    if small then continue
  }
  expect(n == 6)
  var outer = 0
  for i in 0..3 {
    for j in 0..3 {
      if j == 1 then break
      outer += 10 * i + j
    }
  }
  expect(outer == 30)
  var found = 0
  for xs in [[1, 2], [3, 4]] {
    let [first, ..] = xs else { continue }
    found += first
  }
  expect(found == 4)
}

test "a record changes in place only while one variable holds it" {
  var p: Point = { x: 0, y: 0 }
  p.x = 1
  let q = p
  p.x = 2
  p.y += 5
  expect(q == { x: 1, y: 0 } && p == { x: 2, y: 5 })
  var r = { p with x: 10 }
  let kept = [r]
  r = { r with x: 11 }
  r.y = 12
  expect(kept == [{ x: 10, y: 5 }] && r == { x: 11, y: 12 })
  var s = { r with y: 0 }
  s = { s with x: s.y, y: s.x }
  expect(s == { x: 0, y: 11 })
  s = { s with x: sumPoint(s), y: s.x }
  expect(s == { x: 11, y: 0 })
  var t = { s with x: 1 }
  let list = [t, t]
  t.x = 2
  expect(list == [{ x: 1, y: 0 }, { x: 1, y: 0 }] && t.x == 2)
  var u = { t with y: 3 }
  let pair = (u, 0)
  u.y = 4
  let (before, _) = pair
  expect(before.y == 3 && u.y == 4)
}
