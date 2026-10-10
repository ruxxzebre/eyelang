# Eyelang 2 — language design (draft)

Status: **draft**. These decisions come from design sessions on 2026-10-10. The build plan is in
[ROADMAP.md](ROADMAP.md). None of it is implemented yet: the code in this repository is Eyelang 1, a Monkey-style tree-walking interpreter that served
as a testing ground (see [What happens to Eyelang 1](#20-what-happens-to-eyelang-1)).

Eyelang 2 is a general-purpose, statically typed, compiled language. It is designed to make one
kind of program easy to get right: code that manages data spread across many entities (users,
devices, game worlds, apps) and jobs that run briefly and then wait for minutes, hours or days. It
is not a configuration DSL. Every feature below is an ordinary language construct that also works
outside that domain.

## Contents

1. [Goals](#1-goals)
2. [Decision log](#2-decision-log)
3. [A tour](#3-a-tour)
4. [Lexical structure](#4-lexical-structure)
5. [Values and types](#5-values-and-types)
6. [Bindings, functions and pipes](#6-bindings-functions-and-pipes)
7. [Mutation](#7-mutation)
8. [Methods and generics](#8-methods-and-generics)
9. [Pattern matching](#9-pattern-matching)
10. [Errors: Result only](#10-errors-result-only)
11. [Traits](#11-traits)
12. [Effects and signatures](#12-effects-and-signatures)
13. [Programs and concurrency](#13-programs-and-concurrency)
14. [Entities](#14-entities)
15. [Jobs and waiting](#15-jobs-and-waiting)
16. [Views and triggers](#16-views-and-triggers)
17. [Contracts: talking to other services](#17-contracts-talking-to-other-services)
18. [Calling C and Go](#18-calling-c-and-go)
19. [Time, secrets, tests](#19-time-secrets-tests)
20. [What happens to Eyelang 1](#20-what-happens-to-eyelang-1)
21. [Implementation plan](#21-implementation-plan)
22. [Grammar sketch](#22-grammar-sketch)
23. [Open questions](#23-open-questions)

## 1. Goals

- **A good general language first.** Values with no shared mutation, variants with exhaustive
  matching, traits and generics, inferred types, structured concurrency, and no nulls or exceptions.
  The domain constructs (entities, jobs, views, contracts) sit on top of a core that stands on its
  own.
- **Good enough to host itself.** The core must be comfortable for systems code: parsers, stores,
  type checkers. Eyelang's own compiler should be writable in Eyelang once it compiles to native
  code (v0.4).
- **Waiting is cheap.** A job that is waiting is a saved record in the store, not a process or a
  thread. Resuming one takes milliseconds.
- **State has an owner.** Every piece of stored state belongs to exactly one entity. Changes inside
  eyelang are atomic, and changes to other services are keyed and reconciled, never blindly retried.
- **Everything a reviewer needs is in the diff.** Public signatures state their types and effects,
  and every point where a job can pause is marked with `!`.
- **Small and predictable for agents.** Coding agents write most of the changes, so there is one
  obvious way to do each thing, strict checks, and tests that can't flake.
- **Low memory.** Compiled to native code with reference counting (no tracing GC). A waiting job
  holds no memory at all.

## 2. Decision log

| Area | Decision | Why |
|---|---|---|
| Core model | **Entities** hold shared data; **jobs** are durable functions; **views** react to conditions across many entities | Data has a home, waiting has a home, and cross-entity conditions don't need polling |
| Waiting | **Only jobs wait.** Entity methods are short atomic transactions | No state changes in the middle of a method; every method is all-or-nothing |
| Wait marker | A **`!` suffix** on every durable step in a job, which is every call that may pause or has an outside effect: `sleep!(20 min)`, `notify.send!(…)` | Every save point is visible in code review |
| Checkpoints | Every `!` call in a job is also a **checkpoint**: its result is saved and it runs once | No separate `step` keyword is needed |
| Entity lifecycle | **Virtual**: an entity exists on first use. Ending one with protected state needs an `Approval` | No create/exists dance; deletion always involves a human |
| Storage | An **embeddable engine** with backends behind a `Store` trait. The default backend keeps everything in memory and is written in Eyelang; **Postgres** is selectable. (Proposed: the in-memory backend can journal each commit to an append-only file, for durability on one machine without a database.) | Programs, tests and embedders need no database; production picks durability; writing the default backend in Eyelang tests the language on real systems code |
| Types | **Static.** Public items carry full signatures, including effects; everything inside bodies is inferred | Changes to types and permissions show up in the diff |
| Errors | **Result only**: no exceptions. `?` wraps an error into the caller's error type when exactly one of its variants holds it; otherwise `.mapErr(…)`. Runtime faults and `unreachable` become outcomes of the run | Every failure is visible in a type, and wrapping needs no boilerplate |
| Arithmetic | **Total types**: `Int` has arbitrary precision; `/` and `%` need a `NonZero` divisor; indexing returns `Option` | Removes the runtime errors that "Result only" would otherwise need |
| Consistency | **Atomic inside eyelang**; keyed commands and **reconcile** at the boundary with other services | One database makes real transactions cheap; outside services own their own state |
| Queries | **Pipelines** (`filter`, `map`, …), compiled to SQL when the source is an entity set | The same code as for lists |
| Reactions | **Views trigger jobs**: `@on(enters(view))`, `@on(leaves(view))` | Replaces most polling cron jobs |
| Evolution | Adding a field with a default is automatic; anything else needs a **`migrate from N`** function; deploy is refused if any stored version has no path to the current one | Stored data outlives code |
| Mutation | **Mutable value semantics**: `var` locals, `mut` parameters and `mut self` methods, field assignment and `+=`. Callers write `mut` too: `unify(mut subst, a, b)`. Values never alias; there are no references or pointers | Systems code reads naturally, every place a value can change is visible in the diff, and without aliasing there are no cycles, so reference counting stays enough |
| Generics and methods | Lower-case **type variables** (`List(a)`), requirements with `where a: Ord`, and methods on a type with `impl Type { … }` | Generic code without new brackets; types get their own operations |
| Shared behaviour | **Traits** with explicit `impl`; equality, ordering, printing, hashing and saving are derived for data types | One mechanism for generic code |
| Declarations | **Typed attributes** (`@on`, `@memory`, `@priority`, …) on jobs, entities and contracts | One general mechanism instead of special syntax per feature |
| Programs | **`main` is an ordinary function**; files, processes and other I/O are effects called directly. `!` marks durable steps and is used only in jobs. `unreachable("why")` is the one way for code to stop itself | The core is complete without the durable runtime; jobs stay explicit about every step |
| Concurrency | **Structured**, without `async`/`await`: `parallelMap`, `both`, `race` start tasks that finish before the call returns, and a waiting task parks without blocking others. Tasks share only immutable values. From v0.2, task groups with `spawn` and bounded channels, also scoped | Parallelism without coloured functions or data races; back-pressure keeps memory bounded |
| Dynamic dispatch | Static by default; **`dyn Trait`** from v0.2 for values whose type is known only at run time (`fn open(config) -> dyn Store`, `List(dyn Handler)`) | The cost of a run-time lookup is visible in the type; libraries can accept implementations they don't know |
| Interop | **`extern fn`** for C functions and for functions provided by a Go program that embeds Eyelang | Reuse existing libraries; the standard library's primitives use the same mechanism |
| Keywords | **Only statement and expression words are reserved.** Declaration words (`type`, `entity`, `state`, `job`, `view`, `test`, …) are keywords only where a declaration starts | Common names such as `state`, `view`, `type` and `test` stay usable |
| Blocks | **Braces** | Keeps parsing simple; no whitespace pitfalls |
| Statements | **A newline ends a statement** unless the next line starts with an operator, `.`, `,`, `:`, `=` or `else`. `;` also separates statements | No semicolons, and a long pipeline can still break before each `|>` |
| Braces in expressions | A `{` after `if`, `else`, `for`, `while`, `=>`, a signature or a lambda's parameters opens a block; anywhere else it starts a record or map. Variant payloads use parentheses: `Pending(receipt: id)` | Every `{` has one reading, found without lookahead, in both parsers |
| Pipes | Data-first: `x |> f(a)` is `f(x, a)`; `_` places the value elsewhere: `x |> f(a, _)` | Matches method-call intuition |
| Memory | Reference counting with in-place reuse of uniquely owned values; immutable values; entities referenced by ID | No GC pauses, and no reference cycles by construction |
| Parsing | A **tree-sitter grammar is the spec**; the compiler's parser is hand-written; a conformance suite fails CI if the two disagree | One definition of the syntax, editor support for free, and compiler error messages under our control |
| Metaprogramming | **No user-defined macros before v1.0.** `entity`, `job`, `contract` and `view` are built in, each lowered by its own compiler pass. Deriving is limited to the built-in traits; rare code generation goes through a build step (`eye generate`) | Fixed constructs give clearer errors and are easier for agents to work with; a macro system would delay v0.1; separate passes keep the option open |

## 3. A tour

One file that touches most of the language: an entity for Minecraft worlds, a view that notices
idle worlds, a job that puts them to sleep, and a test.

```eye
import zeropod                       # contract: the checkpoint service (Go)
import notify                        # contract: notifications (TypeScript)

let owner = UserId("operator")

type Phase = Asleep | Waking | Running

# A world is an entity: identity, owned state, atomic methods.
entity World(id: WorldId) {
  state phase: Phase = Asleep
  state players: Set(PlayerId) = {}

  pub fn join(p: PlayerId) {
    players = players |> add(p)
    if phase == Asleep {
      phase = Waking
      wake.start(id)                 # starting a job is part of this transaction
    }
  }

  pub fn leave(p: PlayerId) {
    players = players |> remove(p)
  }

  pub fn markRunning() { phase = Running }
  pub fn markAsleep() { phase = Asleep }
}

# Kept up to date by the runtime as World transactions commit.
view idleWorlds = World.all
  |> filter(fn w: w.phase == Running && w.players == {})

# Every world that becomes idle gets one run of this job.
@on(enters(idleWorlds))
@priority(background)
@memory(16 MiB)
job sleepWhenIdle(w: Ref(World)) -> Result((), SleepError) {
  sleep!(20 min)                                   # the run is a row for 20 minutes
  let world = w.read!()                            # fresh snapshot after the wait
  if world.players != {} {
    return Ok(())                                  # someone joined meanwhile
  }
  match zeropod.checkpoint!(w.id, key: "sleep-{w.id}-{world.version}").reconcile!() {
    Done(_) => {
      w.markAsleep!()                              # exactly once: same transaction as the checkpoint
      Ok(())
    }
    Refused(reason) => {
      notify.send!(owner, "world {w.id} didn't sleep: {reason}", key: "sleep-refused-{w.id}")
      Err(SleepRefused(reason))
    }
  }
}

test "an idle world sleeps after 20 minutes" {
  with fakes(zeropod, notify, from: "receipts/")
  with clock(2026-10-10 12:00 in "Europe/Dublin")
  let w = World(WorldId("survival"))
  let alice = PlayerId("alice")

  w.join(alice)
  w.markRunning()
  w.leave(alice)                                   # enters idleWorlds

  advance(21 min)
  expect(w.phase == Asleep)
}
```

## 4. Lexical structure

Source files are UTF-8. Names and keywords are ASCII; strings and comments may contain any text.

- **Comments:** `#` to the end of the line.
- **Names:** a letter or `_`, then letters, digits and `_`. A name that starts with an upper-case
  letter names a type, variant, entity or contract (`World`, `Some`, `Notify`); any other name is a
  value, function, field or module (`world`, `markAsleep`, `zeropod`). Names can no longer end in
  `!` or `?` (Eyelang 1 allowed `update!` and `ok?`): `!` now marks waiting calls and `?`
  propagates errors.
- **Literals:**

  | Kind | Examples |
  |---|---|
  | Int (arbitrary precision) | `0`, `42`, `1_000_000` |
  | Float (64-bit IEEE) | `1.5`, `0.25`, `6.02e23` |
  | String, with interpolation | `"hello"`, `"ireland-{day}"`, `"\{literal brace}"` |
  | Bool | `true`, `false` |
  | Char | `'a'`, `'\n'`, `'\u{1F600}'` |
  | Unit | `()` |
  | Duration | `500 ms`, `20 min`, `3 h`, `2 days`, `1.5 h` |
  | Size | `512 B`, `64 KiB`, `64 MiB`, `6 GiB` |
  | Time of day | `07:40`, `23:59:30` |
  | Date | `2026-10-10` |
  | Instant | `2026-10-10 07:40 in "Europe/Dublin"` |
  | Next time of day in a zone | `07:45 in "Europe/Dublin"` |

  A number followed by a unit name (`ms`, `s`, `min`, `h`, `day`, `days`, `week`, `weeks`, `B`,
  `KiB`, `MiB`, `GiB`) is a Duration or Size. A date or time of day is written without spaces;
  `2026 - 10 - 10` is still subtraction. Times are two digits each: `7:40` is not a time.
- **Characters:** `'a'` is a `Char`, one Unicode scalar value, with the escapes of strings plus
  `\'`. Where a `Byte` is expected, an ASCII character literal is a `Byte`: `src[i] == Some('"')`.
- **Strings:** a string's text stays on one line, though an interpolated expression may break
  across lines. The escapes are `\n`, `\r`, `\t`, `\0`, `\\`, `\"`, `\{`, `\}` and
  `\u{1F600}`. `{expression}` interpolates a value that has `Show`.
- **Keywords.** These are reserved and are never names: `let var mut fn pub return if else match
  for in while break continue true false _`. Every other keyword is a keyword only where the grammar
  expects it, and an ordinary name everywhere else, so `state`, `view`, `type` and `test` work as
  variable and field names:
  - at the start of a declaration: `import type opaque trait impl entity job view contract test
    extern`;
  - inside declarations: `state` and `migrate … from` in entities; `command`, `query`, `event` and
    `on` in contracts; `uses` and `where` after a signature;
  - `with` at the start of a statement and inside a record update, `dyn` at the start of a type,
    and the unit names after a number.

  A contextual keyword is read as a keyword wherever one is allowed, so a variable named `with`
  can't begin a statement. `in` and `for` may also be argument labels: `daily(07:40, in: zone)`.
- **Attributes:** `@name` or `@name(args)` before a declaration or member.
- **Newlines end statements.** A line break ends a statement, declaration, member or match arm
  wherever one could end. It doesn't when the next line (ignoring blank and comment lines) starts
  with a binary operator, `.`, `?`, `,`, `:`, `=`, `->` or `else`; `-` and `!` continue only as
  `->` and `!=`, since `-x` and `!x` can start a statement. Inside `(…)`, `[…]` and record or map
  braces no statement can end, so line breaks there are ordinary spaces (until a lambda's block
  starts a new list of statements). `;` separates statements on one line.

  ```eye
  let ready = worlds
    |> filter(fn w: w.phase == Running)      # continues: the line starts with |>
    |> size
  let next = a
  (b)                                        # a new statement, not the call a(b)
  ```
- **Braces.** `{` opens a block after `if`, `else`, `for … in …`, `while`, `match` arms (`=>`),
  `let … else`, a function or job signature, `test "…"` and a lambda's parameters. In any other
  expression position it starts a record (`{ name: "Ann" }`), a map (`{ "a": 1 }`), an update
  (`{ user with age: 4 }`) or the empty `{}`. So a match arm that returns a record wraps it in
  parentheses: `Some(u) => ({ u with seen: true })`. A lambda doesn't need to: `fn u: { u with seen: true }`.
- **Waiting calls:** in `f!(x)` the `!` and `(` follow the callee with no spaces. `x != y` and `!x`
  are the usual operators.

## 5. Values and types

- **Primitives:** `Int`, `NonZero(Int)`, `Float`, `Bool`, `Char`, `String`, `()`.
- **Bytes and fixed-width integers:** `Byte`, `Bytes`, `Int32`, `Int64`, `UInt32` and `UInt64`,
  for binary formats, hashing and foreign functions. They compare with the usual operators but have
  no `+`, `-`, `*` or `/`: arithmetic on them is explicit, as `a.wrappingAdd(b)` or
  `a.checkedAdd(b)` (an `Option`), and so are bit operations (`a.and(b)`, `a.xor(b)`,
  `a.shiftLeft(3)`). A conversion that could lose information returns an `Option`: `Int32.from(n)`.
  Integer literals that fit are fixed-width values where one is expected: `src[i] ?? 0`.
- **Time:** `Instant`, `Date`, `Time`, `Duration`, `Zone`, `Schedule`. **Memory:** `Size`.
- **Collections:** `List(a)`, `Map(k, v)` and `Set(a)`. A `List` is a growable array. Collections
  are values: assigning or passing one never shares later changes (section 7). An update to a
  collection that nothing else refers to happens in place; otherwise the collection is copied first.
- **Tuples:** `(Int, String)`, built with `(1, "a")` and taken apart with patterns:
  `let (n, s) = pair`. A tuple has at least two elements, and there is no `.0`; when the parts
  deserve names, use a record.
- **Records:** `type User = { name: String, age: Int }`, built with `{ name: "Ann", age: 3 }` and
  updated with `{ user with age: 4 }`, or with `user.age = 4` when `user` is a `var`.
- **Variants:** `type Grant = Admitted(revision: Revision) | Pending(receipt: ReceiptId) | Refused(reason: Reason)`,
  built with `Pending(receipt: id)`. Payload fields may be named or positional (`Some(a)`). A
  variant list needs at least one `|`, so a type with a single variant starts with one:
  `type Wrapper = | Wrapped(Int)`. Without a `|`, `type Ids = List(Id)` is an alias.
- **Built-in variants:** `Option(a) = Some(a) | None` and `Result(a, e) = Ok(a) | Err(e)`.
  There is no null.
- **Type variables** are lower-case names in types: `List(a)`, `Map(k, v)` (section 8).
- **Ranges:** `0..n` counts up to but not including `n`, and `0..=n` includes it. Ranges drive `for`
  loops, slices and patterns.
- **Slices:** `xs[2..5]`, `xs[..n]` and `bytes[i..]` are views that share storage with what they
  come from. A slice is clamped to the elements that exist, so it never fails, while `xs[i]`
  returns an `Option`. Strings are sliced by byte offsets with `s.slice(2..5)`, an `Option(String)`
  that is `None` when the range would split a character.
- **Iteration:** `for` walks lists, sets, ranges, `Bytes`, a map's `(key, value)` pairs, a string's
  characters (`s.chars()`) and any `Iter(a)`. `xs.iter()` gives a lazy `Iter(a)` whose pipelines
  (`map`, `filter`, `take`, `enumerate`, `zip`) don't build intermediate lists; `toList` collects
  one.
- **Entity references:** `Ref(World)` is an ID, not a pointer. It can be stored, sent and saved
  anywhere, and it can't be confused with `Ref(Player)`.
- **Special types:** `Secret(a)` (no printing, no saving), `Approval` (only a human approval flow
  can create one), `Key` (an idempotency key), `Run(a)` (a started job), `Handle(name)` (an object
  owned by foreign code, section 18).
- **Opaque types:** `pub opaque type Lexer = { … }` exports the type but not its fields, so other
  modules use it only through its functions and methods.
- **Maps** use literal or parenthesised keys: `{"a": 1}`, `{(key): 1}`. Records use names:
  `{a: 1}`, and `{ name, age }` is short for `{ name: name, age: age }`. `{}` is an empty map,
  set or record, depending on the type expected. Other sets are built from lists: `toSet([a, b])`.

## 6. Bindings, functions and pipes

```eye
pub fn area(w: Float, h: Float) -> Float = w * h       # public: full signature
fn double(n) = n * 2                                     # private: inferred
```

Inside a function body:

```eye
let x = 5                    # immutable, block-scoped; a later `let x` shadows it
var total = 0                # mutable local; closures can't capture a `var` (section 7)
total += x

let inc = fn n: n + 1                                    # lambdas
let add = fn (a, b): a + b
let show = fn item {
  let label = item.name
  "<{label}>"
}
let later = fn { log.info("done") }                       # no parameters

let sizes = urls |> map(fetchSize) |> filter(fn s: s > 0)   # data-first pipe
let diff = 2 |> sub(10, _)                                   # _ places the value: sub(10, 2)

if n < 2 { n } else { fib(n - 1) + fib(n - 2) }          # if is an expression; no parentheses
for item in items { log.info(item.name) }
for (i, item) in items |> enumerate { log.info("{i}: {item.name}") }
for i in 0..n { log.info("{i}") }
```

- Named arguments with defaults: `fn page(size: Int = 50, after: Option(Cursor) = None)`.
- No partial application by argument count. Use `_` or a lambda.

## 7. Mutation

Eyelang has mutable value semantics, as Swift's value types and Hylo do. A value never changes
behind your back: the only things that change are `var` variables and `mut` parameters, and a
change is visible only through the variable it was made to.

```eye
type Counter = { hits: Int, last: Option(Instant) }

fn record(mut c: Counter, at: Instant) {      # may change the caller's counter
  c.hits += 1
  c.last = Some(at)
}

impl Counter {
  fn reset(mut self) { self.hits = 0 }
}

fn tally(times: List(Instant)) -> Counter {
  var counter = { hits: 0, last: None }
  for t in times {
    record(mut counter, t)                    # `mut` at the call: this call may change counter
  }
  let snapshot = counter                      # a copy; later changes to counter don't reach it
  counter.reset()                             # a `mut self` method needs a `var` receiver
  snapshot
}
```

- **What can change:** a `var`, a `mut` parameter, and fields of either (`c.last = …`). `+=`, `-=`,
  `*=`, `/=` and `%=` combine an operation with an assignment. A `let` never changes.
- **Visible at the call.** A `mut` argument is written `mut x` (or `mut x.field`) and must be a
  `var` or a `mut` parameter. A method that takes `mut self` can only be called on one.
- **No aliasing.** The same variable can't be passed as `mut` twice in one call, or passed as `mut`
  and read in the same call: `swap(mut a, mut a)` is a compile error. Closures can't capture a
  `var` or a `mut` parameter.
- **No references.** A `mut` parameter lasts for the call; it can't be stored or returned. Nothing
  in the language is a pointer, so values can't form cycles, and reference counting reclaims
  everything without a garbage collector.
- **In place when unique.** A change to a value that nothing else refers to happens in place. If
  something does, the value is copied first (copy on write), so the change stays local.
- **Entity methods** are `mut self` methods over the entity's state, run as a transaction
  (section 14).

## 8. Methods and generics

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
  Trait methods are called the same way (section 11).
- **Pipes and methods.** `x |> f(a)` calls an ordinary function with `x` first; `x.f(a)` calls a
  method. Collection operations in the standard library are functions, used with pipes, so the same
  pipeline works on lists, iterators and entity sets.
- **Type variables** are lower-case names in types, as in Gleam: `fn first(xs: List(a)) -> Option(a)`,
  `type Pair(a, b) = { first: a, second: b }`. A lower-case name followed by `.` is a module:
  `notify.Delivery`.
- **Requirements** on type variables follow `where`: `where a: Ord + Show`. A trait used as a
  parameter type is short for one: `fn label(x: Named)` means `fn label(x: t) where t: Named`.
- Generic code is specialised for the types it is used with, so it costs nothing at run time.
- **`dyn Trait`** (from v0.2) is a value of some type that implements the trait, chosen at run
  time. Calls on it look up the implementation, and `dyn` in the type says so:

  ```eye
  fn open(config: Config) -> dyn Store =
    if config.postgres { PgStore.connect(config.url) } else { MemoryStore.new() }

  let handlers: List(dyn Handler) = [Logger.new(), Metrics.new(port: 9100)]
  ```

  Only traits whose methods don't mention their own type in arguments or results, and aren't
  generic, can be used this way. Until v0.2, a variant that wraps each implementation does the job.

## 9. Pattern matching

```eye
match grant {
  Admitted(revision) => use(revision)
  Pending(receipt) if receipt.age > 1 h => escalate(receipt)
  Pending(receipt) => wait(receipt)
  Refused(..) => report(grant)
}

let { name, age } = user                     # destructuring
let [first, ..rest] = items else { return Err(Empty) }
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

Every `match` must be exhaustive. Adding a variant to a type makes each `match` that doesn't handle
it a compile error, which is how a new outcome from another service reaches every caller.

## 10. Errors: Result only

- Expected failures are values: `Result(T, E)` and domain variants such as `Refused` and
  `Uncertain`. Postfix `?` returns the error to the caller: `let p = Player(id).profile()?`. In a
  function that returns an `Option`, `?` on an `Option` returns `None`.
- **Converting errors.** When the caller's error type has exactly one variant whose payload is the
  error's type, `?` wraps the error in it. With none or several, write the conversion:
  `.mapErr(Variant)?` or `.mapErr(fn e: …)?`.

  ```eye
  type CountError = | Usage | Unreadable(FsError) | BadNumber(ParseError)

  fn count(path: String) -> Result(Int, CountError) uses {fs} {
    let text = fs.readText(path)?        # an FsError becomes Unreadable(…)
    let n = parseInt(text)?              # a ParseError becomes BadNumber(…)
    Ok(n)
  }
  ```
- **Programs that only report errors** can use the standard library's `Problem`: `?` turns any
  error with `Show` into a `Problem`, and `.context("reading {path}")` adds a line to it.
- There are no exceptions. Library functions are total: `xs[0]` and `first(xs)` return `Option`,
  and `parseInt(s)` returns `Result`.
- **`unreachable("why")`** is the one way for code to stop itself. It ends the program, or the
  job's run, as a fault: `Failed(Unreachable("why"))`. It is for states the code has already ruled
  out, such as an index a loop just checked, never for failures that can happen.
- **Arithmetic is total.** `Int` has arbitrary precision, so it can't overflow. Small values stay
  unboxed at run time. `/` and `%` take a `NonZero(Int)` divisor. Literals other than `0` qualify
  automatically; for any other value, `nonzero(n)` returns `Option(NonZero(Int))`. `Float` follows
  IEEE 754, so dividing by zero gives an infinity rather than a failure.

  ```eye
  let avg = total / 3
  match nonzero(count) {
    Some(n) => total / n
    None => 0
  }
  ```
- **Runtime faults** (out of memory, over budget, killed, deadline passed, `unreachable`) are not
  errors that code can throw or catch. They end the run, and whoever started it sees `Failed(fault)` as the run's
  outcome.

## 11. Traits

```eye
trait Named {
  fn name(self) -> String
}

impl Named for Player {
  fn name(self) = self.nick
}

fn label(x: Named) -> String = "<{x.name()}>"     # any type with an `impl Named` (section 8)
```

- `Eq`, `Ord`, `Show`, `Hash` and `Save` are derived automatically for records, variants and
  collections of types that have them.
- `Secret(a)` deliberately has no `Show` and no `Save`. Printing a secret, or holding one across a
  `!`, is a compile error.
- Trait calls are resolved at compile time where the type is known, so they cost nothing extra.

## 12. Effects and signatures

The compiler infers what each function touches. Public signatures state it with `uses`, so a new
permission always shows up in the diff.

```eye
pub fn summarise(items: List(Item)) -> String uses {wait, net Llm.complete} {
  llm.complete!(prompt(items))
}
```

| Effect | Comes from | What the compiler does with it |
|---|---|---|
| `wait` | any `!` call | Compiles the function to a resumable state machine; allowed only in jobs and waiting functions |
| `net C.op` | calling an operation on contract `C` | Derives network policy, credentials and allowed-caller checks |
| `publish T`, `consume T` | sending or receiving events | Derives message-queue permissions |
| `entity E` | reading or changing entities of type `E` | Database grants; includes the type in backups |
| `secret S` | using secret `S` | Delivers that secret only to code that uses it |
| `clock`, `random` | `now()`, `random()` | Lets tests replace them |
| `fs`, `process`, `stdio`, `env` | files, running programs, standard input and output, environment variables | Lets embedders and sandboxes grant them one by one |
| `ffi` | calling an `extern fn` (section 18) | Marks code the compiler can't check |

A private function's effects are inferred and never need writing.

**`!` in jobs.** Inside a job, every call with an outside effect (`net`, `fs`, `process`, `clock`,
`random`, `ffi` and the rest) is a `!` call: it runs once and its result is recorded, so a resumed run
never repeats it and sees the same values. Calls that only touch entities or compute are ordinary.
Outside jobs the same functions are called without `!` (section 13).

## 13. Programs and concurrency

A program starts at `pub fn main`. It is an ordinary function, not a job: its effects happen
directly, without `!`, and nothing about it is durable.

```eye
import fs

pub fn main(args: List(String)) -> Result((), FsError) uses {fs, stdio} {
  for path in args |> drop(1) {
    let text = fs.readText(path)?
    print("{path}: {text |> lines |> size} lines")
  }
  Ok(())
}
```

- `main` returns `()` or `Result((), e)`. An `Err` is printed with `Show`, and the program exits
  with status 1.
- `eye run` also starts the job runtime when the program declares entities or jobs, using the
  in-memory store unless another is configured.

**Concurrency** is structured. Tasks start inside one call, and all of them finish (or are
cancelled) before that call returns, so no task outlives the code that started it.

```eye
let pages = urls |> parallelMap(fn u: http.get(u), limit: 8)    # results in input order
let (user, orders) = both(fn { users.get(id) }, fn { orders.list(id) })
let fastest = race([fn { mirrorA.get(path) }, fn { mirrorB.get(path) }])
```

- **No `async` or `await`.** Any function can run in a task. A task that waits for input or output
  is parked by the runtime while others run, so functions don't come in two colours.
- **No data races.** Tasks share only immutable values; a `var` or `mut` parameter can't be
  captured. Results come back as return values.
- **Cheap tasks.** A task's stack starts small and grows, so thousands of waiting tasks cost little
  memory.
- `race` cancels the losers at their next wait. `timeout(5 s, fn { … })` returns an `Option`.
- **From v0.2, task groups and channels** cover streams and work that is discovered as it goes. A
  group starts tasks with `spawn` and returns only when all of them have finished. A channel has a
  fixed capacity, and `put` parks while it is full, so a fast producer can't fill memory:

  ```eye
  taskGroup(fn group {
    let (send, receive) = channel(capacity: 64)
    group.spawn(fn {
      for line in fs.lines(path) { send.put(line) }
      send.close()
    })
    for line in receive { handle(line) }
  })
  ```

  Channels aren't durable and can't be live across a `!`; jobs talk through entities and events.
- **In jobs**, concurrent durable steps are child runs (`digest.start(day)`) joined with `all!` or
  `first!`. `parallelMap` is allowed in a job only with a pure function, since its steps aren't
  recorded.

## 14. Entities

```eye
@version(3)
entity Player(id: PlayerId) {
  state name: String
  state devices: Set(DeviceId) = {}          # v3: added; the default fills old rows

  pub fn addDevice(d: DeviceId) -> Result((), DeviceError) {
    if devices |> size >= 5 { return Err(TooManyDevices) }
    devices = devices |> add(d)
    Ok(())
  }

  pub fn retire(ok: Approval) { end(ok) }     # ending an entity needs an Approval

  migrate from 2 (old) {                      # pure function from the old stored shape
    { name: old.nick, devices: {} }
  }
}
```

- **Identity:** `Player(id)` is a reference. The entity exists on first use, starting from
  default state.
- **Methods are atomic transactions.** One writer at a time per entity, all or nothing. A method
  may call methods on other entities, and the whole call tree commits or rolls back together.
- **State and `mut self`:** an entity method is a `mut self` method over the entity's state
  (section 7), and a `state` field such as `devices` is short for `self.devices`. The state is
  loaded before the call and saved when the transaction commits; nothing outside the method sees a
  change until the commit.
- **Methods never wait.** They can't use `!`, so they can't call other services; those calls
  belong in jobs. A method can start a job (`wake.start(id)`), and the job begins only if the
  transaction commits.
  Because entity methods have no outside side effects, the runtime can safely retry one after a
  conflict.
- **Reading from a job:** `w.read!()` returns a snapshot with a `version` field. Snapshots go stale,
  and commands can name the version they were based on (see `@fresh` in section 17).
- **Evolution:** adding a field with a default is automatic. Any other change to stored state needs
  a `migrate from N` function. The compiler refuses to deploy if a stored version has no path to the
  current one, and rows are migrated when they are next loaded.

## 15. Jobs and waiting

A job is a durable function: it can pause for any length of time and holds no memory while paused.

```eye
@on(daily(07:40, in: "Europe/Dublin"))
@priority(background)
@memory(64 MiB)
@deadline(13:40, in: "Europe/Dublin")
job irelandDigest(day: Date) -> Result(Delivery, DigestError) {
  let items = miniflux.unread!("ireland", since: day - 1 day)?
  let text = reserve!(256 MiB, fn {            # waits until the host grants the memory
    summarise!(items)
  })
  Ok(notify.send!(owner, text, key: "ireland-{day}").reconcile!())
}
```

- **`!` marks a checkpoint.** Every call with an outside effect in a job is a `!` call (section 12).
  At each `!` the job's live variables are saved, and the call's result is recorded. A resumed or restarted run continues from the last checkpoint and never
  repeats a recorded call.
- **Only values that can be saved** (types with `Save`) may be live across a `!`. Holding a socket,
  file handle or `Secret` across a checkpoint is a compile error.
- **Entity calls from jobs** (`World(w).markAsleep!()`) commit in the same store transaction as the
  checkpoint, so they happen exactly once.
- **Where runs live.** Entities, runs and checkpoints are kept by a store, chosen when the program is
  started or embedded. The default store keeps them in memory and is itself written in Eyelang: it is
  an immutable value, each transaction is a pure function from one store value to the next, and the
  runtime holds the only reference, so updates happen in place. It is what tests, the REPL and
  embedders use; on its own it doesn't survive a restart. The Postgres store survives restarts and
  serves several runners. Both implement the `Store` trait from the standard library, and programs
  don't change when the store does.
- **Calls to other services** carry a key. If the outcome is `Uncertain`, `.reconcile!()` asks the
  owner with the same key until it knows. Nothing is resent automatically.
- **Waiting constructs** (all library functions, all marked with `!`):

  | Call | Waits for |
  |---|---|
  | `sleep!(20 min)`, `sleepUntil!(07:45 in "Europe/Dublin")` | time |
  | `approval!("delete 3 backups", from: operator, within: 3 days)` | a human; returns `Approved(Approval) \| Declined \| Expired` |
  | `next!(notify.delivered, where: fn e: e.key == k, within: 1 h)` | an event |
  | `reserve!(1 GiB, fn { … })` | memory from the host, released when the function returns |
  | `lock!("ci", for: 30 min, fn { … })` | a named lock that records its owner and expiry |
  | `run.join!()` | a job started earlier |
  | `all!([a, b])`, `first!([a, b])` | every one, or whichever finishes first, of several waits |

- **Starting jobs:** `irelandDigest.start(day)` returns a `Run(a)`. It works from entity methods (as
  part of the transaction) and from jobs, where it starts a child run. Scheduled starts take
  `after:` or `at:`: `sleepWhenIdle.start(w, after: 20 min)`,
  `irelandDigest.start(day, at: 07:40 in "Europe/Dublin")`.
- **Attributes:**

  | Attribute | Meaning |
  |---|---|
  | `@on(trigger)` | `daily(…)`, `weekdays(…)`, `every(15 min)`, `cron("…", in: zone)`, `enters(view)`, `leaves(view)`, `event(contract.event)` |
  | `@memory(size)` | budget admitted before the run starts; the runtime enforces it |
  | `@priority(p)` | `interactive`, `normal`, `background` or `batch`; under memory pressure, lower priorities pause at their next checkpoint |
  | `@deadline(time or duration)` | after this the run ends as `Failed(DeadlinePassed)` |
  | `@concurrency(n, by: key)` | at most `n` runs at once, optionally per key |
  | `@limit(n, per: duration)` | rate limit |

## 16. Views and triggers

```eye
view idleWorlds = World.all
  |> filter(fn w: w.phase == Running && w.players == {})

@on(enters(idleWorlds))
job sleepWhenIdle(w: Ref(World)) { sleep!(20 min) }   # full version in the tour
```

- A view is a named pipeline over entities. The runtime keeps it up to date as entity transactions
  commit.
- `enters(view)` starts one run per entity that begins matching; `leaves(view)` starts one per
  entity that stops matching.
- Over entity sets, pipelines compile to SQL. The same functions work on in-memory lists.

## 17. Contracts: talking to other services

A contract is the single description of another service's interface. It is written once and shared
by eyelang and the Go, TypeScript, Rust and Python code that implements or calls it. Think of it as a
trait (an interface of operations) plus the facts needed to deploy it: port, allowed callers, keys and
how to reconcile.

```eye
@port(3001)
@allow(digest, inbox, welcome)          # who may call; everyone else is denied
contract Notify {
  @key(required)
  @reconcile(receipt)
  command send(to: UserId, text: String) -> Delivery

  query receipt(key: Key) -> Delivery
  query identity(user: UserId) -> Identity

  event delivered: Delivered on "notify.events"
}

type Delivery =
  | Sent(receipt: ReceiptId, at: Instant)
  | Pending(receipt: ReceiptId)
  | Refused(reason: Refusal)
```

- **`import notify` is a typed edge.** Port, network policy, credentials and the allowed-caller check
  are derived from it, so a wrong port or an unapproved caller is a compile error.
- **Commands** return their declared variants plus `Uncertain(key)`, which `.reconcile!()`
  resolves through the declared query.
- **`@fresh`** on a command adds a required `basedOn: Version` argument and `Stale`/`Expired`
  outcomes, so an old command can't act on newer state.
- **`@destructive`** on a command adds a required `Approval` argument.
- **Fakes come from recordings.** `with fakes(notify, from: "receipts/")` replays real recorded
  exchanges, checked against the contract. Reading a field that no recording contains is a compile
  error, unless it is marked `@unwitnessed("why")`.
- **Generated code for other languages** (types, clients and server stubs for Go, TypeScript, Rust
  and Python) is a build output, never committed.

## 18. Calling C and Go

```eye
@library("z")
extern fn crc32(crc: UInt64, data: Bytes, length: UInt32) -> UInt64

@host("lab.desk")                  # provided by the Go program that embeds Eyelang
extern fn showOnDesk(text: String) -> Result((), String)
```

- An `extern fn` has a signature and no body. Calling one has the `ffi` effect, which public
  signatures state like any other.
- `@library` links a C function by its symbol. `@host` names a function that the Go program
  embedding Eyelang registers through the embedding API; the standard library's own primitives are
  provided the same way.
- Across the boundary go `Bool`, `Byte`, the fixed-width integers, `Float`, and `Bytes` and `String`
  (copied). An object owned by foreign code is a `Handle(name)`, which can't be saved, so it can't
  be live across a `!`.
- The compiler trusts an extern's signature. It can't check that the foreign side is total or that
  it has no other effects, which is why `ffi` shows up in every signature that depends on one.

## 19. Time, secrets, tests

**Time.** Schedules carry their zone: `daily(07:40, in: "Europe/Dublin")`. Daylight-saving gaps and
overlaps follow a declared policy (`onGap: shift`, `onOverlap: first`), so there are never two UTC
crons and an hour check.

**Secrets.**

```eye
let token: Secret(String) = secret("telegram-bot-token")
http.post(url, auth: bearer(token))        # bearer() accepts a Secret
log.info("token is {token}")               # compile error: Secret has no Show
```

**Tests.** A `test` block is a declaration. `with` applies a handler to the rest of the block:
`fakes(…)`, `clock(…)` with `advance(…)`, `memory(…)`. In tests, jobs run against an in-memory
store, so every run is deterministic and fast.

## 20. What happens to Eyelang 1

The current code was a testing ground. Little of it carries over, because Eyelang 2 changes the
semantics, not just the syntax. Proposed handling:

| Part | Size | Verdict | Reason |
|---|---|---|---|
| `featureflags/` | 5 lines | **Delete now** | Defines one constant that nothing uses |
| `assert/` | 61 lines | **Delete now** | Only referenced by a commented-out import |
| `exploration/` | 23 lines | **Delete now** | A scratch test about runes |
| `testing/` | 17 lines | **Delete now** | Changes the working directory in `init()` so tests find `examples/`; tests should use `testdata/` instead |
| `bin/` | 8.2 MB of binaries | **Delete now** | Build outputs don't belong in git; publish them as release assets from CI |
| `parser/debug.go`, `parser/parser_tracer.go` | 59 lines | Delete with the parser | Commented-out code and a debug tracer called from 17 places |
| `token/`, `lexer/` | ~420 lines | **Replace** | New tokens (units, dates, times, interpolation, `!`, `@`), byte-offset spans, keeping comments for the formatter |
| `parser/`, `ast/` | ~1,100 lines | **Replace** | Needs a lossless syntax tree with error recovery (see section 21) |
| `eval/`, `object/` | ~1,600 lines | **Replace** | The new reference interpreter runs typed IR with entities and jobs; the current evaluator can't be adapted to that |
| `cli/`, `main.go` | ~120 lines | **Replace** | Becomes the `eye` command: `run`, `build`, `test`, `fmt`, `check`, `repl`, `why` |
| `repl/lineedit.go` | 318 lines | **Keep** | The line editor and history work and aren't tied to the language |
| `repl/repl.go` | 125 lines | Adapt | Reuse the loop around the new front end |
| `examples/*.eye` | 6 files | Replace | Eyelang 1 syntax; Eyelang 2 examples double as test fixtures |
| `.github/workflows/ci.yml`, `Taskfile.yml`, `go.mod` | — | Keep | Update tasks as the packages change |

Before deleting anything, tag the current state as `eyelang-1` so it stays easy to find and run.

## 21. Implementation plan

**Parsing: the grammar is the spec, the compiler's parser is hand-written.** Decided 2026-10-10.

- **`tree-sitter-eyelang/grammar.js` is the executable grammar.** It is the canonical definition of
  the syntax, and section 22 is a readable summary of it. It also drives highlighting in editors
  (Neovim, Helix, Zed, and VS Code through an extension). A small external scanner in C decides
  which newlines end statements, tells a block's `{` from a literal's, and reads string text.
- **The compiler parses by hand,** in Go, with recursive descent and Pratt parsing for expressions
  (precedence table in section 22). That keeps cgo and runtime libraries out of the compiler and keeps
  error messages under our control. Most production compilers work this way: Go, Rust, TypeScript,
  Swift, Clang and Zig.
- **A conformance suite keeps the two honest.** `testdata/syntax/` holds example files, both valid and
  invalid. CI parses each file with both parsers and fails if they disagree on whether it parses,
  or if the trees differ in shape. Both parsers print the same normalised S-expression for the
  comparison: `tree-sitter parse` on one side, `eye parse --sexp` on the other. A syntax change must
  update the grammar, the parser and the suite together.
- **What the compiler's parser produces:**
  - a lossless syntax tree that keeps every byte, comments and whitespace included, as Roslyn and
    rust-analyzer do; `eye fmt` is built on it, and agents need the formatter to insert signatures;
  - byte-offset spans on every node, with lines and columns from a line table, so diagnostics point
    at exact source ranges;
  - error recovery, so one parse reports every syntax error and still builds a tree for the parts
    that parse.

Rejected: generating the compiler's parser from a grammar with goyacc, ANTLR or a PEG tool. They give
weaker error messages, and string interpolation, the `!` marker and contextual keywords all need
workarounds. Also rejected: tree-sitter as the compiler's own parser, which would need cgo, a separate
diagnostics layer, and a conversion from its editor-oriented tree.

**Front-end order**, all in Go:

1. Tree-sitter grammar, hand-written lexer and lossless parser, and the conformance suite, with
   `eye fmt` and `eye check --syntax`.
2. Name resolution and modules (`import`, `pub`).
3. Type checker: inference inside bodies, checked signatures, generics with `where`, traits and
   methods, exhaustiveness, total arithmetic.
4. Effect inference and checks: `uses`, the `!` rules, `Save` across checkpoints, and the
   no-aliasing rule for `mut`.
5. Typed IR, plus a reference interpreter on that IR. Used by the REPL and tests, with entities
   and jobs on an in-memory store, structured concurrency on lightweight tasks, and `extern`
   functions from the Go host or from C libraries loaded at run time (without cgo). `entity`, `job`, `contract` and `view` are each lowered into core
   IR by their own pass, kept separate from the type checker. Nothing else in the compiler needs to
   know about them, and the passes are already shaped like macros if metaprogramming is added after
   v1.0.
6. The runtime pieces: the `Store` trait, the runner, the in-memory store written in Eyelang, and the
   Postgres store.
7. Code generation to C with reference counting, as described in the compilation analysis.

**Self-hosting**, after native compilation (v0.4), starts with the lexer and parser:

- Once the interpreter runs programs (phase 3), an Eyelang lexer and parser are written as a test
  program in `testdata/programs/`. The conformance suite checks it as a third implementation of the
  grammar, so it needs no tests of its own, and it exercises `mut`, characters, slices and byte
  patterns on real systems code.
- After v0.4, the formatter, `eye doc` and then the checker follow. The Go compiler (stage 0)
  builds the Eyelang compiler (stage 1), which builds itself (stage 2); CI checks that stages 1
  and 2 produce the same output.
- Building Eyelang must not require an existing Eyelang binary. Releases publish the compiler's
  generated C as a bootstrap seed, as Zig keeps a WebAssembly build of its compiler for the same
  purpose; the repository itself still holds no build output.

## 22. Grammar sketch

A readable summary of [`tree-sitter-eyelang/grammar.js`](tree-sitter-eyelang/grammar.js), which is
the definition. `sep` is `;` or a newline that ends a statement (section 4); `{` marked *block* is
the brace that opens a block. Lists in `( )`, `[ ]` and literal braces allow a trailing comma.
Every `eye` example in this document is a whole file or the statements of a function body, and CI
parses each one with both parsers.

```ebnf
file       = { sep } [ item { sep item } { sep } ] ;
item       = import | fn | extern | job | entity | view | type | trait | impl | contract | test | let ;
import     = "import" ident { "." ident } [ ".{" name { "," name } "}" ] ;
attribute  = "@" ident [ args ] ;
fn         = { attribute } [ "pub" ] "fn" ident params [ "->" type ] [ uses ] [ where ]
             ( "=" expr | block ) ;
extern     = { attribute } [ "pub" ] "extern" "fn" ident params [ "->" type ] [ uses ] ;
params     = "(" [ param { "," param } ] ")" ;
param      = [ "mut" ] ident [ ":" type ] [ "=" expr ] ;           (* self, mut self *)
uses       = "uses" "{" [ effect { "," effect } ] "}" ;          (* wait, net Llm.complete *)
where      = "where" ident ":" bound { "," ident ":" bound } ;
bound      = typeexpr { "+" typeexpr } ;
job        = { attribute } [ "pub" ] "job" ident params [ "->" type ] [ uses ] block ;
entity     = { attribute } [ "pub" ] "entity" Ident params "{" members "}" ;
member     = { attribute } "state" ident ":" type [ "=" expr ]
           | fn
           | { attribute } "migrate" "from" int "(" ident ")" block ;
view       = { attribute } [ "pub" ] "view" ident "=" expr ;
type       = { attribute } [ "pub" [ "opaque" ] ] "type" Ident [ "(" ident { "," ident } ")" ]
             "=" ( typeexpr | variants ) ;
variants   = [ "|" ] variant "|" variant { "|" variant } | "|" variant ;
variant    = Ident [ "(" ( ident ":" typeexpr | typeexpr ) { "," … } ")" ] ;
trait      = { attribute } [ "pub" ] "trait" Ident [ "(" ident { "," ident } ")" ] "{" { fn | fnsig } "}" ;
impl       = { attribute } "impl" typeexpr [ "for" typeexpr ] "{" { fn } "}" ;
contract   = { attribute } [ "pub" ] "contract" Ident "{" { cmember } "}" ;
cmember    = { attribute } ( "command" | "query" ) ident params "->" typeexpr
           | { attribute } "event" ident ":" typeexpr "on" string ;
test       = { attribute } "test" string block ;

block      = "{"(block) { sep } [ stmt { sep stmt } { sep } ] "}" ;
stmt       = let | "var" ident [ ":" type ] "=" expr | place assign expr
           | "for" pattern "in" expr block | "while" expr block
           | "with" expr | "return" [ expr ] | "break" | "continue" | expr ;
let        = "let" pattern [ ":" typeexpr ] "=" expr [ "else" block ] ;
place      = ident { "." ident } ;
assign     = "=" | "+=" | "-=" | "*=" | "/=" | "%=" ;

expr       = lambda | binary ;                              (* precedence table below *)
lambda     = "fn" [ ident | params ] ( ":" expr | block ) ;
unary      = ( "-" | "!" ) unary | postfix ;
postfix    = primary { args | "!(" args… | "." name | "[" ( expr | slice ) "]" | "?" } ;
slice      = [ expr ] ( ".." | "..=" ) [ expr ] ;
args       = "(" [ arg { "," arg } ] ")" ;
arg        = [ ( ident | "in" | "for" ) ":" ] ( expr | "mut" place ) ;
primary    = ident | Ident | "_" | literal | "(" expr ")" | "(" expr "," expr { "," expr } ")"
           | "[" [ expr { "," expr } ] "]"
           | "{" "}" | "{" field { "," field } "}" | "{" expr "with" field { "," field } "}"
           | "{" key ":" expr { "," key ":" expr } "}"
           | "if" expr block [ "else" ( block | if ) ]
           | "match" expr "{" { arm ( "," | sep ) } "}" ;
field      = ident ":" expr | ident ;
key        = string | int | date | time | "(" expr ")" ;
arm        = pattern [ "if" expr ] "=>" ( block | expr ) ;
literal    = int | float | char | string | "true" | "false" | "()" | number unit
           | date | time | date time "in" string | time "in" string ;

pattern    = pattern "|" pattern | ident | "_" | literal | "-" number
           | bound ( ".." | "..=" ) bound                      (* bound: a number or char *)
           | Ident [ "(" [ ppart { "," ppart } ] ")" ]
           | "(" pattern "," pattern { "," pattern } ")"
           | "{" [ ( ident ":" pattern | ident | ".." ) { "," … } ] "}"
           | "[" [ ( pattern | ".." [ ident ] ) { "," … } ] "]" ;
ppart      = pattern | ident ":" pattern | ".." ;
typeexpr   = Ident [ "(" typeexpr { "," typeexpr } ")" ] | ident "." Ident [ "(" … ")" ]
           | ident                                            (* a type variable *)
           | "dyn" typeexpr                                   (* from v0.2 *)
           | "()" | "(" typeexpr "," typeexpr { "," typeexpr } ")"
           | "{" [ ident ":" typeexpr { "," … } ] "}"
           | "fn" "(" [ typeexpr { "," typeexpr } ] ")" [ "->" typeexpr ] ;
```

**Operator precedence**, from tightest to loosest:

| Level | Operators |
|---|---|
| 1 | postfix: call `f(x)`, waiting call `f!(x)`, field `.x`, index `[i]`, slice `[a..b]`, propagate `?` |
| 2 | prefix: `-`, `!` (not) |
| 3 | `*` `/` `%` |
| 4 | `+` `-` |
| 5 | `|>` |
| 6 | `..` `..=` (ranges; they don't chain) |
| 7 | `<` `>` `<=` `>=` |
| 8 | `==` `!=` |
| 9 | `&&` |
| 10 | `||` |
| 11 | `??` (default for `Option`) |
| 12 | lambda body: `fn x: …` takes everything to its right |

`!` directly after an expression and directly before `(` is the waiting marker; `!` before an
expression is logical not. `?` directly after an expression propagates an error; `??` is the
operator. All binary operators associate to the left except `??`, which associates to the right.

## 23. Open questions

1. **Time conditions in views**, such as "idle for 20 minutes". Today that is `enters(view)` plus
   `sleep!`. Should views support `for 20 min` directly?
2. **Trait coherence:** may a module implement a trait it doesn't own for a type it doesn't own?
   (Rust's orphan rule says no.)
3. **Exposing entities and jobs to other services:** a generated contract per entity, or explicit
   contracts only?
4. **Job code upgrades:** when a new version changes what is live at a checkpoint, do paused runs
   finish on the old version, or must the change come with a migration?
5. **Packages:** `eye.toml` and an `eye.lock` that carries hashes (so Nix needs no placeholder
   hash); registry or git only?
6. **REPL behaviour for jobs:** run them against the in-memory store with a controllable clock?
7. **Metaprogramming after v1.0:** if real code keeps repeating the same patterns, add attribute
   macros (compile-time Eyelang functions that inspect a declaration's types and generate code).
   The built-in constructs could then be re-expressed with them, and grammar and editor support
   would stay unchanged because macros would only appear as attributes.
8. **Multi-line strings:** strings stay on one line for now, which keeps an unclosed quote from
   swallowing the rest of a file in editors. Prompts and templates will want a multi-line form
   (indented, with the margin stripped). Which delimiter?
9. **Durability without a database:** the in-memory store could append each commit (entity changes
   and checkpoints) to a journal file, fsynced before the commit is acknowledged, and replay it on
   start, with periodic snapshots to bound replay time. Is that enough for single-machine
   deployments, and what is the file format?
