---
title: "All together: approvals"
---

# All together: approvals

This page walks through one larger program: temporary access, granted by a person. Someone asks
to join a group, say `prod-db`, for two hours. A model writes a one-line summary for the
approver. The approver gets a message with **Allow** and **Deny** buttons. If they allow it, the
program adds the person to the group, and two hours later takes them out again. Every weekday
morning the approver hears how many requests are still waiting.

Programs like this are usually built from several parts: a database table for the requests, a
queue and a worker for the slow steps, a cron job that looks for expired access, retry loops
around every HTTP call, and a test setup that mocks the clock. In Eyelang it is one program of
about 300 lines with comments, plus 100 lines of tests, and the language checks the parts that usually go wrong: that state
changes are atomic, that a retry can't do something twice, that every failure is handled, and
that a wait of three days survives a restart.

The program is in
[`docs/tutorial/code/approvals`](https://github.com/ruxxzebre/eyelang/tree/dev/docs/tutorial/code/approvals).
Its tests check everything this page says. Run them from the repository's root:

```sh
eye test docs/tutorial/code/approvals
```

```text
ok    a request outside the rules is denied before anyone is asked (693µs)
ok    the approver allows, and the access ends when its time is up (2.182ms)
ok    the approver says no (1.226ms)
ok    a request nobody answers for three days is closed (337µs)
ok    a lost reply from the access service is settled by asking, not by granting again (427µs)
ok    a model that fails costs the summary, not the request (347µs)
ok    the approver gets one question at a time, in order (392µs)
ok    on weekday mornings the approver hears what is waiting (233µs)
8 tests passed (5.837ms)
```

The tests cover three days of waiting, a two-hour grant and a weekend, and they take a few
milliseconds.

## The layout

```text
approvals/
  eye.toml              marks the program's root
  main.eye              requests, the jobs, main, and the tests
  notify/notify.eye     contract: the chat service that sends messages and asks questions
  access/access.eye     contract: the service that says who is in which group
  recordings/           real answers from those services, replayed by the tests
```

A module is a directory. `main.eye` imports the two contracts by directory name, and the standard
library's `ai` module:

<<< ./code/approvals/main.eye#setup

## The services it talks to

A **contract** describes another service: its operations, their types, and how they go over HTTP.
Here is the chat service:

<<< ./code/approvals/notify/notify.eye#contract

Three things in it matter for the rest of the program:

- `@key(required)` means every call carries a key. The service remembers what it did with each
  key, so asking again with the same key can't send a second message.
- `@reconcile(send)` says how to find out what happened when a reply is lost: send again with the
  same key, and the service answers without acting twice.
- Every command can also return `Uncertain(key)`: the request went out, but the reply didn't come
  back. The compiler won't let you use the result until you have dealt with that case, either by
  calling `.reconcile!()` or by matching `Uncertain(_)` yourself.

The access service works the same way, but finds lost grants with a query instead of sending
again:

<<< ./code/approvals/access/access.eye#contract

A typical HTTP client returns a status code or throws, and you decide in each place whether a
timeout means "retry" or "give up". Here the contract decides once, and a command is never sent
twice on a guess. A request that never reached the service is simply sent again. A request whose
reply was lost is resolved by asking its owner.

## A request has one owner

The request's state lives in an **entity**: one `Request` per id, with its own fields and
methods.

<<< ./code/approvals/main.eye#details

<<< ./code/approvals/main.eye#request

Each method is a transaction. One caller at a time changes a given request, and a method either
commits completely or not at all. `submit` does two things in one transaction: it records the
request, and it starts the `review` job. If the transaction rolls back, the job never starts. If
the job starts, the request is recorded.

With a database and a queue, those are two writes to two systems, so a crash between them leaves
a recorded request that nobody reviews, or a review for a request that isn't recorded. The
usual fix is an outbox table and a relay process. Here the language handles it.

`submit` is also safe to call twice. A request that isn't `New` any more ignores it, so a
double-clicked button or a retried webhook starts one review.

## Rules are pure functions

Which groups can be asked for, and for how long, is plain code:

<<< ./code/approvals/main.eye#policy

`admissible` is public and has no `uses` clause, so the compiler holds it to being pure: it can't
read the clock, a file or the network. If someone later adds a rule like "no prod access after
six in the evening", the signature has to change, and the change shows in review:

```text
main.eye:40:1: admissible uses clock, which its signature doesn't list: add uses {clock}
```

## The review: a job that waits

The review is a **job**: a function that can pause for as long as it needs, and that keeps no
memory or thread while it waits.

<<< ./code/approvals/main.eye#review

Read it top to bottom, the way it runs:

1. `summarise!(d)` asks a model for a summary (shown below).
2. `notify.ask!(…)` sends the question with two buttons. The key `review-{id}` makes it safe to
   repeat. `.reconcile!()` resolves a lost reply before the program goes on.
3. `next!(notify.answered, …, within: 3 days)` waits up to three days for the approver to press a
   button. It returns `None` if nobody does.
4. `access.grant!(…).reconcile!()` adds the person to the group, with its own key.

**Every `!` is a checkpoint.** Its result is saved before the job goes on. If the program
restarts while the job waits on `next!`, the job resumes at that line with the same `question` in
hand. The model isn't called again and the question isn't sent again. The pauses are all visible:
a reviewer can see every point where this job can stop or touch the outside world by looking for
`!`. Leave one out and the checker says so:

```text
main.eye:149:17: summarise waits, so calling it is a durable step: write summarise!(…)
main.eye:206:3: in a job, an entity call is a durable step: write close!(…)
```

**`@concurrency(1)`** makes the approver get one question at a time, in the order the requests
came in. A second request's run queues until the first one finishes, and keeps its place in line
while it waits.

The alternative is usually a `pending_reviews` table with a `state` column, a worker that polls
it, an `asked_at` timestamp, a cron job that expires old rows, and code to make sure two workers
don't take the same row. The job above is that state machine, written as straight-line code.

## Failures are values

Nothing in Eyelang throws. Every failure is a value in a type, and every `match` must cover all of
its cases. The program uses this in three places.

**The model.** `ai.generate!` names the type it wants, and the compiler derives a JSON Schema
from it. The reply is checked against the schema and decoded into a `Summary`, or it is an
`Err`:

<<< ./code/approvals/main.eye#summary

If the model is down or answers with something that doesn't fit, the approver still gets the
request, without a summary. That is a decision written in the code, not a stack trace in a log.
The `##` comments on `Summary` and `Risk` become descriptions in the schema the model sees.

**The services.** The `match` on `access.grant!` handles `Added` and `NoSuchGroup`. Remove
`.reconcile!()` and the checker points out the case you would be missing:

```text
main.eye:174:3: this match doesn't handle Uncertain(_): add an arm for it, or `_ => …`
```

**Your own types.** Add a fourth risk level, `Critical`, and every `match` on `Risk`, such as
`describe`, stops compiling until it says what to do with it:

```text
main.eye:130:34: this match doesn't handle Critical: add an arm for it, or `_ => …`
```

## Effects show in signatures

`tell` sends a person a message. It is public, so its signature lists everything it can do:

<<< ./code/approvals/main.eye#tell

`uses {wait, net Notify.send}` says it is a durable step and that it calls exactly one operation
of one service. If it also asked a question, `net Notify.ask` would have to appear here too. A
new permission can't sneak in without changing a signature that a reviewer sees.

## Taking access away: a view and a sleeping job

A **view** is a named query over entities that the runtime keeps up to date as transactions
commit:

<<< ./code/approvals/main.eye#active

`@on(enters(active))` starts one run of `expire` for each request that becomes active:

<<< ./code/approvals/main.eye#expire

`sleepUntil!(until)` waits two hours, or eight, or a week. While it waits, the run is a few
hundred bytes in the journal, not a thread or a timer in memory. This replaces the usual cron job
that runs every minute, scans a table for rows where `expires_at < now()`, and must be written
so that two overlapping runs don't revoke twice. Here the revoke has a key, and each grant gets
exactly one run.

## Weekday mornings: a schedule

<<< ./code/approvals/main.eye#morning

`weekdays(09:00, in: dublin)` fires at nine on Irish time, Monday to Friday, through daylight
saving changes. The job gets the date of the firing, which goes into the message's key, so
whatever happens to that run, the approver gets one message a day.

## main

`main` is an ordinary function that runs each time the program starts. It submits one request
from the command line:

<<< ./code/approvals/main.eye#main

It calls an entity method, so its signature says `uses {entity Request}`. Because `submit`
ignores a request it has seen, running the program again with the same arguments changes nothing.

## Tests that play out days in milliseconds

Tests run against an in-memory store with a clock that moves only when the test says so. Calls
to other services replay recordings of real answers. Here is the whole happy path, from request
to expiry:

<<< ./code/approvals/main.eye#test-data

<<< ./code/approvals/main.eye#test-allow

- `with clock(…)` starts the test's clock at a fixed time, and `advance(119 min)` moves it. Every
  run that wakes on the way runs before `advance` returns, so the test can check the state right
  after.
- `notify.answered.publish(…)` stands in for the approver pressing a button. The waiting review
  wakes at once.
- `with fakes(ai, notify, access, from: "recordings/")` answers each call from a recording,
  matched by operation and key, and checked against the contract when it is loaded.

The recording for the access service includes a grant whose reply was lost:

<<< ./code/approvals/recordings/access.json

The test for it checks that the program asks `grantFor` with the same key, finds the grant that
did land, and goes on, without granting a second time:

<<< ./code/approvals/main.eye#test-lost

Waiting three days, and a weekend, cost nothing either:

<<< ./code/approvals/main.eye#test-unanswered

<<< ./code/approvals/main.eye#test-morning

There is no sleep in these tests, no retry, and no mock of `Date.now`. They give the same result
on every run, on any machine, in any time zone.

## Run it for real

`eye run` runs `main`, then keeps the program's jobs going and saves everything to a journal in
`.eye/journal`. It reads each service's address from the environment: `NOTIFY_URL`, `ACCESS_URL`,
and `AI_URL` for an OpenAI-compatible model gateway. The run below used small local stand-ins for
the three services.

```sh
cd docs/tutorial/code/approvals
eye run . -- r-1 ann prod-db 2 "fix the stuck migration"
```

The program calls the model, sends the question, and then waits for an answer. Stop it with
Ctrl-C, and look at the journal with `eye runs`:

```text
$ eye runs .
.eye/journal: 7.2 KiB on disk, 1.8 KiB live, 6.8 GiB free; 1 runs, 1 not finished
#1 review("r-1", { person: "ann", group: "prod-db", reason: "fix the stuck migration", lasting: 2 h })  waiting for the event notify.answered until 2026-10-14 00:10:39 (in 71h59m56s)  [5 steps, 922 B]
```

The review is waiting for `notify.answered`, with three days to go. It takes 922 bytes. Its steps
show what has been recorded so far:

```text
$ eye runs . 1
#1 review("r-1", { person: "ann", group: "prod-db", reason: "fix the stuck migration", lasting: 2 h })  waiting for the event notify.answered until 2026-10-14 00:10:39 (in 71h59m52s)  [5 steps, 922 B]
    1. step → Ok({ text: "Ann wants two hours on the production database to fix a stuck migration.", risk: High(r…
    2. Request(r-1).summarise
    3. notify.ask → Asking("q-1")
    4. reconcile → Asking("q-1")
    5. next, waiting until 2026-10-14 00:10:39 (in 71h59m52s)
```

Start it again with the same command. `main` runs again, `submit` sees a request it already has,
and the waiting run carries on from step 5. The stand-ins' log shows one model call and one
question across both starts:

```text
POST /v1/chat/completions
POST /v1/ask key=review-r-1
```

::: info Not yet
In `eye run`, nothing outside the program can publish an event yet, so the approver's answer has
no way in, and the review waits out its three days. The tests above publish answers directly.
:::

## What you used

| Feature | Where it is in the program | Reference |
|---|---|---|
| Contracts with keys, `@reconcile` and `Uncertain` | `notify`, `access`, every `.reconcile!()` | [Contracts](/reference/language/contracts-talking-to-other-services) |
| Entities: owned state, atomic methods, jobs started in the transaction | `Request` | [Entities](/reference/language/entities) |
| Pure public functions, `uses` on the rest | `admissible`, `tell`, `main` | [Effects and signatures](/reference/language/effects-and-signatures) |
| Jobs, `!` checkpoints, `next!`, `sleepUntil!`, `@concurrency` | `review`, `expire` | [Jobs and waiting](/reference/language/jobs-and-waiting) |
| `Result`, exhaustive `match`, `let … else` | `summarise`, `describe`, `review` | [Errors: Result only](/reference/language/errors-result-only) |
| Typed model calls with a schema from a type | `summarise` | [AI models and agents](/reference/language/ai-models-and-agents) |
| Views and `@on(enters(…))` | `active`, `expire` | [Views and triggers](/reference/language/views-and-triggers) |
| Schedules, test clocks, recorded fakes | `morning`, the tests | [Time, secrets, tests](/reference/language/time-secrets-tests) |
| `main`, modules, `eye run` and the journal | `main`, the layout | [Programs and concurrency](/reference/language/programs-and-concurrency) |

## Next steps

- Change the program and let `eye check` and `eye test` guide you. For example, make a
  `Critical` risk deny a request at once, without asking, and add a test with a recorded model
  reply for it.
- Read the lab's workloads in
  [`testdata/lab`](https://github.com/ruxxzebre/eyelang/tree/dev/testdata/lab), the real
  programs this one is modelled on: a morning news digest, train reminders, and NetBird access.
- See [the `eye` command](/reference/cli) for `eye run`, `eye runs` and `eye test` in full.
