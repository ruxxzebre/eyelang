# eyelang

> **Eyelang 2 is being designed.** The language is described in [SPECS.md](SPECS.md) and the plan in
> [ROADMAP.md](ROADMAP.md), with progress tracked in GitHub issues. Everything below describes
> Eyelang 1, the current interpreter.

# REPL

Binaries are available for Windows, Mac, and Linux.
```sh
> ./bin/eyelang_windows_amd64
> ./bin/eyelang_darwin_amd64
> ./bin/eyelang_linux_arm64

Hey username! Welcome to Eye Language REPL
>>> 2 + 3
5
>> puts("Hello, World!")
"Hello, World!"
null
```

# Running a script

```sh
> eyelang "./example.eye"
15
```

Examples for syntax, data types and others are available in the `examples` directory.

# Pipe operator

`x |> f` calls `f` with `x` as its **last** argument, so it works together with partial application:

```
let sub = fn(a, b) { a - b }
2 |> sub(10)            # sub(10, 2) -> 8

let add = fn(a, b) { a + b }
let square = fn a: a * a
2 |> add(3) |> square   # square(add(3, 2)) -> 25
```

`|>` binds looser than arithmetic and tighter than comparison:
`"a" + "b" |> len` is `("a" + "b") |> len`, and `xs |> len > 3` is `(xs |> len) > 3`.

Shorthand function bodies extend as far right as possible, so wrap a lambda in parentheses to keep piping after it:
`5 |> (fn a: a + 1) |> square`.

# Commands

```sh
# run all tests
> go test ./...
```

# TODOs (from highest)

- MORE EXAMPLES
- ~~Support for Floats~~
- Debugging (line numbers, print ast, objects)
- Decide on approach for environments (global, local, etc, pure functions) (scopes)
- Figure out "symbols"
- Error handling
- keyword arguments (strings as keys)
- EBNF
- Option<type> instead on NULL (implies Void type)
- ~~PIPE operator~~
- REPL (history, multiline, completion)
- Tests for REPL
- Syntax highlighting for editors (nvim and vscode)
- LSP (Language Server Protocol), syntax highlighting, autocompletion, etc
- Type system
- Immutable Environments (memo if unchanged)
