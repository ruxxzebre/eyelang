# Working on Eyelang

Eyelang 2 lets you write a program that runs for days (waits for people, keeps state, calls
services that fail) as plainly as a script, and its compiler catches the mistakes such programs
usually ship with. This file is the guide for working on it while it is being designed and built,
for people and coding agents alike. The language is defined in
[SPECS.md](SPECS.md), the syntax in [tree-sitter-eyelang/grammar.js](tree-sitter-eyelang/grammar.js),
and the plan in [ROADMAP.md](ROADMAP.md); progress lives in the GitHub issues.

**Start every session with `task next`.** It says what's next from what is true now (the last
release, ROADMAP.md's "What's next", open issues, the gaps in
[docs/reference/status.md](docs/reference/status.md)) and who works on what: each session's
claimed paths and every worktree with what it holds that `dev` doesn't. Asked "what's next?",
answer from it, and check what it says against the code. Before changing files, claim them
(`task claim -- WHO PATH…`), leave others' claims alone or ask their owner, work in a worktree of
your own unless you are the one merging into `dev`, and release the claim when your work is
merged (`task claim -- --release WHO`). When work lands or a check turns up a gap, update
status.md or "What's next" in the same commit; `task check` fails when "What's next" was last
checked before the latest release.

## The main idea

**Eyelang lets you write a program that runs for days (waits for people, keeps state, calls
services that fail) as plainly as a script, and the compiler catches the mistakes such programs
usually ship with.** Five promises say what a user gets:

1. **Waiting is one line.**
2. **Replays can't go wrong.**
3. **State has one owner.**
4. **Outside calls are typed and settle once.**
5. **Days of behaviour test in milliseconds.**

Everything else, the core language and safe parallelism included, is a means to them. The
promises, what each one means, and the principles we decide by live in
[SPECS.md section 1](SPECS.md#1-goals); this file doesn't keep a copy. The README and the docs site
are written from the promises, so a change that moves one changes them too.

**Scope test:** a feature belongs if it makes the general core better for everyone, or makes
stateful, waiting, failure-prone programs simpler or safer, and it breaks no promise or principle.
Long-running AI agents pass easily; a GUI toolkit or class inheritance would not.

## How to decide

Weigh every choice against the principles in [SPECS.md section 1](SPECS.md#1-goals), roughly in
their order. Read them there; in short:

1. Good programs come out: the obvious program is the correct one.
2. Values, not shared objects.
3. Performance by construction: no hidden or unbounded costs.
4. Concise, without being cryptic.
5. Organisation is enforced, not suggested.
6. Developer experience is part of the language.
7. Strict, but never clumsy.
8. Visible where it matters: `!`, `mut`, `uses`, `dyn`, `unreachable`, budgets.
9. A general language first, good enough one day to host its own compiler.
10. Agents are first-class users.

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
- **Writing an Eyelang program** (the lab's workloads in `testdata/lab`, examples, test
  programs)? Read [docs/writing-eyelang.md](docs/writing-eyelang.md) first.
- **Syntax changes touch three places together:** `grammar.js` (and `src/scanner.c` if newlines,
  braces or strings are involved), the Go parser in `syntax/`, and tests in
  `tree-sitter-eyelang/test/corpus` and `testdata/syntax`. Then run `task generate`.
- **The conformance suite is the referee.** Both parsers must agree, node for node, on every file
  in `testdata/syntax` and the corpus. Add a valid file for each new construct and an invalid file
  for each new error; `testdata/syntax/invalid/diagnostics.txt` holds each invalid file's exact
  messages (`go test ./syntax -run TestSyntaxErrors -update` rewrites it, and the diff shows what
  changed). After changing either parser, run `task fuzz`, which feeds both mutated inputs and
  saves any disagreement under `syntax/testdata/fuzz` as a test.
- **Don't edit generated files.** `testdata/syntax/spec-*.eye` and `testdata/programs/tour/tour.eye`
  come from the examples in SPECS.md (`tools/specexamples`, using `internal/specdoc`);
  `syntax/kind_gen.go` and `syntax/view_gen.go` from the grammar's `node-types.json`
  (`syntax/gen`); and `tree-sitter-eyelang/src/` from `grammar.js`, except `scanner.c`. To change
  one, change its source and run `task generate`.
- **Iterate on one thing,** then run everything:
  `go test ./syntax -run 'TestConformance/valid/if-match.eye'` (one conformance file),
  `go test ./syntax -run TestFormat`, `go test ./internal/lsp`,
  `go run ./cmd/eye test testdata/programs/core` (the language's own tests),
  `cd tree-sitter-eyelang && tree-sitter test -i 'Waiting'` (corpus tests by name),
  `go run ./cmd/eye lsp --definition|--references FILE:LINE:COL` (ask the language server).
  Between steps, `task quick` checks the core in seconds; `task check` is for before pushing.
- **Edit code with the editor's edit tool, not with replace scripts.** A Python `s.replace` or a
  `sed` changes the first match, even when it is not the one you meant, and says nothing; the edit
  tool refuses a match that is not unique. One wrong match here caused an infinite recursion.
- **Check the environment before a long run, not after it fails:** `gh auth status` for the
  token's scopes (pushing `.github/workflows` needs `workflow`), and which `eye` is on PATH: direnv
  adds the main checkout's `scripts/`, so in a worktree call `./bin/eye` or `go run ./cmd/eye`
  instead. Run commands from the repository root or with absolute paths; a probe run from the
  wrong directory looks like a bug in your code. Never leave a command waiting on stdin, such as a
  bare `cat`: it blocks the shell until the timeout.
- **Run `task check` before pushing.** It runs everything CI runs: generated files, build, vet,
  lint, tests, formatting, the conformance suite, the grammar's corpus and the VS Code extension's
  tests. On macOS it skips the extension's end-to-end test, which opens a VS Code window; run
  `task vscode:e2e`, or set `VSCODE_E2E=1`, when a change touches the extension or `eye lsp`.
- **Keep dependencies current.** Go itself (`go.mod`'s `go` line, on a supported release),
  modules (`go list -m -u all`, then `go get -u ./... && go mod tidy`), the npm packages of
  `editors/vscode` and `docs` (`npm outdated`), and the versions `.github/workflows` and
  `internal/pgtest` pin (actions, golangci-lint, Task, tree-sitter-cli, Node, Postgres), and
  zig, whose version and checksums `internal/zig` pins (C is built only with `eye build`,
  `scripts/zig` or `task zig -- …`, never a zig from PATH). Update
  them in a commit of their own, regenerate, and run `task check` with the extension's
  end-to-end test; a major version's changes are read before it goes in.
- **One task, one change.** Commits and pull requests say `Closes #N` for the ROADMAP task they
  finish. The `Close issues` workflow closes it when the commit reaches `dev`, which isn't the
  default branch; check that it did, and close by hand, naming the commit, an issue it missed.

## Writing

Specs, docs, comments and error messages use plain, concrete English: short sentences, active
voice, an example where it helps, no hype. Say what a thing does and when to use it. Error messages
say what was expected, what was found, and where.

## Map

Everything in the repository is Eyelang 2 or its tooling. Eyelang 1 lives only at the tag
`eyelang-1`; `repl/` is the one piece carried over.

| Path | What it is |
|---|---|
| `SPECS.md` | The language: promises and principles (section 1), decisions, rules, grammar sketch, open questions |
| `ROADMAP.md` | Phases, versions and tasks, each linked to an issue |
| `tree-sitter-eyelang/` | The grammar (the definition of the syntax), its scanner, corpus and queries |
| `syntax/` | Lexer, lossless parser, typed views, formatter; `syntax/gen` generates the views |
| `testdata/syntax/` | Conformance suite; `spec-*.eye` are generated from SPECS.md |
| `testdata/fmt/` | Formatter input and expected output |
| `cmd/eye/` | The `eye` command |
| `internal/lsp/` | The language server behind `eye lsp` |
| `internal/specdoc/`, `tools/specexamples/` | Extract SPECS.md's examples into `testdata/syntax` |
| `tools/next/` | `task next` and `task claim`: what's next, and who works on what |
| `internal/zig/`, `tools/zig/` | The pinned zig: `eye build`, `scripts/zig` and `task zig -- …` fetch it once, check its checksum and run it |
| `tools/spechtml/`, `internal/highlight/` | Render SPECS.md as one HTML page (`task spec`), highlighted by the parser |
| `internal/apidoc/` | The API reference `eye doc` writes from `##` comments |
| `internal/wire/` | How a contract's values and operations go over HTTP: the rules the interpreter and the generators share |
| `internal/codegen/` | `eye gen go` and `eye gen ts`: types, clients and server stubs from contracts; golden files in `testdata/` |
| `docs/` | The docs site (VitePress): guides and tutorial written by hand, reference pages generated; `task docs` builds it, `task docs-dev` serves it |
| `tools/specpages/` | SPECS.md as the site's language reference (`docs/reference/language`) and design pages (`docs/design`) |
| `tools/clidoc/` | `eye help` as the site's command reference, `docs/reference/cli.md` |
| `editors/vscode/` | The VS Code extension: TextMate grammar and the `eye lsp` client |
| `check/` | Loads a program's modules, resolves names, checks types, matches and effects, reports errors; `prelude.eye` and `std/` are the standard library written in Eyelang, and `runtime/store.eye` the in-memory store the runtime keeps every program's state in |
| `ir/` | The resolved form programs are lowered to |
| `lir/` | The low IR C is printed from: monomorphic instances in blocks of named values (`eye check --lir`) |
| `cgen/` | The C back end behind `eye build`: C11 from the low IR, compiled with the pinned zig; `runtime/` is the core runtime in C, and `testdata/` programs whose native tests must match `eye test` |
| `interp/` | Reference interpreter and runtime: entities, views, jobs, events, schedules, clock, fakes; the journal store and the runner behind `eye run` and `eye runs` |
| `host/` | The embedding API: a Go program loads an Eyelang program, registers its `@host` functions and runs it or its tests; `host/clib` gives the interpreter the C loader for `@library` (on Linux it makes a binary dynamic, so `eye` has it only with `-tags clib`) |
| `internal/ffi/` | Loads C functions from shared libraries at run time without cgo, on purego: `@library` in the interpreter, and later compiled programs in the Go runner |
| `testdata/programs/` | Programs whose tests `go test ./interp` runs; `tour/tour.eye` is generated from SPECS.md |
| `testdata/lab/` | The homelab's real workloads in Eyelang (digest, Notify, train reminders, NetBird access): the programs that decide what the language needs |
| `repl/` | Line editor with history, for `eye repl` |

Commands: `task check` (everything), `task generate` (after editing the grammar or a spec
example), `task quick` (build, test check, interp and syntax, then check and test
`testdata/lab`), `task conformance`, `task grammar`, `task vscode` (or one step of it:
`task vscode:unit|e2e|package`),
`go run ./cmd/eye parse|check|fmt FILE`, `go run ./cmd/eye test|run|runs DIR` (`eye run` keeps
its state in `DIR/.eye/journal`; `--memory` keeps nothing), `go run ./cmd/eye build [--test]
[--target os/arch] DIR` (a native binary; `--emit-c dir` writes the C instead).
