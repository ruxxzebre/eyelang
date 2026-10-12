---
title: Errors
---

# Errors

Eyelang has no exceptions. A function that can fail says so in its return type, with `Result`,
and the caller decides what to do with the failure. This page reads a quantity typed by a
customer, and takes an order from the shop's stock.

The program is [`docs/tutorial/code/errors/main.eye`](https://github.com/ruxxzebre/eyelang/blob/dev/docs/tutorial/code/errors/main.eye).

## Result

`Result(a, e)` is a built-in variant type, like `Option`: `Ok(value)` when the work succeeded, or
`Err(error)` when it didn't. The error is an ordinary value, usually a variant type you define for
the ways a function can fail:

<<< ./code/errors/main.eye#parse-error

<<< ./code/errors/main.eye#parse

The return type, `Result(Int, ParseError)`, tells every caller that this can fail, and how.

- `return Err(Empty)` ends the function early with an error.
- `parseInt` from the standard library returns a `Result` too. Its error is a message, which
  `mapErr` turns into this function's `NotANumber`.
- `?` after a `Result` is how you say "if this failed, fail too". On `Ok(n)` it gives `n`; on `Err`
  it returns the error from the function at once.

<<< ./code/errors/main.eye#test-parse

## Handling an error with `match`

A `Result` is a variant type, so `match` handles it. Patterns nest, so one arm can match both the
`Err` and the kind of error inside it:

<<< ./code/errors/main.eye#match-result

<<< ./code/errors/main.eye#test-match-result

The `match` is checked for every case, as on the [previous page](/tutorial/types-and-matching). If
you add a case to `ParseError`, the compiler shows you this `match`, and every other place that
must decide what to say about it.

## Defaults with `??`

Sometimes a missing value has an obvious stand-in. `??` gives the value inside a `Some`, or the
default after it for `None`:

<<< ./code/errors/main.eye#default

`stock["pears"]` looks a key up in a map, and returns an `Option`, because the key may not be
there. `??` works on `Option`, not on `Result`: an error usually deserves a decision, not a
default.

::: tip
`??` binds more loosely than any other operator, so `stock["pears"] ?? 0 == 0` means
`stock["pears"] ?? (0 == 0)`. Name the value with `let` first, or use parentheses.
:::

## Passing errors on

A larger operation has its own error type. One case can carry the error of a smaller step:

<<< ./code/errors/main.eye#order-error

<<< ./code/errors/main.eye#take

`parseQuantity` fails with a `ParseError`, but `take` returns an `OrderError`. `?` converts it:
when the caller's error type has exactly one case that holds the error's type, here
`BadQuantity(ParseError)`, `?` wraps the error in that case. When there is no such case, or more
than one, write the conversion yourself with `mapErr`, as `parseQuantity` does.

## Ignoring an error on purpose

A `Result` can't be left unused. A call on a line of its own whose `Result` nothing reads, such as
`fs.writeText(path, text)`, is an error, since a failure there would go unnoticed. When a failure
really changes nothing, say so with `let _ =`:

```eye
let _ = fs.writeText("{path}.bak", text)   # a backup that fails changes nothing
```

The same holds for the outcome of a [command](/guide/contracts) to another service, which may be a
refusal, or `Uncertain` when the reply was lost. Values that aren't failures, such as the `()` that `print` returns, or an `Option`,
need nothing.

## Changing a value you are given: `mut`

`take` also changes the stock. A parameter marked `mut` may be changed by the function, and the
change is visible to the caller. The call says `mut` too, so a reader of the call can see which
arguments may change:

<<< ./code/errors/main.eye#test-take

Both halves are checked. Leave `mut` off the call, `take(stock, "apples", "3")`, and `eye check`
says:

```text
main.eye:75:15: take changes its argument stock, so the call says so: write mut stock
```

Only a `var` (or another `mut` parameter) can be passed as `mut`. With `let stock = ...`, the
checker says:

```text
main.eye:75:19: stock can't change: it is a let or a parameter; declare it with var, or as a mut parameter
```

`take` checks everything before it changes anything, so a failed call leaves the stock as it was:
the last line shows the pears are all still there. The function returns as soon as `?` or
`return Err(...)` sees a problem, and the change on the line after it never happens.

`mut` is not a reference. The function can't keep the parameter or hand it to anything that
outlives the call, and no other variable sees the change. See [mutation](/reference/language/mutation).

## `?` on Option

In a function that returns an `Option`, `?` works on an `Option` the same way: on `None` it returns
`None` from the function.

<<< ./code/errors/main.eye#option-question

## What isn't an error

Errors are for failures that can happen: bad input, a full shelf, a service that says no. Two other
things stop code, and neither is a `Result`:

- `unreachable("why")` stops the program, or a job's run, for a state the code has already ruled
  out. It is not for failures that can happen.
- Running out of memory or time is a fault. Code can't catch one where it happens; whoever started
  the work sees it as the outcome.

Library functions are total: indexing returns an `Option`, parsing returns a `Result`, and `Int`
can't overflow. So most code has no hidden ways to fail.

## Next

[An entity](/tutorial/an-entity): state that lives on between calls, owned by one entity.

The full rules:

- [Errors: Result only](/reference/language/errors-result-only): `?`, converting errors,
  `unreachable` and faults.
- [Mutation](/reference/language/mutation): `var`, `mut` parameters and `mut self`, and why there
  is no aliasing.
- [Pattern matching](/reference/language/pattern-matching): nested patterns and `let ... else`.
