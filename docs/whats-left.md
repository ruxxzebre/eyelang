---
title: What's left
---

# What's left

Every open item, as of 2026-10-13, in one list. [The future of Eyelang](/future-of-eyelang) tells
the story: what Eyelang is, what it can do today and where it is going. This page is the
checklist behind it. Task by task, [ROADMAP.md](https://github.com/ruxxzebre/eyelang/blob/dev/ROADMAP.md)
and the [GitHub issues](https://github.com/ruxxzebre/eyelang/issues) are the record; feature by
feature, [What runs today](/reference/status) is. When an item here is done, remove it in the
same commit.

## A. Before the native-focus checkpoint

ROADMAP's "What's next" opens with the checkpoint: when it is reached, every session works on
compiling and the native port, and new features wait for v0.4.

- **#143** `eye fix`'s help text names every call it rewrites.
- **#117** The rest of 7.13: maps specialised per key type (1.62× Go's time today), and `String`
  and `Bytes` as views whose slices share their buffer.

## B. v0.4: native compilation

- **#79** 7.6, the runner hosts compiled programs. The Go runner already runs compiled entity
  methods and jobs, answers their callbacks and host steps, and carries the program in the binary
  (shown with a hand-written C program). Left:
  - the C back end writes the program's tables and entry points: entity methods, jobs as state
    machines, encoding and decoding saved values, `main` and tests;
  - `eye build --runner` builds real programs, with the runner's Go source embedded;
  - `testdata/programs/entities` runs natively under the runner;
  - **the morning digest runs as a native job** on the journal and on Postgres, killed and
    resumed, with its idle memory and resume time recorded beside the interpreter's.
- **The v0.4 release:** a tag, binaries for the five targets, release notes.

## C. Native code after v0.4 (v0.4.x)

- **#114** 7.11, faster than Go on every benchmark, not only on the geometric mean (0.85× Go's
  time and 0.36× its memory today). `fib` is 1.69×, mostly `Int` overflow checks.
- **#115** 7.12, a `match` arm that builds a value of the same size reuses the matched cell, and a
  failing test frees what it allocated.
- **Compile speed** (open question 47): the owner's targets for building a million lines.
- Smaller native items:
  - `x / -7` isn't turned into a multiplication;
  - a small program starts at 0.28 MB, up from 0.16 MB;
  - a cold `-O2` build is a few seconds slower since the runtime compiles with the program;
  - the Windows paths (`process.run`, the stack thread) are cross-compiled but never run;
  - tasks and channels run in the Go runner, not in compiled C;
  - a `wasm32-wasi` target for `eye build`.

## D. Finishing v0.3: contracts and agents

- **#58** 6.6, Notify's boundary generated from its contract, in the homelab repository.
- **#60** 6.8, the `agents` module: coding-agent sessions owned by jobs.
- **The v0.3 release:** the owner decides whether to release before 6.6 and 6.8.

## E. Owed by releases already made

- Phase 5's exit criterion: one good morning run of the digest in the homelab, then the owner's
  decision on replacing the TypeScript digest.
- Move the homelab digest's pin to the latest `dev` (retries, clear errors, effort, checkpoints).
- The decision log should say that pipelines over entity sets run in memory until several
  runners share a database.

## F. Other open issues

- **#144** Webhooks to many URLs: `http` has only `get`, and a contract has one base URL.
- **#148** Replace the remaining homelab-specific teaching examples in the docs.
- **#149** `main` is stale (May, Go 1.25): update it, or make `dev` the default branch.
- **#152** `llms-full.txt` with the whole reference.
- **#153** Standard library gaps: a secure random token, and JSON into typed values.

## G. Self-hosting

- **#82** The compiler, written in Eyelang, builds itself. The lexer (S.1) is done; the parser,
  the formatter, `eye doc` and the checker come next, then the bootstrap.
- **#116** S.2, the runner in Eyelang, so a job program has no Go in it and can beat Go on memory.

## H. Parked until v0.4 is out

- The package manager, phases P0 to P3 (open questions 5 and 18).
- Open question 41: runs pinned to the code they started on.

## I. Open questions not yet decided

From [SPECS.md's open questions](/design/open-questions):

| # | Question |
|---|---|
| 1 | Time conditions in views ("idle for 20 minutes") |
| 3 | Exposing entities and jobs to other services |
| 5 | Packages |
| 7 | Metaprogramming after v1.0 |
| 10 | Streaming model output |
| 11 | Prompts as text files |
| 12 | Request-scoped values (a trace id, the current user) |
| 13 | `eye fmt` and the two forms of `if` |
| 14 | `if let` |
| 18 | Contracts shared by programs |
| 23 | Where `!` goes on a helper |
| 24 | `dyn Store`, and traits that name their own type |
| 26 | What `@memory` measures |
| 27 | One flag for the store |
| 28 | Several runners on one database |
| 29 | The shape of task groups |
| 30 | Model prices and routing |
| 31 | `Option` parameters with defaults |
| 32 | Time-travel debugging |
| 33 | Effects of a function that is only stored |
| 35 | Who may publish an event |
| 36 | `@destructive` means two things |
| 37 | One shape for "time ran out" |
| 38 | Slices and memory |
| 39 | A module with two contracts |
| 40 | What `main` repeats |
| 41 | Runs pinned to the code they started on |
| 44 | Regex patterns checked at compile time, and raw strings |
| 46 | A permission model for effects |
| 47 | How fast a million lines compile |
| 48 | Events in the network policy |

## J. Findings without an issue

- A generic private function that calls itself at a bigger type is reported by
  `eye check --lir`, not by `eye check`.
- The lab's `trains.watch` stops growing in the homelab only once its pin moves past checkpoints.

## K. v1.0

A syntax freeze, migration tooling (`eye fix` grows into it), `eye why`, hot code replacement, and
complete documentation.

## L. Decisions waiting on the owner

1. Events need the contract's `@auth`: add `@auth` to the lab's Telegram contract, with Telegram's
   webhook secret header?
2. Should a refused request (a 404 from a missing URL) be retryable, through a fault of its own?
3. `0.0 == -0.0` is false, beside `NaN == NaN` being true: confirm.
4. Native recursion is limited by the stack's size, not by exactly 100,000 calls: keep?
5. `regex` and `http.get` in compiled code are answered by the Go runner: keep?
6. Release v0.3 now?
