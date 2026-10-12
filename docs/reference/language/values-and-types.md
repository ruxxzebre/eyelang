---
title: "Values and types"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Values and types

- **Primitives:** `Int`, `NonZero(Int)`, `Float`, `Bool`, `Char`, `String`, `()`.
- **Bytes and fixed-width integers:** `Byte` (0 to 255), `Int32`, `Int64`, `UInt32` and `UInt64`,
  and `Bytes`, a sequence of `Byte`s, for binary formats, hashing, lexers and foreign functions.
  They compare with the usual operators but have no `+`, `-`, `*` or `/`. Arithmetic and bit
  operations are methods, each saying what happens at the edges of the type:

  | Method | What it gives |
  |---|---|
  | `a.wrappingAdd(b)`, `a.wrappingSub(b)`, `a.wrappingMul(b)` | the result wrapped into the type, modulo 2 to the power of its bits: for `Byte`s, `255` plus `1` is `0` |
  | `a.checkedAdd(b)`, `a.checkedSub(b)`, `a.checkedMul(b)` | `Some(result)`, or `None` when the result doesn't fit |
  | `a.and(b)`, `a.or(b)`, `a.xor(b)`, `a.not()` | the bits combined, two's complement for the signed types |
  | `a.shiftLeft(n)`, `a.shiftRight(n)` | the bits moved by `n`, an `Int`; bits moved out are lost, `shiftRight` keeps the sign of a signed type, and a negative `n` is a fault |
  | `a.toInt()` | the value as an `Int`, which always fits |
  | `Int32.from(n)`, and `T.from(n)` for each type | `Some` of the `Int` `n` as that type when it fits, otherwise `None` |

  `a` and `b` have the same type: `x.wrappingAdd(1)` adds a literal of `x`'s type. To mix widths,
  convert through `Int`: `Int64.from(x.toInt())`.

  ```eye
  fn fnv1a(data: Bytes) -> UInt32 {
    var hash: UInt32 = 2166136261
    for b in data {
      hash = hash.xor(UInt32.from(b.toInt()) ?? 0).wrappingMul(16777619)
    }
    hash
  }
  ```

  Integer literals that fit are fixed-width values where one is expected: `src[i] ?? 0`. One that
  doesn't fit, as in `let b: Byte = 300`, is an error. An ASCII character literal is a `Byte`
  where one is expected, in expressions and in patterns, so byte-level code reads as text:

  ```eye
  fn isComment(src: Bytes) -> Bool = match src {
    ['-', '-', ..] => true
    _ => false
  }
  ```

  Any other character, such as `'é'`, is more than one byte in UTF-8, and is an error where a
  `Byte` is expected.
- **Time:** `Instant`, `Date`, `Time`, `Duration`, `Zone`, `Schedule`. **Memory:** `Size`.
- **Collections:** `List(a)`, `Map(k, v)` and `Set(a)`. A `List` is a growable array. Collections
  are values: assigning or passing one never shares later changes ([section 7](/reference/language/mutation)). An update to a
  collection that nothing else refers to happens in place; otherwise the collection is copied first.
- **Tuples:** `(Int, String)`, built with `(1, "a")` and taken apart with patterns:
  `let (n, s) = pair`. A tuple has at least two elements, and there is no `.0`; when the parts
  deserve names, use a record. A lambda with as many parameters as a tuple has elements accepts
  the tuple, so `m |> entries |> filter(fn (_, at): at <= now)` needs no destructuring; `_` names a
  parameter that is ignored.
- **Records:** `type User = { name: String, age: Int }`, built with `{ name: "Ann", age: 3 }` and
  updated with `{ user with age: 4 }`, or with `user.age = 4` when `user` is a `var`.
- **Records are nominal.** A record type is its name: two record types with the same fields are
  different types, so a `Celsius` can't be passed where a `Fahrenheit` is expected, and methods,
  traits and schemas attach to the name. A record literal takes the type its context expects: a
  parameter, a field, a return type or an annotation. With no type expected, the compiler asks for
  one:

  ```eye
  fn make() -> User = { name: "Ann", age: 3 }    # a User: the return type says so
  let u: User = { name: "Bo", age: 4 }
  let x = { name: "Cy", age: 5 }                 # error: which record type? write `let x: User = …`
  ```
- **Variants:** `type Grant = Admitted(revision: Revision) | Pending(receipt: ReceiptId) | Refused(reason: Reason)`,
  built with `Pending(receipt: id)`. Payload fields may be named or positional (`Some(a)`). A
  variant list needs at least one `|`, so a type with a single variant starts with one:
  `type Wrapper = | Wrapped(Int)`. Without a `|`, `type Ids = List(Id)` is an alias.
- **Built-in variants:** `Option(a) = Some(a) | None` and `Result(a, e) = Ok(a) | Err(e)`.
  There is no null.
- **Type variables** are lower-case names in types: `List(a)`, `Map(k, v)` ([section 8](/reference/language/methods-and-generics)).
- **Ranges:** `0..n` counts up to but not including `n`, and `0..=n` includes it. Ranges drive `for`
  loops, slices and patterns. A range is a value of type `Range(a)` with fields `start` and `end`,
  which hold the bounds as written: `(0..5).end` and `(0..=5).end` are both 5, so `end` doesn't say
  whether the range includes it. A range whose end comes before its start is empty: `5..2` holds
  nothing, and a `for` over it runs no times. A range of times of day is the exception: it wraps
  midnight, so `23:00..08:00` holds 23:30 and 07:00, as quiet hours need.
- **Slices:** `xs[2..5]`, `xs[..n]` and `bytes[i..]` are views that share storage with what they
  come from. A slice is clamped to the elements that exist, so it never fails, while `xs[i]`
  returns an `Option`.
- **Positions in strings** are `Offset`s, not `Int`s. Only a string's own functions make one:
  `s.start()` and `s.end()`, `s.indexOf(part, from: i)`, and `s.after(i)` and `s.before(i)`, which
  step over one character, or `by: n`, and are `None` past either end. `s.slice(a..b)` takes a
  `Range(Offset)` and gives an `Option(String)`, `None` when an end would split a character, and
  `s.charAt(i)` gives the character there. Offsets compare with `==` and `<`; they have no
  arithmetic. `size`, `take` and `drop` count characters, as `for` over `s.chars()` does, and a
  count is an `Int`, so it can't be passed where a position goes: with `let city = "Málaga"`,
  `city.slice(city.start()..city.end())` is `Some("Málaga")`, and `city.slice(0..(city |> size))`
  is an error, `expected Range(Offset), found Range(Int)`, which says where offsets come from.
  An `Offset` counts bytes underneath, so scanning by position costs nothing per step, but there
  is no number to read from it, since it means something only in its own string; it shows as
  `byte 5`.
- **Iteration:** `for` walks lists, sets, ranges, `Bytes`, a map's `(key, value)` pairs, a string's
  characters (`s.chars()`) and any `Iter(a)`. `xs.iter()` gives a lazy `Iter(a)` whose pipelines
  (`map`, `filter`, `take`, `enumerate`, `zip`) don't build intermediate lists; `toList` collects
  one.
- **Entity references:** `Ref(World)` is an ID, not a pointer. It can be stored, sent and saved
  anywhere, and it can't be confused with `Ref(Player)`.
- **Special types:** `Secret(a)` (no printing, no saving), `Approval` (only a human approval flow
  can create one; its `what` and `by` say what was approved and by whom), `Key` (an idempotency
  key), `Version` (the version of some state, which a `@fresh` command names; it has `==` and no
  order, and an integer literal can be one), `Run(a)` (a started job). A value that foreign code owns has an `extern type` ([section 18](/reference/language/calling-c-and-go)).
- **Opaque types:** `pub opaque type Lexer = { … }` exports the type but not its fields, so other
  modules use it only through its functions and methods.
- **String literals take the type expected,** as record literals do: where a `Key` or a `Zone` is
  expected, `"ireland-{day}"` is a `Key` and `"Europe/Dublin"` a `Zone`. The expectation may come
  later: `let key = "digest-{id}"` is a `Key` when `key` is passed where a `Key` goes, and a literal
  nothing decides is a `String`. A variable declared `String` isn't a `Key`. A zone's name is
  checked against the time-zone database when the program is compiled. Integer literals take a
  fixed-width type the same way.
- **Maps** use literal or parenthesised keys: `{"a": 1}`, `{(key): 1}`. Records use names:
  `{a: 1}`, and `{ name, age }` is short for `{ name: name, age: age }`. `{}` is an empty map,
  set or record, depending on the type expected. Other sets are built from lists: `toSet([a, b])`.
