# The typed IR of operators, traits and generic code (eye check --ir).

type Money = { cents: Int }

impl Add(Money, Money) for Money {
  fn add(self, other: Money) -> Money = { cents: self.cents + other.cents }
}

impl Neg(Money) for Money {
  fn neg(self) -> Money = { cents: -self.cents }
}

impl Show for Money {
  fn show(self) -> String = "€{self.cents}"
}

## Adds two prices, through Money's impl of Add.
fn both(a: Money, b: Money) -> Money = a + b

## Turns a price around, through Money's impl of Neg.
fn refund(a: Money) -> Money = -a

## Adds up anything that adds to itself: the value's impl answers.
fn total(xs: List(a), zero: a) -> a where a: Add(a, a) {
  var sum = zero
  for x in xs { sum = sum + x }
  sum
}

## A private function without types is generic.
fn twice(n) = n + n

fn uses() -> (Int, Money, String) {
  let price: Money = { cents: 250 }
  (twice(2), total([price, price], { cents: 0 }), "{refund(price)}")
}
