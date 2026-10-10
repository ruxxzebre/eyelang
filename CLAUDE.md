# Working on Eyelang

Eyelang 2 is a statically typed, compiled language for programs that manage data across many
entities and run jobs that mostly wait. This file is the guide for decisions while it is being
designed and built, for people and coding agents alike. The language is defined in
[SPECS.md](SPECS.md), the syntax in [tree-sitter-eyelang/grammar.js](tree-sitter-eyelang/grammar.js),
and the plan in [ROADMAP.md](ROADMAP.md); progress lives in the GitHub issues.

## The main idea

**Eyelang is for software that keeps state and waits:** services, automations, schedulers and agents
that run for days, hold data, call other systems that fail, and spend most of their time waiting.
It makes such programs as simple to write as a script, and correct by construction.

Five pillars carry the idea. Everything else, including the functional core and safe parallelism,
is a means to them:

1. **Values, not shared objects.** Immutable data, local and visible mutation (`mut`), no aliasing.
   This is why parallel code has no data races and memory needs no garbage collector.
2. **State has an owner.** Shared data lives in entities with atomic methods.
3. **Waiting is free and durable.** Jobs pause on `!` steps for as long as they need, cost no memory
   while they wait, and survive restarts.
4. **Boundaries are typed and failure is explicit.** Contracts for services, schemas for AI calls,
   keyed and reconciled calls, `Result` instead of exceptions.
5. **Costs and effects are visible.** `uses`, `!`, `mut`, `dyn`, budgets.

**Scope test:** a feature belongs if it makes the general core better for everyone, or makes
stateful, waiting, failure-prone programs simpler or safer, and it breaks none of the pillars.
Long-running AI agents pass easily; a GUI toolkit or class inheritance would not.

## How to decide

When a choice comes up, weigh it against these, roughly in this order.

1. **Good programs come out.** We are building a fast, concise language that is a pleasure to read,
   type and review, and a feature earns its place by the programs it produces: correct, fast, lean
   on memory, well organised and easy to change. Prefer the design that makes the obvious program
   the correct one. If the easy way to write something is the wrong way, the language is wrong.
   How the compiler feels to work on comes second.
2. **Performance by construction.** Native code, reference counting with in-place updates, no GC
   pauses, and waiting that costs no memory. Don't add features whose cost is hidden or unbounded.
3. **Concise, without being cryptic.** Little ceremony, inference inside bodies, one obvious way to
   do each thing. A line of Eyelang should say what it does, and nothing extra. Prefer words to
   symbols unless the symbol is already universal.
4. **Organisation is enforced, not suggested.** State has one owner, effects appear in public
   signatures, matches are exhaustive, the formatter has no options. Structure that matters is
   checked by the compiler, not left to convention.
5. **Developer experience is part of the language.** Error messages point at exact spans and say
   what to do. Tools are fast. Tests are deterministic. Editor support, formatting and docs ship
   with the features, not after them.
6. **Strict, but never clumsy.** Every rule must pay for itself in ordinary code. When a rule makes
   normal code awkward (as a reserved `start` did for every lexer), change the rule.
7. **Visible where it matters.** Pillar 5 in practice: costs and risks show at the place they happen: `!` for a durable
   step, `mut` for a change, `uses` for effects, `dyn` for a run-time lookup, `unreachable` for a
   stop. Everything else stays quiet.
8. **A general language first.** Entities, jobs, views, contracts and AI calls sit on a core that
   stands on its own, good enough to write Eyelang's own compiler in.
9. **Agents are first-class users.** Much of the code will be written by coding agents. Strict
   types, canonical formatting, explicit signatures and recorded tests make their work checkable,
   and the same things make it pleasant for people.

### When to add syntax

Add syntax when it is **generally useful** and **clearly better than a library function or an
attribute**: it reads better, checks more, or removes a whole class of mistakes. Syntax that also
makes one area much better (jobs, agents, data) is welcome, as long as it pays off elsewhere too.
Avoid syntax that serves a single feature; that belongs in a library, in types or in effects.
Before adding a keyword, check it doesn't take away a name people will want: make it contextual.

### When to change a decision

Before v1.0 the language changes whenever real programs show a better way. Write the change down
(decision log or open question in SPECS.md), update every example, and keep old code working
through the formatter or an `eye fix` rewrite rather than through compatibility rules.

**The scope is open again** (since 2026-10-10, when the tour started running under `eye test`).
New rules and syntax are welcome when real programs ask for them: the lab's workloads (the digest,
Notify, reminders, approvals) written in Eyelang are the test. Each change still goes through the
scope test above and the steps below.

## How to make a change

- **Spec first.** A language change starts in SPECS.md: the rule, an example, and a decision-log
  row if it is a decision. Ideas that aren't settled yet go to the open questions at the end of
  the spec.
- **Syntax changes touch three places together:** `grammar.js` (and `src/scanner.c` if newlines,
  braces or strings are involved), the Go parser in `syntax/`, and tests in
  `tree-sitter-eyelang/test/corpus` and `testdata/syntax`. Then run `task generate`.
- **The conformance suite is the referee.** Both parsers must agree, token for token, on every file
  in `testdata/syntax` and the corpus. Add a valid file for each new construct and an invalid file
  for each new error.
- **Don't edit generated files.** `testdata/syntax/spec-*.eye` and `testdata/programs/tour/tour.eye`
  come from the examples in SPECS.md (`tools/specexamples`, using `internal/specdoc`);
  `syntax/kind_gen.go` and `syntax/view_gen.go` from the grammar's `node-types.json`
  (`syntax/gen`); and `tree-sitter-eyelang/src/` from `grammar.js`, except `scanner.c`. To change
  one, change its source and run `task generate`.
- **Iterate on one thing,** then run everything:
  `go test ./syntax -run 'TestConformance/valid/if-match.eye'` (one conformance file),
  `go test ./syntax -run TestFormat`, `go test ./internal/lsp`,
  `go run ./cmd/eye test testdata/programs/core` (the language's own tests),
  `cd tree-sitter-eyelang && tree-sitter test -i 'Waiting'` (corpus tests by name).
- **Run `task check` before pushing.** It runs everything CI runs: generated files, build, vet,
  lint, tests, formatting, the conformance suite, the grammar's corpus and the VS Code extension's
  tests.
- **One task, one change.** Commits and pull requests say `Closes #N` for the ROADMAP task they
  finish.

## Writing

Specs, docs, comments and error messages use plain, concrete English: short sentences, active
voice, an example where it helps, no hype. Say what a thing does and when to use it. Error messages
say what was expected, what was found, and where.

## Map

Everything in the repository is Eyelang 2 or its tooling. Eyelang 1 lives only at the tag
`eyelang-1`; `repl/` is the one piece carried over.

| Path | What it is |
|---|---|
| `SPECS.md` | The language: goals, decisions, rules, grammar sketch, open questions |
| `ROADMAP.md` | Phases, versions and tasks, each linked to an issue |
| `tree-sitter-eyelang/` | The grammar (the definition of the syntax), its scanner, corpus and queries |
| `syntax/` | Lexer, lossless parser, typed views, formatter; `syntax/gen` generates the views |
| `testdata/syntax/` | Conformance suite; `spec-*.eye` are generated from SPECS.md |
| `testdata/fmt/` | Formatter input and expected output |
| `cmd/eye/` | The `eye` command |
| `internal/lsp/` | The language server behind `eye lsp` |
| `internal/specdoc/`, `tools/specexamples/` | Extract SPECS.md's examples into `testdata/syntax` |
| `tools/spechtml/`, `internal/highlight/` | Render SPECS.md as one HTML page (`task spec`), highlighted by the parser |
| `internal/apidoc/` | The API reference `eye doc` writes from `##` comments |
| `editors/vscode/` | The VS Code extension: TextMate grammar and the `eye lsp` client |
| `check/` | Loads a program's modules, resolves names, checks types, matches and effects, reports errors; `prelude.eye` and `std/` are the standard library written in Eyelang, and `runtime/store.eye` the in-memory store the runtime keeps every program's state in |
| `ir/` | The resolved form programs are lowered to |
| `interp/` | Reference interpreter and runtime: entities, views, jobs, events, schedules, clock, fakes; the journal store and the runner behind `eye run` and `eye runs` |
| `testdata/programs/` | Programs whose tests `go test ./interp` runs; `tour/tour.eye` is generated from SPECS.md |
| `testdata/lab/` | The homelab's real workloads in Eyelang (digest, Notify, train reminders, NetBird access): the programs that decide what the language needs |
| `repl/` | Line editor with history, for `eye repl` |

Commands: `task check` (everything), `task generate` (after editing the grammar or a spec
example), `task conformance`, `task grammar`, `task vscode`,
`go run ./cmd/eye parse|check|fmt FILE`, `go run ./cmd/eye test|run|runs DIR` (`eye run` keeps
its state in `DIR/.eye/journal`; `--memory` keeps nothing).
