# eyelang

# REPL

```sh
> go run .
Hey username! Welcome to Eye Language REPL
>>> 2 + 3
```

# Evaluate script

```sh
> go run . "./example.eye"
15
```

# Commands

```sh
# run all tests
> go test ./...
```

# TODOs

- keyword arguments/lists (?)
- Option<type> instead on NULL
- Type system
- Makefile or smth
- Decide on approach to environments (global, local, etc, pure functions) (scopes)
- Comments (POUND operator)
- PIPE operator
- Error handling
- LSP (Language Server Protocol), syntax highlighting, autocompletion, etc
- REPL history
- REPL multiline
- REPL autocompletion
- REPL syntax highlighting
