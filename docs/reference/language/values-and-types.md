---
title: "Values and types"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Values and types

- **Primitives:** `Int`, `NonZero(Int)`, `Float`, `Bool`, `Char`, `String`, `()`.
- **Bytes and fixed-width integers:** `Byte`, `Bytes`, `Int32`, `Int64`, `UInt32` and `UInt64`,
  for binary formats, hashing and foreign functions. They compare with the usual operators but have
  no `+`, `-`, `*` or `/`: arithmetic on them is explicit, as `a.wrappingAdd(b)` or
  `a.checkedAdd(b)` (an `Option`), and so are bit operations (`a.and(b)`, `a.xor(b)`,
  `a.shiftLeft(3)`). A conversion that could lose information returns an `Option`: `Int32.from(n)`.
  Integer literals that fit are fixed-width values where one is expected: `src[i] ?? 0`.
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
  loops, slices and patterns. A range is a value of type `Range(a)` with fields `start` and `end`.
  A range of times of day may wrap midnight: `23:00..08:00` holds 23:30 and 07:00, as quiet hours
  need.
- **Slices:** `xs[2..5]`, `xs[..n]` and `bytes[i..]` are views that share storage with what they
  come from. A slice is clamped to the elements that exist, so it never fails, while `xs[i]`
  returns an `Option`. Strings are sliced by byte offsets with `s.slice(2..5)`, an `Option(String)`
  that is `None` when the range would split a character. `s.indexOf("<", from: i)` finds an offset
  and `s.byteSize()` is the end one; `size`, `take` and `drop` count characters, as `for` over
  `s.chars()` does.
- **Iteration:** `for` walks lists, sets, ranges, `Bytes`, a map's `(key, value)` pairs, a string's
  characters (`s.chars()`) and any `Iter(a)`. `xs.iter()` gives a lazy `Iter(a)` whose pipelines
  (`map`, `filter`, `take`, `enumerate`, `zip`) don't build intermediate lists; `toList` collects
  one.
- **Entity references:** `Ref(World)` is an ID, not a pointer. It can be stored, sent and saved
  anywhere, and it can't be confused with `Ref(Player)`.
- **Special types:** `Secret(a)` (no printing, no saving), `Approval` (only a human approval flow
  can create one), `Key` (an idempotency key), `Run(a)` (a started job), `Handle(name)` (an object
  owned by foreign code, [section 18](/reference/language/calling-c-and-go)).
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
