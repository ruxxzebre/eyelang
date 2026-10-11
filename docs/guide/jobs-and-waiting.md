---
title: Jobs and waiting
---

# Jobs and waiting

A program that keeps state spends most of its time waiting: for a reminder's time, for a person
to answer, for a slow export, for tomorrow morning. The usual ways to wait are a sleeping thread,
which is lost on a restart, or a table of pending work with a status column, a worker that polls
it, and a cron job that cleans up after both.

In Eyelang, waiting is done by **jobs**. A job is a function that can pause for as long as it
needs. While it waits it holds no thread and no memory, only a saved record, and it carries on
after a restart from where it was. This page explains what is saved, how runs start and are
limited, and how they end. It assumes you have read [A job that waits](/tutorial/a-job-that-waits)
in the tutorial.

The code on this page is a [real program](https://github.com/ruxxzebre/eyelang/tree/dev/docs/guide/code/jobs-and-waiting)
that the test suite checks and runs.

## A job is a durable function

A reminder rings at its time, unless it is snoozed first:

<<< ./code/jobs-and-waiting/main.eye#reminder

<<< ./code/jobs-and-waiting/main.eye#ring

`job ring` reads like an ordinary function. The difference is the calls marked with `!`. Each one
is a **durable step**: a call that waits, reads the clock, touches an entity or calls another
service. Calls without `!` only compute.

`ring` reads the reminder, sleeps until it is due, and reads it again. If someone snoozed it in
the meantime, `due` has moved, and the loop sleeps again. A job that waits should read state again
after the wait, because whatever it read before is a snapshot from before the wait.

<<< ./code/jobs-and-waiting/main.eye#test-ring

## What is saved at a `!`

At each `!`, the run saves its live variables and records the call's result. When the run resumes,
after a sleep or after a restart, it goes on from the last step, with the same values. It never
makes a recorded call again: an entity call that committed isn't repeated, a message that was sent
isn't sent twice, and `now!()` returns the time it returned the first time.

You can see this with `eye run`. The program's `main` sets a reminder for one minute from now, the
first time it runs:

<<< ./code/jobs-and-waiting/main.eye#main

Start it with `eye run`, stop it with Ctrl-C while the reminder waits, and look at the journal with
`eye runs`:

```text
.eye/journal: 2.5 KiB on disk, 795 B live, 6.9 GiB free; 1 runs, 1 not finished
#1 ring("stretch")  sleeping until 2026-10-11 02:11:23 (in 56s)  [3 steps, 437 B]
    1. Reminder(stretch).read → { id: "stretch", version: 1, text: "stand up and stretch", due: 2026-10-11 02:11 in "Local" }
    2. now → 2026-10-11 02:10 in "Local"
    3. sleepUntil, sleeping until 2026-10-11 02:11:23 (in 56s)
```

The waiting run is 437 bytes in the journal: its arguments, its live variables and three recorded
steps. Start the program again, and `main` sees the reminder exists and does nothing, while the
run sleeps on. When its time comes it finishes, and `eye run` exits because nothing is left to
wait for:

```text
#1 ring("stretch")  done  [7 steps, 760 B]
    1. Reminder(stretch).read → { id: "stretch", version: 1, text: "stand up and stretch", due: 2026-10-11 02:11 in "Local" }
    2. now → 2026-10-11 02:10 in "Local"
    3. sleepUntil, slept until 2026-10-11 02:11:23
    4. Reminder(stretch).read → { id: "stretch", version: 1, text: "stand up and stretch", due: 2026-10-11 02:11 in "Local" }
    5. now → 2026-10-11 02:11 in "Local"
    6. now → 2026-10-11 02:11 in "Local"
    7. Log(rang).add
```

Steps 5 and 6 are the two `now!()` calls: the loop's test, and the time in the message. If the
program had stopped after step 7 was recorded, a restart would not add the line again. A run that
was due while the program was down runs as soon as it starts.

An entity call from a job commits in the same transaction as the step that records it, so it
happens exactly once. The checker makes sure every such call is marked:

```text
main.eye:34:10: in a job, an entity call is a durable step: write read!(…)
```

### Only values that can be saved

A value that is live across a `!` is saved with the run, so it must be a value that can be saved.
A `Secret` can't, so holding one across a wait is an error:

```text
main.eye:4:14: token is a Secret, which can't be saved, and it is still needed after a waiting call: get it again after the `!`
```

## Ways to wait

Every waiting call is a library function, called with `!`:

| Call | Waits for |
|---|---|
| `sleep!(20 min)`, `sleepUntil!(at)` | a time |
| `approval!(text, from:, within:)` | a person to approve or decline |
| `next!(event, where:, within:)` | an event; see [Views, events and schedules](/guide/views-events-schedules#waiting-for-an-event) |
| `poll!(every:, within:, check)` | a service that can't say when it is ready |
| `run.join!()` | a run started earlier; see [Outcomes and faults](#outcomes-and-faults) |

A wait that can last long takes a `within:`, and says what happens when time runs out. Here a
clean-up asks a person before it deletes anything:

<<< ./code/jobs-and-waiting/main.eye#prune

`approval!` returns a `Decision`, and the `match` must handle all three: `Approved`, `Declined`
and `Expired`. A day with no answer is a case the code decides on, not an exception or a run that
waits forever. In tests, `approve(...)` and `decline(...)` answer the request whose text contains
what they are given:

<<< ./code/jobs-and-waiting/main.eye#test-prune

Outside tests, a request reaches a person through the program. When a run starts waiting, the
runtime publishes `approvals.requested`; a job of the program sends it on, say as a question with
two buttons, and publishes `approvals.answered` with the answer:

```eye
import approvals

@on(event(approvals.requested))
job ask(r: approvals.Request) {
  # send r.what to whoever r.from names, wait for their answer, then:
  approvals.answered.publish!({ what: r.what, approved: true, by: "ann" })
}
```

From a shell, `eye approve` and `eye decline` answer a run of `eye run` in the same directory:
`eye approve . "delete 3 backups"`. `eye runs` shows what each run waits for.

`poll!` is for services that can only be asked:

```eye
let ready = poll!(every: 1 min, within: 6 h, fn { feed.ready!() })
```

It calls the check every minute until it returns `true`, and gives `false` if six hours pass
first. Only its outcome is kept, so a long poll doesn't grow the run's history.

## Starting runs

A run starts in one of four ways:

- **From an entity method,** as part of its transaction. `set` starts `ring`, and the run begins
  only if `set` commits. See [Entities](/guide/entities#starting-a-job-from-a-method).
- **From a job,** as a child run: `export.start(t)` below.
- **From `main` or a test.** In a test, the run goes as soon as the call returns, until it
  finishes or waits.
- **From a trigger:** `@on(...)` starts a run on a schedule, when an entity enters or leaves a view,
  or when an event arrives. See [Views, events and schedules](/guide/views-events-schedules).

`start` returns a `Run`, and takes three optional arguments:

```eye
ring.start(id, after: 20 min)
```

- `after:` starts the run later, and `at:` at a given instant.
- `key:` starts a job once per key. A later start with the same key returns the first run,
  finished or not, and starts nothing.

`set` gives its run a key, so setting a reminder twice rings it once. Since `ring` reads the
reminder when it wakes, it rings with the latest text and time:

<<< ./code/jobs-and-waiting/main.eye#test-key

`main` runs every time the program starts, so a job it starts must have a key. Without one, the
checker refuses the call:

```text
main.eye:6:3: eye run runs main every time it starts, so this would start ping again after each restart; give the run a key, which starts it once: ping.start(…, key: "…")
```

## Limits

Attributes on a job tell the runtime how many runs may go at once, how often they may begin, and
how long they may take:

<<< ./code/jobs-and-waiting/main.eye#post

- `@limit(n, per: duration, by: param)` lets at most `n` runs begin in any window of that length.
  A run over the rate waits until the window lets it begin. Use it for another service's rate
  limit.
- `@concurrency(n, by: param)` lets at most `n` runs go at once. A run over the limit waits in a
  queue and begins when one finishes, in the order they were started. Use it when runs must not
  overlap, such as one question at a time to the same person.
- `by:` names a parameter, and applies the limit to each of its values on its own: chats don't wait
  for each other.

<<< ./code/jobs-and-waiting/main.eye#test-post

A scheduled firing that finds a `@concurrency` limit reached is skipped, not queued, so a slow run
of a job that fires every minute doesn't build a backlog.

## Outcomes and faults

A run ends with an `Outcome`: `Completed(value)`, with what the job returned, or `Failed(fault)`
when it stopped first. A `Fault` is not an error the job returns. It is something that stopped the
run from outside its logic, such as `DeadlinePassed` from `@deadline`, or `Unreachable` when the
code reached `unreachable(...)`. Code can't catch a fault where it happens. To report one, start
the work as a child run and join it:

<<< ./code/jobs-and-waiting/main.eye#export

`@deadline(30 min)` ends a run that is still going 30 minutes after it started, even while it
sleeps. `nightly` starts all three exports before it waits for any, so they run at the same time,
and the whole night takes as long as the slowest one:

<<< ./code/jobs-and-waiting/main.eye#test-export

`@deadline` also takes a time of day: `@deadline(13:40, in: "Europe/Dublin")`.

Errors that the job can handle belong in its return type, as a `Result`. Faults are for what it
can't: a deadline, a bug, a service that stayed down.

## Concurrency inside a job

Child runs are how a job does durable work at the same time, as `nightly` does. The task functions
`parallelMap`, `both`, `race` and `timeout` work in a job only for work that computes, because a
task's steps can't be recorded. A task that takes a `!` step stops the run:

```text
main.eye:8:26: job inBoth: a task in a job can only compute: its steps couldn't be recorded, so start a child run for durable work (SPECS.md §13)
```

The checker doesn't catch this yet; the run fails when it gets there, and so does its test. Outside
jobs, in `main` and in plain functions, tasks can wait, and their waits overlap. See
[Programs and concurrency](/reference/language/programs-and-concurrency).

## When to write a job

Write a job when the work **waits, or must finish even if the program restarts**: a timeout, a
reminder, a call to a service that may be down, a question to a person, a nightly export. Each step
is recorded, so the job does each thing once.

Write a plain function for everything else. A function that only computes doesn't need to be
durable, and recording its steps would cost for nothing. A job can call it freely, without `!`. A
public function that waits says `uses {wait}` in its signature, and is called with `!` from jobs.

## Mistakes this prevents

| The bug | Why it can't happen |
|---|---|
| A restart loses a sleeping timer, or a pending step | Runs and their steps are in the journal, and a due run runs on start |
| A retried step sends a message twice | A recorded step isn't repeated, and an entity call commits with its checkpoint |
| A wait that hangs forever | Long waits take `within:`, and their result says time ran out |
| A hidden pause or side effect | Every durable step is marked with `!`, and the checker insists |
| A secret or socket saved to disk | Only values that can be saved live across a `!` |
| Duplicate runs after a restart | A job started from `main` needs a key |
| Overlapping runs, a flooded API | `@concurrency` and `@limit`, enforced by the runtime |

::: info Not yet
`reserve!`, `lock!`, and `all!` and `first!` for joining several runs are designed but not built;
join child runs one after another, as `nightly` does. `@memory` and `@priority` are checked but
not enforced; `@budget` is (see [AI calls](/guide/ai-calls#budgets)). See
[What runs today](/reference/status#jobs-and-waiting).
:::

## See also

- [Jobs and waiting](/reference/language/jobs-and-waiting) in the reference: every waiting call,
  `start`, and the attributes.
- [Entities](/guide/entities): the state jobs read and change.
- [Views, events and schedules](/guide/views-events-schedules): runs started by triggers, and
  `next!`.
- [Testing with time and recordings](/guide/testing): tests that play out days of waiting in
  milliseconds.
- [Running in production](/guide/running): `eye run`, the journal and `eye runs`.
