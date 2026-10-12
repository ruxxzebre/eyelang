# eyelang

Eyelang lets you write a program that runs for days (waits for people, keeps state, calls services
that fail) as plainly as a script, and the compiler catches the mistakes such programs usually ship
with.

**A typed language for programs that outlive their process.** Jobs wait for days, survive restarts
and call other services once. No queue, no cron, no retry library, and tests that run days in
milliseconds.

- **Waiting is one line.** A *job* is a function that can pause for as long as it needs. In a job,
  `sleep!(3 days)` waits three days and `next!(notify.answered, within: 6 h)` waits up to six
  hours for an answer; on one, `@on(daily(07:40, in: "Europe/Dublin"))` runs it every morning.
  They replace the queue, the cron and the `next_run_at` column. A waiting run is a record on
  disk, not a thread, and it survives restarts.
- **Replays can't go wrong.** Every *step*, a call whose result the job records so that a restarted
  run doesn't make it again, is marked `!`. A job that reads stored state outside a step, or keeps
  something that can't be saved across one, doesn't compile.
- **State has one owner.** Data that outlives a call lives in an *entity*. Only its own methods
  change it, one transaction at a time, with no locks.
- **Outside calls are typed and settle once.** A *contract* types every call to another service.
  When a reply is lost the result is `Uncertain`, and where the contract declares `@reconcile`,
  `.reconcile!()` asks the service what happened instead of sending the call again.
- **Days of behaviour test in milliseconds.** `advance(3 days)` moves a test's clock, and
  `with fakes(…)` replays recorded replies. The approvals tutorial's tests run in about 5 ms.

[Why Eyelang](docs/why.md) makes the case; the [quickstart](docs/quickstart.md) installs `eye` and
runs a first program. The promises in full, and the principles behind them, are in
[SPECS.md section 1](SPECS.md#1-goals).

Eyelang 2 is being designed and built now. `eye build` compiles the core language to a native
binary; programs that use jobs, contracts or the rest of the durable part of the language run in
the interpreter, `eye run`, until the runner hosts compiled programs. Where to look:

- [SPECS.md](SPECS.md) is the language design (draft). `task spec` renders it as one HTML page,
  `bin/spec.html`, with its examples highlighted by Eyelang's own parser.
- [docs/](docs) is the documentation site: a quickstart, a tutorial, guides, and references for the
  language, the standard library and the `eye` command. See [Documentation](#documentation).
- [ROADMAP.md](ROADMAP.md) lists the phases, versions and exit criteria. Progress is tracked in
  [GitHub issues](https://github.com/ruxxzebre/eyelang/issues).

The tour from the spec runs: entities, a view, a job that sleeps for 20 minutes on a test clock,
and a call to another service replayed from a recording.

```sh
go run ./cmd/eye test testdata/programs/tour
```

`eye check` checks names, types, traits, exhaustive matches, effects, `mut` without aliasing, and
attributes, each error at its exact line and column.

| Command | What it does |
|---|---|
| `eye run [dir] [-- args]` | runs a program's `main`, then keeps its jobs going: sleeping runs wake on time, schedules fire, and everything is kept in `.eye/journal` across restarts |
| `eye runs [dir] [id] [--json]` | lists a program's runs from its journal: status, what each waits for, its steps, and the journal's size |
| `eye approve [--id ID] [dir] [text]`, `eye decline …` | answers the `approval!` a run of `eye run` waits for, found by its text or id; the running program picks the answer up within a second |
| `eye repl [dir]` | an interactive session: expressions print their value and type, declarations join the session, and jobs run on a clock `:advance` moves |
| `eye test [--run text] [dir]` | runs a program's `test` blocks, each with a fresh in-memory store and its own clock |
| `eye check [dir]` | reports errors in a program as `file:line:column: message`; `--syntax` checks only the syntax of any files |
| `eye build [--test] [--target os/arch] [dir]` | compiles a program to a native binary through C, with a pinned zig fetched on first use; `--test` builds its tests. It compiles the core language; entities, jobs, views and contracts still run in `eye run` |
| `eye gen policy\|go\|ts [dir]` | writes build output from a program's contracts: a Kubernetes network policy, or Go and TypeScript types, clients and server stubs |
| `eye fmt [--check] [paths]` | formats files in the one canonical style |
| `eye fix [--check] [dir]` | rewrites code the language changed under: adds the `!` a durable step in a job now needs |
| `eye doc [--out dir] [dir]` | writes the API reference as Markdown, from `##` comments: the standard library, and a program's modules |
| `eye parse [--sexp] files` | prints syntax trees |
| `eye lsp` | the language server editors run |

A program is a directory of `.eye` files; `import lab.notify` names the directory `lab/notify` under
it.

## Documentation

The site in [docs/](docs) is Markdown, built with [VitePress](https://vitepress.dev). `task docs-dev`
serves it at http://localhost:5173 and reloads pages as you edit them; `task docs` builds it into
`docs/.vitepress/dist`. Both need Node.

Part of it is generated, and `task generate` rewrites it: the language reference and design pages
from SPECS.md, the API reference from the standard library's `##` comments (`eye doc`), and the
`eye` command's page from its help. Edit the source, not the page. The tutorial's code lives in real
programs next to the pages (`docs/**/code/*`), which `go test ./interp` runs, and every `eye` block
written into a page must parse, so the docs fail CI when the language moves under them.

## Editors

VS Code: `task vscode` builds `bin/eyelang.vsix`; install it with
`code --install-extension bin/eyelang.vsix` and put `bin/eye` on your `PATH`. You get highlighting,
errors as you type, formatting on save, an outline and folding. See
[editors/vscode](editors/vscode/README.md). Editors that use tree-sitter (Neovim, Helix, Zed) can
use the grammar and queries in [tree-sitter-eyelang](tree-sitter-eyelang/README.md), and any
editor with an LSP client can run `eye lsp`.

## Building

You need Go 1.27 or later (a Go from 1.21 on downloads it by itself, as `go.mod` asks). With
[direnv](https://direnv.net), run `direnv allow` once in the repository; `eye` is then on your PATH
whenever you are in it, rebuilt from the sources whenever they change (`.envrc` adds `scripts/` to
PATH):

```sh
direnv allow
eye help
eye test testdata/programs/tour
```

Without direnv, `go run ./cmd/eye help` does the same, or build a binary with
`go build -o bin/eye ./cmd/eye`.

With [Task](https://taskfile.dev): `task build` writes `bin/eye`, and `task test` runs the tests.
`task check` runs everything CI runs; besides Go and golangci-lint it needs a C compiler, for the
conformance suite, which compiles the tree-sitter grammar into the test, parses every file in
`testdata/syntax` with both it and the Go parser, and fails if their trees differ;
[tree-sitter-cli](https://www.npmjs.com/package/tree-sitter-cli) 0.27, for the grammar's own tests;
and Node for the VS Code extension's tests. The `eye` binary itself needs no C compiler; `eye build` fetches the zig it pins
and checks its checksum. After changing `grammar.js` or an example in SPECS.md, run
`task generate`.

Prebuilt binaries for Linux, macOS and Windows are attached to each
[GitHub release](https://github.com/ruxxzebre/eyelang/releases), starting with v0.1.0. Pushing a
`v*` tag builds and publishes them; nothing built is committed to git. The latest release, v0.2.0,
lags far behind `dev`: it lacks much of what this README and the docs describe, such as
`eye build`, so build from source.

## Repository layout

| Path | Contents |
|---|---|
| `cmd/eye/` | The `eye` command |
| `internal/lsp/` | The language server behind `eye lsp` |
| `editors/vscode/` | The VS Code extension: TextMate grammar and a client for `eye lsp` |
| `tree-sitter-eyelang/` | The tree-sitter grammar: the definition of the syntax, and editor highlighting |
| `syntax/` | Lexer, lossless parser, syntax tree and formatter (phase 1) |
| `testdata/syntax/` | The conformance suite: files that both parsers must parse identically, or both reject |
| `testdata/programs/` | Programs whose tests `go test ./interp` runs: the tour, and the language tested in itself |
| `check/` | Loads a program's modules, resolves names, checks types, matches and effects, reports errors |
| `ir/` | The resolved, typed form programs are lowered to |
| `lir/` | The low IR the C is printed from: one copy of each generic function per type it is used at (`eye check --lir`) |
| `cgen/` | The C back end behind `eye build`, and the core runtime in C |
| `interp/` | The reference interpreter and runtime: entities, views, jobs, events, schedules, clock, fakes, and the journal store behind `eye run` |
| `host/` | The embedding API: a Go program loads an Eyelang program, gives it its `@host` functions, and runs it or its tests |
| `internal/wire/` | How a contract's values and operations go over HTTP, shared by the interpreter and the generators |
| `internal/codegen/` | `eye gen go` and `eye gen ts`: types, clients and server stubs from contracts |
| `internal/ffi/` | Loads C functions from shared libraries at run time, without cgo, for `@library` |
| `testdata/lab/` | The homelab's real workloads in Eyelang (digest, Notify, train reminders): the programs that decide what the language needs |
| `repl/` | Line editor with history, reused by `eye repl` |
| `docs/` | The documentation site (VitePress); `tools/specpages` and `tools/clidoc` generate parts of it |

## Eyelang 1

The previous implementation, a Monkey-style tree-walking interpreter, is preserved at the git tag
[`eyelang-1`](https://github.com/ruxxzebre/eyelang/tree/eyelang-1). To run it:

```sh
git checkout eyelang-1
go run . examples/fib.eye
```
