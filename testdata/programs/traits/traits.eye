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

impl Neg(Money) for Money {
  fn neg(self) -> Money = { cents: -self.cents }
}

impl Show for Money {
  fn show(self) -> String = "€{self.cents / 100}.{self.cents % 100}"
}

## A price in pence: the same fields as Money, shown its own way.
type Pence = { cents: Int }

impl Show for Pence {
  fn show(self) -> String = "{self.cents}p"
}

## Turns anything around that has a minus.
fn opposite(x: a) -> a where a: Neg(a) = -x

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
  expect(-price == { cents: -250 })
  expect(opposite(shipping) == { cents: -199 })
  expect(opposite(5) == -5)
}

test "an impl's show replaces the derived form" {
  let price: Money = { cents: 1250 }
  expect("{price}" == "€12.50")
}

## Says hello its own way: a field that holds a function.
type Greeter = { name: String, greet: fn(String) -> String }

test "a field that holds a function is called like a method" {
  let g: Greeter = { name: "plain", greet: fn n: "hi {n}" }
  expect(g.greet("Ann") == "hi Ann")
  expect([g, { g with greet: fn n: "yo {n}" }] |> map(fn x: x.greet("Bo")) == ["hi Bo", "yo Bo"])
}

test "a record shows through its own type, not another with the same fields" {
  let price: Money = { cents: 1250 }
  let small: Pence = { cents: 40 }
  expect("{price} and {small}" == "€12.50 and 40p")
  let more = { small with cents: 45 }
  expect("{more}" == "45p")
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

## A player's stand-in: the same fields as Player, but a type of its own.
type Stand = { nick: String, level: Int }

impl Named for Stand {
  fn name(self) = "{self.nick}'s stand-in"

  fn label(self) -> String = "({self.name()})"
}

test "dyn Named calls the impl of the value's own type, not another with the same fields" {
  let p: Player = { nick: "ann", level: 3 }
  let s: Stand = { nick: "bo", level: 3 }
  expect(roll([p, s]) == "<ann> (bo's stand-in)")
  expect(roll([s, p]) == "(bo's stand-in) <ann>")
  expect(s.label() == "(bo's stand-in)")
  expect(p.label() == "<ann>")
}

## How someone new is let in: a choice that arrives as a pressed button.
type Admission = Guest | Member | Refuse

fn groups(choice: String) -> List(String) =
  match Admission.parse(choice) {
    Some(Guest) => ["guests"]
    Some(Member) => ["guests", "members"]
    Some(Refuse) | None => []
  }

test "an enum lists its variants and reads one by name" {
  expect(Admission.variants() == [Guest, Member, Refuse])
  expect(Admission.variants() |> map(fn a: "{a}") == ["Guest", "Member", "Refuse"])
  expect(Admission.parse("Member") == Some(Member))
  expect(Admission.parse("member") == None)
  expect(Priority.variants() |> map(fn p: Priority.parse("{p}")) == [Some(Low), Some(Normal), Some(High)])
  expect(["Guest", "Member", "Admin"] |> map(groups) == [["guests"], ["guests", "members"], []])
  expect(["High", "Low"] |> map(Priority.parse) == [Some(High), Some(Low)])
}
