---
title: "Methods and generics"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Methods and generics

```eye
pub opaque type Lexer = { src: Bytes, pos: Int, tokens: List(Token) }

impl Lexer {
  pub fn new(src: Bytes) -> Lexer = { src, pos: 0, tokens: [] }
  pub fn peek(self) -> Byte = self.src[self.pos] ?? 0
  pub fn advance(mut self) { self.pos += 1 }
}

fn skipSpaces(mut lexer: Lexer) {
  while lexer.peek() == ' ' { lexer.advance() }
}

fn largest(xs: List(a)) -> Option(a) where a: Ord {
  var best = first(xs)?                      # ? on an Option returns None
  for x in xs {
    if x > best { best = x }
  }
  Some(best)
}
```

- **Methods** are declared in `impl Type { … }` in the module that defines the type, and called as
  `x.method(args)`. A function in the block without `self` is called on the type: `Lexer.new(src)`.
  Trait methods are called the same way ([section 11](/reference/language/traits)).
- **A field that holds a function is called like a method.** With `type Timer = { name: String,
  read: fn() -> Instant uses {clock} }`, `timer.read()` calls the function in the field; the call
  has the effects its type states. So that the call means one thing, a record type can't have a
  method named like a field that holds a function.
- **Pipes and methods.** `x |> f(a)` calls an ordinary function with `x` first; `x.f(a)` calls a
  method. Collection operations in the standard library are functions, used with pipes, so the same
  pipeline works on lists, iterators and entity sets.
- **Type variables** are lower-case names in types, as in Gleam: `fn first(xs: List(a)) -> Option(a)`,
  `type Pair(a, b) = { first: a, second: b }`. A lower-case name followed by `.` is a module:
  `notify.Delivery`.
- **Requirements** on type variables follow `where`: `where a: Ord + Show`. A trait used as a
  parameter type is short for one: `fn label(x: Named)` means `fn label(x: t) where t: Named`.
  Inside the function a type variable has only what its requirements give it: `x > best` needs
  `a: Ord`, `x + y` needs `a: Add(a, a)`, and `x.name()` a trait with a `name` method. A call
  checks that the types it passes have them: `largest([s])` with a `Secret` is an error, since a
  `Secret` has no `Ord`. A trait is a requirement, not a type, so it can't go inside one
  (`List(Named)` is written `List(t) where t: Named`).
- Generic code is specialised for the types it is used with, so it costs nothing at run time.
- **`dyn Trait`** (from v0.2) is a value of some type that implements the trait, chosen at run
  time. Calls on it look up the implementation, and `dyn` in the type says so:

  ```eye
  fn notifier(config: Config) -> dyn Notifier =
    if config.telegram then Telegram.new(config.token) else Console.new()

  let handlers: List(dyn Handler) = [Logger.new(), Metrics.new(port: 9100)]
  ```

  - A value converts to `dyn Trait` where one is expected, if its type implements the trait:
    `expected dyn Notifier, found Email: Email doesn't implement Notifier` otherwise. Since values
    never alias, a `List(Logger)` is also a `List(dyn Handler)`. Nothing converts back.
  - A `dyn Trait` has its trait's methods and nothing else: no `Show`, `Eq` or `Save`, so it can't
    be printed, compared, or live across a `!` (its type, which saving needs, is known only at run
    time).
  - A trait takes its type arguments as in a bound, `dyn Add(Money, Money)`, and a trait with a
    method that has type variables of its own can't be used this way, since a call through `dyn`
    couldn't choose them.

  A function that takes a trait as a parameter type (`fn label(x: Named)`) is still generic and
  specialised; `dyn` is for values whose types differ at run time.
