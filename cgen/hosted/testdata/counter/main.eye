# The program counter.c stands in for, compiled: the runner reads its
# declarations from here, and runs its main, its tests, its entity methods
# and its job in C.

import bus

## Counts what it is given, and adds it to the total.
entity Counter(id: String) {
  state count: Int = 0

  ## Adds n, tells the bus, and gives the new count.
  pub fn add(n: Int) -> Int uses {entity Total, publish Bus.counted} {
    count += n
    bus.counted.publish({ id, count })
    let _ = Total("all").add(n)
    count
  }
}

## The sum of every count.
entity Total(id: String) {
  state sum: Int = 0

  ## Adds n, and gives the new sum.
  pub fn add(n: Int) -> Int {
    sum += n
    sum
  }
}

## Waits two seconds, reads TICK, then counts one.
job tick(id: String) {
  sleep!(2 s)
  let _ = env.get!("TICK")
  let _ = Counter(id).add!(1)
}

pub fn main() uses {entity Counter, start tick, stdio} {
  print("count {Counter("a").count}")
  let _ = tick.start("a", key: "a")
  print("started")
}

test "counts and totals" {
  expect(Counter("t").add(2) == 2)
  expect(Total("all").sum == 2)
}

test "a failing expect stops the test" {
  expect(Counter("f").add(1) == 5)
}
