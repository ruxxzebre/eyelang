# eyelang

# REPL

Based on your OS, run the corresponding binary from the `bin` directory.
```sh
> ./bin/eyelang_windows_amd64
> ./bin/eyelang_darwin_amd64
> ./bin/eyelang_linux_arm64

Hey username! Welcome to Eye Language REPL
>>> 2 + 3
```

# Evaluate script

`eyelang` is the binary that evaluates the script.

```sh
> eyelang "./example.eye"
15
```

# Commands

```sh
# run all tests
> go test ./...
```

# TODOs (from highest)

- MORE EXAMPLES
- Decide on approach to environments (global, local, etc, pure functions) (scopes)
- Figure out "symbols"
- Error handling
- keyword arguments (strings as keys)
- EBNF
- Option<type> instead on NULL
- PIPE operator
- REPL (history, multiline, completion)
- Syntax highlighting for editors
- Tests for REPL
- Type system
- LSP (Language Server Protocol), syntax highlighting, autocompletion, etc
- Immutable Environments (memo if unchanged)
