# Eyelang 2 roadmap

**Status, 2026-10-12:** v0.2.0 is released (phase 5; tag `v0.2.0`), after v0.1.0 (phases 0–4,
2026-10-10). Phase 6 (v0.3) is under way, with 7.1 to 7.3 of phase 7 pulled into it. Development
and releases happen on `dev`; `main` stays at Eyelang 1 until that changes.

What gets built, in what order, and how we know each part is done. The language itself is defined
in [SPECS.md](SPECS.md); what runs today, feature by feature, in
[docs/reference/status.md](docs/reference/status.md).

**Status lives in the GitHub issues**, not in this file. Each phase has an issue, and each of its
tasks is a sub-issue of it; carry-over tasks (C.n) name the closed issue they continue instead. A
pull request closes its task issue (`Closes #N`), and a phase is done when all its sub-issues are
closed and its exit test passes.

## What's next

`task next` answers "what's next?" from what is true now: the last release and what landed since,
this section, the open issues, the gaps [docs/reference/status.md](docs/reference/status.md)
lists (`task next -- --all` names each), and who works on what. This section holds only what
those can't say, so it stays short: what is owed, and the order. Change it in the commit that
changes it, and set the date below when you check it; `task next -- --check`, part of
`task check`, fails when it was checked before the last release or names a closed issue.

Checked: 2026-10-12

Start with [HANDOFF.md](HANDOFF.md): the digest in the homelab, and findings that have no issue
yet.

**Owed by releases already made**

- Phase 5's exit criterion, "the morning digest runs in the lab." The digest runs in the homelab
  on Postgres, in shadow mode beside the TypeScript digest (session eyelang-af; HANDOFF.md says
  what is deployed and how to read its runs). Its first scheduled morning is 2026-10-12 07:40
  Europe/Dublin. The criterion closes after one good morning run; replacing the TypeScript
  digest is the user's decision.
- Closed tasks whose exit tests weren't met: carry-over tasks C.1, C.2, C.6 and C.7 under the
  phases (`end(ok)`, `lock!`, `all!` and `first!`, 10,000 waiting tasks, the spec's examples
  through the checker). C.3 (`cron` and the schedule policies) is done.
- Approvals are answered by id, and only by the person asked (C.8, done 2026-10-12), so the
  lab's approvals have buttons again. Who may publish an answer at all is open question 35.
- Pipelines over entity sets compiled to SQL (decision log, "Queries") were listed for v0.2 and
  not built: `World.all |> filter(…)` loads every entity. **Recommendation:** not phase 6. SQL
  saves nothing while the runner holds every entity in memory, which both stores do today (the
  Postgres store reads everything when it opens and keeps rows as bytes, not columns). It belongs
  with several runners on one database (open question 28), when the store stops living in one
  runner's memory, after v0.4. Until then the decision log should say that pipelines over entity
  sets run in memory; it doesn't yet, since the user hasn't taken the recommendation.

**Ahead, in order**

1. The core gaps that self-hosting and phase 7 need (G.1 to G.6, under the phases): lazy `iter`
   (#86) is left. Fixed-width integers, character literals as `Byte`s, `Type.f()` through another
   module and two checker bugs are done. The spec review of 2026-10-12
   ([docs/internals/reviews](docs/internals/reviews/conclusions-2026-10-12.md)) adds four
   correctness bugs to fix first: entity methods reaching outside, unrecorded reads in jobs,
   re-entrant entity calls losing a write, and effects missing at the boundary.
2. The digest in the homelab: read its first runs and fix what they show (HANDOFF.md), then move
   its pin to the latest `dev`, which brings checkpoints (`trains.watch` stops growing) and the
   image layout of C.11 (the program is `/lab/deploy/digest`).
3. Phase 6 (v0.3): 6.2, 6.3, 6.5 and 6.10 to 6.13 are done; 6.1, 6.4 and 6.6 to 6.9 are
   open, each with its issue. `eye gen policy` (6.5) writes plain Kubernetes NetworkPolicy, with
   pods labelled `eyelang.module`; the homelab names the namespace with `--namespace`. Since 6.13, `eye run` serves a contract over HTTP on its `@port`, which 6.4's round trip
   and the online playground (open question 34) build on. The phase's exit test is 6.6 (Notify,
   with the user, in the homelab repository) and 6.8 (agents, after 6.7).
4. Self-hosting's early start (#82): S.1 (#83), an Eyelang lexer as a test program, checked by
   the conformance suite. Not begun; it needs G.1, G.2 and G.4.
5. Phase 7 (v0.4): the typed IR (7.1), the low IR with specialisation (7.2) and checkpoints in
   the interpreter (7.3) are done. Next is the C back end (7.4), then reference counting (7.5);
   the runner hosting compiled programs (7.6) waits on the user's choice of how it loads them
   without cgo. The design is [docs/internals/compilation.md](docs/internals/compilation.md).

The decisions drafted in [docs/internals/proposals.md](docs/internals/proposals.md) were all
accepted on 2026-10-11, with the C back end and a shipped `zig cc` for phase 7; SPECS.md's
decision log records each. `eye` itself stays free of cgo. Still open for the user: how the Go
runner hosts compiled code without cgo (compilation.md, section 1), and whether a choice's label
comes from its `##` comment or its `Show` (open question 15).

Open questions 1–34 in [SPECS.md](SPECS.md) are settled as real programs ask; those on `!` (23),
`Self` (24) and what `@memory` measures (26) bear on phase 7. Time-travel debugging (32) is
wanted, not yet a task: it builds on the checkpoints of 7.3, and `eye test --debug` on a failing
test is the first step. An online playground (34) is wanted too, not yet a task: the core
already builds for WebAssembly, and its contracts demo needs `eye run` to serve a contract over
HTTP first.

## Versions

| Version | Phases | What you can do with it |
|---|---|---|
| **v0.1** | 0–4 | Write, check, format, run and test Eyelang 2 programs, including entities, jobs and views, against an in-memory store with a controllable clock |
| v0.2 | 5 | Run jobs durably, with Postgres selectable as the store; typed model calls; `dyn Trait`, task groups and channels; `@memory`, `@priority` and `@budget` enforced; approvals that reach people; the morning digest runs in the lab |
| v0.3 | 6 | Describe other services with contracts; network policy and Go/TypeScript types are derived from them; jobs run coding agents; jobs resume from saved frames, so a run that never ends stops growing |
| v0.4 | 7 | Compile to native code through C, with reference counting; jobs compiled to state machines, and a waiting run is a record in the store |
| v1.0 | — | Syntax freeze, migration tooling, `eye why`, hot code replacement, documentation |

**Scope is open.** It was frozen until the tour in SPECS.md §3 ran under `eye test`; since the thin
slice below runs it, the spec grows again where real programs show it should. The lab's workloads,
written in Eyelang, are the programs that decide.

**Thin slice, done 2026-10-10.** The tour runs under `eye test` (`testdata/programs/tour`), ahead of
the phases, by taking a thin cut through them:

- names and imports are resolved, and waiting calls checked (part of 2.1 and 2.6);
- programs are lowered to an IR without types (part of 3.1) and run by the reference interpreter
  (3.2), with `eye run` and `eye test` (3.4);
- entities with transactional methods (4.1), jobs that resume by replaying their recorded steps,
  saved as bytes (4.3), a test clock (4.4), and `enters`/`leaves` triggers (4.5);
- fakes replayed from recordings, checked against the contract (part of phase 6).

**The lab in Eyelang, 2026-10-10.** The homelab's stateful workloads are written in Eyelang in
`testdata/lab` and pass `eye check` and their tests. They brought multi-line strings, `name:`
arguments, `pub let`, zones by name and one-assignment branches into the syntax, and into the
interpreter: events (`publish`, `@on(event(…))`, `next!`), `poll!`, `sleepUntil!`, reconciling
through a query, and contracts served in Eyelang (part of 4.4, 4.5 and phase 6).

**Calling services, 2026-10-10.** Contracts go over HTTP (`@http`, `@auth`, the key's header),
values have one JSON shape everywhere, `ai.generate!` asks an OpenAI-compatible gateway for a typed
value by its schema, `http.get` reads pages, and secrets come from the environment or
`$EYE_SECRETS`. The lab's digest runs end to end against stand-ins for its services
(`TestDigestOverHTTP`). Part of phase 5's typed model calls and phase 6's contracts.

**Types and effects, first slice, 2026-10-10.** `eye check` infers and checks types (2.2), Hindley-Milner
style inside bodies against signatures as written, with generic type variables, nominal records,
variants, entities and their snapshots, contracts (a command's result includes `Uncertain`), events,
jobs and runs, and the built-in library. Matches must be exhaustive, a `let` without `else` must
match every value (2.3), and `?` wraps errors only into a variant that holds them (2.4). Effects are
inferred and a public function must state them with `uses`; waiting functions and entity calls in
jobs need `!`, and a Secret, Handle or function can't be held across a `!` (2.6). A call records the
type of its result (`ir.Call.Expected`), which a model call's schema comes from.

**Phase 2 finished, 2026-10-11.** Traits (2.5): declarations with default methods, `impl Trait for
Type` with coherence (an impl lives with its trait or its type), `where` bounds and traits as
parameter types, checked at every call and the only traits a type variable has inside its function;
operators as `Add`, `Sub`… impls, and derived `Eq`, `Ord`, `Show`, `Hash`, `Save` and `Schema` for
types whose parts have them, asked for where they are used (`==`, `<`, `"{x}"`, `sort`, what a job
saves, `ai.generate`). Mutation (2.8): `mut` arguments only and always for `mut` parameters, no
variable passed as `mut` twice or as `mut` and read in one call, `mut self` methods only on what can
change, and no closure capturing a `var` or `mut` parameter. Diagnostics (2.7):
`check/classes_test.go` lists every error class the spec promises, by section, with the test that
shows it at its exact place. The store's `Store` trait, which `MemoryStore` implements, closes 4.2.
Still approximated: function types kept in data, `dyn` (v0.2), NonZero divisors, which are their
number type, and the types of private functions that leave them out, which are one type for the
whole program rather than generic. (Since 2026-10-11 NonZero divisors are checked and private
functions are generic.)

**The durable runner, 2026-10-10.** `eye run` keeps a program's jobs going on the system clock and
across restarts, in a journal file (SPECS.md open question 9): sleeping runs wake on time,
`daily`, `weekdays` and `every` fire in their zone (also on a test's clock), a full disk stops
nothing for good, and `eye runs` shows every run with its status, what it waits for and its steps.
Its exit test kills `eye run` in the middle of a job's sleep and checks the job finishes on time
after a restart (`cmd/eye`, `TestRunSurvivesKill`). Part of 4.3, 4.4 and 4.5.

**Attributes and limits, 2026-10-10.** Every attribute is declared with where it goes and what it
takes, and checked; `@concurrency` (queued in order, scheduled starts skipped at the limit),
`@limit` and `@deadline` are enforced; a job started with `key:` starts once per key, and `main`
must give one (`testdata/programs/limits`).

**The store in Eyelang, and entity versions, 2026-10-10.** The in-memory store is written in
Eyelang (`check/runtime/store.eye`, 4.2): each commit is a pure function from one store value to the
next, and the journal wraps it. Entities have `@version(N)` and `migrate from N` (4.6): rows record
their version, migrations chain when a row is loaded, the checker refuses gaps and impure
migrations, and `eye run` refuses to start on rows no migration can bring to the current version
(`cmd/eye`, `TestRunMigratesStoredEntities`). The `Store` trait came with phase 2.

**The REPL and structured concurrency, 2026-10-10.** `eye repl` evaluates expressions, statements
and declarations, keeps bindings, entities and runs for the session, and runs jobs on a clock that
`:advance` moves (3.4; SPECS.md open question 6 decided). `parallelMap`, `both`, `race` and
`timeout` run tasks whose waits overlap: on goroutines on the system clock, and taking turns by
wake time on a test's clock, so tests stay the same on every run (3.5,
`testdata/programs/tasks`, `interp` `TestTasksOverlapTheirWaits`). Left from 3.5: the checker
doesn't yet refuse a durable step in a task in a job (the runtime does), and the cheap,
growing stacks of §13 are Go's goroutines for now.

**v0.1.0, 2026-10-10.** Phases 2–4 are done: the checker (types, traits, exhaustive matches,
effects, `mut` without aliasing, a test per error class), the interpreter with `eye run`, `eye test`
and `eye repl`, structured concurrency, and entities, jobs, views, events and schedules on a store
written in Eyelang, with entity migrations. Moved to v0.2, where model calls and memory pressure
arrive: enforcing `@memory`, `@priority` and `@budget` at run time (they are checked already), and
`approval!` reaching people (tests answer it).

**v0.2.0, 2026-10-11.** Phase 5 is done. The Postgres store (`eye run --store postgres://…`) keeps
runs through restarts of the runner and the database; `dyn Trait` is a type of its own; task groups
with `spawn` and bounded channels; model calls pick their provider and keep `@budget`; `@memory`,
`@priority` and `reserve!` are enforced; `approval!` reaches people through `approvals.requested`,
and `eye approve` answers; every outside call in a job is a `!` step. The exit test runs the lab's
morning digest under `eye run` on Postgres against stand-ins for its services, kills the runner
while the model writes and stops the database while Notify is sent the briefing, and the digest
goes out once; the same program runs on the journal and in memory (`cmd/eye`,
`TestMorningDigestOnPostgres`). Deploying the digest to the homelab itself is not part of the
release. Left for later: several runners on one database, typed function types in signatures,
`lock!`, `all!` and `first!`, and the open questions 22–31 the phase raised.

## Phases

### Phase 0: Clean slate — [#1](https://github.com/ruxxzebre/eyelang/issues/1) · v0.1

Done when the repository builds and tests the new skeleton in CI, the `eyelang-1` tag points at the
last Eyelang 1 commit, and no build output is committed.

| Task | Issue |
|---|---|
| 0.1 Tag the last Eyelang 1 commit as `eyelang-1` (done) | [#9](https://github.com/ruxxzebre/eyelang/issues/9) |
| 0.2 Remove dead packages: `featureflags`, `assert`, `exploration`, `testing` (done) | [#10](https://github.com/ruxxzebre/eyelang/issues/10) |
| 0.3 Stop committing binaries: remove `bin/`, build releases in CI (done) | [#11](https://github.com/ruxxzebre/eyelang/issues/11) |
| 0.4 New layout and `eye` command skeleton (done) | [#12](https://github.com/ruxxzebre/eyelang/issues/12) |
| 0.5 CI: build, vet, lint and test the new layout (done) | [#13](https://github.com/ruxxzebre/eyelang/issues/13) |

Order: 0.1 first, because later tasks delete code the tag preserves. The rest can run in any order.

### Phase 1: Syntax — [#2](https://github.com/ruxxzebre/eyelang/issues/2) · v0.1

Done when every example in SPECS.md parses identically with both parsers, the conformance suite runs
in CI, and `eye fmt` gives the same output when run twice on the whole corpus.

| Task | Issue |
|---|---|
| 1.1 tree-sitter grammar for the Eyelang 2 syntax (done) | [#14](https://github.com/ruxxzebre/eyelang/issues/14) |
| 1.2 tree-sitter external scanner: interpolation and the `!` marker (done) | [#15](https://github.com/ruxxzebre/eyelang/issues/15) |
| 1.3 Hand-written lexer with byte spans and trivia (done) | [#16](https://github.com/ruxxzebre/eyelang/issues/16) |
| 1.4 Hand-written lossless parser with Pratt expressions and error recovery (done) | [#17](https://github.com/ruxxzebre/eyelang/issues/17) |
| 1.5 Conformance suite: both parsers on `testdata/syntax` in CI (done) | [#18](https://github.com/ruxxzebre/eyelang/issues/18) |
| 1.6 `eye fmt` on the lossless tree (done) | [#19](https://github.com/ruxxzebre/eyelang/issues/19) |
| 1.7 Editor highlighting queries (done) | [#20](https://github.com/ruxxzebre/eyelang/issues/20) |
| 1.8 Syntax for the general-purpose core: `mut`, tuples, characters, ranges, `impl Type`, `where`, `extern` (done) | [#39](https://github.com/ruxxzebre/eyelang/issues/39) |
| 1.9 VS Code: TextMate grammar, `eye check`, and `eye lsp` (errors, formatting, outline, semantic tokens) (done) | [#43](https://github.com/ruxxzebre/eyelang/issues/43) |

Order: the tree-sitter track (1.1 → 1.2) and the compiler track (1.3 → 1.4) can run in parallel.
1.8 follows 1.4. 1.5 needs both tracks and 1.8, and 1.6 needs 1.8. 1.7 needs 1.8, and 1.9 needs 1.6.

### Phase 2: Checker — [#3](https://github.com/ruxxzebre/eyelang/issues/3) · v0.1

Done when every error class the spec promises has a diagnostic test with an exact source span, and
all SPECS.md examples pass the checker.

| Task | Issue |
|---|---|
| 2.1 Name resolution, modules, `import` and `pub` (done) | [#21](https://github.com/ruxxzebre/eyelang/issues/21) |
| 2.2 Type inference, generics and signature checking (done) | [#22](https://github.com/ruxxzebre/eyelang/issues/22) |
| 2.3 Exhaustive pattern matching and destructuring (done) | [#23](https://github.com/ruxxzebre/eyelang/issues/23) |
| 2.4 Result-only errors and total arithmetic (done) | [#24](https://github.com/ruxxzebre/eyelang/issues/24) |
| 2.5 Traits, methods, impls and derived traits (done) | [#25](https://github.com/ruxxzebre/eyelang/issues/25) |
| 2.6 Effect inference, `uses`, and the `!` and `Save` rules (done) | [#26](https://github.com/ruxxzebre/eyelang/issues/26) |
| 2.7 Diagnostics with spans, and a test per error class (done) | [#27](https://github.com/ruxxzebre/eyelang/issues/27) |
| 2.8 Mutation: `mut` parameters, places and the no-aliasing rule (done) | [#40](https://github.com/ruxxzebre/eyelang/issues/40) |

Order: 2.1 → 2.2, then 2.3, 2.4, 2.5 and 2.8 in any order, then 2.6. Start 2.7 with 2.1 and extend it in
every later task.

### Phase 3: Interpreter — [#4](https://github.com/ruxxzebre/eyelang/issues/4) · v0.1

Done when ordinary programs (no entities or jobs yet), starting at `main` with file and process
effects, and their `test` blocks run through `eye run` and `eye test`, and the REPL evaluates
expressions and declarations.

| Task | Issue |
|---|---|
| 3.1 Typed IR and lowering: moved to phase 7 as 7.1 | [#28](https://github.com/ruxxzebre/eyelang/issues/28) |
| 3.2 Reference interpreter for the core language (done) | [#29](https://github.com/ruxxzebre/eyelang/issues/29) |
| 3.3 Standard library basics (done) | [#30](https://github.com/ruxxzebre/eyelang/issues/30) |
| 3.4 `eye run`, `eye test` and the REPL (done) | [#31](https://github.com/ruxxzebre/eyelang/issues/31) |
| 3.5 Structured concurrency on lightweight tasks (done) | [#41](https://github.com/ruxxzebre/eyelang/issues/41) |
| 3.6 `extern` functions: moved to phase 6 as 6.1 | [#42](https://github.com/ruxxzebre/eyelang/issues/42) |

Order: 3.2 → 3.4. 3.3 can start alongside 3.2. 3.5 needs 3.2. The interpreter runs on the untyped IR
of the thin slice, so the typed IR waits for native compilation, which needs it; the standard
library's own host functions work, so other programs' externs wait for phase 6.

### Phase 4: Entities, jobs and views in memory — [#5](https://github.com/ruxxzebre/eyelang/issues/5) · v0.1 release

Done when the tour example in SPECS.md §3 runs deterministically under `eye test`, and every saved job
state is serialised to bytes and restored at each checkpoint.

| Task | Issue |
|---|---|
| 4.1 Entities: state, atomic methods, `Ref`, virtual lifecycle (done) | [#32](https://github.com/ruxxzebre/eyelang/issues/32) |
| 4.2 `Store` trait and the in-memory store, written in Eyelang (done) | [#33](https://github.com/ruxxzebre/eyelang/issues/33) |
| 4.3 Jobs as resumable state machines, saved at every `!` (done) | [#34](https://github.com/ruxxzebre/eyelang/issues/34) |
| 4.4 Waiting library and controllable clock (done) | [#35](https://github.com/ruxxzebre/eyelang/issues/35) |
| 4.5 Views and triggers, schedules with time zones (done) | [#36](https://github.com/ruxxzebre/eyelang/issues/36) |
| 4.6 Entity versions and `migrate from N` (done) | [#37](https://github.com/ruxxzebre/eyelang/issues/37) |
| 4.7 Tour example end to end, and the v0.1 release (done) | [#38](https://github.com/ruxxzebre/eyelang/issues/38) |

Order: 4.2 first, then 4.1 and 4.3 together. 4.4 needs 4.3. 4.5 needs 4.1 and 4.3. 4.6 needs 4.1.
4.7 comes last.

### Phase 5: Durable stores — [#6](https://github.com/ruxxzebre/eyelang/issues/6) · v0.2 release

Done when the morning digest runs in the lab as an Eyelang job on the Postgres store, survives
restarts of the runner and the database, and costs only the runner's memory while it waits; and the
same program runs unchanged on the in-memory store. The phase also brings `dyn Trait`, which
selecting a store at run time needs, task groups with bounded channels (SPECS.md §8 and §13), and
typed model calls with `ai.generate!`, since the digest summarises with a model (§19).
Durability for the in-memory store (SPECS.md open question 9) was decided on 2026-10-10: the
journal, which `eye run` already uses.

| Task | Issue |
|---|---|
| 5.1 `dyn Trait` as a type of its own (done) | [#45](https://github.com/ruxxzebre/eyelang/issues/45) |
| 5.2 The Postgres store, chosen when a program starts (done) | [#46](https://github.com/ruxxzebre/eyelang/issues/46) |
| 5.3 Durable runs on Postgres survive restarts of the runner and the database (done) | [#47](https://github.com/ruxxzebre/eyelang/issues/47) |
| 5.4 Typed model calls: providers and `@budget` (done) | [#48](https://github.com/ruxxzebre/eyelang/issues/48) |
| 5.5 Task groups, `spawn` and bounded channels (done) | [#49](https://github.com/ruxxzebre/eyelang/issues/49) |
| 5.6 `@memory` and `@priority` enforced at run time (done) | [#50](https://github.com/ruxxzebre/eyelang/issues/50) |
| 5.7 `approval!` reaches people (done) | [#51](https://github.com/ruxxzebre/eyelang/issues/51) |
| 5.8 `!` on every outside call in a job (done) | [#52](https://github.com/ruxxzebre/eyelang/issues/52) |
| 5.9 The morning digest on Postgres, and the v0.2 release (done) | [#53](https://github.com/ruxxzebre/eyelang/issues/53) |

Order: 5.1 → 5.2 → 5.3. 5.4, 5.5, 5.6, 5.7 and 5.8 can run in any order, alongside them; 5.4 and
5.6 both add run-time faults, so they share the code that ends a run. 5.9 comes last.

**The exit test was relaxed for the release.** v0.2.0 shipped when the digest ran under `eye run`
on Postgres against stand-ins for Miniflux, the proxy, FlareSolverr, the model gateway and Notify
(`cmd/eye`, `TestMorningDigestOnPostgres`), not in the lab itself. Deploying it to the homelab is
still owed ("What's next").

### Carried over: exit tests that closed tasks left unmet

Several tasks were closed with v0.1.0 and v0.2.0 although part of their "Done when" wasn't met,
checked against the code on 2026-10-11 (C.1 to C.7, one per closed task). The decisions of
2026-10-11 add four more that change what v0.2 shipped (C.8 to C.11). None has a phase issue;
each names the closed issue it continues.

| Task | Issue |
|---|---|
| C.1 Entities: `end(ok)`, and tests of rollback and the single writer (from 4.1) | [#65](https://github.com/ruxxzebre/eyelang/issues/65) |
| C.2 Waiting: `lock!`, `all!` and `first!` (from 4.4) | [#66](https://github.com/ruxxzebre/eyelang/issues/66) |
| C.3 Schedules: `cron`, `onGap:` and `onOverlap:`, and `leaves` tested (from 4.5) (done) | [#67](https://github.com/ruxxzebre/eyelang/issues/67) |
| C.4 `NonZero` divisors checked (from 2.4) (done) | [#68](https://github.com/ruxxzebre/eyelang/issues/68) |
| C.5 `eye fmt` writes public functions' `uses` clauses (from 2.6) (done) | [#69](https://github.com/ruxxzebre/eyelang/issues/69) |
| C.6 10,000 waiting tasks within a memory budget (from 3.5) | [#70](https://github.com/ruxxzebre/eyelang/issues/70) |
| C.7 Every SPECS.md example passes the checker (phase 2's exit test) | [#80](https://github.com/ruxxzebre/eyelang/issues/80) |
| C.8 Approvals answered by id, and only by the person asked (done) | [#71](https://github.com/ruxxzebre/eyelang/issues/71) |
| C.9 `!` on publishing and starting child runs; every effect inferred (done) | [#72](https://github.com/ruxxzebre/eyelang/issues/72) |
| C.10 A job changed under a paused run faults instead of replaying (done) | [#73](https://github.com/ruxxzebre/eyelang/issues/73) |
| C.11 Module roots in `eye.toml` (done) | [#74](https://github.com/ruxxzebre/eyelang/issues/74) |

What each task is, until its issue holds it:

- **C.1 Entities (#32, 4.1).** `end(ok)` doesn't exist (`eye check` says "undefined name end"),
  so retiring an entity without an `Approval` can't fail to compile. No test checks a rollback
  after a fault part way through a multi-entity call, the single-writer rule, or that a job
  started by a method that rolled back never runs. The issue's "rollback when a method returns
  `Err`" no longer holds: an `Err` is a value and commits (the guide's entities page says so);
  only a fault rolls back.
  **Done when:** `end(ok)` ends an entity and calling it without an `Approval` fails to compile;
  tests show a fault part way through a multi-entity call rolling back every entity, one writer
  at a time per entity, and a job started by a rolled-back method never running.
- **C.2 Waiting (#35, 4.4).** `lock!` is undefined. `all!` and `first!` resolve to the collection
  functions `all` and `first`, and `eye check` refuses them with `!` ("all neither waits nor
  reaches outside the program"). So there is no test of `first!` between an approval and a
  timeout, or of a lock that expires while its holder waits.
  **Done when:** `lock!`, `all!` and `first!` work as SPECS §15's table says, with a test of
  `first!` between an approval and a timeout and one of a lock that expires while its holder
  waits.
- **C.3 Schedules (#36, 4.5).** `@on(cron("…", in: zone))` is refused ("@on takes a schedule,
  such as daily(…)"). The daylight-saving behaviour is built and tested for `daily` at 01:30 in
  Dublin (`testdata/programs/schedules`), but it can't be chosen: `onGap:` and `onOverlap:` don't
  exist. `leaves(view)` is tested only by the guide's program
  (`docs/guide/code/views-events-schedules`), and an entity that enters, leaves and enters again
  has no test.
  **Done when:** `cron` schedules fire in their zone; `onGap:` and `onOverlap:` choose the
  policy, each with a test across a change; `leaves` and an entity that enters, leaves and
  enters again have tests in `testdata/programs`.
  Done on 2026-10-12: SPECS §20 settles the cron dialect (five standard fields, no extensions)
  and the policies, `onGap: Shift | Skip` and `onOverlap: First | Second | Both`, prelude variants
  defaulting to what `daily` did (decision-log row "Schedules"). `internal/cron` reads a line,
  and `check` reports a wrong one at the string; `interp/schedule.go` fires `daily`, `weekdays`
  and `cron` by one rule. Tests: `testdata/programs/schedules` (cron in New York and Dublin, each
  policy across a change), `testdata/programs/jobs/views.eye` (`leaves`, and enters, leaves and
  enters again), `interp/store_test.go` (`TestCronFiresEachTimeAcrossRestarts`),
  `internal/cron/cron_test.go`, and `check/types_test.go`.
- **C.4 `NonZero` (#24, 2.4).** `pub fn div(a: Int, b: Int) -> Int = a / b` checks:
  `NonZero(Int)` is treated as `Int`, so dividing by a plain `Int` variable is not an error.
  **Done when:** dividing by an `Int` that isn't a nonzero literal or a `NonZero(Int)` is an error
  that says to use `nonzero(n)`, with a test in `check/classes_test.go`; the lab still checks.
  Done on 2026-10-11 (#68), with the test in `check/types_test.go` ("dividing by Int fails when
  it is 0").
- **C.5 `eye fmt` writes `uses` clauses (#26, 2.6).** Done: as decided, it is the default when
  formatting a program's directory, not a `--signatures` flag; `--syntax-only` turns it off.
  **Done when:** `eye fmt` writes each public function's inferred `uses` into its signature, and a
  test shows it on a file with a missing and a stale `uses` (`TestFmtWritesUses`).
- **C.6 Many tasks (#41, 3.5).** No test runs 10,000 waiting tasks within a stated memory budget.
  **Done when:** a test starts 10,000 tasks that wait, and checks the memory they hold against a
  budget stated in the test.
- **C.7 SPECS.md's examples through the checker (phase 2's exit test, #3).** No test runs the
  checker on them. Run one by one, 4 of the 33 example files check; most of the others are
  fragments that use names they don't declare. They need a context to check in (a stub module
  per section, or a header naming what an example assumes) before the test can exist.
  **Done when:** a test checks every example in SPECS.md, each in its context, and fails on an
  example that doesn't check.
- **C.8 Approvals by id (#51, 5.7; open question 17).** `approvals.Answer` names its request's
  `id` instead of `what`. The runtime matches answers by id and refuses one whose `by` isn't the
  request's `from`, logging it. `eye approve TEXT` and `eye decline TEXT` answer the one waiting
  approval whose text contains `TEXT`, refuse when several do and list them with their ids, and
  `eye approve --id ID` answers one. `approve(…)` and `decline(…)` in tests fail when their text
  matches more than one. `Approval` gets `what` and `by` fields. The lab's `approving` module
  publishes the id. SPECS §15 has the rules. Deferred by the user on 2026-10-11: for now the lab
  answers only with `eye approve` and `eye decline`, and `approving` sends the operator a message
  saying how instead of buttons; tests show waiting, `within:`, decline and a restart around it.
  **Done when:** tests show two runs asking the same thing answered one by one, an answer by
  someone other than `from` refused while the run keeps waiting, an ambiguous `eye approve` and
  test `approve(…)` refused, and `ok.by` read in a job; the lab's tests pass.
  Done on 2026-10-12: answers match by id and count only from `from:`, and a refused one is
  logged once; `eye approve` and test `approve(…)` refuse text that no waiting approval or
  several contain, and answer as the person asked; `eye approve --id ID` answers one, and
  `eye runs` shows each approval's id. The lab's `approving` asks on Notify with buttons again.
  Any module may still publish an answer with `by: r.from` (open question 35).
- **C.9 Every recorded step marked (#52, 5.8, and #26, 2.6; open questions 19 and 22).** Done
  on 2026-10-11, except `eye fix`: the call sites that needed `start!` were changed by hand. The
  checker infers `publish T`, `consume T`, `secret S` and `ffi`, so a public function that
  publishes, reads a secret or calls an extern must list it in `uses`; in a job it requires
  `publish!`, `start!` on a child run and `!` on an extern call, and refuses `!` on them
  elsewhere. `print` keeps `stdio` but is not a step; logging is not an effect. `eye fix` (or
  the formatter) adds the `!` to existing code. SPECS §12 has the rules.
  **Done when:** `check/classes_test.go` has a test for each error with its span; the lab, the
  tour, the docs' programs and `testdata/programs` check with the new `!`.
- **C.10 Changed jobs under paused runs (open question 4) (done).** `replay` in
  `interp/jobs.go` compares each recorded step's call with the call it replays, and faults the
  run on a difference ("the code of wait changed while this run was paused: its step 1 called
  Out(first).add, and the code now calls sleep there") instead of decoding the wrong value. The
  check at start-up, which refuses to run on a changed resume point, came with 7.3. SPECS §15
  has the rules.
  **Done when:** a test pauses a run, changes the job's order of steps, and the resumed run ends
  with that fault rather than a wrong value; `eye runs` shows the fault
  (`TestAChangedHelperFaultsItsReplay`, `TestAChangedJobFaultsItsPausedRun`).
- **C.11 Module roots in `eye.toml` (done).** `modules = ["../.."]` in a program's `eye.toml`
  names more directories to find imports in, after the program's own, in order (SPECS §13). The
  checker reads it (`check/eyetoml.go`), so `eye check FILE`, `eye run` and the editor do too.
  `testdata/lab/deploy/digest` has lost its symbolic links, and the lab's README copies the lab
  instead of following them.
  **Done when:** a test finds an import through `modules`, another shows the error listing every
  directory searched, and `eye check` and `eye test` pass on `testdata/lab/deploy/digest` without
  links.

Also owed, found while preparing the digest's deploy. Each is small enough to be its own task:

- **`eye runs` shows what a step's model calls spent.** Each step records its tokens and dollars
  for `@budget` (`step.Tokens`, `step.USD` in `interp/runtime.go`), but `eye runs` doesn't show
  them, so nobody can see what a morning's digest cost.
- **The memory limit's default reads the container's limit.** Without `--memory-limit`, a runner
  admits runs against the host's memory, read from `/proc/meminfo` (`interp/hostmem_linux.go`).
  In a container, the cgroup limit is the real one, and a runner that ignores it is killed by the
  kernel instead of pausing runs.
- Done: **`xs += [x]` copied the whole list each time**, so a loop that built a list was quadratic
  in the interpreter (status.md, "Updates in place when unique"). Compiled code fixes it with
  reference counts (docs/internals/compilation.md); the interpreter fixes appends without them:
  a list's buffer records how many of its elements are in use, and a list whose length is that
  number appends in place, since every other list on the buffer sees only a shorter prefix; any
  other list copies first. A set's `add` and a map's `insert` of a new key work the same way
  (`interp/value.go`, `shelf`).

### Core gaps: what self-hosting and phase 7 need from the core

Gaps in the general-purpose core, found since v0.2 by writing real programs and by checking
status.md against the code on 2026-10-12. Like the carry-overs, they have no phase issue.

| Task | Issue |
|---|---|
| G.1 Fixed-width integers: `wrappingAdd`, `checkedAdd`, bit operations, `Int32.from` (done) | [#84](https://github.com/ruxxzebre/eyelang/issues/84) |
| G.2 A character literal is a `Byte` where one is expected, so list patterns over `Bytes` work (done) | [#85](https://github.com/ruxxzebre/eyelang/issues/85) |
| G.3 Lazy `xs.iter()` and `Iter(a)` | [#86](https://github.com/ruxxzebre/eyelang/issues/86) |
| G.4 `Type.f()` called through another module (`lex.Lexer.new`) (done) | [#87](https://github.com/ruxxzebre/eyelang/issues/87) |
| G.5 A type alias that names itself is an error, with a hint for a one-variant type (done) | [#88](https://github.com/ruxxzebre/eyelang/issues/88) |
| G.6 `impl Trait` for an entity or `Ref(Entity)` is refused, not ignored (done) | [#89](https://github.com/ruxxzebre/eyelang/issues/89) |
| G.7 Entity methods and served operations can't reach outside the program | [#90](https://github.com/ruxxzebre/eyelang/issues/90) |
| G.8 A job reads entities and views only through recorded steps | [#91](https://github.com/ruxxzebre/eyelang/issues/91) |
| G.9 A call tree that comes back to an entity shares its transaction's state | [#92](https://github.com/ruxxzebre/eyelang/issues/92) |
| G.10 Effects at the boundary: entity methods, jobs, starting jobs, and calls that wait | [#93](https://github.com/ruxxzebre/eyelang/issues/93) |
| G.11 One sweep of SPECS.md: stale text, unrecorded rules, contradictions, open questions, glossary | [#94](https://github.com/ruxxzebre/eyelang/issues/94) |

G.7 to G.11 come from the spec review of 2026-10-12 (docs/internals/reviews). Each issue says
what it is and when it is done. G.1, G.2 and G.4 come before S.1, the
self-hosted lexer; G.1 before 7.4 too.

### Phase 6: Contracts — [#7](https://github.com/ruxxzebre/eyelang/issues/7) · v0.3

Done when one real lab boundary (for example Notify) is generated from its contract, a call to an
undeclared port or by a caller not in `@allow` fails to compile, and a job reviews a pull request
through an agent session and returns a typed `Review` (as issue #7 says).

The phase also brings coding agents (SPECS.md §19): a job starts a Claude Code or Codex session,
serves Eyelang functions to it as tools, and gets a typed result back, surviving restarts while it
waits.

| Task | Issue |
|---|---|
| 6.1 `extern` functions: Go host functions and C libraries (from phase 3) | [#42](https://github.com/ruxxzebre/eyelang/issues/42) |
| 6.2 `@port` and `@allow` enforced (done) | [#54](https://github.com/ruxxzebre/eyelang/issues/54) |
| 6.3 `@fresh` and `@destructive` (done) | [#55](https://github.com/ruxxzebre/eyelang/issues/55) |
| 6.4 Go and TypeScript types, clients and server stubs from contracts | [#56](https://github.com/ruxxzebre/eyelang/issues/56) |
| 6.5 Network policy from contracts (done) | [#57](https://github.com/ruxxzebre/eyelang/issues/57) |
| 6.6 Notify's boundary generated from its contract | [#58](https://github.com/ruxxzebre/eyelang/issues/58) |
| 6.7 Model tools: functions passed as `tools:` | [#59](https://github.com/ruxxzebre/eyelang/issues/59) |
| 6.8 The `agents` module: coding-agent sessions owned by jobs | [#60](https://github.com/ruxxzebre/eyelang/issues/60) |
| 6.9 `eye test --live`, and recording real exchanges | [#61](https://github.com/ruxxzebre/eyelang/issues/61) |
| 6.10 `extern type` replaces `Handle`, and can't be saved (done) | [#62](https://github.com/ruxxzebre/eyelang/issues/62) |
| 6.11 Enums: `T.variants()` and `T.parse(s)` (done) | [#63](https://github.com/ruxxzebre/eyelang/issues/63) |
| 6.12 A test sees what a fake received (done) | [#64](https://github.com/ruxxzebre/eyelang/issues/64) |
| 6.13 `eye run` serves a contract over HTTP on its `@port` (done) | [#81](https://github.com/ruxxzebre/eyelang/issues/81) |

What each task is, until its issue holds it:

- **6.2 `@port` and `@allow` enforced.** `@allow(digest, trains, access, approving)` on a contract
  lists the modules that may call its operations; a call from any other module is a compile
  error at the call, and a name in `@allow` that isn't a module of the program is an error at the
  attribute. SPECS doesn't yet say what "a call to an undeclared port" is; the task settles it in
  SPECS first (the likeliest reading: a program may call a contract over the network only if the
  contract declares `@port`, unless the program serves it itself, and a program that serves a
  contract serves it on that port). Needs `pub` and `opaque` enforced first ("What's next",
  item 1), so module boundaries hold before service boundaries.
  **Done when:** a test shows each error with its span; the lab checks with Notify's `@allow` as
  it is, and fails when `approving` is taken out of it; the port rule is in SPECS §17 with a
  passing and a failing test.
  Done on 2026-10-11, with a different port rule than the likeliest reading, which would have
  given the lab's outside services (Telegram, Miniflux, NetBird) made-up ports: a contract that a
  program serves must declare `@port`, and so must one with `@allow`; a contract without a port
  is an outside service any module may call. A call from a module `@allow` doesn't list (its own
  module, its servers and tests aside), a name in it that the program can't import, and any
  call under `@allow(nobody)` are errors (SPECS §17, decision log "Ports and callers";
  `check/ports.go`, `check/ports_test.go`).
- **6.3 `@fresh` and `@destructive`.** A `@fresh` command takes a required `basedOn: Version`
  and adds `Stale` and `Expired` to its outcomes, which a `match` must handle. A `@destructive`
  command takes a required `Approval`, which only `approval!` makes. The same `@destructive` on a
  function passed to a model as a tool comes with 6.7.
  **Done when:** calling a `@fresh` command without `basedOn:`, or matching its result without
  `Stale` and `Expired`, fails to compile; calling a `@destructive` command without an
  `Approval` fails to compile; a served `impl` receives `basedOn` and can answer `Stale`; each
  has a test in `testdata/programs`.
  Done on 2026-10-12 (#55): the checker adds `basedOn: Version` and `approval: Approval` to the
  command's parameters, after its own and before `key` (`check/check.go`), and gives a `@fresh`
  command's result `Stale` and `Expired` as it gives every command `Uncertain` (`check/infer.go`,
  `check/exhaust.go`); `Expired` is `approval!`'s. A snapshot's `version` is now a `Version`, which
  has `==` and takes an integer literal. Over HTTP both go as arguments, 409 is `Stale` and 410
  `Expired`, and the outcomes are tagged objects (SPECS §17, decision log "Fresh and destructive
  commands"; `interp/json.go`, `interp/services.go`, `interp/fakes.go`). Tests:
  `check/fresh_test.go`, `testdata/programs/contracts`, `TestFreshAndDestructiveOverHTTP`.
  Serving these over HTTP, including reading an incoming `approval`, comes with 6.13.
- **6.4 Go and TypeScript from contracts.** `eye gen go|ts DIR` writes, for each contract, its
  types in the JSON shape of SPECS §17 (a variant type as a discriminated union on `"kind"`), a
  client that calls the operations over `@http`, and a server stub to implement. Build output,
  never committed.
  **Done when:** golden tests cover the lab's Notify and Miniflux contracts; the generated
  TypeScript passes `tsc --strict` and the Go `go vet`; a round trip test sends every type of the
  lab's contracts through a generated client to an Eyelang `impl` and back unchanged.
- **6.5 Network policy from contracts.** From each contract's `@port` and `@allow` and the program's
  imports, `eye gen policy DIR` writes which module may reach which service on which port. The
  first output format is the one the homelab deploys with, which the user names.
  **Done when:** for the lab, the generated policy lets `digest`, `trains`, `access` and
  `approving` reach Notify on its port and nothing else reach it, checked by a golden test; a
  call that 6.2 refuses has no rule.
  Done on 2026-10-12, in the format the user named: plain Kubernetes `NetworkPolicy`
  (`networking.k8s.io/v1`), pods labelled `eyelang.module: <module>`, the namespace from
  `eye gen policy --namespace`. Each contract with `@port` gets a policy selecting its serving
  module's pods (the contract's own module when nothing in the program serves it), letting in
  TCP on its port from the modules that call it, which `check.Callers` finds in the checked
  code, among those `@allow` lets (SPECS §17, decision log "Network policy";
  `internal/netpolicy`, `cmd/eye/gen.go`; golden `internal/netpolicy/testdata/lab.yaml`, and
  `TestRefusedCallHasNoRule` for a refused call).
- **6.6 Notify's boundary generated from its contract.** The phase's exit test. The homelab's
  Notify (TypeScript, `js/apps/notify` in the homelab repository) and the digest's callers build
  against the types 6.4 generates from `testdata/lab/notify/notify.eye`, so a change to the
  contract that breaks them fails their build. Needs 6.2 and 6.4, and the user, since the code
  is in another repository.
  **Done when:** Notify builds in the homelab against the generated types, and a test in this
  repository shows that a breaking change to the contract fails `tsc` on a copy of its handler.
- **6.7 Model tools.** `ai.generate!(…, tools: [searchIssues, label])`: a tool's parameter types
  give its input schema and its `##` comment its description; the runtime runs a tool when the
  model calls it and sends back the result. The tools' effects are the call's effects. In a job,
  each tool call is a recorded step; a `@destructive` tool waits for an approval of that one call.
  Needs typed function types ("What's next", item 1), so that a function passed by name keeps
  its effects.
  **Done when:** a test replays recorded model exchanges that call two tools; a public function
  that passes `label` without `uses {net GitHub.label}` fails to compile; a runner killed between
  tool calls resumes without calling a tool twice; a `@destructive` tool waits for `approve(…)`.
- **6.8 The `agents` module.** `claude.start!(prompt:, workdir:, tools:)` and `codex.start!(…)`
  begin a session through the harness's SDK; `send!` adds a message; `result!()` waits for the
  final answer and decodes it into the expected type. A job holds only the session's id, so it
  waits without memory and picks the session up after a restart. `tools:` are served to the agent
  over MCP, and permission requests arrive as events. Needs 6.7.
  **Done when:** the job of SPECS §19 reviews a pull request and returns a typed `Review`, in a
  test from a recorded session; the runner is killed while the session works and the run picks
  it up again; a permission request is answered through `approval!`.
- **6.9 `eye test --live`.** Runs tests against the real services and models instead of their
  recordings, for the modules named (`--live ai,notify`), and skips a test with a message when a
  service's address or secret is missing. `--record` writes what the real services answered as
  recordings that a plain `eye test` then replays.
  **Done when:** `eye test --live ai testdata/lab` runs the digest's model test against a real
  model when its key is set and skips it otherwise; `--record` writes a recording that the next
  `eye test` replays and passes.
- **6.10 `extern type` replaces `Handle` (open question 21).** `Handle(name)` leaves
  `ir/builtin.eye`, the checker and SPECS; a program's `extern type` has no `Save`, `Show`, `Eq`,
  `Hash` or `Schema` (`unsaveable` in `check/effects.go`, `check/traits.go`), while the built-in
  types declared as `extern type` keep their traits. `testdata/syntax/valid/extern.eye` uses an
  `extern type`. SPECS §18 has the rules.
  **Done when:** holding a value of a program's `extern type` across a `!` is a compile error
  with a test, as are printing, comparing and asking a model for one; `Handle` is undefined.
  Done on 2026-10-11 (#62): a program's `extern type` has none of `Eq`, `Ord`, `Show`, `Hash`,
  `Save` and `Schema` (`check/types_test.go`, "foreign code owns an extern type's values").
- **6.11 Enums (open question 15).** Done on 2026-10-11. A type whose variants have no fields
  gets `T.variants()` and `T.parse(s)`, derived as the traits are (SPECS §11), in the checker
  and the interpreter. The NetBird admission's buttons in `access.eye` come from
  `Admission.variants()`, and the answer is matched on `Admission.parse(…)`. `trains.eye` keeps
  its string buttons: "Not taking it" isn't a variant's name, and an impl of `Show` would make
  `parse("{v}")` miss, which waits on open question 15; `approving.eye` has no buttons. Labels
  are the variant's `Show` until open question 15 decides otherwise.
  **Done when:** tests show both functions, `parse` of an unknown name giving `None`, a
  misspelt variant and a missing arm in a match on `T.parse(…)` failing to compile, and
  `variants()` refused on a type with fields; the lab's NetBird admission matches an enum.
- **6.12 What a fake received (open question 16).** A test can't see which calls a fake
  answered, so the digest records the keys it sent with in an entity (`Day.sent`) to make sending
  testable. A test should be able to ask a fake what it received, for example `received(op)`,
  and fail when a recording has exchanges no call used. The form is still open question 16; the
  task settles it in SPECS first.
  **Done when:** SPECS §20 says how a test reads what a fake received; a test in
  `testdata/programs` checks a call's arguments that way; the digest's `Day.sent` workaround is
  gone.
  Done on 2026-10-11 (6134df0, open question 16 decided): a test asks a fake for the calls it
  answered with `fakes(…).calls(op)`, one record per call with the operation's parameters
  (`testdata/programs/events`, `TestFakeCalls`), and the digest's tests ask the fake what was
  sent instead of `Day.sent`. Still open: `fakes(…)` returns the plain `Handler` type, so
  `.calls(op)` on a clock fails only at run time.
- **6.13 `eye run` serves a contract over HTTP.** A program that serves a contract with `impl`
  answers it only from inside itself: `eye run` doesn't listen on the contract's `@port`, and the
  lab's tests stand in for services with Go handlers. `eye run` listens on each served contract's
  port, routes requests to the `impl` by the contract's `@http` attributes in the JSON shape of
  SPECS §17, checks `@auth`, and runs each operation as one transaction. 6.4's round trip and the
  online playground (open question 34) need it.
  **Done when:** a test runs two Eyelang programs, one serving the lab's Notify contract on its
  port and one calling `notify.send!`, and shows the call, a repeat with the same key answered
  without a second send, and an unauthorised request refused.
  Done on 2026-10-12: `interp/listen.go` serves each served contract on its port from `eye run`
  (`interp/serve.go`), read the way `callHTTP` sends it; requests take the interpreter's lock and
  are answered between steps and while runs wait. SPECS §17 "Serving over HTTP" and a
  decision-log row settle the open points: `NOTIFY_PORT` moves the port (`0` for any free one),
  401 for a missing `@auth` secret, the lowest status `status:` gives a result's variant, else
  200, 400/404/405 with `{ "error": … }`, 500 for a fault, and no reply cache (the key reaches
  the `impl`). `TestServeContractOverHTTP`, `TestCallServedContract`, `TestServeWhileARunWaits`
  and `TestServeNeedsItsSecretAndPort` (`interp/listen_test.go`) cover routing, decoding,
  statuses, auth, a fault's rollback, a request answered while a run waits, and start-up errors;
  `TestServeNotifyBetweenPrograms` (`cmd/eye/serve_test.go`) runs the lab's notifier and a caller
  as two `eye run`s. Still open: `@allow` isn't enforced at the port (6.5), and an operation
  without `@http` has no route.

Order (6.2, 6.10, 6.11, 6.12 and 6.13 are done): 6.1 any time; the interpreter loads C libraries
without cgo, as `eye` stays free of it. 6.5 now that 6.2 is done. 6.4, then 6.6; 6.4's round
trip to an Eyelang `impl` can use `eye run` serving the contract on its `@port` (6.13). 6.3 any
time. 6.7 now that function types state their
effects, then 6.8. 6.9 any time, and early: the lab's tests need it. Pipelines compiled to SQL
are not part of this phase ("What's next").

### Phase 7: Native compilation — [#8](https://github.com/ruxxzebre/eyelang/issues/8) · v0.4

Done when the morning digest runs as a native job, with its idle cost and resume time measured
and recorded here. A waiting run is a record in the store, not a process: v0.4's runner is a
native process that executes runs as compiled state machines, one runner per store (decision of
2026-10-11, [proposals](docs/internals/proposals.md), contradiction 1).

The design is [docs/internals/compilation.md](docs/internals/compilation.md): C as the back end,
compiled by the `zig cc` that `eye` ships (decided 2026-10-11), Perceus reference counting,
whole-program specialisation, and jobs resuming from saved frames.

| Task | Issue |
|---|---|
| 7.1 Typed IR and lowering (from phase 3) (done) | [#28](https://github.com/ruxxzebre/eyelang/issues/28) |
| 7.2 Low IR and specialisation (done) | [#75](https://github.com/ruxxzebre/eyelang/issues/75) |
| 7.3 Jobs as state machines, resuming from saved frames (done in the interpreter) | [#76](https://github.com/ruxxzebre/eyelang/issues/76) |
| 7.4 The C back end and the core runtime | [#77](https://github.com/ruxxzebre/eyelang/issues/77) |
| 7.5 Reference counting and reuse | [#78](https://github.com/ruxxzebre/eyelang/issues/78) |
| 7.6 The runner hosts compiled programs, and the digest runs natively | [#79](https://github.com/ruxxzebre/eyelang/issues/79) |

- **7.1 Typed IR.** The checker keeps what it computes: a resolved type on every expression,
  every call resolved to its target, the impls each generic call's `where` bounds use, and each
  function's effects. The interpreter runs it, and a record knows its type, so `typeOf` goes.
  **Done when:** no expression of any program in the repository (the lab, `testdata/programs`,
  the docs' programs, the tour) has `unknown` or an unresolved type variable, and
  `eye check --ir` prints the typed IR, covered by golden tests.
  Done on 2026-10-11: every expression has its `ir.Ty` (`TestTypedIR`, beside
  `TestProgramsAreFullyTyped`); a method call has its `Target` and an operator on a declared type
  its `Impl`; each function, lambdas too, has its `Effects`; a record value knows its type, and
  nothing guesses it from fields (records saved before that come back without one). `eye check
  --ir` prints it, with
  golden tests in `testdata/ir`. Not kept separately: the impls a generic call's `where` bounds
  use. The callee's type at the call (its `Ref`'s `Ty`) says what each type variable is, and
  7.2's specialisation looks the impl up from the trait and that type.
- **7.2 Low IR and specialisation.** A second IR built from the typed IR, which C is printed
  from: monomorphic, in blocks with named values, with `dup`, `drop` and reuse explicit. Generic
  code is specialised from the program's roots; entities, jobs, views and contracts are lowered
  here, each by its own pass (compilation.md, sections 3 and 5).
  **Done when:** every repository program lowers to monomorphic low IR; the lab's generic code is
  specialised; polymorphic recursion is reported as an error at the call.
  Done on 2026-10-11 (package `lir`; `eye check --lir` prints it). From the roots (every body
  whose signature has no type variable), each generic function is lowered once per set of types
  its calls use: a call's types are matched against the callee's `ir.Func.Sig`, a trait's method
  on a known type goes to that type's impl (or the trait's default), an operator on a declared
  type to its impl, and a value converted to `dyn Trait` gets a table of instances. Instances
  are blocks of named values ending in jumps, branches, returns or stops; patterns are tests
  and field reads; loops are blocks; `?` and `??` are branches. Entities, jobs, views and
  contracts are lowered by their own rules (`lir/domain.go`): an entity method takes the state
  first, a job's `!` calls are marked durable, and entity calls, operations, starts and events
  are runtime calls. `lir.Verify` checks every repository program (`TestEveryProgramLowers`):
  no type variable, every block ends, every call names an instance with the right arity, every
  value read is made. Polymorphic recursion is an error at the inner call
  (`TestPolymorphicRecursion`). Golden files in `testdata/ir` (`lir.golden`). The lab declares
  no generic function and calls none from the prelude: the collection functions are built-ins,
  which the back end specialises by their arguments' types, as derived `show`, `eq` and `cmp`
  are. `dup`, `drop` and reuse are inserted by 7.5.
- **7.3 Jobs as state machines (open questions 9 and 4).** A pass splits each waiting function
  into states, and a checkpoint saves the stack of frames (code, state, live variables) instead
  of a log of steps. The interpreter runs it in v0.3, ahead of the rest of the phase. Runs saved
  as step logs are replayed once and then saved as frames. A run's record keeps its resume
  point's identity, and `eye run` refuses to start when a paused run's resume point changed
  (SPECS §15); `eye runs` names such runs. SPECS §15 then drops its sentence about replay.
  Done in the interpreter on 2026-10-11, in a simpler form than the pass
  (`interp/checkpoint.go`, compilation.md section 4): a run keeps a checkpoint at the start of
  each statement of its job's body, nested ones too (its variables, the path to the statement,
  where each loop on it is), and replays only the steps since. A function the statement calls
  replays its steps, so frames inside helpers wait for compiled jobs. Runs saved as step logs
  replay once and then keep checkpoints. A checkpoint names its statement by a hash of its typed
  IR; `eye run` refuses to start when a paused run's statement changed, a resumed run faults,
  and a replayed step for another call faults too (C.10). The lab's watcher keeps its
  `while true` loop. Tests: `interp/checkpoint_test.go`, `TestAWatcherKeepsItsRecordSmall` (a
  job shaped like the train watcher, a simulated week), `TestRunRefusesAChangedJob`.
  **Done when:** every job test passes with jobs resuming from frames, in the interpreter; the
  train watcher's record stays the same size over a simulated week; changing a job under a
  paused run stops `eye run` with the job and the number of runs.
- **7.4 The C back end and the core runtime.** C11 printed from the low IR, one file per module,
  with `#line` directives, compiled by `zig cc` (`-O0` for tests, `-O2` for `eye build`). The core
  runtime (allocation, reference counts, `Int` with big integers, strings, collections, hashing,
  formatting, `fs`, `process`, `stdio`) is written in C. `eye` releases ship `zig`.
  **Done when:** `eye build` makes a native binary of `testdata/programs/core` on the five
  release targets, and its tests pass.
- **7.5 Reference counting and reuse.** Perceus: precise `dup` and `drop`, borrowed parameters,
  reuse in `match`, and in-place updates when a value is unique; `mut` parameters become
  pointers.
  **Done when:** a loop that appends to a list runs in linear time and allocates once per growth;
  a test counts allocations and frees and finds no leak in the core programs.
- **7.6 The runner hosts compiled programs.** The Go runner calls a compiled program's interface
  (run `main`, run a test, call an entity method, step a job, encode and decode saved types)
  without cgo in `eye`; how it loads the program is open in compilation.md, section 1.
  **Done when:** the morning digest runs as a native job on the journal and on Postgres; its idle
  memory and resume time are measured and recorded in this file.

Order: 7.1, then 7.3, both in v0.3. 7.2 after 7.1, then 7.4, then 7.5. 7.6 last, after 7.3 and
7.5.

### After v0.4: Self-hosting — [#82](https://github.com/ruxxzebre/eyelang/issues/82)

Done when the Eyelang compiler, written in Eyelang, builds itself, and the second build's output
matches the first (SPECS.md §22). It starts early: once phase 3 runs programs, an Eyelang lexer and
parser are written as a test program and checked by the conformance suite. Then come the
formatter, `eye doc` and the checker.

| Task | Issue |
|---|---|
| S.1 An Eyelang lexer as a test program, checked by the conformance suite | [#83](https://github.com/ruxxzebre/eyelang/issues/83) |

Later tasks are added when phase 7 is close to done.

## Working agreements

- **One task, one pull request.** The pull request says `Closes #N` and keeps CI green.
- **A spec change comes first.** If a task needs the language to change, update SPECS.md in the same
  pull request (or an earlier one), and the decision log if it is a decision.
- **Syntax changes touch three places together:** `grammar.js`, the hand-written parser and the
  conformance suite.
- **New tasks** for phases 5–7 are added as sub-issues of their phase issue and listed here, when
  the phase before them is close to done.

## Working in parallel

Several sessions and agents often work at once, each in its own git worktree. So that their work
merges cleanly:

- **Name the base.** A track starts from a named commit on `dev` and says which in its first
  message and its report.
- **Rebase before reporting.** Before a track reports, it rebases onto the current `dev` tip and
  runs the checks again; the report gives the rebased commit. Whoever merges announces each new tip
  to the tracks still running.
- **Claim paths, and release them.** A track says which paths it changes when it starts, and gives
  them back when its work is merged: `task claim -- WHO PATH…` and
  `task claim -- --release WHO`. Claims live in the repository's git config, which every
  worktree shares and nothing commits, so `task next` always shows them as they are, with their
  age. Others leave claimed paths alone or ask first. A claim a day old whose worktree has no new
  commits is stale: ask, then take it. Files every track touches (SPECS.md's status line, decision
  log and open questions; ROADMAP.md) are merged by whoever merges, after `task generate`.
- **One merger in the main checkout.** Tracks work in worktrees of their own, on branches named
  `track/…`. The main checkout belongs to whoever claims `dev`: only they commit there or leave
  files uncommitted, and they merge what tracks hand them. `task next` lists every worktree with
  what it holds that `dev` doesn't, and the main checkout's uncommitted files.
- **Remove what is merged.** A worktree that `task next` shows as merged into dev, with nothing
  uncommitted, is removed: `git worktree remove PATH`.
- **Close issues by commit.** A commit that finishes a task says `Closes #N`; the
  `Close issues` workflow closes it when the commit reaches `dev`, since GitHub does that only for
  the default branch. (It skips the push that creates a branch; issues finished before `dev` existed
  are closed by hand.)
