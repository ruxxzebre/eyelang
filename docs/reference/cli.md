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
| [`eye fix`](#fix) | rewrite code the language changed under, such as a durable step without its ! |
| [`eye doc`](#doc) | write the API reference as Markdown |
| [`eye gen`](#gen) | write build output from a program's contracts: network policy, Go and TypeScript |
| [`eye check`](#check) | report errors in a program: syntax, names, types and effects |
| [`eye run`](#run) | run a program: main, then its jobs, kept in a journal across restarts |
| [`eye runs`](#runs) | list a program's job runs: status, what each waits for, and steps |
| [`eye approve`](#approve) | approve what a run of eye run waits for at approval! |
| [`eye decline`](#decline) | decline what a run of eye run waits for at approval! |
| [`eye test`](#test) | run the test blocks of a program |
| [`eye build`](#build) | compile a program to a native binary, through C |
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
usage: eye fmt [--check] [--syntax-only] [file.eye | directory]...

Formats files in place; directories are searched for .eye files. With no
paths, formats standard input to standard output.
Recursive searches skip hidden directories and node_modules, unless named explicitly.

A directory's files are also checked as programs, and each public function's
uses clause is written from the effects its body has. A file named alone is
formatted for syntax only.
  -check
    	list files that aren't formatted and exit 1 if there are any, without changing them
  -syntax-only
    	format a directory's files without checking its program, leaving uses clauses as written
```

## eye fix {#fix}

```text
usage: eye fix [--check] [program-directory | file.eye]...

Rewrites a program's code where the language changed under it, then formats
the files it changed. Today it adds the ! that a durable step needs in a job or
a waiting function: publish!(…), start!(…), an entity call, a contract's
operation. It prints each change as file:line:column, and exits 1 if errors
it can't fix are left; eye check lists them. With no paths, fixes the program
in the current directory.
  -check
    	list what would change and exit 1 if anything would, without changing files
```

## eye doc {#doc}

```text
usage: eye doc [--out dir] [program-directory]

Writes the API reference as Markdown: the standard library and, given a program,
its modules. Each public declaration gets its signature and its ## comment.
A [Name] in a ## comment links to that declaration; eye doc exits 1 if one
names nothing.

The pages go to ./api in the current directory, or to the directory --out names.
eye doc replaces that directory only if it is missing, empty, or holds nothing
but pages eye doc wrote. If it holds anything else, eye doc stops, changes
nothing and exits 1: pass --out with another directory.

  -out string
    	the directory to write the Markdown pages to (default "api")
```

## eye gen {#gen}

```text
usage: eye gen <target> [flags] [program-directory]

Writes build output from a program's contracts to standard output. The program
must check; with no directory, it is the program in the current directory.

targets:
  policy   Kubernetes network policy: which module may reach which service on which port
  go       Go types, client and server stub for each contract
  ts       TypeScript types, client and server stub for each contract

usage: eye gen policy [--namespace name] [program-directory]

Writes a Kubernetes NetworkPolicy (networking.k8s.io/v1) for each contract with
@port: the pods of the module that serves it accept TCP on that port only from
the pods of the modules whose code calls its operations, among those its @allow
lets call it. A module's pods carry the label eyelang.module: <module>, its
import path; the program's own module goes by the name of the program's
directory. When nothing in the program serves a contract, the policy selects
the pods labelled with the contract's own module, such as eyelang.module: notify
for a Notify written in another language. Tests, events and contracts without
@port, which are services outside the network, get no rule; nor does egress.
Each policy is named after the program and the contract, such as lab-notify,
so the policies of programs that call one service add up. For example:

  eye gen policy --namespace homelab testdata/lab | kubectl apply -f -

  -namespace namespace
    	the Kubernetes namespace the policies go in, such as homelab; without it they have none, and kubectl apply -n decides

usage: eye gen go [--module name] [--out directory] [--package name] [program-directory]

Writes a Go package for a module's contracts: their types, which marshal to
Eyelang's one JSON shape; for each contract a client, NotifyClient for
Notify, that calls its operations over @http; and a server stub, the
NotifyService interface to implement and NewNotifyHandler, an http.Handler
that routes requests to it the way eye run serves the contract. A command's
lost reply is an *Uncertain error, and a @fresh command's outcomes are
ErrStale and ErrExpired.

Operations without @http are left out of the client and the stub. The output
is a build artifact, never committed. Without --out, one module's file goes
to standard output; with it, each module with contracts gets a file, such as
notify/notify.go. For example:

  eye gen go --module notify testdata/lab > notify.go

  -module module
    	the module whose contracts to write, by import path or name, such as notify; without it, the program's only module with contracts
  -out directory
    	a directory to write a file for each module with contracts into, instead of printing one
  -package name
    	the Go package's name; the module's name without it

usage: eye gen ts [--module name] [--out directory] [program-directory]

Writes a TypeScript module for a module's contracts: their types as the JSON
they are on the wire, a variant type as a union discriminated on "kind"; for
each contract a client class, NotifyClient for Notify, that calls its
operations over @http; and a server stub, the NotifyService interface to
implement and notifyHandler, a Fetch API handler from Request to Response
that routes requests to it the way eye run serves the contract. It compiles
with tsc --strict for ES2022 with the DOM library's fetch types.

Operations without @http are left out of the client and the stub. The output
is a build artifact, never committed. Without --out, one module's file goes
to standard output; with it, each module with contracts gets a file, such as
notify.ts. For example:

  eye gen ts --module notify testdata/lab > notify.ts

  -module module
    	the module whose contracts to write, by import path or name, such as notify; without it, the program's only module with contracts
  -out directory
    	a directory to write a file for each module with contracts into, instead of printing one
```

## eye check {#check}

```text
usage: eye check [--syntax | --ir] [program-directory | file.eye]...

Checks programs: a directory and the modules it imports, or the program a file
belongs to: the nearest directory above it with an eye.toml, else its own.
Reports errors as file:line:column: message and exits 1 if there are any.
With no paths, checks the program in the current directory. Names, imports,
types, exhaustive matches, effects and where waiting calls may appear are checked.
  -ir
    	print the typed IR of a program that checks: each function, and each expression with its type
  -lir
    	print the low IR of a program that checks: each function at each set of types it is used at, in blocks
  -syntax
    	check only syntax, of files and of directories searched for .eye files
```

## eye run {#run}

```text
usage: eye run [--store url | --journal file [--fresh] | --memory] [--crossings] [program-directory] [-- arguments…]

Runs main, then keeps the program's jobs going: sleeping runs wake on time,
schedules fire, and a restart carries on where the last run of eye stopped.
It stops when nothing is left to wait for, or on Ctrl-C or SIGTERM.

A program that serves a contract with impl answers it over HTTP on the
contract's @port until it is stopped. NOTIFY_PORT, named after the contract's
module, serves it on another port; 0 picks a free one, which eye run names.

State is kept in the program's journal, or with --store in a Postgres database,
in the first schema of its search_path: postgres://…/db?search_path=lab gives
a program its own schema. One eye run at a time writes a journal or a schema.
  -crossings
    	count main's, each job run's and the runner's operations that would cross between
    	compiled code and the runner, and print them by kind on stderr when eye run ends
  -fresh
    	start with an empty journal; the old one is kept as JOURNAL.old
  -journal string
    	the journal that keeps entities, runs and events (default program-directory/.eye/journal)
  -memory
    	keep everything in memory, so nothing survives the process
  -memory-limit string
    	the memory runs may hold, which @memory and reserve! wait for, such as 512MiB (default the cgroup's limit, such as a container's, or the host's memory)
  -store string
    	keep entities, runs and events in this Postgres database, postgres://… (default $EYE_STORE)
```

## eye runs {#runs}

```text
usage: eye runs [--store url | --journal file] [--json | --show-replies] [program-directory] [run-id [step]]

Lists the job runs in a program's journal, or its Postgres store: each run's
status, what it waits for and until when, and with a run's id, its steps, with
what each model call spent. With a step's number too, it shows that step's
whole result, such as a model's reply. It reads without changing anything, so
it works while eye run is running.
  -journal string
    	the journal to read (default program-directory/.eye/journal)
  -json
    	print the runs as JSON, with every value whole
  -show-replies
    	with a run's id, show each model call's whole reply below its step
  -store string
    	read the store in this Postgres database, postgres://… (default $EYE_STORE)
```

## eye approve {#approve}

```text
usage: eye approve [--id ID] [--by name] [--journal file] [program-directory] [text]

Answers, with approve, the one approval! a run of the program waits for whose
text contains text, or whose id is ID. When several contain text, it answers
none and lists them with their ids. The program's eye run picks the answer up
within a second, and publishes it as approvals.answered. eye runs shows what
each run waits for.
  -by string
    	who answers; only the person the approval asks may (default that person)
  -id string
    	the id of the approval to answer, as eye runs and an ambiguous eye approve show it
  -journal string
    	the journal to look for waiting approvals in (default program-directory/.eye/journal)
```

## eye decline {#decline}

```text
usage: eye decline [--id ID] [--by name] [--journal file] [program-directory] [text]

Answers, with decline, the one approval! a run of the program waits for whose
text contains text, or whose id is ID. When several contain text, it answers
none and lists them with their ids. The program's eye run picks the answer up
within a second, and publishes it as approvals.answered. eye runs shows what
each run waits for.
  -by string
    	who answers; only the person the approval asks may (default that person)
  -id string
    	the id of the approval to answer, as eye runs and an ambiguous eye decline show it
  -journal string
    	the journal to look for waiting approvals in (default program-directory/.eye/journal)
```

## eye test {#test}

```text
usage: eye test [--run text] [-v] [--live modules [--record]] [--crossings] [program-directory]

Runs the test blocks of a program's module, each with a fresh in-memory store
and a clock that moves only when the test advances it. Calls to other services
are answered from recordings, except for the modules --live names, whose calls
go where eye run sends them, with the address and secret from the environment.
  -crossings
    	count, for each test and each job run in it, the operations that would cross
    	between compiled code and the runner, and print them by kind at the end
  -live modules
    	call the real services of these modules, separated by commas (ai, http or contract modules),
    	instead of replaying their recordings; a test whose service has no address or secret set is skipped
  -record
    	with --live, write what the real services answered as the recordings
    	the fakes read, for the tests that passed
  -run string
    	run only the tests whose names contain this text
  -v	show what passing tests logged too
```

## eye build {#build}

```text
usage: eye build [--test] [--target os/arch] [-o file] [-g] [--emit-c dir] [program-directory]

Compiles a program to a native binary: C from the low IR, built with the zig
that eye pins, fetched into the user's cache the first time. Object files are
cached by the hash of what made them. The binary runs main, or with --test the
program's tests. The C back end compiles the core language; entities, jobs,
views and contracts still run in eye run.
  -emit-c string
    	write the C, the runtime's with it, into this directory instead of compiling it
  -g	compile without optimisation and with debug information (-O0 -g)
  -o string
    	the binary to write (default: the program's directory name, with .test for --test)
  -target string
    	the platform to build for: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64 (default: this machine)
  -test
    	build a binary that runs the program's test blocks, as eye test does
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
usage: eye lsp [--definition | --type-definition | --references | --implementation | --declaration | --hover FILE:LINE:COLUMN]

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
  -type-definition FILE:LINE:COLUMN
    	print where the type of what is at FILE:LINE:COLUMN is declared, and exit
```

## eye version {#version}

```text
usage: eye version
```

## eye help {#help}

```text
usage: eye help
```
