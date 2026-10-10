# Eyelang 2 — language design (draft)

Status: **draft**. These decisions come from design sessions on 2026-10-10. None of it is implemented
yet: the code in this repository is Eyelang 1, a Monkey-style tree-walking interpreter that served
as a testing ground (see [What happens to Eyelang 1](#16-what-happens-to-eyelang-1)).

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
7. [Pattern matching](#7-pattern-matching)
8. [Errors: Result only](#8-errors-result-only)
9. [Traits](#9-traits)
10. [Effects and signatures](#10-effects-and-signatures)
11. [Entities](#11-entities)
12. [Jobs and waiting](#12-jobs-and-waiting)
13. [Views and triggers](#13-views-and-triggers)
14. [Contracts: talking to other services](#14-contracts-talking-to-other-services)
15. [Time, secrets, tests](#15-time-secrets-tests)
16. [What happens to Eyelang 1](#16-what-happens-to-eyelang-1)
17. [Implementation plan](#17-implementation-plan)
18. [Grammar sketch](#18-grammar-sketch)
19. [Open questions](#19-open-questions)

## 1. Goals

- **A good general language first.** Immutable values, variants with exhaustive matching, traits,
  inferred types, and no nulls, exceptions or panics.
- **Waiting is cheap.** A job that is waiting is a row in a database, not a process or a thread.
  Resuming one takes milliseconds.
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
| Wait marker | A **`!` suffix** on every call that may pause: `sleep!(20 min)`, `notify.send!(…)` | Every save point is visible in code review |
| Checkpoints | Every `!` call in a job is also a **checkpoint**: its result is saved and it runs once | No separate `step` keyword is needed |
| Entity lifecycle | **Virtual**: an entity exists on first use. Ending one with protected state needs an `Approval` | No create/exists dance; deletion always involves a human |
| Storage | Behind a **runtime interface**. Postgres first; an embedded store is possible later; never an in-memory store like Redis for durable state | Memory is the scarce resource, and durability is already solved there |
| Types | **Static.** Public items carry full signatures, including effects; everything inside bodies is inferred | Changes to types and permissions show up in the diff |
| Errors | **Result only**: no exceptions, no panics. Runtime faults become outcomes of the run | Every failure is visible in a type |
| Arithmetic | **Total types**: `Int` has arbitrary precision; `/` and `%` need a `NonZero` divisor; indexing returns `Option` | Removes the runtime errors that "Result only" would otherwise need |
| Consistency | **Atomic inside eyelang**; keyed commands and **reconcile** at the boundary with other services | One database makes real transactions cheap; outside services own their own state |
| Queries | **Pipelines** (`filter`, `map`, …), compiled to SQL when the source is an entity set | The same code as for lists |
| Reactions | **Views trigger jobs**: `@on(enters(view))`, `@on(leaves(view))` | Replaces most polling cron jobs |
| Evolution | Adding a field with a default is automatic; anything else needs a **`migrate from N`** function; deploy is refused if any stored version has no path to the current one | Stored data outlives code |
| Shared behaviour | **Traits** with explicit `impl`; equality, ordering, printing, hashing and saving are derived for data types | One mechanism for generic code |
| Declarations | **Typed attributes** (`@on`, `@memory`, `@priority`, …) on jobs, entities and contracts | One general mechanism instead of special syntax per feature |
| Blocks | **Braces** | Keeps parsing simple; no whitespace pitfalls |
| Pipes | Data-first: `x |> f(a)` is `f(x, a)`; `_` places the value elsewhere: `x |> f(a, _)` | Matches method-call intuition |
| Memory | Reference counting with in-place reuse of uniquely owned values; immutable values; entities referenced by ID | No GC pauses, and no reference cycles by construction |
| Parsing | A **tree-sitter grammar is the spec**; the compiler's parser is hand-written; a conformance suite fails CI if the two disagree | One definition of the syntax, editor support for free, and compiler error messages under our control |

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
      start wake(id)                 # starting a job is part of this transaction
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
      w.markAsleep!()                              # exactly once: same database as the journal
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

- **Comments:** `#` to the end of the line.
- **Identifiers:** a letter or `_`, then letters, digits and `_`. Identifiers can no longer end in
  `!` or `?` (Eyelang 1 allowed `update!` and `ok?`): `!` now marks waiting calls and `?`
  propagates errors.
- **Literals:**

  | Kind | Examples |
  |---|---|
  | Int (arbitrary precision) | `0`, `42`, `1_000_000` |
  | Float (64-bit IEEE) | `1.5`, `0.25` |
  | String, with interpolation | `"hello"`, `"ireland-{day}"`, `"\{literal brace}"` |
  | Bool | `true`, `false` |
  | Duration | `500 ms`, `20 min`, `3 h`, `2 days` |
  | Size | `512 B`, `64 KiB`, `64 MiB`, `6 GiB` |
  | Time of day | `07:40`, `23:59:30` |
  | Date | `2026-10-10` |
  | Instant | `2026-10-10 07:40 in "Europe/Dublin"` |

  A number followed by a unit name (`ms`, `s`, `min`, `h`, `day`, `days`, `week`, `weeks`, `B`,
  `KiB`, `MiB`, `GiB`) is a Duration or Size. A date or time of day is written without spaces;
  `2026 - 10 - 10` is still subtraction.
- **Keywords:** `let var fn pub return if else match for in while break continue type trait impl
  entity state job view contract import test with start true false`. A few more words are keywords
  only in one position: `command`, `query`, `event` inside contracts; `migrate`, `from` inside
  entities; `after`, `at` after `start`; `uses` in signatures.
- **Attributes:** `@name` or `@name(args)` before a declaration or member.

## 5. Values and types

- **Primitives:** `Int`, `NonZero(Int)`, `Float`, `Bool`, `String`, `Bytes`, `()`.
- **Time:** `Instant`, `Date`, `Time`, `Duration`, `Zone`, `Schedule`. **Memory:** `Size`.
- **Collections:** `List(T)`, `Map(K, V)`, `Set(T)`. All immutable. An update that holds the only
  reference happens in place at run time, without changing the meaning.
- **Records:** `type User = { name: String, age: Int }`, built with `{ name: "Ann", age: 3 }` and
  updated with `{ user with age: 4 }`.
- **Variants:** `type Grant = Admitted { revision: Revision } | Pending { receipt: ReceiptId } | Refused { reason: Reason }`.
- **Built-in variants:** `Option(T) = Some(T) | None` and `Result(T, E) = Ok(T) | Err(E)`.
  There is no null.
- **Entity references:** `Ref(World)` is an ID, not a pointer. It can be stored, sent and saved
  anywhere, and it can't be confused with `Ref(Player)`.
- **Special types:** `Secret(T)` (no printing, no saving), `Approval` (only a human approval flow
  can create one), `Key` (an idempotency key), `Run(T)` (a started job).
- **Maps** use string or expression keys: `{"a": 1}`. Records use identifier keys: `{a: 1}`.

## 6. Bindings, functions and pipes

```eye
let x = 5                    # immutable, block-scoped; a later `let x` shadows it
var total = 0                # mutable local; closures can't capture a `var`
total = total + x

pub fn area(w: Float, h: Float) -> Float = w * h       # public: full signature
fn double(n) = n * 2                                     # private: inferred

let inc = fn n: n + 1                                    # lambdas
let add = fn (a, b): a + b
let show = fn item {
  let label = item.name
  "<{label}>"
}

let sizes = urls |> map(fetchSize) |> filter(fn s: s > 0)   # data-first pipe
let diff = 2 |> sub(10, _)                                   # _ places the value: sub(10, 2)

if n < 2 { n } else { fib(n - 1) + fib(n - 2) }          # if is an expression; no parentheses
for item in items { log.info(item.name) }
```

- Named arguments with defaults: `fn page(size: Int = 50, after: Option(Cursor) = None)`.
- No partial application by argument count. Use `_` or a lambda.

## 7. Pattern matching

```eye
match grant {
  Admitted { revision } => use(revision)
  Pending { receipt } if receipt.age > 1 h => escalate(receipt)
  Pending { receipt } => wait(receipt)
  Refused { reason } => report(reason)
}

let { name, age } = user                     # destructuring
let [first, ..rest] = items else { return Err(Empty) }
```

Every `match` must be exhaustive. Adding a variant to a type makes each `match` that doesn't handle
it a compile error, which is how a new outcome from another service reaches every caller.

## 8. Errors: Result only

- Expected failures are values: `Result(T, E)` and domain variants such as `Refused` and
  `Uncertain`. Postfix `?` returns the error to the caller: `let p = Player(id).profile()?`.
- There are no exceptions and no panics. Library functions are total: `xs[0]` and `first(xs)`
  return `Option`, and `parseInt(s)` returns `Result`.
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
- **Runtime faults** (out of memory, over budget, killed, deadline passed) are not errors that code
  can throw or catch. They end the run, and whoever started it sees `Failed(fault)` as the run's
  outcome.

## 9. Traits

```eye
trait Named {
  fn name(self) -> String
}

impl Named for Player {
  fn name(self) = self.nick
}

fn label(x: Named) -> String = "<{x.name()}>"     # any type with an `impl Named`
```

- `Eq`, `Ord`, `Show`, `Hash` and `Save` are derived automatically for records, variants and
  collections of types that have them.
- `Secret(T)` deliberately has no `Show` and no `Save`. Printing a secret, or holding one across a
  `!`, is a compile error.
- Trait calls are resolved at compile time where the type is known, so they cost nothing extra.

## 10. Effects and signatures

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
| `clock`, `random` | `now()`, `random()` | In jobs, allowed only through `!` calls, so a resumed run sees the same values |

A private function's effects are inferred and never need writing.

## 11. Entities

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
- **Methods never wait.** They can't use `!`, so they can't call other services; those calls
  belong in jobs. A method can `start` a job, and the job begins only if the transaction commits.
  Because entity methods have no outside side effects, the runtime can safely retry one after a
  conflict.
- **Reading from a job:** `w.read!()` returns a snapshot with a `version` field. Snapshots go stale,
  and commands can name the version they were based on (see `@fresh` in section 14).
- **Evolution:** adding a field with a default is automatic. Any other change to stored state needs
  a `migrate from N` function. The compiler refuses to deploy if a stored version has no path to the
  current one, and rows are migrated when they are next loaded.

## 12. Jobs and waiting

A job is a durable function: it can pause for any length of time and holds no memory while paused.

```eye
@on(daily(07:40, in: "Europe/Dublin"))
@priority(background)
@memory(64 MiB)
@deadline(13:40, in: "Europe/Dublin")
job irelandDigest(day: Date) -> Result(Delivery, DigestError) {
  let items = miniflux.unread!("ireland", since: day - 1 day)?
  let text = reserve!(256 MiB) {               # waits until the host grants the memory
    summarise!(items)
  }
  Ok(notify.send!(owner, text, key: "ireland-{day}").reconcile!())
}
```

- **`!` marks a checkpoint.** At each `!` the job's live variables are saved to a row, and the call's
  result is recorded. A resumed or restarted run continues from the last checkpoint and never
  repeats a recorded call.
- **Only values that can be saved** (types with `Save`) may be live across a `!`. Holding a socket,
  file handle or `Secret` across a checkpoint is a compile error.
- **Entity calls from jobs** (`World(w).markAsleep!()`) commit in the same database transaction as
  the checkpoint, so they happen exactly once.
- **Calls to other services** carry a key. If the outcome is `Uncertain`, `.reconcile!()` asks the
  owner with the same key until it knows. Nothing is resent automatically.
- **Waiting constructs** (all library functions, all marked with `!`):

  | Call | Waits for |
  |---|---|
  | `sleep!(20 min)`, `sleepUntil!(07:45 in "Europe/Dublin")` | time |
  | `approval!("delete 3 backups", from: operator, within: 3 days)` | a human; returns `Approved(Approval) \| Declined \| Expired` |
  | `next!(notify.delivered, where: fn e: e.key == k, within: 1 h)` | an event |
  | `reserve!(1 GiB) { … }` | memory from the host, released at the end of the block |
  | `lock!("ci", for: 30 min) { … }` | a named lock that records its owner and expiry |
  | `run.join!()` | a job started earlier |
  | `first!([a, b])` | whichever of several waits finishes first |

- **Starting jobs:** `start irelandDigest(day)` returns a `Run(T)`. It works from entity methods (as
  part of the transaction) and from jobs, where it starts a child run. Scheduled starts use
  `start x after 20 min` or `start x at 07:40 in "Europe/Dublin"`.
- **Attributes:**

  | Attribute | Meaning |
  |---|---|
  | `@on(trigger)` | `daily(…)`, `weekdays(…)`, `every(15 min)`, `cron("…", in: zone)`, `enters(view)`, `leaves(view)`, `event(contract.event)` |
  | `@memory(size)` | budget admitted before the run starts; the runtime enforces it |
  | `@priority(p)` | `interactive`, `normal`, `background` or `batch`; under memory pressure, lower priorities pause at their next checkpoint |
  | `@deadline(time or duration)` | after this the run ends as `Failed(DeadlinePassed)` |
  | `@concurrency(n, by: key)` | at most `n` runs at once, optionally per key |
  | `@limit(n, per: duration)` | rate limit |

## 13. Views and triggers

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

## 14. Contracts: talking to other services

A contract is the single description of another service's interface. It is written once and shared
by eyelang and the Go, TypeScript, Rust and Python code that implements or calls it.

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

type Delivery = Sent { receipt: ReceiptId, at: Instant }
              | Pending { receipt: ReceiptId }
              | Refused { reason: Refusal }
```

- **`import notify` is a typed edge.** Port, network policy, credentials and the allowed-caller check
  are derived from it, so a wrong port or an unapproved caller is a compile error.
- **Commands** return their declared variants plus `Uncertain { key }`, which `.reconcile!()`
  resolves through the declared query.
- **`@fresh`** on a command adds a required `basedOn: Version` argument and `Stale`/`Expired`
  outcomes, so an old command can't act on newer state.
- **`@destructive`** on a command adds a required `Approval` argument.
- **Fakes come from recordings.** `with fakes(notify, from: "receipts/")` replays real recorded
  exchanges, checked against the contract. Reading a field that no recording contains is a compile
  error, unless it is marked `@unwitnessed("why")`.
- **Generated code for other languages** (types, clients and server stubs for Go, TypeScript, Rust
  and Python) is a build output, never committed.

## 15. Time, secrets, tests

**Time.** Schedules carry their zone: `daily(07:40, in: "Europe/Dublin")`. Daylight-saving gaps and
overlaps follow a declared policy (`onGap: shift`, `onOverlap: first`), so there are never two UTC
crons and an hour check.

**Secrets.**

```eye
let token: Secret(String) = secret("telegram-bot-token")
http.post!(url, auth: bearer(token))       # bearer() accepts a Secret
log.info("token is {token}")               # compile error: Secret has no Show
```

**Tests.** A `test` block is a declaration. `with` applies a handler to the rest of the block:
`fakes(…)`, `clock(…)` with `advance(…)`, `memory(…)`. In tests, jobs run against an in-memory
store, so every run is deterministic and fast.

## 16. What happens to Eyelang 1

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
| `parser/`, `ast/` | ~1,100 lines | **Replace** | Needs a lossless syntax tree with error recovery (see section 17) |
| `eval/`, `object/` | ~1,600 lines | **Replace** | The new reference interpreter runs typed IR with entities and jobs; the current evaluator can't be adapted to that |
| `cli/`, `main.go` | ~120 lines | **Replace** | Becomes the `eye` command: `run`, `build`, `test`, `fmt`, `check`, `repl`, `why` |
| `repl/lineedit.go` | 318 lines | **Keep** | The line editor and history work and aren't tied to the language |
| `repl/repl.go` | 125 lines | Adapt | Reuse the loop around the new front end |
| `examples/*.eye` | 6 files | Replace | Eyelang 1 syntax; Eyelang 2 examples double as test fixtures |
| `.github/workflows/ci.yml`, `Taskfile.yml`, `go.mod` | — | Keep | Update tasks as the packages change |

Before deleting anything, tag the current state as `eyelang-1` so it stays easy to find and run.

## 17. Implementation plan

**Parsing: the grammar is the spec, the compiler's parser is hand-written.** Decided 2026-10-10.

- **`tree-sitter-eyelang/grammar.js` is the executable grammar.** It is the canonical definition of
  the syntax, and section 18 is a readable summary of it. It also drives highlighting in editors
  (Neovim, Helix, Zed, and VS Code through an extension). Interpolated strings and the `!` waiting
  marker need a small external scanner in C.
- **The compiler parses by hand,** in Go, with recursive descent and Pratt parsing for expressions
  (precedence table in section 18). That keeps cgo and runtime libraries out of the compiler and keeps
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
3. Type checker: inference inside bodies, checked signatures, traits, exhaustiveness, total arithmetic.
4. Effect inference and checks: `uses`, the `!` rules, and `Save` across checkpoints.
5. Typed IR, plus a reference interpreter on that IR. Used by the REPL and tests, with entities
   and jobs on an in-memory store.
6. The runtime pieces: runs table, runner, and storage interface on Postgres.
7. Code generation to C with reference counting, as described in the compilation analysis.

## 18. Grammar sketch

A readable summary. Once `tree-sitter-eyelang/grammar.js` exists, it is the definition, and this
section follows it.

```ebnf
file       = { import | decl } ;
import     = "import" path [ ".{" ident { "," ident } "}" ] ;
decl       = { attribute } ( fn | job | entity | view | type | trait | impl | contract | test | let ) ;
attribute  = "@" ident [ "(" args ")" ] ;
fn         = [ "pub" ] "fn" ident params [ "->" type ] [ "uses" effects ] ( "=" expr | block ) ;
job        = "job" ident params [ "->" type ] block ;
entity     = "entity" Ident params "{" { { attribute } member } "}" ;
member     = "state" ident ":" type [ "=" expr ] | fn | "migrate" "from" int "(" ident ")" block ;
view       = "view" ident "=" expr ;
type       = "type" Ident [ "(" tparams ")" ] "=" ( record | variants | typeexpr ) ;
variants   = variant { "|" variant } ;
variant    = Ident [ record | "(" typeexpr { "," typeexpr } ")" ] ;
trait      = "trait" Ident "{" { fnsig } "}" ;
impl       = "impl" Ident "for" typeexpr "{" { fn } "}" ;
contract   = "contract" Ident "{" { { attribute } cmember } "}" ;
cmember    = ( "command" | "query" ) ident params "->" typeexpr
           | "event" ident ":" typeexpr "on" string ;
test       = "test" string block ;
block      = "{" { stmt } [ expr ] "}" ;
stmt       = "let" pattern [ ":" typeexpr ] "=" expr [ "else" block ]
           | "var" ident "=" expr | ident "=" expr
           | "for" pattern "in" expr block | "while" expr block
           | "with" expr | "return" [ expr ] | "break" | "continue" | expr ;
primary    = literal | ident | "(" expr ")" | list | record | map | lambda
           | "if" expr block [ "else" ( block | if ) ] | "match" expr "{" { arm } "}"
           | "start" expr [ ( "after" | "at" ) expr ] ;
call       = primary { "(" args ")" | "!" "(" args ")" | "." ident | "[" expr "]" | "?" } ;
lambda     = "fn" ( ident | "(" idents ")" ) ( ":" expr | block ) ;
arm        = pattern [ "if" expr ] "=>" expr ;
```

**Operator precedence**, from tightest to loosest:

| Level | Operators |
|---|---|
| 1 | postfix: call `f(x)`, waiting call `f!(x)`, field `.x`, index `[i]`, propagate `?` |
| 2 | prefix: `-`, `!` (not) |
| 3 | `*` `/` `%` |
| 4 | `+` `-` |
| 5 | `|>` |
| 6 | `<` `>` `<=` `>=` |
| 7 | `==` `!=` |
| 8 | `&&` |
| 9 | `||` |
| 10 | `??` (default for `Option`) |

`!` after a name and directly before `(` is the waiting marker. `!` before an expression is logical
not. The lexer tells them apart by position, and the waiting `!` can't contain spaces.

## 19. Open questions

1. **Time conditions in views**, such as "idle for 20 minutes". Today that is `enters(view)` plus
   `sleep!`. Should views support `for 20 min` directly?
2. **Trait coherence:** may a module implement a trait it doesn't own for a type it doesn't own?
   (Rust's orphan rule says no.)
3. **Exposing entities and jobs to other services:** a generated contract per entity, or explicit
   contracts only?
4. **Job code upgrades:** when a new version changes what is live at a checkpoint, do paused runs
   finish on the old version, or must the change come with a migration?
5. **Parallel pure computation:** `pmap` over pure functions inside a job, without creating child
   runs?
6. **Packages:** `eye.toml` and an `eye.lock` that carries hashes (so Nix needs no placeholder
   hash); registry or git only?
7. **Calling C or Go libraries:** is a foreign-function interface needed for v1?
8. **REPL behaviour for jobs:** run them against the in-memory store with a controllable clock?
