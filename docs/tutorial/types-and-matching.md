---
title: Types and matching
---

# Types and matching

This page defines types of your own: records, which hold several named values, and variant types,
which are one of several cases. `match` takes them apart, and the compiler checks that you handled
every case. The examples are tasks on a to-do list.

The program is [`docs/tutorial/code/types-and-matching/main.eye`](https://github.com/ruxxzebre/eyelang/blob/main/docs/tutorial/code/types-and-matching/main.eye).

## Records

A record type lists its fields and their types:

<<< ./code/types-and-matching/main.eye#record

Build a record with braces and field names. Read a field with `.`. To change a field, build a new
record with `with`, which copies every field you don't name:

<<< ./code/types-and-matching/main.eye#record-values

`finished` is a new value; `task` is unchanged. Records compare by their fields, so `==` and `!=`
work without any code from you.

A record literal takes the type its context expects. In `let task: Task = { ... }` the annotation
says it is a `Task`. In a function that returns a `Task`, the return type says so, and you write no
annotation.

## Variant types

A variant type is one of a fixed set of cases. Each case can carry values of its own:

<<< ./code/types-and-matching/main.eye#variant

- A `Priority` is `Low`, `Normal` or `High`, and nothing else.
- A `Status` is `Open`, or `Snoozed` with the date it wakes up, or `Done` with the date it was
  finished. Build one by calling the case: `Snoozed(until: 2026-10-12)`.
- `2026-10-12` is a `Date` literal. Eyelang has literals for dates, times and durations
  (`15 min`), because programs that wait use them a lot.

## `match`

`match` picks the arm whose pattern fits the value, and binds the values inside it to names:

<<< ./code/types-and-matching/main.eye#match

<<< ./code/types-and-matching/main.eye#test-match

`match` is an expression, so `describe` is one `match` after `=`.

### Every case, or a compile error

Every `match` must handle every case. Delete the `Done` arm and run `eye check`:

```text
main.eye:28:36: this match doesn't handle Done(_): add an arm for it, or `_ => …`
```

This is what makes variant types safe to grow. When you add a case to `Status` later, every `match`
that doesn't handle it stops compiling, and the errors list the places to update.

### Guards, alternatives and `_`

<<< ./code/types-and-matching/main.eye#guards

- `High if !t.done` matches only when the condition after `if` holds. This is a guard.
- `Low | Normal` matches either case.
- `_` matches anything. Use it for "all the other cases", but know that it also swallows cases you
  add later.

<<< ./code/types-and-matching/main.eye#test-guards

The last line shows that variants compare in the order they are declared, so `Low < High`.

## Option

`Option(a)` is a built-in variant type: `Some(value)` or `None`. A function returns one when it may
have nothing to give. `find` returns the first element that passes its test, or `None`:

<<< ./code/types-and-matching/main.eye#option

You can't use an `Option(Task)` where a `Task` is expected. The compiler makes you handle `None`
first, usually with `match`. Two shortcuts help:

- `map` on an `Option` changes the value inside a `Some`, and leaves `None` alone.
- `option ?? default` gives the value inside a `Some`, or the default for `None`.

<<< ./code/types-and-matching/main.eye#test-option

## Methods

Functions in an `impl` block are methods of the type. A method that takes `self` is called on a
value, `t.line()`. One without `self` is called on the type, `Task.new("Buy milk")`, which is the
usual way to write a constructor:

<<< ./code/types-and-matching/main.eye#impl

<<< ./code/types-and-matching/main.eye#test-impl

`{ title, done: false, ... }` is short for `{ title: title, ... }`. `finish` returns a new task
rather than changing `self`. The [next page](/tutorial/errors) shows how a function can change a
value it is given, and makes that visible at the call.

Methods and pipes do different jobs. `t.line()` calls a method of `t`'s type. `tasks |> find(...)`
calls an ordinary function with `tasks` as its first argument. Collection functions are ordinary
functions, so one pipeline works on lists, sets and maps alike.

## Next

[Errors](/tutorial/errors): `Result`, `?` and error types of your own.

The full rules:

- [Values and types](/reference/language/values-and-types): records, variants, `Option` and
  `Result`, and the literal types.
- [Pattern matching](/reference/language/pattern-matching): every kind of pattern, including
  lists, tuples, records and ranges.
- [Methods and generics](/reference/language/methods-and-generics): `impl`, type variables such as
  `List(a)`, and `where`.
- [Traits](/reference/language/traits): behaviour that several types share, such as `Show` and
  `Ord`, which records and variants get without any code from you.
