# eyelang

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

# Commands

```sh
# run all tests
> go test ./...
```

# TODOs (from highest)

- MORE EXAMPLES
- Decide on approach for environments (global, local, etc, pure functions) (scopes)
- Figure out "symbols"
- Error handling
- keyword arguments (strings as keys)
- EBNF
- Option<type> instead on NULL (implies Void type)
- PIPE operator
- REPL (history, multiline, completion)
- Tests for REPL
- Syntax highlighting for editors (nvim and vscode)
- LSP (Language Server Protocol), syntax highlighting, autocompletion, etc
- Type system
- Immutable Environments (memo if unchanged)
