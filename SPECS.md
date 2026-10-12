# Eyelang 2 — language design (draft)

Status: **draft**. These decisions come from design sessions since 2026-10-10 and from the
programs written in Eyelang since. The build plan is in
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

## Glossary

Words this document uses in one sense, and names that mean different things in different places.

| Word | Meaning |
|---|---|
| run | One execution of a job, from its start to its outcome. Each start makes one, unless its key started one already; `Run(a)` is the value that names it (section 15). |
| step | A `!` call in a job. It runs once, and its result is recorded in the run, so a resumed run gets the recorded result instead of calling again (section 12). |
| checkpoint | A run's saved place: its live variables, the statement it is at, and where each loop around it is. The interpreter takes one at a statement of the job's body once steps have been taken since the last; a run that wakes restores the latest and replays only the steps since (section 15). `zeropod.checkpoint!` in the tour is an operation of a service, not this. |
| transaction | What runs all or nothing: an entity method's call tree, a served operation, or a job's step that calls an entity. Jobs it starts and events it publishes happen only if it commits (section 14). |
| key | An idempotency key, a `Key`: what tells one command, job start or event from a repeat of it, so the same key never does the work twice (`key:` on a call, `@key` on an operation or event). A map's keys are ordinary values. |
| version | A snapshot's `version` is a `Version` that counts the transactions that changed the entity, and is what a `@fresh` command's `basedOn` takes (sections 14 and 17). `@version(N)` numbers the shape of an entity's stored state, for migrations. A state field can't be named `version`. |
| handler | What `with` applies to the rest of a test's block: `fakes(…)`, `clock(…)`, of the built-in type `Handler` (section 20). A program may declare its own `Handler`, as section 12's example does; in its module the name is the program's. |
| outcome | A run's `Outcome`, `Completed(value)` or `Failed(fault)` (section 15); a command's outcome is the variant its result is (section 17). |
| fault | What ends a run that code can't catch, such as `DeadlinePassed` or `Unreachable` (section 15). An `Err` is a value, not a fault. |
| `end` | In an entity's own method, `end(ok)` ends the entity (section 14). Elsewhere `s.end()` is a string's end position, and `r.end` a range's end. |
| `start` | `j.start(…)` starts a run of job `j`, and `claude.start!(…)` an agent's session. `s.start()` is a string's first position, and `r.start` a range's start. |
| `first` | `first(xs)` is a collection's first element, an `Option`. Waiting for whichever of several runs finishes first is `firstDone!`, and for all of them `allDone!`, named apart from `first` and `all` (section 15). |
| `join` | `xs \|> join(", ")` joins strings; `run.join!()` waits for a run's outcome. |

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
  holds no thread or stack, only its saved record: about 1.5 KB of RAM today, since the stores
  keep every record in memory, and nothing once records wait outside RAM (ROADMAP R.7).

## 2. Decision log

| Area | Decision | Why |
|---|---|---|
| Core model | **Entities** hold shared data; **jobs** are durable functions; **views** react to conditions across many entities | Data has a home, waiting has a home, and cross-entity conditions don't need polling |
| Waiting | **Only jobs wait.** Entity methods are short atomic transactions | No state changes in the middle of a method; every method is all-or-nothing |
| Transactions | **Entity methods and served operations stay inside the program**: they change state, call pure code and other entities' methods, start jobs and publish events, which happen only when the transaction commits, and they can't use `net`, `fs`, `process`, `clock`, `random`, `env`, `ffi`, `secret` or `wait`. Output (`print`, `log`) is allowed (section 14) | The runtime may run a transaction again after a conflict, so a call to a service or a file write in one would happen twice, which pillar 4's keyed calls exist to prevent; what commits with the transaction happens once. A line of output written twice harms no one. The clock is outside too: a method takes the time from its caller, so a retry sees the same time. From the spec review of 2026-10-12: methods could call services and sleep |
| Wait marker | A **`!` suffix** on every durable step in a job, which is every call that may pause or has an outside effect: `sleep!(20 min)`, `notify.send!(…)` | Every save point is visible in code review |
| Checkpoints | Every `!` call in a job is a recorded **step**: its result is saved and it runs once. A **checkpoint** is a run's saved place, its live variables and statement, taken at the statements of the job's body once steps have been taken; a run resumes from its latest and replays only the steps since (section 15) | No separate `step` keyword is needed |
| Entity lifecycle | **Virtual**: an entity exists on first use. Ending one with protected state needs an `Approval` | No create/exists dance; deletion always involves a human |
| Storage | An **embeddable engine** with backends behind a `Store` trait. The default backend keeps everything in memory and is written in Eyelang (`check/runtime/store.eye`); a journal file wraps it for durability on one machine (open question 9); **Postgres** is selectable | Programs, tests and embedders need no database; production picks durability; writing the default backend in Eyelang tests the language on real systems code |
| Types | **Static.** Public items carry full signatures, including effects; everything inside bodies is inferred | Changes to types and permissions show up in the diff |
| Errors | **Result only**: no exceptions. `?` wraps an error into the caller's error type when exactly one of its variants holds it; otherwise `.mapErr(…)`. Runtime faults and `unreachable` become outcomes of the run | Every failure is visible in a type, and wrapping needs no boilerplate |
| Records | **Nominal**: a record type is its name, and a literal takes the type its context expects | Same-shaped types can't be mixed up, and methods, operators, schemas and error messages have a name to use |
| Results | **A function without `->` returns `()` when its body is a block**, and its last value is dropped; one written `= expr` returns the expression's type, inferred. `while true` without `break` has any type, since only `return` leaves it | Entity methods, jobs and helpers that do something need no `-> ()`, and a loop that serves until it returns needs no `unreachable` after it. From the type checker: Notify's delivery loop |
| Effects | **Inferred inside, stated at the boundary**: public functions list their effects with `uses` (none means pure); a callback's effects pass to the call that gives it; a function type kept in data states its effects | Permissions show up in the diff without annotating every helper, and higher-order functions work with any callback |
| Operators | **Traits**: `+ - * / %` call `Add`, `Sub`, `Mul`, `Div` and `Rem`, which any type can implement once per right-hand type; no implicit conversions | Money, vectors and units get real operators, and the result type is always known from the operands |
| Arithmetic | **Total types**: `Int` has arbitrary precision; `/` and `%` need a `NonZero` divisor; indexing returns `Option` | Removes the runtime errors that "Result only" would otherwise need |
| Fixed-width integers | **No operators; each operation names its edge.** `Byte`, `Int32`, `Int64`, `UInt32` and `UInt64` add with `wrappingAdd` or `checkedAdd` (an `Option`), and likewise subtract and multiply; bit operations are methods too (`and`, `xor`, `shiftLeft`, …). `T.from(n)` converts an `Int` and is an `Option`; `x.toInt()` always fits. A literal that doesn't fit its type is a compile error, and an ASCII character literal is a `Byte` where one is expected (section 5) | Overflow is never silent and never a fault: the code says at each step whether it wants wrapping, as a hash does, or a check, as a length does, as Rust's `wrapping_add` and `checked_add` do. Byte-level code, such as a lexer over `Bytes`, matches `'-'` instead of `45` |
| Consistency | **Atomic inside eyelang**; keyed commands and **reconcile** at the boundary with other services. A `@reconcile` query returns an `Option` of the command's outcome: `None` means the command never arrived, and only then is it sent again, with the same key | One database makes real transactions cheap; outside services own their own state; the one case where resending is safe is decided by the owner, never guessed. From the lab: a lookup that can't say "never arrived" leaves a lost command uncertain for good |
| Values as JSON | **One JSON shape everywhere**: records are objects, a variant is an object tagged with `"kind"`, a type of fieldless variants is a string, a wrapped value is the value, `None` is null (section 17) | Requests, replies, recordings, model schemas and the generated TypeScript and Go types agree, and a tagged union reads naturally as a discriminated union in TypeScript |
| Contracts over HTTP | **Attributes map a contract to HTTP**: `@http("POST /v1/send", status: {202: Pending})` on an operation, `@http(names: snake)` and `@auth(…)` on the contract, the key's header and body field on `@key`; addresses and credentials come from the environment | The real APIs the lab calls (Notify, Miniflux) are described without code; a request that never went out is retried, and only a lost reply is `Uncertain`. From the lab: Notify's 200 and 202 answers and its idempotency header |
| Serving contracts | **A program serves a contract with `impl Contract { … }`**: one function per operation, with the contract's types and defaults, run as one transaction; a command with `@key` takes the key last | A service written in Eyelang shares one contract with its callers, and a key can name an entity, so repeats need no dedupe table. From the lab: Notify, written in Eyelang |
| Serving over HTTP | **`eye run` serves each contract a program serves on its `@port`, read the way a caller sends it**: the `@http` route picks the operation, arguments and replies are in the one JSON shape, `@auth` is checked before any code runs (401), each request is one transaction, and a result has the status `status:` gives its variant, else 200. `NOTIFY_PORT` moves the port, `0` to any free one (section 17) | Callers and servers agree because both read the same attributes, and the key, not a cache of replies, makes a repeat safe, as for any service. From the lab: Notify written in Eyelang must answer the lab's other programs |
| Ports and callers | **`@port` is where a service listens, and `@allow` who may reach it, checked at compile time.** A contract a program serves must declare `@port`; a contract without one is a service outside the network, reached at its configured URL, that any module may call. `@allow(m, …)` lists the modules that may call its operations (besides its own module, the modules that serve it, and tests); each name is a module the program can import; `@allow(nobody)` lets no module call it; `@allow` needs `@port` (section 17) | The lab's own Notify has a port and four callers, which network policy can be written from, while Telegram, Miniflux and NetBird are outside services with no port of ours to guard: a rule that every called contract declares a port would have given them made-up ones. Callers write no port: a call goes to the URL the configuration gives, and the policy opens only the contract's port, so a call configured for another port reaches nothing |
| Fresh and destructive commands | **`@fresh` adds a `basedOn: Version` parameter and the outcomes `Stale` and `Expired`; `@destructive` adds an `approval: Approval` parameter.** Both come after the declared parameters, before `key`, and a served `impl` takes them too. `Stale` means the state changed since that version, `Expired` that what the command was based on is gone; `Expired` is the one `approval!` gives. A `Version` has `==` and no order; a snapshot's `version` is one. Over HTTP they travel as arguments, a reply of 409 is `Stale` and 410 `Expired`, and both outcomes are objects tagged with `"kind"` (section 17) | The check-then-act race is the usual bug in programs that act on another service's state, and the fix (send the version you read, let the service refuse) is easy to forget, so the contract asks for it and the match must handle the refusal. Two outcomes because a caller does different things: read again after `Stale`, give up after `Expired`. Reusing `approval!`'s `Expired` keeps one word for "too late" |
| Network policy | **`eye gen policy` writes Kubernetes `NetworkPolicy` objects from the contracts**: for each contract with `@port`, the pods of the module that serves it (else of the contract's own module) accept TCP on that port only from the pods of the modules whose code calls it, among those `@allow` lets. Pods are labelled `eyelang.module: <module>`; the namespace is the `--namespace` flag (section 17) | The homelab deploys to Kubernetes, and a plain `NetworkPolicy` needs no CNI extension. The checker already knows who calls what, so the policy admits the callers the code has, not every name `@allow` lists: a module listed but not yet calling stays out until it does. The namespace is a deployment's choice, not the program's |
| Generated code | **`eye gen go` and `eye gen ts` write each contract module's types, a client and a server stub**, from the rules the interpreter follows (`internal/wire`) and the defaults it computes. TypeScript's types are the JSON, a variant type a union on `"kind"`; Go's are structs and interfaces with JSON methods. Only operations with `@http` get a client method and a route. The outcomes a contract adds are values in TypeScript and errors in Go (`*Uncertain`, `ErrStale`, `ErrExpired`); a stub answers as `eye run` does, 500 for a failure (section 17) | Notify in TypeScript and any service in Go build against the contract, so a change that breaks them fails their build. One set of rules read by the interpreter and both generators, checked by a round trip through `eye run`, keeps the three in step. Go code checks `err` after every call, so the outcomes it must handle are errors there; a TypeScript union handles them as values |
| Events | **Published when the transaction commits, and handled once per key.** `@on(event(e))` starts one run per event, and `@key(field)` on the event makes a repeat start nothing; `next!` counts every event published since the run's previous step | A reply that comes quickly is never missed, and a message the bus delivers twice never starts a second run. From the lab: a button pressed seconds after its question, and Telegram updates delivered again after a restart |
| Queries | **Pipelines** (`filter`, `map`, …) over lists and entity sets alike. Over an entity set they run in memory today; compiling them to SQL waits for several runners on one database (open question 28) | The same code as for lists |
| Reactions | **Views trigger jobs**: `@on(enters(view))`, `@on(leaves(view))` | Replaces most polling cron jobs |
| Evolution | Adding a field with a default is automatic; anything else raises the entity's **`@version`** by one and adds a **`migrate from N`** function, from a row as it was stored at version N to the state at N+1. Migrations chain, with no gaps; `eye run` refuses to start if a stored row has no path to the current version | Stored data outlives code, and the shape of old data needs no declaration kept around: a migration reads the old row by name |
| Mutation | **Mutable value semantics**: `var` locals, `mut` parameters and `mut self` methods, field assignment and `+=`. Callers write `mut` too: `unify(mut subst, a, b)`. Values never alias; there are no references or pointers | Systems code reads naturally, every place a value can change is visible in the diff, and without aliasing there are no cycles, so reference counting stays enough |
| Generics and methods | Lower-case **type variables** (`List(a)`), requirements with `where a: Ord`, and methods on a type with `impl Type { … }` | Generic code without new brackets; types get their own operations |
| Shared behaviour | **Traits** with explicit `impl`; equality, ordering, printing, hashing, saving and schemas are derived for data types whose parts have them. An impl lives in the module of the trait or of the type (an `impl` of a contract, which serves it, may go in any module), and a type variable has only the traits its `where` gives it | One mechanism for generic code; a requirement written once is checked at every call, and two modules can't give one type two meanings of a trait |
| Declarations | **Typed attributes** (`@on`, `@memory`, `@priority`, …) on jobs, entities and contracts. Each attribute is declared with where it goes and what it takes, and the checker holds every use to that: `@concurency isn't an attribute; did you mean @concurrency?` | One general mechanism instead of special syntax per feature, and a program can't say "one at a time" and quietly get five. From the lab: a misspelt attribute was ignored, and `@concurrency(1)` wasn't enforced |
| Starting jobs | **A run can be started with a key, and starts once per key**: `ring.start("kitchen", key: "kitchen")`. `main` runs on every start of `eye run`, so a job it starts must have a key | The obvious way to start long-lived work from `main` is also the right one, and an event or command that comes twice starts one run. From the lab: a restart started the morning's job a second time |
| Effects at the boundary | **Public entity methods and public jobs state their effects as public functions do, and starting a job is an effect, `start J`.** The key rule for `main` follows calls through it: a function or method `main` calls that starts a job without a key is refused. `eye fmt` writes the clauses (section 12) | A method that starts a job, or a job that reads files, does as much as a function that does, and a reviewer needs to see it in the same place. Starting a job is a change to the outside of the call: in a job a function that starts one is a recorded step, and a helper could otherwise hide from `main`'s key rule a start that repeats on every restart. From the spec review of 2026-10-12 |
| Reading entities in jobs | **A job reads stored state only in recorded steps**: `w.read!()`, and `idleWorlds.read!()` or `World.all.read!()`, which give a list of snapshots. In a job or a waiting function, `w.phase`, a view and `World.all` read any other way are errors that name the step; outside jobs `w.phase` is short for `w.read().phase`, and a reference's identity reads anywhere (sections 14 and 15) | A resumed run must see the values it saw, and an unrecorded read in a replayed statement sees the entity as it is now. Making the reads errors, rather than quietly recording them, keeps every step visible as a `!`. From the spec review of 2026-10-12: a job read `w.phase`, slept, and recorded a value the test had changed meanwhile |
| Waiting functions | **A function whose body has `!` steps is a waiting function**: it has `wait` in its effects, and jobs and other waiting functions call it with `!`, but `main` and tests can't. **Outside a job a wait blocks**: `sleep(1 s)` in `main` holds it, and the call keeps `wait` in the caller's effects; a job can't call a function that waits that way (section 12) | Every wait in a run is a recorded step, so a job can't block on a sleep a helper hides. Outside jobs blocking is what a script expects, and `wait` in a signature shows that a call can hold its caller up. From the spec review of 2026-10-12: `pub fn pause() { sleep(1 s) }` passed as pure |
| Programs | **`main` is an ordinary function**; files, processes and other I/O are effects called directly. `!` marks durable steps, and is used only in jobs and in the waiting functions they call. `unreachable("why")` is the one way for code to stop itself | The core is complete without the durable runtime; jobs stay explicit about every step |
| What a fake received | **`fakes(…)` is a value; `fake.calls(op)` lists the calls it answered**, each a record of the call's arguments with defaults filled in, and the key (section 17) | A test checks what a program sent to a service without the program keeping a record only tests read, such as the digest's `Day.sent`; a record of the arguments compares whole, and shows what differs when a test fails |
| Callbacks and stored functions | **A parameter's function type without `uses` takes a callback; every other function type states its effects.** A callback may do anything, and its effects count where it is given. A lambda or named function kept in a field, a collection or a result does no more than that type states, and a callback can't be kept in data without its parameter's type stating its effects (section 12) | Higher-order functions stay generic over effects without effect variables, and a stored function can't do behind its caller's back what the caller's signature doesn't show |
| Writing effects | **`eye fmt` writes public functions' `uses` clauses.** Formatting a program's directory checks the program and writes each public function's inferred effects into its signature: it adds what is missing and drops what the body no longer does. A file formatted alone, and a program with other errors, keep their clauses as written (section 12) | Effects in a signature are there for the reviewer, who reads them in the diff; working them out and keeping them current is the compiler's job, not the author's, and a permission the code stopped using doesn't linger |
| Live tests | **`eye test --live ai,notify` sends the named modules' calls to the real services, and a test that can't reach one is skipped, not failed; `--record` writes their answers where the fakes read them**, merged by operation and key, only from tests that passed, and never with a secret in them (section 20) | The same tests check the program against the recordings every day and against the real services when asked, with no second set of tests; a missing key on a laptop or in CI is a skip with a reason, and a re-recorded file is reviewed in the diff like code |
| Tests and the machine | **A test works in an empty directory of its own, and draws random numbers from a fixed seed.** `fs` and `process` resolve relative paths there, and it is removed when the test ends | A test that touches files or runs programs gives the same result on every run and every machine, and can't change the repository it runs in |
| Concurrency | **Structured**, without `async`/`await`: `parallelMap`, `both`, `race` start tasks that finish before the call returns, and a waiting task parks without blocking others. Tasks share only immutable values. From v0.2, task groups with `spawn` and bounded channels, also scoped | Parallelism without coloured functions or data races; back-pressure keeps memory bounded |
| Dynamic dispatch | Static by default; **`dyn Trait`** from v0.2 for values whose type is known only at run time (`fn notifier(config) -> dyn Notifier`, `List(dyn Handler)`): a value converts to it where one is expected, and has only the trait's methods | The cost of a run-time lookup is visible in the type; libraries can accept implementations they don't know |
| AI | **Typed model calls and agents, as library code**, with no syntax that serves only them: a call names the Eyelang type it wants, the compiler derives a JSON Schema from it, and the reply is checked and decoded into that type. Tools are functions whose effects become the caller's; in jobs every model call is a recorded `!` step; coding agents are sessions that durable jobs own | No new syntax: permissions, cost and nondeterminism are handled by effects, budgets and recordings, which already exist |
| Syntax | **Add syntax when it is generally useful and clearly better than a library function**; syntax that also makes one area (jobs, agents) much better is welcome if it pays off elsewhere too | Keeps the language small without making common code clumsy |
| Interop | **`extern fn`** for C functions and for functions provided by a Go program that embeds Eyelang | Reuse existing libraries; the standard library's primitives use the same mechanism |
| Keywords | **Only statement and expression words are reserved.** Declaration words (`type`, `entity`, `state`, `job`, `view`, `test`, …) are keywords only where a declaration starts | Common names such as `state`, `view`, `type` and `test` stay usable |
| Blocks | **Braces**, only where they hold several statements. A branch that is one expression or one assignment needs none: `if c then a else b`, `Pattern => value`, `Ok(t) => found += [t]`, `fn x: value`, `fn f() = value`. Layout is free; indentation means nothing | Keeps parsing simple and pasting safe, with no whitespace pitfalls, while one-liners stay light. From the lab: sorting results into lists in a `match` needed braces on every arm |
| Strings | **One line, or `"""` for many lines.** A multi-line string starts with `"""` and a line break and ends with `"""` on a line of its own, apart from what follows it; that line's indentation is removed from every line (section 4) | Prompts and messages read as text in the code, an unclosed `"` still can't swallow a file, and the formatter re-indents code without changing a string. From the lab: the digest's 40-line prompt |
| String positions | **A position in a string is an `Offset`, not an `Int`**, as Swift's `String.Index` is. `s.indexOf(part)` gives one, `s.slice(a..b)` takes a `Range(Offset)`, and `s.start()`, `s.end()`, `s.after(i)` and `s.before(i)` make and move them; `size`, `take` and `drop` count characters, in `Int`s | Scanning by position costs nothing per step and can't cut a character in two, and a count can't be passed where a position goes: with `Int` offsets, `s.slice(0..(s \|> size))` on `"Málaga"` quietly dropped the last letter. From the lab: the digest's HTML-to-text scanner |
| Arguments | **`name:` alone passes the variable of that name**: `send(key:)` is `send(key: key)` | Keys, priorities and recipients pass through several layers under their own names, and the label still shows at each call. From the lab: `key: key` and `priority: priority` in every layer of the digest and the reminders |
| Statements | **A newline ends a statement** unless the next line starts with a binary operator, `.`, `?`, `,`, `:`, `=`, `->` or `else` (section 4). `;` also separates statements | No semicolons, and a long pipeline can still break before each `|>` |
| Braces in expressions | A `{` after `if … `, `else`, `for`, `while`, `let … else`, a signature, `test "…"` or a lambda's parameters opens a block; anywhere else it starts a record or map. After `=>` it opens a block unless `name:`, `name,` or `name with` follows, which only a record can start. Variant payloads use parentheses: `Pending(receipt: id)` | Every `{` has one reading, found with at most two tokens of lookahead, in both parsers; a match arm returns a record without extra parentheses |
| Pipes | Data-first: `x |> f(a)` is `f(x, a)`; `_` places the value elsewhere: `x |> f(a, _)` | Matches method-call intuition |
| Memory | Reference counting with in-place reuse of uniquely owned values; immutable values; entities referenced by ID | No GC pauses, and no reference cycles by construction |
| Parsing | A **tree-sitter grammar is the spec**; the compiler's parser is hand-written; a conformance suite fails CI if the two disagree | One definition of the syntax, editor support for free, and compiler error messages under our control |
| Metaprogramming | **No user-defined macros before v1.0.** `entity`, `job`, `contract` and `view` are built in, each lowered by its own compiler pass. Deriving is limited to the built-in traits; rare code generation goes through a build step (`eye generate`) | Fixed constructs give clearer errors and are easier for agents to work with; a macro system would delay v0.1; separate passes keep the option open |
| Program root | **`eye.toml` marks the directory a program starts from**; a file belongs to the nearest one above it, else to its own directory | Tools given one file, such as an editor jumping to a definition, must find the modules it imports: `lab/digest/digest.eye` imports `lab/home`. A marker file says so once, and it is the file that packages (open question 5) need anyway |
| Built-in docs | **The built-ins are declared in Eyelang**, in `ir/builtin.eye`, with `extern type`, `extern fn` and `##` comments; the checker checks calls against those signatures and takes effects from their `uses`, and the interpreter implements them | Like Go's `builtin` package: an editor goes to a built-in's documented declaration as to any other, and the reference is written in one place, next to the signatures it describes. Since the checker uses them, the docs can't drift from the rules |
| Doc comments | **`##` comments are Markdown, with a first-sentence summary, `[Name]` links that are checked, and a module doc at the top of a file**. One extractor feeds `eye doc`, the editor's hover and completion, and AI schemas | One source of documentation for people, editors and models. Checked links keep docs from pointing at renamed code, and the summary rule gives every list a short line without a second comment |
| Answering approvals | **An answer names its request's `id`, and only the person in `from:` may give it.** The runtime refuses any other answer and logs it. Text matches only from the command line and in tests, and is refused when it matches more than one waiting approval. An `Approval` has `what` and `by` fields (section 15) | An `Approval` is the one value that stands for a person's consent, so the obvious routing job must be the safe one: one that forgets to check who pressed the button can't grant it, and an answer can't reach a request it wasn't meant for. From the lab: two runs asking "Restart the router?" were answered by one press (open question 17) |
| Recorded steps | **`!` marks every recorded step in a job**: every outside effect, every entity call, publishing an event (`bus.pinged.publish!(…)`) and starting a child run (`digest.start!(day)`). The effects of section 12's table are inferred, `publish C.event`, `consume C.event`, `secret "name"` and `ffi` included. A step's result is recorded, so it must be something that can be saved. Output is not a step: `print` keeps `stdio` for signatures, and logging is not an effect (section 12) | Every point where a run can pause or commit is visible, with no exception to remember; output is a side channel for people, and marking it would put `!` on every log line (open questions 19 and 22) |
| Foreign values | **`extern type` is the one way to name a value foreign code owns**; `Handle(name)` is gone. A program's `extern type` has no `Save`, `Show`, `Eq`, `Hash` or `Schema`, so it can't be saved, live across a `!`, printed, compared or asked of a model (section 18) | One obvious way, a name the checker tells apart from every other, and the saving rule applied to it: an `extern type` could be held across a `!` while a `Handle` couldn't (open question 21) |
| Loading C | **The interpreter loads `@library` functions with purego** (github.com/ebitengine/purego), which calls the system's `dlopen` and then C functions from Go without cgo; the loader is `internal/ffi`, which the Go runner can also load compiled programs with. **The released `eye` for Linux leaves the loader out**, since it makes a binary dynamically linked; `go build -tags clib` puts it in, and on macOS and Windows it is always in (section 18). Decided 2026-10-12 | Linux has no system call that loads a library: `dlopen` belongs to the C library, so the choice was a loader of our own, which would also have to start libc for each library it loads, or the system's. The lab runs `eye` on `scratch` images, which have no dynamic linker, so the release must stay static |
| Host functions | **A `@host` function is any Go function whose types match its declaration**, checked when it is registered: `Int` is a Go `int`, `Result(T, String)` is `(T, error)`, and an `extern type` is any Go pointer, struct, map, channel, function or interface type (section 18). Decided 2026-10-12 | The embedding program writes ordinary Go, and a mismatch is an error at `Register` that names the parameter, not a wrong value at a call. Go's `int` and `error` are what Go code already has |
| Long runs | **A run's saved state is its live variables, not its history.** The interpreter keeps a checkpoint at each statement of a job's body and replays only the steps since (from 2026-10-11); compiled code saves frames at each `!` from v0.4. The language gets no `continue`-as-new | A run that never ends, such as the lab's train watcher, costs the same on its tenth day as on its first, with no new syntax (open question 9) |
| Job code changes | **A paused run resumes only at a resume point that still exists with the same live variables**: in the interpreter, the same statement of the job with the same typed code and as many variables. `eye run` refuses to start otherwise, as it does for entity rows it can't migrate, and names the job and its runs. Replaying the steps since a checkpoint compares each recorded step's call with the call it replays and faults the run on a difference | A code change under a paused run is an error with a message, not a wrong value found later; the old job keeps its name until its runs finish, and the new code takes a new one. Finishing old runs on old code is rejected, since entities migrate forward and old code would read new state (open question 4) |
| Typed choices | **A type whose variants have no fields gets `T.variants()` and `T.parse(s)`, derived**: every variant in order, and `Some` of the variant named `s`, else `None` (section 11) | A person's choice, such as a button, is matched as a variant, so a misspelt or forgotten choice fails to compile, and the buttons come from the type the match uses. No syntax: they are calls on a type, as `Lexer.new` is (open question 15) |
| Module roots | **`eye.toml` may list `modules`, more directories to find imports in**, searched after the program's own directory, in order (section 13) | A program deployed alone, such as the lab's digest, names the lab's modules in its `eye.toml` instead of reaching them through symbolic links that break and must list every module imported. From the lab: `deploy/digest` |
| Native code | **Compiled programs are C, built by the `zig cc` that `eye` ships**, with Perceus reference counting, whole-program specialisation and jobs resuming from saved frames ([docs/internals/compilation.md](docs/internals/compilation.md)). `eye` itself stays free of cgo: the interpreter loads C libraries at run time without it | The workload needs memory and resume time more than the best code, and C at `-O2` gives both; the bootstrap seed is C anyway; Koka and Lean 4 compile reference-counted functional code to C. Shipping `zig cc` cross-compiles to all five release targets from any host, with the same compiler everywhere |
| Generic private functions | **A private function that leaves its types out is generic**: its body is checked on its own, and each call decides afresh what the body leaves open, so `fn twice(n) = n + n` works on `Int`s and `Float`s alike. A call that gives types without the operators or traits the body uses is an error at the call. Functions that call each other share one type while they are checked (section 6) | One type for the whole program made a helper's first caller decide it for every other, and the error appeared at the second caller, far from the cause. Let-polymorphism is what ML and Haskell do for the same reason, and compiled code specialises each use (section 22) |
| Schedules | **`cron("…", in: zone)` takes the five standard fields** (values, names, ranges, lists and steps; either day field matches when both are given), and no extensions. **Daylight-saving changes are policies, `onGap: Shift \| Skip` and `onOverlap: First \| Second \| Both`**, prelude variants, defaulting to `Shift` and `First` (section 20) | A crontab line copied from elsewhere means what it meant there, and its errors show at the string; the extensions (`L`, `W`, `#`, seconds) are rare and differ between crons. The defaults are what `daily` did before the policies could be chosen, and they need no hour check; `Both` is for jobs that should run every hour of real time. Variants, not strings, so a misspelt policy fails to compile |
| Joining runs | **`allDone!` and `firstDone!` take a list of runs; `firstDone!` stops the runs that lost**, which end as `Failed(Killed)` (section 15) | A durable wait is a child run, so joining several needs nothing new beside `join!`, and each run's steps stay in one order. A loser that went on, such as an approval still waiting for an answer after its timeout, would act on a decision already made |
| Lock expiry | **A run still waiting inside `lock!` when the lock runs out ends as `Failed(LockExpired(name))`** (section 15) | The expiry lets the next run take a lock a stuck holder keeps. Letting the old holder go on would run the guarded work twice at once, which is what the lock is for |
| Naming the joins | **The waits for several runs are `allDone!` and `firstDone!`, not `all!` and `first!`**; `all` and `first` are only the collection functions, and `all!(runs)` or `first!(runs)` is an error that names the wait to write (section 15). Decided by the owner on 2026-10-12 | `!` only ever marks the same call as a step; it never changes which function a name means. With `all!` and `first!`, adding or dropping a `!` switched between a wait and a collection function, and a reader had to know both. From the spec review of 2026-10-12, design risk 11 ([docs/internals/reviews/spec-review-2026-10-12.md](docs/internals/reviews/spec-review-2026-10-12.md)) |
| Ending entities | **`end(ok)` in an entity's own method ends it for good**: `ok` must be an `Approval`, the state is removed, and the store keeps only when it ended and whose approval let it. Calling or reading an ended entity faults; its identity never starts again. In an entity's method `end` is the entity's; elsewhere it is a string's end (section 14) | Ending deletes data, so it takes a person's consent that the compiler checks. If a later call started the identity again from its defaults, a job holding an old reference would bring back what a person approved ending, silently. Resolving `end` by context keeps `s.end()` and the spec's `end(ok)` both |
| Lazy iteration | **`xs.iter()` gives an `Iter(a)`, a value that describes a walk and runs it again each time it is walked.** Lists, sets, maps, `Int` ranges, strings and `Bytes` give one, and `s.chars()` is one; `map`, `filter`, `take`, `drop`, `enumerate`, `zip` and `flatMap` on an `Iter` give an `Iter`, and the other collection functions walk it. Its functions only compute; it has none of the derived traits, so it can't be live across a `!`; and there are no endless sources (section 5) | Walking twice gives the same elements because an `Iter` is a value, not a cursor a first walk would use up, which would be state shared by everyone holding it. Functions without effects make a second walk cost time and nothing else, wherever it happens. Sources that end keep `size`, `toList` and `sort` sure to finish |
| Division helpers | **`/` and `%` keep their `NonZero` divisor, and `xs.average()`, `a.div(b)` and `a.rem(b)` give an `Option` instead**: `None` for an empty list or a divisor of 0. `average` gives a `Float` for `Int`s too; `div` and `rem` round down as `/` and `%` do (section 10) | Every mean needed `match nonzero(xs \|> size) { … }`, so the obvious code was long. One result type for every list keeps `average` predictable, and a mean of `1` and `2` that is quietly `1` would be the wrong answer made easy; `xs.sum().div(xs.size())` is there for an `Int` mean (decided 2026-10-12) |
| Calls that come back | **A call tree is one transaction, and an entity has one state in it**: a call that comes back to an entity whose method is still running starts from the tree's changes, the outer method goes on from the call's changes as from a `mut self` call, reads in the tree see the state in progress, and the entity commits once (from 2026-10-12, section 14) | Each call loaded its own copy and the last to finish won, so `A.ping` calling `B.pong` calling `A.bump` lost a write without a word. Refusing re-entry would be simpler, but it rejects programs that "the whole call tree commits together" invites |
| Model tools | **A function declared with `fn` and named where an `ai.Tool` is expected converts to one**, as a value converts to `dyn Trait`; its parameters and result need `Schema`. Its effects count where it is named, and a model call's own effect is `net Ai.generate`, not `net`. In a job each model reply and each tool result is a step (section 19) | `tools: [searchIssues, label]` holds functions of different types, which no rule typed. A conversion keeps what a schema needs, the name, the `##` comment and the types, without a wrapper to write. A call whose own effect was `net` made every public caller list `net`, which covers any tool, so a new tool never showed in review. Recording each step keeps a restarted conversation from running a tool twice. From the spec review of 2026-10-12 |
| Destructive tools | **`@destructive` adds `approval: Approval` to a function as to a command, and a model's call of one is approved through `ai.generate`'s `approve:`**, a function, usually `fn call: approval!(call, from: someone, within: …)`, called once for each call; without it the call is declined (section 19) | One meaning for the attribute: what it marks runs only with a person's consent, which the type shows. "Until someone approves" named no one; asking through `approval!` answers whom with its `from:`, and keeps its rules (answers by `id`, only from the person asked) with no second way to ask. A function rather than `approver:` and `within:` arguments, which would copy `approval!`'s. From the spec review of 2026-10-12 |
| `?` in a lambda | **A lambda's `?` returns from the lambda, so the lambda's result must be an `Option` for an `Option`'s `None` and a `Result` for an error, which is wrapped into the lambda's error type.** `?` on an `Option` in a function, job or lambda that doesn't return one is an error that names what it returns and points to `??` (section 10, from 2026-10-12) | `return` can't leave a lambda, so `?` returning from the enclosing function would be the one way out, and the interpreter already returned from the lambda. Checking against the enclosing function let `xs \|> map(fn x: x? + 1)` put a `None` where an `Int` belongs, and `fn g(x: Option(Int)) -> Int = x? + 1` returned a `None` from a function that returns `Int` |

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

  pub fn join(p: PlayerId) uses {start wake} {
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

- **Comments:** `#` to the end of the line. A comment that starts with `##` is a doc comment: it
  documents the declaration, member, field or variant directly below it. A blank line or a plain
  comment in between detaches it. `eye doc` publishes doc comments, editors show them on hover and
  in completion, and a model call uses them to describe that part of the schema (section 19).
  - A doc comment is Markdown. Its first sentence is the summary that indexes, completion lists
    and hover headers show, so it should say what the thing is or does on its own.
  - `[Name]`, `[Type.member]` and `[module.Name]` link to the declaration they name. `eye doc`
    turns them into links and fails on a link that names nothing, so docs can't point at code that
    was renamed or removed.
  - A `##` block at the top of a file, separated from the first declaration by a blank line,
    documents the module. Plain `#` comments are never published: use them for notes to whoever
    maintains the code.
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
  `\'`. Where a `Byte` is expected, an ASCII character literal is a `Byte`: `src[i] == Some('"')`
  (section 5).
- **Strings:** a string's text stays on one line, though an interpolated expression may break
  across lines. The escapes are `\n`, `\r`, `\t`, `\0`, `\\`, `\"`, `\{`, `\}` and
  `\u{1F600}`. `{expression}` interpolates a value that has `Show`.

  A **multi-line string** starts with `"""` at the end of a line and ends at a `"""` with only
  spaces before it on its line. Those spaces are the margin: every line starts with it, or is
  blank, and loses it, and the line breaks after the opening `"""` and before the closing line
  aren't part of the string. One or two quotes inside are text; escapes and interpolation work as in other
  strings.

  ```eye context=lexical
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
  has no options. Formatting a program's directory also writes public functions' `uses` clauses
  (section 12).
- **Newlines end statements.** A line break ends a statement, declaration, member or match arm
  wherever one could end. It doesn't when the next line (ignoring blank and comment lines) starts
  with a binary operator, `.`, `?`, `,`, `:`, `=`, `->` or `else`; `-` and `!` continue only as
  `->` and `!=`, since `-x` and `!x` can start a statement. Inside `(…)`, `[…]` and record or map
  braces no statement can end, so line breaks there are ordinary spaces (until a lambda's block
  starts a new list of statements). `;` separates statements on one line.

  ```eye context=lexical
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

- **Primitives:** `Int`, `NonZero(Int)`, `Float`, `Bool`, `Char`, `String`, `()`. A
  `NonZero(Int)` is what `/` and `%` take as a divisor, so dividing can't fail (section 10). Most
  code doesn't need one by name: `xs.average()` and `a.div(b)` give an `Option` instead.
- **Bytes and fixed-width integers:** `Byte` (0 to 255), `Int32`, `Int64`, `UInt32` and `UInt64`,
  and `Bytes`, a sequence of `Byte`s, for binary formats, hashing, lexers and foreign functions.
  They compare with the usual operators but have no `+`, `-`, `*` or `/`. Arithmetic and bit
  operations are methods, each saying what happens at the edges of the type:

  | Method | What it gives |
  |---|---|
  | `a.wrappingAdd(b)`, `a.wrappingSub(b)`, `a.wrappingMul(b)` | the result wrapped into the type, modulo 2 to the power of its bits: for `Byte`s, `255` plus `1` is `0` |
  | `a.checkedAdd(b)`, `a.checkedSub(b)`, `a.checkedMul(b)` | `Some(result)`, or `None` when the result doesn't fit |
  | `a.and(b)`, `a.or(b)`, `a.xor(b)`, `a.not()` | the bits combined, two's complement for the signed types |
  | `a.shiftLeft(n)`, `a.shiftRight(n)` | the bits moved by `n`, an `Int`; bits moved out are lost, `shiftRight` keeps the sign of a signed type, and a negative `n` is a fault |
  | `a.toInt()` | the value as an `Int`, which always fits |
  | `Int32.from(n)`, and `T.from(n)` for each type | `Some` of the `Int` `n` as that type when it fits, otherwise `None` |

  `a` and `b` have the same type: `x.wrappingAdd(1)` adds a literal of `x`'s type. To mix widths,
  convert through `Int`: `Int64.from(x.toInt())`.

  ```eye
  fn fnv1a(data: Bytes) -> UInt32 {
    var hash: UInt32 = 2166136261
    for b in data {
      hash = hash.xor(UInt32.from(b.toInt()) ?? 0).wrappingMul(16777619)
    }
    hash
  }
  ```

  Integer literals that fit are fixed-width values where one is expected: `src[i] ?? 0`. One that
  doesn't fit, as in `let b: Byte = 300`, is an error. An ASCII character literal is a `Byte`
  where one is expected, in expressions and in patterns, so byte-level code reads as text:

  ```eye
  fn isComment(src: Bytes) -> Bool = match src {
    ['-', '-', ..] => true
    _ => false
  }
  ```

  Any other character, such as `'é'`, is more than one byte in UTF-8, and is an error where a
  `Byte` is expected.
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

  ```eye context=records
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
  loops, slices and patterns. A range is a value of type `Range(a)` with fields `start` and `end`,
  which hold the bounds as written: `(0..5).end` and `(0..=5).end` are both 5, so `end` doesn't say
  whether the range includes it. A range whose end comes before its start is empty: `5..2` holds
  nothing, and a `for` over it runs no times. A range of times of day is the exception: it wraps
  midnight, so `23:00..08:00` holds 23:30 and 07:00, as quiet hours need.
- **Slices:** `xs[2..5]`, `xs[..n]` and `bytes[i..]` are views that share storage with what they
  come from. A slice is clamped to the elements that exist, so it never fails, while `xs[i]`
  returns an `Option`.
- **Positions in strings** are `Offset`s, not `Int`s. Only a string's own functions make one:
  `s.start()` and `s.end()`, `s.indexOf(part, from: i)`, and `s.after(i)` and `s.before(i)`, which
  step over one character, or `by: n`, and are `None` past either end. `s.slice(a..b)` takes a
  `Range(Offset)` and gives an `Option(String)`, `None` when an end would split a character, and
  `s.charAt(i)` gives the character there. Offsets compare with `==` and `<`; they have no
  arithmetic. `size`, `take` and `drop` count characters, as `for` over `s.chars()` does, and a
  count is an `Int`, so it can't be passed where a position goes: with `let city = "Málaga"`,
  `city.slice(city.start()..city.end())` is `Some("Málaga")`, and `city.slice(0..(city |> size))`
  is an error, `expected Range(Offset), found Range(Int)`, which says where offsets come from.
  An `Offset` counts bytes underneath, so scanning by position costs nothing per step, but there
  is no number to read from it, since it means something only in its own string; it shows as
  `byte 5`.
- **Iteration:** `for` walks lists, sets, ranges, `Bytes`, a map's `(key, value)` pairs, a string's
  characters (`s.chars()`) and any `Iter(a)`.
- **Lazy iteration:** `xs.iter()` gives an `Iter(a)`, a sequence computed as it is walked.
  `map`, `filter`, `take`, `drop`, `enumerate`, `zip` and `flatMap` on an `Iter` give another
  `Iter` and build no lists in between (`zip` does when either side is one); a `for` loop, or a
  function that needs the elements, walks it, and `toList` collects it:

  ```eye
  let squares = (1..1_000_000_000).iter() |> map(fn n: n * n) |> filter(fn n: n % 3 == 1)
  let few = squares |> take(3) |> toList    # [1, 4, 16]: squares four numbers, not a billion
  for (i, c) in "héllo".chars() |> enumerate { print(i, c) }
  ```

  - **Sources.** Lists, sets, maps (their `(key, value)` pairs), ranges of `Int`, strings (their
    characters) and `Bytes` have `iter()`; `s.chars()` is `s.iter()`, and an `Iter`'s `iter()`
    is itself. Every source ends, so `size`, `toList` and `sort` on an `Iter` always finish; a
    large range stands in for an endless count.
  - **An `Iter` is a value.** Walking it twice gives the same elements: each walk starts again
    from the source and runs the functions again. An `Iter` that a first walk used up would be a
    cursor shared by everyone holding it, the shared state section 1 rules out. To walk once and
    keep the results, `toList`.
  - **Its functions only compute.** They run whenever and wherever the `Iter` is walked, so a
    function given to `map`, `filter` or `flatMap` on an `Iter` has no effects: `print` belongs
    in the loop that walks it. A callback parameter given to one states `uses {}` in its type,
    as a function kept in a record does (section 12).
  - **What walks it.** `toList`, `toSet`, `reduce`, `any`, `all`, `find`, `first`, `last`,
    `size`, `isEmpty`, `count`, `contains`, `sum`, `join`, `sort`, `reverse` and `parallelMap`
    walk an `Iter`, and `first`, `find`, `any`, `all`, `contains` and `isEmpty` stop at the
    answer. An `Iter` has no positions, so `get`, `xs[i]` and slices don't take one.
  - **It can't be kept.** It holds functions, so it has none of `Eq`, `Ord`, `Show`, `Hash`,
    `Save` and `Schema`: it can't be compared, printed or saved, so it can't be live across a `!`,
    and a job's `for` loop over one can't wait inside: collect it with `toList` first, so a
    resumed run finds its place.
- **Entity references:** `Ref(World)` is an ID, not a pointer. It can be stored, sent and saved
  anywhere, and it can't be confused with `Ref(Player)`.
- **Special types:** `Secret(a)` (no printing, no saving), `Approval` (only a human approval flow
  can create one; its `what` and `by` say what was approved and by whom), `Key` (an idempotency
  key), `Version` (the version of some state, which a `@fresh` command names; it has `==` and no
  order, and an integer literal can be one), `Run(a)` (a started job). A value that foreign code owns has an `extern type` (section 18).
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

```eye context=bindings
let x = 5                    # immutable, block-scoped; a later `let x` shadows it
var total = 0                # mutable local; closures can't capture a `var` (section 7)
total += x

let inc = fn n: n + 1                                    # lambdas
let add = fn (a, b): a + b
let show = fn (item: Item) {
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
  log.info("tried {retries} times")
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
  function may leave out parameter types too. They are inferred from its body alone, and what the
  body leaves open is generic: each call decides it afresh, so `fn twice(n) = n + n` adds `Int`s at
  one call and `Float`s at the next, and a call that gives it a type without `+`, such as `Bool`,
  is an error at that call. A body that reads a field or calls a method on such a parameter needs
  its type written, since nothing in the body says which record it is. `while true` without a
  `break` never ends but by `return`, so it can stand where any value is expected.
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
  `var` or a `mut` parameter. A `mut` parameter is always passed that way, and only a `mut`
  parameter is. A method that takes `mut self` can only be called on a `var` or a `mut` parameter.
- **No aliasing.** The same variable can't be passed as `mut` twice in one call, or passed as `mut`
  and read in the same call: `swap(mut a, mut a)` is a compile error. Closures can't capture a
  `var` or a `mut` parameter. An entity method's closures may read its state fields, though
  (section 14).
- **No references.** A `mut` parameter lasts for the call; it can't be stored or returned. Nothing
  in the language is a pointer, so values can't form cycles, and reference counting reclaims
  everything without a garbage collector.
- **In place when unique.** A change to a value that nothing else refers to happens in place. If
  something does, the value is copied first (copy on write), so the change stays local.
- **Entity methods** are `mut self` methods over the entity's state, run as a transaction
  (section 14).

## 8. Methods and generics

```eye context=lexer
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

- **Methods** are declared in `impl Type { … }` in the module that defines the type (an `impl` of a
  contract goes anywhere, section 11), and called as
  `x.method(args)`. A function in the block without `self` is called on the type: `Lexer.new(src)`.
  Trait methods are called the same way (section 11).
- **A field that holds a function is called like a method.** With `type Timer = { name: String,
  read: fn() -> Instant uses {clock} }`, `timer.read()` calls the function in the field; the call
  has the effects its type states. So that the call means one thing, a record type can't have a
  method named like a field that holds a function.
- **Pipes and methods.** `x |> f(a)` calls an ordinary function with `x` first; `x.f(a)` calls a
  method. Collection operations in the standard library are functions, used with pipes, so the same
  pipeline works on lists, iterators and entity sets. The built-in functions can also be called as
  methods today (`xs.first()` is `first(xs)`); calling any function whose first parameter takes
  the value as `x.f(a)` was decided on 2026-10-12, and ROADMAP task G.15 builds it and writes the
  rule here.
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

  ```eye context=dyn
  fn notifier(config: Config) -> dyn Notifier =
    if config.telegram then Telegram.new(config.token) else Console.new()

  let sinks: List(dyn Sink) = [Logger.new(), Metrics.new(port: 9100)]
  ```

  - A value converts to `dyn Trait` where one is expected, if its type implements the trait:
    `expected dyn Notifier, found Email: Email doesn't implement Notifier` otherwise. Since values
    never alias, a `List(Logger)` is also a `List(dyn Sink)`. Nothing converts back.
  - A `dyn Trait` has its trait's methods and nothing else: no `Show`, `Eq` or `Save`, so it can't
    be printed, compared, or live across a `!` (its type, which saving needs, is known only at run
    time).
  - A trait takes its type arguments as in a bound, `dyn Add(Money, Money)`, and a trait with a
    method that has type variables of its own can't be used this way, since a call through `dyn`
    couldn't choose them.

  A function that takes a trait as a parameter type (`fn label(x: Named)`) is still generic and
  specialised; `dyn` is for values whose types differ at run time.

## 9. Pattern matching

```eye context=matching
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
(section 13).

Every `match` must be exhaustive. Adding a variant to a type makes each `match` that doesn't handle
it a compile error, which is how a new outcome from another service reaches every caller.

## 10. Errors: Result only

- Expected failures are values: `Result(T, E)` and domain variants such as `Refused` and
  `Uncertain`. Postfix `?` returns the error to the caller: `let p = Player(id).profile()?`. In a
  function that returns an `Option`, `?` on an `Option` returns `None`.
- **`?` needs somewhere to return to.** On an `Option` it needs a function that returns an
  `Option`, and on a `Result` one that returns a `Result`; anywhere else it is an error that says
  what the function returns. For a `None` there, give a default with `??`, match the `Option`, or
  make the function return an `Option`. The same holds in a job and in a lambda, and a lambda's
  `?` returns from the lambda, not from the function it is written in, so the lambda's own result
  decides, and an error is wrapped into the lambda's error type.

  ```eye
  fn next(x: Option(Int)) -> Int = x? + 1          # error: next returns Int, not an Option
  fn nextOr(x: Option(Int)) -> Int = (x ?? 0) + 1
  fn doubled(xs: List(Option(Int))) -> List(Option(Int)) = xs |> map(fn x: Some(x? * 2))
  ```
- **Converting errors.** When the caller's error type has exactly one variant whose payload is the
  error's type, `?` wraps the error in it. With none or several, write the conversion:
  `.mapErr(Variant)?` or `.mapErr(fn e: …)?`.

  ```eye
  import fs

  type CountError = | Usage | Unreadable(fs.FsError) | BadNumber(String)

  fn count(path: String) -> Result(Int, CountError) uses {fs} {
    let text = fs.readText(path)?        # an fs.FsError becomes Unreadable(…)
    let n = parseInt(text)?              # parseInt's message becomes BadNumber(…)
    Ok(n)
  }
  ```
- **Programs that only report errors** can use the standard library's `Problem`: `?` turns any
  error with `Show` into a `Problem`, and `.context("reading {path}")` adds a line to it. A
  `Problem` shows its lines outermost first, then the error: `reading eye.toml: not a whole
  number: "x"`.
- There are no exceptions. Library functions are total: `xs[0]` and `first(xs)` return `Option`,
  and `parseInt(s)` returns `Result`.
- **`unreachable("why")`** is the one way for code to stop itself. It ends the program, or the
  job's run, as a fault: `Failed(Unreachable("why"))`. It is for states the code has already ruled
  out, such as an index a loop just checked, never for failures that can happen.
- **Arithmetic is total.** `Int` has arbitrary precision, so it can't overflow. Small values stay
  unboxed at run time. `/` and `%` take a `NonZero(Int)` divisor. Literals other than `0` qualify
  automatically; for any other value, `nonzero(n)` returns `Option(NonZero(Int))`. `Float` follows
  IEEE 754, so dividing by zero gives an infinity rather than a failure.

  ```eye context=division
  let avg = total / 3
  match nonzero(count) {
    Some(n) => total / n
    None => 0
  }
  ```

  Two kinds of division come up often enough to need no `match`. `xs.average()` is the mean of a
  list of `Int`s or `Float`s as a `Float`, or `None` for an empty list: `[1, 2].average()` is
  `Some(1.5)`. It gives a `Float` for `Int`s too, so its type is the same for every list and a
  mean of `1` and `2` isn't quietly `1`. `a.div(b)` and `a.rem(b)` are `a / b` and `a % b` for
  `Int`s, or `None` when `b` is `0`. They round as `/` and `%` do, down, so `div(-7, 2)` is
  `Some(-4)` and `rem(-7, 2)` is `Some(1)`.

  ```eye
  fn summary(latencies: List(Int), sent: Int, failed: Int) -> String {
    let mean = latencies.average() ?? 0.0
    let failedPerHundred = (failed * 100).div(sent) ?? 0
    "{mean} ms on average, {failedPerHundred} of every 100 failed"
  }
  ```

  For an `Int` mean that rounds as `/` does, write `xs.sum().div(xs.size())`.
- **Runtime faults** (out of memory, over budget, killed, deadline passed, `unreachable`) are not
  errors that code can throw or catch. They end the run, and whoever started it sees `Failed(fault)` as the run's
  outcome.

## 11. Traits

```eye context=named
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
  `notifier` serves `notify.Notify` (section 17).
- `Eq`, `Ord`, `Show`, `Hash`, `Save` and `Schema` are derived automatically for records, variants and
  collections of types that have them. Derived `Ord` orders a type's variants as they are declared,
  then by their fields, and a record's fields in the order the type declares them, so
  `type Priority = Low | Normal | High` has `Low < High`. An impl of `Show`, with its `show`
  method, replaces the derived form wherever the value is written.
- What uses them asks for them: `==` needs `Eq` and `<` `Ord`; `"{x}"`, `print` and `log` need
  `Show`; `sort`, `min` and `max` need `Ord`, `toSet` `Hash`, `sum` `Add`; whatever a job saves
  needs `Save`, and `ai.generate` the `Schema` of the type it asks for.
- `Secret(a)` deliberately has none of `Show`, `Save`, `Eq`, `Ord`, `Hash` or `Schema`, and nor
  does a value of a program's `extern type` (section 18). A function has no `Show`, `Save` or
  `Schema`. Printing a secret, or holding one across a `!`, is a compile error, and so is a record
  that holds one in either place.
- **Enums list and read their variants.** A type whose variants have no fields gets two functions
  on the type, derived as the traits are: `T.variants()`, every variant in the order declared, and
  `T.parse(s)`, `Some` of the variant whose name is `s`, else `None`. A choice that arrives as text,
  such as a pressed button, is then matched as a variant, so a misspelt name or a forgotten choice
  is a compile error:

  ```eye context=admission
  type Admission = Guest | Member | Refuse

  fn admit(choice: String, user: UserId) -> Step =
    match Admission.parse(choice) {
      Some(Guest) => Admit(user, groups: toSet(["guests"]))
      Some(Member) => Admit(user, groups: toSet(["members"]))
      Some(Refuse) | None => Reject(user)
    }

  let buttons: List(Button) = Admission.variants() |> map(fn a: { id: "{a}", label: "{a}" })
  ```

  `parse` takes the variant's name, which is what the derived `Show` writes, so
  `T.parse("{v}")` is `Some(v)` unless an impl of `Show` writes something else. A record, or a
  type with a variant that has fields, has neither function: `Shape.variants()` is a compile
  error that names the variant with fields. A method the type declares with either name is
  called instead. Whether a choice's label should come from its `##` comment or from `Show` is
  open question 15.
- Trait calls are resolved at compile time where the type is known, so they cost nothing extra.
- **Operators are traits.** `a + b` calls `Add`, and likewise `-` (`Sub`), `*` (`Mul`), `/`
  (`Div`), `%` (`Rem`) and unary `-` (`Neg`); `==` uses `Eq` and `<` uses `Ord`. A trait names the
  right-hand type and the result, and a type implements it at most once per right-hand type, so
  the result is always known from the operands:

  ```eye context=money
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
  `Size + Size`, `Int / NonZero(Int)`, and `String + String` and `List(a) + List(a)`, which join.
  Nothing converts implicitly: `1 + 2.0` is an error.

## 12. Effects and signatures

The compiler infers what each function touches. Public signatures state it with `uses`, so a new
permission always shows up in the diff.

```eye context=llm
pub fn summarise(items: List(Item)) -> String uses {wait, net Llm.complete} {
  llm.complete!(prompt(items))
}
```

| Effect | Comes from | What the compiler does with it |
|---|---|---|
| `wait` | any `!` call, and a call of a function that waits, such as `sleep(1 s)` outside a job | Compiles the function to a resumable state machine; see waiting functions below |
| `net C.op` | calling an operation on contract `C` | Derives network policy, credentials and allowed-caller checks |
| `publish C.event`, `consume C.event` | publishing contract `C`'s event (`bus.pinged.publish!(…)`), or waiting for it or starting on it (`next!(bus.pinged)`, `@on(event(bus.pinged))`) | Derives message-queue permissions |
| `start J` | starting a run of job `J`, `digest.start(day)` | Follows starts through calls, so `main` can't start a job without a key through a function it calls (section 13) |
| `entity E` | reading or changing entities of type `E` | Database grants; includes the type in backups |
| `secret "name"` | reading a secret, `secret("telegram-bot-token")` | Delivers that secret only to code that uses it |
| `clock`, `random` | `now()`, `random.int(1..=6)` | Lets tests replace them |
| `fs`, `process`, `stdio`, `env` | files, running programs, standard input and output, environment variables | Lets embedders and sandboxes grant them one by one |
| `ffi` | calling an `extern fn` (section 18) | Marks code the compiler can't check |

Every effect in the table is inferred, and a private function's effects never need writing. A
public function that publishes an event, reads a secret, starts a job or calls an `extern fn`
lists it in `uses` as the table names it, as one that calls a service lists `net C.op`:
`uses {publish Bus.pinged}`, `uses {secret "telegram-bot-token"}`, with the name as passed to
`secret(…)`, `uses {start digest}`, with the job's name, and `uses {ffi}`. A bare kind covers all
of its kind: `net` every operation, `publish` and `consume` every event, `entity` every entity,
`start` every job, and `secret` every secret, which is what reading a secret by a name that isn't
a literal needs. An `extern fn` that states its own `uses`, as `http.get` states `net Http.get`,
has those effects instead of `ffi`.

- **Pure means no `uses`.** A public function without a `uses` clause is pure, and the compiler
  holds it to that: if `area` read a file, the error would say `area uses fs, which its signature
  doesn't list: run eye fmt, or add uses {fs}`. Editors show the inferred effects of private
  functions.
- **Public entity methods and jobs state theirs too.** A `pub fn` in an entity and a `pub job`
  are boundaries like any public function: `pub fn join(p: PlayerId) uses {start wake}` in the
  tour, and a `pub job` lists everything it does, `wait` included. What a method does to its own
  entity's state isn't an effect; calling another entity's method is `entity E`.
- **`eye fmt` writes the clauses.** Formatting a program's directory, `eye fmt DIR`, checks the
  program and writes each public function's, entity method's and job's `uses` from what its body
  does: it adds a missing
  effect and drops one the body no longer has. An entry that still covers an effect stays, as
  `net` stays while the body calls any service; a clause that changes is written in sorted order.
  A file formatted alone (`eye fmt FILE`, or an editor formatting on save) is formatted for syntax
  only, and so is a program with other errors, which `eye fmt` reports. A private function's
  clause is a limit its author sets, so `eye fmt` leaves it as written.
- **Callbacks pass their effects to the call.** A function that takes a function lists only its own
  effects; calling it adds the effects of the function passed in, where the call is written. `map`
  is pure, and `paths |> map(fs.readText)` uses `fs`:

  ```eye
  import fs

  pub fn missing(paths: List(String)) -> List(String) uses {fs} =
    paths |> filter(fn p: !fs.exists(p))    # fs comes from the lambda, not from filter
  ```

  So `retry`, `timeout`, `race`, `parallelMap` and `reserve!` work with any function, and every
  public signature still lists everything its body can do.
- **Functions stored in data state their effects.** A function type without `uses` is pure,
  except in a parameter's type, where it takes a callback (below). One kept in a field or a
  collection, and called later, says what it may do:

  ```eye
  type Handler = { name: String, run: fn(Event) uses {net Notify.send} }
  ```

  In a function's return type, the `uses` after a function type belongs to that function type.

  Whoever calls a stored function later counts only what its type states, so a function kept
  there may do no more: `{ name: "n", run: fn e: now() }` is an error, `this function uses clock,
  which its type here, fn(Event) uses {net Notify.send}, doesn't list`. The same holds for a
  lambda or a named function put in a list, returned, or passed where a parameter's function type
  has a `uses` clause, which is then a limit. A function type without `uses` anywhere in a
  parameter's type, as in `map`'s `fn(a) -> b` or inside `race`'s `List(fn() -> a)`, takes a
  callback, which may do anything (the parameters and result of that function type keep the
  general rule);
  so a callback can't be kept in data, nor can a lambda that calls one. To keep it, state its
  effects in the parameter's type: `fn register(run: fn(Event) uses {clock}) -> Handler`. A
  stored function given as a callback, as in `xs |> map(h.run)`, does what its type states, which
  counts where it is given.

**`!` in jobs.** Inside a job, `!` marks every recorded step: it runs once and its result is
recorded, so a resumed run never repeats it and sees the same values. The steps are:

- every call with an outside effect (`net`, `fs`, `process`, `clock`, `random`, `env`, `ffi`,
  `secret`, `entity`, `publish` and `start`), an `extern fn` included (`open!(path)`). That
  holds for a function of the program too: `stamp!()`, where `fn stamp() = now()`, is one step, and calling it without
  `!` is an error, `in a job, stamp uses clock, so calling it is a durable step: write stamp!(…)`;
- entity calls (`w.read!()`, `w.markAsleep!()`), each committed in the same transaction as its
  checkpoint (section 15), and reading a view or every entity of a type, `idleWorlds.read!()`
  and `World.all.read!()`;
- publishing an event, `bus.pinged.publish!({ n: 1 })`, which is sent when the step commits;
- starting a child run, `digest.start!(day)`, so a resumed run doesn't start it again.

Calls that only compute are ordinary, and `!` on one is an error. Outside jobs the same functions
are called without `!`: in `main` (section 13) and in tests. Entity methods and served operations
can't make most of these calls at all: they are transactions, which stay inside the program
(section 14). **Output is not a
step:** `print` has the effect `stdio`, which public signatures list, but in a job it is written
without `!` and isn't recorded, and logging (`log.info`) is not an effect at all. Output is for
people watching a run, not part of what the run computes.

**Waiting functions.** A function whose body has `!` steps, such as `summarise` above, is a
*waiting function*. It has `wait` in its effects, and a job calls it with `!`: its steps are
recorded as the job's own, and replayed with them (section 15). A waiting function writes every
wait with `!`, and calls another waiting function with `!` too; as in a job, an entity call,
publishing, starting a child run and reading entities or views are `!` steps in it, and an error
without one (`in a waiting function, an entity call is a durable step: write bump!(…)`). Only a
run records steps, so
neither `main` nor a test can call a waiting function, with `!` or without: `summarise has
durable steps, which only a job records, so main can't call it: start a job that does`.

- **Outside a job, a wait blocks.** `sleep(1 s)`, `sleepUntil(…)` and `poll(…)`, written without
  `!` in `main`, a test or a plain function, hold the caller until they return; in a test they
  move the test's clock. They still wait, and so does a function that calls one, so the call
  keeps `wait` in the caller's effects: `pub fn pause() uses {wait} { sleep(1 s) }`. A job can't
  call such a function, with `!` or without, since the run couldn't record its wait: `pause waits
  through calls without !, which block outside a job, so a run couldn't record its wait: write the
  waiting calls in pause with !, as sleep!(…)`. `next`, `approval` and `reserve` wait for what
  only a run can wait for, so outside jobs and waiting functions they are errors.

**A step's result is saved.** It is recorded so that a resumed run sees the same value, so it must
be something that can be saved (section 15). A `Secret`, or a value of an `extern type`, is used
inside a function that returns what can be saved, and that function is called as one step:
`checksum!(path)` rather than `open!(path)` followed by more calls on the handle. An error says so:
`this durable step's result is recorded, and a Secret can't be saved: use it in a function that
returns what can be, and call that with !`.

```eye context=bus
import bus

job ping(day: Date) {
  bus.pinged.publish!({ n: 1 })              # a step: published when it commits
  let child = digest.start!(day)             # a step: a resumed run doesn't start it twice
  log.info("started {day}")                  # not a step
  sleep!(1 min)
}
```

## 13. Programs and concurrency

A program starts at `pub fn main`. It is an ordinary function, not a job: its effects happen
directly, without `!`, and nothing about it is durable.

```eye
import fs

pub fn main(args: List(String)) -> Result((), fs.FsError) uses {fs, stdio} {
  for path in args |> drop(1) {
    let text = fs.readText(path)?
    print("{path}: {text |> lines |> size} lines")
  }
  Ok(())
}
```

- `main` returns `()` or `Result((), e)`. An `Err` is printed with `Show`, and the program exits
  with status 1.
- **Files, programs and random numbers** come from the standard modules `fs`, `process` and
  `random`, each with the effect of its name. `fs.readText`, `writeText`, `appendText`, `list`,
  `makeDir` and `remove` return a `Result` whose error is an `FsError` (`NotFound(path)`,
  `NotAllowed(path)`, …), and `fs.exists` a `Bool`. `process.run("git", args: ["pull"], input:
  "", dir: None)` runs a program without a shell and gives `Ok` with its exit status and output
  for any status, `Err(NotStarted(…))` when it couldn't start. `random.int(1..=6)` (an `Option`,
  `None` for an empty range), `random.float()`, `random.pick(xs)` and `random.shuffle(xs)` draw
  numbers. A relative path is relative to the directory the program was started in.
- **Which standard modules need `import`.** `ai`, `approvals`, `fs`, `http`, `process` and
  `random` are imported as a program's own modules are (`import fs`); without the import,
  `fs.readText` is `undefined name fs`. `log` and `env` are built in, as `print` is, so
  `log.info(…)` and `env.get(…)` need no import.
- **`eye run` keeps a program's jobs going.** It runs `main`, if the program has one, every time
  it starts; then, if the program declares jobs, it stays up: sleeping runs wake when their time
  comes, schedules fire, and it stops when nothing is left to wait for, or on Ctrl-C or SIGTERM.
  Entities, runs and events are kept in the program's journal (`.eye/journal`), so a restart
  carries on where the last one stopped: a run that was due while the program was down runs at
  once, and a schedule that missed firings fires once, for the latest. Because `main` runs on every
  start, a job it starts takes a key, which starts it once (`ring.start("kitchen", key:
  "kitchen")`); without one, the checker refuses the call. The rule follows calls, through the
  `start J` effect (section 12): a call in `main` of a function or entity method that starts a
  job without a key, itself or through what it calls, is refused too, `eye run runs main every
  time it starts, and kick starts ring without a key, so each restart would start it again`.
  `eye runs` lists the runs in the journal: their status, what each waits for and until when,
  and their steps.
- **`eye repl [dir]`** is an interactive session in a program, or in an empty one. An expression
  prints its value and type (`[1, 2] |> map(fn x: x * 10)` prints `[10, 20]: List(Int)`), a
  statement runs, and `let` and `var` bindings last for the session. A declaration (`fn`, `type`,
  `entity`, `job`, `view`, `impl`, `contract`, `test`, `import`) joins the session, and declaring
  a name again replaces it. Every input is checked with the whole session, so its errors are the
  compiler's, shown under the line of the input they are on. Jobs run on a clock that moves only
  with `:advance`; `:runs`, `:type`, `:doc`, `:test`, `:reset` and `:quit` are the other
  commands.
- **Modules.** A module is a directory: its `.eye` files share one namespace and one set of
  imports. A program is the module in its directory and the modules it imports. `import lab.notify`
  names the directory `lab/notify` below the program's directory; its public names are then used as
  `notify.Delivery`, or brought in by name with `import lab.notify.{Delivery}`. A variant of an
  imported type may be written alone, `Accepted(…)`, when no other imported module has a variant
  of that name and the prelude has none (its `Skip` is a `GapPolicy`); otherwise it is written
  with its module, `notify.Accepted(…)`.
- **A program's root is marked by `eye.toml`.** A file belongs to the program of the nearest
  directory, from the file upward, that holds an `eye.toml`; without one, it belongs to the
  program in its own directory. So `lab/digest/digest.eye` belongs to the program in `lab/` when
  `lab/eye.toml` exists, and its `import home` finds `lab/home`. `eye check file.eye` and the
  editor's go-to-definition use this rule. The file may be empty.
- **`modules` in `eye.toml` names more directories to find imports in.** An import is looked for
  below the program's own directory first, then below each listed directory, in order; the first
  that has it wins. Paths are relative to the `eye.toml`, and each must stay inside the repository
  (or the build's context), so that a copy of the program has everything it needs. A program
  deployed alone, such as the lab's digest, is then its `eye.toml` and a file that imports what
  it runs:

  ```toml
  # testdata/lab/deploy/digest/eye.toml
  modules = ["../.."]                      # testdata/lab, where digest, home and notify are
  ```

  An import that no directory has is an error that lists every directory searched. This is the
  one key `eye.toml` holds so far; what else it will hold is open question 5.

**Concurrency** is structured. Tasks start inside one call, and all of them finish (or are
cancelled) before that call returns, so no task outlives the code that started it.

```eye context=concurrency
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
- `race` cancels the losers at their next wait. `timeout(5 s, fn { … })` returns an `Option`:
  `None`, with the function cancelled at its next wait, if it doesn't finish in time.
  `parallelMap` runs at most `limit:` tasks at once, all of them without one. If a task fails,
  the others are cancelled and the call fails with it.
- **Waits overlap.** On the system clock, a task that waits for the network, a timer or another
  service lets the others run, so two requests of 300 ms each take 300 ms together. On a test's
  clock, tasks take turns: one runs until it waits, and the one that wakes first goes next, with the
  clock moving to its wake time. A call takes as long as its longest task, in either, and every run
  of a test is the same.
- **From v0.2, task groups and channels** cover streams and work that is discovered as it goes. A
  group starts tasks with `spawn` and returns only when all of them have finished. A channel has a
  fixed capacity, and `put` parks while it is full, so a fast producer can't fill memory:

  ```eye context=concurrency
  taskGroup(fn group {
    let (send, receive) = channel(capacity: 64)
    group.spawn(fn {
      for url in feeds { send.put(fetch(url)) }
      send.close()
    })
    for page in receive { handle(page) }
  })
  ```

  - `taskGroup` runs its function as the group's first task and returns that function's result
    once every task has finished. A task may spawn more, so work found on the way joins the same
    group. If a task fails, the others are cancelled at their next wait and the group fails with
    it.
  - `channel(capacity: n)` gives a `Sender(a)` and a `Receiver(a)`, and holds at most `n` values,
    at least one. `send.put(x)` parks while it is full, `send.close()` ends it, and a put after
    that fails. `for x in receive` reads until the channel is closed and empty, and
    `receive.take()` gives `Some` of the next value, or `None` once it is closed and empty.
  - A group whose tasks all wait for channels that no task will put into or close would wait
    forever, so it fails instead, at the `taskGroup` call: a producer that forgets `close()` is
    a bug the test shows, not a hang.
  - `TaskGroup`, `Sender` and `Receiver` can't be saved, so channels can't be live across a `!`;
    jobs talk through entities and events.
- **In jobs**, concurrent durable steps are child runs (`digest.start!(day)`) joined with
  `allDone!` or `firstDone!`. A task a job starts with `parallelMap`, `both`, `race` or `timeout` only computes,
  since its steps couldn't be recorded in order: a `!` step in it, or a call that would need one,
  is an error.

## 14. Entities

```eye context=players
@version(3)
entity Player(id: PlayerId) {
  state name: String = ""                    # v3: renamed from nick
  state devices: Set(DeviceId) = {}          # v3: added

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
  An `Err` is a value like any other: a method that returns one commits what it changed. Only a
  fault (section 15), such as `unreachable`, rolls the call tree back, and with it the jobs it
  started and the events it published, which never happen.
- **A call tree that comes back to an entity shares its state.** The call tree is one
  transaction, and in it each entity has one state in progress, which every call on the entity
  reads and changes. A call that comes back to an entity whose method is still running is not a
  second writer: it starts from what the tree has changed so far, and the entity commits once,
  with every change.

  ```eye
  entity Ping(id: String) {
    state n: Int = 0

    pub fn ping() -> Int uses {entity Ping, entity Pong} {
      n += 1
      let before = n   # 1
      Pong(id).pong()  # comes back to bump, which sees n == 1 and makes it 2
      n                # 2; before is still 1
    }

    pub fn bump() { n += 1 }
  }

  entity Pong(id: String) {
    pub fn pong() uses {entity Ping} { Ping(id).bump() }
  }
  ```

  After `Ping("x").ping()`, `Ping("x").n` is 2, at version 1.
  - **The outer method goes on from the inner call's changes.** A method's state is its `mut
    self` (section 7), and a call that may come back to the entity passes it on, the way
    `unpin(p)` passes it to another method of the entity: the call takes the state as it is and
    gives it back changed. So after `Pong(id).pong()` returns, `n` in `ping` reads 2. A value
    read before the call keeps what it read, as values do: `before` stays 1. So does the first
    `n` in `n = n + c.more()`, when the call `c.more()` comes back and changes `n`: the
    assignment writes over that change. `n += x` evaluates `x` before it reads `n`, so
    `n += c.more()` keeps both.
  - **Reads in the tree see the state in progress.** `Ping(id).n` or `Ping(id).read()` in
    `pong` gives `ping`'s changes so far, at the version the entity was loaded at; nothing outside
    the tree sees them until it commits.
  - **An end in a call that comes back ends the entity for the whole tree.** When `pong` calls a
    method of `Ping` that runs `end(ok)`, `Ping` ends when the tree commits. Its methods still
    running, such as `ping`, run to the end, and what they change after the end is dropped with
    the rest of the state, as in the method that called `end`; a new call on `Ping`, or a read
    of it, in the same tree faults.
  - **Jobs and events from every level commit together.** A job started or an event published in
    any call of the tree, on any entity, starts or is published when the tree commits, in the
    order the calls made them; a fault anywhere in the tree drops all of them with every
    entity's changes.
- **Ending an entity needs an Approval.** `end(ok)` in one of the entity's methods ends it when
  the transaction commits. `ok` is an `Approval`, which only `approval!` makes (section 15), so
  retiring an entity takes a person's consent, and `end` without one doesn't compile. The
  `Approval` isn't matched against the entity: it shows that a person said yes, and the store
  keeps its `what` and `by`, with when the entity ended, for the record.
  - **`end` is resolved by context.** In an entity's own method, `end(…)` is the entity's, as its
    methods are, unless a local, a state field or a method of that name is nearer. Elsewhere
    `end` is a string's end position, `s.end()`. `end` can't be called on another entity, which
    ends itself through a method of its own, such as `retire`; the checker says so.
  - **What an ended entity is.** Its state is gone, and the store keeps only that it ended, when,
    and the approval's `what` and `by`. It leaves every view, so `@on(leaves(…))` jobs start, and
    `Player.all` no longer lists it. Its `Ref` stays a valid value whose identity can be read
    (`p.id`), but calling a method on it or reading it (`read()`, a state field) faults:
    `Player("ann") ended at 2026-10-12 09:00 UTC, approved by operator ("Retire ann?"), so it
    can't be read or changed`. The identity stays ended: using it never starts the entity again,
    so a job holding an old reference can't bring it back by mistake. A `leaves` job that the
    end starts gets the reference; what it needs of the state is passed on before the end.
  - What the method changes after `end(ok)` is dropped with the rest of the state, and a later
    call on the entity in the same transaction faults.
- **State and `mut self`:** an entity method is a `mut self` method over the entity's state
  (section 7). A `state` field such as `devices` is short for `self.devices`, and a method is
  called by its name too: `unpin(p)` in another method is `self.unpin(p)`, in the same
  transaction. A pipe still calls a function, never a method: `shapes |> add(s)` is the collection
  function `add`, even in an entity that has a method of that name. The state is loaded when the
  transaction first calls the entity and saved when it commits; nothing outside the transaction
  sees a change until the commit.
  - **A method's closures may read state fields**, unlike a `var` (section 7):
    `players |> filter(fn p: p != owner)`. A closure gets each field's value as it was when the
    closure was made, as it gets a `let`'s, and can't change one: `a closure can't change the
    entity's state`.
- **Methods stay inside the program.** A method is a transaction, and the runtime may run one
  again after a conflict, so it does only what can happen twice without anyone outside seeing it:
  it changes its state, calls pure code and other entities' methods, ends its entity with
  `end(ok)`, starts jobs (`wake.start(id)`) and publishes events. A job begins, and an event is
  sent, only if the transaction commits. A method never waits and never reaches outside the
  program: a call that uses `net`, `fs`, `process`, `clock`, `random`, `env`, `ffi`, `secret` or
  `wait`, itself or through the functions it calls, is an error that names the effect and says
  where the call belongs: `now uses clock, which an entity method can't: it runs as a
  transaction, which may run again after a conflict, so what reaches outside the program or
  waits belongs in a job, which the method can start`. A method that needs the time takes it as
  an argument, which a job reads with `now!()`, so a retry sees the same time.
  - **Output is allowed.** `print` and `log.info` write their line again when a transaction is
    retried, which is harmless: output is for people watching, not part of what the method
    computes (section 12).
  - **A served operation** (`impl Contract`, section 17) is a transaction too, and follows the
    same rule.
- **Reading from a job:** `w.read!()` returns a snapshot, whose type is the entity's name
  (`fn due(me: Traveller)` takes one): the entity's state, its identity and a `version` field, a
  `Version` that counts the transactions that changed the entity: 0 when it is new, then one more
  for each. Snapshots go stale, and commands can name the version they were based on (see
  `@fresh` in section 17). Outside a job, in a test for instance, `w.read()` gives a snapshot without `!`;
  in a transaction that has called `w`, it gives the state in progress.
  Since the snapshot's `version` is the transactions', a state field can't be named `version`:
  `a snapshot's version counts the transactions that changed Doc, so a state field can't be
  named version: name it revision, or what it counts`.
  - **A field of a reference.** Outside jobs, `w.phase` is short for `w.read().phase`. In a job,
    and in a waiting function, it would read the entity without recording it, so a resumed run
    could see another value; it is an error that says to read a snapshot: `in a job, w.phase reads
    World without recording it, so a resumed run could see another value: read a snapshot with
    w.read!() and take phase from it`. The identity, `w.id`, reads without loading the entity,
    so it is allowed everywhere. A job reads views and `World.all` with `read!()` too
    (section 15).
- **Evolution:** `@version(N)` numbers the shape of an entity's stored state (1 without it), and
  every row records the version it was saved with; it isn't the snapshot's `version`, which counts
  transactions.
  - Adding a field with a default needs no new version: a row without the field gets the default
    when it is loaded, and a field the entity no longer declares is dropped.
  - Any other change, such as a rename or a new field without a default, raises `@version` by one
    and adds `migrate from` the old version: a pure function from the row as it was stored at that
    version to the state at the next one. `old` is that row; the old shape isn't declared anywhere,
    so its fields are read by name and checked when the migration runs. The migration to the
    current version gives the state the entity declares, which the checker holds it to: every
    field without a default, each of its type. An older one gives the next version's shape.
  - Migrations chain. A row saved at version 1 of a version-3 entity runs `migrate from 1`, then
    `migrate from 2`, when it is next loaded, and is saved at version 3 the next time a transaction
    changes it. So an entity has a migration from each version between its oldest one and the
    current one, with no gaps; dropping the oldest migrations retires the rows saved before them.
  - `eye run` reads every stored row's version before it starts, and refuses to start if one is
    newer than the program's or older than its oldest migration: `3 Player saved at version 1, and
    Player has no migrate from 1`.

## 15. Jobs and waiting

A job is a durable function: it can pause for any length of time and holds no thread while paused,
only its saved record.

```eye context=notify,jobs
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

- **`!` marks a step; a checkpoint is where a run resumes.** Every recorded step in a job is a
  `!` call (section 12). A checkpoint is the run's saved place. A resumed
  or restarted run continues from its last checkpoint: no recorded call runs again, and it sees
  the values it saw. The interpreter keeps a checkpoint at the start of each statement of the
  job's body, the statements inside its loops, ifs and matches included, once the run has taken
  steps since the last: the job's variables, the statement, and where each loop around it is.
  A run that wakes restores them and replays only the steps since, each returning its recorded
  result. A statement that calls a function with `!` steps of its own replays that function's
  steps, so a loop that never ends belongs in the job's body, not in a function it calls once.
  While a variable holds what can't be saved, the run keeps its earlier checkpoint. Compiled jobs
  save their live variables at each `!` (section 22).
- **A paused run resumes only where its code still is.** A checkpoint names the statement it
  resumes at by its place in the job and its code, with types; the job must keep as many
  variables. When the code changes under a paused run so that the statement is gone or reads
  differently, `eye run` refuses to start, as it does for entity rows no migration can bring
  forward, and says which job and how many runs: `the code of a job changed where its runs are
  paused, so they can't resume: watch (2 paused runs)`. A change elsewhere in the job is fine. To
  change a job where its runs are paused, keep the old job under its name until its runs finish
  and give the new code a new name, or end the old runs. A step replayed after the checkpoint
  whose call differs from the one recorded, as when a function the statement calls changed,
  faults the run: `the code of watch changed while this run was paused: its step 12 called
  notify.send, and the code now calls sleep there`.
- **Long runs.** A run's saved state is its variables and the steps of the statement it is in,
  so a job that loops for weeks, such as the lab's train watcher, costs no more on its last day
  than on its first. `eye runs` keeps the last 20 steps a checkpoint dropped, and counts the
  rest.
- **Only values that can be saved** (types with `Save`) may be live across a `!`. Holding a socket,
  file handle or `Secret` across a `!` is a compile error.
- **A job reads stored state only in steps.** A resumed run must see what it saw, so every read
  of entities in a job is a recorded step: `w.read!()` gives one entity's snapshot, and
  `idleWorlds.read!()` a view's snapshots, as a list, as `World.all.read!()` gives every
  `World`'s. The run records the list, and a replay returns it, even when the entities have
  changed since. A view, `World.all` or a state field read any other way in a job is an error
  that names the step to write: `in a job, reading idleWorlds isn't recorded, so a resumed run
  could see other entities: read it with idleWorlds.read!(), a recorded step that gives a list
  of snapshots`. A waiting function follows the same rule, since it runs in the job's run.
  Outside jobs a view and `World.all` are those lists already, and `read()` gives the same.
- **Entity calls from jobs** (`World(w).markAsleep!()`) commit in the same store transaction that
  records the step in the run, so they happen exactly once.
- **Where runs live.** Entities, runs and checkpoints are kept by a store, chosen when the program is
  started or embedded. The default store keeps them in memory and is itself written in Eyelang
  (`MemoryStore` in `check/runtime/store.eye`, part of every program): it is an immutable value, each
  transaction is a pure function from one store value to the next, and the runtime holds the only
  reference, so compiled code updates it in place. (The interpreter has no reference counts, so it
  copies what changes; the store keeps runs in buckets of 64 and rows by entity type to keep that
  small.) The trait `Store(s)` in the same file says what a store does, and `MemoryStore`
  implements it; the runtime, written in Go, calls the store through a Go interface with the
  same methods. It is what tests, the REPL and
  embedders use; on its own it doesn't survive a restart. For one machine, the journal (question 9)
  appends each commit to a file and syncs it before the commit counts, which is what `eye run`
  uses by default. The Postgres store (`eye run --store postgres://…`, or `$EYE_STORE`) keeps the
  same state in a database, so it survives the loss of the machine: each commit is one SQL
  transaction, written before it counts, and a commit made while the database is down or not
  answering changes nothing and is tried again, so the runner carries on once the database is back.
  One runner writes a store at a time, held by an advisory lock; one database holds several
  programs' stores, one per schema. Both wrap the in-memory store, and programs don't change when
  the store does.
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
  | `allDone!([a, b])`, `firstDone!([a, b])` | every one, or whichever finishes first, of several runs |

- **Joining several runs.** `allDone!` and `firstDone!` take a list of runs, `List(Run(a))`, which
  a job started with `start!`. `allDone!(runs)` waits until every one has finished and gives their
  outcomes, a `List(Outcome(a))` in the list's order. `firstDone!(runs)` waits until one has
  finished and gives its `Outcome(a)`; if several had finished before it was called, the one that
  finished first. It stops the others, which end as `Failed(Killed)` with the steps they took, so
  a wait that lost doesn't go on: an approval that timed out no longer waits for its answer.
  `all` and `first` are only the collection functions: `all!(runs)` and `first!(runs)` are
  errors that name the wait to write, since `!` marks a call as a step and never changes which
  function a name means. A timeout is a run that sleeps:

  ```eye context=notify,jobs
  job ask(what: String) -> Decision { approval!(what, from: operator, within: 3 days) }

  job timer(d: Duration) -> Decision {
    sleep!(d)
    Expired
  }

  job ship(build: String) {
    match firstDone!([ask.start!("ship {build}?"), timer.start!(1 h)]) {
      Completed(Approved(ok)) => deploy!(build, ok)
      Completed(_) | Failed(_) => log.info("{build} wasn't approved within the hour")
    }
  }
  ```
- **Locks.** `lock!(name, for: d, fn { … })` waits until no other run holds the lock of that name,
  takes it for `d`, runs the function, whose `!` steps are the run's own, and gives the lock back
  when the function returns, with its result. The store keeps who holds each lock and until when,
  in the commit that records the step, so a restart neither frees a lock nor grants it twice. A
  run that faults, or that `firstDone!` stops, gives its locks back.
  - **A lock runs out.** When its time is up while the holder is still waiting inside the
    function, the holder ends as `Failed(LockExpired(name))` and the next run that asks takes the
    lock, so the work it guards never runs beside another holder. A holder that is computing when
    the time comes ends when it next waits inside the function. To report it, run the locked work
    as a child run and join it.
  - A run that asks for a lock it already holds faults: locks don't nest.

- **`next!` misses nothing.** It counts every matching event published since the run's previous
  step, so an answer that comes while the run is still sending its question, or sleeping, is found.
  `poll!` keeps only its outcome, so a long poll doesn't grow the run's history.
- **Approvals reach people through the program.** When a run starts waiting at `approval!`, the
  runtime publishes `approvals.requested`, an event of the standard `approvals` module, with the
  request's `id`, `what`, `from` and `until`. A program that imports `approvals` routes it with a
  job of its own, through Notify's buttons or anything else, and answers with
  `approvals.answered.publish!({ id, approved, by })`.
  - **An answer goes to the request it names.** Its `id` is the request's, so two runs that ask
    the same thing are answered one by one.
  - **Only the person asked may answer.** `by` must be the request's `from`, both as text; the
    runtime refuses any other answer, logs it, and the run keeps waiting. A routing job that
    forgets to check who pressed a button therefore can't grant consent. The check trusts the
    `by` it is given: events have no `@allow`, so any module of the program can publish
    `approvals.answered` with `by: r.from` and grant consent without a person. It guards against
    a press by the wrong person, not against the program's own code (open question 35).
  - **People at a command line, and tests, answer by text.** `eye approve TEXT` and
    `eye decline TEXT` answer through the inbox of the program's `eye run` the one waiting
    approval whose text contains `TEXT`, as the person it asks; when several do, they answer none
    and list them with their ids, and `eye approve --id ID` answers one. Tests answer with
    `approve(…)` and `decline(…)`, as the person asked unless `by:` names someone else; they fail
    the test when their text matches no waiting approval, or more than one.
  - **An `Approval` says what was approved and by whom**: `ok.what` and `ok.by`, for audit logs.
    It is still made only by `approval!`.

  ```eye context=notify,jobs
  import approvals

  @on(event(approvals.requested))
  job ask(r: approvals.Request) {
    let left = r.until - now!()
    let asked = notify.ask!(to: operator, text: r.what, choices: [yes, no], within: left,
      key: "approval-{r.id}").reconcile!()
    let Asking(question) = asked else { return }
    let Some(press) = next!(notify.answered, where: fn a: a.question == question, within: left) else {
      return
    }
    approvals.answered.publish!({ id: r.id, approved: press.choice == yes.id, by: "{press.by}" })
  }
  ```
- **A run's outcome** is `Outcome(a) = Completed(a) | Failed(Fault)`, from the prelude. A fault
  (`DeadlinePassed`, `OverBudget`, `OutOfMemory`, `Unavailable`, `Unreachable`, `Killed`,
  `LockExpired`) can't be
  caught where it happens, so code that must report one starts the work as a child run and joins
  it:

  ```eye context=notify,jobs
  fn written(day: Date) -> Result(Article, DigestError) =
    match write.start!(day).join!() {
      Completed(result) => result
      Failed(fault) => Err(Faulted(fault))
    }
  ```

  Every other fault the runtime meets ends the run as `Unreachable`, with a message that says
  what happened: a request the service refused (a 4xx reply that `status:` doesn't name, section
  17), a replayed step whose call differs from the recorded one, a call a test's fakes have no
  exchange for, and a dollar `@budget` on a model whose price isn't known (section 19). These are
  bugs in the program or its configuration, so like `unreachable` they fail the test they happen
  in, while the other five faults come from a limit or a service, not a bug, and don't (section
  20).
- **Starting jobs:** `irelandDigest.start(day)` returns a `Run(a)`. It works from entity methods (as
  part of the transaction), from `main` and tests, and from jobs, where it starts a child run and
  is written `irelandDigest.start!(day)`: the start is a recorded step, so a resumed run doesn't
  start the child again. Starting a job is the effect `start J` (section 12), so a function that
  starts one is a step in a job too, `kick!()`. Scheduled starts take
  `after:` or `at:`: `sleepWhenIdle.start(w, after: 20 min)`,
  `irelandDigest.start(day, at: 07:40 in "Europe/Dublin")`. With `key:`, a job starts once per key:
  a later start with the same key returns the run the first one started, finished or not.
- **Attributes:**

  | Attribute | Meaning |
  |---|---|
  | `@on(trigger)` | `daily(…)`, `weekdays(…)`, `every(15 min)`, `cron("…", in: zone)`, `enters(view)`, `leaves(view)`, `event(contract.event)`. A schedule passes the job the date of the firing in the schedule's zone, if the job takes one; a view, the entity's `Ref`; an event, the event |
  | `@memory(size)` | budget admitted before the run starts; the runtime enforces it |
  | `@priority(p)` | `interactive`, `normal`, `background` or `batch`; under memory pressure, a run of a lower priority doesn't start or resume while one of a higher priority waits for memory, and a run that is executing isn't paused (open question 26) |
  | `@deadline(duration)`, `@deadline(13:40, in: zone)` | how long after its start, or by what time, a run must finish; after that it ends as `Failed(DeadlinePassed)`, even while it sleeps or waits |
  | `@concurrency(n, by: param)` | at most `n` runs at once, or per value of the parameter named. A run over the limit waits in a queue and begins when a run of its group finishes, in the order they were started; a waiting run keeps its place, so `@concurrency(1)` asks one question at a time, in order. A scheduled start that finds the limit reached is skipped, not queued |
  | `@limit(n, per: duration, by: param)` | at most `n` runs begin in any window of that length, optionally per value of a parameter: `@limit(1, per: 1 s, by: chat)`. A run over the rate waits until the window lets it begin |

  All of them are enforced by the runtime (`@budget` in section 19).
- **Memory.** A runner has a memory limit (`eye run --memory-limit 512MiB`, the host's memory
  without one, 1 GiB in tests). A run holds its `@memory` only while it executes, and
  `reserve!(size, fn { … })` holds `size` until `fn` returns, across `fn`'s own waits. A run executes,
  and a `reserve!` is granted, only when the memory fits beside what is held and no run of a
  higher `@priority` waits for memory; otherwise it waits at the checkpoint it reached, so under
  pressure lower priorities pause and none overtakes a higher one. What is reserved is kept with
  the runs, so a restart neither forgets nor repeats a grant. A run whose `@memory` is more than
  the runner has, or that holds more than its `@memory`, ends as `Failed(OutOfMemory)`; the
  reference interpreter counts what it keeps for the run, its saved state with each step's result
  (open question 26).

## 16. Views and triggers

```eye context=worlds
view idleWorlds = World.all
  |> filter(fn w: w.phase == Running && w.players == {})

@on(enters(idleWorlds))
job sleepWhenIdle(w: Ref(World)) { sleep!(20 min) }   # full version in the tour
```

- A view is a named pipeline over entities. The runtime keeps it up to date as entity transactions
  commit. A job reads it as a recorded step, `idleWorlds.read!()` (section 15).
- `enters(view)` starts one run per entity that begins matching; `leaves(view)` starts one per
  entity that stops matching.
- Over entity sets, pipelines run in memory today, over the entities the store holds, with the
  same functions that work on lists. Compiling them to SQL waits for several runners on one
  database (open question 28), since one runner already holds every entity in memory.

## 17. Contracts: talking to other services

A contract is the single description of another service's interface. It is written once and shared
by eyelang and the Go, TypeScript, Rust and Python code that implements or calls it. Think of it as a
trait (an interface of operations) plus the facts needed to deploy it: port, allowed callers, keys and
how to reconcile.

```eye context=contracts
@port(3001)
@allow(digest, inbox, welcome)          # who may call; everyone else is denied
contract Notify {
  @key(required)
  @reconcile(receipt)
  command send(to: UserId, text: String, priority: Priority = Normal) -> Delivery

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
- **`@port(n)` is where the service listens.** A program that serves a contract with `impl`
  serves it on that port, so a contract that a program serves must declare one: an `impl` of a
  contract without `@port` is an error at the contract's name. Callers never write a port in
  code: a call goes to the URL the configuration gives (below), and the network policy written
  from `@port` lets callers through to that port only, so in a deployment a call configured for
  another port reaches nothing. A contract without
  `@port` describes a service outside the program's network, such as Telegram or a hosted feed
  reader: the program reaches it at the URL its configuration gives (below), and any module may
  call it.
- **`@allow(m, …)` lists the modules that may call the contract's commands and queries.** A call
  of an operation, or a use of one as a value, from any other module is an error at the
  operation's name. The contract's own module and the modules that serve it may always call it,
  and so may tests, which reach fakes or the program's own `impl`, never the network. A contract
  without `@allow` may be called by any module.
  - Each name is a module the program can import, loaded or not, so the lab's digest deployed
    alone still names `trains` in Notify's list; a misspelt or removed module is an error at its
    name.
  - `@allow(nobody)` lets no module of the program call it: the program only serves it, to
    callers outside. `nobody` takes no other names beside it.
  - `@allow` needs `@port`: the callers it lists are let through to that port, and nothing else
    reaches it (the network policy that `eye gen policy` writes). A service outside the network
    has no port to guard.

  ```eye context=allow
  # notify/notify.eye
  @port(3001)
  @allow(digest, approving)
  contract Notify {
    query identity(user: UserId) -> Identity
  }

  # trains/trains.eye
  import notify.{Identity, UserId}

  fn whoIs(user: UserId) -> Identity = notify.identity(user)    # error: trains isn't in @allow
  ```
- **`eye gen policy DIR` writes the network policy** that `@port` and `@allow` describe, as
  Kubernetes `NetworkPolicy` objects (`networking.k8s.io/v1`), one per contract with `@port`.
  Each module runs in pods labelled `eyelang.module: <module>`, its import path; the program's
  own module goes by the name of the program's directory.
  - The policy selects the pods of the module that serves the contract with `impl`. When nothing
    in the program serves it, as for a Notify written in TypeScript, it selects the pods labelled
    with the contract's own module, `eyelang.module: notify`, which that service's pods carry.
  - It lets in TCP on the contract's port from the pods of each module whose code calls one of
    its operations or names one as a value, among the modules `@allow` lets call it: the calls
    the checker refuses get no rule. Tests, a fake's `calls(op)` and the serving module itself
    don't count. With no caller, as under `@allow(nobody)`, it lets in no pod of the program;
    callers outside the program need a policy of their own, which Kubernetes adds to this one.
  - Contracts without `@port` get no policy, events go through the bus rather than the port, and
    egress isn't written: a service outside the network is reached as the cluster allows.
  - `--namespace homelab` puts the policies in that namespace; without it they name none, and
    `kubectl apply -n` decides. Each is named after the program and the contract (`lab-notify`
    for Notify in `testdata/lab`), so the policies of two programs that call one service add up
    rather than replace each other. Like generated code, the output is a build artifact.

  For the lab, whose `notifier` serves Notify:

  ```yaml
  apiVersion: networking.k8s.io/v1
  kind: NetworkPolicy
  metadata:
    name: lab-notify
    namespace: homelab
  spec:
    podSelector:
      matchLabels:
        eyelang.module: notifier
    policyTypes:
      - Ingress
    ingress:
      - from:
          - podSelector:
              matchLabels:
                eyelang.module: access
          # … approving, digest and trains likewise
        ports:
          - protocol: TCP
            port: 3001
  ```
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
  operation when the transaction commits, and from a job as a `!` step, `notify.delivered.publish!(d)`. `@key(field)` on an event
  names the field that tells one event from another, so an event delivered twice starts one run of
  each `@on(event(…))` job.
- **`@fresh`** on a command adds a required `basedOn: Version` argument and `Stale`/`Expired`
  outcomes, so an old command can't act on newer state.
  - `basedOn` comes after the parameters the command declares, and before `key`. It is the
    version of the state the caller read, as the service gave it; an entity's snapshot has one in
    its `version` field (section 14), and an integer literal can be one.
  - `Stale` means the state changed after that version, and `Expired` that what the command was
    based on is gone, so it can never apply. Neither does anything. `Expired` is the same variant
    `approval!` gives.
  - Both join the variants of the command's result, so a `match` on it handles them, and until it
    does, the result can't be passed where the declared type is expected, as with `Uncertain`.
    Reconciling resolves `Uncertain` and keeps them.
  - A command whose result isn't a variant type, or already has a variant `Stale` or `Expired`,
    can't be `@fresh`.
- **`@destructive`** on a command adds a required `Approval` argument, `approval`, after the
  parameters it declares (and after `basedOn`), and before `key`. Only `approval!` makes an
  `Approval`, so a program calls the command only after a person said yes. On a function it adds
  the same parameter, which is how a model's tool asks for a person (section 19).

  ```eye context=shelves
  # shelves/shelves.eye
  pub type Shelf = { id: String, items: List(String), version: Version }

  @port(3004)
  contract Shelves {
    query shelf(id: String) -> Shelf

    @fresh                                  # clear(id, basedOn)
    command clear(id: String) -> Cleared

    @destructive                            # burn(id, approval)
    command burn(id: String) -> Burned
  }

  # main.eye: reads again when the shelf changed in between
  job tidy(id: String) {
    let shelf = shelves.shelf!(id)
    match shelves.clear!(id, basedOn: shelf.version) {
      Cleared(n) => Log(id).add!("cleared {n}")
      Stale => {
        let _ = tidy.start!(id)
      }
      Expired | Uncertain(_) => ()
    }
  }

  # a program that serves Shelves answers from its entity's snapshot
  impl shelves.Shelves {
    fn clear(id, basedOn) {
      let rack = Rack(id).read()
      if rack.gone { return Expired }
      if rack.version != basedOn { return Stale }
      Cleared(count: Rack(id).empty())
    }

    fn shelf(id) {
      let rack = Rack(id).read()
      { id, items: rack.items, version: rack.version }
    }

    fn burn(id, approval) = Burned(count: Rack(id).burn())
  }
  ```
- **A contract's operations are its module's functions.** A module that declares one contract offers
  its commands and queries by name: `notify.send!(…)` for `contract Notify { command send … }`.
- **Serving a contract.** A program written in Eyelang serves a contract with `impl Contract { … }`,
  in any of its modules (section 11): one function per operation, named after it, with its
  parameters in order and `key` last for a
  command with `@key`. Types and defaults come from the contract. Each runs as one transaction, as
  an entity method does, so it can't wait or reach outside the program (section 14), and it may
  start jobs and publish events. A key can name an entity, so a
  repeated command finds the work it already did:

  ```eye context=serving
  impl notify.Notify {
    fn send(to, text, priority, key) = Message(key).queue({ text, to, priority })
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
  - A `Version` is a number or a string, as the service gave it; a snapshot's is a number. An
    `Approval` is `{ "what": "…", "by": "…" }`.
  - The outcomes a contract adds to a command are objects tagged with `"kind"`, whatever the
    shape of the command's own variants: `{ "kind": "Stale" }`, `{ "kind": "Expired" }` and
    `{ "kind": "Uncertain", "key": "…" }`. A reader takes `"Stale"` and `"Expired"` alone too.

  Fields that a type doesn't declare are ignored when a value is read, so a service can add some
  without breaking its callers.
- **Over HTTP.** Attributes say how a contract's operations go over HTTP. The address and the
  credentials come from the configuration, never from the code:

  ```eye context=notifyhttp
  @http(names: snake)                       # dedupeKey is dedupe_key, Normal is "normal"
  @auth(bearer: "notify-token")             # Authorization: Bearer <secret("notify-token")>
  contract Notify {
    @http("POST /v1/send", status: { 200: Accepted, 202: Pending, 400: Refused, 403: Refused })
    @key(required, header: "X-Actions-Idempotency-Key", field: "dedupe_key")
    @reconcile(send)
    command send(to: UserId, text: String, priority: Priority = Normal) -> Delivery
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
  - A `@fresh` command sends `basedOn` and a `@destructive` one `approval` as arguments, like the
    others (`based_on` with `names: snake`). A reply of 409 to a `@fresh` command is `Stale` and
    one of 410 is `Expired`, unless `status:` maps those codes; `status: { 412: Stale }` maps
    another. The service can't check an approval, only record it, so `@allow` is what limits who
    may send one.
  - `@auth(bearer: "secret")`, or `@auth(header: "X-Auth-Token", secret: "secret")`, sends a
    secret with every request.
  - A refused request (a 4xx that `status:` doesn't name) ends the run with a fault that shows the
    reply, `Unreachable` (section 15): it is a bug in the program or its configuration, not an
    outcome to handle.

  A program finds a contract's service at the URL in the environment variable named after its
  module (`NOTIFY_URL` for `notify`, `LAB_NOTIFY_URL` for `lab.notify`). A program that serves a
  contract itself calls its own implementation, unless that variable is set. The URL alone
  decides where a call goes, port included; `@port`, and `NOTIFY_PORT` below, decide only where a
  program that serves the contract listens. A server moved with `NOTIFY_PORT`, as in a test, is
  reached by callers whose `NOTIFY_URL` names that port.
- **Serving over HTTP.** `eye run` serves each contract the program serves on the contract's
  port, on every network interface, and runs until it is stopped. It names the port when it
  starts: `eye run: serving notify.Notify on :3001`. `NOTIFY_PORT`, named after the module as
  `NOTIFY_URL` is, serves it on another port instead, and `0` on any free one, so tests can run a
  server and its callers side by side. A request is read the way a caller sends it, so both sides
  follow the same attributes:
  - The operation's `@http` method and path pick it. An operation without `@http` is reached only
    from inside the program. A path nothing answers gets 404, and a known path with another method
    405.
  - Arguments come from the path, the query parameters or the JSON body, in the shape above. One
    left out takes its default, an `Option` left out is `None`, and anything else missing or of the
    wrong shape gets 400. A command with `@key` reads its key from the key's header, or from its
    body `field:`, and gets 400 without one. A refusal's body is `{ "error": "…" }`.
  - With `@auth`, a request without the secret, as a caller sends it, gets 401 and reaches no
    code. `eye run` reads the secret when it starts, and doesn't start without it.
  - The reply is the result in the shape above. A result whose variant `status:` names has that
    status, the lowest when several name it (Notify's `Refused` is 400); any other result has 200.
  - Each request runs its operation as one transaction, and the jobs it starts run after it
    commits, among the program's other runs. A fault answers 500 and changes nothing, so the
    caller's command is `Uncertain` and reconciles. The runtime keeps no replies: a repeated key
    reaches the `impl` again, which finds the work it already did through an entity the key
    names, as Notify's `Message(key)` does.
  - Requests are answered one at a time, between the program's steps and while its runs wait for
    a timer or another service.
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
- **A test sees what a fake received.** `fakes(…)` is a value: a test that wants to look names
  it, applies it with `with`, and asks it with `calls(op)`. That lists every call of the operation
  the fakes answered, oldest first, each a record of the call's arguments with their defaults
  filled in, and `key` last for a command with `@key`. A reconcile that asks a query, or sends a
  command again, adds a call too. So a test checks what a program sent without the program
  keeping a record of it:

  ```eye context=notify,briefing
  test "the morning's briefing goes at 07:40, for the Irish day" {
    let sent = fakes(notify, from: "recordings/")
    with sent
    with clock(2026-10-12 06:30 in "UTC")
    advance(10 min)
    let [briefing] = sent.calls(notify.send) else { unreachable("sent more or less than once") }
    expect(briefing.key == "ireland-2026-10-12" && briefing.priority == Normal)
  }
  ```

  `calls` takes an operation of a contract the fakes answer for; its result is a `List` of the
  record whose fields are the operation's parameters. `ai` and `http` have no operations, so
  their fakes show nothing this way. A call the fakes had no answer for stopped its run, so it
  isn't listed.
- **Services without a contract,** such as web pages, are read with the standard `http` module:
  `http.get!(url, headers: { "Accept": "text/html" }, via: proxy)` returns
  `Result(Response, HttpError)`, with the status, the headers and the body as text. Any answer is
  `Ok`, whatever its status; `Err` is `NoAnswer(reason)` or `TimedOut`. `via` is the URL of an HTTP
  proxy to go through. In tests, `with fakes(http, from: "recordings/")` answers `http.get` from
  `recordings/http.json`: `"contract": "http"`, and an exchange per URL, with `"op": "get"`, the
  URL as its `"key"` and `http.get`'s result (`{ "kind": "Ok", "value": { "status": 200, … } }`).
  An exchange without a key answers every other URL; a URL nothing answers fails the test.
- **Generated code for other languages.** `eye gen go DIR` and `eye gen ts DIR` write, for each
  module that declares contracts, its types, a client and a server stub: a Go package, or a
  TypeScript module. Like the network policy, the output is a build artifact, never committed.
  Rust and Python may follow. Both read the same rules as the interpreter, so a generated client,
  a generated stub and `eye run` send and read the same requests.

  ```sh
  eye gen ts --module notify testdata/lab > notify.ts
  eye gen go --out gen testdata/lab   # gen/notify/notify.go, gen/miniflux/miniflux.go, …
  ```

  - The types are the one JSON shape above, with their `##` comments. TypeScript's describe the
    JSON itself: a record is an interface with the names on the wire (`published_at`), a variant
    type is a union discriminated on `"kind"`, a type of fieldless variants a union of strings, a
    wrapper its value's type, and `Option(T)` is `T | null`. Go's marshal to it: a struct for each
    record and each variant, an interface for a variant type (`Delivery`, which `Accepted`,
    `Pending` and `Refused` implement), a string type with constants for fieldless variants, `*T`
    for an `Option`, `time.Time` for an `Instant`, and `Duration`, in milliseconds on the wire.
    Go names take Go's capitals: `MessageId` is `MessageID`.

    ```ts
    export type Delivery = Accepted | Pending | Refused;

    export interface Accepted {
      kind: "Accepted";
      id: MessageId;
      deliveries: number;
      deduped: boolean;
    }
    ```

  - Every type the contracts' operations and events use is written, from whichever module
    declares it. When two things would have one name, both take a longer one: a type puts its
    module's name in front, and a variant its type's, as NetBird's record `Pending` and Notify's
    variant `Pending` become `NetbirdPending` and `DeliveryPending`. A variant type with one
    variant of its own name is that variant alone.
  - A client calls each operation that has `@http` as the interpreter does: path, query or body,
    defaults filled in, a `None` left out, `@auth`, the key's header and body field, and
    `status:`. Its answers are the operation's, with what the contract adds: in TypeScript,
    `Uncertain` for a lost reply, and `Stale` and `Expired`, as values; in Go, as the errors
    `*Uncertain`, `ErrStale` and `ErrExpired`. A query of `Result(T, http.HttpError)` returns
    `T`, and an answer that isn't a success is an `HttpError` (Go's `*HTTPError`). A request
    that never reached the service gives the error that stopped it, never `Uncertain`, so it can
    be sent again.
  - A server stub is an interface to implement, `NotifyService`, and a handler that serves it
    as `eye run` serves the contract: Go's `NewNotifyHandler(service, secret)` is an
    `http.Handler`, and TypeScript's `notifyHandler(service, secret)` a function from a Fetch API
    `Request` to a `Response`. It checks `@auth` first, routes by `@http`, reads the arguments,
    gives those left out their defaults, reads the key, and answers with the status `status:`
    gives the result's variant. A method that fails answers 500, which the caller's command takes
    as `Uncertain`; returning `ErrStale` or `ErrExpired` in Go, or `Stale` or `Expired` in
    TypeScript, gives those outcomes. A Go client implements its service interface, so a server
    can pass calls on.
  - Operations without `@http` have no client method and no route, as `eye run` gives them none.
  - A type these languages' JSON can't say the same way, a tuple, a map whose keys aren't strings
    on the wire, an unnamed record or an `extern type`, is an error that names it.

## 18. Calling C and Go

```eye
@library("z")
extern fn crc32(crc: UInt64, data: Bytes, length: UInt32) -> UInt64

extern type GzFile                 # owned by foreign code: only extern functions make one
@library("z")
extern fn gzopen(path: String, mode: String) -> Option(GzFile)   # None for C's NULL

@host("lab.desk")                  # provided by the Go program that embeds Eyelang
extern fn showOnDesk(text: String) -> Result((), String)

extern type Db
@host("lab.sqlite")
extern fn open(path: String) -> Result(Db, String)
```

- An `extern fn` has a signature and no body. Calling one has the `ffi` effect, which public
  signatures state like any other, and in a job the call is a `!` step: `showOnDesk!(text)`
  (section 12).
- Every `extern fn` of a program says where its code is, with exactly one of `@library` and
  `@host`: `crc has no body, so it needs @library("…"), naming the C library it is in, or
  @host("…"), naming the function the Go program that embeds Eyelang provides`.
- `@library` links a C function by its symbol, the extern fn's name. The interpreter loads it
  from the library at run time, without cgo, so `eye` itself needs no C toolchain; a compiled
  program calls it directly, linked like any other symbol. `@library("z")` is found as the system
  names libraries: `libz.so`, else the newest `libz.so.N`, on Linux, `libz.dylib` on macOS, and
  `z.dll` on Windows; a file name or a path is used as given.
- `@host` names a function that the Go program embedding Eyelang registers through the embedding
  API, the Go package `eyelang/host`; the standard library's own primitives, such as `http.get`,
  are `@host` functions too, which the interpreter provides.

  ```go
  p, err := host.Load("desk")
  err = p.Register("lab.desk", func(text string) error { return desk.Show(text) })
  code := p.Run(ctx, host.RunOptions{Stdout: os.Stdout, Stderr: os.Stderr})
  ```

- Across the boundary go these types. The checker reports any other in a `@library` function's
  signature at its span; what a `@host` function takes depends on what provides it, so `Register`
  checks it (the interpreter's own take any value):

  | Eyelang | C (`@library`) | Go (`@host`) |
  |---|---|---|
  | `Bool` | `bool` | `bool` |
  | `Byte`, `Int32`, `Int64`, `UInt32`, `UInt64` | `uint8_t`, `int32_t`, `int64_t`, `uint32_t`, `uint64_t` | `uint8`, `int32`, `int64`, `uint32`, `uint64` |
  | `Int` | none: `C has no integer of Int's unbounded size` | `int`; a value that doesn't fit is a fault |
  | `Float` | `double` | `float64` |
  | `String` | `const char *`, copied and ended with NUL; a returned one is copied, not freed | `string` |
  | `Bytes` | `const uint8_t *` to a copy, as a parameter only: pass its length too | `[]byte`, copied |
  | an `extern type` | a pointer; NULL is a fault | any pointer, struct, map, channel, function or interface type |
  | `Option` of an `extern type` | a pointer that may be NULL, which is `None` | none |
  | `()` as the result | `void` | no result |
  | `Result(T, String)` as the result | none | `(T, error)`, or `error` for `Result((), String)` |

  A Go function may also take a `context.Context` first, which is done when the task that made
  the call is cancelled.
- What can be checked is checked before anything runs. `Register` compares the Go function's
  type with each declaration of its name and returns an error naming the first difference:
  `@host("lab.add") is add(a: Int32, b: Int32) -> Int32 at desk/main.eye:8:1: parameter a is
  Int32, which Go holds as int32, and the Go function's parameter 1 is int64`. Before `main`
  starts, running a program reports each `@host` function nothing registered, and each
  `@library` function whose library or symbol can't be loaded; a test fails at such a call. A
  host function that panics, a `Db` made by one host function passed to another that takes a
  different Go type, and NULL where no `Option` allows it are faults at the call. C's parameter
  types are recorded nowhere a loader can read, so a C function declared with the wrong ones gives
  wrong values or crashes.
- Loading C libraries needs the system's dynamic linker, so on Linux the C loader makes a binary
  dynamically linked. The released `eye` for Linux stays static, so it runs on a `scratch` image,
  and loads no C library; `go build -tags clib ./cmd/eye` builds one that does, and a Go program
  that embeds Eyelang imports `eyelang/host/clib` for it. On macOS and Windows `eye` always has
  it.
- The compiler trusts an extern's signature. It can't check that the foreign side is total or that
  it has no other effects, which is why `ffi` shows up in every signature that depends on one.
- **`extern type Db`** names a type whose values foreign code owns, and it is the one way to name
  one. Only extern functions make or take one, so Eyelang code can't build one or look inside it.
  It has no `Save`, `Show`, `Eq`, `Hash` or `Schema`, so it can't be saved, and so can't be live
  across a `!`, printed, compared or asked of a model: `d is a Db, which can't be saved, and it is
  still needed after a waiting call`. It is a type of its own, so a `Db` can't be passed where a
  `Socket` is expected. It may take type parameters, `extern type Cursor(row)`, and an
  `extern fn` may have a `where` clause, as a function does. The built-in types declared as
  `extern type` in `ir/builtin.eye` keep the traits they have.
- An `extern fn`'s parameters may be named `in` and `for`, the labels calls use for a zone and a
  duration: `daily(at: Time, in: Zone)`. A function with a body can't, since it couldn't read
  them: they are keywords.
- **eye's own built-ins are declared the same way**, in `ir/builtin.eye`: an `extern type` for each
  built-in type, such as `Int` and `List(a)`, and an `extern fn` for each built-in function, each
  with its `##` documentation. Programs never import it. The checker checks a call of a built-in
  against its declaration as against any function's, and its `uses` clause gives the call's
  effects: `now() -> Instant uses {clock}`, and `uses {wait}` for those that wait, such as `sleep`.
  Tools read it too, so hover, Go to Declaration and `eye doc` show the built-ins as they show any
  declaration. Built-ins one signature can't describe yet, such as the collection functions,
  which take any kind of collection, and `print`, which takes any number of values, keep a rule
  in the checker, and their declarations document them.

## 19. AI models and agents

Programs that call language models, and jobs that run coding agents, use the pieces the rest of the
language already has: types, effects, `!` steps and recordings. The standard library adds two
modules, `ai` and `agents`. Syntax that helps here and elsewhere is welcome: `##` doc comments
describe schemas and tools as well as documentation, and prompts are written as multi-line
strings (section 4).

**Typed model calls.** A call names the type it wants back. The compiler derives a JSON Schema from
that type, the provider is asked for structured output matching it, and the reply is checked and
decoded into an Eyelang value. Code never sees unchecked text.

```eye context=news
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
job summarise(items: List(Item)) -> Result(Digest, ai.AiError) {
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
  descriptions. Strict schemas, OpenAI's and Anthropic's alike, need `additionalProperties:
  false` on every object and have no fixed-length arrays, so a model is asked for a map as a
  list of `{"key": …, "value": …}` entries and for a tuple as an object with fields `value1`,
  `value2`, …; reading a reply takes either shape. A type that holds a `Secret`, a value of a
  program's `extern type` or a function has no `Schema`, so it can't be asked for.
- **The expected type picks the schema.** `ai.generate` returns `Result(a, AiError) where a:
  Schema`, so the type comes from where the result goes: here, the job's return type. When nothing
  says, write it: `let d: Digest = ai.generate!(…)?`.
- **Replies are checked.** If a reply doesn't match the schema, the call asks once more with the
  problems listed, and then returns `Err(Malformed(reply, problems))`.
- **Providers.** `ai.Provider` is `Anthropic` (the Messages API), `OpenAI` (chat completions) or
  `Gateway`, an OpenAI-compatible server such as the lab's, which serves every model. A call has
  the effect `net Ai.generate`, which `uses {net}` covers too (`Ai` is the `ai` module's contract
  for the providers), and each provider's address and key come from the environment and `secret`:
  `ANTHROPIC_URL` and `secret("anthropic-api-key")`, `OPENAI_URL` and `secret("openai-api-key")`,
  `AI_URL` and `secret("ai-key")`. `provider: Some(ai.Anthropic)` names one; without it, a gateway
  answers every call when `AI_URL` is set, since a deployment that has one routes through it, and
  otherwise the model's name decides: `claude-…` is Anthropic, `gpt-…` and `o1`, `o3`… OpenAI.
- **In a job, every model call is a `!` step.** Its reply is recorded, so a resumed run neither calls
  the model again nor pays twice, and it sees the same answer.
- **Tests replay recordings.** `with fakes(ai, from: "recordings/")` makes model-backed tests
  deterministic and free, and a recording that no longer matches the schema fails its test.
  `eye test --live ai` runs them against the real models, and `--record` writes their replies
  as the recordings (section 20).
- **Budgets.** `@budget(usd: 5, tokens: 2_000_000)` on a job or function works like `@memory`: a call
  that would go over it isn't sent, and the run ends as `Failed(OverBudget)`. A job's budget is for
  its run, a function's for each call of it, and a call counts against every budget it runs
  under. Each reply's tokens, and their price, are recorded with its step, so a resumed run counts
  them again and a restart doesn't reset the budget. A call is sent only if the budget leaves
  room for its prompt and at least 64 tokens of reply, and the reply is limited to what is left;
  a reply cut off there ends the run as `Failed(OverBudget)` too. Prices are known for Anthropic's
  models, and a deployment gives others in `AI_PRICES` (`"gpt-5.5=1.25/10"`: dollars per million
  tokens in and out); a provider that reports what a call cost is taken at its word. A budget in
  dollars for a model whose price isn't known isn't kept quietly by tokens instead: the call
  stops with a fault, `Unreachable`, that says to give the price. Recordings may say what a reply spent
  (`"usage": {"input": 900, "output": 100}`), and tests count it.

**Tools are functions.** Eyelang functions passed as `tools:` become the model's tools. A tool's
parameter types give its input schema and its `##` comment its description, and the runtime runs
the function when the model calls it and sends back what it returns.

```eye context=triage
import ai

## Find open issues that mention a phrase.
fn searchIssues(phrase: String, limit: Int = 10) -> List(Issue) uses {net GitHub.search} {
  github.search(phrase, limit)
}

## Add a label to an issue.
@destructive
fn label(issue: IssueId, name: String) uses {net GitHub.label} {
  github.label(issue, name, approval)
}

type Triage = { duplicateOf: Option(IssueId), labels: List(String) }

job triage(report: String) -> Result(Triage, ai.AiError) {
  ai.generate!(model: "claude-opus-5-5", prompt: report, tools: [searchIssues, label],
    approve: fn call: approval!(call, from: maintainer, within: 1 day))
}
```

- **A tool is a function declared with `fn`, named where it is passed.** `tools:` is a
  `List(ai.Tool)`, and a named function converts to an `ai.Tool` where one is expected, as a
  value converts to `dyn Trait` (section 11), so one list holds functions of different types.
  Its parameters and its result must have `Schema`. The parameters give the input schema, one
  property each, required unless it has a default, and the `##` comment the description. A
  lambda, a function passed in, a generic function, an `extern fn`, a contract's operation and a
  built-in don't convert: a tool needs a name, a description and one schema. A `Tool` has no
  `Save` and no `Schema`, so it can't be kept in an entity or across a `!`.
- **What a model may do shows up in the signature.** A model call's own effect is
  `net Ai.generate`, and naming a function as a tool counts its effects where it is named, as for
  any function named as a value (section 12). So `triage` uses `net Ai.generate`,
  `net GitHub.search` and `net GitHub.label`; a public function that passes `label` lists
  `net GitHub.label` in its `uses`, and adding a tool shows up in review. A tool runs inside the
  model call, so it can't wait.
- **The runtime holds the conversation.** It sends the tools with the call, in each provider's
  own format, and when a reply calls tools, it decodes each call's input into the parameters'
  types, runs the function and sends back its result as JSON (section 17), until the model
  answers without calling one. That answer is decoded into the expected type, as above. A call
  whose input doesn't decode, or that names no tool, goes back to the model as an error, and no
  function runs. After 20 replies that call tools, the call gives up with `ProviderError`.
- **In a job, each tool call is a step.** In `ai.generate!(…)` every reply of the model and every
  tool's result is recorded as a step of its own, so a run that restarts in the middle replays
  the conversation so far: it doesn't ask the model again, and no tool runs twice. `eye runs`
  shows them as `ai.generate: reply` and `tool: searchIssues`.
- **A `@destructive` tool needs a person for each call.** `@destructive` on a function adds a
  required `approval: Approval` parameter after those it declares, as on a command (section 17),
  so code calls it only after a person said yes, and its body can pass the approval on, as
  `label` does. The parameter isn't in the tool's schema. When the model calls the tool, the
  runtime calls `approve:` with the call written out, `label(issue: IssueId("8"), name: "dup")`,
  and runs the tool with the `Approval` it gets. `approve:` is usually
  `fn call: approval!(call, from: someone, within: …)`, so the request goes to the person it
  names, and only they can answer it (section 15); a test answers with `approve("label(")`. On
  `Declined` or `Expired` the tool doesn't run, and the model is told that the call was declined.
  Without `approve:` every such call is declined, and a call whose own `tools:` list names a
  `@destructive` tool must give `approve:`.
- **Recordings hold tool calls too.** In a recording, an exchange whose reply calls tools has
  `"calls": [{"tool": "searchIssues", "input": {"phrase": "crash"}}]` in place of `"result"`.

**Agents are sessions owned by jobs.** A coding agent such as Claude Code or OpenAI Codex works for
minutes or hours, asks for permission and can be interrupted. `agents` drives them through their
SDKs as sessions that a job owns. The job holds only the session's ID, so it waits without holding
memory and picks the session up again after a restart.

```eye context=review fails=6.8
import agents.{claude, codex}

type Review = { verdict: Verdict, findings: List(Finding) }
type Verdict = | Approve | RequestChanges(reason: String)
type Finding = { file: String, line: Int, message: String }

@on(event(github.pullRequestOpened))
@budget(usd: 5)
@deadline(2 h)
job review(pr: PullRequest) -> Result(Review, agents.AgentError) {
  let session = claude.start!(
    prompt: "Review {pr.url}. Report real bugs only.",
    workdir: checkout!(pr.head),
    tools: [readFiles, runTests],
  )
  let review: Review = session.result!()?
  if review.findings != [] {
    let fixed: String = codex.start!(prompt: "Fix these: {review.findings}",
      workdir: checkout!(pr.head), tools: [editFiles, runTests]).result!()?
    log.info("codex: {fixed}")
  }
  Ok(review)
}
```

- `start!` begins a session, `send!` adds a message, and `result!()` waits for the final answer,
  decoded into the expected type through the harness's structured output.
- `tools:` lists what the agent may use. Eyelang functions are served to the agent over MCP, and
  their effects become the job's, as with model calls. Permission requests from the agent arrive
  as events; code can answer them, or hand them to a person with `approval!`.
- Several agents run side by side as child runs, joined with `allDone!` or `firstDone!`.

## 20. Time, secrets, tests

**Time.** Schedules carry their zone: `daily(07:40, in: "Europe/Dublin")`, and `weekdays(…)` is the
same from Monday to Friday. `cron("*/20 7-9 * * mon-fri", in: dublin)` fires when the five fields
of a crontab line say, on the zone's clock. `every(15 min)` fires at each multiple of its interval
since midnight UTC: :00, :15, :30, :45.

`cron` takes the fields every cron takes, and nothing more:

- **Five fields:** minute (0–59), hour (0–23), day of month (1–31), month (1–12, or `jan` to
  `dec`) and day of week (0–7, where 0 and 7 are Sunday, or `sun` to `sat`). Names are in any case.
- **Each field** is `*`, a value, a range (`mon-fri`), a list (`1,15`) or a step over `*` or a
  range (`*/15`, `8-18/2`).
- **When both day fields are given, a day matches if either does,** as in every cron:
  `0 9 1 * mon` fires on the 1st and on every Monday. A day field that starts with `*`, such as
  `*/2`, narrows the other instead.
- **Not taken:** a seconds or year field, the macros (`@daily` is `daily(00:00, in: zone)`), and
  `L`, `W`, `#` and `?`. A line that is wrong, or that never fires (`0 0 30 2 *`), is an error at
  the string.

A daylight-saving change skips an hour or repeats one, and the schedules on a zone's clock
(`daily`, `weekdays` and `cron`) say what they do then with `onGap:` and `onOverlap:`. The
defaults mean there are never two UTC crons and an hour check:

| Policy | At a time the change skips or repeats, the schedule… |
|---|---|
| `onGap: Shift` (default) | fires as far after the change as it was meant to be: `01:30`, on the night the clock jumps from 01:00 to 02:00, fires at 02:30 |
| `onGap: Skip` | doesn't fire for that time that day |
| `onOverlap: First` (default) | fires the first time the clock shows it |
| `onOverlap: Second` | fires the second time |
| `onOverlap: Both` | fires both times, an hour apart: for a job that should run every hour, `cron("0 * * * *", in: dublin, onOverlap: Both)` |

A time shifted onto another firing of the same schedule fires once: with `Shift`, an hourly cron
fires every hour across the jump. The policies are the prelude's `GapPolicy` and `OverlapPolicy`.

A schedule counts from when the program first starts; each firing is saved with the run it
starts, so a restart neither repeats nor skips one; firings missed while the program was stopped
run once, for the latest, when it starts again. Dates in another zone are asked for by name, never
taken from the host's clock:

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
store, so every run is deterministic and fast. Each test has an empty directory of its own, where
`fs` and `process` work with relative paths and which is removed when it ends, and `random` draws
from the same seed in every test. The clock starts at `clock(…)`'s time (2026-01-01
00:00 UTC without one) and moves only when the test calls `advance(…)`; each run that wakes on the
way runs before `advance` goes on, and so does each schedule that fires, if its job is in the
test's own module (a test of the reminders doesn't start the NetBird sync). Jobs started by a call run as soon as the call returns, until
they finish or wait, and so do runs that an event a test publishes starts or wakes:
`notify.answered.publish(answer)` stands in for a person pressing a button, and
`approve("delete 3 backups")` or `decline(…)` answers, as the person asked, the `approval!` whose
text contains it, and fails the test when none or several do.
`expect(cond)` fails the test, and for a comparison shows the value it got:
`expected w.phase == Asleep, but w.phase is Running`. A run that stops on a bug (`unreachable`, a
fake with no recorded answer) fails the test too; one that ends as an outcome the program
declared, such as `Failed(DeadlinePassed)` from its `@deadline`, doesn't, and the test checks it
by joining the run.

**Live tests.** `eye test --live ai,notify` runs the tests against the real services and models of
the modules named, instead of their recordings. A `with fakes(…)` leaves those modules out, and
their calls go where `eye run` sends them: to the address and with the secret the environment
gives, such as `NOTIFY_URL` and `NOTIFY_TOKEN` for a contract, and `AI_URL`, `ANTHROPIC_API_KEY`
or `OPENAI_API_KEY` for a model (sections 17 and 19). `calls(op)` still lists the calls the live
service answered, and the fakes of the other modules keep replaying. `--live` takes `ai`, `http`
and modules that declare one contract, by path or by name.

A test that needs a live service it can't reach is skipped, not failed: when the `with` names a
contract module whose address or `@auth` secret isn't set, or `ai` while no provider is, and when
a model call's provider has no key, or a call's operation has no `@http`. `eye test` prints `skip`
and why, and a skipped test doesn't change the exit code, which is 1 only when a test failed:

```text
ok    a reminder is due once, inside its lead (2ms)
skip  the morning reads its sources, has the briefing written, and sends it
      --live ai: no model provider is set: set AI_URL, ANTHROPIC_API_KEY or OPENAI_API_KEY
11 tests passed, 1 skipped (40ms)
```

`--record`, with `--live`, writes what the live services answered as recordings, in the file the
`with fakes(…)` would have read, so the next plain `eye test` replays them. `eye test` names each
file it writes:

- **Only a test that passed writes.** A failed or skipped test writes nothing.
- **A contract's file keeps one exchange per operation and key,** the last answer the service gave
  (for a lost reply, the answer to the send that settled it, not `Uncertain`). `http.json` keeps
  one per URL. Exchanges already in the file for other operations, keys or URLs stay, so
  re-recording one test with `--run` doesn't lose what the others replay. A file is updated in
  place, so `--record` overwrites; review the change with the rest of the diff.
- **`ai.json` holds one test's replies, in order,** with what each spent and the tools it called,
  since replies are replayed in order. It replaces the file; when several tests record into one
  file, the first that passes writes it. A provider's failure or refusal isn't written.
- **Keys and times are written as they were.** A call's key is the one it was sent with, which
  comes from the test's clock and data, so the replay sends the same. A result keeps the times the
  service answered with, and nothing says when it was recorded, so recording an unchanged answer
  again leaves the file as it was.
- **A secret is never written.** A recording holds results, never the requests or headers sent,
  and the value of every secret the test read (an `@auth` secret, a `secret(…)`, a provider's key)
  is replaced by `<secret name>` wherever it appears in an answer, such as a service's error that
  repeats the token.

## 21. What happens to Eyelang 1

Eyelang 1, the tree-walking interpreter that served as a testing ground, lives only at the tag
`eyelang-1`. Its line editor, `repl/`, is the one piece carried over.

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
5. An untyped, resolved IR (`ir/`), and a reference interpreter on it. Every name points at its
   declaration or a slot of its frame; entities, jobs, views and contracts are declarations of
   the IR, which the interpreter runs directly. The interpreter resumes a job from its latest
   checkpoint and replays the steps since, each returning its recorded result, so nothing outside
   happens twice, and output from those steps isn't printed again (section 15; until task 7.3 it
   replayed every step from the start). Used by `eye run`, the REPL and
   tests, with entities and jobs on an in-memory store, structured concurrency on lightweight
   tasks, and `extern` functions from the Go host or from C libraries loaded at run time (without
   cgo).
6. The runtime pieces: the `Store` trait, the runner, the in-memory store written in Eyelang, and the
   Postgres store.
7. The typed IR: the IR with a resolved type on every expression and every call resolved to its
   target (ROADMAP task 7.1). The interpreter runs it too, and resumes jobs from checkpoints at
   the statements of their bodies (task 7.3); compiled jobs resume from saved frames, produced by
   a pass that splits each waiting function into states.
   Then, on the way to code generation, generic code is specialised and `entity`, `job`,
   `contract` and `view` are each lowered into core IR by their own pass, kept separate from the
   type checker. Nothing else in the compiler needs to know about them, and the passes are already
   shaped like macros if metaprogramming is added after v1.0.
8. Code generation to C with reference counting, as the compilation design describes:
   [docs/internals/compilation.md](docs/internals/compilation.md).

**Native code: C, built by the `zig cc` that `eye` ships.** Decided 2026-10-11. The compiler emits
C11, one file per module, through a low-level IR from which a second back end (QBE, for fast debug
builds) could be printed later. `eye` releases ship `zig` beside `eye`, and `eye build` compiles
the C it emits with `zig cc`, which cross-compiles to every release target from any host. `eye`
itself stays free of cgo: it parses by hand, and loads C libraries at run time without a C
toolchain; only `eye build` compiles C.

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
parses each one with both parsers and checks it. An example that uses names it doesn't declare is
checked as one more file of the program in `testdata/specctx/NAME`, which its fence names
(`context=NAME`); one that shows what a ROADMAP task will build says so (`fails=TASK`), and must
check once that task is done.

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
2. **Trait coherence.** **Decided** (see the decision log, row "Shared behaviour"): an impl lives
   in the module of the trait or of the type (section 11).
3. **Exposing entities and jobs to other services:** a generated contract per entity, or explicit
   contracts only?
4. **Job code upgrades.** **Decided** (see the decision log, row "Job code changes"): detect, and
   refuse (section 15). Still open: a `migrate` for a job's resume points, once a program needs
   one.
5. **Packages:** `eye.toml` and an `eye.lock` that carries hashes (so Nix needs no placeholder
   hash); registry or git only? `eye.toml` marks a program's root and may list `modules`, more
   directories to find imports in (section 13); what it holds beyond that is open.
6. **REPL behaviour for jobs.** **Decided** (section 13, `eye repl`): jobs run against the
   in-memory store on a clock that moves only with `:advance`, as a test's does.
7. **Metaprogramming after v1.0:** if real code keeps repeating the same patterns, add attribute
   macros (compile-time Eyelang functions that inspect a declaration's types and generate code).
   The built-in constructs could then be re-expressed with them, and grammar and editor support
   would stay unchanged because macros would only appear as attributes.
8. **Multi-line strings.** **Decided** (see the decision log, row "Strings"): `"""`, with the
   closing line's indentation as the margin (section 4).
9. **Durability without a database.** **Decided** (see the decision log, rows "Storage" and "Long
   runs"): the journal. Each commit (entity
   writes, runs, events, schedule firings, event keys seen, commands whose reply was lost) is one
   line of JSON appended to `.eye/journal` and synced before it counts. A commit that can't be
   written, on a full disk or otherwise, changes nothing: the journal is cut back, memory isn't
   touched, and the run stays at its last checkpoint and is tried again later. The journal is
   rewritten as one snapshot when it is opened and when it grows past four times its live data,
   and a 4 MiB reserve file beside it is given up when a write fails, so that compaction can still
   finish. Events no unfinished run can still see are dropped, an event's key is forgotten after
   30 days, and a run that finished more than 7 days ago keeps a summary (job, arguments, outcome,
   times, step count) instead of its steps; waiting runs and unsettled lost commands are never
   dropped. Runs save their variables, not their history (row "Long runs"), and `eye run` warns
   about a run that has taken 1,000 steps since its checkpoint. One process writes a journal at a
   time; several runners on one database is question 28. Still open: the retention times are
   fixed; should a program set them, and should `eye runs` show what was summarised?
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
15. **Typed choices.** **Decided** (see the decision log, row "Typed choices"): `T.variants()`
    and `T.parse(s)`, derived (section 11). Still open:
    should a choice's label be its `##` comment, or its `Show`, with an impl where the text differs
    from the variant's name? `##` comments are documentation, and using them as text for people
    mixes two jobs; an impl of `Show` is the existing way to choose how a value is written. Until
    this is decided, a label is the variant's `Show`.
16. **What a fake received.** **Decided** (see the decision log, row "What a fake received"):
    `sent.calls(notify.send)` (section 17). Still open: should a recording's unused exchanges
    fail a test, when one recording serves several tests?
17. **Where approvals go.** **Decided** (see the decision log, row "Answering approvals"), and
    built: through the program, which routes `approvals.requested` and answers with
    `approvals.answered`, naming the request's `id`, from the person in `from:` (section 15). Who
    may publish an answer at all is question 35.
18. **Contracts shared by programs.** The lab is one program so that its modules share the Notify
    contract; separate programs would each need a copy. This belongs with packages (question 5).
19. **Which actions are effects?** **Decided** (see the decision log, row "Recorded steps"):
    every effect in section 12's table is inferred, and `!` marks every recorded step in a job.
20. **Collection functions' types.** **Decided** for v0.1 (section 18): one function per
    operation, with a rule for each kind of collection it takes and a requirement where one is
    needed (`sort` needs `Ord`). A `Collection` trait would need traits over type constructors;
    reopen this if user-defined collections want the same pipelines. The built-ins also work as
    methods, `xs.first()`; decided on 2026-10-12 to extend that to every function whose first
    parameter takes the value (section 8, ROADMAP task G.15).
21. **`Handle(name)` and `extern type`.** **Decided** (see the decision log, row "Foreign
    values"): `extern type` is the one way to name what foreign code owns (section 18).
22. **Printing in jobs.** **Decided** (see the decision log, row "Recorded steps"): output is not
    a step, and logging is not an effect (section 12).
23. **Where `!` goes on a helper.** Writing `fn stamp() = now()` and calling `stamp!()` makes the
    whole helper one recorded step; a helper that itself has `!` steps is called with `!` too, but
    then its steps are recorded one by one. The same mark means "record this call" in one case and
    "this call contains steps" in the other. Is one mark for both right, since either way the
    caller sees a point where the run may stop and resume, or should the difference show?
24. **`dyn Store`, and traits that name their own type.** Choosing a store at run time is an
    obvious use of `dyn`, but the `Store` trait is `Store(s)`, whose `commit`
    returns `s`, the store's own type, so a `dyn Store` would need to say which store it is. A
    trait has no name for "the type that implements me" (`Self`), and threads it through a
    parameter instead, as `Add(rhs, out)` does. Should traits get `Self`, with traits whose
    methods return `Self` kept out of `dyn` (as Rust does), or should a store's `commit` change
    the store in place (`mut self`) so the trait needs no type of its own?
25. **Naming a record's type at its literal.** **Decided** (2026-10-12): no syntax at the
    literal; a lambda's record literal takes its type from where the result goes, as other
    literals do, so `users |> map(fn u: { name: u.name })` builds the record type the result is
    used as. Built by ROADMAP task G.14. Still open: where the context is a `dyn Trait`, nothing
    says which type a literal is, so each implementation needs a constructor (`Square.new(2)`)
    or a `let s: Square = { side: 2 }` first. Is that enough?
26. **What `@memory` measures.** A compiled job would count its process's memory. The reference
    interpreter runs one run at a time and counts what it keeps for a run: its saved state, with
    every step's result. So `@priority` acts only in that a run waiting for memory isn't
    overtaken by a lower one; nothing is paused mid-run. Is saved state the right
    measure until v0.4, and should `eye runs` show each run's against its `@memory`?
27. **One flag for the store.** `eye run` takes `--journal file`, `--store postgres://…` and
    `--memory` (keep nothing), and now `--memory-limit`, which reads like the third. Should the
    store be one flag, `--store file:… | postgres://… | memory`, and should `eye.toml` name a
    program's store, including the Postgres schema that today goes in the address as
    `?search_path=`?
28. **Several runners on one database.** The Postgres store allows one `eye run` per database
    and schema, held by an advisory lock. Running more needs runners to claim runs (leases that
    expire), and the in-memory copy each keeps would no longer be the whole truth. Is that v0.2's,
    or later? And when a commit whose reply was lost turns out to have arrived, `eye run` stops so
    a restart reads the database again; should it reload in place instead?
29. **The shape of task groups.** `taskGroup(fn group { … })` with `group.spawn(fn { … })` needs
    no syntax, but a `group { … }` block would read better. A group returns its first function's
    result; should it gather its tasks' results instead? `channel(capacity:)` returns a tuple to
    destructure; would one `Channel(a)` with `put`, `take` and `close` be simpler, at the cost of
    handing both ends to everyone? A group whose tasks all wait on channels nobody can reach fails
    (a forgotten `close()`); should a `Sender` close itself when the task that owns it ends?
30. **Model prices and routing.** Dollar budgets need prices: Anthropic's are built in, others
    come from `AI_PRICES`, and a dollar budget on a model with no known price stops the run. Should
    prices live in `eye.toml`? `AI_URL`, when set, sends every model through the gateway; is an
    explicit routing table clearer? A function's `@budget` is per call; should it pool across the
    run? And the prompt is estimated at about 3 bytes a token; should the runtime ask the
    provider's token counter instead, at the cost of a call?
31. **`Option` parameters with defaults.** `ai.generate(…, provider: Some(ai.Anthropic))` reads
    clumsily: the parameter is `Option(Provider) = None` so that leaving it out means "from the
    model's name". Should a call be able to pass a bare value to such a parameter, or should a
    default be able to say "chosen later" without `Option`?
32. **Time-travel debugging.** A job's run is deterministic between its `!` steps, which record
    everything that could differ (the clock, the network, randomness, entity reads, model calls),
    and tests run on a test clock against recorded fakes. Since checkpoints (section 15) save a
    run's variables at its statements, any moment of a run can be reached again: restore the
    nearest checkpoint before it and replay the recorded steps up to it, calling nothing outside.
    Stepping back is restoring an earlier checkpoint and replaying forward to the step before.
    That asks for: a run that keeps its history on request (`eye run --history`), since a
    checkpoint drops the steps before it and the journal compacts; `eye debug RUN` and
    `eye test --debug` that move a run or a failing test forward and back by statement and step,
    showing its typed variables (7.1) and the entities as they were; and a Debug Adapter Protocol
    server, whose stepBack and reverseContinue give editors the same. Which first, and how much
    history may a run keep? A failing test is the cheapest start, since tests are deterministic
    already.
33. **Effects of a function that is only stored.** A lambda or a named function counts its
    effects where it is written, even when it goes into data whose function type states them, so
    a function that only builds a record of functions needs their `uses` (`testdata/ir/effects`).
    Since a function kept in data is held to its type's `uses` (section 12), a value that only
    ever meets such limits could stop counting where it is written, and `eye fmt` would stop
    writing those effects into signatures. Is the clause worth keeping as a reader's hint, or
    should it go?
34. **An online playground.** A page where you write Eyelang and check, test and run it, with the
    editor support `eye lsp` gives (errors at their spans, completion, hover, formatting), and
    examples to start from. The checker, interpreter, language server and parser are Go without
    cgo and already build for WebAssembly (`GOOS=js GOARCH=wasm`), so the page could run them
    with no server, behind an editor such as CodeMirror. What it would take: the checker reads
    a program's files from `Options.Overlay` but still lists its directory on disk, so a
    program held only in memory must be accepted; the language server needs an entry point that
    takes text and a position instead of a stdio connection; `eye run`'s jobs need a clock that
    yields to the page; and model calls need recorded fakes or a key the visitor brings.
    Contracts are the case that shows the language best: two programs side by side, one that
    serves `impl Notify` on its `@port` and one that calls `notify.send!`, with `@allow` and
    port errors at compile time and the calls visible as they cross. In the page, the caller's
    `@http` calls could go to the other program's handler in memory; from the command line, two
    `eye run`s on real ports, which works since 2026-10-12, when `eye run` started serving a
    program's `impl Contract` on its port (section 17). Is the playground worth building before
    v1.0, and should it share its examples with the docs site and the tour?
35. **Who may publish an event.** `@allow` names the modules that may call a contract's
    commands and queries, but any module may publish any of its events. For
    `approvals.answered` that matters: the runtime checks that `by` is the request's `from`
    (section 15), but a module that writes `by: r.from` itself grants consent with no person
    involved. One way: `@allow(approving)` on an event, checked like a command's, so only the
    module that hears people may answer, and `eye approve` and tests stay outside it as the
    runtime's own. The standard `approvals` module can't name the program's modules, so the
    program would need a place to say it, such as `eye.toml` or an attribute on the routing job
    (`@answers(approvals)`). Is a guard on publishing worth a new rule, and where does it go?
36. **`@destructive` means two things.** On a command it adds an `Approval` parameter, which the
    caller gets from `approval!` (section 17); on a function given to a model as a tool, it
    pauses the job until someone approves that one call (section 19), though nothing says who is
    asked, as `approval!`'s `from:` does. Should the tool form take the same `Approval`, or have a
    name of its own and say whom it asks?
37. **One shape for "time ran out".** `poll!` gives `false`, `next!` gives `None`, and
    `approval!` gives `Expired` when their time runs out (section 15). Should every wait with a
    limit end the same way, such as a variant `TimedOut` or an `Option`?
38. **Slices and memory.** `xs[2..5]` and `bytes[i..]` share storage with what they come from
    (section 5), so a 10-byte slice keeps a 100 MB `Bytes` alive, a cost nothing shows. What is a
    slice's type, the type it was cut from or one of its own, and should keeping a small slice of
    a large value copy it?
39. **A module with two contracts.** A module that declares one contract offers its operations
    and events by name, `notify.send` (section 17). With two contracts, two operations or events
    could share a name. Should a module declare at most one contract, or should calls name the
    contract, `notify.Notify.send`?
40. **What `main` repeats.** `main` runs on every start of `eye run`, and the key rule stops it
    starting a job twice (section 13), but its entity calls and publishes happen again on every
    start: `pub fn main() { Ledger("x").credit(10) }` credits on each restart. Should `main` be
    held to keyed work only, or should a program's setup move to something that runs once?
