---
title: Values and functions
---

# Values and functions

This page covers the everyday core: functions, `let`, strings, lists, pipes, closures, and the
tests that check them. The examples price a shopping basket. Prices are whole cents, an `Int`, so
no rounding gets in the way.

The program is [`docs/tutorial/code/values-and-functions/main.eye`](https://github.com/ruxxzebre/eyelang/blob/dev/docs/tutorial/code/values-and-functions/main.eye).

## Functions

<<< ./code/values-and-functions/main.eye#functions

`fn` declares a function. Parameters and the result have types: `Int`, `String`, and so on.

- When the body is one expression, write it after `=`, as in `double`.
- When it needs several steps, use braces. The value of the last expression is the result, so
  `price` needs no `return`.
- `if ... then ... else ...` is an expression: it has a value, like the ternary operator in other
  languages.
- `"€{euros}.{rest}"` is string interpolation. Any expression can go inside the braces.

`Int` has arbitrary precision, so it never overflows. `/` on two `Int`s rounds down, and `%` gives
the remainder. Long numbers take `_` between digits, `1_000_000`, and bits read best in hex or
binary: `0xFF` and `0b1010` are `Int`s too.

## Tests

A `test` block is a declaration, like a function. It has a name and a body, and `expect` fails the
test when its condition is false:

<<< ./code/values-and-functions/main.eye#test-functions

Run the tests with `eye test` and the program's directory:

```sh
eye test docs/tutorial/code/values-and-functions
```

```text
ok    functions (256µs)
ok    let names a value (1µs)
ok    lists (28µs)
ok    pipes pass the value on (5µs)
ok    closures keep what they capture (2µs)
ok    var and for (2µs)
6 tests passed (295µs)
```

When an `expect` fails, it shows the value it got. If `double(21) == 42` said `43` instead:

```text
FAIL  functions (730µs)
      main.eye:4:3: expected double(21) == 43, but double(21) is 42

1 of 1 tests failed
```

The rest of this page uses tests to show what each line does. Read `expect(a == b)` as "`a` is
`b`".

## Names with `let`

<<< ./code/values-and-functions/main.eye#lets

`let` gives a value a name. The type is inferred, so you rarely write one inside a body. A `let`
never changes. Trying to change one is an error:

```text
main.eye:3:3: total can't change: it is a let or a parameter; declare it with var, or as a mut parameter
```

A later `let` with the same name shadows the earlier one, which is often what you want instead.

## Lists

<<< ./code/values-and-functions/main.eye#lists

`[250, 120, 80]` is a `List(Int)`. Lists are values: adding to one gives you a new list and leaves
the old one as it was.

`basket[0]` doesn't fail when the index is out of range. It returns an `Option(Int)`: `Some(250)`
when the element exists, `None` when it doesn't. There is no null in Eyelang, and no exception for a
bad index. You will see how to take an `Option` apart on the [next page](/tutorial/types-and-matching).

## Pipes

`x |> f(y)` calls `f(x, y)`: the value on the left becomes the first argument. Long chains read
top to bottom, in the order things happen:

<<< ./code/values-and-functions/main.eye#pipes

`filter` keeps the prices under the limit, `sort` puts them in order, and `map` turns each one
into a string with `price`. A line that starts with `|>` continues the line before it.

<<< ./code/values-and-functions/main.eye#test-pipes

The collection functions, `size`, `sum`, `filter`, `map`, `sort` and the rest, take the collection
first so they work well after `|>`. They are listed in [built-in functions](/reference/api/std/builtins).

Arguments can be passed by name, as in `bargains(basket, under: 200)`. That helps when the meaning
of a number isn't obvious at the call.

## Closures

`fn cents: ...` is a function without a name, a lambda. You already passed one to `filter`. A
lambda can use names from where it was written, and keeps their values:

<<< ./code/values-and-functions/main.eye#closures

`discount(10)` returns a function that takes ten percent off. `discount` has no `->` type; for a
private function, Eyelang infers the result and even the parameter types. Public functions (`pub
fn`) write out their full signature.

## `var` and `for`

Most code doesn't need a variable that changes. When it does, use `var`:

<<< ./code/values-and-functions/main.eye#var-for

- `var` declares a variable that can change. `+=` adds to it.
- `for x in xs { ... }` runs the block once for each element. `1..=4` is a range that includes
  4; `1..4` stops before it.
- `if ... then ...` with a single statement needs no braces.
- `basket[1] = 99` changes one element of a `var` list, and `basket[2] += 1` adds to one. The
  element must be there: an index past the end stops the program, as a bug, since a change that
  went nowhere would be lost. Add elements with `+=`, as in `basket += [99]`.

A `var` is local, and nothing else can see it change. A lambda given straight to a call, such as
`xs |> filter(fn x: x < total)`, may read it, and sees its value at the call; a lambda kept in a
`let` or returned can't capture it.
[Mutation](/reference/language/mutation) explains why that rules out a whole class of bugs.

## Next

[Types and matching](/tutorial/types-and-matching): records, variant types and `match`.

The full rules:

- [Bindings, functions and pipes](/reference/language/bindings-functions-and-pipes): `let`, `var`,
  lambdas, pipes with `_`, named arguments and defaults.
- [Values and types](/reference/language/values-and-types): the built-in types, collections,
  ranges and slices.
- [Lexical structure](/reference/language/lexical-structure): literals, strings, comments, and
  where a line break ends a statement.
- [Time, secrets, tests](/reference/language/time-secrets-tests): what `test` and `expect` do.
