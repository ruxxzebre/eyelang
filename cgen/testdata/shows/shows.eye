# A type's own impl of Show is used wherever one of its values is written
# (SPECS.md §11): on its own, and inside lists, sets, maps, Options, tuples,
# records and variants, and in expect's messages, natively as in the
# interpreter. join writes the derived form, as the interpreter's does.

import fs

type Money = { cents: Int }

impl Show for Money {
  fn show(self) -> String = "{self.cents}c"
}

type Line = { item: String, price: Money, note: Option(Money) }

type Level =
  | Low
  | High

impl Show for Level {
  fn show(self) -> String = match self {
    Low => "low"
    High => "HIGH"
  }
}

type Shape =
  | Dot
  | Circle(radius: Int)

impl Show for Shape {
  fn show(self) -> String = match self {
    Dot => "·"
    Circle(r) => "○{r}"
  }
}

type Plain = { level: Level, shape: Shape, tags: List(Level) }

type Wrapped =
  | Bare(Money)
  | Two(Money, Level)

test "on its own and inside collections" {
  let m: Money = { cents: 250 }
  expect("{m}" == "250c")
  expect("{[m]}" == "[250c]")
  expect("{Some(m)}" == "Some(250c)")
  expect("{[Low, High]}" == "[low, HIGH]")
  expect("{{ "a": High }}" == "\{ \"a\": HIGH \}")
  expect("{{ (Low): 1 }}" == "\{ low: 1 \}")
  expect("{toSet([Dot, Circle(radius: 3)])}" == "\{·, ○3\}")
  expect("{(1, Low, m)}" == "(1, low, 250c)")
  print("{[Some(m), None]} {[[Low], []]} {Ok(High)} {Err(Circle(radius: 2))}")
}

test "inside records and variants" {
  let l: Line = { item: "tea", price: { cents: 250 }, note: Some({ cents: 5 }) }
  expect("{l}" == "\{ item: \"tea\", price: 250c, note: Some(5c) \}")
  let p: Plain = { level: High, shape: Circle(radius: 1), tags: [Low] }
  expect("{p}" == "\{ level: HIGH, shape: ○1, tags: [low] \}")
  expect("{[(1, l)]}" == "[(1, \{ item: \"tea\", price: 250c, note: Some(5c) \})]")
  print("{Bare({ cents: 100 })} {Two({ cents: 1 }, Low)}")
  print(l, p)
}

test "the prelude's and the standard modules' impls" {
  let s = "hello"
  print("{[s.start(), s.end()]} {s.indexOf("l")}")
  let missing = fs.readText("missing.txt")
  print("{missing}")
  let p: Problem = Problem(["reading"], "no such file")
  print("{[p]} {Some(p)}")
}

test "join writes the derived form" {
  print([Low, High] |> join(", "))
  print([Circle(radius: 1), Dot] |> join(" "))
}

test "expect's message uses the impl" {
  let m: Money = { cents: 250 }
  expect([m] == [{ cents: 1 }])
}

test "expect's message uses it inside a record" {
  let l: Line = { item: "tea", price: { cents: 250 }, note: None }
  expect(l == { item: "tea", price: { cents: 1 }, note: None })
}
