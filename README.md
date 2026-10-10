# eyelang

Eyelang 2 is a statically typed, compiled language for programs that manage data across many
entities and run jobs that mostly wait. It is being designed and built now:

- [SPECS.md](SPECS.md) is the language design (draft).
- [ROADMAP.md](ROADMAP.md) lists the phases, versions and exit criteria. Progress is tracked in
  [GitHub issues](https://github.com/ruxxzebre/eyelang/issues).

Nothing in the spec runs yet. `eye parse` prints the syntax tree of a file and reports syntax
errors; the other subcommands arrive phase by phase.

## Building

```sh
go build -o eye ./cmd/eye
./eye help
```

With [Task](https://taskfile.dev): `task build` writes `bin/eye`, and `task test` runs the tests.
`task check` runs everything CI runs; besides Go and golangci-lint it needs
[tree-sitter-cli](https://www.npmjs.com/package/tree-sitter-cli) 0.27 and a C compiler, for the
grammar's tests and for the conformance suite, which parses every file in `testdata/syntax` with
both the tree-sitter grammar and the Go parser and fails if their trees differ. After changing
`grammar.js` or an example in SPECS.md, run `task generate`.

Prebuilt binaries for Linux, macOS and Windows are attached to each
[GitHub release](https://github.com/ruxxzebre/eyelang/releases), starting with v0.1.0. Pushing a
`v*` tag builds and publishes them; nothing built is committed to git.

## Repository layout

| Path | Contents |
|---|---|
| `cmd/eye/` | The `eye` command |
| `tree-sitter-eyelang/` | The tree-sitter grammar: the definition of the syntax, and editor highlighting |
| `syntax/` | Lexer, lossless parser, syntax tree and formatter (phase 1) |
| `testdata/syntax/` | The conformance suite: files that both parsers must parse identically, or both reject |
| `check/` | Name resolution, type and effect checking, diagnostics (phase 2) |
| `ir/` | Typed intermediate representation (phase 3) |
| `interp/` | Reference interpreter and in-memory runtime (phases 3 and 4) |
| `repl/` | Line editor with history, reused by `eye repl` |

## Eyelang 1

The previous implementation, a Monkey-style tree-walking interpreter, is preserved at the git tag
[`eyelang-1`](https://github.com/ruxxzebre/eyelang/tree/eyelang-1). To run it:

```sh
git checkout eyelang-1
go run . examples/fib.eye
```
