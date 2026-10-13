---
title: "Values and types"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Values and types

- **Primitives:** `Int`, `NonZero(Int)`, `Float`, `Bool`, `Char`, `String`, `()`. A
  `NonZero(Int)` is what `/` and `%` take as a divisor, so dividing can't fail ([section 10](/reference/language/errors-result-only)). Most
  code doesn't need one by name: `xs.average()` and `a.div(b)` give an `Option` instead.
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
    var hash: UInt32 = 0x811C_9DC5
    for b in data {
      hash = hash.xor(UInt32.from(b.toInt()) ?? 0).wrappingMul(0x0100_0193)
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
  `Byte` is expected; a string literal matches its bytes where `Bytes` are matched ([section 9](/reference/language/pattern-matching)).

  `Bytes` come from a string, `s.bytes()`; from a file, `fs.readBytes(path)`, whatever it holds;
  and from a list, `toBytes(xs)`, which takes a `List(Byte)`. Bytes that aren't text are built
  as a list, changed with element assignment ([section 7](/reference/language/mutation)), and made `Bytes`; `b.toText()` gives
  `Some` of their text when they are UTF-8, and `fs.writeBytes(path, b)` writes them:

  ```eye
  fn utf8FromLatin1(text: Bytes) -> Bytes {
    var out: List(Byte) = []
    for b in text {
      out += if b < 0x80 then [b] else [b.shiftRight(6).or(0xC0), b.and(0x3F).or(0x80)]
    }
    toBytes(out)
  }
  ```
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

  A lambda's result goes where the call's result goes, so a record literal in a lambda's body
  takes its type from there: from an annotation, a return type, a parameter or a field, through
  any stages of a pipeline, and into the branches of an `if` or a `match` in the body. The call
  takes that type before its lambdas are checked, so a later stage's lambda can read the fields:

  ```eye
  let rows: List(Row) = users |> map(fn u: { name: u.name, age: u.age })   # each a Row

  fn adults(users: List(User)) -> List(Row) =
    users |> map(fn u: { name: u.name, age: u.age }) |> filter(fn r: r.age >= 18)   # r is a Row

  let names = users |> map(fn u: { name: u.name }) |> map(fn r: r.name)   # error: which record type?
  ```

  Where nothing says, it is still an error, as `names` shows; write the type on the `let`, or on
  the lambda's parameter, `fn (r: Row): r.name`.
- **Variants:** `type Grant = Admitted(revision: Revision) | Pending(receipt: ReceiptId) | Refused(reason: Reason)`,
  built with `Pending(receipt: id)`. Payload fields may be named or positional (`Some(a)`). A
  variant list needs at least one `|`, so a type with a single variant starts with one:
  `type Wrapper = | Wrapped(Int)`. Without a `|`, `type Ids = List(Id)` is an alias.
  A type and its one variant may share a name, `type Id = | Id(String)`.
- **A variant is a value, not a type.** Where a type goes, a name means a type: the module's, the
  prelude's, an import's or a built-in one. So a variant may be named like a built-in type:
  with `type Token = Name | Int | String`, `fn next(n: Int) -> Int` takes the built-in `Int`,
  while `Int` in an expression or a pattern is the variant. A variant written where a type goes
  is an error that names its type, `Circle is a variant of Shape, not a type: write Shape`; the
  one exception is a variant that shares its type's name, which names that type.
- **Built-in variants:** `Option(a) = Some(a) | None` and `Result(a, e) = Ok(a) | Err(e)`.
  There is no null.
- **Type variables** are lower-case names in types: `List(a)`, `Map(k, v)` ([section 8](/reference/language/methods-and-generics)).
- **Ranges:** `0..n` counts up to but not including `n`, and `0..=n` includes it. Ranges drive `for`
  loops, slices and patterns. A range is a value of type `Range(a)` with fields `start` and `end`,
  which hold the bounds as written: `(0..5).end` and `(0..=5).end` are both 5, so `end` doesn't say
  whether the range includes it. A range whose end comes before its start is empty: `5..2` holds
  nothing, and a `for` over it runs no times. A range of times of day is the exception: it wraps
  midnight, so `23:00..08:00` holds 23:30 and 07:00, as quiet hours need. A range has `Eq`,
  `Hash` and `Show` when its bounds do. Two ranges are equal when they are written alike, with
  equal bounds and the same `..` or `..=`: `1..5 == 1..5`, but `1..5 != 1..=5`, and `5..2 != 6..1`
  though both are empty. A range shows as it is written: `"{0..=n}"` with `n` 5 is `0..=5`.
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
- **Regular expressions** come from the standard module `regex`, imported with `import regex`.
  `regex.parse(pattern)` gives `Ok` of a `Regex`, or `Err(BadPattern(pattern, reason))`. A
  pattern is RE2's syntax, the one Go's `regexp` reads: classes, repetition, `(?i)` and the other
  flags, and groups, named with `(?P<name>…)` or `(?<name>…)`. It has no backreferences and no
  lookaround, so matching takes time linear in the size of the text whatever the pattern, and a
  pattern from a user can't stall a program. A `Regex` is a value that holds its pattern: it
  compares and hashes by the pattern as written, so `a+` and `aa*` are different, it shows as its
  pattern, and it can be saved, so a job keeps one across a `!`. Its methods only compute:

  | Method | What it gives |
  |---|---|
  | `re.matches(text)` | whether the pattern matches the whole of `text`; `re.find(text) != None` asks whether it matches anywhere in it |
  | `re.find(text)` | `Some` of the first match, a `Match`, or `None` |
  | `re.findAll(text)` | every match, left to right, none overlapping |
  | `re.replace(text, with)` | `text` with every match replaced, as a string's `replace` replaces every occurrence; `re.replaceFirst(text, with)` replaces the first |
  | `re.split(text)` | the parts of `text` between matches, as a string's `split` gives the parts between separators |

  A `Match` has the `text` it matched; its `range` in the text searched, a `Range(Offset)`;
  `groups`, a `List(Option(String))` of what each group took, `None` for a group that took no
  part; and `named`, a `Map(String, String)` of the named groups that took part. `m.group(n)` is
  group `n`, and `m.group(0)` the whole match. In a replacement, `$1` is what group 1 took,
  `$<name>` what a named group took, and `$$` is a `$`; `$1x` is group 1 followed by `x`, and
  `$<1>0` group 1 followed by `0`. The brackets are JavaScript's, because Go's `${name}` would be
  interpolation in an Eyelang string. `regex.escape(text)` quotes text to match it as it is.

  ```eye
  import regex

  ## Dates written 2026-10-12, as 12/10/2026.
  fn irishDates(text: String) -> Result(String, regex.RegexError) {
    let date = regex.parse("(\\d\{4})-(\\d\{2})-(\\d\{2})")?
    Ok(date.replace(text, "$3/$2/$1"))
  }
  ```

  In a string, a pattern's `\` is written `\\` and its `{` is written `\{`, as above. A pattern
  written in the program is parsed when the program runs, like any other; where the program
  knows it is right, a `match` on `regex.parse(p)` whose `Err(e)` arm is `unreachable("{e}")`
  says so (open question 44 asks for patterns checked when the program is compiled).
- **Iteration:** `for` walks lists, sets, ranges, `Bytes`, a map's `(key, value)` pairs, a string's
  characters (`s.chars()`) and any `Iter(a)`.
- **Lazy iteration:** `xs.iter()` gives an `Iter(a)`, a sequence computed as it is walked.
  `map`, `filter`, `take`, `drop`, `enumerate`, `zip` and `flatMap` on an `Iter` give another
  `Iter` and build no lists in between (`zip` does when either side is one); a `for` loop, or a
  function that needs the elements, walks it, and `toList` collects it:

  ```eye
  let squares = (1..1_000_000_000).iter() |> map(fn n: n * n) |> filter(fn n: n % 3 == 1)
  let few = squares |> take(3) |> toList    # [1, 4, 16]: squares four numbers, not a billion
  for (i, c) in "héllo".chars() |> enumerate { print(i, c) }
  ```

  - **Sources.** Lists, sets, maps (their `(key, value)` pairs), ranges of `Int`, strings (their
    characters) and `Bytes` have `iter()`; `s.chars()` is `s.iter()`, and an `Iter`'s `iter()`
    is itself. Every source ends, so `size`, `toList` and `sort` on an `Iter` always finish; a
    large range stands in for an endless count.
  - **An `Iter` is a value.** Walking it twice gives the same elements: each walk starts again
    from the source and runs the functions again. An `Iter` that a first walk used up would be a
    cursor shared by everyone holding it, the shared state [section 1](/design/goals) rules out. To walk once and
    keep the results, `toList`.
  - **Its functions only compute.** They run whenever and wherever the `Iter` is walked, so a
    function given to `map`, `filter` or `flatMap` on an `Iter` has no effects: `print` belongs
    in the loop that walks it. A callback parameter given to one states `uses {}` in its type,
    as a function kept in a record does ([section 12](/reference/language/effects-and-signatures)).
  - **What walks it.** `toList`, `toSet`, `reduce`, `any`, `all`, `find`, `first`, `last`,
    `size`, `isEmpty`, `count`, `contains`, `sum`, `join`, `sort`, `reverse` and `parallelMap`
    walk an `Iter`, and `first`, `find`, `any`, `all`, `contains` and `isEmpty` stop at the
    answer. An `Iter` has no positions, so `get`, `xs[i]` and slices don't take one.
  - **It can't be kept.** It holds functions, so it has none of `Eq`, `Ord`, `Show`, `Hash`,
    `Save` and `Schema`: it can't be compared, printed or saved, so it can't be live across a `!`,
    and a job's `for` loop over one can't wait inside: collect it with `toList` first, so a
    resumed run finds its place.
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
