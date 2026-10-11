# Eyelang 2 roadmap

**Status, 2026-10-11:** v0.2.0 is released (phase 5; tag `v0.2.0`), after v0.1.0 (phases 0–4,
2026-10-10). Development and releases happen on `dev`; `main` stays at Eyelang 1 until that
changes.

What runs today, feature by feature: [docs/reference/status.md](docs/reference/status.md).

## What's next

The answer to "what's next?". Keep it current: when a task lands, or a check finds a gap, change
this list in the same commit. Check it against the code before relying on it.

**Owed by releases already made**

- Phase 5's exit test ran the lab's digest against stand-ins for its services. Nothing written in
  Eyelang runs in the homelab yet, so the exit criterion "the morning digest runs in the lab" is
  still open: deploy the digest on Postgres there.
- Pipelines over entity sets compiled to SQL (decision log, "Queries") were listed for v0.2 and
  not built: `World.all |> filter(…)` loads every entity. Not yet a task of any phase.

**Gaps in the core**, which phase 7 and self-hosting need closed, so they come first:

- Function types in signatures aren't typed: a lambda passed for `f: fn(Int) -> Int`, and what
  calling `f` returns, aren't checked.
- A private function's inferred type is one type for the whole program, not generic.
- `NonZero`, `opaque`, and the effects of functions kept in data aren't enforced; the effects
  `secret`, `publish`, `consume` and `ffi` aren't inferred.
- `Bytes` and fixed-width integers have no operations, and `s.chars()`, `xs.iter()` and
  `s.slice()` don't exist (and aren't reported as missing).

**Designed, not built:** `lock!`, `all!` and `first!`, ending an entity with `end(ok)`, `cron(…)`
schedules, `Problem` and `.context(…)`, `fs`, `process` and `random`, the `Killed` fault,
`onGap:` and `onOverlap:`, `with memory(…)` in tests, and several runners on one Postgres database.

**Built, not tested:** `start(at:)`, `@on(leaves(view))`, `$EYE_SECRETS`, a rollback after a fault,
and `Type.f()` through a module (`lex.Lexer.new`), which fails.

**Ahead, in order**

1. The core gaps above, function types and generic private functions first.
2. The digest in the homelab, closing phase 5's exit criterion.
3. Phase 6 (v0.3) broken into tasks, beside 6.1 (#42): `@port` and `@allow` enforced (its exit
   test), `@fresh` and `@destructive`, Notify generated from its contract, Go and TypeScript types
   and network policy from contracts, model `tools:`, the `agents` module and `eye test --live`.
   Decide there whether SQL pipelines join it.
4. Self-hosting's early start: an Eyelang lexer as a test program, checked by the conformance
   suite. Not begun.
5. Phase 7 (v0.4), after 1: typed IR (#28), code generation, reference counting with in-place
   updates, generic specialisation, jobs as short-lived processes.

Open questions 1–31 in [SPECS.md](SPECS.md) are settled as real programs ask; those on `!` (23),
`Self` (24) and what `@memory` measures (26) bear on phase 7.

What gets built, in what order, and how we know each part is done. The language itself is defined
in [SPECS.md](SPECS.md).

**Status lives in the GitHub issues**, not in this file. Each phase has an issue, and each task in
v0.1 is a sub-issue of its phase. A pull request closes its task issue (`Closes #N`), and a phase
is done when all its sub-issues are closed and its exit test passes.

## Versions

| Version | Phases | What you can do with it |
|---|---|---|
| **v0.1** | 0–4 | Write, check, format, run and test Eyelang 2 programs, including entities, jobs and views, against an in-memory store with a controllable clock |
| v0.2 | 5 | Run jobs durably, with Postgres selectable as the store; typed model calls; `dyn Trait`, task groups and channels; `@memory`, `@priority` and `@budget` enforced; approvals that reach people; the morning digest runs in the lab |
| v0.3 | 6 | Describe other services with contracts; network policy and Go/TypeScript types are derived from them; jobs run coding agents |
| v0.4 | 7 | Compile to native code with reference counting; jobs run as short-lived processes |
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
whole program rather than generic.

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
| 0.1 Tag the last Eyelang 1 commit as `eyelang-1` | [#9](https://github.com/ruxxzebre/eyelang/issues/9) |
| 0.2 Remove dead packages: `featureflags`, `assert`, `exploration`, `testing` | [#10](https://github.com/ruxxzebre/eyelang/issues/10) |
| 0.3 Stop committing binaries: remove `bin/`, build releases in CI | [#11](https://github.com/ruxxzebre/eyelang/issues/11) |
| 0.4 New layout and `eye` command skeleton | [#12](https://github.com/ruxxzebre/eyelang/issues/12) |
| 0.5 CI: build, vet, lint and test the new layout | [#13](https://github.com/ruxxzebre/eyelang/issues/13) |

Order: 0.1 first, because later tasks delete code the tag preserves. The rest can run in any order.

### Phase 1: Syntax — [#2](https://github.com/ruxxzebre/eyelang/issues/2) · v0.1

Done when every example in SPECS.md parses identically with both parsers, the conformance suite runs
in CI, and `eye fmt` gives the same output when run twice on the whole corpus.

| Task | Issue |
|---|---|
| 1.1 tree-sitter grammar for the Eyelang 2 syntax | [#14](https://github.com/ruxxzebre/eyelang/issues/14) |
| 1.2 tree-sitter external scanner: interpolation and the `!` marker | [#15](https://github.com/ruxxzebre/eyelang/issues/15) |
| 1.3 Hand-written lexer with byte spans and trivia | [#16](https://github.com/ruxxzebre/eyelang/issues/16) |
| 1.4 Hand-written lossless parser with Pratt expressions and error recovery | [#17](https://github.com/ruxxzebre/eyelang/issues/17) |
| 1.5 Conformance suite: both parsers on `testdata/syntax` in CI | [#18](https://github.com/ruxxzebre/eyelang/issues/18) |
| 1.6 `eye fmt` on the lossless tree | [#19](https://github.com/ruxxzebre/eyelang/issues/19) |
| 1.7 Editor highlighting queries | [#20](https://github.com/ruxxzebre/eyelang/issues/20) |
| 1.8 Syntax for the general-purpose core: `mut`, tuples, characters, ranges, `impl Type`, `where`, `extern` | [#39](https://github.com/ruxxzebre/eyelang/issues/39) |
| 1.9 VS Code: TextMate grammar, `eye check`, and `eye lsp` (errors, formatting, outline, semantic tokens) | [#43](https://github.com/ruxxzebre/eyelang/issues/43) |

Order: the tree-sitter track (1.1 → 1.2) and the compiler track (1.3 → 1.4) can run in parallel.
1.8 follows 1.4. 1.5 needs both tracks and 1.8, and 1.6 needs 1.8. 1.7 needs 1.8, and 1.9 needs 1.6.

### Phase 2: Checker — [#3](https://github.com/ruxxzebre/eyelang/issues/3) · v0.1

Done when every error class the spec promises has a diagnostic test with an exact source span, and
all SPECS.md examples pass the checker.

| Task | Issue |
|---|---|
| 2.1 Name resolution, modules, `import` and `pub` | [#21](https://github.com/ruxxzebre/eyelang/issues/21) |
| 2.2 Type inference, generics and signature checking | [#22](https://github.com/ruxxzebre/eyelang/issues/22) |
| 2.3 Exhaustive pattern matching and destructuring | [#23](https://github.com/ruxxzebre/eyelang/issues/23) |
| 2.4 Result-only errors and total arithmetic | [#24](https://github.com/ruxxzebre/eyelang/issues/24) |
| 2.5 Traits, methods, impls and derived traits | [#25](https://github.com/ruxxzebre/eyelang/issues/25) |
| 2.6 Effect inference, `uses`, and the `!` and `Save` rules | [#26](https://github.com/ruxxzebre/eyelang/issues/26) |
| 2.7 Diagnostics with spans, and a test per error class | [#27](https://github.com/ruxxzebre/eyelang/issues/27) |
| 2.8 Mutation: `mut` parameters, places and the no-aliasing rule | [#40](https://github.com/ruxxzebre/eyelang/issues/40) |

Order: 2.1 → 2.2, then 2.3, 2.4, 2.5 and 2.8 in any order, then 2.6. Start 2.7 with 2.1 and extend it in
every later task.

### Phase 3: Interpreter — [#4](https://github.com/ruxxzebre/eyelang/issues/4) · v0.1

Done when ordinary programs (no entities or jobs yet), starting at `main` with file and process
effects, and their `test` blocks run through `eye run` and `eye test`, and the REPL evaluates
expressions and declarations.

| Task | Issue |
|---|---|
| 3.1 Typed IR and lowering: moved to phase 7 as 7.1 | [#28](https://github.com/ruxxzebre/eyelang/issues/28) |
| 3.2 Reference interpreter for the core language | [#29](https://github.com/ruxxzebre/eyelang/issues/29) |
| 3.3 Standard library basics | [#30](https://github.com/ruxxzebre/eyelang/issues/30) |
| 3.4 `eye run`, `eye test` and the REPL | [#31](https://github.com/ruxxzebre/eyelang/issues/31) |
| 3.5 Structured concurrency on lightweight tasks | [#41](https://github.com/ruxxzebre/eyelang/issues/41) |
| 3.6 `extern` functions: moved to phase 6 as 6.1 | [#42](https://github.com/ruxxzebre/eyelang/issues/42) |

Order: 3.2 → 3.4. 3.3 can start alongside 3.2. 3.5 needs 3.2. The interpreter runs on the untyped IR
of the thin slice, so the typed IR waits for native compilation, which needs it; the standard
library's own host functions work, so other programs' externs wait for phase 6.

### Phase 4: Entities, jobs and views in memory — [#5](https://github.com/ruxxzebre/eyelang/issues/5) · v0.1 release

Done when the tour example in SPECS.md §3 runs deterministically under `eye test`, and every saved job
state is serialised to bytes and restored at each checkpoint.

| Task | Issue |
|---|---|
| 4.1 Entities: state, atomic methods, `Ref`, virtual lifecycle | [#32](https://github.com/ruxxzebre/eyelang/issues/32) |
| 4.2 `Store` trait and the in-memory store, written in Eyelang | [#33](https://github.com/ruxxzebre/eyelang/issues/33) |
| 4.3 Jobs as resumable state machines, saved at every `!` | [#34](https://github.com/ruxxzebre/eyelang/issues/34) |
| 4.4 Waiting library and controllable clock | [#35](https://github.com/ruxxzebre/eyelang/issues/35) |
| 4.5 Views and triggers, schedules with time zones | [#36](https://github.com/ruxxzebre/eyelang/issues/36) |
| 4.6 Entity versions and `migrate from N` | [#37](https://github.com/ruxxzebre/eyelang/issues/37) |
| 4.7 Tour example end to end, and the v0.1 release | [#38](https://github.com/ruxxzebre/eyelang/issues/38) |

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
| 5.1 `dyn Trait` as a type of its own | [#45](https://github.com/ruxxzebre/eyelang/issues/45) |
| 5.2 The Postgres store, chosen when a program starts | [#46](https://github.com/ruxxzebre/eyelang/issues/46) |
| 5.3 Durable runs on Postgres survive restarts of the runner and the database | [#47](https://github.com/ruxxzebre/eyelang/issues/47) |
| 5.4 Typed model calls: providers and `@budget` | [#48](https://github.com/ruxxzebre/eyelang/issues/48) |
| 5.5 Task groups, `spawn` and bounded channels | [#49](https://github.com/ruxxzebre/eyelang/issues/49) |
| 5.6 `@memory` and `@priority` enforced at run time | [#50](https://github.com/ruxxzebre/eyelang/issues/50) |
| 5.7 `approval!` reaches people | [#51](https://github.com/ruxxzebre/eyelang/issues/51) |
| 5.8 `!` on every outside call in a job | [#52](https://github.com/ruxxzebre/eyelang/issues/52) |
| 5.9 The morning digest on Postgres, and the v0.2 release | [#53](https://github.com/ruxxzebre/eyelang/issues/53) |

Order: 5.1 → 5.2 → 5.3. 5.4, 5.5, 5.6, 5.7 and 5.8 can run in any order, alongside them; 5.4 and
5.6 both add run-time faults, so they share the code that ends a run. 5.9 comes last.

### Phase 6: Contracts — [#7](https://github.com/ruxxzebre/eyelang/issues/7) · v0.3

Done when one real lab boundary (for example Notify) is generated from its contract, and a call to an
undeclared port or by a caller not in `@allow` fails to compile.

The phase also brings coding agents (SPECS.md §19): a job starts a Claude Code or Codex session,
serves Eyelang functions to it as tools, and gets a typed result back, surviving restarts while it
waits.

| Task | Issue |
|---|---|
| 6.1 `extern` functions: Go host functions and C libraries (from phase 3) | [#42](https://github.com/ruxxzebre/eyelang/issues/42) |

The rest of the phase's tasks are added when v0.2 is close to done.

### Phase 7: Native compilation — [#8](https://github.com/ruxxzebre/eyelang/issues/8) · v0.4

Done when the morning digest runs as a native job process, with its idle cost and resume time
measured and recorded here.

| Task | Issue |
|---|---|
| 7.1 Typed IR and lowering (from phase 3) | [#28](https://github.com/ruxxzebre/eyelang/issues/28) |

### After v0.4: Self-hosting

Done when the Eyelang compiler, written in Eyelang, builds itself, and the second build's output
matches the first (SPECS.md §22). It starts early: once phase 3 runs programs, an Eyelang lexer and
parser are written as a test program and checked by the conformance suite. Then come the
formatter, `eye doc` and the checker.

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
- **Own paths, and say so.** Each active track lists the paths it changes, and others leave them
  alone or ask first. Files every track touches (SPECS.md's status line, decision log and open
  questions; ROADMAP.md) are merged by whoever merges, after `task generate`.
- **Close issues by commit.** A commit that finishes a task says `Closes #N`; the
  `Close issues` workflow closes it when the commit reaches `dev`, since GitHub does that only for
  the default branch. (It skips the push that creates a branch; issues finished before `dev` existed
  are closed by hand.)

Paths owned now (2026-10-11):

| Who | Worktree or branch | Paths |
|---|---|---|
| Main session (release, merges) | `dev` | merges; `.github/workflows`; ROADMAP.md, CHANGELOG.md |
| eyelang-42 (docs site) | `docs-site` | `docs/`, `tools/specpages`, `internal/apidoc`, `cmd/eye/doc.go` |
| eyelang-55 (language server) | `lsp-*` | `internal/lsp`, `check/index.go`, `check/resolve_test.go` |
| eyelang-3b (editor and tasks) | — | `Taskfile.yml`, `editors/vscode` |
| eyelang-de (process) | — | `CLAUDE.md` |
