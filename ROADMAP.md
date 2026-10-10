# Eyelang 2 roadmap

What gets built, in what order, and how we know each part is done. The language itself is defined
in [SPECS.md](SPECS.md).

**Status lives in the GitHub issues**, not in this file. Each phase has an issue, and each task in
v0.1 is a sub-issue of its phase. A pull request closes its task issue (`Closes #N`), and a phase
is done when all its sub-issues are closed and its exit test passes.

## Versions

| Version | Phases | What you can do with it |
|---|---|---|
| **v0.1** | 0–4 | Write, check, format, run and test Eyelang 2 programs, including entities, jobs and views, against an in-memory store with a controllable clock |
| v0.2 | 5 | Run jobs durably, with Postgres selectable as the store; typed model calls; `dyn Trait`, task groups and channels; the morning digest runs in the lab |
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

**Types and effects, first slice, 2026-10-10.** `eye check` infers and checks types (2.2), Hindley-Milner
style inside bodies against signatures as written, with generic type variables, nominal records,
variants, entities and their snapshots, contracts (a command's result includes `Uncertain`), events,
jobs and runs, and the built-in library. Matches must be exhaustive, a `let` without `else` must
match every value (2.3), and `?` wraps errors only into a variant that holds them (2.4). Effects are
inferred and a public function must state them with `uses`; waiting functions and entity calls in
jobs need `!`, and a Secret, Handle or function can't be held across a `!` (2.6). A call records the
type of its result (`ir.Call.Expected`), which a model call's schema comes from. Not checked yet:
traits and their bounds (2.5: `where` is read but not enforced, and only the operators of the built-in
types and declared `add`, `sub`… methods are known), `mut` and aliasing (2.8), function types kept in
data, `dyn`, and NonZero divisors, which are their number type.

Still missing before v0.1: the rest of phase 2 (traits, mutation, diagnostics for every class), the store written in Eyelang (4.2),
schedules and time zones in triggers, a runner that keeps jobs across restarts, migrations (4.6),
and the REPL. `approval!` is answered only by tests so far.

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
| 3.1 Typed IR and lowering | [#28](https://github.com/ruxxzebre/eyelang/issues/28) |
| 3.2 Reference interpreter for the core language | [#29](https://github.com/ruxxzebre/eyelang/issues/29) |
| 3.3 Standard library basics | [#30](https://github.com/ruxxzebre/eyelang/issues/30) |
| 3.4 `eye run`, `eye test` and the REPL | [#31](https://github.com/ruxxzebre/eyelang/issues/31) |
| 3.5 Structured concurrency on lightweight tasks | [#41](https://github.com/ruxxzebre/eyelang/issues/41) |
| 3.6 `extern` functions: Go host functions and C libraries | [#42](https://github.com/ruxxzebre/eyelang/issues/42) |

Order: 3.1 → 3.2 → 3.4. 3.3 can start alongside 3.2; 3.6 alongside 3.3, since the standard
library's primitives are host functions. 3.5 needs 3.2.

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

### Phase 5: Durable stores — [#6](https://github.com/ruxxzebre/eyelang/issues/6) · v0.2

Done when the morning digest runs in the lab as an Eyelang job on the Postgres store, survives
restarts of the runner and the database, and costs only the runner's memory while it waits; and the
same program runs unchanged on the in-memory store. The phase also brings `dyn Trait`, which
selecting a store at run time needs, task groups with bounded channels (SPECS.md §8 and §13), and
typed model calls with `ai.generate!`, since the digest summarises with a model (§19).
Tasks will be broken down when v0.1 is close to done; durability for the in-memory store (SPECS.md
open question 9) is decided then.

### Phase 6: Contracts — [#7](https://github.com/ruxxzebre/eyelang/issues/7) · v0.3

Done when one real lab boundary (for example Notify) is generated from its contract, and a call to an
undeclared port or by a caller not in `@allow` fails to compile.

The phase also brings coding agents (SPECS.md §19): a job starts a Claude Code or Codex session,
serves Eyelang functions to it as tools, and gets a typed result back, surviving restarts while it
waits.

### Phase 7: Native compilation — [#8](https://github.com/ruxxzebre/eyelang/issues/8) · v0.4

Done when the morning digest runs as a native job process, with its idle cost and resume time
measured and recorded here.

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
