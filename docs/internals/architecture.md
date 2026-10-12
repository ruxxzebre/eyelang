---
title: How Eyelang works
---

# How Eyelang works

This page is a map of the repository for someone who uses Eyelang or wants to change it, and
hasn't read the code. It says what each part does, what it takes and gives, and which file to open
first. It describes `dev` as of 2026-10-13. For what is built and what isn't, see
[What runs today](/reference/status).

A few words first. An **entity** is a piece of shared state with an owner: its data changes only
through its own methods. A **job** is a function that can wait for days. Each **step** of a job is
a call marked `!`, whose result is recorded. A **run** is one execution of a job. A **contract**
describes another service's operations and types. A **view** is a named query over entities that
stays up to date. The [glossary](/reference/language/#glossary) has the rest.

[[toc]]

## 1. The pipeline on one screen

Every command starts the same way: read the source, check it, and turn it into the resolved IR.
From there, a program either runs in the interpreter or is compiled to C.

```text
  .eye files + eye.toml
          │
          ▼
  syntax/    lexer → lossless parser → syntax tree ──────────► formatter (eye fmt)
          │
          ▼
  check/     load modules, resolve names, infer types,
          │  check matches and effects
          │  (adds prelude.eye and runtime/store.eye, and std/ modules on import)
          ▼
  ir/        the resolved, typed program                       (eye check --ir)
          │
     ┌────┴───────────────────────┐
     ▼                            ▼
  interp/                      lir/    monomorphic functions in blocks  (eye check --lir)
  reference interpreter           │
  and runtime                     ▼
  (eye run, eye test,          cgen/   C11, plus the runtime written in C
   eye repl)                      │
                                  ▼
                               zig (internal/zig) ──► native binary (eye build)
```

| Stage | Takes | Gives | Open first |
|---|---|---|---|
| `syntax/` | a file's bytes | tokens (`syntax.Lex`), a lossless tree (`syntax.Parse`) that keeps every space and comment, syntax errors, formatted text (`syntax.Format`) | [`syntax/parser.go`](https://github.com/ruxxzebre/eyelang/blob/dev/syntax/parser.go), then `parse_decl.go`, `parse_expr.go`, `parse_stmt.go`, `parse_type.go` |
| `check/` | a program's directory | an `*ir.Program` and a list of diagnostics (`check.Program(dir)`) | [`check/check.go`](https://github.com/ruxxzebre/eyelang/blob/dev/check/check.go); types in `infer.go`, matches in `exhaust.go`, effects in `effects.go`, lowering in `lower.go` |
| `ir/` | (built by `check/`) | a tree where every name points at its declaration, every expression has its type, and pipes and string interpolation are plain calls | [`ir/ir.go`](https://github.com/ruxxzebre/eyelang/blob/dev/ir/ir.go); the built-in functions are declared in [`ir/builtin.eye`](https://github.com/ruxxzebre/eyelang/blob/dev/ir/builtin.eye) |
| `interp/` | an `*ir.Program` | the program running: `main`, then its jobs (`interp.Run`), or its tests (`interp.RunTests`) | [`interp/eval.go`](https://github.com/ruxxzebre/eyelang/blob/dev/interp/eval.go) for expressions, [`interp/runtime.go`](https://github.com/ruxxzebre/eyelang/blob/dev/interp/runtime.go) for entities and runs |
| `lir/` | an `*ir.Program` | the low IR: one function per generic function and set of types it is used at, each a list of blocks of named values (`lir.Lower`) | [`lir/lir.go`](https://github.com/ruxxzebre/eyelang/blob/dev/lir/lir.go) |
| `cgen/` | the low IR | C11 files with `#line` directives pointing at `.eye` lines, compiled by the pinned zig (`cgen.Generate`, `cgen.Build`) | [`cgen/cgen.go`](https://github.com/ruxxzebre/eyelang/blob/dev/cgen/cgen.go), and the C runtime in [`cgen/runtime/`](https://github.com/ruxxzebre/eyelang/tree/dev/cgen/runtime), starting at `eye.h` |

Three things to know about this picture:

- **The grammar is defined twice.** [`tree-sitter-eyelang/grammar.js`](https://github.com/ruxxzebre/eyelang/blob/dev/tree-sitter-eyelang/grammar.js)
  is the definition of the syntax. The parser in `syntax/` is written by hand, because it gives
  better error messages. Tree-sitter runs only in tests, as an oracle: the two parsers must build
  the same tree for every file in the conformance suite.
- **The standard library is Eyelang.** `check/prelude.eye` and the store in
  `check/runtime/store.eye` are checked with every program, and the modules in `check/std/`
  (`ai`, `approvals`, `fs`, `http`, `process`, `random`, `regex`) with each program that imports
  them.
- **Native code covers the core language only.** `eye build` compiles functions, records,
  variants, `match`, loops, closures, strings, collections, traits and tests. Entities, jobs,
  views and contracts run in the interpreter (`eye run`) until ROADMAP task 7.6, when the Go
  runner hosts compiled code. The design is in [Compiling Eyelang](/internals/compilation).

## 2. The runtime

The runtime is the part of `interp/` that keeps what a program holds beyond its values: entities,
runs, views, the clock and the fakes of other services. The language rules are in the
reference's [Entities](/reference/language/entities), [Jobs and waiting](/reference/language/jobs-and-waiting)
and [Views and triggers](/reference/language/views-and-triggers); the guide's
[Jobs and waiting](/guide/jobs-and-waiting) explains them with examples.

**Transactions.** An entity method runs as a transaction: one writer per entity at a time, all or
nothing. When a method calls other entities, the whole call tree commits or rolls back together.
A served operation is one transaction too. Jobs started and events published inside a transaction
happen only if it commits. See `txn`, `callEntity` and `commit` in `interp/runtime.go`.

**Runs and steps.** Starting a job makes a run. Each `!` call in the job is a step: it runs once,
and its result is saved in the run's record. An entity call from a job commits in the same store
transaction as the step that records it, so it happens exactly once. A job reads entities only
through steps (`w.read!()`, `view.read!()`), so a resumed run sees what it saw before.

**Checkpoints and replay.** At the start of each statement in a job's body, a run that has taken
steps since its last checkpoint saves a new one: its variables, the statement it is at, and where
each loop around it is. The steps before it are dropped. When a run wakes, it restores the
checkpoint and replays only the steps since, each returning its recorded result instead of calling
again. So a job that loops for weeks keeps a record the size of its variables. A checkpoint names
its statement by a hash of the typed IR, so a run whose code changed under it faults instead of
resuming in the wrong place. Open [`interp/checkpoint.go`](https://github.com/ruxxzebre/eyelang/blob/dev/interp/checkpoint.go)
and `interp/jobs.go`.

```text
  remind("ann") starts                                       run 1
    step 1  notify.send!(…)                 result recorded; entity calls commit with it
            checkpoint: variables and place saved, step 1 dropped
    step 2  next!(answered, within: 3 days)
            the run waits: record on disk, about 140 bytes in memory
    … three days pass, or advance(3 days) in a test …
            wake: record read back, checkpoint restored; step 2 gives None
    step 3  notify.send!(…)                 the reminder, recorded
    end     outcome Completed(…) saved
```

**Waiting.** `sleep!`, `next!` (an event), `approval!` (a person), `run.join!()` and the other
waiting calls park the run. A parked run is a handle in memory (its number, job, when it wakes and
what event wakes it, about 140 bytes) and a record on disk. See `interp/handles.go`, `sleep` in
`interp/jobs.go`, and `interp/waits.go` for joining runs and locks.

**Views and events.** When a transaction commits, the runtime updates the views over the entities
it changed (`updateViews` in `interp/runtime.go`). `@on(enters(view))` starts a run for each entity
that begins to match, `@on(leaves(view))` for each that stops. Events are published on commit, or
as a job's step; `next!` waits for one.

**Schedules.** `@on(daily(07:40, in: zone))`, `weekdays(…)`, `cron(…)` and `every(…)` start runs
on a time zone's clock, with rules for daylight-saving gaps and overlaps
([`interp/schedule.go`](https://github.com/ruxxzebre/eyelang/blob/dev/interp/schedule.go)). Each
firing is saved with the run it starts, so a restart neither repeats nor skips one.

**The clock and fakes.** Under `eye run` the runtime's clock follows real time. In a test it
starts at 2026-01-01 00:00 UTC, or where `with clock(…)` puts it, and moves only when the test
calls `advance(…)`; runs that wake on the way run in order before `advance` returns. The
playground uses a third kind, a virtual clock (`VirtualClock` in `interp/serve.go`) that skips
ahead instead of waiting. `with fakes(notify, from: "recordings/")` answers
a contract's calls from recorded exchanges in JSON ([`interp/fakes.go`](https://github.com/ruxxzebre/eyelang/blob/dev/interp/fakes.go)),
checked against the contract when they load. Tests are deterministic in the clock, the store and
random numbers; a test reaches the real network only if it calls a service without fakes. The
guide's [Testing](/guide/testing) page shows both.

## 3. The store and the journal

Everything a program keeps goes through one interface, `Store` in
[`interp/store.go`](https://github.com/ruxxzebre/eyelang/blob/dev/interp/store.go): load and
save entity rows, run records, events and meta keys, one `Commit` per transaction.

```text
  runtime (interp/runtime.go)
      │  Store interface (interp/store.go)
      ▼
  MemoryStore ─ written in Eyelang: check/runtime/store.eye
      │         an immutable value; each commit is a pure function to the next one
      │         used alone by tests, the REPL and eye run --memory
      │
      ├── wrapped by Journal (interp/journal.go)    ──► DIR/.eye/journal   (eye run's default)
      └── wrapped by Postgres (interp/postgres.go)  ──► --store postgres://…  or $EYE_STORE
```

- **The in-memory store is written in Eyelang.** `MemoryStore` in
  [`check/runtime/store.eye`](https://github.com/ruxxzebre/eyelang/blob/dev/check/runtime/store.eye)
  is part of every program and is interpreted like any other code. It doesn't survive a restart
  on its own.
- **The journal** is a file for one machine. It appends each commit, and syncs it to disk before
  anything outside the runner can see it: before a service call, output, an answered request or a
  wait. One sync covers every commit since the last. When it opens, and when the file grows past
  four times the live data, it is rewritten as a snapshot, so it grows with the state, not the
  history. A line cut short by a crash is dropped.
- **The Postgres store** keeps the same state in a database, so it survives the loss of the
  machine. Each commit is one SQL transaction. One runner writes a store at a time, held by an
  advisory lock; one database can hold several programs, one per schema.
- **Waiting runs leave memory.** Both durable stores keep the record of a run that waits or has
  ended on disk, and read it back when it wakes.
- **`eye runs`** reads the journal or the database without changing it, so it works while
  `eye run` runs. It lists each run's status, what it waits for, and its steps; `eye runs DIR 12 3`
  shows run 12's third step whole. `eye approve` and `eye decline` answer an `approval!` by
  writing a file to `DIR/.eye/inbox`, which the runner reads (`interp/inbox.go`).

The guide's [Running in production](/guide/running) page covers flags, restarts and stopping.

## 4. Boundaries

| Boundary | What it does | Where |
|---|---|---|
| Contracts over HTTP | A contract's values go as JSON, and its operations as HTTP requests, by one set of rules. The interpreter follows them when it calls a service and when `eye run` serves a contract. | [`internal/wire`](https://github.com/ruxxzebre/eyelang/blob/dev/internal/wire/wire.go); calls in `interp/services.go` (`callHTTP`), serving in `interp/listen.go` |
| Generated clients | `eye gen go` and `eye gen ts` write types, a client and a server stub for each contract, from the same rules, so the two sides can't drift. `eye gen policy` writes Kubernetes network policies. | [`internal/codegen`](https://github.com/ruxxzebre/eyelang/blob/dev/internal/codegen/codegen.go), golden files in `internal/codegen/testdata`; `internal/netpolicy` |
| Embedding in Go | A Go program loads an Eyelang program, registers the functions its `@host` declarations name, and runs it or its tests. | [`host/host.go`](https://github.com/ruxxzebre/eyelang/blob/dev/host/host.go) |
| Calling C | `@library` functions are loaded from shared libraries at run time, without cgo. On Linux this makes a binary dynamic, so the released `eye` has it only with `-tags clib`. | [`internal/ffi`](https://github.com/ruxxzebre/eyelang/blob/dev/internal/ffi/ffi.go), used through `host/clib` |

The guide's [Talking to other services](/guide/contracts) page shows contracts from the user's
side, and the reference's [Calling C and Go](/reference/language/calling-c-and-go) gives the rules.

## 5. Tools on the same front end

Every tool reads code through `syntax/` and `check/`, so they agree with the compiler by
construction.

| Tool | What it does | Where |
|---|---|---|
| `eye fmt` | Formats source. It has no options, and it refuses a file with syntax errors. | `syntax/format.go`; cases in `testdata/fmt` |
| `eye lsp` | The language server: completion, definitions, references, hover, semantic highlighting. `eye lsp --definition FILE:LINE:COL` answers one request from the shell. | [`internal/lsp`](https://github.com/ruxxzebre/eyelang/tree/dev/internal/lsp); the VS Code client in `editors/vscode` |
| `eye doc` | Writes the API reference as Markdown from `##` comments. | [`internal/apidoc`](https://github.com/ruxxzebre/eyelang/blob/dev/internal/apidoc/apidoc.go) |
| Playground | The checker, interpreter, formatter and language server built for WebAssembly, run in the browser on a virtual clock. | [`internal/playground`](https://github.com/ruxxzebre/eyelang/blob/dev/internal/playground/playground.go), `cmd/eye-wasm`; `task playground` builds it into `docs/public/wasm` |
| `eye repl` | An interactive session for expressions, declarations and jobs. | `cmd/eye/repl.go`, line editor in `repl/` |

All commands live in [`cmd/eye`](https://github.com/ruxxzebre/eyelang/tree/dev/cmd/eye), one file
each; [The eye command](/reference/cli) lists them.

## 6. How a language change flows

A change to the language touches every stage, in this order. CLAUDE.md is the source for these
rules.

| # | Step | Files | Command |
|---|---|---|---|
| 1 | Write the rule and an example in the spec. A decision gets a row in the decision log (§2); an unsettled idea goes to the open questions (§24). | `SPECS.md` | |
| 2 | Change the grammar. Newlines, braces and strings also need the scanner. | `tree-sitter-eyelang/grammar.js`, `src/scanner.c` | `cd tree-sitter-eyelang && tree-sitter test -i 'Name'` |
| 3 | Change the hand-written parser, and the formatter if layout changes. | `syntax/` | `go test ./syntax -run TestFormat` |
| 4 | Add tests: a valid file per construct, an invalid file per error, and corpus cases. | `testdata/syntax/valid`, `testdata/syntax/invalid`, `tree-sitter-eyelang/test/corpus` | `go test ./syntax -run TestSyntaxErrors -update`, then read the diff of `diagnostics.txt` |
| 5 | Regenerate, then check both parsers agree. | | `task generate`, `task conformance`, `task fuzz` |
| 6 | Teach the checker; test its messages. | `check/`, tests in `check/*_test.go` | `go test ./check` |
| 7 | Teach the interpreter; add a program whose tests use the feature. | `interp/`, `testdata/programs/<area>` | `go run ./cmd/eye test testdata/programs/core`, `go test ./interp` |
| 8 | Lower it for native code, or make `eye build` refuse it with a clear error. | `lir/`, `cgen/`, `cgen/testdata`, `testdata/ir` | `go test ./cgen`, `go test ./cmd/eye -run TestCheckIR -update` |
| 9 | Editor support. | `internal/lsp`, `editors/vscode/syntaxes` | `go test ./internal/lsp`, `task vscode` |
| 10 | Docs and bookkeeping: the guide, the status page, ROADMAP.md's "What's next". | `docs/guide`, `docs/reference/status.md`, `ROADMAP.md` | `task quick` while working, `task check` before pushing |

`task quick` builds `eye`, tests the checker, interpreter and parser, then checks and tests the
lab's programs in `testdata/lab`, in seconds. `task check` runs everything CI runs.

When the language changes under existing code, the fix is a rewrite in `eye fix`, not a
compatibility rule.

## 7. What is generated from what

Never edit a generated file. Change its source and run the command.

| Generated | From | Command |
|---|---|---|
| `tree-sitter-eyelang/src/` (except `scanner.c`) | `grammar.js` | `task generate` |
| `syntax/kind_gen.go`, `syntax/view_gen.go` | the grammar's `node-types.json`, by `syntax/gen` | `task generate` |
| `testdata/syntax/valid/spec-*.eye`, `testdata/programs/tour/tour.eye` | the examples in `SPECS.md`, by `tools/specexamples` | `task generate` |
| `docs/reference/language/`, `docs/design/`, `docs/.vitepress/spec-sidebar.json` | `SPECS.md`, by `tools/specpages` | `task generate` |
| `docs/reference/api/` | `##` comments in the standard library, by `eye doc` | `task generate` |
| `docs/reference/cli.md` | `eye help`, by `tools/clidoc` | `task generate` |
| `docs/internals/benchmarks.md`, `testdata/bench/results.json` | the programs in `testdata/bench`, by `tools/bench` | `go run ./tools/bench --write` |
| `docs/public/wasm/` (not in git) | `cmd/eye-wasm` | `task playground` |
| `testdata/syntax/invalid/diagnostics.txt` | the parser's messages | `go test ./syntax -run TestSyntaxErrors -update` |
| `testdata/ir/*/ir.golden`, `lir.golden` | `eye check --ir` and `--lir` | `go test ./cmd/eye -run TestCheckIR -update` |

`task generated` fails when a generated file in git doesn't match its source; `task check` runs it.

## 8. How work is coordinated

Several people and coding agents work on the repository at once.

- **`task next`** says what to do next, from what is true now: the last release and what landed
  since, ROADMAP.md's "What's next", the open issues, the gaps in
  [What runs today](/reference/status), and what each worktree holds that `dev` doesn't. Start
  every session with it.
- **`task claim -- WHO PATH…`** records which paths a session is changing, in the repository's git
  config, which every worktree shares. `task next` lists the claims. Leave others' claims alone or
  ask their owner, and give yours back with `task claim -- --release WHO` once the work is merged.
- **Worktrees.** Each session works in a git worktree of its own, unless it is the one merging
  into `dev`. `git worktree list` shows them.
- **Branches.** Development and releases happen on `dev`; `main` stays at Eyelang 1 for now.
  Commits say `Closes #N` for the ROADMAP task they finish, and the `Close issues` workflow closes
  the issue when the commit reaches `dev`.
- **CI** ([`.github/workflows/ci.yml`](https://github.com/ruxxzebre/eyelang/blob/dev/.github/workflows/ci.yml))
  runs `task check` on every push and pull request to `dev` and `main`, with a Postgres server
  and Docker for the tests that need them. A local `task check` skips those tests, so watch the
  run after pushing.
