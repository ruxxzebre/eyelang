---
title: What runs today
---

# What runs today

The [language reference](/reference/language/) describes Eyelang as it is designed. This page says
how much of each part is built. It describes the code at commit `737f783` on 2026-10-11, and it is
updated with each merge.

Each feature has three columns:

- **Parses**: both parsers accept it. The conformance suite checks that they agree on every file in
  `testdata/syntax/valid` and on the tree-sitter corpus, and every example in the spec is one of those files.
  Every example also passes the checker, in the program from `testdata/specctx` that its fence
  names, apart from the one marked as waiting for a task (6.8, the `agents` module); see
  [specdoc's check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/internal/specdoc/check_test.go#L26).
- **Checks**: `eye check` resolves and type-checks it, or reports a specific error when it is misused.
- **Runs**: the reference interpreter runs it, and a passing test shows that.

A cell holds ✓ (done), "partial" with what is missing, "—" (not built), or "n/a" (the column
doesn't apply). A feature with "—" under Checks and Runs is **spec only**: it parses, and then it is
rejected or ignored. "Shown by" links to a test or program that shows the feature working. For
compile-time errors it links to the checker's test cases. Every error class the spec promises is
listed in [check/classes_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/classes_test.go),
along with the test that reports it.

**The biggest gaps today.** Coding agents, `@unwitnessed` recordings, and native compilation of
entities, jobs, views and contracts (v0.4): `eye build` compiles the core language only.

## [A tour](/reference/language/a-tour)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| The tour: an entity, a view, a job that waits, a contract with fakes, three tests | ✓ | ✓ | ✓ | [programs/tour](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tour/tour.eye#L57) |

## [Lexical structure](/reference/language/lexical-structure)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| `#` comments; `##` doc comments with a summary, `[Name]` links and a module doc | ✓ | partial: links are checked by `eye doc`, not `eye check` | ✓ (they become a model schema's descriptions) | [apidoc_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/internal/apidoc/apidoc_test.go#L12), [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L243) |
| Names: upper case for types, lower case for values | ✓ | ✓ | ✓ | [conformance](https://github.com/ruxxzebre/eyelang/blob/dev/syntax/conformance_test.go#L56) |
| Int, Float, String with interpolation, Bool, Unit | ✓ | ✓ | ✓ | [core: integers, strings](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L56) |
| Char literals and escapes | ✓ | ✓ | ✓ | [core: match](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L117) |
| Duration and Size literals | ✓ | ✓ | ✓ | [core: strings](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L70) |
| Time of day, Date, Instant, next time of day in a zone | ✓ | ✓ | partial: `07:45 in zone` has no test yet | [core: time](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L185) |
| A zone given by name or by a `Zone` value, checked when compiled | ✓ | ✓ | ✓ | [core: zones](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L231), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L156) |
| Multi-line strings with a margin | ✓ | ✓ | ✓ | [core: multi-line](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L194) |
| Contextual keywords (`state`, `view`, `type`, `test` as names) | ✓ | ✓ | n/a | [contextual-names.eye](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/syntax/valid/contextual-names.eye) |
| Newlines end statements; `;` separates them | ✓ | n/a | ✓ | [newlines.eye](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/syntax/valid/newlines.eye), [core: pipes](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L138) |
| Braces only for blocks; one-expression branches; `{` after `=>` | ✓ | ✓ | ✓ | [arm-records.eye](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/syntax/valid/arm-records.eye), [core: branches](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L208) |
| Waiting calls `f!(x)` | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L26) |

## [Values and types](/reference/language/values-and-types)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| `Int` with arbitrary precision | ✓ | ✓ | ✓ | [core: integers](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L56) |
| `NonZero(Int)` divisors and `nonzero(n)` | ✓ | ✓ (`/` and `%` on an `Int` take a `NonZero(Int)` or a literal other than 0) | ✓ | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L316) |
| `average`, `div` and `rem`: division that gives an `Option` instead of needing a `NonZero` | ✓ | ✓ (`average` takes `Int`s or `Float`s and gives an `Option(Float)`; `div` and `rem` take two `Int`s) | ✓ | [core: div](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L270), [core: average](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L280) |
| `Float` | ✓ | ✓ | ✓ | [core: match](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L98) |
| `Byte`, `Bytes`, `Int32`, `Int64`, `UInt32`, `UInt64` | ✓ | ✓ (`wrappingAdd`, `checkedAdd` and the rest of SPECS §5's methods, `T.from(n)`, `x.toInt()`; `+` and the other operators are errors that name the methods; a literal too large for its type is an error at its span; an ASCII character literal is a `Byte` where one is expected) | ✓ (a fixed-width value is an `Int` at run time, and its methods take the width from the receiver's type in the typed IR) | [core: fixed](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/fixed.eye#L3), [core: bytes](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/strings.eye#L82), [fixed_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/fixed_test.go#L13) |
| `Instant`, `Date`, `Time`, `Duration`, `Zone`, `Schedule`, `Size` | ✓ | ✓ | ✓ | [core: time](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L185) |
| `List`, `Map`, `Set` as values | ✓ | ✓ | ✓ | [core: collections](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L156) |
| Tuples, and lambdas that take a tuple's parts | ✓ | ✓ | ✓ | [core: closures](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L147) |
| Records, `with` updates, field assignment | ✓ | ✓ | ✓ | [core: records](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L74) |
| Nominal records; a literal takes the type expected | ✓ | ✓ | ✓ | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L79) |
| Variants, one-variant types, aliases | ✓ | ✓ | ✓ | [core](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L3) |
| `Option` and `Result` | ✓ | ✓ | ✓ | [core: errors](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L124) |
| Type variables | ✓ | ✓ (one can't be called or indexed; a requirement on an untyped parameter waits for its calls) | ✓ | [programs/traits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/traits/traits.eye#L33), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L261) |
| Ranges `..` and `..=`, as loops, patterns and values; time ranges that wrap midnight | ✓ | ✓ | ✓ | [core: mutation](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L94), [lab notifier: quiet hours](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/notifier/notifier.eye#L242) |
| Slices `xs[a..b]`; `s.slice(range)` on strings, by `Offset`, with `indexOf`, `start`, `end`, `after`, `before` and `charAt`; an `Int` there is an error that says where offsets come from | ✓ | ✓ | ✓ | [core: collections](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L166), [core: strings](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/strings.eye#L3), [offset_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/offset_test.go) |
| Iteration with `for`; `s.chars()`; lazy `xs.iter()` | ✓ | ✓ | ✓ | [core: iter](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/iter.eye#L6), [checker cases](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L279), [core: mutation](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L94) |
| `Ref(E)`, different for each entity | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L35) |
| `Secret(a)`: no printing, no saving | ✓ | ✓ | ✓ | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L180) |
| `Approval`, made only by `approval!`, with `what` and `by` | ✓ | ✓: a `@destructive` command or function requires one, and `ok.what` and `ok.by` read what was approved and by whom | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/approvals.eye#L64), [lab access](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/access/access.eye#L192), [programs/contracts](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/contracts/contracts.eye#L116) |
| `Version`: a snapshot's `version`, `==` and no order, from an integer literal | ✓ | ✓ | ✓ | [programs/contracts](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/contracts/contracts.eye#L130) |
| `Key`, `Run(a)` | ✓ | ✓ | ✓ | [programs/limits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/limits/limits.eye#L101) |
| A program's `extern type`, whose values foreign code owns: not saved, shown or compared (`Handle` is gone, open question 21) | ✓ | ✓ | — (no foreign code yet) | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L285) |
| `opaque` types | ✓ | ✓ (other modules can't read, write, build or match its fields or variants) | ✓ as an ordinary type | [visibility_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/visibility_test.go#L35), [spec-08 example](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/syntax/valid/spec-08-methods-and-generics-1.eye) |
| String literals as `Key` or `Zone` where one is expected | ✓ | ✓ | ✓ | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L145) |
| Map literals, `(key):` keys, `{ name, age }` shorthand, `{}` | ✓ | ✓ | ✓ | [core: collections](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L156) |

## [Bindings, functions and pipes](/reference/language/bindings-functions-and-pipes)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| Public functions with full signatures; private ones inferred, and generic where their bodies leave types open | ✓ | ✓ (each call decides the open types, and must give the operators and traits the body uses) | ✓ | [core](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L151), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L261) |
| `let`, `var`, shadowing | ✓ | ✓ | ✓ | [core: mutation](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L83) |
| Lambdas: one expression, a block, several parameters, none | ✓ | ✓ (a parameter may state its type, `fn (item: Item) { … }`, which must be what the lambda is given) | ✓ | [core: closures](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L147), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L718) |
| Data-first pipes and `_` | ✓ | ✓ | ✓ | [core: pipes](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L134) |
| `?` after a piped call | ✓ | ✓ | ✓ | [core](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L52) |
| `if` as an expression, `then`, blocks | ✓ | ✓ | ✓ | [core](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L30) |
| `for`, `while`, `break`, `continue`; `while true` as any type | ✓ | ✓ | ✓ | [core: loops](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L173) |
| Named arguments with defaults | ✓ | ✓ | ✓ | [core: pipes](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L143) |
| `name:` passes the variable of that name | ✓ | ✓ | ✓ | [core: labels](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L225) |
| A function without `->` returns `()` | ✓ | ✓ | ✓ | [core](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L35) |
| Module-level `let` and `pub let` | ✓ | ✓ | ✓ | [lab home](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/home/home.eye#L7) |
| A branch that is one assignment | ✓ | ✓ | ✓ | [core: branches](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L208) |
| Errors for `_` outside a pipe, `return` in a lambda, `break` outside a loop, a name declared twice | ✓ | ✓ | n/a | [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L112) |

## [Mutation](/reference/language/mutation)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| `var`, field assignment, `+=` and the other compound assignments | ✓ | ✓ | ✓ | [core: mutation](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L83) |
| A `let` never changes | ✓ | ✓ | n/a | [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L35) |
| `mut` parameters, with `mut` written at the call | ✓ | ✓ | ✓ | [core: mutation](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L85), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L253) |
| `mut self` methods, only on a `var` or `mut` parameter | ✓ | ✓ | ✓ | [core: mutation](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L90), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L275) |
| No aliasing: no variable passed as `mut` twice, or as `mut` and read | ✓ | ✓ | n/a | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L264) |
| Closures can't capture a `var` or `mut` parameter | ✓ | ✓ | n/a | [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L45), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L283) |
| Updates in place when a value is unique | n/a | n/a | partial: appending to a list (`xs \|> add(x)`, `xs += [x]`, `xs + ys`), a set's `add` and a map's `insert` of a new key are in place in the interpreter, and replacing a map's value copies only O(log n) of it (a persistent vector); `s += t` and `s = s + t` on a local `String` variable append in place; `with` on records and other `String +` still copy, since the interpreter has no reference counts. Compiled by `eye build`, a variable updated by itself (`s += t`, `xs += ys`, `xs = xs.add(x)`, `m = m.insert(k, v)`, a set's `add`, field assignment) changes in place when it holds the only reference; `with` and values passed on by their last use still copy until precise counting (7.5) | [core: mutation](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L89), [append_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/append_test.go#L63), [shelf_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/shelf_test.go#L168), [vec_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/vec_test.go), [text_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/text_test.go), [cgen: in place](https://github.com/ruxxzebre/eyelang/blob/dev/cgen/testdata/inplace/inplace.eye) |

## [Methods and generics](/reference/language/methods-and-generics)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| A field that holds a function, called like a method: `timer.read()` | ✓ | ✓ (a method named like such a field is an error) | ✓ | [programs/traits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/traits/traits.eye) |
| `impl Type { … }` methods, and functions called on the type (`Lexer.new`, and `lex.Lexer.new` from another module) | ✓ | ✓ | ✓ | [core](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L18), [core: modules](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/modules.eye#L3), [visibility_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/visibility_test.go#L35) |
| An `impl Type` only in the module that declares the type | ✓ | ✓ | n/a | [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L117) |
| Generic functions and types with type variables | ✓ | ✓ | ✓ | [programs/traits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/traits/traits.eye#L33) |
| `where` requirements, and a trait as a parameter's type | ✓ | ✓ | ✓ | [programs/traits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/traits/traits.eye#L27), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L225) |
| Generic code specialised at compile time | n/a | ✓ (`eye check --lir`: one instance per set of types; polymorphic recursion is an error) | ✓ compiled to C by `eye build`, for the core language | [lir_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/lir/lir_test.go), [cgen_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cgen/cgen_test.go) |
| `dyn Trait` (v0.2) | ✓ | ✓ | ✓ | [programs/traits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/traits/traits.eye#L109), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L188) |

## [Pattern matching](/reference/language/pattern-matching)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| Variants with positional or named payloads, guards, alternatives, `..` | ✓ | ✓ | ✓ | [core: match](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L98) |
| Literals, negative numbers, characters, ranges | ✓ | ✓ | ✓ | [core: match](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L117) |
| Record and tuple patterns; destructuring `let` | ✓ | ✓ | ✓ | [patterns.eye](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/syntax/valid/patterns.eye), [core: closures](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L152) |
| List patterns `[first, ..rest]` and `let … else` | ✓ | ✓ | ✓ | [core: match](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L109), [core: errors](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L130) |
| List patterns over `Bytes` with character literals | ✓ | ✓ (`['-', '-', ..rest]`, `Some('a'..='z')`; `'é'` where a `Byte` is expected is an error at its span) | ✓ | [core: fixed](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/fixed.eye#L166), [fixed_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/fixed_test.go#L13) |
| Variants of imported modules without a prefix; an error when two modules declare one | ✓ | ✓ | ✓ | [programs/tour](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tour/tour.eye#L44), [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L171) |
| Exhaustive matches, including nested patterns and list lengths | ✓ | ✓ | n/a | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L103) |

## [Errors: Result only](/reference/language/errors-result-only)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| `?` on `Result` and `Option` | ✓ | ✓ (the function, job or lambda it returns from must return an `Option` or a `Result` in turn; a lambda's `?` returns from the lambda and wraps into its error type; a value whose type a later call gives is checked once the function is, and one nothing types is an error that asks for its type) | ✓ | [core: errors](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L124), [core: wrap](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/wrap.eye#L42), [try_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/try_test.go#L14), [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L229) |
| `?` wraps the error in the one variant that holds it | ✓ | ✓ | ✓ | [core: errors](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L127), [core: wrap](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/wrap.eye#L34), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L96) |
| A `Result` can't be dropped by a statement: it is used, matched, passed on with `?` or discarded with `let _ = …`; a block's last expression is used when the block's value is, and `()` and `Option`s may be dropped | ✓ | ✓ | n/a | [drop_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/drop_test.go#L15) |
| `.mapErr(…)` | ✓ | ✓ | ✓ | [lab digest](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/digest/digest.eye#L132) |
| `Problem` and `.context(…)` | ✓ | ✓ | ✓ | [core: problems](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/problems.eye#L16), [problem_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/problem_test.go) |
| Total library functions: `xs[i]` and `first` give `Option`, `parseInt` and `parseFloat` give `Result`, `toInt` gives `Option` | ✓ | ✓ | ✓ | [core: collections](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L165), [core: numbers](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/strings.eye#L60) |
| String functions: `replace`, `words`, `endsWith`, `chars`, `code`, `fromCode`; Float `round`, `floor`, `ceil`, `toFloat` | ✓ | ✓ | ✓ | [core: strings](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/strings.eye#L39) |
| `unreachable("why")` ends the program or run as a fault | ✓ | ✓ | ✓ | [interp_test.go: faults](https://github.com/ruxxzebre/eyelang/blob/dev/interp/interp_test.go#L153) |
| Runtime faults as a run's outcome | ✓ | ✓ | partial: `Unreachable`, `DeadlinePassed`, `Unavailable`, `OverBudget` and `OutOfMemory` happen; `Killed` never does | [programs/limits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/limits/limits.eye#L101) |

## [Traits](/reference/language/traits)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| Trait declarations, with default methods | ✓ | ✓ (a default method sees self as any type with the trait) | ✓ | [programs/traits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/traits/traits.eye#L71), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L418) |
| `impl Trait for Type`, every method and no others, with the trait's types | ✓ | ✓ | ✓ | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L192) |
| Coherence: an impl lives with its trait or its type, once per type | ✓ | ✓ | n/a | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L208) |
| A one-variant wrapper shows as the value it wraps | ✓ | ✓ | ✓ | [programs/tour](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tour/tour.eye#L44) |
| Derived `Eq`, `Ord`, `Show`, `Hash`, `Save`, `Schema`, asked for where they are used | ✓ | ✓ | ✓ | [programs/traits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/traits/traits.eye#L98), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L244) |
| An impl of `Show` replaces the derived form | ✓ | ✓ | ✓ | [programs/traits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/traits/traits.eye#L93) |
| `T.variants()` and `T.parse(s)` on enums | ✓ | ✓ (refused on a record or a type with a variant that has fields) | ✓ | [programs/traits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/traits/traits.eye#L163), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L772), [lab access](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/access/access.eye#L188) |
| Operators as traits (`Add`, `Mul`, `Neg` for `-x`, …); no implicit conversions | ✓ | ✓ | ✓ | [programs/traits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/traits/traits.eye#L81), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L285) |

## [Effects and signatures](/reference/language/effects-and-signatures)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| `uses` on public functions; none means pure; a private function's uses clause and a trait method's are held too; effect names are checked | ✓ | ✓ | n/a | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L171) |
| `uses` on public entity methods and public jobs, which `eye fmt` writes | ✓ | ✓ | n/a | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L776), [main_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/main_test.go#L146) |
| `start J`: starting a job is an effect; in a job, a function that starts one is a step | ✓ | ✓ | n/a | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L808) |
| Waiting functions: a function with `!` steps has `wait`, and only jobs and other waiting functions call it, with `!`; outside a job `sleep`, `sleepUntil` and `poll` block and keep `wait` in the caller's effects, and a job can't call a function that blocks | ✓ | ✓ (`next`, `approval` and `reserve` outside a run are errors too) | ✓ | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L837) |
| Effects inferred through private functions and callbacks, and functions passed by name | ✓ | ✓ | n/a | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L261) |
| `wait`, `net C.op`, `entity E`, `clock`, `stdio`, `env` | ✓ | ✓ | n/a | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L171) |
| `secret "name"`, `publish C.event`, `consume C.event`, `ffi` | ✓ | ✓ (inferred; `consume` from `next!`, not yet from `@on(event(…))`) | n/a | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L379) |
| `fs`, `process`, `random` | ✓ | ✓ | ✓, each test in a directory of its own, with a fixed seed | [programs/system](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/system/system.eye#L29) |
| Function types in signatures: `f: fn(Int) -> Int` | ✓ | ✓ (a function has none of the derived traits, so it can't be shown, compared or saved) | ✓ | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L406) |
| Function types kept in data state their effects | ✓ | ✓ (calling a function value counts what its type states; a lambda or named function kept where its type states less, a callback kept in data, and a lambda that calls one are errors) | ✓ | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L305) |
| In a job, every call with an outside effect, and every entity call, takes `!` | ✓ | ✓ (publishing takes `publish!`, starting a child run `start!`, an extern call `!`; a step's result must be saveable; `!` on a call that only computes is an error, and printing and logging aren't steps) | ✓ | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L379), [interp_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/interp_test.go#L114), [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L106) |

## [Programs and concurrency](/reference/language/programs-and-concurrency)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| `pub fn main`, returning `()` or `Result`; an `Err` exits with status 1 | ✓ | ✓ | ✓ | [main_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/main_test.go#L238) |
| `eye run` keeps jobs going across restarts, in `.eye/journal` | ✓ | n/a | ✓ | [runner_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/runner_test.go#L68) |
| A job started from `main` needs a key | ✓ | ✓ (also when `main` starts it through a function or entity method it calls) | ✓ | [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L95), [runner_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/runner_test.go#L75) |
| `eye runs` lists runs, what they wait for, and their steps; a model call's step shows the model, tokens in and out, dollars and bytes sent and received, and `eye runs DIR ID STEP` its whole reply | n/a | n/a | ✓ | [runner_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/runner_test.go#L135), [runs_model_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/runs_model_test.go) |
| `eye repl` | n/a | ✓ | ✓ | [repl_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/repl/repl_test.go#L41) |
| Modules as directories; `import a.b` and `import a.b.{Name}`; other modules use only what is `pub` | ✓ | ✓ | ✓ | [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L150), [visibility_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/visibility_test.go#L35), [lab](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/lab.eye) |
| `eye.toml` marks a program's root | n/a | ✓ | ✓ | [index_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/index_test.go#L151) |
| `modules` in `eye.toml`: more directories to find imports in | n/a | ✓ (each must exist, be written relative to `eye.toml` and stay inside the repository; a missing import lists every directory searched) | ✓ | [eyetoml_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/eyetoml_test.go#L12), [lab deploy/digest](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/deploy/digest/eye.toml) |
| `parallelMap`, `both`, `race`, `timeout`, with waits that overlap | ✓ | ✓ (a task can't capture a `var` or a `mut` parameter) | ✓ | [programs/tasks](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tasks/tasks.eye#L18), [tasks_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/tasks_test.go#L16) |
| A task fails, and the call fails with it | ✓ | n/a | ✓ | [tasks_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/tasks_test.go#L70) |
| A task a job starts (`parallelMap`, `both`, `race`, `timeout`, `taskGroup`) only computes | ✓ | ✓ | ✓ (the run fails, should one get past the checker) | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L232) |
| Cheap tasks with small, growing stacks | n/a | n/a | ✓ (each task is a Go goroutine; 10,000 waiting tasks hold about 9 KiB each, under a budget of 12 KiB, on either clock) | [manytasks_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/manytasks_test.go#L14) |
| Task groups, `spawn` and channels (v0.2) | ✓ | ✓ | ✓ | [programs/tasks](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tasks/tasks.eye#L77), [channels_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/channels_test.go#L116) |
| A failing task cancels its group | ✓ | n/a | ✓ | [channels_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/channels_test.go#L31) |
| A group whose tasks all wait for channels nobody can change fails | ✓ | n/a | partial: on the system clock, a group inside another task isn't checked, and waits | [channels_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/channels_test.go#L61) |
| A channel can't be live across a `!` | ✓ | ✓ | n/a | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L224) |
| A task group in a job only computes, in its spawned tasks too | ✓ | ✓ | ✓ (the run fails) | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L200) |
| Child runs joined with `allDone!` and `firstDone!` | ✓ | ✓ (they take a `List(Run(a))`; `all` and `first` are only the collection functions, and `all!` or `first!` is an error that names the wait to write) | ✓ (`firstDone!` stops the runs that lost, which end as `Failed(Killed)`; a run paused in either resumes after a restart) | [programs/waiting](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/waiting/waiting.eye#L35), [waits_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/waits_test.go#L139), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L63) |

## [Entities](/reference/language/entities)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| `entity` with `state` and defaults; exists on first use | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L5) |
| Methods as atomic transactions; nested entity calls commit together | ✓ | ✓ | ✓ (an `Err` commits; a fault part way through a call rolls back every entity it changed, and the jobs it started never run; a call that comes back to an entity already in the call tree shares its state in progress, the outer method goes on from its changes, and the entity commits once) | [entities_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/entities_test.go#L63), [re-entry](https://github.com/ruxxzebre/eyelang/blob/dev/interp/entities_test.go#L343), [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L41), [programs/entities](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/entities/entities.eye#L29) |
| One writer at a time per entity | n/a | n/a | ✓ (tasks that call one entity's method at once lose no change, on a test's clock and on the system clock) | [programs/entities](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/entities/entities.eye#L21), [entities_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/entities_test.go#L175) |
| `state` fields and methods named without `self` | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L9) |
| Methods never wait | ✓ | ✓ | n/a | [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L49) |
| Methods and served operations stay inside the program: no `net`, `fs`, `process`, `clock`, `random`, `env`, `ffi`, `secret` or `wait`, themselves or through what they call; output is allowed | n/a | ✓ (an error at the call that names the effect) | n/a | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L715) |
| A method starts a job when its transaction commits | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L12) |
| The identity can't change | ✓ | ✓ | n/a | [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L101) |
| `read!()` snapshots with a `version`, which no state field may be named; `read()` outside jobs | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L36), [programs/tour](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tour/tour.eye#L44), [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L23) |
| A job reads entities only in steps: `w.phase`, a view and `E.all` are errors in a job and a waiting function; `view.read!()` and `E.all.read!()` record a list of snapshots; outside jobs `w.phase` is `w.read().phase` | ✓ | ✓ (an error that names the step to write) | ✓ (a replayed read gives what it first saw) | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L875), [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/reads.eye#L35) |
| Ending an entity with `end(ok)` and an `Approval` | ✓ | ✓ (only in the entity's own methods, and only with an `Approval`) | ✓ (when the transaction commits the state is removed and every store keeps only when it ended and whose approval let it; the entity leaves its views, and a call on it or a read of it faults, after a restart too) | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L63), [programs/entities](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/entities/entities.eye#L69), [entities_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/entities_test.go#L147) |
| `@version` and `migrate from N`, chained, without gaps, pure | ✓ | ✓ | ✓ | [migrations_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/migrations_test.go#L14), [interp migrate_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/migrate_test.go#L15) |
| `eye run` refuses rows no migration can bring up to date | n/a | n/a | ✓ | [cmd/eye migrate_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/migrate_test.go#L14) |

## [Jobs and waiting](/reference/language/jobs-and-waiting)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| Jobs saved at every `!`; a resumed run never repeats a recorded call | ✓ | ✓ | ✓ | [interp_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/interp_test.go#L88) |
| Runs saved as bytes and finished by a new interpreter | n/a | n/a | ✓ | [interp_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/interp_test.go#L112) |
| Only values with `Save` live across a `!`, and as job arguments | ✓ | ✓ | n/a | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L180) |
| Entity calls commit with the checkpoint | ✓ | ✓ | ✓ | [interp_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/interp_test.go#L112) |
| The store written in Eyelang, with a `Store` trait | n/a | ✓ | ✓ | [memory_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/memory_test.go#L13) |
| The journal store for one machine | n/a | n/a | ✓ | [store_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/store_test.go#L16) |
| The Postgres store: `eye run --store postgres://…`, one runner per database and schema | n/a | n/a | ✓ | [postgres_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/postgres_test.go), [runner_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/runner_test.go) |
| Runs survive a restart of the runner, and the database stopping or not answering, also before the runner first connects | n/a | n/a | ✓ | [durable_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/durable_test.go), [postgres_wait_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/postgres_wait_test.go), [runner_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/runner_test.go) |
| Jobs resume from saved frames, not by replaying their steps | n/a | n/a | ✓ in the interpreter: a checkpoint at each statement of the job's body; only the steps since replay, and a function the statement calls replays its steps | [checkpoint_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/checkpoint_test.go), [store_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/store_test.go) |
| A job changed under a paused run is caught, not replayed into wrong values | n/a | n/a | ✓ (`eye run` refuses to start; a resumed run, or a replayed step for another call, faults) | [checkpoint_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/checkpoint_test.go), [runner_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/runner_test.go) |
| A waiting run holds no goroutine, only its saved bytes | n/a | n/a | ✓ | [durable_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/durable_test.go) |
| `sleep!`, `sleepUntil!` | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L41), [lab notifier](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/notifier/notifier.eye#L242) |
| `poll!` | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L93) |
| `approval!`, with requests published as `approvals.requested` and answered by `approvals.answered`, `eye approve` or `eye decline` | ✓ | ✓ | ✓: an answer names its request's `id` and counts only when its `by` is the request's `from`; another is refused and logged, and the run keeps waiting. `eye approve TEXT` and test `approve(…)` refuse text that no waiting approval or several contain; `eye approve --id ID` answers one. A wait survives a restart of `eye run` with its id. Any module may still publish an answer (open question 35) | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/approvals.eye), [approvals_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/approvals_test.go), [lab approving](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/approving/approving.eye#L24), [approve_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/approve_test.go#L16) |
| `next!`, which misses no event | ✓ | ✓ | ✓ | [programs/events](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/events/events.eye#L45) |
| `reserve!` | ✓ | ✓ | ✓ | [programs/memory](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/memory/memory.eye#L42) |
| `lock!` | ✓ | ✓ | ✓ (the store keeps the holder and expiry; a holder still waiting inside when the lock runs out ends as `Failed(LockExpired(name))`; a held or awaited lock survives a restart) | [programs/waiting](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/waiting/waiting.eye#L111), [waits_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/waits_test.go#L201) |
| `run.join!()` and `Outcome` | ✓ | ✓ | ✓ | [programs/limits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/limits/limits.eye#L101) |
| `run.outcome()` in tests and `main`: `None` while the run is still going | ✓ | ✓ (`run.join()` without `!` is an error that names `join!()` in a job and `outcome()` elsewhere; `outcome()` in a job is an error that names `join!()`) | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L167), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L83) |
| `start()`, with `after:`, `at:` and `key:` | ✓ | ✓ | partial: `after:` and `key:` are tested; `at:` has no test yet | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L12), [programs/limits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/limits/limits.eye#L118) |

## Attributes

Every attribute is declared with where it goes and what it takes, and `eye check` holds each use to
that ([check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L60)). The
Runs column says whether the runtime does what the attribute says.

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| `@on(daily(…))`, `weekdays(…)`, `every(…)` | ✓ | ✓ | ✓ | [programs/schedules](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/schedules/schedules.eye#L29) |
| `@on(cron("…", in: zone))` | ✓ | ✓: a wrong cron line, or one that never fires, is an error at the string | ✓ | [programs/schedules](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/schedules/schedules.eye#L75), [cron_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/internal/cron/cron_test.go), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L66) |
| `@on(enters(view))` | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L54) |
| `@on(leaves(view))` | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/views.eye#L49) |
| `@on(event(contract.event))` | ✓ | ✓ | ✓ | [programs/events](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/events/events.eye#L37) |
| `@concurrency(n, by: param)` | ✓ | ✓ | ✓ | [programs/limits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/limits/limits.eye#L73) |
| `@limit(n, per: duration, by: param)` | ✓ | ✓ | ✓ | [programs/limits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/limits/limits.eye#L92) |
| `@deadline(duration)`, `@deadline(time, in: zone)` | ✓ | ✓ | ✓ | [programs/limits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/limits/limits.eye#L101) |
| `@memory(size)`, `@priority(p)` | ✓ | ✓ | ✓ (what a run holds is its saved state; open question 26); the runner has its cgroup's limit, or `--memory-limit` | [programs/memory](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/memory/memory.eye#L20), [cgroup_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/cgroup_test.go) |
| `@budget(usd:, tokens:)` | ✓ | ✓ | ✓ | [ai_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/ai_test.go) |
| `@version(N)` | ✓ | ✓ | ✓ | [migrations_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/migrations_test.go#L14) |
| `@key`, `@reconcile` | ✓ | ✓ | ✓ | [programs/tour](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tour/tour.eye#L44) |
| `@http`, `@auth` | ✓ | ✓ | ✓ | [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L24) |
| `@port`, `@allow` | ✓ | ✓: a served contract and one with `@allow` need `@port`; a call from a module `@allow` doesn't list, a name that isn't a module, and any call under `@allow(nobody)` are errors | ✓: `eye run` serves a served contract on its `@port` (`<MODULE>_PORT` moves it); `@allow` isn't enforced at the port until network policy (6.5) | [ports_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/ports_test.go#L41), [serve_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/serve_test.go) |
| `@fresh`, `@destructive` | ✓ | ✓ on commands; `@destructive` on a function adds `approval: Approval` | ✓ on commands, and on a model's tool | [fresh_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/fresh_test.go#L61), [programs/contracts](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/contracts/contracts.eye#L92) |
| `@unwitnessed("why")` | ✓ | partial: only where it goes | — | |
| `@library`, `@host` | ✓ | ✓: a program's extern fn needs exactly one, and a C function's types must cross to C | ✓ (see "Calling C and Go" below) | [host_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/host/host_test.go), [clib_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/host/clib/clib_test.go) |

## [Views and triggers](/reference/language/views-and-triggers)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| A view is a pipeline over an entity set, kept up to date | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L24), [read by name](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/views.eye#L15) |
| A view must start from an entity set | ✓ | ✓ | n/a | [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L110) |
| `enters(view)` and `leaves(view)`, each time an entity enters or leaves | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L54), [enters, leaves, enters again](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/views.eye#L61) |
| Pipelines over entities compiled to SQL (v0.2) | n/a | n/a | — | |

## [Contracts: talking to other services](/reference/language/contracts-talking-to-other-services)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| `contract` with commands, queries and events | ✓ | ✓ | ✓ | [tour notify](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tour/notify/notify.eye#L16) |
| `import` as a typed edge: a wrong port or caller is a compile error | ✓ | ✓ | n/a | [ports_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/ports_test.go#L41), [lab](https://github.com/ruxxzebre/eyelang/blob/dev/check/ports_test.go#L154) |
| Network policy from contracts: `eye gen policy` writes a Kubernetes `NetworkPolicy` per contract with `@port`, letting in the modules that call it and `@allow` lets | n/a | ✓ | ✓: pods by `eyelang.module`, namespace from `--namespace`; no egress rules | [netpolicy_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/internal/netpolicy/netpolicy_test.go), [the lab's policy](https://github.com/ruxxzebre/eyelang/blob/dev/internal/netpolicy/testdata/lab.yaml) |
| Commands can return `Uncertain`, which must be handled or reconciled | ✓ | ✓ | ✓ | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L127), [tour worlds](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tour/worlds.eye#L26) |
| A command's outcome, with or without `Uncertain`, `Stale` and `Expired`, can't be dropped by a statement: it is matched, used or discarded with `let _ = …` | ✓ | ✓ | n/a | [drop_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/drop_test.go#L46), [lab trains](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/trains/trains.eye#L83) |
| `.reconcile!()` through a query, resending only on `None` | ✓ | ✓ | ✓ | [programs/tour](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tour/tour.eye#L44), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L136) |
| `@reconcile(send)`: reconciling resends with the same key | ✓ | ✓ | ✓ | [lab digest](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/digest/digest.eye#L151) |
| A request never sent is retried, not `Uncertain` | n/a | n/a | ✓ | [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L289) |
| Queries retried, then `Failed(Unavailable(…))`; `Result(T, HttpError)` gets the failures | ✓ | ✓ | ✓ | [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L289), [lab web](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/web/web.eye#L16) |
| Events published on commit or as a job's step; `@key` starts one run per key | ✓ | ✓ | ✓ | [programs/events](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/events/events.eye#L37), [lab notifier](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/notifier/notifier.eye#L191) |
| `@fresh` and `@destructive` commands | ✓ | ✓ A `@fresh` command takes `basedOn: Version` and may be `Stale` or `Expired`, which a match must handle; a `@destructive` one takes an `Approval`; a served `impl` takes both and answers `Stale` | ✓ in calls, fakes and an `impl`; over HTTP `basedOn` and `approval` go as arguments, and 409 is `Stale`, 410 `Expired` | [fresh_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/fresh_test.go#L61), [programs/contracts](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/contracts/contracts.eye#L92), [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L382) |
| A contract's operations are its module's functions | ✓ | ✓ | ✓ | [programs/tour](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tour/tour.eye#L44) |
| Serving a contract with `impl Contract { … }` | ✓ | ✓ | ✓ | [lab notifier](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/notifier/notifier.eye#L233), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L297) |
| Values as JSON, one shape everywhere | n/a | n/a | ✓ | [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L24) |
| Over HTTP: paths, query parameters, key header, `status:` | ✓ | ✓ | ✓ | [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L24) |
| `eye run` serves a contract over HTTP on its `@port`: routed by `@http`, the one JSON shape, `@auth` checked (401), one transaction per request, `status:` for the reply | n/a | n/a | ✓ | [listen_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/listen_test.go), [serve_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/serve_test.go) (the lab's notifier serving Notify to a second program) |
| A service's URL from the environment; a served contract calls itself | n/a | n/a | ✓ | [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L24), [lab notifier](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/notifier/notifier.eye#L233) |
| Fakes from recordings, checked against the contract | ✓ | partial: reading a field no recording contains isn't an error | ✓ | [programs/tour](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tour/tour.eye#L57) |
| `http.get!` for services without a contract | ✓ | ✓ | ✓ | [lab web](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/web/web.eye#L30), [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L24) |
| Generated types, clients and stubs for other languages: `eye gen go` and `eye gen ts` write each contract module's types in the one JSON shape, a client over `@http` and a server stub that routes as `eye run` does | n/a | ✓: tuples, maps without string keys, unnamed records and `extern type`s are refused | partial: Go and TypeScript; no Rust or Python; operations without `@http` get no method; the TypeScript stub checks that arguments are there, not their shapes | [codegen_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/internal/codegen/codegen_test.go) (golden [Notify](https://github.com/ruxxzebre/eyelang/blob/dev/internal/codegen/testdata/notify.ts.golden) and Miniflux, `go vet`, `tsc --strict`), [gen_roundtrip_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/gen_roundtrip_test.go) (every type of the lab's contracts through the generated clients and stubs to `eye run` and back) |

## [Calling C and Go](/reference/language/calling-c-and-go)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| `extern fn` with a signature and no body | ✓ | ✓ (calling a program's own adds `ffi` unless it states its uses; a program's own needs exactly one of `@library` and `@host`, and a C function's parameter or result that can't cross to C is an error at its type) | ✓: the standard library's own (`ai.generate`, `http.get`), a program's `@host` and `@library` ones, and in a job each is a recorded step | [std/http.eye](https://github.com/ruxxzebre/eyelang/blob/dev/check/std/http.eye), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go) |
| `@library`: C functions | ✓ | ✓ (no `Int`, no `Bytes` result; `Option` of an `extern type` for a pointer that may be NULL) | ✓: loaded at run time without cgo (`internal/ffi`, on purego); `Bool`, `Byte`, the fixed-width integers, `Float`, `String` and `Bytes` copied, `extern type` values as pointers; a missing library or symbol is reported before `main`, NULL without `Option` is a fault. The released `eye` for Linux is static and has no loader: build with `-tags clib` | [clib_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/host/clib/clib_test.go) (zlib's `crc32` and gzip functions) |
| `@host`: functions from a Go program that embeds Eyelang | ✓ | ✓ (its types are checked by `Register`, against the Go function) | ✓: the embedding API, package `eyelang/host` (`Load`, `Register`, `Run`, `Test`); `Register` checks the Go function's type against the declaration; a host function nothing registered is reported before `main`; a panic is a fault | [host_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/host/host_test.go) |

## [AI models and agents](/reference/language/ai-models-and-agents)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| `ai.generate!`, with the schema taken from the type expected | ✓ | ✓ | ✓ | [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L243), [maps, tuples, times](https://github.com/ruxxzebre/eyelang/blob/dev/interp/ai_test.go#L381), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L376) |
| Only types with `Schema` can be asked for | ✓ | ✓ | n/a | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L249) |
| A reply is checked, asked for once more, then `Err(Malformed(…))` | n/a | n/a | ✓ | [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L243) |
| Providers: Anthropic, OpenAI and a gateway, by `provider:` or by the model's name | ✓ | ✓ | ✓ | [ai_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/ai_test.go) |
| In a job, a model call is a recorded step | ✓ | ✓ | ✓ | [lab digest](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/digest/digest.eye#L128) |
| Tests replay recordings with `fakes(ai, …)` | ✓ | ✓ | ✓ | [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L272) |
| `eye test --live` and `--record`: the named modules' calls go to the real services, a test that can't reach one is skipped, and `--record` writes their answers, without secrets, as the recordings the fakes read | n/a | n/a | ✓ | [live_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/live_test.go), [cmd/eye live_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/live_test.go) |
| `@budget`: tokens and dollars, counted again on replay, the call that would go over not sent | ✓ | ✓ | ✓ | [ai_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/ai_test.go) |
| Tools: functions passed as `tools:`, each converted to an `ai.Tool`, with its schema from its parameters and its description from its `##` comment | ✓ | ✓ (parameters and result need `Schema`; a lambda, an `extern fn` or a generic function is refused) | ✓ for Anthropic and chat completions, and from recordings | [tools_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/tools_test.go#L46), [interp tools_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/tools_test.go#L45), [programs/tools](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tools/tools.eye#L46) |
| A tool's effects count where it is named; a model call's own effect is `net Ai.generate` | ✓ | ✓ | n/a | [tools_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/tools_test.go#L46) |
| In a job, each model reply and each tool's result is a step, so a restarted run runs no tool twice | ✓ | ✓ | ✓ | [runner_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/runner_test.go#L332) |
| `@destructive` tools: each call waits for `approve:`, usually `approval!` | ✓ | ✓ (a call whose `tools:` names one gives `approve:`) | ✓ | [programs/tools](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tools/tools.eye#L46) |
| Coding agents: the `agents` module | ✓ | — (no such module) | — | |

## [Time, secrets, tests](/reference/language/time-secrets-tests)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| Schedules in a zone; skipped and repeated times at daylight-saving changes | ✓ | ✓ | ✓ | [programs/schedules](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/schedules/schedules.eye#L42) |
| `onGap: Shift \| Skip` and `onOverlap: First \| Second \| Both` | ✓ | ✓ | ✓ | [programs/schedules](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/schedules/schedules.eye#L96) |
| A missed firing runs once on restart | n/a | n/a | ✓ | [store_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/store_test.go#L355), [a cron across restarts](https://github.com/ruxxzebre/eyelang/blob/dev/interp/store_test.go#L391) |
| `at.date(in:)`, `at.time(in:)`, `time.next(…)`, `day.at(…)`, `day.format(…)`, `at.unix()` | ✓ | ✓ | ✓ | [lab digest](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/digest/digest.eye#L344) |
| `secret(name)` from the environment or `$EYE_SECRETS` | ✓ | ✓ | partial: the environment is tested; `$EYE_SECRETS` works but has no test | [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L24) |
| `env.get(name)` | ✓ | ✓ | ✓ | [lab web](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/web/web.eye#L16) |
| `test` blocks, including those of imported modules | ✓ | ✓ | ✓ | [interp_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/interp_test.go#L44) |
| `with clock(…)`, `advance(…)`, `with fakes(…)` | ✓ | ✓ | ✓ | [programs/schedules](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/schedules/schedules.eye#L35), [lab trains](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/trains/trains.eye#L200) |
| `let sent = fakes(…)`, `with sent`, and `sent.calls(op)`: the calls a fake answered, as records of their arguments and key | ✓ | ✓ | ✓ | [programs/events](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/events/events.eye#L54), [lab digest](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/digest/digest.eye) |
| `with memory(…)` | ✓ | — (undefined) | — | |
| `with` only in tests | ✓ | ✓ | n/a | [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L108) |
| Published events start and wake runs in tests; `approve` and `decline` | ✓ | ✓ | ✓ | [lab access](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/access/access.eye#L180) |
| `expect` shows the value it got | ✓ | ✓ | ✓ | [main_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/main_test.go#L212) |
| A run that stops on a bug fails its test; a declared outcome doesn't | n/a | n/a | ✓ | [programs/limits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/limits/limits.eye#L101), [interp_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/interp_test.go#L153) |

## Tooling

| Tool | Status | Shown by |
|---|---|---|
| `eye fmt` | ✓ One style with no options; `--check`; reads standard input; refuses files with syntax errors; a program's directory gets each public function's `uses` clause written from its body (`--syntax-only` skips it) | [format_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/syntax/format_test.go#L72), [main_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/main_test.go#L96) |
| `eye parse` | ✓ The syntax tree, or an S-expression with `--sexp` | [main_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/main_test.go#L58) |
| `eye check` | ✓ Syntax, names, types, matches, effects and attributes; `--syntax` for syntax alone | [main_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/main_test.go#L193), [classes_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/classes_test.go) |
| `eye test` | ✓ `--run` to filter, `-v` to show logs, `--live` to call the real services of the modules named, `--record` to write what they answered as recordings | [main_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/main_test.go#L212) |
| `eye run` and `eye runs` | ✓ The journal in `.eye/journal`, a Postgres database with `--store` or `$EYE_STORE`, or `--memory`; survives a kill; stops on Ctrl-C; starts with a line naming the program, store, memory and jobs, and a line per contract it serves and the port | [runner_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/runner_test.go#L40), [startup_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/startup_test.go) |
| `eye run --fresh` | ✓ Starts over with an empty journal, kept as `JOURNAL.old`; refused with a database or `--memory`; a run that has nothing to do says so | [runner_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/runner_test.go) |
| `eye doc` | ✓ The API reference in Markdown from `##` comments; fails on a link that names nothing; writes to `./api` or `--out`, and replaces that folder only if it is missing, empty or its own output | [apidoc_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/internal/apidoc/apidoc_test.go#L12), [main_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/main_test.go#L427) |
| `eye lsp`: diagnostics, formatting, outline, folding, semantic tokens | ✓ | [server_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/internal/lsp/server_test.go#L136) |
| `eye lsp`: go to definition and find references, also from the command line | ✓ Into modules, the prelude, the standard library and the Go behind built-ins | [definition_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/internal/lsp/definition_test.go#L15), [main_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/main_test.go#L272) |
| `eye lsp`: hover | ✓ A name's signature and its doc comment | [hover_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/internal/lsp/hover_test.go#L13) |
| `eye lsp`: completion, and the inferred effects of private functions | Not started | |
| `eye repl` | ✓ Expressions with their types, bindings, declarations, jobs on a clock that `:advance` moves | [repl_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/repl/repl_test.go#L113) |
| VS Code extension | ✓ TextMate highlighting and the `eye lsp` client | [editors/vscode/test](https://github.com/ruxxzebre/eyelang/tree/dev/editors/vscode/test) |
| `eye build` | ✓ for the core language: C11 from the low IR with `#line` directives, compiled by the pinned zig (`-O2`, or `-O0 -g` with `-g`) for this machine or, with `--target`, any of the five release targets; `--test` makes a binary that runs the test blocks as `eye test` does; object files cached by content hash; `--emit-c` writes the C. A program with entities, jobs, views or contracts gets an error that says they aren't compiled yet | [cgen_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cgen/cgen_test.go) |
| Native compilation with reference counting (v0.4) | partial: the core language compiles, and every test of `testdata/programs/core` passes natively on linux/amd64; counts are simple (precise counting and reuse are 7.5), and the runner doesn't host compiled code yet (7.6) | [cgen_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cgen/cgen_test.go) |
