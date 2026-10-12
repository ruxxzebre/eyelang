# SPECS.md section 6, example 2.
# The example is a list of statements, wrapped in a function.
fn example() {
  let x = 5                    # immutable, block-scoped; a later `let x` shadows it
  var total = 0                # mutable local; only a callback's lambda may read it (section 7)
  total += x

  let inc = fn n: n + 1                                    # lambdas
  let add = fn (a, b): a + b
  let show = fn (item: Item) {
    let label = item.name
    "<{label}>"
  }
  let later = fn { log.info("done") }                       # no parameters

  let sizes = urls |> map(fetchSize) |> filter(fn s: s > 0)   # data-first pipe
  let diff = 2 |> sub(10, _)                                   # _ places the value: sub(10, 2)

  let small = if n < 2 then n else fib(n - 1) + fib(n - 2)   # if is an expression
  if n > limit then log.warn("{n} is over the limit")       # `then` for one expression
  if retries > 3 {                                           # braces for several statements
    log.warn("giving up")
    log.info("tried {retries} times")
  }
  for item in items { log.info(item.name) }
  for (i, item) in items |> enumerate { log.info("{i}: {item.name}") }
  for i in 0..n { log.info("{i}") }
}
