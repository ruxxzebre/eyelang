---
title: The future of Eyelang
---

# The future of Eyelang

This page covers three things: what Eyelang already is, what you can build with it today, and
everything planned next, in order. It describes the code on `dev` on 2026-10-13. When a task
lands, this page changes in the same commit. Feature by feature, [What runs today](/reference/status)
is the finer record. Task by task,
[ROADMAP.md](https://github.com/ruxxzebre/eyelang/blob/dev/ROADMAP.md) and the
[GitHub issues](https://github.com/ruxxzebre/eyelang/issues) are.

## The idea

**Eyelang lets you write a program that runs for days (waits for people, keeps state, calls
services that fail) as plainly as a script, and the compiler catches the mistakes such programs
usually ship with.** Five promises say what you get:

1. **Waiting is one line.** `sleep!(3 days)`, `next!(answered, within: 6 h)` and
   `@on(daily(07:40, in: "Europe/Dublin"))` replace the queue, the cron table and the
   `next_run_at` column.
2. **Replays can't go wrong.** Every durable step in a job is marked `!`. A resumed run gets each
   step's recorded result instead of calling again, and code that would see a different value
   on replay doesn't compile.
3. **State has one owner.** An entity's data changes only through its own methods. Each call
   tree is one transaction, and you write no locks.
4. **Outside calls are typed and settle once.** A contract types every call to another service.
   A keyed command never does its work twice, and a lost reply is `Uncertain` until
   `.reconcile!()` settles it.
5. **Days of behaviour test in milliseconds.** `advance(3 days)` moves a fake clock, and fakes
   replay recorded replies.

[Goals](/design/goals) holds the promises in full and the principles every change is weighed
against. Everything below is a means to them.

## Where it stands

| Version | What it brings | State |
|---|---|---|
| **v0.1.0** | Write, check, format, run and test programs with entities, jobs and views, on an in-memory store with a controllable clock | Released 2026-10-10 |
| **v0.2.0** | Durable runs on a journal or Postgres, typed model calls, `dyn Trait`, task groups and channels, `@memory`, `@priority` and `@budget` enforced, approvals that reach people | Released 2026-10-11 |
| **v0.3** | Contracts with other services, network policy and Go/TypeScript code generated from them, model tools, coding agents | 11 of 13 tasks done. Left: 6.6 and 6.8, its exit tests |
| **v0.4** | Native code through C with reference counting; jobs as state machines; the runner hosting compiled code | 9 of 14 tasks done; 7.9's work is merged and closes with 7.6. Its exit test, the digest running natively (7.6), is the next piece of work |
| **v1.0** | Syntax freeze, migration tooling, `eye why`, hot code replacement, documentation | Planned |

Nearly every feature [What runs today](/reference/status) lists is done; that page names each
one still partial or not built. The biggest gaps are coding agents (6.8) and native compilation of entities, jobs
and contracts (7.6).

## What is implemented

### The language

- **The core.** Values and types:
  - `Int` with arbitrary precision, divisors checked as `NonZero`;
  - fixed-width integers, floats, strings with interpolation, chars, `Bytes`;
  - durations, sizes, dates, times of day, instants and zones checked when compiled.

  Also records, variants and enums (`T.variants()`, `T.parse(s)`), lists, sets and maps, tuples,
  pipes, lambdas, and pattern matching with exhaustiveness checks and `let … else`. Errors are
  `Result` only, with `?`.
- **Types and traits.** Inference inside bodies, against signatures as written. Generics with
  `where` bounds, traits with default methods and coherent impls, and operators as traits. Derived
  `Eq`, `Ord`, `Show`, `Hash`, `Save` and `Schema`. `dyn Trait` as a type of its own, and any
  function callable as a method.
- **Mutation without aliasing.** `var`, `mut` parameters marked at the call, element assignment
  (`xs[i] = v`), and no variable passed as `mut` twice in one call. Values, not shared objects.
- **Effects.** Inferred, and stated with `uses` on public functions. `!` marks every recorded
  step in a job. A secret, a handle or a function can't be held across a `!` unless it can be
  saved. A function kept in data is held to its type's effects.
- **Structured concurrency.** `parallelMap`, `both`, `race`, `timeout`, task groups with `spawn`,
  and bounded channels. On a test's clock they take turns by wake time, so tests give the same
  result every run.

### Durable programs

- **Entities.** Records with an identity, changed only through their own methods. Each call tree
  is one transaction. Entities are versioned with `@version(N)`, with `migrate from N` chained when
  a row is loaded.
- **Jobs.** Functions that run in the background and wait. A run waits with `sleep!`,
  `sleepUntil!`, `next!`, `poll!` and `approval!`, starts once per key, and is scheduled with
  `daily`, `weekdays` and `every` in a zone. Runs are limited with `@concurrency`, `@limit`,
  `@deadline`, `@memory`, `@priority` and `@budget`.
- **Resuming.** A job resumes from checkpoints at its statements and replays only the steps since.
  So a run that never ends, such as a watcher, stops growing. A waiting run costs a 48-byte handle
  in memory; 20,000 waiting runs take about 7 MB above an idle runner.
- **Views, events and schedules.** Views over entities, with `enters` and `leaves` triggers;
  `publish` and `@on(event(…))`. Another program's events arrive as a `POST /events/<topic>` to
  the program's port, under the contract's `@auth`.
- **Stores.** The in-memory store is written in Eyelang itself. The journal (`eye run`, the
  default) and Postgres (`--store postgres://…`) survive restarts of the runner and of the
  database. `eye runs` lists every run, what it waits for and its steps. `eye runs --retry ID`
  starts a failed run again, replaying its steps.
- **Approvals.** `approval!(…, from: person, within: 2 days)` reaches people through
  `approvals.requested`, and is answered by id, only by the person asked: `eye approve`,
  `eye decline`, or the program's own buttons.

### Other services and models

- **Contracts.** Typed operations over HTTP, with `@http`, `@auth`, `@port` and `@allow` enforced.
  Commands carry keys (`@key(required)`), and a lost reply is `Uncertain` until `.reconcile!()`
  settles it. `@fresh` and `@destructive` are checked. `eye run` serves a contract over HTTP on its
  `@port`.
- **Generated boundaries.** `eye gen go` and `eye gen ts` write types, clients and server stubs.
  `eye gen policy` writes Kubernetes NetworkPolicy from what each module may call.
- **Model calls.** `ai.generate!` asks a model for a value of an Eyelang type, by its schema, from
  Anthropic or any OpenAI-compatible gateway, within `@budget`. Functions passed as `tools:` are
  run when the model calls them, each a recorded step; a `@destructive` tool waits for approval.
- **Foreign code.** `extern` functions from Go hosts (the `host` package embeds Eyelang in a Go
  program) and from C libraries (`@library`, loaded at run time). `extern type` names a value
  foreign code owns.

### Testing

- `eye test` gives each test a fresh store and a clock that moves only with `advance(…)`.
- `fakes(module, from: "recordings/")` replays recorded replies, checked against the contract,
  and `sent.calls(op)` shows what a fake received.
- `eye test --live` runs against the real services and models. `--record` writes what they
  answered for the next plain run.
- Tests don't reach the network: a call without fakes, outside `--live`, fails the test and names
  what to add.
- `with memory(…)` sets the memory a test's runs are admitted against.

### Native compilation (v0.4, in progress)

`eye build` compiles a program to one native binary through C11, built by a pinned `zig cc` that
`eye` fetches and checks itself. It cross-compiles to every release target from any host.

- **Done.** The typed IR (7.1) and a monomorphic low IR (7.2). The C back end and core runtime
  (7.4). Reference counting in the low IR, with drops after last use and updates in place when a
  value is unique (7.5). Values laid out by type: enums as a byte, records up to 64 bytes as C
  structs, `Option` in a niche, lists holding their elements inline (most of 7.13). Cheap fixes
  (7.14). Every compiled program's tests are checked line by line against the interpreter at both
  `-O0` and `-O2` (7.8).
- **The core gaps (7.9)** are closed: time methods, `random`, `fs.lines`, `toJson`, saved values
  encoded and decoded byte for byte as the interpreter does them, and `http.get` and `regex` as
  host calls the Go runner answers.
- **Speed.** Against the same programs in Go, native code takes **0.85× Go's time and 0.36× its
  memory** on the geometric mean of nine benchmarks. It beats Go on trees, records, strings, logs
  and Eyelang's own lexer ([Benchmarks against Go](/internals/benchmarks)).
- **Hosting (7.6), the runner's side.** `eye build --runner` links a program's C into the Go
  runner through cgo, using a pinned Go toolchain. The result is one static binary on Linux,
  cross-built for macOS. Through `interp.Native` and `cgen/hosted/eye_host.h`, the runner runs a
  compiled program's `main`, tests, entity methods and jobs on the journal and on Postgres. It
  answers the program's callbacks, records its steps and resumes it after a restart.

### Tools

- `eye fmt` and `eye fix`, which rewrites code the language changed under.
- `eye check`, with an error class for every error the spec promises, each at its exact place.
- `eye doc`, which writes the API reference from `##` comments.
- `eye repl`, whose jobs run on a clock that `:advance` moves.
- `eye lsp`: errors at their spans, hover, completion, go to definition, references, formatting.
  The VS Code extension is its client.
- The tree-sitter grammar, for Neovim, Helix and Zed, kept in step with the compiler's parser by a
  conformance suite and a fuzzer.
- [The playground](/playground): the checker, interpreter, formatter and language server built for
  WebAssembly, in the browser. Two programs can call each other through a contract in the page.
- [The command reference](/reference/cli) has every command and flag.

### Programs written in it

- **The homelab's workloads** (`testdata/lab`): the morning digest, Notify, train reminders and
  NetBird access approvals. These are the programs that decide what the language needs.
- **The morning digest** runs in the homelab on Postgres, in shadow mode beside the TypeScript
  digest it is to replace.
- **Eyelang's own lexer, in Eyelang** (S.1). It gives the same 24,959 tokens as the Go lexer on
  the conformance suite, and compiles natively.

## What you can build with it today

Any program that waits, keeps state and calls services that fail, where you would otherwise wire
together a web framework, a database table, a queue, a cron, a retry library and hand-made
idempotency keys:

- reminders and follow-ups ("three days after the invoice, unless it's paid");
- approval flows that wait days for a person, with buttons;
- scheduled digests and reports that call models for typed summaries;
- monitors and watchers that run forever without growing;
- webhook delivery and other commands that must settle exactly once;
- AI agents whose model calls and tool calls are recorded steps, resumed after a crash
  without calling a tool twice.

[Examples](/examples/) has runnable programs of each kind, and [the tutorial](/tutorial/) builds
one from the start. Today such programs run on the interpreter under `eye run`, on the journal or
on Postgres. Programs of the core language alone (no entities or jobs) also compile to fast native
binaries with `eye build`.

## What is planned

Every open item, issue by issue, with the undecided open questions and the decisions waiting on
the owner, is listed in [What's left](/whats-left).

### v0.4: native compilation (now)

The work in order. The owner's native-focus checkpoint is close to reached: `NaN` equal to
itself, `with memory(…)` in tests, events between programs, `eye runs --retry`, and the core
gaps in C (7.9) are done.

1. **7.6, the runner hosts compiled programs (#79), v0.4's exit test.** The runner's side is
   done. Two things remain:
   - the C back end writing the hosting entry points: the program's tables, entity methods, jobs
     as state machines over saved frames, and encoders at the boundary;
   - `eye build --runner` building real programs with the runner's module inside.

   **Done when** the morning digest runs as a native job on the journal and on Postgres, with its
   idle memory and resume time recorded beside the interpreter's. Then v0.4 is released.
2. **Before the checkpoint closes:**
   - the rest of 7.13 (#117): maps specialised per key type, and strings and `Bytes` as views,
     so slicing them stops copying.
3. **7.11, faster than Go (#114).** Native code beats Go on time on every benchmark, and on
   memory for every program without entities or jobs. Tracked by `go run ./tools/bench --check`.
4. **7.12, reuse and fault regions (#115).** A `match` arm reuses the cell it matched, and a
   failing test frees what it allocated.

### v0.3: what is left

- **6.6, Notify's boundary from its contract (#58).** The homelab's Notify builds against
  generated TypeScript types, so a breaking change to the contract fails its build. This needs
  the homelab repository.
- **6.8, the `agents` module (#60).** `claude.start!` and `codex.start!` begin a coding-agent
  session that a job owns. The job waits without memory and picks the session up after a
  restart. Tools are served over MCP, and permission requests are answered through `approval!`.

### Open issues outside the phases

- **Library gaps:**
  - a secure random token, and JSON decoded into typed values (#153);
  - HTTP beyond `get`, and contracts with more than one base URL, for webhooks (#144).
- **Diagnostics and CLI polish:**
  - a `Secret` held across `!` should suggest a fix that compiles (#151);
  - `eye fix`'s help should name every call it rewrites (#143);
  - small wording fixes in `eye test`, `eye help build` and `eye run` (#150).
- **Docs and the playground:**
  - the whole reference as one `llms-full.txt` for agents (#152);
  - homelab-specific teaching examples replaced (#148);
  - the playground's 24-hour run limit and its files (#147).
- **The repository:** `main` still holds Eyelang 1; `dev` should become the default (#149).

### After v0.4

- **S.2, the runner in Eyelang (#116).** The durable runner moves from Go into Eyelang part by
  part, so a native job program has no Go in it and can beat Go on memory too. **Done when** the
  digest runs natively with no Go in its binary, using less memory than under the Go runner.
- **Self-hosting (#82).** After the lexer (S.1) come the parser, formatter, `eye doc` and the
  checker in Eyelang. Then the compiler builds itself, and CI checks that the second build matches
  the first. Releases will publish the compiler's generated C as a bootstrap seed, so building
  Eyelang never needs an existing Eyelang binary.
- **Compile speed (open question 47).** The owner's targets: a program of a million lines built
  in a minute, then in 10 seconds, then in 1 second. Today a million lines check in 3.8 s, but an
  optimised build of 100,000 lines takes 4 minutes. The first step is C in one unit per module,
  compiled in parallel and cached by hash.
- **Packages (P0, waiting on the owner).** `[dependencies]` in `eye.toml` from paths and git, an
  `eye.lock` with content hashes, a cache and `eye vendor`. Contracts shared between programs come
  with it (open question 18).
- **Several runners on one database** (open question 28). With it, pipelines over entity sets
  compiled to SQL, which load every entity into memory today.
- **Runs pinned to the code they started on** (open question 41). The runner would keep each
  version of a program while runs paused in it remain, and start new runs on the newest.
- **Time-travel debugging** (open question 32). Checkpoints and recorded steps already make every
  moment of a run reachable again. `eye test --debug` on a failing test comes first, then
  `eye debug RUN` and a Debug Adapter Protocol server with step back.

### v1.0

- A syntax freeze, after which old code keeps working.
- Migration tooling: `eye fix` rewrites for every change before the freeze.
- `eye why`, to explain the compiler's decisions.
- Hot code replacement for running programs.
- Complete documentation.

Before v1.0 the language still changes whenever real programs show a better way. Each change
comes with a rewrite for existing code, not a compatibility rule. Attribute macros are considered
only after v1.0, if real code keeps repeating the same patterns (open question 7).

### Questions still open

[Open questions](/design/open-questions) has each in full. The ones not yet decided:

- **Views and data:** time conditions in views such as "idle for 20 minutes" (1); exposing
  entities and jobs to other services (3).
- **Jobs and runs:** a `migrate` for a job's resume points (4); several runners on one database
  (28); runs pinned to their code (41); what `main` repeats on each start (40); one shape for
  "time ran out" across `poll!`, `next!` and `approval!` (37).
- **Packages and contracts:** packages (5); contracts shared by programs (18); a module with two
  contracts (39); who may publish an event (35); the two meanings of `@destructive` (36); a
  permission model for effects at run time (46).
- **Models:** streaming model output (10); prompts in template files (11); prices and routing
  (30).
- **Syntax and types:**
  - request-scoped values (12);
  - one canonical `if` (13);
  - `if let` (14);
  - a choice's label (15);
  - where `!` goes on a helper (23);
  - `Self` in traits (24);
  - literals in a `dyn` context (25);
  - `Option` parameters with defaults (31);
  - the effects of a stored function (33);
  - slices that keep large values alive (38);
  - regex patterns checked when compiled (44).
- **Runtime and tools:** what `@memory` measures natively (26); one flag for the store (27); task
  groups' shape (29); time-travel debugging (32); unused recordings in fakes (16); compile speed
  (47); attribute macros after v1.0 (7).

## How this page stays true

The plan is decided by real programs: the lab's workloads are the test of every new rule.
[ROADMAP.md](https://github.com/ruxxzebre/eyelang/blob/dev/ROADMAP.md) holds the order, and
`task next` prints what is next from the repository's current state. When a version is released
or a task here lands, this page is updated with it.
