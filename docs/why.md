---
title: Why Eyelang
---

# Why Eyelang

Some programs mostly wait. A reminder service waits for the time to send. An approval flow waits
days for a person to answer. A scheduler waits for the next slot. While they wait, they hold
state, and the systems they call fail in all the usual ways.

In most languages you build such a program from parts: a web framework, a database table for
state, a queue or cron for the waiting, a retry library for the calls, idempotency keys by hand,
and a test suite that sleeps or mocks the clock. Each part is reasonable. Together they spread one
idea, "wait three days, then remind them unless they answered", across five places, and the bugs
live in the gaps between them.

Eyelang puts those parts in the language, so the program says what it means and the compiler checks
it.

## What the language does for you

**Waiting is one line.** A *job* is a function that runs in the background and can pause.
`sleep!(3 days)` in a job pauses it for three days. `next!(answered, within: 6 h)` waits for the
next `answered` event, or gives up after six hours. `@on(daily(07:40, in: dublin))` starts a job
every morning at 07:40 Dublin time, daylight saving included. While a run waits, it is a record on
disk, not a thread, and memory keeps a handle of about 140 bytes for it (with the journal or the
Postgres store). If the process restarts, the run carries on where it was. You don't write a
queue, a cron table or a `next_run_at` column. See
[Jobs and waiting](/reference/language/jobs-and-waiting).

**Replays can't go wrong.** A call marked `!` is a *step*: it runs once, and its result is saved
with the run. After a restart the run picks up from its last saved place and replays the steps
since, each returning its saved result instead of running again. That only works if the code
between steps does the same thing the second time, so the compiler checks it. In a job, reading an
entity's state outside a step (`w.phase` instead of `w.read!()`) is an error, and so is holding a
value that can't be saved, such as a `Secret` or a channel, across a `!`. Temporal finds the same
mistakes when a workflow replays; Eyelang reports them when you compile. A reviewer
also sees from the diff alone where a job can stop.

**State has one owner.** Data that outlives a call lives in an
[*entity*](/reference/language/entities): a named record, such as one `Request` or one
`Traveller`, kept in the program's store. Only its own methods change it, one transaction at a
time, so two requests can't interleave half-finished updates. There are no locks to take or forget.

**Outside calls are typed and settle once.** A
[*contract*](/reference/language/contracts-talking-to-other-services) describes another service's
operations and types, and every call to it is checked against them. A *key* is an idempotency key:
a contract can require one on a command (`@key(required)`), and a command sent twice with the same
key happens once. When a command's reply is lost, the result says so: it is `Uncertain`, and where
the contract declares `@reconcile`, `.reconcile!()` asks the service what really happened instead
of sending the command again. There is no retry library. The runtime sends again, for about a
minute, only what is safe to repeat, such as a request that never reached the service; an
operation that declares its errors gets them at once, and a policy of your own is a short waiting
function ([Jobs and waiting](/reference/language/jobs-and-waiting) shows `retry`). A model call is
typed the same way: `ai.generate!` asks for a value of an Eyelang type and checks the reply before your
code sees it. See [AI models and agents](/reference/language/ai-models-and-agents).

**Days of behaviour test in milliseconds.** Each test gets a fresh store, a fixed random seed and a
clock that moves only when the test says `advance(3 days)`. With `fakes(…)`, calls to other
services replay replies recorded from the real ones. A test of a three-day flow runs in
milliseconds and gives the same answer every time: the eight tests of the
[approvals tutorial](/tutorial/approvals) take about 5 ms together. See
[Time, secrets, tests](/reference/language/time-secrets-tests).

## A small, strict core

Under those features is an ordinary general-purpose language: records and variants, exhaustive
pattern matching, traits and generics, type inference inside function bodies, pipes, and no null.
There are no exceptions: a call that can fail returns a `Result`, and a `match` must handle every
case, or the program doesn't compile. Values are immutable and never shared; a change is local and
marked with `mut` where the call is made, so parallel code has no data races. A public function
lists its effects, such as `uses {net}`, and a pure function says nothing; the compiler holds both
to it.

The rules are strict so that programs are easy to check, for people and for coding agents. There is
one obvious way to write each thing, one formatting, and public signatures that state their types
and effects.

## Compared with what you use now

### Cron, a queue, Postgres and a retry library

The homelab's train reminders were a Go service of 10,750 lines, with a `train_questions` table, a
`train_reminders_sent` table, and a polling loop that ran on every tick. In Eyelang the same
reminder is one job:

```eye
job ask(person: UserId, r: Reminder) -> Result((), notify.Refusal) {
  let key = "remind-{person}-{r.pin.train}-{r.pin.date}-{leadName(r.lead)}"
  let asked = notify.ask!(
    event: "trains.reminder",
    to: person,
    text: reminderText(r),
    choices: [boarded, skip],
    within: 6 h,
    key:,
  ).reconcile!()
  let question = match asked {
    Asking(q) => q
    NotAsked(reason) => return Err(reason)
  }
  let answer = next!(notify.answered, where: fn a: a.question == question, within: 6 h)
  match answer {
    Some(a) if a.choice == boarded.id => Traveller(person).board!(r.pin)
    Some(_) => Traveller(person).unpin!(r.pin)
    None => ()
  }
  Ok(())
}
```

`next!(…, within: 6 h)` replaces both the polling and the expiry: the run waits for the answer,
and after six hours it gets `None`. The key stops a second reminder for the same train, and the
traveller entity replaces the tables. The whole program,
[trains.eye](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/trains/trains.eye), is
about 230 lines. That isn't a like-for-like count, since the Go service did more than the
reminders, but the tables and the polling loop are gone, not moved elsewhere.

The lab's morning digest tells the same story. Its earlier version, on Hatchet in TypeScript,
needed two cron entries and a daylight-saving check to fire at 07:40 Irish time, retries set per
task, and a `dedupe_key`. In Eyelang it is `@on(daily(07:40, in: dublin))`.

### Temporal, Restate, Inngest, DBOS

These run durable workflows too, and they do it well. The difference is that they are SDKs and
servers added to another language, and Eyelang is the language:

- **Replay rules are checked when you compile.** A workflow SDK finds out at run time, if at all,
  that your code read the clock or a database between steps; Temporal reports it when a replay
  goes another way. Eyelang's
  checker rejects such a job before it runs.
- **There is no separate server to run.** A program keeps its state, runs and schedule in its own
  store, a journal on disk or Postgres.
- **Keys and lost replies are in the types.** A contract says whether a command needs a key, and a
  lost reply is an `Uncertain` value the program must handle, not an exception to remember.

Where they are stronger: they are mature, with years of production use and many users. They scale
a workflow out across many workers and machines, offer hosted services, and have SDKs for the
languages your team already writes, with the libraries those bring. Eyelang has none of that yet.

Two questions to ask of any durable system, and Eyelang's answers today:

- **What happens to sleeping runs when I deploy new code?** If a job's code changed while runs of
  it are paused, `eye run` refuses to start, or the run faults when it replays the changed step.
  You keep the old job under its name until its runs finish, or end them. Temporal and Restate
  finish each run on the version it started on; whether Eyelang should do the same is
  [open question 41](/design/open-questions).
- **How does it scale out?** It doesn't yet. One `eye run` owns a store; Postgres holds a lock so a
  second runner can't share it. Several runners on one database is
  [open question 28](/design/open-questions).

## What runs today, and when to use something else

The reference interpreter runs nearly all of the language: `eye run` and `eye test` run entities,
jobs, views (live queries over entities), contracts and model calls. Coding agents are the biggest
gap. `eye build` compiles the core language to a native binary, with
reference counting and no garbage collector; programs with entities, jobs, views or contracts run in
the interpreter, which is written in Go, until native code covers them (v0.4). The language
changes whenever real programs show a better way, until v1.0. The compiler is written in Go;
of Eyelang's own tools, only the lexer is written in Eyelang so far.
[What runs today](/reference/status) lists each feature and how far it is built.

Eyelang is a poor fit for GUI applications, for numeric code that needs every cycle of the machine,
and for a team that needs a mature, supported workflow engine in production now.

## See it work

- The [quickstart](/quickstart) installs Eyelang and runs a small reminder program and its test.
- The [tutorial](/tutorial/) starts with small programs and ends with
  [a complete approval workflow](/tutorial/approvals) that uses all of the above.
- The [tour](/reference/language/a-tour) shows the language on one page.
- The [design](/design/) explains each decision and the questions that are still open.
