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
- **Checks**: `eye check` resolves and type-checks it, or reports a specific error when it is misused.
- **Runs**: the reference interpreter runs it, and a passing test shows that.

A cell holds ✓ (done), "partial" with what is missing, "—" (not built), or "n/a" (the column
doesn't apply). A feature with "—" under Checks and Runs is **spec only**: it parses, and then it is
rejected or ignored. "Shown by" links to a test or program that shows the feature working. For
compile-time errors it links to the checker's test cases. Every error class the spec promises is
listed in [check/classes_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/classes_test.go),
along with the test that reports it.

**The biggest gaps today.** Some waits (`lock!`, `all!`, `first!`, ending an entity), coding
agents and model tools, the checks that come from contracts (`@port`, `@allow`, `@fresh`, `@destructive`,
`@unwitnessed`), `fs` and `process`, fixed-width integers, and native compilation (v0.4).

## [A tour](/reference/language/a-tour)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| The tour: an entity, a view, a job that waits, a contract with fakes, three tests | ✓ | ✓ | ✓ | [programs/tour](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tour/tour.eye#L56) |

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
| `NonZero(Int)` divisors and `nonzero(n)` | ✓ | partial: `NonZero` is treated as its number type, so `a / b` checks for any `Int` | partial: dividing by zero ends the run as a fault | [interp_test.go: faults](https://github.com/ruxxzebre/eyelang/blob/dev/interp/interp_test.go#L153) |
| `Float` | ✓ | ✓ | ✓ | [core: match](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L98) |
| `Byte`, `Bytes`, `Int32`, `Int64`, `UInt32`, `UInt64` | ✓ | partial: the types are known, but `Int32.from`, `wrappingAdd` and the bit operations don't exist, and a char literal isn't read as a `Byte` | — | [lexer.eye](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/syntax/valid/lexer.eye) |
| `Instant`, `Date`, `Time`, `Duration`, `Zone`, `Schedule`, `Size` | ✓ | ✓ | ✓ | [core: time](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L185) |
| `List`, `Map`, `Set` as values | ✓ | ✓ | ✓ | [core: collections](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L156) |
| Tuples, and lambdas that take a tuple's parts | ✓ | ✓ | ✓ | [core: closures](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L147) |
| Records, `with` updates, field assignment | ✓ | ✓ | ✓ | [core: records](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L74) |
| Nominal records; a literal takes the type expected | ✓ | ✓ | ✓ | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L79) |
| Variants, one-variant types, aliases | ✓ | ✓ | ✓ | [core](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L3) |
| `Option` and `Result` | ✓ | ✓ | ✓ | [core: errors](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L124) |
| Type variables | ✓ | ✓ (one can't be called or indexed; a requirement on an untyped parameter waits for its calls) | ✓ | [programs/traits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/traits/traits.eye#L33), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L261) |
| Ranges `..` and `..=`, as loops, patterns and values; time ranges that wrap midnight | ✓ | ✓ | ✓ | [core: mutation](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L94), [lab notifier: quiet hours](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/notifier/notifier.eye#L236) |
| Slices `xs[a..b]`; `s.slice(range)` on strings, by `Offset`, with `indexOf`, `start`, `end`, `after`, `before` and `charAt`; an `Int` there is an error that says where offsets come from | ✓ | ✓ | ✓ | [core: collections](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L166), [core: strings](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/strings.eye#L3), [offset_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/offset_test.go) |
| Iteration with `for`; `s.chars()`; lazy `xs.iter()` | ✓ | partial: `iter` isn't built, so the checker reports it as a method that doesn't exist | partial: lists, sets, ranges, maps and `s.chars()` work; `iter` and `Iter` don't exist | [core: mutation](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L94), [core: strings](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/strings.eye#L49) |
| `Ref(E)`, different for each entity | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L35) |
| `Secret(a)`: no printing, no saving | ✓ | ✓ | ✓ | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L180) |
| `Approval`, made only by `approval!` | ✓ | partial: nothing requires one yet | ✓ | [lab access](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/access/access.eye#L186) |
| `Key`, `Run(a)` | ✓ | ✓ | ✓ | [programs/limits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/limits/limits.eye#L101) |
| A program's `extern type`, whose values foreign code owns: not saved, shown or compared (`Handle` is gone, open question 21) | ✓ | ✓ | — (no foreign code yet) | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L285) |
| `opaque` types | ✓ | ✓ (other modules can't read, write, build or match its fields or variants) | ✓ as an ordinary type | [visibility_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/visibility_test.go#L35), [spec-08 example](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/syntax/valid/spec-08-methods-and-generics-1.eye) |
| String literals as `Key` or `Zone` where one is expected | ✓ | ✓ | ✓ | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L145) |
| Map literals, `(key):` keys, `{ name, age }` shorthand, `{}` | ✓ | ✓ | ✓ | [core: collections](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L156) |

## [Bindings, functions and pipes](/reference/language/bindings-functions-and-pipes)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| Public functions with full signatures; private ones inferred | ✓ | partial: a private function's inferred type is one type for the whole program, not generic | ✓ | [core](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L30) |
| `let`, `var`, shadowing | ✓ | ✓ | ✓ | [core: mutation](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L83) |
| Lambdas: one expression, a block, several parameters, none | ✓ | ✓ | ✓ | [core: closures](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L147) |
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
| Updates in place when a value is unique | n/a | n/a | partial: values behave as copies, but the interpreter has no reference counts and copies them (in place comes with v0.4) | [core: mutation](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L89) |

## [Methods and generics](/reference/language/methods-and-generics)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| `impl Type { … }` methods, and functions called on the type (`Lexer.new`) | ✓ | ✓ | partial: methods are tested; `Type.f()` has no test, and fails through a module (`lex.Lexer.new`) | [core](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L18) |
| An `impl Type` only in the module that declares the type | ✓ | ✓ | n/a | [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L117) |
| Generic functions and types with type variables | ✓ | ✓ | ✓ | [programs/traits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/traits/traits.eye#L33) |
| `where` requirements, and a trait as a parameter's type | ✓ | ✓ | ✓ | [programs/traits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/traits/traits.eye#L27), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L225) |
| Generic code specialised at compile time | n/a | n/a | — (v0.4) | |
| `dyn Trait` (v0.2) | ✓ | ✓ | ✓ | [programs/traits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/traits/traits.eye#L99), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L188) |

## [Pattern matching](/reference/language/pattern-matching)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| Variants with positional or named payloads, guards, alternatives, `..` | ✓ | ✓ | ✓ | [core: match](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L98) |
| Literals, negative numbers, characters, ranges | ✓ | ✓ | ✓ | [core: match](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L117) |
| Record and tuple patterns; destructuring `let` | ✓ | ✓ | ✓ | [patterns.eye](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/syntax/valid/patterns.eye), [core: closures](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L152) |
| List patterns `[first, ..rest]` and `let … else` | ✓ | ✓ | ✓ | [core: match](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L109), [core: errors](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L130) |
| List patterns over `Bytes` with character literals | ✓ | partial: `'-'` isn't read as a `Byte`, so the pattern is an error | — | |
| Variants of imported modules without a prefix; an error when two modules declare one | ✓ | ✓ | ✓ | [programs/tour](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tour/tour.eye#L44), [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L171) |
| Exhaustive matches, including nested patterns and list lengths | ✓ | ✓ | n/a | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L103) |

## [Errors: Result only](/reference/language/errors-result-only)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| `?` on `Result` and `Option` | ✓ | ✓ | ✓ | [core: errors](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L124) |
| `?` wraps the error in the one variant that holds it | ✓ | ✓ | ✓ | [core: errors](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L127), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L96) |
| `.mapErr(…)` | ✓ | ✓ | ✓ | [lab digest](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/digest/digest.eye#L120) |
| `Problem` and `.context(…)` | ✓ | ✓ | ✓ | [core: problems](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/problems.eye#L16), [problem_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/problem_test.go) |
| Total library functions: `xs[i]` and `first` give `Option`, `parseInt` and `parseFloat` give `Result`, `toInt` gives `Option` | ✓ | ✓ | ✓ | [core: collections](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/values.eye#L165), [core: numbers](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/strings.eye#L60) |
| String functions: `replace`, `words`, `endsWith`, `chars`, `code`, `fromCode`; Float `round`, `floor`, `ceil`, `toFloat` | ✓ | ✓ | ✓ | [core: strings](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/core/strings.eye#L39) |
| `unreachable("why")` ends the program or run as a fault | ✓ | ✓ | ✓ | [interp_test.go: faults](https://github.com/ruxxzebre/eyelang/blob/dev/interp/interp_test.go#L153) |
| Runtime faults as a run's outcome | ✓ | ✓ | partial: `Unreachable`, `DeadlinePassed`, `Unavailable`, `OverBudget` and `OutOfMemory` happen; `Killed` never does | [programs/limits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/limits/limits.eye#L101) |

## [Traits](/reference/language/traits)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| Trait declarations, with default methods | ✓ | ✓ (a default method sees self as any type with the trait) | ✓ | [programs/traits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/traits/traits.eye#L64), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L418) |
| `impl Trait for Type`, every method and no others, with the trait's types | ✓ | ✓ | ✓ | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L192) |
| Coherence: an impl lives with its trait or its type, once per type | ✓ | ✓ | n/a | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L208) |
| A one-variant wrapper shows as the value it wraps | ✓ | ✓ | ✓ | [programs/tour](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tour/tour.eye#L44) |
| Derived `Eq`, `Ord`, `Show`, `Hash`, `Save`, `Schema`, asked for where they are used | ✓ | ✓ | ✓ | [programs/traits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/traits/traits.eye#L88), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L244) |
| An impl of `Show` replaces the derived form | ✓ | ✓ | ✓ | [programs/traits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/traits/traits.eye#L83) |
| Operators as traits (`Add`, `Mul`, `Neg` for `-x`, …); no implicit conversions | ✓ | ✓ | partial: `-x` runs on numbers and durations, not through a declared type's impl of `Neg` | [programs/traits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/traits/traits.eye#L74), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L285) |

## [Effects and signatures](/reference/language/effects-and-signatures)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| `uses` on public functions; none means pure; a private function's uses clause and a trait method's are held too; effect names are checked | ✓ | ✓ | n/a | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L171) |
| Effects inferred through private functions and callbacks, and functions passed by name | ✓ | ✓ | n/a | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L261) |
| `wait`, `net C.op`, `entity E`, `clock`, `stdio`, `env` | ✓ | ✓ | n/a | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L171) |
| `secret S`, `publish T`, `consume T`, `ffi` | ✓ | — (not inferred; open question 19) | n/a | |
| `fs`, `process`, `random` | ✓ | — | — (no `fs` or `process` module, no `random()`) | |
| Function types in signatures: `f: fn(Int) -> Int` | ✓ | ✓ (a function has none of the derived traits, so it can't be shown, compared or saved) | ✓ | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L406) |
| Function types kept in data state their effects | ✓ | partial: a function kept in a field can be called, but its effects aren't checked | ✓ | |
| In a job, every call with an outside effect, and every entity call, takes `!` | ✓ | ✓ (and `!` on a call that only computes is an error; printing and logging aren't steps, open question 22) | ✓ | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L158), [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L106) |

## [Programs and concurrency](/reference/language/programs-and-concurrency)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| `pub fn main`, returning `()` or `Result`; an `Err` exits with status 1 | ✓ | ✓ | ✓ | [main_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/main_test.go#L238) |
| `eye run` keeps jobs going across restarts, in `.eye/journal` | ✓ | n/a | ✓ | [runner_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/runner_test.go#L68) |
| A job started from `main` needs a key | ✓ | ✓ | ✓ | [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L95), [runner_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/runner_test.go#L75) |
| `eye runs` lists runs, what they wait for, and their steps | n/a | n/a | ✓ | [runner_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/runner_test.go#L135) |
| `eye repl` | n/a | ✓ | ✓ | [repl_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/repl/repl_test.go#L41) |
| Modules as directories; `import a.b` and `import a.b.{Name}`; other modules use only what is `pub` | ✓ | ✓ | ✓ | [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L150), [visibility_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/visibility_test.go#L35), [lab](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/lab.eye) |
| `eye.toml` marks a program's root | n/a | ✓ | ✓ | [index_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/index_test.go#L151) |
| `parallelMap`, `both`, `race`, `timeout`, with waits that overlap | ✓ | partial: what a task captures isn't checked | ✓ | [programs/tasks](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tasks/tasks.eye#L18), [tasks_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/tasks_test.go#L16) |
| A task fails, and the call fails with it | ✓ | n/a | ✓ | [tasks_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/tasks_test.go#L70) |
| A task a job starts (`parallelMap`, `both`, `race`, `timeout`, `taskGroup`) only computes | ✓ | ✓ | ✓ (the run fails, should one get past the checker) | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L232) |
| Cheap tasks with small, growing stacks | n/a | n/a | partial: Go goroutines for now | |
| Task groups, `spawn` and channels (v0.2) | ✓ | ✓ | ✓ | [programs/tasks](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tasks/tasks.eye#L77), [channels_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/channels_test.go#L116) |
| A failing task cancels its group | ✓ | n/a | ✓ | [channels_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/channels_test.go#L31) |
| A group whose tasks all wait for channels nobody can change fails | ✓ | n/a | partial: on the system clock, a group inside another task isn't checked, and waits | [channels_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/channels_test.go#L61) |
| A channel can't be live across a `!` | ✓ | ✓ | n/a | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L224) |
| A task group in a job only computes, in its spawned tasks too | ✓ | ✓ | ✓ (the run fails) | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L200) |
| Child runs joined with `all!` and `first!` | ✓ | — (resolved as the collection functions `all` and `first`) | — | |

## [Entities](/reference/language/entities)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| `entity` with `state` and defaults; exists on first use | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L5) |
| Methods as atomic transactions; nested entity calls commit together | ✓ | ✓ | partial: methods run as transactions, but no test checks a rollback after a fault | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L41) |
| `state` fields and methods named without `self` | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L9) |
| Methods never wait | ✓ | ✓ | n/a | [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L49) |
| A method starts a job when its transaction commits | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L12) |
| The identity can't change | ✓ | ✓ | n/a | [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L101) |
| `read!()` snapshots with a `version`; `read()` outside jobs | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L36), [programs/tour](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tour/tour.eye#L44) |
| Ending an entity with `end(ok)` and an `Approval` | ✓ | — (`end` is undefined) | — | |
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
| Runs survive a restart of the runner, and the database stopping or not answering | n/a | n/a | ✓ | [durable_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/durable_test.go), [runner_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/runner_test.go) |
| A waiting run holds no goroutine, only its saved bytes | n/a | n/a | ✓ | [durable_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/durable_test.go) || `sleep!`, `sleepUntil!` | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L41), [lab notifier](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/notifier/notifier.eye#L236) |
| `poll!` | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L93) |
| `approval!`, with requests published as `approvals.requested` and answered by `approvals.answered`, `eye approve` or `eye decline` | ✓ | ✓ | ✓ | [lab approving](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/approving/approving.eye#L14), [approve_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/approve_test.go#L16) |
| `next!`, which misses no event | ✓ | ✓ | ✓ | [programs/events](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/events/events.eye#L45) |
| `reserve!` | ✓ | ✓ | ✓ | [programs/memory](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/memory/memory.eye#L42) |
| `lock!` | ✓ | — (undefined) | — | |
| `run.join!()` and `Outcome` | ✓ | ✓ | ✓ | [programs/limits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/limits/limits.eye#L101) |
| `start()`, with `after:`, `at:` and `key:` | ✓ | ✓ | partial: `after:` and `key:` are tested; `at:` has no test yet | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L12), [programs/limits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/limits/limits.eye#L118) |

## Attributes

Every attribute is declared with where it goes and what it takes, and `eye check` holds each use to
that ([check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L60)). The
Runs column says whether the runtime does what the attribute says.

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| `@on(daily(…))`, `weekdays(…)`, `every(…)` | ✓ | ✓ | ✓ | [programs/schedules](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/schedules/schedules.eye#L29) |
| `@on(cron("…", in: zone))` | ✓ | — (rejected: not a schedule) | — | |
| `@on(enters(view))` | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L54) |
| `@on(leaves(view))` | ✓ | ✓ | partial: built, but no test yet | |
| `@on(event(contract.event))` | ✓ | ✓ | ✓ | [programs/events](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/events/events.eye#L37) |
| `@concurrency(n, by: param)` | ✓ | ✓ | ✓ | [programs/limits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/limits/limits.eye#L73) |
| `@limit(n, per: duration, by: param)` | ✓ | ✓ | ✓ | [programs/limits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/limits/limits.eye#L92) |
| `@deadline(duration)`, `@deadline(time, in: zone)` | ✓ | ✓ | ✓ | [programs/limits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/limits/limits.eye#L101) |
| `@memory(size)`, `@priority(p)` | ✓ | ✓ | ✓ (what a run holds is its saved state; open question 26) | [programs/memory](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/memory/memory.eye#L20) |
| `@budget(usd:, tokens:)` | ✓ | ✓ | ✓ | [ai_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/ai_test.go) |
| `@version(N)` | ✓ | ✓ | ✓ | [migrations_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/migrations_test.go#L14) |
| `@key`, `@reconcile` | ✓ | ✓ | ✓ | [programs/tour](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tour/tour.eye#L44) |
| `@http`, `@auth` | ✓ | ✓ | ✓ | [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L24) |
| `@port`, `@allow` | ✓ | partial: only where they go and what they take | — | |
| `@fresh`, `@destructive` | ✓ | partial: only where they go; no `basedOn:` or `Approval` argument is added | — | |
| `@unwitnessed("why")` | ✓ | partial: only where it goes | — | |
| `@library`, `@host` | ✓ | ✓ | partial: only the standard library's host functions exist | [lab web](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/web/web.eye#L30) |

## [Views and triggers](/reference/language/views-and-triggers)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| A view is a pipeline over an entity set, kept up to date | ✓ | ✓ | ✓ | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L24) |
| A view must start from an entity set | ✓ | ✓ | n/a | [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L110) |
| `enters(view)` and `leaves(view)` | ✓ | ✓ | partial: `leaves` has no test yet | [programs/jobs](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/jobs/jobs.eye#L54) |
| Pipelines over entities compiled to SQL (v0.2) | n/a | n/a | — | |

## [Contracts: talking to other services](/reference/language/contracts-talking-to-other-services)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| `contract` with commands, queries and events | ✓ | ✓ | ✓ | [tour notify](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tour/notify/notify.eye#L16) |
| `import` as a typed edge: a wrong port or caller is a compile error | ✓ | — | — | |
| Commands can return `Uncertain`, which must be handled or reconciled | ✓ | ✓ | ✓ | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L127), [tour worlds](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tour/worlds.eye#L26) |
| `.reconcile!()` through a query, resending only on `None` | ✓ | ✓ | ✓ | [programs/tour](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tour/tour.eye#L44), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L136) |
| `@reconcile(send)`: reconciling resends with the same key | ✓ | ✓ | ✓ | [lab digest](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/digest/digest.eye#L144) |
| A request never sent is retried, not `Uncertain` | n/a | n/a | ✓ | [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L289) |
| Queries retried, then `Failed(Unavailable(…))`; `Result(T, HttpError)` gets the failures | ✓ | ✓ | ✓ | [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L289), [lab web](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/web/web.eye#L16) |
| Events published on commit or as a job's step; `@key` starts one run per key | ✓ | ✓ | ✓ | [programs/events](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/events/events.eye#L37), [lab notifier](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/notifier/notifier.eye#L190) |
| `@fresh` and `@destructive` commands | ✓ | — | — | |
| A contract's operations are its module's functions | ✓ | ✓ | ✓ | [programs/tour](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tour/tour.eye#L44) |
| Serving a contract with `impl Contract { … }` | ✓ | ✓ | ✓ | [lab notifier](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/notifier/notifier.eye#L228), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L297) |
| Values as JSON, one shape everywhere | n/a | n/a | ✓ | [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L24) |
| Over HTTP: paths, query parameters, key header, `status:` | ✓ | ✓ | ✓ | [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L24) |
| A service's URL from the environment; a served contract calls itself | n/a | n/a | ✓ | [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L24), [lab notifier](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/notifier/notifier.eye#L228) |
| Fakes from recordings, checked against the contract | ✓ | partial: reading a field no recording contains isn't an error | ✓ | [programs/tour](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/tour/tour.eye#L56) |
| `http.get!` for services without a contract | ✓ | ✓ | ✓ | [lab web](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/web/web.eye#L30), [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L24) |
| Generated types, clients and stubs for other languages | n/a | n/a | — | |

## [Calling C and Go](/reference/language/calling-c-and-go)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| `extern fn` with a signature and no body | ✓ | partial: calling one doesn't add `ffi` | partial: only the standard library's own (`ai.generate`, `http.get`) | [std/http.eye](https://github.com/ruxxzebre/eyelang/blob/dev/check/std/http.eye) |
| `@library`: C functions | ✓ | ✓ | — | |
| `@host`: functions from a Go program that embeds Eyelang | ✓ | ✓ | — (no embedding API yet) | |

## [AI models and agents](/reference/language/ai-models-and-agents)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| `ai.generate!`, with the schema taken from the type expected | ✓ | ✓ | ✓ | [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L243), [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L376) |
| Only types with `Schema` can be asked for | ✓ | ✓ | n/a | [types_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/types_test.go#L249) |
| A reply is checked, asked for once more, then `Err(Malformed(…))` | n/a | n/a | ✓ | [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L243) |
| Providers: Anthropic, OpenAI and a gateway, by `provider:` or by the model's name | ✓ | ✓ | ✓ | [ai_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/ai_test.go) |
| In a job, a model call is a recorded step | ✓ | ✓ | ✓ | [lab digest](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/digest/digest.eye#L116) |
| Tests replay recordings with `fakes(ai, …)` | ✓ | ✓ | ✓ | [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L272) |
| `eye test --live` | n/a | n/a | — | |
| `@budget`: tokens and dollars, counted again on replay, the call that would go over not sent | ✓ | ✓ | ✓ | [ai_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/ai_test.go) |
| Tools: functions passed as `tools:`, `@destructive` tools | ✓ | — (`generate` has no `tools:`) | — | |
| Coding agents: the `agents` module | ✓ | — (no such module) | — | |

## [Time, secrets, tests](/reference/language/time-secrets-tests)

| Feature | Parses | Checks | Runs | Shown by |
|---|---|---|---|---|
| Schedules in a zone; skipped and repeated times at daylight-saving changes | ✓ | ✓ | ✓ | [programs/schedules](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/schedules/schedules.eye#L42) |
| `onGap:` and `onOverlap:` policies | n/a | — | — | |
| A missed firing runs once on restart | n/a | n/a | ✓ | [store_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/store_test.go#L269) |
| `at.date(in:)`, `at.time(in:)`, `time.next(…)`, `day.at(…)`, `day.format(…)`, `at.unix()` | ✓ | ✓ | ✓ | [lab digest](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/digest/digest.eye#L337) |
| `secret(name)` from the environment or `$EYE_SECRETS` | ✓ | ✓ | partial: the environment is tested; `$EYE_SECRETS` works but has no test | [services_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/services_test.go#L24) |
| `env.get(name)` | ✓ | ✓ | ✓ | [lab web](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/web/web.eye#L16) |
| `test` blocks, including those of imported modules | ✓ | ✓ | ✓ | [interp_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/interp_test.go#L44) |
| `with clock(…)`, `advance(…)`, `with fakes(…)` | ✓ | ✓ | ✓ | [programs/schedules](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/schedules/schedules.eye#L35), [lab trains](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/trains/trains.eye#L198) |
| `with memory(…)` | ✓ | — (undefined) | — | |
| `with` only in tests | ✓ | ✓ | n/a | [check_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/check_test.go#L108) |
| Published events start and wake runs in tests; `approve` and `decline` | ✓ | ✓ | ✓ | [lab access](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/access/access.eye#L178) |
| `expect` shows the value it got | ✓ | ✓ | ✓ | [main_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/main_test.go#L212) |
| A run that stops on a bug fails its test; a declared outcome doesn't | n/a | n/a | ✓ | [programs/limits](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/programs/limits/limits.eye#L101), [interp_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/interp/interp_test.go#L153) |

## Tooling

| Tool | Status | Shown by |
|---|---|---|
| `eye fmt` | ✓ One style with no options; `--check`; reads standard input; refuses files with syntax errors | [format_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/syntax/format_test.go#L72), [main_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/main_test.go#L96) |
| `eye parse` | ✓ The syntax tree, or an S-expression with `--sexp` | [main_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/main_test.go#L58) |
| `eye check` | ✓ Syntax, names, types, matches, effects and attributes; `--syntax` for syntax alone | [main_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/main_test.go#L193), [classes_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/check/classes_test.go) |
| `eye test` | ✓ `--run` to filter, `-v` to show logs. No `--live` yet | [main_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/main_test.go#L212) |
| `eye run` and `eye runs` | ✓ The journal in `.eye/journal`, a Postgres database with `--store` or `$EYE_STORE`, or `--memory`; survives a kill; stops on Ctrl-C | [runner_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/runner_test.go#L40) |
| `eye doc` | ✓ The API reference in Markdown from `##` comments; fails on a link that names nothing | [apidoc_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/internal/apidoc/apidoc_test.go#L12) |
| `eye lsp`: diagnostics, formatting, outline, folding, semantic tokens | ✓ | [server_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/internal/lsp/server_test.go#L136) |
| `eye lsp`: go to definition and find references, also from the command line | ✓ Into modules, the prelude, the standard library and the Go behind built-ins | [definition_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/internal/lsp/definition_test.go#L15), [main_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/cmd/eye/main_test.go#L272) |
| `eye lsp`: hover | ✓ A name's signature and its doc comment | [hover_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/internal/lsp/hover_test.go#L13) |
| `eye lsp`: completion, and the inferred effects of private functions | Not started | |
| `eye repl` | ✓ Expressions with their types, bindings, declarations, jobs on a clock that `:advance` moves | [repl_test.go](https://github.com/ruxxzebre/eyelang/blob/dev/repl/repl_test.go#L113) |
| VS Code extension | ✓ TextMate highlighting and the `eye lsp` client | [editors/vscode/test](https://github.com/ruxxzebre/eyelang/tree/dev/editors/vscode/test) |
| Native compilation with reference counting (v0.4) | Not started | |
