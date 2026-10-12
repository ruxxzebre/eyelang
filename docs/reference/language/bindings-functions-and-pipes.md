---
title: "Bindings, functions and pipes"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Bindings, functions and pipes

```eye
pub fn area(w: Float, h: Float) -> Float = w * h       # public: full signature
fn double(n) = n * 2                                     # private: inferred
```

Inside a function body:

```eye
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
```

- A `?` after a piped call applies to that call: `xs |> find(isEven)?` is `find(xs, isEven)?`.
- Named arguments with defaults: `fn page(size: Int = 50, after: Option(Cursor) = None)`.
- **`name:` alone** passes the variable of that name: `page(size:, after:)` is
  `page(size: size, after: after)`.
- **Results.** A function without `->` returns `()` if its body is a block, and the value of its
  last expression is dropped; `fn double(n) = n * 2` returns its expression's type. A private
  function may leave out parameter types too. They are inferred from its body alone, and what the
  body leaves open is generic: each call decides it afresh, so `fn twice(n) = n + n` adds `Int`s at
  one call and `Float`s at the next, and a call that gives it a type without `+`, such as `Bool`,
  is an error at that call. A body that reads a field or calls a method on such a parameter needs
  its type written, since nothing in the body says which record it is. `while true` without a
  `break` never ends but by `return`, so it can stand where any value is expected.
- **A module-level `let`** binds a value computed once; `pub let dublin: Zone = "Europe/Dublin"`
  exports it, and other modules import it by name like any other declaration.
- **A branch that is one assignment** needs no braces, as one that is one expression doesn't:
  `Ok(text) => found += [text]`, `if late then count += 1`.
- **`return` leaves the function it is written in, and not a lambda.** Inside a lambda it is an
  error, since it would leave only the lambda: the lambda's value is its last expression, so give
  that instead, with an `if` or a `match` for an early way out, or `?`, which returns from the
  lambda ([section 10](/reference/language/errors-result-only)).
- No partial application by argument count. Use `_` or a lambda.
