---
title: "Traits"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Traits

```eye
trait Named {
  fn name(self) -> String
}

impl Named for Account {
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
  once for each set of type arguments. An `impl` of a contract is the exception: it gives no type
  a trait, but says which module serves the contract, so it may go in any module, as the lab's
  `notifier` serves `notify.Notify` ([section 17](/reference/language/contracts-talking-to-other-services)).
- `Eq`, `Ord`, `Show`, `Hash`, `Save` and `Schema` are derived automatically for records, variants and
  collections of types that have them. Derived `Ord` orders a type's variants as they are declared,
  then by their fields, and a record's fields in the order the type declares them, so
  `type Priority = Low | Normal | High` has `Low < High`. An impl of `Show`, with its `show`
  method, replaces the derived form wherever the value is written.
- What uses them asks for them: `==` needs `Eq` and `<` `Ord`; `"{x}"`, `print` and `log` need
  `Show`; `sort`, `min` and `max` need `Ord`, `toSet` `Hash`, `sum` `Add`; whatever a job saves
  needs `Save`, and `ai.generate` the `Schema` of the type it asks for.
- `Secret(a)` deliberately has none of `Show`, `Save`, `Eq`, `Ord`, `Hash` or `Schema`, and nor
  does a value of a program's `extern type` ([section 18](/reference/language/calling-c-and-go)). A function has no `Show`, `Save` or
  `Schema`. Printing a secret, or holding one across a `!`, is a compile error, and so is a record
  that holds one in either place.
- **Enums list, read and number their variants.** A type whose variants have no fields gets two
  functions on the type, derived as the traits are: `T.variants()`, every variant in the order
  declared, and `T.parse(s)`, `Some` of the variant whose name is `s`, else `None`. Each value
  gets a method too, `v.index()`, its place in `T.variants()` counting from 0, so
  `T.variants()[v.index()]` is `Some(v)`: a number for a table, a checksum or a foreign format,
  as Go's `int(kind)` is. A choice that arrives as text,
  such as a pressed button, is then matched as a variant, so a misspelt name or a forgotten choice
  is a compile error:

  ```eye
  type Admission = Guest | Member | Refuse

  fn admit(choice: String, user: UserId) -> Step =
    match Admission.parse(choice) {
      Some(Guest) => Admit(user, groups: toSet(["guests"]))
      Some(Member) => Admit(user, groups: toSet(["members"]))
      Some(Refuse) | None => Reject(user)
    }

  let buttons: List(Button) = Admission.variants() |> map(fn a: { id: "{a}", label: "{a}" })
  let order = Member.index()                  # 1
  ```

  `parse` takes the variant's name, which is what the derived `Show` writes, so
  `T.parse("{v}")` is `Some(v)` unless an impl of `Show` writes something else. A record, or a
  type with a variant that has fields, has none of the three: `Shape.variants()` is a compile
  error that names the variant with fields. A method the type declares with one of the names is
  called instead. Whether a choice's label should come from its `##` comment or from `Show` is
  open question 15.
- Trait calls are resolved at compile time where the type is known, so they cost nothing extra.
- **Operators are traits.** `a + b` calls `Add`, and likewise `-` (`Sub`), `*` (`Mul`), `/`
  (`Div`), `%` (`Rem`) and unary `-` (`Neg`); `==` uses `Eq` and `<` uses `Ord`. A trait names the
  right-hand type and the result, and a type implements it at most once per right-hand type, so
  the result is always known from the operands:

  ```eye
  # The prelude declares:
  #   trait Add(rhs, out) {
  #     fn add(self, other: rhs) -> out
  #   }

  type Money = { cents: Int }

  impl Add(Money, Money) for Money {
    fn add(self, other: Money) -> Money = { cents: self.cents + other.cents }
  }

  let total = price + shipping                 # Money + Money
  fn double(n: t) -> t where t: Mul(Int, t) = n * 2
  ```

  A private function that leaves its types out is generic: what its body doesn't decide, each
  call does, and the call must give types that have the operators the body uses.

  ```eye
  fn twice(n) = n + n

  let a = twice(21)                            # 42, an Int
  let b = twice(1.5)                           # 3.0, a Float
  ```

  A literal decides a type: in `fn double(n) = n * 2`, the `2` is an `Int`, so `double` takes
  `Int`s, and `double(1.5)` is an error, as `1.5 * 2` is. A public function says what it takes, as
  `fn double(n: t) -> t where t: Mul(Int, t)` above does.

  The built-in types come with the combinations that make sense: `Int + Int`, `Float + Float`,
  `Duration + Duration`, `Instant + Duration`, `Instant - Instant` (a `Duration`), `Duration * Int`,
  `Size + Size`, `Int / NonZero(Int)`, `String + String` and `List(a) + List(a)`, which join,
  and `Set(a) + Set(a)`, which gives the elements of both.
  Nothing converts implicitly: `1 + 2.0` is an error.
