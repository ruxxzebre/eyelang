---
title: A job that waits
---

# A job that waits

Much of what a stateful program does is wait: for a timeout, for a reply, for a person, for
tomorrow morning. In Eyelang that waiting is done by **jobs**. A job is a function that can pause
for as long as it needs, keeps only a small handle in memory while it waits, and carries on after
a restart.

This page gives the shelf from the [previous page](/tutorial/an-entity) a rule: a hold on stock
lasts fifteen minutes. If the order isn't paid for by then, the stock goes back on the shelf.

The program is [`docs/tutorial/code/a-job-that-waits/main.eye`](https://github.com/ruxxzebre/eyelang/blob/dev/docs/tutorial/code/a-job-that-waits/main.eye).

[Open it in the playground](/playground#example=tutorial/a-job-that-waits) to run it, test it and change it in your browser.

## The job

<<< ./code/a-job-that-waits/main.eye#job

`job` declares a job. Its body reads like an ordinary function, with one difference: the calls
marked with `!`.

- `sleep!(15 min)` pauses the run for fifteen minutes. `15 min` is a `Duration` literal.
- `Shelf(item).release!(order)` calls a method on the entity, from a job.
- `now!()` reads the clock.

## `!` marks the waiting steps

In a job, every call that waits or touches the outside world is written with `!`. Each one is a
**durable step**. When the step finishes, its result is recorded with the run, and when the run
resumes, after a sleep or after a restart, it doesn't do the step again. It picks up from the last
step with the same values it had.

So `!` shows you, at a glance, every place a job may pause and every effect it has. A call without
`!` only computes. The checker holds you to it. Leave the `!` off `release` and `eye check` says:

```text
main.eye:35:6: in a job, an entity call is a durable step: write release!(…)
```

An entity call from a job commits in the same transaction as the step that records it, so the
release happens exactly once, even if the program stops right after it.

## Starting the job

The shelf starts the job when it takes a hold:

<<< ./code/a-job-that-waits/main.eye#shelf

`expire.start(item, order, key: …)` in `hold` starts a run of the job. The start is part of the
method's transaction: the run begins only if `hold` commits. A hold and its expiry can't come apart.
The key starts one run per hold, however often `hold` is called with it, which `main` below needs.
Starting a job is an effect, so the public method says so: `uses {start expire}`.

`release` returns whether it found the hold, so the job logs only when it put stock back. An order
that was paid in time has been removed by `sell`, and its expiry does nothing.

## Testing time

Nobody wants a test that waits fifteen minutes. In a test, jobs run against a clock that moves
only when the test says so:

<<< ./code/a-job-that-waits/main.eye#test

- `with clock(...)` sets the test clock's start time for the rest of the block. Without it, the
  clock starts at 2026-01-01 00:00 UTC.
- `advance(10 min)` moves the clock forward. Every run that wakes on the way runs before `advance`
  returns, so the next line sees its effects.
- A job started by a call runs as soon as the call returns, until it finishes or waits. Here both
  runs start sleeping when `hold` returns.

The test reads like the story it checks: after ten minutes order-2 is paid; at fifteen minutes
order-1's hold runs out and its two apples go back. It runs in milliseconds, and the same way every
time. `eye test -v` also shows what the runs logged:

```text
ok    a hold ends after 15 minutes unless the order is paid (1.924ms)
      info: released order-1's hold on apples at 2026-10-10 12:15 in "Europe/Dublin"
1 tests passed (1.924ms)
```

## Running it for real

To run the program outside a test, give it a `main`. `eye run` runs `main`, then stays up for as
long as any job has something to wait for:

<<< ./code/a-job-that-waits/main.eye#main

`main` runs every time the program starts, so it checks the shelf's `version` and stocks it only
the first time. Like any public function, it lists its effects: `uses {entity Shelf, start expire}`,
since `hold` starts `expire`. A job that `main` starts, itself or through a method it calls, needs
a key, or every restart would start it again; the checker follows the call into `hold` to see that
it has one.

```sh
eye run
```

`eye run` keeps entities and runs in a journal, `.eye/journal` in the program's directory, and
writes each step there before it counts. Stop the program with Ctrl-C while the hold is waiting,
and look at the journal with `eye runs`:

```sh
eye runs
```

```text
.eye/journal: 1.1 KiB on disk, 492 B live, 9.4 GiB free; 1 runs, 1 not finished
#1 expire("apples", "order-1")  sleeping until 2026-10-11 00:18:30 (in 14m58s)  [1 step, 237 B]
```

`eye runs 1` shows one run and its steps:

```text
#1 expire("apples", "order-1")  sleeping until 2026-10-11 00:18:30 (in 14m52s)  [1 step, 237 B]
    1. sleep, sleeping until 2026-10-11 00:18:30 (in 14m52s)
```

Start the program again with `eye run`. `main` sees that the shelf already has a version and
does nothing, and the sleeping run carries on where it was. When its time comes, it releases the
hold, and `eye run` exits because nothing is left to wait for:

```text
info: released order-1's hold on apples at 2026-10-11 00:18 in "Local"
```

`eye runs 1` now shows the run as done, with its three recorded steps:

```text
#1 expire("apples", "order-1")  done  [3 steps, 377 B]
    1. sleep, slept until 2026-10-11 00:18:30 (22s ago)
    2. Shelf(apples).release → true
    3. now → 2026-10-11 00:18 in "Local"
```

Each step is recorded with its result. If the program had stopped between step 2 and step 3, the
restarted run would not release the hold again: it would read `true` from the journal and go on to
`now!()`.

A run that was due while the program was down runs as soon as it starts again. `eye run --memory`
keeps nothing, which is handy for trying things out.

## Next

[All together: approvals](/tutorial/approvals) puts everything from these pages into one larger
program: entities, jobs that wait for a person to answer, and tests that play out days in
milliseconds.

The full rules:

- [Jobs and waiting](/reference/language/jobs-and-waiting): every waiting call (`sleep!`, `poll!`,
  `next!`, `approval!` and more), starting runs with `after:`, `at:` and `key:`, and the attributes
  that schedule and limit them.
- [Effects and signatures](/reference/language/effects-and-signatures): why every outside call in
  a job is a `!` call.
- [Time, secrets, tests](/reference/language/time-secrets-tests): schedules, time zones, and test
  clocks.
- [Programs and concurrency](/reference/language/programs-and-concurrency): `main`, `eye run` and
  the journal.
