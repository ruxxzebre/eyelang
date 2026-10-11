# Traits: declarations, impls, bounds, operators and the derived traits.

## Something with a name to show.
trait Named {
  fn name(self) -> String

  ## A label, the same for everything Named unless an impl says otherwise.
  fn label(self) -> String = "<{self.name()}>"
}

type Player = { nick: String, level: Int }

type Robot = | Robot(serial: Int)

impl Named for Player {
  fn name(self) = self.nick
}

impl Named for Robot {
  fn name(self) = match self {
    Robot(serial) => "robot-{serial}"
  }

  fn label(self) -> String = "[{self.name()}]"
}

## A trait used as a parameter's type: any type that is Named.
fn greet(x: Named) -> String = "hello, {x.name()}"

## The same, with the bound written out.
fn labels(xs: List(t)) -> List(String) where t: Named = xs |> map(fn x: x.label())

fn largest(xs: List(a)) -> Option(a) where a: Ord {
  var best = xs |> first?
  for x in xs {
    if x > best { best = x }
  }
  Some(best)
}

type Money = { cents: Int }

impl Add(Money, Money) for Money {
  fn add(self, other: Money) -> Money = { cents: self.cents + other.cents }
}

impl Mul(Int, Money) for Money {
  fn mul(self, n: Int) -> Money = { cents: self.cents * n }
}

impl Show for Money {
  fn show(self) -> String = "€{self.cents / 100}.{self.cents % 100}"
}

## Adds up anything that adds to itself.
fn total(xs: List(a), zero: a) -> a where a: Add(a, a) {
  var sum = zero
  for x in xs { sum = sum + x }
  sum
}

type Priority = Low | Normal | High

test "a trait's methods, and its default" {
  let p: Player = { nick: "ann", level: 3 }
  expect(p.name() == "ann")
  expect(p.label() == "<ann>")
  expect(Robot(serial: 7).label() == "[robot-7]")
  expect(greet(p) == "hello, ann")
  expect(greet(Robot(serial: 1)) == "hello, robot-1")
  expect(labels([p]) == ["<ann>"])
}

test "operators are traits" {
  let price: Money = { cents: 250 }
  let shipping: Money = { cents: 199 }
  expect(price + shipping == { cents: 449 })
  expect(price * 3 == { cents: 750 })
  expect(total([price, shipping], { cents: 0 }) == { cents: 449 })
  expect(total([1, 2, 3], 0) == 6)
}

test "an impl's show replaces the derived form" {
  let price: Money = { cents: 1250 }
  expect("{price}" == "€12.50")
}

test "Ord is derived: variants as declared, records part by part" {
  expect(Low < High && Normal < High)
  expect([High, Low, Normal] |> sort == [Low, Normal, High])
  let a: Player = { nick: "ann", level: 3 }
  let b: Player = { nick: "bo", level: 1 }
  expect(largest([a, b]) == Some(b))
  expect(largest([3, 9, 2]) == Some(9))
}

## A value of any Named type, chosen at run time: the player, or a robot
## standing in when nobody is there.
fn host(nick: Option(String)) -> dyn Named =
  match nick {
    Some(n) => {
      let p: Player = { nick: n, level: 1 }
      p
    }
    None => Robot(serial: 9)
  }

## Names of things of different types, kept in one list.
fn roll(xs: List(dyn Named)) -> String = xs |> map(fn x: x.label()) |> join(" ")

test "dyn Named holds values of different types, and calls their own methods" {
  let p: Player = { nick: "ann", level: 3 }
  let crowd: List(dyn Named) = [p, Robot(serial: 7), host(None)]
  expect(roll(crowd) == "<ann> [robot-7] [robot-9]")
  expect(host(Some("bo")).name() == "bo")
  expect(roll([p]) == "<ann>")  # a List(Player) is a List(dyn Named)
}
