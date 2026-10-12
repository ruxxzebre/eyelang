---
title: "Pattern matching"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Pattern matching

```eye
match grant {
  Admitted(revision) => use(revision)
  Pending(receipt) if waited(receipt) > 1 h => escalate(receipt)
  Pending(receipt) => wait(receipt)
  Refused(..) => report(grant)
}

let { name, age } = user                     # destructuring
let [first, ..rest] = items else { return }
let (key, value) = entry

match rest {                                 # rest: Bytes
  ["->", ..] => Arrow
  ['0'..='9', ..] => number(rest)
  [] => End
  _ => Illegal
}
```

Patterns are names (which bind), `_`, literals (including negative numbers and characters),
ranges of numbers or characters (`'a'..='z'`, `0..10`), variants with positional or named payloads
(`Sent(receipt: r, ..)`), records (`{ name, age: a, .. }`), tuples (`(k, v)`), lists (`[first,
..rest]`, which also match `Bytes` and slices) and alternatives (`Asleep | Waking`). `..` skips the
remaining fields or elements; `..rest` binds the remaining elements.

A variant pattern names a variant of the matched value's type, so a variant declared in an imported
module needs no module prefix in a pattern, even when another module has a variant of the same name:
`Refused(reason)` in the tour matches zeropod's `Refused`. Outside patterns an imported variant may
be written alone too, `fn t() = Blocked`, when no other imported module has one of that name, or
when the type expected where it is written says which ([section 13](/reference/language/programs-and-concurrency)).

**A string literal matches bytes.** Where `Bytes` or a `List(Byte)` are matched, a string literal
stands for the bytes of its text in UTF-8: alone it matches exactly those bytes, and as an element
of a list pattern it matches them in a row, so `"->"` above is `'-', '>'`. With `..` it matches a
prefix, and `..rest` binds what follows:

```eye
type Word = Keyword | Number | Other

fn word(src: Bytes) -> Word = match src {
  "let" | "var" | "continue" => Keyword
  ["0x", _, ..] | ["0b", _, ..] => Number
  _ => Other
}
```

Every `match` must be exhaustive. Adding a variant to a type makes each `match` that doesn't handle
it a compile error, which is how a new outcome from another service reaches every caller. A string
pattern over bytes counts as the list pattern it is.
