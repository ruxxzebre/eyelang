# Recursion too deep is a fault, not a crash (SPECS.md §15): the test that
# recursed fails with the interpreter's message, and the others run. Natively
# the stack's size is the limit (cgen/runtime/stack.c): it holds the 100,000
# calls the interpreter allows, so recursion as deep as that runs on both.

fn count(n: Int) -> Int = if n == 0 then 0 else 1 + count(n - 1)

fn isEven(n: Int) -> Bool = if n == 0 then true else isOdd(n - 1)

fn isOdd(n: Int) -> Bool = if n == 0 then false else isEven(n - 1)

type Pair = { name: String, items: List(Int), note: Option(String) }

## A recursion with records and lists live in each call.
fn build(n: Int) -> List(Pair) {
  if n == 0 { return [] }
  let p: Pair = { name: "p{n}", items: [n, n + 1], note: Some("n") }
  let rest = build(n - 1)
  rest + [p]
}

## A recursion through a closure.
fn sumWith(xs: List(Int), at: Int, f: fn(Int) -> Int) -> Int =
  if at == xs |> size then 0 else f(xs[at] ?? 0) + sumWith(xs, at + 1, f)

test "a recursion deeper than the limit fails the test" {
  expect(count(100_000_000) == 100_000_000)
}

test "the tests after it run" {
  expect(count(90_000) == 90_000)
}

test "a recursion through two functions" {
  expect(isEven(90_000))
  expect(isOdd(89_999))
}

test "values live in each call" {
  let ps = build(90_000)
  expect(ps |> size == 90_000)
  expect(ps[0] == Some({ name: "p1", items: [1, 2], note: Some("n") }))
}

test "a recursion through a closure" {
  let xs = (0..90_000) |> toList
  expect(sumWith(xs, 0, fn x: x % 2) == 45_000)
}

test "a recursion too deep after one that wasn't" {
  expect(count(1_000) == 1_000)
  expect(count(100_000_000) == 100_000_000)
}
