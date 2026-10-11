# The typed IR of effects: each function's, a lambda's, and a stored
# function's type (eye check --ir).

type Timer = { name: String, read: fn() -> Instant uses {clock} }

fn stamp() -> Instant = now()

## Naming stamp and writing the lambda count their effects here, though
## nothing runs them yet (HANDOFF.md, a proposal).
pub fn timers() -> List(Timer) uses {clock} = [
  { name: "named", read: stamp },
  { name: "lambda", read: fn: now() },
]

pub fn readAll(ts: List(Timer)) -> List(Instant) uses {clock} = ts |> map(fn t: t.read())

pub fn shout(xs: List(String)) uses {stdio} {
  for x in xs { print(x) }
}
