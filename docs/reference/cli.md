---
title: The eye command
---

<!-- Generated from `eye help` and `eye <command> -h` by `task generate`. Don't edit. -->

# The eye command

`eye` is the one tool for Eyelang: it checks, formats, tests and runs programs, writes their API reference, and is the language server editors talk to. A program is a directory of `.eye` files; an `eye.toml` file marks its root.

| Command | What it does |
|---|---|
| [`eye parse`](#parse) | print the syntax tree of files |
| [`eye fmt`](#fmt) | format source files |
| [`eye doc`](#doc) | write the API reference as Markdown |
| [`eye check`](#check) | report errors in a program: syntax, names, types and effects |
| [`eye run`](#run) | run a program: main, then its jobs, kept in a journal across restarts |
| [`eye runs`](#runs) | list a program's job runs: status, what each waits for, and steps |
| [`eye test`](#test) | run the test blocks of a program |
| [`eye repl`](#repl) | start an interactive session: expressions, declarations and jobs |
| [`eye lsp`](#lsp) | run the language server for editors |
| [`eye version`](#version) | print the eye version |
| [`eye help`](#help) | show this message |

## eye parse {#parse}

```text
usage: eye parse [--sexp] file.eye...

Prints the syntax tree of each file and reports syntax errors.
  -sexp
    	print the normalised S-expression the conformance suite compares, one file per line
```

## eye fmt {#fmt}

```text
usage: eye fmt [--check] [file.eye | directory]...

Formats files in place; directories are searched for .eye files. With no
paths, formats standard input to standard output.
  -check
    	list files that aren't formatted and exit 1 if there are any, without changing them
```

## eye doc {#doc}

```text
usage: eye doc [--out dir] [program-directory]

Writes the API reference as Markdown: the standard library and, given a program,
its modules. Each public declaration gets its signature and its ## comment.
A [Name] in a ## comment links to that declaration; eye doc exits 1 if one
names nothing.
  -out string
    	the directory to write the Markdown pages to; it is replaced (default "api")
```

## eye check {#check}

```text
usage: eye check [--syntax] [program-directory | file.eye]...

Checks programs: a directory and the modules it imports, or the program a file
belongs to: the nearest directory above it with an eye.toml, else its own.
Reports errors as file:line:column: message and exits 1 if there are any.
With no paths, checks the program in the current directory. Names, imports,
types, exhaustive matches, effects and where waiting calls may appear are checked.
  -syntax
    	check only syntax, of files and of directories searched for .eye files
```

## eye run {#run}

```text
usage: eye run [--journal file | --memory] [program-directory] [-- arguments…]

Runs main, then keeps the program's jobs going: sleeping runs wake on time,
schedules fire, and a restart carries on where the last run of eye stopped.
It stops when nothing is left to wait for, or on Ctrl-C or SIGTERM.
  -journal string
    	the journal that keeps entities, runs and events (default program-directory/.eye/journal)
  -memory
    	keep everything in memory, so nothing survives the process
```

## eye runs {#runs}

```text
usage: eye runs [--journal file] [--json] [program-directory] [run-id]

Lists the job runs in a program's journal: each run's status, what it waits
for and until when, and with a run's id, its steps. It reads the journal
without changing it, so it works while eye run is running.
  -journal string
    	the journal to read (default program-directory/.eye/journal)
  -json
    	print the runs as JSON
```

## eye test {#test}

```text
usage: eye test [--run text] [-v] [program-directory]

Runs the test blocks of a program's module, each with a fresh in-memory store
and a clock that moves only when the test advances it.
  -run string
    	run only the tests whose names contain this text
  -v	show what passing tests logged too
```

## eye repl {#repl}

```text
usage: eye repl [program-directory]

Starts an interactive session: expressions print their value and type,
statements run, and declarations join the session. Given a program, the
session can use its modules. :help lists the commands.
```

## eye lsp {#lsp}

```text
usage: eye lsp [--definition | --references | --implementation | --declaration | --hover FILE:LINE:COLUMN]

Runs the language server, speaking LSP on standard input and output.
Editors start it; see editors/vscode. With one of the flags it answers one
request as an editor would get it and prints the locations as
path:line:column, or a hover's Markdown, which checks the server
without an editor.
  -declaration FILE:LINE:COLUMN
    	print the SPECS.md section of the keyword at FILE:LINE:COLUMN, and exit
  -definition FILE:LINE:COLUMN
    	print where the name or keyword at FILE:LINE:COLUMN is defined, and exit
  -hover FILE:LINE:COLUMN
    	print the signature and doc comment of the name at FILE:LINE:COLUMN, and exit
  -implementation FILE:LINE:COLUMN
    	print every place eye implements the keyword at FILE:LINE:COLUMN, and exit
  -references FILE:LINE:COLUMN
    	print every use of the name at FILE:LINE:COLUMN, and exit
```

## eye version {#version}

```text
usage: eye version
```

## eye help {#help}

```text
usage: eye help
```
