# Eyelang 2 — language design (draft)

Status: **draft**. These decisions come from design sessions on 2026-10-10. The build plan is in
[ROADMAP.md](ROADMAP.md). The syntax is implemented (phase 1), and a reference interpreter runs
programs, the tour in section 3 included. `eye check` checks types, exhaustive matches and effects
(a first slice of phase 2; ROADMAP.md says what is left). The lab's real workloads, written in
Eyelang in [testdata/lab](testdata/lab), check and pass their tests; the rules they changed say
"from the lab" in the decision log. Contracts call their services over HTTP, and model calls go to
an OpenAI-compatible gateway. Eyelang 1, the
Monkey-style tree-walking interpreter that served as a testing ground, has been removed; it is kept
at the tag `eyelang-1` (see [What happens to Eyelang 1](#21-what-happens-to-eyelang-1)).

Eyelang 2 is a general-purpose, statically typed, compiled language for **software that keeps state
and waits**: services, automations, schedulers and agents that run for days, hold data, call other
systems that fail, and spend most of their time waiting. It is designed to make such programs as
simple to write as a script and correct by construction. Typically that means data spread across
many entities (users, devices, game worlds, apps) and jobs that run briefly and then wait for
minutes, hours or days. It is not a configuration DSL: every feature below is an ordinary language
construct that also works outside that domain.

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
19. [AI models and agents](#19-ai-models-and-agents)
20. [Time, secrets, tests](#20-time-secrets-tests)
21. [What happens to Eyelang 1](#21-what-happens-to-eyelang-1)
22. [Implementation plan](#22-implementation-plan)
23. [Grammar sketch](#23-grammar-sketch)
24. [Open questions](#24-open-questions)

## 1. Goals

- **Good programs are the point.** A feature earns its place by the programs written with it:
  correct, fast, lean on memory, well organised and easy to change. The obvious way to write
  something should be the right way. [CLAUDE.md](CLAUDE.md) turns this into a guide for decisions.
- **Concise and pleasant.** Little ceremony, inference inside bodies, one obvious way to do each
  thing, and errors, formatting and editor support that ship with every feature.
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
| Records | **Nominal**: a record type is its name, and a literal takes the type its context expects | Same-shaped types can't be mixed up, and methods, operators, schemas and error messages have a name to use |
| Results | **A function without `->` returns `()` when its body is a block**, and its last value is dropped; one written `= expr` returns the expression's type, inferred. `while true` without `break` has any type, since only `return` leaves it | Entity methods, jobs and helpers that do something need no `-> ()`, and a loop that serves until it returns needs no `unreachable` after it. From the type checker: Notify's delivery loop |
| Effects | **Inferred inside, stated at the boundary**: public functions list their effects with `uses` (none means pure); a callback's effects pass to the call that gives it; a function type kept in data states its effects | Permissions show up in the diff without annotating every helper, and higher-order functions work with any callback |
| Operators | **Traits**: `+ - * / %` call `Add`, `Sub`, `Mul`, `Div` and `Rem`, which any type can implement once per right-hand type; no implicit conversions | Money, vectors and units get real operators, and the result type is always known from the operands |
| Arithmetic | **Total types**: `Int` has arbitrary precision; `/` and `%` need a `NonZero` divisor; indexing returns `Option` | Removes the runtime errors that "Result only" would otherwise need |
| Consistency | **Atomic inside eyelang**; keyed commands and **reconcile** at the boundary with other services. A `@reconcile` query returns an `Option` of the command's outcome: `None` means the command never arrived, and only then is it sent again, with the same key | One database makes real transactions cheap; outside services own their own state; the one case where resending is safe is decided by the owner, never guessed. From the lab: a lookup that can't say "never arrived" leaves a lost command uncertain for good |
| Values as JSON | **One JSON shape everywhere**: records are objects, a variant is an object tagged with `"kind"`, a type of fieldless variants is a string, a wrapped value is the value, `None` is null (section 17) | Requests, replies, recordings, model schemas and the generated TypeScript and Go types agree, and a tagged union reads naturally as a discriminated union in TypeScript |
| Contracts over HTTP | **Attributes map a contract to HTTP**: `@http("POST /v1/send", status: {202: Pending})` on an operation, `@http(names: snake)` and `@auth(…)` on the contract, the key's header and body field on `@key`; addresses and credentials come from the environment | The real APIs the lab calls (Notify, Miniflux) are described without code; a request that never went out is retried, and only a lost reply is `Uncertain`. From the lab: Notify's 200 and 202 answers and its idempotency header |
| Serving contracts | **A program serves a contract with `impl Contract { … }`**: one function per operation, with the contract's types and defaults, run as one transaction; a command with `@key` takes the key last | A service written in Eyelang shares one contract with its callers, and a key can name an entity, so repeats need no dedupe table. From the lab: Notify, written in Eyelang |
| Events | **Published when the transaction commits, and handled once per key.** `@on(event(e))` starts one run per event, and `@key(field)` on the event makes a repeat start nothing; `next!` counts every event published since the run's previous step | A reply that comes quickly is never missed, and a message the bus delivers twice never starts a second run. From the lab: a button pressed seconds after its question, and Telegram updates delivered again after a restart |
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
| AI | **Typed model calls and agents, as library code**, with no syntax that serves only them: a call names the Eyelang type it wants, the compiler derives a JSON Schema from it, and the reply is checked and decoded into that type. Tools are functions whose effects become the caller's; in jobs every model call is a recorded `!` step; coding agents are sessions that durable jobs own | No new syntax: permissions, cost and nondeterminism are handled by effects, budgets and recordings, which already exist |
| Syntax | **Add syntax when it is generally useful and clearly better than a library function**; syntax that also makes one area (jobs, agents) much better is welcome if it pays off elsewhere too | Keeps the language small without making common code clumsy |
| Interop | **`extern fn`** for C functions and for functions provided by a Go program that embeds Eyelang | Reuse existing libraries; the standard library's primitives use the same mechanism |
| Keywords | **Only statement and expression words are reserved.** Declaration words (`type`, `entity`, `state`, `job`, `view`, `test`, …) are keywords only where a declaration starts | Common names such as `state`, `view`, `type` and `test` stay usable |
| Blocks | **Braces**, only where they hold several statements. A branch that is one expression or one assignment needs none: `if c then a else b`, `Pattern => value`, `Ok(t) => found += [t]`, `fn x: value`, `fn f() = value`. Layout is free; indentation means nothing | Keeps parsing simple and pasting safe, with no whitespace pitfalls, while one-liners stay light. From the lab: sorting results into lists in a `match` needed braces on every arm |
| Strings | **One line, or `"""` for many lines.** A multi-line string starts with `"""` and a line break and ends with `"""` on a line of its own, apart from what follows it; that line's indentation is removed from every line (section 4) | Prompts and messages read as text in the code, an unclosed `"` still can't swallow a file, and the formatter re-indents code without changing a string. From the lab: the digest's 40-line prompt |
| Arguments | **`name:` alone passes the variable of that name**: `send(key:)` is `send(key: key)` | Keys, priorities and recipients pass through several layers under their own names, and the label still shows at each call. From the lab: `key: key` and `priority: priority` in every layer of the digest and the reminders |
| Statements | **A newline ends a statement** unless the next line starts with an operator, `.`, `,`, `:`, `=` or `else`. `;` also separates statements | No semicolons, and a long pipeline can still break before each `|>` |
| Braces in expressions | A `{` after `if … `, `else`, `for`, `while`, a signature or a lambda's parameters opens a block; anywhere else it starts a record or map. After `=>` it opens a block unless `name:`, `name,` or `name with` follows, which only a record can start. Variant payloads use parentheses: `Pending(receipt: id)` | Every `{` has one reading, found with at most two tokens of lookahead, in both parsers; a match arm returns a record without extra parentheses |
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

- **Comments:** `#` to the end of the line. A comment that starts with `##` documents the
  declaration, member, field or variant after it: `eye doc` publishes it, and a model call uses it
  to describe that part of the schema (section 19).
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
  | Instant | `2026-10-10 07:40 in "Europe/Dublin"`, `2026-10-10 07:40 in dublin` |
  | Next time of day in a zone | `07:45 in "Europe/Dublin"`, `07:45 in dublin` |

  A number followed by a unit name (`ms`, `s`, `min`, `h`, `day`, `days`, `week`, `weeks`, `B`,
  `KiB`, `MiB`, `GiB`) is a Duration or Size. A date or time of day is written without spaces;
  `2026 - 10 - 10` is still subtraction. Times are two digits each: `7:40` is not a time. The zone
  after `in` is a zone's name, or a name for a `Zone` value: `dublin` after
  `pub let dublin: Zone = "Europe/Dublin"`.
- **Characters:** `'a'` is a `Char`, one Unicode scalar value, with the escapes of strings plus
  `\'`. Where a `Byte` is expected, an ASCII character literal is a `Byte`: `src[i] == Some('"')`.
- **Strings:** a string's text stays on one line, though an interpolated expression may break
  across lines. The escapes are `\n`, `\r`, `\t`, `\0`, `\\`, `\"`, `\{`, `\}` and
  `\u{1F600}`. `{expression}` interpolates a value that has `Show`.

  A **multi-line string** starts with `"""` at the end of a line and ends at a `"""` with only
  spaces before it on its line. Those spaces are the margin: every line starts with it, or is
  blank, and loses it, and the line breaks after the opening `"""` and before the closing line
  aren't part of the string. One or two quotes inside are text; escapes and interpolation work as in other
  strings.

  ```eye
  let prompt = """
    You write a morning briefing for {reader}.
      - this line keeps two spaces
    Say "Nothing new." when nothing matters.
    """
  ```
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
- **One style.** `eye fmt` keeps the line breaks you chose and fixes everything else: two spaces
  of indentation, spacing around operators and after commas and colons, `;` turned into line
  breaks, trailing commas in lists that end on their own line, and aligned trailing comments. It
  has no options.
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
- **Braces.** Braces hold blocks of statements and data; layout is free and indentation means
  nothing. A branch that is one expression needs no braces: `if n < 2 then n else fib(n - 1)`,
  `Some(v) => v`, `fn x: x + 1`, `fn double(n) = n * 2`.

  `{` opens a block after `if …` (without `then`), `else` (after a block), `for … in …`, `while`,
  `let … else`, a function or job signature, `test "…"` and a lambda's parameters. In any other
  expression position it starts a record (`{ name: "Ann" }`), a map (`{ "a": 1 }`), an update
  (`{ user with age: 4 }`) or the empty `{}`. After a match arm's `=>`, `{` opens a block unless
  `name:`, `name,` or `name with` follows, since only a record starts that way:
  `Some(u) => { u with seen: true }` returns a record, `None => {}` is an empty block, and a record
  with one shorthand field, `{ u }`, or a map needs parentheses there: `=> ({ u })`.
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
- **Type variables** are lower-case names in types: `List(a)`, `Map(k, v)` (section 8).
- **Ranges:** `0..n` counts up to but not including `n`, and `0..=n` includes it. Ranges drive `for`
  loops, slices and patterns. A range is a value of type `Range(a)` with fields `start` and `end`.
  A range of times of day may wrap midnight: `23:00..08:00` holds 23:30 and 07:00, as quiet hours
  need.
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
- **String literals take the type expected,** as record literals do: where a `Key` or a `Zone` is
  expected, `"ireland-{day}"` is a `Key` and `"Europe/Dublin"` a `Zone`. The expectation may come
  later: `let key = "digest-{id}"` is a `Key` when `key` is passed where a `Key` goes, and a literal
  nothing decides is a `String`. A variable declared `String` isn't a `Key`. A zone's name is
  checked against the time-zone database when the program is compiled. Integer literals take a
  fixed-width type the same way.
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

let small = if n < 2 then n else fib(n - 1) + fib(n - 2)   # if is an expression
if n > limit then log.warn("{n} is over the limit")       # `then` for one expression
if retries > 3 {                                           # braces for several statements
  log.warn("giving up")
  return Err(TooManyRetries)
}
for item in items { log.info(item.name) }
for (i, item) in items |> enumerate { log.info("{i}: {item.name}") }
for i in 0..n { log.info("{i}") }
```

- A `?` after a piped call applies to that call: `xs |> find(isEven)?` is `find(xs, isEven)?`.
- Named arguments with defaults: `fn page(size: Int = 50, after: Option(Cursor) = None)`.
- **`name:` alone** passes the variable of that name: `page(size:, after:)` is
  `page(size: size, after: after)`.
- **Results.** A function without `->` returns `()` if its body is a block, and the value of its
  last expression is dropped; `fn double(n) = n * 2` returns its expression's type. A private
  function may leave out parameter types too; they are inferred from its body and its calls, and
  are the same at every call. `while true` without a `break` never ends but by `return`, so it can
  stand where any value is expected.
- **A module-level `let`** binds a value computed once; `pub let dublin: Zone = "Europe/Dublin"`
  exports it, and other modules import it by name like any other declaration.
- **A branch that is one assignment** needs no braces, as one that is one expression doesn't:
  `Ok(text) => found += [text]`, `if late then count += 1`.
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
  `*=`, `/=` and `%=` combine an operation with an assignment, so `found += [text]` appends to a
  list and `out += "\n"` to a string, in place when nothing else refers to them. A `let` never
  changes.
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
    if config.postgres then PgStore.connect(config.url) else MemoryStore.new()

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

A variant pattern names a variant of the matched value's type, so a variant declared in an imported
module needs no module prefix in a pattern, even when another module has a variant of the same name:
`Refused(reason)` in the tour matches zeropod's `Refused`.

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

- A type that wraps one value in a variant of its own name, such as `type WorldId = | WorldId(String)`,
  shows as the value it wraps: `"sleep-{w.id}"` is `sleep-survival`, not `sleep-WorldId("survival")`.
- `Eq`, `Ord`, `Show`, `Hash`, `Save` and `Schema` are derived automatically for records, variants and
  collections of types that have them. Derived `Ord` orders a type's variants as they are declared,
  then by their fields, so `type Priority = Low | Normal | High` has `Low < High`.
- `Secret(a)` deliberately has no `Show` and no `Save`. Printing a secret, or holding one across a
  `!`, is a compile error.
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
  fn double(n) = n * 2                         # inferred: fn(t) -> o where t: Mul(Int, o)
  ```

  The built-in types come with the combinations that make sense: `Int + Int`, `Float + Float`,
  `Duration + Duration`, `Instant + Duration`, `Instant - Instant` (a `Duration`), `Duration * Int`,
  `Size + Size`, `Int / NonZero(Int)`, and `String + String` and `List(a) + List(a)`, which join.
  Nothing converts implicitly: `1 + 2.0` is an error.

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

- **Pure means no `uses`.** A public function without a `uses` clause is pure, and the compiler
  holds it to that: if `area` read a file, the error would say `area uses fs, which its signature
  doesn't list: add uses {fs}`. Editors show the inferred effects of private functions.
- **Callbacks pass their effects to the call.** A function that takes a function lists only its own
  effects; calling it adds the effects of the function passed in, where the call is written. `map`
  is pure, and `paths |> map(fs.readText)` uses `fs`:

  ```eye
  pub fn sizes(paths: List(String)) -> List(Int) uses {fs} =
    paths |> map(fn p: fs.readText(p) |> size)    # fs comes from the lambda, not from map
  ```

  So `retry`, `timeout`, `race`, `parallelMap` and `reserve!` work with any function, and every
  public signature still lists everything its body can do.
- **Functions stored in data state their effects.** A function type without `uses` is pure; one
  kept in a field or a collection, and called later, says what it may do:

  ```eye
  type Handler = { name: String, run: fn(Event) uses {net Notify.send} }
  ```

  In a function's return type, the `uses` after a function type belongs to that function type.

**`!` in jobs.** Inside a job, every call with an outside effect (`net`, `fs`, `process`, `clock`,
`random`, `ffi` and the rest) is a `!` call: it runs once and its result is recorded, so a resumed run
never repeats it and sees the same values. Entity calls are `!` calls too (`w.read!()`,
`w.markAsleep!()`): each commits in the same transaction as its checkpoint (section 15). Calls that
only compute are ordinary. Outside jobs the same functions are called without `!` (section 13).

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
- **`eye run` keeps a program's jobs going.** It runs `main`, if the program has one, every time
  it starts; then, if the program declares jobs, it stays up: sleeping runs wake when their time
  comes, schedules fire, and it stops when nothing is left to wait for, or on Ctrl-C or SIGTERM.
  Entities, runs and events are kept in the program's journal (`.eye/journal`), so a restart
  carries on where the last one stopped: a run that was due while the program was down runs at
  once, and a schedule that missed firings fires once, for the latest. Because `main` runs on every
  start, it starts long-lived work through an entity, which exists once (`Alarm("a").arm()` starts
  its job only if it isn't armed), or leaves it to schedules. `eye runs` lists the runs in the
  journal: their status, what each waits for and until when, and their steps.
- **Modules.** A module is a directory: its `.eye` files share one namespace and one set of
  imports. A program is the module in its directory and the modules it imports. `import lab.notify`
  names the directory `lab/notify` below the program's directory; its public names are then used as
  `notify.Delivery`, or brought in by name with `import lab.notify.{Delivery}`.

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
  (section 7). A `state` field such as `devices` is short for `self.devices`, and a method is
  called by its name too: `unpin(p)` in another method is `self.unpin(p)`, in the same
  transaction. A pipe still calls a function, never a method: `shapes |> add(s)` is the collection
  function `add`, even in an entity that has a method of that name. The state is
  loaded before the call and saved when the transaction commits; nothing outside the method sees a
  change until the commit.
- **Methods never wait.** They can't use `!`, so they can't call other services; those calls
  belong in jobs. A method can start a job (`wake.start(id)`), and the job begins only if the
  transaction commits.
  Because entity methods have no outside side effects, the runtime can safely retry one after a
  conflict.
- **Reading from a job:** `w.read!()` returns a snapshot, whose type is the entity's name
  (`fn due(me: Traveller)` takes one): the entity's state, its identity and a `version` field,
  which counts the transactions that changed the entity: 0 when it is new, then one more for each.
  Snapshots go stale, and commands can name the version they were based on (see `@fresh` in
  section 17). Outside a job, in a test for instance, `w.read()` gives a snapshot without `!`.
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
  embedders use; on its own it doesn't survive a restart. For one machine, the journal (question 9)
  appends each commit to a file and syncs it before the commit counts, which is what `eye run`
  uses. The Postgres store survives restarts and serves several runners. Both implement the `Store` trait from the standard library, and programs
  don't change when the store does.
- **Calls to other services** carry a key. If the outcome is `Uncertain`, `.reconcile!()` asks the
  owner with the same key until it knows. Nothing is resent automatically.
- **Waiting constructs** (all library functions, all marked with `!`):

  | Call | Waits for |
  |---|---|
  | `sleep!(20 min)`, `sleepUntil!(07:45 in "Europe/Dublin")` | time |
  | `poll!(every: 1 min, within: 6 h, fn { miniflux.ready!() })` | a service that can't say when it is ready; `false` if time runs out |
  | `approval!("delete 3 backups", from: operator, within: 3 days)` | a human; returns `Approved(Approval) \| Declined \| Expired` |
  | `next!(notify.delivered, where: fn e: e.key == k, within: 1 h)` | an event; `None` if time runs out |
  | `reserve!(1 GiB, fn { … })` | memory from the host, released when the function returns |
  | `lock!("ci", for: 30 min, fn { … })` | a named lock that records its owner and expiry |
  | `run.join!()` | a job started earlier: `Completed(value)`, or `Failed(fault)` if it stopped first |
  | `all!([a, b])`, `first!([a, b])` | every one, or whichever finishes first, of several waits |

- **`next!` misses nothing.** It counts every matching event published since the run's previous
  step, so an answer that comes while the run is still sending its question, or sleeping, is found.
  `poll!` keeps only its outcome, so a long poll doesn't grow the run's history.
- **A run's outcome** is `Outcome(a) = Completed(a) | Failed(Fault)`, from the prelude. A fault
  (`DeadlinePassed`, `OverBudget`, `OutOfMemory`, `Unavailable`, `Unreachable`, `Killed`) can't be
  caught where it happens, so code that must report one starts the work as a child run and joins
  it:

  ```eye
  fn written(day: Date) -> Result(Article, DigestError) =
    match write.start(day).join!() {
      Completed(result) => result
      Failed(fault) => Err(Faulted(fault))
    }
  ```
- **Starting jobs:** `irelandDigest.start(day)` returns a `Run(a)`. It works from entity methods (as
  part of the transaction) and from jobs, where it starts a child run. Scheduled starts take
  `after:` or `at:`: `sleepWhenIdle.start(w, after: 20 min)`,
  `irelandDigest.start(day, at: 07:40 in "Europe/Dublin")`.
- **Attributes:**

  | Attribute | Meaning |
  |---|---|
  | `@on(trigger)` | `daily(…)`, `weekdays(…)`, `every(15 min)`, `cron("…", in: zone)`, `enters(view)`, `leaves(view)`, `event(contract.event)`. A schedule passes the job the date of the firing in the schedule's zone, if the job takes one; a view, the entity's `Ref`; an event, the event |
  | `@memory(size)` | budget admitted before the run starts; the runtime enforces it |
  | `@priority(p)` | `interactive`, `normal`, `background` or `batch`; under memory pressure, lower priorities pause at their next checkpoint |
  | `@deadline(time or duration)` | after this the run ends as `Failed(DeadlinePassed)` |
  | `@concurrency(n, by: param)` | at most `n` runs at once, or per value of the parameter named; a waiting run keeps its place, so `@concurrency(1)` asks one question at a time, in order. A scheduled start that finds the limit reached is skipped |
  | `@limit(n, per: duration, by: param)` | rate limit, optionally per value of a parameter: `@limit(1, per: 1 s, by: chat)` |

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

  query receipt(key: Key) -> Option(Delivery)
  query identity(user: UserId) -> Identity

  @key(receipt)
  event delivered: Delivered on "notify.events"
}

type Delivery =
  | Accepted(receipt: ReceiptId, at: Instant)  # the outbox has it; quiet hours may hold it
  | Pending(receipt: ReceiptId)
  | Refused(reason: Refusal)
```

- **`import notify` is a typed edge.** Port, network policy, credentials and the allowed-caller check
  are derived from it, so a wrong port or an unapproved caller is a compile error.
- **Commands** return a variant type, and every command can also return `Uncertain(key)`: its
  reply was lost, so the other service may or may not have acted. Until it is reconciled, or a
  `match` handles `Uncertain(_)`, the result can't be passed where the declared type is expected. With `@reconcile(query)`,
  `.reconcile!()` resolves it by asking the query, which returns `Option` of the command's outcome:
  `Some` is the outcome, and `None` means the command never arrived, so it is sent again with the
  same key, which can't do it twice. A command without `@reconcile`, such as one to a service that
  takes no idempotency key, has no `.reconcile!()`: its caller matches `Uncertain(key)` and decides,
  usually by stopping for a person. Nothing is ever resent on a guess. A service that keeps what
  each key did, as Notify does, names the command itself: with `@reconcile(send)`, reconciling
  sends it again with the same key, and the service answers without doing it twice.
- **Never sent is not uncertain.** A request that never reached the service (the connection was
  refused, or the name didn't resolve) can't have done anything, so it is sent again, as a query
  would be. Only a request that went out and whose reply was lost (a timeout, a broken connection,
  or a 5xx answer to a command) is `Uncertain(key)`.
- **Queries** are safe to repeat: one that can't reach its service is retried, while the run
  waits, and after a minute the run ends as `Failed(Unavailable(…))`. A query whose failures the
  caller handles declares them in its result: `query page(url: String) -> Result(String, FetchError)`,
  or, over HTTP, `Result(T, http.HttpError)`, which gets the failures instead of retrying.
- **Events** are published with `notify.delivered.publish(d)`: from an entity method or an
  operation when the transaction commits, and from a job as a `!` step. `@key(field)` on an event
  names the field that tells one event from another, so an event delivered twice starts one run of
  each `@on(event(…))` job.
- **`@fresh`** on a command adds a required `basedOn: Version` argument and `Stale`/`Expired`
  outcomes, so an old command can't act on newer state.
- **`@destructive`** on a command adds a required `Approval` argument.
- **A contract's operations are its module's functions.** A module that declares one contract offers
  its commands and queries by name: `notify.send!(…)` for `contract Notify { command send … }`.
- **Serving a contract.** A program written in Eyelang serves a contract with `impl Contract { … }`:
  one function per operation, named after it, with its parameters in order and `key` last for a
  command with `@key`. Types and defaults come from the contract. Each runs as one transaction, as
  an entity method does, so it can't wait, and it may start jobs. A key can name an entity, so a
  repeated command finds the work it already did:

  ```eye
  impl notify.Notify {
    fn send(to, text, key) = Message(key).queue({ text, to })
    fn receipt(key) = Message(key).delivery()
  }
  ```
- **Values as JSON.** Every value that leaves a program as JSON has one shape: in requests and
  replies over HTTP, in recordings, in model schemas (section 19), in `toJson`, and in the types
  generated for other languages, where a variant type becomes a discriminated union.
  - A record is an object, with every field.
  - A type whose variants have no fields is a string: `Normal` is `"Normal"`.
  - Any other variant is an object tagged with `"kind"`, with its fields:
    `{ "kind": "Accepted", "receipt": "r-41", "at": "…" }`. A positional field is `"value"`, or
    `"value1"`, `"value2"`, … when there are several.
  - A type that wraps one value, such as `| ReceiptId(String)`, is that value.
  - `Some(x)` is `x`, and `None` is `null`; a missing field of an `Option` type is `None`.
  - Lists, sets and tuples are arrays, and a map with string keys is an object.
  - An `Instant` is an RFC 3339 string, a `Date` `"2026-10-12"`, a `Duration` a number of
    milliseconds, and a `Size` a number of bytes.

  Fields that a type doesn't declare are ignored when a value is read, so a service can add some
  without breaking its callers.
- **Over HTTP.** Attributes say how a contract's operations go over HTTP. The address and the
  credentials come from the configuration, never from the code:

  ```eye
  @http(names: snake)                       # dedupeKey is dedupe_key, Normal is "normal"
  @auth(bearer: "notify-token")             # Authorization: Bearer <secret("notify-token")>
  contract Notify {
    @http("POST /v1/send", status: { 200: Accepted, 202: Pending, 400: Refused, 403: Refused })
    @key(required, header: "X-Actions-Idempotency-Key", field: "dedupe_key")
    @reconcile(send)
    command send(event: String, title: String, priority: Priority = Normal,
      to: Option(List(UserId)) = None) -> Delivery
  }
  ```

  - `@http("METHOD /path")` on an operation gives its request. A path segment `:name` takes the
    argument of that name, `GET` and `DELETE` send the other arguments as query parameters (a list
    as the parameter repeated), and the other methods send them as a JSON object.
  - Arguments are sent with their defaults filled in; a `None` is left out, which is how JSON says
    a value is absent.
  - `@key(…, header: "…")` names the header that carries the key (`Idempotency-Key` without one),
    and `field: "…"` puts it in the body too.
  - `status: { 202: Pending }` reads a reply with that status as that variant, from the fields of
    the body. Any other success is read as the operation's result type.
  - `@auth(bearer: "secret")`, or `@auth(header: "X-Auth-Token", secret: "secret")`, sends a
    secret with every request.
  - A refused request (a 4xx that `status:` doesn't name) ends the run with a fault that shows the
    reply: it is a bug in the program or its configuration, not an outcome to handle.

  A program finds a contract's service at the URL in the environment variable named after its
  module (`NOTIFY_URL` for `notify`, `LAB_NOTIFY_URL` for `lab.notify`). A program that serves a
  contract itself calls its own implementation, unless that variable is set.
- **Fakes come from recordings.** `with fakes(notify, from: "receipts/")` replays real recorded
  exchanges, checked against the contract. Reading a field that no recording contains is a compile
  error, unless it is marked `@unwitnessed("why")`. A recording is a JSON file per contract module,
  named after it (`receipts/notify.json`), that lists exchanges by operation and key, each result
  in the shape above:

  ```json
  { "contract": "Notify",
    "exchanges": [{ "op": "send", "key": "ireland-2026-10-10",
                    "result": { "kind": "Accepted", "receipt": "r-41", "at": "2026-10-10T07:40:02Z" } }] }
  ```

  A command's lost reply is recorded as `{ "kind": "Uncertain", "key": "…" }`. Recordings written
  before this shape was settled, with a variant as its name's only key (`{ "Accepted": {…} }`), are
  still read.
- **Services without a contract,** such as web pages, are read with the standard `http` module:
  `http.get!(url, headers: { "Accept": "text/html" }, via: proxy)` returns
  `Result(Response, HttpError)`, with the status, the headers and the body as text. Any answer is
  `Ok`, whatever its status; `Err` is `NoAnswer(reason)` or `TimedOut`. `via` is the URL of an HTTP
  proxy to go through.
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

## 19. AI models and agents

Programs that call language models, and jobs that run coding agents, use the pieces the rest of the
language already has: types, effects, `!` steps and recordings. The standard library adds two
modules, `ai` and `agents`. Syntax that helps here and elsewhere is welcome: `##` doc comments
describe schemas and tools as well as documentation, and prompts are written as multi-line
strings (section 4).

**Typed model calls.** A call names the type it wants back. The compiler derives a JSON Schema from
that type, the provider is asked for structured output matching it, and the reply is checked and
decoded into an Eyelang value. Code never sees unchecked text.

```eye
import ai

## How one morning's news is summarised.
type Digest = {
  ## One sentence, at most 25 words.
  headline: String,
  topics: List(Topic),
  urgency: Urgency,
}

type Topic = { name: String, items: List(String) }
type Urgency = | Calm | Busy | Breaking(reason: String)

@budget(usd: 0.50)
job summarise(items: List(Item)) -> Result(Digest, AiError) {
  ai.generate!(
    model: "claude-sonnet-5-5",
    system: """
      You summarise Irish news for a morning digest.
      Use only facts in the input. Never guess dates, numbers or rules.
      """,
    prompt: items |> map(fn i: "- {i.title}") |> join("\n"),
  )
}
```

- **Schemas come from types.** Records, variants, lists, maps with string keys, `Option`, tuples,
  numbers, strings, booleans, dates and times have a derived `Schema` trait. The schema describes
  the JSON shape of section 17: a record becomes an object with every field required (`Option`
  fields may be null), a type whose variants have no fields an enum of strings, and any other
  variant a tagged union, `{"kind": "Breaking", "reason": "…"}`; `##` doc comments become
  descriptions. A type that holds a `Secret`, a `Handle` or a function has no `Schema`, so it can't
  be asked for.
- **The expected type picks the schema.** `ai.generate` returns `Result(a, AiError) where a:
  Schema`, so the type comes from where the result goes: here, the job's return type. When nothing
  says, write it: `let d: Digest = ai.generate!(…)?`.
- **Replies are checked.** If a reply doesn't match the schema, the call asks once more with the
  problems listed, and then returns `Err(Malformed(reply, problems))`.
- **Providers are contracts.** `ai` declares contracts for Anthropic, OpenAI and OpenAI-compatible
  servers. A call has the effect `net` on its provider, the key comes from `secret`, and the
  provider follows from the model's name unless `provider:` says otherwise. The reference
  interpreter speaks the OpenAI-compatible chat completions protocol with structured output, which
  gateways such as the lab's offer for every model: `AI_URL` is the gateway's base URL, and
  `secret("ai-key")` its key, if it needs one.
- **In a job, every model call is a `!` step.** Its reply is recorded, so a resumed run neither calls
  the model again nor pays twice, and it sees the same answer.
- **Tests replay recordings.** `with fakes(ai, from: "recordings/")` makes model-backed tests
  deterministic and free, and a recording that no longer matches the schema fails its test.
  `eye test --live` runs them against the real models.
- **Budgets.** `@budget(usd: 5, tokens: 2_000_000)` on a job or function works like `@memory`: a call
  that would go over it isn't sent, and the run ends as `Failed(OverBudget)`.

**Tools are functions.** Eyelang functions passed as `tools:` become the model's tools. A tool's
parameter types give its input schema and its `##` comment its description, and the runtime runs
the function when the model calls it.

```eye
## Find open issues that mention a phrase.
fn searchIssues(phrase: String, limit: Int = 10) -> List(Issue) uses {net GitHub.search} {
  github.search(phrase, limit)
}

## Add a label to an issue.
@destructive
fn label(issue: IssueId, name: String) uses {net GitHub.label} {
  github.label(issue, name)
}

type Triage = { duplicateOf: Option(IssueId), labels: List(String) }

job triage(report: String) -> Result(Triage, AiError) {
  ai.generate!(model: "claude-opus-5-5", prompt: report, tools: [searchIssues, label])
}
```

- **What a model may do shows up in the signature.** The tools' effects are the call's effects, so
  `triage` uses `net GitHub.search` and `net GitHub.label`, and adding a tool shows up in review.
- **Some calls need a person.** When the model calls a `@destructive` tool, the job pauses, as at
  `approval!`, until someone approves that one call.
- In a job, each tool call is recorded like any other step.

**Agents are sessions owned by jobs.** A coding agent such as Claude Code or OpenAI Codex works for
minutes or hours, asks for permission and can be interrupted. `agents` drives them through their
SDKs as sessions that a job owns. The job holds only the session's ID, so it waits without holding
memory and picks the session up again after a restart.

```eye
import agents.{claude, codex}

type Review = { verdict: Verdict, findings: List(Finding) }
type Verdict = | Approve | RequestChanges(reason: String)
type Finding = { file: String, line: Int, message: String }

@on(event(github.pullRequestOpened))
@budget(usd: 5)
@deadline(2 h)
job review(pr: PullRequest) -> Result(Review, AgentError) {
  let session = claude.start!(
    prompt: "Review {pr.url}. Report real bugs only.",
    workdir: checkout!(pr.head),
    tools: [readFiles, runTests],
  )
  let review: Review = session.result!()?
  if review.findings != [] {
    codex.start!(prompt: "Fix these: {review.findings}", workdir: checkout!(pr.head),
      tools: [editFiles, runTests]).result!()?
  }
  Ok(review)
}
```

- `start!` begins a session, `send!` adds a message, and `result!()` waits for the final answer,
  decoded into the expected type through the harness's structured output.
- `tools:` lists what the agent may use. Eyelang functions are served to the agent over MCP, and
  their effects become the job's, as with model calls. Permission requests from the agent arrive
  as events; code can answer them, or hand them to a person with `approval!`.
- Several agents run side by side as child runs, joined with `all!` or `first!`.

## 20. Time, secrets, tests

**Time.** Schedules carry their zone: `daily(07:40, in: "Europe/Dublin")`, and `weekdays(…)` is the
same from Monday to Friday. `every(15 min)` fires at each multiple of its interval since midnight
UTC: :00, :15, :30, :45. A time that a daylight-saving change skips fires as far after the change
as it was meant to be (`01:30` on the night the clock jumps from 01:00 to 02:00 fires at 02:30),
and a time a change repeats fires the first time it comes, so there are never two UTC crons and
an hour check. (The policies are to be choosable, as `onGap:` and `onOverlap:`; these two are the
defaults and the only ones so far.) A schedule counts from when the program first starts; each
firing is saved with the run it starts, so a restart neither repeats nor skips one. Dates in
another zone are asked for by name, never taken from the host's clock:

| Call | Gives |
|---|---|
| `at.date(in: dublin)`, `at.time(in: dublin)` | the date or time of day the clock in that zone shows at instant `at` |
| `08:00.next(in: dublin, after: at)` | the first instant after `at` when that clock shows 08:00 |
| `day.at(07:40, in: dublin)` | the instant that day at that time |
| `day.format("%A %-d %B")` | the date written out: `Monday 12 October` |
| `at.unix()` | seconds since 1970-01-01 00:00 UTC, which many APIs take |

**Secrets and configuration.** A secret is read by name when the program needs it: from the
environment variable of that name in upper case, with `-` and `.` as `_`, or from the file of that
name in the directory `$EYE_SECRETS`, where Kubernetes and systemd put mounted secrets. A contract
sends its secret with `@auth` (section 17), so code rarely holds one.

```eye
let token: Secret(String) = secret("telegram-bot-token")  # TELEGRAM_BOT_TOKEN
log.info("token is {token}")               # compile error: Secret has no Show
```

Configuration that isn't secret, such as a proxy's address, comes from `env.get("VPN_PROXY_URL")`,
an `Option(String)`, with the effect `env`.

**Tests.** A `test` block is a declaration. `eye test` runs the tests of every module of the
program, the modules it imports from its own directory included. `with` applies a handler to the rest of the block:
`fakes(…)`, `clock(…)` with `advance(…)`, `memory(…)`. In tests, jobs run against an in-memory
store, so every run is deterministic and fast. The clock starts at `clock(…)`'s time (2026-01-01
00:00 UTC without one) and moves only when the test calls `advance(…)`; each run that wakes on the
way runs before `advance` goes on, and so does each schedule that fires, if its job is in the
test's own module (a test of the reminders doesn't start the NetBird sync). Jobs started by a call run as soon as the call returns, until
they finish or wait, and so do runs that an event a test publishes starts or wakes:
`notify.answered.publish(answer)` stands in for a person pressing a button, and
`approve("delete 3 backups")` or `decline(…)` answers the `approval!` whose text contains it.
`expect(cond)` fails the test, and for a comparison shows the value it got:
`expected w.phase == Asleep, but w.phase is Running`.

## 21. What happens to Eyelang 1

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
| `parser/`, `ast/` | ~1,100 lines | **Replace** | Needs a lossless syntax tree with error recovery (see section 22) |
| `eval/`, `object/` | ~1,600 lines | **Replace** | The new reference interpreter runs typed IR with entities and jobs; the current evaluator can't be adapted to that |
| `cli/`, `main.go` | ~120 lines | **Replace** | Becomes the `eye` command: `run`, `build`, `test`, `fmt`, `check`, `repl`, `why` |
| `repl/lineedit.go` | 318 lines | **Keep** | The line editor and history work and aren't tied to the language |
| `repl/repl.go` | 125 lines | Adapt | Reuse the loop around the new front end |
| `examples/*.eye` | 6 files | Replace | Eyelang 1 syntax; Eyelang 2 examples double as test fixtures |
| `.github/workflows/ci.yml`, `Taskfile.yml`, `go.mod` | — | Keep | Update tasks as the packages change |

Before deleting anything, tag the current state as `eyelang-1` so it stays easy to find and run.

## 22. Implementation plan

**Parsing: the grammar is the spec, the compiler's parser is hand-written.** Decided 2026-10-10.

- **`tree-sitter-eyelang/grammar.js` is the executable grammar.** It is the canonical definition of
  the syntax, and section 23 is a readable summary of it. It also drives highlighting in editors
  (Neovim, Helix, Zed, and VS Code through an extension). A small external scanner in C decides
  which newlines end statements, tells a block's `{` from a literal's, and reads string text.
- **The compiler parses by hand,** in Go, with recursive descent and Pratt parsing for expressions
  (precedence table in section 23). That keeps cgo and runtime libraries out of the compiler and keeps
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
5. Typed IR, plus a reference interpreter on that IR. The interpreter resumes a job by running it
   again from the start: each step it already took returns its recorded result, so nothing outside
   happens twice, and output from those steps isn't printed again. Used by the REPL and tests, with entities
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

## 23. Grammar sketch

A readable summary of [`tree-sitter-eyelang/grammar.js`](tree-sitter-eyelang/grammar.js), which is
the definition. `sep` is `;` or a newline that ends a statement (section 4); `{` marked *block* is
the brace that opens a block. Lists in `( )`, `[ ]` and literal braces allow a trailing comma.
Every `eye` example in this document is a whole file or the statements of a function body, and CI
parses each one with both parsers.

```ebnf
file       = { sep } [ item { sep item } { sep } ] ;
item       = import | fn | extern | job | entity | view | type | trait | impl | contract | test
           | [ "pub" ] let ;
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
lambda     = "fn" [ ident | "_" | params ] ( ":" expr | block ) ;
unary      = ( "-" | "!" ) unary | postfix ;
postfix    = primary { args | "!(" args… | "." name | "[" ( expr | slice ) "]" | "?" } ;
slice      = [ expr ] ( ".." | "..=" ) [ expr ] ;
args       = "(" [ arg { "," arg } ] ")" ;
arg        = [ ( ident | "in" | "for" ) ":" ] ( expr | "mut" place )
           | ident ":" ;                                     (* `key:` is `key: key` *)
primary    = ident | Ident | "_" | literal | "(" expr ")" | "(" expr "," expr { "," expr } ")"
           | "[" [ expr { "," expr } ] "]"
           | "{" "}" | "{" field { "," field } "}" | "{" expr "with" field { "," field } "}"
           | "{" key ":" expr { "," key ":" expr } "}"
           | "if" expr block [ "else" ( block | if ) ]
           | "if" expr "then" branch [ "else" branch ]
           | "match" expr "{" { arm ( "," | sep ) } "}" ;
field      = ident ":" expr | ident ;
key        = string | int | date | time | "(" expr ")" ;
arm        = pattern [ "if" expr ] "=>" ( block | branch ) ;  (* "{" name ":"|","|"with" is a record *)
branch     = expr | place assign expr ;
literal    = int | float | char | string | "true" | "false" | "()" | number unit
           | date | time | date time "in" zone | time "in" zone ;
zone       = string | ident ;
string     = '"' { text | escape | "{" expr "}" } '"'
           | '"""' newline { text | newline | escape | "{" expr "}" } newline indent '"""' ;

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

## 24. Open questions

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
8. **Multi-line strings:** decided on 2026-10-10: `"""`, with the closing line's indentation as
   the margin (section 4).
9. **Durability without a database:** decided on 2026-10-10: the journal. Each commit (entity
   writes, runs, events, schedule firings, event keys seen, commands whose reply was lost) is one
   line of JSON appended to `.eye/journal` and synced before it counts. A commit that can't be
   written, on a full disk or otherwise, changes nothing: the journal is cut back, memory isn't
   touched, and the run stays at its last checkpoint and is tried again later. The journal is
   rewritten as one snapshot when it is opened and when it grows past four times its live data,
   and a 4 MiB reserve file beside it is given up when a write fails, so that compaction can still
   finish. Events no unfinished run can still see are dropped, an event's key is forgotten after
   30 days, and a run that finished more than 7 days ago keeps a summary (job, arguments, outcome,
   times, step count) instead of its steps; waiting runs and unsettled lost commands are never
   dropped. Still open:
   - The retention times are fixed; should a program set them, and should `eye runs` show what
     was summarised?
   - A run that never ends, such as the lab's reminders watcher (`while true { …; sleep!(45 s) }`),
     keeps every step: four or more per loop, so 7,700 or more a day per person, each wake
     replaying all of them, so its cost
     grows with the square of its age. `eye run` warns past 1,000 steps and `eye runs` flags such
     runs. The fix is the model section 15 describes, saving the live variables at each `!`
     instead of the history, which compiled jobs will do; until then, a loop can end by starting
     its job again (`watch.start(t)`) so that each run stays short. Should the language say so,
     or offer `continue`-as-new for jobs?
   - One process per journal (a file lock); several runners need the Postgres store.
10. **Streaming model output:** `ai.generate!` returns a whole, checked value. Interfaces that show
    a reply as it arrives want partial results; should `ai.stream` yield an `Iter` of partial
    values, checked as far as they go?
11. **Prompts as text:** prompts are multi-line strings now (question 8). Should long prompts also
    live in template files next to the code, loaded and checked when the program is compiled?
12. **Request-scoped values,** such as a trace ID or the current user. Go passes them in a
    `context.Context`. Eyelang has no such parameter: cancellation and deadlines come from
    structured concurrency, and capabilities from effects. Today such values are passed as
    arguments. Should `with` provide a value to the rest of a block, as it provides fakes and clocks
    in tests, with the dependency shown in signatures as an effect?
13. **`eye fmt` and the two forms of `if`.** Should the formatter rewrite a one-line
    `if c { a } else { b }` to `if c then a else b`, so that each `if` has one canonical form?
14. **`if let`.** Writing the lab, `if let Some(a) = answer then return Err(Taken(a))` came to hand
    twice, where today a `match` with a `None => ()` arm does the same in four lines. `let … else`
    covers the opposite case. Is a third form worth it, or does `match` stay the one way?
15. **Typed choices.** A question's buttons are strings (`choice: "boarded"`), and the answer is
    matched as a string. Could a variant type give the choices, with labels from `##` comments, and
    the answer come back as that type?
16. **What a fake received.** A test can't see which calls a fake answered, so the lab's digest
    records the keys it sent with in an entity to make sending testable. Should a test be able to
    ask `fakes(notify).sent(…)`, or fail when a recording has exchanges no call used?
17. **Where approvals go.** Tests answer `approval!` with `approve(…)` and `decline(…)`. How does a
    deployment say where requests go and who may answer (the lab would use Notify's buttons)?
18. **Contracts shared by programs.** The lab is one program so that its modules share the Notify
    contract; separate programs would each need a copy. This belongs with packages (question 5).
19. **Which actions are effects?** Calling a service, waiting, touching entities, the clock and
    standard output are. Are logging, publishing an event and starting a job effects too, to be
    stated with `uses`? (`publish T` is in the table of section 12; the checker doesn't infer it yet.)
20. **Collection functions' types.** `map`, `filter` and `size` work on lists, sets, maps, ranges and
    `Option`s; `take` on strings too. Until traits are checked, the checker has a rule for each. Is
    the final form a `Collection` trait, or functions named per kind?
