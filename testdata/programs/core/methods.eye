# Any function in scope is called as a method on its first argument:
# x.f(a) is f(x, a) (SPECS.md §8).

import tally.{merged}

type Span = { from: Int, to: Int }

type Gauge = { level: Int, read: fn(Int) -> Int }

impl Span {
  fn width(self) -> Int = self.to - self.from
}

fn width(s: Span) -> Int = 0

fn read(g: Gauge, n: Int) -> Int = 0

fn tripled(n) = n + n + n

fn clamp(n: Int, low: Int, high: Int) -> Int = if n < low then low else if n > high then high else n

fn biggest(xs: List(a)) -> Option(a) where a: Ord = xs |> sort |> last

fn moved(s: Span, by: Int) -> Span = { from: s.from + by, to: s.to + by }

test "a function is called on its first argument" {
  expect(4.tripled() == 12)
  expect(1.5.tripled() == 4.5)
  expect(15.clamp(0, 10) == 10)
  expect(15.clamp(0, 10) == clamp(15, 0, 10))
  expect(["b", "c", "a"].biggest() == Some("c"))
  let s: Span = { from: 1, to: 4 }
  expect(s.moved(2).moved(by: 1) == { from: 4, to: 7 })
  expect(s.moved(2) == s |> moved(2))
  let t = tally.Tally.of(2).merged(tally.Tally.new().add())
  expect(t.count() == 3)
}

test "a type's method, then a field, come before a function" {
  let s: Span = { from: 1, to: 4 }
  expect(s.width() == 3)
  expect(width(s) == 0)
  let g: Gauge = { level: 2, read: fn n: n * 10 }
  expect(g.read(3) == 30)
}
