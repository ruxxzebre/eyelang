## A count that only grows, made through its type's own functions.

## A count of things seen.
pub opaque type Tally = { seen: Int }

impl Tally {
  ## An empty tally.
  pub fn new() -> Tally = { seen: 0 }

  ## A tally that has seen n things.
  pub fn of(n: Int) -> Tally = { seen: n }

  ## The tally with one more thing seen.
  pub fn add(self) -> Tally = { seen: self.seen + 1 }

  ## How many things the tally has seen.
  pub fn count(self) -> Int = self.seen
}

## The things two tallies have seen together.
pub fn merged(a: Tally, b: Tally) -> Tally = { seen: a.seen + b.seen }
