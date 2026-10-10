# eyelang

Eyelang 2 is a statically typed, compiled language for programs that manage data across many
entities and run jobs that mostly wait. It is being designed and built now:

- [SPECS.md](SPECS.md) is the language design (draft). `task spec` renders it as one HTML page,
  `bin/spec.html`, with its examples highlighted by Eyelang's own parser.
- [ROADMAP.md](ROADMAP.md) lists the phases, versions and exit criteria. Progress is tracked in
  [GitHub issues](https://github.com/ruxxzebre/eyelang/issues).

The tour from the spec runs: entities, a view, a job that sleeps for 20 minutes on a test clock,
and a call to another service replayed from a recording.

```sh
go run ./cmd/eye test testdata/programs/tour
```

The interpreter resolves names but doesn't check types yet; that is phase 2.

| Command | What it does |
|---|---|
| `eye run [dir] [-- args]` | runs a program's `main` |
| `eye test [--run text] [dir]` | runs a program's `test` blocks, each with a fresh in-memory store and its own clock |
| `eye check [dir]` | reports errors in a program as `file:line:column: message`; `--syntax` checks only the syntax of any files |
| `eye fmt [--check] [paths]` | formats files in the one canonical style |
| `eye doc [--out dir] [dir]` | writes the API reference as Markdown, from `##` comments: the standard library, and a program's modules |
| `eye parse [--sexp] files` | prints syntax trees |
| `eye lsp` | the language server editors run |

A program is a directory of `.eye` files; `import lab.notify` names the directory `lab/notify` under
it.

## Editors

VS Code: `task vscode` builds `bin/eyelang.vsix`; install it with
`code --install-extension bin/eyelang.vsix` and put `bin/eye` on your `PATH`. You get highlighting,
errors as you type, formatting on save, an outline and folding. See
[editors/vscode](editors/vscode/README.md). Editors that use tree-sitter (Neovim, Helix, Zed) can
use the grammar and queries in [tree-sitter-eyelang](tree-sitter-eyelang/README.md), and any
editor with an LSP client can run `eye lsp`.

## Building

You need Go 1.25 or later. With [direnv](https://direnv.net), run `direnv allow` once in the
repository; `eye` is then on your PATH whenever you are in it, rebuilt from the sources whenever they
change (`.envrc` adds `scripts/` to PATH):

```sh
direnv allow
eye help
eye test testdata/programs/tour
```

Without direnv, `go run ./cmd/eye help` does the same, or build a binary with
`go build -o bin/eye ./cmd/eye`.

With [Task](https://taskfile.dev): `task build` writes `bin/eye`, and `task test` runs the tests.
`task check` runs everything CI runs; besides Go and golangci-lint it needs
[tree-sitter-cli](https://www.npmjs.com/package/tree-sitter-cli) 0.27 and a C compiler, for the
grammar's tests and for the conformance suite, which parses every file in `testdata/syntax` with
both the tree-sitter grammar and the Go parser and fails if their trees differ, and Node for the
VS Code extension's tests. After changing `grammar.js` or an example in SPECS.md, run
`task generate`.

Prebuilt binaries for Linux, macOS and Windows are attached to each
[GitHub release](https://github.com/ruxxzebre/eyelang/releases), starting with v0.1.0. Pushing a
`v*` tag builds and publishes them; nothing built is committed to git.

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
| `check/` | Loads a program's modules, resolves names, reports errors; types and effects come in phase 2 |
| `ir/` | The resolved form programs are lowered to; types will be added to it |
| `interp/` | The reference interpreter and in-memory runtime: entities, views, jobs, clock, fakes |
| `repl/` | Line editor with history, reused by `eye repl` |

## Eyelang 1

The previous implementation, a Monkey-style tree-walking interpreter, is preserved at the git tag
[`eyelang-1`](https://github.com/ruxxzebre/eyelang/tree/eyelang-1). To run it:

```sh
git checkout eyelang-1
go run . examples/fib.eye
```
