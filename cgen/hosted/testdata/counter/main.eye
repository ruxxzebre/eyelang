# The program counter.c stands in for, compiled: the runner reads its
# declarations from here and runs its entity method and its job in C.

## Counts what it is given.
entity Counter(id: String) {
  state count: Int = 0

  ## Adds n, and gives the new count.
  pub fn add(n: Int) -> Int {
    count += n
    count
  }
}

## Waits two seconds, then counts one.
job tick(id: String) {
  sleep!(2 s)
  let _ = Counter(id).add!(1)
}

pub fn main() uses {entity Counter, start tick, stdio} {
  print("count {Counter("a").count}")
  let _ = tick.start("a", key: "a")
  print("started")
}
