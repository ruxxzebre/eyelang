---
title: Quickstart
---

# Quickstart

This page installs `eye`, then builds a small program that waits three days, sends one email,
and is tested in a couple of milliseconds. It takes about five minutes.

::: tip Nothing to install
The [playground](/playground#example=quickstart/remind) runs this page's program in your browser:
check, run, test and format it there, with the same errors, hover and completion that `eye lsp`
gives an editor.
:::

## Install

Build `eye` from source. You need [Go](https://go.dev/dl) 1.27 or later.

```sh
git clone --branch dev https://github.com/ruxxzebre/eyelang
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

The [release binaries](https://github.com/ruxxzebre/eyelang/releases) are far behind `dev`, and
this page needs `dev`.

## A first program

The program sends an invoice, then reminds the customer three days later, unless they have paid
by then. Get it from the repository you just cloned:

```sh
cp -R docs/quickstart/code/remind ~/remind
cd ~/remind
```

A program is a directory of `.eye` files. The file `eye.toml` marks its root, so that `import`
can find the directories below it.

### The invoice

<<< ./quickstart/code/remind/main.eye#invoice

An **entity** is a piece of state with one owner. Each invoice, `Invoice("inv-1")`, has its own
`email` and `paid`, and they change only through its methods. Each method call is one
transaction.

`issue` also starts the reminder. `remind.start(…)` is part of the same transaction, so an invoice
can't be issued without its reminder. The **key**, `"remind-{id}"`, names the run: starting it
again with the same key starts nothing, so an invoice issued twice is still reminded once.
`uses {start remind}` says that `issue` starts that job. Public functions and methods list their
effects this way, so a new one shows in review, and `eye fmt` writes the list for you.

### The job that waits

<<< ./quickstart/code/remind/main.eye#job

A **job** is a function that can wait for as long as it needs. A waiting run is kept on disk, not
in memory, and it carries on after a restart.

Every call marked with `!` is a **durable step**. When a step finishes, its result is saved with
the run. When the run resumes, it doesn't do the step again; it carries on with the saved value.

- `sleep!(3 days)` pauses the run for three days.
- `Invoice(id).read!()` reads a snapshot of the invoice, as it is three days later.
- `mail.send!(…)` calls the mail service. Its key stops a second email if the step runs again.
  If the reply is lost, `.reconcile!()` asks the service whether the email went out, instead of
  sending it again; only if it never arrived is it sent again, with the same key. `text:` alone
  is short for `text: text`.
- `match` must handle every outcome: `Sent` or `Bounced`.

The mail service is described by a **contract**, in `mail/mail.eye`. The contract types every
call: its arguments, its outcomes, and whether it needs a key.

<<< ./quickstart/code/remind/mail/mail.eye

### The test

Three days are too long to wait for a test. The tests set the clock and move it forward:

<<< ./quickstart/code/remind/main.eye#test

- `with clock(…)` fixes the time; `advance(3 days)` moves it, and every job that was waiting for
  that time runs.
- `with sent` answers calls to the mail service from `recordings/mail.json`, a recorded reply,
  instead of the network. `sent.calls(mail.send)` lists the emails the program sent.

## Test it

`eye test` runs every `test` block in the program.

```sh
eye test
```

```text
ok    a paid invoice gets no reminder (1.556ms)
ok    an unpaid invoice gets one reminder, three days later (162µs)
2 tests passed (1.717ms)
```

Three days of waiting ran in under two milliseconds. The tests are deterministic: the clock, the
store and the replies are the test's own. Use `eye test --run unpaid` to run only the tests whose
names contain `unpaid`.

## Check it

`eye check` finds errors in names, types, matches and effects without running anything. It prints
nothing when the program is fine.

Now make a mistake that a durable job usually ships with. In `remind`, keep the invoice itself
instead of a snapshot, by changing `Invoice(id).read!()` to `Invoice(id)`:

```sh
eye check
```

```text
main.eye:26:6: in a job, invoice.paid reads Invoice without recording it, so a resumed run could see another value: read a snapshot with invoice.read!() and take paid from it
main.eye:28:24: in a job, invoice.email reads Invoice without recording it, so a resumed run could see another value: read a snapshot with invoice.read!() and take email from it
```

Without the step, a run that resumed after a restart could read `paid` again, see a different
value, and take a different path than it did before. Temporal finds this kind of mistake when it
replays a run; Eyelang refuses to compile it. Leaving the `!` off `sleep!` is caught the same way:

```text
main.eye:24:3: in a job, sleep is a durable step: write sleep!(…)
```

Each error starts with `file:line:column`, so editors and terminals can jump to it. Undo the change
before you go on.

## Run it

`eye run` runs `main` with the arguments after `--`, then stays up to run the jobs. Here `main`
issues one invoice, which starts one reminder. Stop it with Ctrl-C.

```sh
eye run -- inv-1 ann@example.com
```

```text
eye run: started .; store: journal .eye/journal; jobs: remind
```

Run the same command again and stop it again. Then ask what is waiting with `eye runs`:

```sh
eye runs
```

```text
.eye/journal: 423 B on disk, 420 B live, 14.9 GiB free; 1 runs, 1 not finished
#1 remind("inv-1")  sleeping until 2026-10-16 00:16:38 (in 71h59m57s)  [1 step, 243 B]
```

The run survived both stops, and the second start began no second run, because of its key. When
three days have passed, the next `eye run` wakes the run at once and calls the mail service. This
contract doesn't say how to reach a real one, so outside the tests that call fails;
[Talking to other services](/guide/contracts#over-http) shows how to point a contract at a
service with `@http`. In the [playground](/playground#example=quickstart/remind), Run skips the
three days instead of waiting for them: the reminder wakes at once, and the mail service, which
nothing there serves, answers from `recordings/mail.json`, as in the tests.

`eye run` keeps all of this in `.eye/` in the program's directory: the journal, plus a
`journal.reserve` file of 4 MiB kept free so the journal can still be compacted when the disk is
full. Add `.eye/` to your `.gitignore`, or pass `--memory` to keep nothing.
[Running in production](/guide/running) says more.

## Format it

`eye fmt` rewrites files in the one canonical style. It has no options.

```sh
eye fmt --check .   # list the files that need formatting; exit 1 if any do
eye fmt .           # format them in place
```

## Set up your editor

**VS Code.** In the Eyelang repository, `task vscode` (it needs [Task](https://taskfile.dev) and
Node) builds the extension as `bin/eyelang.vsix`. Install it with
`code --install-extension bin/eyelang.vsix`. It finds `eye` on your `PATH` and gives you errors
as you type, formatting on save, hover and go to definition.

**Other editors.** `eye lsp` is a language server over standard input and output; point any LSP
client at it for `.eye` files. Editors that use tree-sitter (Neovim, Helix, Zed) can use
[tree-sitter-eyelang](https://github.com/ruxxzebre/eyelang/tree/dev/tree-sitter-eyelang).

## Next

- [The tutorial](/tutorial/) starts from hello world and builds up to entities and jobs, in five
  short pages, then puts it all together in an approvals program.
- [The guide](/guide/) covers one topic per page: entities, jobs and waiting, testing, contracts,
  running in production.
- [Examples](/examples/): trial reminders, webhook delivery, an uptime monitor, a daily report,
  refund approvals and more, each a complete program you can open in the playground. The
  [lab's workloads](https://github.com/ruxxzebre/eyelang/tree/dev/testdata/lab) are real programs
  that run every day.
- [The language reference](/reference/language/) states the full rules, and
  [the eye command](/reference/cli) lists every command and flag.
