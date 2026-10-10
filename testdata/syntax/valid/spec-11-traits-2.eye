# SPECS.md section 11, example 2.
trait Add(rhs, out) {
  fn add(self, other: rhs) -> out
}

type Money = { cents: Int }

impl Add(Money, Money) for Money {
  fn add(self, other: Money) -> Money = { cents: self.cents + other.cents }
}

let total = price + shipping                 # Money + Money
fn double(n) = n * 2                         # inferred: fn(t) -> o where t: Mul(Int, o)
