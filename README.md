# eyelang

Eyelang 2 is a statically typed, compiled language for programs that manage data across many
entities and run jobs that mostly wait. It is being designed and built now:

- [SPECS.md](SPECS.md) is the language design (draft).
- [ROADMAP.md](ROADMAP.md) lists the phases, versions and exit criteria. Progress is tracked in
  [GitHub issues](https://github.com/ruxxzebre/eyelang/issues).

Nothing in the spec runs yet. The `eye` command exists as a skeleton, and its subcommands arrive
phase by phase.

## Building

```sh
go build -o eye ./cmd/eye
./eye help
```

With [Task](https://taskfile.dev): `task build` writes `bin/eye`, and `task test` runs the tests.

## Repository layout

| Path | Contents |
|---|---|
| `cmd/eye/` | The `eye` command |
| `syntax/` | Lexer, lossless parser, syntax tree and formatter (phase 1) |
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
