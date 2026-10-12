---
title: Why Eyelang
---

# Why Eyelang

Some programs mostly wait. A reminder service waits for the time to send. An approval flow waits
days for a person to answer. An agent waits for a model, then for a tool, then for a human. A
scheduler waits for the next slot. While they wait, they hold state, and the systems they call fail
in all the usual ways.

In most languages you build such a program from parts: a web framework, a database table for
state, a queue or cron for the waiting, a retry library for the calls, idempotency keys by hand,
and a test suite that sleeps or mocks the clock. Each part is reasonable. Together they spread one
idea, "wait three days, then remind them unless they answered", across five places, and the bugs
live in the gaps between them.

Eyelang puts those parts in the language, so the program says what it means and the compiler checks
it.

## What the language does for you

**Waiting is a line of code.** `sleep!(3 days)` in a job pauses it for three days. While it waits,
the run is a record saved on disk, not a thread or a process, and memory keeps only a handle of
about 140 bytes for it. If
the process restarts, the run carries on from its last step. You don't need a queue, a cron table
or a "next run at" column. See [Jobs and waiting](/reference/language/jobs-and-waiting).

**Every pause is visible.** A call that can pause or that commits a durable step ends with `!`:
`sleep!`, `w.read!()`, `notify.send!(…)`. A reviewer can see from the diff alone where a job can
stop, and what has already happened if it does.

**State has one owner.** Data that outlives a call lives in an [entity](/reference/language/entities).
Only its own methods change it, one transaction at a time, so two requests can't interleave
half-finished updates. There are no locks to forget.

**Calls to other services happen once.** A [contract](/reference/language/contracts-talking-to-other-services)
types the calls to another service. Every command carries a key, so a retry after a crash doesn't
send the message twice. When the reply is lost, the result says so (`Uncertain`), and `.reconcile!()`
asks the service what really happened, instead of guessing.

**Failure is a value.** There are no exceptions. A call that can fail returns a `Result`, and a
`match` must handle every case, or the program doesn't compile. See
[Errors: Result only](/reference/language/errors-result-only).

**Effects show in signatures.** A public function lists what it touches: `uses {net}`,
`uses {fs}`. A pure function says nothing, and the compiler holds both to it. See
[Effects and signatures](/reference/language/effects-and-signatures).

**Tests can't flake.** Each test gets a fresh store and a clock that moves only when the test says
`advance(3 days)`. Calls to other services replay from recorded files. A test of a three-day flow
runs in milliseconds and gives the same answer every time. See
[Time, secrets, tests](/reference/language/time-secrets-tests).

**Models return types, not text.** `ai.generate!` asks a model for a value of an Eyelang type. The
type becomes a JSON Schema, its `##` comments become the descriptions, and the reply is checked and
decoded before your code sees it. See [AI models and agents](/reference/language/ai-models-and-agents).

## A small, strict core

Under those features is an ordinary general-purpose language: immutable values, records and
variants, exhaustive pattern matching, traits and generics, type inference inside function bodies,
pipes, and no null. Mutation is local and marked with `mut`, and values are never aliased, so
parallel code has no data races and memory is managed by reference counting with no garbage
collector pauses.

The rules are strict so that programs are easy to check, for people and for coding agents. There is
one obvious way to write each thing, one formatting, and public signatures that state their types
and effects. A change that compiles and passes its tests has been checked for the mistakes that
usually slip through review.

## When to use something else

Eyelang is young. A reference interpreter runs programs today; native compilation comes later, and
the language changes whenever real programs show a better way, until v1.0. It is a poor fit for GUI
applications, for numeric code that needs every cycle of the machine, and for programs that must
run where nothing but a C toolchain exists.

## See it work

- The [tutorial](/tutorial/) starts with small programs and ends with
  [a complete approval workflow](/tutorial/approvals) that uses all of the above.
- The [tour](/reference/language/a-tour) shows the language on one page.
- The [design](/design/) explains each decision and the questions that are still open.
