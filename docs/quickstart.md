---
title: Quickstart
---

# Quickstart

This page gets the `eye` command onto your machine, runs a first program, and sets up your
editor. It takes about five minutes.

## Install

Eyelang is built from source. You need [Go](https://go.dev/dl) 1.25 or later.

```sh
git clone https://github.com/ruxxzebre/eyelang
cd eyelang
go build -o bin/eye ./cmd/eye
```

This writes the `eye` binary to `bin/eye`. Put `bin/` on your `PATH`, or copy the binary somewhere
that already is. Check that it works:

```sh
eye version
```

```text
eye 0.0.0-dev
```

::: tip Working on Eyelang itself?
With [direnv](https://direnv.net), run `direnv allow` once in the repository. `eye` is then on
your `PATH` whenever you are inside it, and it is rebuilt from the sources whenever they change.
:::

## A first program

A program is a directory of `.eye` files. A file named `eye.toml` marks the program's root, so
`import` can find the directories below it. The file may be empty for now.

```sh
mkdir hello
cd hello
touch eye.toml
```

Then write `main.eye`:

<<< ./tutorial/code/hello/main.eye

- `fn greeting(...) -> String = ...` is a function whose body is one expression.
- `"Hello, {name}!"` puts the value of `name` into the string.
- `pub fn main` is where the program starts. It gets the command line in `args`; the first
  element is the program itself, so `drop(1)` skips it.
- `uses {stdio}` says that `main` writes to standard output. Public functions list their effects,
  so a new one always shows in review. See [effects and signatures](/reference/language/effects-and-signatures).
- The `test` block checks `greeting`. Tests live next to the code they test.

## Run it

`eye run` runs `main`. Arguments after `--` go to the program.

```sh
eye run
eye run -- Ann Bo
```

```text
Hello, world!
Hello, Ann!
Hello, Bo!
```

`eye run` keeps a journal in `.eye/` in the program's directory, so that jobs survive a restart
(you will meet jobs in [the tutorial](/tutorial/a-job-that-waits)). Add `.eye/` to your
`.gitignore`, or pass `--memory` to keep nothing.

## Test it

`eye test` runs every `test` block in the program.

```sh
eye test
```

```text
ok    greeting names who it greets (3µs)
1 tests passed (3µs)
```

Use `eye test --run greeting` to run only the tests whose names contain `greeting`, and `-v` to see
what passing tests logged.

## Check it

`eye check` reports errors in names, types, matches and effects, without running anything. It
prints nothing when the program is fine. Change `greeting("world")` to `greeting(42)` and run it:

```sh
eye check
```

```text
main.eye:8:20: expected String, found Int
```

Each error starts with `file:line:column`, so editors and terminals can jump to it.

## Format it

`eye fmt` rewrites files in the one canonical style. It has no options. It keeps the line breaks
you chose and fixes indentation and spacing.

```sh
eye fmt --check .   # list the files that need formatting; exit 1 if any do
eye fmt .           # format them in place
```

A line such as `fn  shout( s:String )->String=upper(s)+"!"` becomes:

```eye
fn shout(s: String) -> String = upper(s) + "!"
```

## Set up your editor

**VS Code.** In the Eyelang repository, `task vscode` (it needs [Task](https://taskfile.dev) and
Node) tests the extension and packages it as `bin/eyelang.vsix`. Install it with:

```sh
code --install-extension bin/eyelang.vsix
```

The extension finds `eye` on your `PATH` (or in `bin/eye` of an open workspace folder). You get
highlighting, errors as you type, formatting on save, an outline, folding and go to definition.

**Other editors.** `eye lsp` is a language server that talks over standard input and output.
Point any LSP client at the command `eye lsp` for `.eye` files. Editors that use tree-sitter
(Neovim, Helix, Zed) can also use the grammar and queries in
[tree-sitter-eyelang](https://github.com/ruxxzebre/eyelang/tree/main/tree-sitter-eyelang).

## Next

- [The tutorial](/tutorial/) builds up from values and functions to entities and jobs that wait,
  in five short pages.
- [The language reference](/reference/language/) states the full rules, one section per page.
- [The eye command](/reference/cli) lists every command and flag.
