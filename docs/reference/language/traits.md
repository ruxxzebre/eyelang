---
title: "Traits"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Traits

```eye
trait Named {
  fn name(self) -> String
}

impl Named for Player {
  fn name(self) = self.nick
}

fn label(x: Named) -> String = "<{x.name()}>"     # any type with an `impl Named` (section 8)
```

- A type that wraps one value in a variant of its own name, such as `type WorldId = | WorldId(String)`,
  shows as the value it wraps: `"sleep-{w.id}"` is `sleep-survival`, not `sleep-WorldId("survival")`.
- **An impl gives every method its trait declares**, apart from those the trait gives a default
  body, and no others; methods that aren't the trait's go in `impl Type { … }`. Its methods may
  leave out their types, which come from the trait, as `fn name(self) = self.nick` does above.
- **Coherence.** An impl lives in the module that declares the trait or the one that declares the
  type, so two modules can't give one type two meanings of a trait. A type implements a trait
  once for each set of type arguments.
- `Eq`, `Ord`, `Show`, `Hash`, `Save` and `Schema` are derived automatically for records, variants and
  collections of types that have them. Derived `Ord` orders a type's variants as they are declared,
  then by their fields, and a record's fields in the order the type declares them, so
  `type Priority = Low | Normal | High` has `Low < High`. An impl of `Show`, with its `show`
  method, replaces the derived form wherever the value is written.
- What uses them asks for them: `==` needs `Eq` and `<` `Ord`; `"{x}"`, `print` and `log` need
  `Show`; `sort`, `min` and `max` need `Ord`, `toSet` `Hash`, `sum` `Add`; whatever a job saves
  needs `Save`, and `ai.generate` the `Schema` of the type it asks for.
- `Secret(a)` deliberately has none of `Show`, `Save`, `Eq`, `Ord`, `Hash` or `Schema`, and a
  function or a `Handle` has no `Show`, `Save` or `Schema`. Printing a secret, or holding one
  across a `!`, is a compile error, and so is a record that holds one in either place.
- Trait calls are resolved at compile time where the type is known, so they cost nothing extra.
- **Operators are traits.** `a + b` calls `Add`, and likewise `-` (`Sub`), `*` (`Mul`), `/`
  (`Div`), `%` (`Rem`) and unary `-` (`Neg`); `==` uses `Eq` and `<` uses `Ord`. A trait names the
  right-hand type and the result, and a type implements it at most once per right-hand type, so
  the result is always known from the operands:

  ```eye
  trait Add(rhs, out) {
    fn add(self, other: rhs) -> out
  }

  type Money = { cents: Int }

  impl Add(Money, Money) for Money {
    fn add(self, other: Money) -> Money = { cents: self.cents + other.cents }
  }

  let total = price + shipping                 # Money + Money
  fn double(n: t) -> t where t: Mul(Int, t) = n * 2
  ```

  A private function that leaves its types out has one type, inferred from its body and its calls:
  `fn double(n) = n * 2` doubles `Int`s. To work for any type with the operator, it says so.

  The built-in types come with the combinations that make sense: `Int + Int`, `Float + Float`,
  `Duration + Duration`, `Instant + Duration`, `Instant - Instant` (a `Duration`), `Duration * Int`,
  `Size + Size`, `Int / NonZero(Int)`, and `String + String` and `List(a) + List(a)`, which join.
  Nothing converts implicitly: `1 + 2.0` is an error.
