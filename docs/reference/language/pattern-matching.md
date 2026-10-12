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
  ['-', '>', ..] => Arrow
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
be written alone too, `fn t() = Blocked`, when no other imported module has one of that name
([section 13](/reference/language/programs-and-concurrency)).

Every `match` must be exhaustive. Adding a variant to a type makes each `match` that doesn't handle
it a compile error, which is how a new outcome from another service reaches every caller.
