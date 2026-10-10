# Changelog

## v0.1.0 — 2026-10-10

The first release of Eyelang 2: a statically typed language for programs that keep state and wait.
Write, check, format, run and test programs with entities, jobs, views, events and contracts; jobs
pause on `!` steps and carry on after a restart. It runs in a reference interpreter; native
compilation comes in v0.4 (ROADMAP.md).

### Commands

- `eye check`: names, types with inference, traits and `where` bounds, exhaustive matches, `Result`
  errors, effects and `uses`, `mut` without aliasing, entity migrations and attributes, each error
  at its exact line and column.
- `eye test`: `test` blocks with an in-memory store and a clock that moves only when the test says;
  other services replayed from recorded exchanges.
- `eye run`: runs `main`, then keeps the program's jobs going across restarts in a journal file:
  sleeps, schedules in their time zone (`daily`, `weekdays`, `every`), events, joins.
- `eye runs`: every run with its status, what it waits for and its steps.
- `eye repl`: expressions with their types, declarations, and jobs on a clock `:advance` moves.
- `eye fmt`, `eye doc`, `eye lsp` (errors, formatting, outline, go to definition) and the VS Code
  extension.

### The language

- Values, not shared objects: immutable data, local `mut`, no aliasing.
- Entities own state; their methods are transactions; views react to them; `@version` and
  `migrate from N` evolve what they store.
- Jobs wait for free: `sleep!`, `sleepUntil!`, `next!` for events, `poll!`, `approval!`, `join!`;
  each `!` step runs once and is recorded.
- Structured concurrency without `async`: `parallelMap`, `both`, `race`, `timeout`.
- Contracts describe other services, over HTTP; commands carry keys, a lost reply is
  `Uncertain(key)`, and `.reconcile!()` asks the owner instead of resending blindly. A program can
  serve a contract with `impl`.
- Typed model calls: `ai.generate!` asks for a value of an Eyelang type by its JSON Schema.
- `@concurrency`, `@limit` and `@deadline` are enforced; a job starts once per `key:`.
- Traits with derived `Eq`, `Ord`, `Show`, `Hash`, `Save` and `Schema`; operators are traits.

### Known limits

- An interpreter, not a compiler: a job that loops forever slows as its recorded history grows.
- One process per journal; the Postgres store and several runners come in v0.2.
- `@memory`, `@priority` and `@budget` are checked but not enforced; `approval!` is answered only
  by tests; `cron(…)` schedules aren't supported.
