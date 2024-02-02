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

# TODOs

- MORE EXPAMPLES
- keyword arguments/lists (?)
- Option<type> instead on NULL
- Type system
- Makefile or smth
- Decide on approach to environments (global, local, etc, pure functions) (scopes)
- ~~Comments (POUND operator)~~
- PIPE operator
- Error handling
- LSP (Language Server Protocol), syntax highlighting, autocompletion, etc
- REPL history
- REPL multiline
- REPL autocompletion
- REPL syntax highlighting
- Tests for REPL
