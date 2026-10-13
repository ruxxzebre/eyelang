---
title: Running in production
---

# Running in production

A program with jobs is meant to keep running: for the three days a question waits for an answer,
for the two hours until access expires, across deploys, crashes and reboots. Usually that takes a
database for the state, a queue for the work, a scheduler for the timers, and a config file that
says where each of them is. `eye run` is all of that for one machine: it runs the program, keeps
its state in one file, and picks up where it stopped after a restart. Its configuration is the
environment.

This page runs the program from [All together: approvals](/tutorial/approvals), with small local
stand-ins for the services it calls. Output shown here is from real runs.

## eye run

```sh
eye run [--journal file | --memory] [program-directory] [-- arguments…]
```

`eye run` does two things, in order:

1. It runs `main`, if the program has one, with the arguments after `--`. `main` runs on every
   start.
2. If the program declares jobs, it stays up and runs them: runs started by `main` or by entity
   methods, sleeping runs when their time comes, schedules when they fire, and runs that events
   start or wake.

It stops when nothing is left that could happen (no schedule, and no run that waits), or when you
stop it with Ctrl-C or SIGTERM. A program without jobs stops after `main`.

Everything the program keeps (entities, runs, events) goes into its **journal**, `.eye/journal` in
the program's directory. `--journal file` puts it somewhere else, such as a volume that outlives
the container. `--memory` keeps everything in memory, so nothing survives the process; use it to
try things out.

Here the approvals program takes a request from the command line:

```sh
cd docs/tutorial/code/approvals
NOTIFY_URL=http://127.0.0.1:18473 ACCESS_URL=http://127.0.0.1:18473 AI_URL=http://127.0.0.1:18473/v1 \
  NOTIFY_TOKEN=… EYE_EVENTS_PORT=8080 eye run . -- r-1 ann prod-db 2 "fix the stuck migration"
```

`main` submits the request, which starts the `review` job. The job asks the model for a summary,
sends the approver a question, and then waits up to three days for an answer, which Notify sends
to port 8080 (see [Events between programs](#events-between-programs)). The program prints
nothing more while it waits. Stop it with Ctrl-C; it exits with status 0.

## Looking at the journal: eye runs

`eye runs` lists the runs in the journal: what each one waits for, until when, and how much space
it takes.

```text
$ eye runs .
.eye/journal: 7.2 KiB on disk, 1.8 KiB live, 6.7 GiB free; 1 runs, 1 not finished
#1 review("r-1", { person: "ann", group: "prod-db", reason: "fix the stuck migration", lasting: 2 h })  waiting for the event notify.answered until 2026-10-14 02:23:35 (in 71h59m58s)  [5 steps, 922 B]
```

With a run's number it shows the run's steps, each with the value that was saved:

```text
$ eye runs . 1
#1 review("r-1", { person: "ann", group: "prod-db", reason: "fix the stuck migration", lasting: 2 h })  waiting for the event notify.answered until 2026-10-14 02:23:35 (in 71h58m54s)  [5 steps, 922 B]
    1. step → Ok({ text: "Ann wants two hours on the production database to fix a stuck migration.", risk: High(r…
    2. Request(r-1).summarise
    3. notify.ask → Asking("q-1")
    4. reconcile → Asking("q-1")
    5. next, waiting until 2026-10-14 02:23:35 (in 71h58m54s)
```

`--json` prints the same as JSON, for scripts and dashboards:

```text
$ eye runs --json .
[
  {
    "id": 1,
    "job": "review",
    "status": "sleeping",
    "args": [
      "\"r-1\"",
      "{ person: \"ann\", group: \"prod-db\", reason: \"fix the stuck migration\", lasting: 2 h }"
    ],
    "wakeAt": "2026-10-14T02:23:35.613142+01:00",
    "waits": "the event notify.answered",
    "steps": [
      {
        "call": "step",
        "value": "Ok({ text: \"Ann wants two hours on the production database to fix a stuck migration.\", risk: High(r…",
        "status": "done"
      },
      …
      {
        "call": "next",
        "until": "2026-10-14T02:23:35.613142+01:00",
        "status": "waiting"
      }
    ],
    "stepCount": 5,
    "size": 922,
    "started": "2026-10-11T02:23:35.607918+01:00"
  }
]
```

`eye runs` reads the journal without changing it, so it works while `eye run` is running. It takes
`--journal` too. Only `--retry` and `--retry-failed` change anything (see
[Retrying a failed run](#retrying-a-failed-run)).

## Restarts

Start the program again with the same command. This is what happens:

- The journal is read back: entities, events, and every run as of its last step. Only the
  records of runs that are ready to run come into memory; a waiting run's record stays in the
  file until the run wakes.
- `main` runs again. Here `submit` sees a request it already has and does nothing, which is why
  `main` should be safe to run twice. A job that `main` starts directly needs a `key:`, so that
  starting it again starts nothing; the checker refuses the call without one.
- Waiting runs carry on from their last step. A run whose time came while the program was down
  runs at once.
- A schedule that missed firings while the program was down fires once, for the latest.
- A command whose reply was lost, and that wasn't settled yet, is still known, so reconciling it
  carries on (see [Talking to other services](/guide/contracts#when-the-reply-is-lost-uncertain)).

The stand-ins' log after two starts and two stops shows one model call and one question:

```text
POST /v1/chat/completions
POST /v1/ask key=review-r-1
```

The model's answer and the question's reply were saved as steps, so the second start used them
instead of calling again.

A step that was in progress when the program stopped didn't save its result, so it runs again
after the restart. That is the reason commands carry keys: running the step again can't do its
work twice. Stopping with `kill -9`, or losing power, is as safe as Ctrl-C, since each step is on
disk before the run goes on.

## Configuration

A program's configuration is its environment. Nothing about addresses or credentials is written in
the code, and there is no configuration file. (`eye.toml` marks the program's root and holds
nothing yet.)

| Variable | What it sets | Read by |
|---|---|---|
| `<MODULE>_URL` | The base URL of the service behind a contract module: `NOTIFY_URL` for `notify`, `SHOP_PAYMENTS_URL` for `shop.payments` | Every call to that contract |
| `<MODULE>_PORT` | The port a contract the program serves listens on instead of its `@port`: `NOTIFY_PORT=8081`; `0` takes any free port | `eye run`, when it starts |
| `EYE_EVENTS_PORT` | The port the program takes other services' events on, besides the ports of the contracts it serves; `0` takes any free port | `eye run`, when it starts |
| `<MODULE>_EVENTS_URL` | Where the events of a contract the program serves go: the base URLs of the programs that take them, separated by commas or spaces, such as `NOTIFY_EVENTS_URL=http://approving:8080,http://trains:8080` | Each event published |
| `AI_URL` | The model gateway's base URL, such as `http://gateway:8317/v1`; when set, it answers every model | `ai.generate` |
| `ANTHROPIC_URL`, `OPENAI_URL` | Where Anthropic's and OpenAI's APIs are, if not at their usual addresses | `ai.generate` |
| `AI_PRICES` | Prices of models for `@budget(usd:)`, in dollars per million tokens in and out: `gpt-5.5=1.25/10,…` | `@budget` |
| A secret's name in upper case, with `-` and `.` as `_` | A secret: `PAYMENTS_TOKEN` for `secret("payments-token")` | `secret(…)`, `@auth`, and the providers' keys `ai-key`, `anthropic-api-key` and `openai-api-key` |
| `EYE_SECRETS` | A directory with one file per secret, named after it, read when the variable isn't set | `secret(…)` |
| Any name you choose | Configuration that isn't secret, such as a proxy's address | `env.get("VPN_PROXY_URL")` |

Secrets come from the environment variable first, then from `$EYE_SECRETS/<name>`, without the
file's trailing newline. Kubernetes and systemd mount secrets as files, so point `EYE_SECRETS` at
that directory and keep secrets out of the environment. A `Secret(String)` can't be shown, logged
or saved, so it can't leak into the journal or a log line.

`env.get(name)` returns an `Option(String)` and has the effect `env`, so a public function that
reads configuration says so in its signature. In a job, or a function with `!` steps, the read
is a recorded step, `env.get!(…)`: a resumed run sees the value it read before, even if the
variable has changed since. See [Effects](/guide/effects).

Settings are read when they are used, not when the program starts. A missing one stops the run that
needed it, with a message that names the variable. From the
[contracts guide's program](/guide/contracts):

```text
main.eye:42:16: job collect: payments.charge calls another service: set PAYMENTS_URL to its address, or replay recordings with `with fakes(payments, from: "…")`
```

```text
main.eye:42:16: job collect: secret "payments-token" isn't set: set PAYMENTS_TOKEN, or put a file payments-token in $EYE_SECRETS
```

and from the [AI calls guide's program](/guide/ai-calls):

```text
main.eye:59:3: job triage: ai.generate asks Anthropic for claude-sonnet-5-5, and has no key: set ANTHROPIC_API_KEY, or put it in $EYE_SECRETS/anthropic-api-key
```

A program that serves a contract itself (with `impl`) calls its own implementation, unless that
contract's `<MODULE>_URL` is set.

## Serving a contract

A program that serves a contract with `impl` answers other programs too: `eye run` listens on the
contract's `@port`, on every network interface, and keeps running until it is stopped, even when
no job is left to wait for. It says where when it starts:

```text
eye run: serving notify.Notify on :3001
```

Requests are routed by the contract's `@http` attributes, read and answered in the contract's JSON
shape, and each runs its operation as one transaction. An operation without `@http` is reached
only from inside the program. With `@auth`, `eye run` reads the secret when it starts, and won't
start without it; a request without it gets 401. See
[Talking to other services](/guide/contracts#implementing-the-other-side).

`NOTIFY_PORT` (the module's name, as for `NOTIFY_URL`) moves the service to another port, and
`NOTIFY_PORT=0` to any free one, which the line above names. Tests use it to run a server and its
callers side by side.

## Events between programs

A contract's events go from the program that serves the contract to the programs that wait for
them, as HTTP requests. Notify's `answered` (`event answered: Answered on "notify.answers"`)
reaches a program as `POST /events/notify.answers`, with the event as JSON in the contract's shape
and the contract's `@auth` secret, the same one its callers send.

**Taking events.** A program takes each event it consumes, with `@on(event(…))` or `next!`, of a
contract it doesn't serve. It takes them on the ports of the contracts it serves, and on
`EYE_EVENTS_PORT`, which a program that serves nothing needs:

```text
eye run: taking notify.answered on :8080
```

- An event is answered 202 once it is in the journal. It then starts its `@on` jobs and wakes the
  runs waiting for it in `next!`, as an event the program published itself would.
- An event with `@key` that comes again is answered 202 and starts nothing, so a sender may
  always send again when it isn't sure.
- A request without the contract's secret gets 401; a body that isn't the event's type, 400.
- Only events of contracts with `@auth` are taken: without one, anyone who reaches the port could
  publish them. `eye run` names such events when it starts (`telegram.updates has no way in:
  contract Telegram has no @auth, so anyone could publish it`), and also those it has no port for.

**Sending events.** A program that serves a contract sends the contract's events to each URL in
`<MODULE>_EVENTS_URL`:

```sh
NOTIFY_EVENTS_URL=http://approving:8080,http://trains:8080 eye run .
```

- Each event, and a delivery for each subscriber, is saved in the journal with the commit that
  publishes it, then sent. Nothing published is lost by a crash: after a restart, what wasn't
  delivered is sent.
- A subscriber that doesn't answer, or answers with a 5xx, gets the event again after half a
  second, then a second, doubling to every 30 seconds, until it takes it. Each subscriber gets
  events in the order they were published.
- A 4xx answer, such as 401 for a wrong secret or 404 for a wrong URL, is a mistake in the
  configuration: the delivery is dropped, with a line that names the subscriber and its answer. So
  is one not taken within 30 days.
- So an event arrives at least once, and one with `@key` starts its jobs once.

A service written in Go or TypeScript sends events with the sender `eye gen go` or `eye gen ts`
writes for each one, such as `SendAnswered`, and keeps them until they are taken.

## When a run fails

A run that stops on a bug, such as a request the service refuses, a missing setting, or an
`unreachable`, is printed on standard error, and `eye run` exits with status 1 when it stops:

```text
main.eye:51:16: job collect: payments.charge was refused with status 404: {"error": "no such route"}
```

The run stays failed in the journal, with its message:

```text
#1 collect("i-3", "cara", 1500, "+353 85 000 0003")  failed: main.eye:51:16: payments.charge was refused with status 404: {"error": "no such route"}  [0 steps, 391 B]
```

Fixing the setting and starting again doesn't start that run again: `main` runs, the invoice
already exists, and nothing new happens. So check the configuration before the first start, for
example by running once with `--memory` against the real services.

### Retrying a failed run

A run that failed because something outside the program wasn't there can be started again once
it is: a service that stayed out of reach for longer than its retries (`Unavailable`), or a run
that ran out of time (`DeadlinePassed`). `eye runs --retry` takes the run's id:

```text
$ eye runs --retry 1 .
run 1 (collect) is ready again: eye run . carries it on, replaying its 2 recorded steps
```

The run keeps its id and key, so `main`'s keyed start and a parent's `join!` find it, and it
replays the steps it recorded before the fault: a charge that went through isn't sent again. Its
`@deadline` counts from when it runs again. `eye runs --retry-failed` retries every failed run
that ended so. While `eye run` holds the journal, the retry goes to its inbox, and the runner
starts the run within a second:

```text
$ eye runs --retry 1 .
retrying run 1 (collect): eye run, which holds the store, starts it within a second, replaying its 2 recorded steps
```

Any other failure is refused, with the reason. A refused request like the one above, `unreachable`
and other bugs end as `Unreachable` and would fail the same way again; `Killed` was on purpose. A
run that failed inside `lock!` or `reserve!` can't be retried either, since it gave the lock or
the memory back as it ended.

## The journal

The journal is one file that only `eye run` writes:

- **Each change is on disk before anyone can see it.** Each commit, such as a step with the
  entity changes it made, is one line, appended at once. The journal is synced before `eye run`
  does anything outside: before it calls a service, runs a program, writes a file, prints,
  answers a request, or waits. One sync covers every commit made since the last, so `main`
  starting 5,000 runs takes a third of a second, not four. A last line cut short by a crash is
  dropped when the journal is read, and so are zeros a crash left where commits never reached
  the disk, with what follows: nothing outside had seen those changes.
- **Each line is small.** A run's record is written as JSON, and a run whose last 20 steps (what
  `eye runs` shows) moved on is written with only the steps they gained. A journal written by an
  `eye` before 2026-10-12 opens as before and is written in this form from then on.
- **A waiting run costs about 140 bytes of memory.** The record of a run that waits or has
  finished stays in the file, written whole when the run starts to wait, and `eye run` keeps only
  what finds it: when it wakes and what wakes it. The record is read back when the run wakes, or
  when something asks for it. 20,000 runs sleeping for a day add about 7 MB to `eye run`.
- **It grows with the state, not the history.** When `eye run` opens the journal, and whenever it
  grows past four times the live data (and past 1 MiB), it is rewritten as a snapshot, through
  a new file renamed over the old one. Above, 7.2 KiB on disk with 1.8 KiB live became 1.8 KiB
  after the restart.
- **Old detail is dropped.** Once an hour, `eye run` drops what no run can use any more: events
  every waiting run has seen, event keys older than 30 days, and the steps of runs that finished
  more than 7 days ago, which keep a summary. Waiting runs and unsettled commands are never
  dropped.
- **A full disk changes nothing.** A change that can't be written is undone, and tried again 10
  seconds later. Beside the journal, `journal.reserve` holds 4 MiB that is given back when a write
  fails, so the rewrite that frees space can still finish.
- **One writer.** `journal.lock` makes sure that only one `eye run` uses a journal. A second one
  stops at once:

  ```text
  eye run: .eye/journal is in use by another eye run
  ```

  So run one instance of a program per journal. To keep the state off the machine, use the
  Postgres store instead, `eye run --store postgres://…`, which also takes one runner per
  database and schema.

A job that never ends, such as a `while true` loop that sleeps, stays small: a run keeps a
checkpoint between the statements of its job, and each wake replays only the steps taken since
the last one. Steps inside a function the job calls have no checkpoint between them, so `eye run`
warns once when a run passes 1,000 steps since its last checkpoint.

**Backups.** Copy the `journal` file; the `.lock` and `.reserve` files beside it aren't needed. A
copy taken while `eye run` is running is still usable, because every change is a whole line and a
cut-short last line is dropped when the journal is read. To restore, stop the program, put the copy
at `.eye/journal` (or wherever `--journal` points), and start it again.

## Stopping

Ctrl-C (SIGINT) and SIGTERM stop `eye run`, and it exits with status 0 unless a run failed. Nothing
is lost: every step that finished is already in the journal, and a step that was cut off runs again
on the next start. A service manager can stop and restart the program at any time.

A signal also stops `main` while it waits, for a timer, a service, its tasks or a channel: it
prints `eye run: stopped while main ran` and exits. `main` is ordinary code, so stopping it half-way
loses nothing the journal keeps. Code that computes without ever waiting isn't interrupted; a
second Ctrl-C ends the process.

A minimal systemd unit, with secrets mounted as credentials:

```ini
[Unit]
Description=Access approvals

[Service]
WorkingDirectory=/srv/approvals
ExecStart=/usr/local/bin/eye run .
Environment=NOTIFY_URL=http://notify:3001 ACCESS_URL=http://access:8080 AI_URL=http://gateway:8317/v1
LoadCredential=ai-key:/etc/approvals/ai-key
Environment=EYE_SECRETS=%d
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

## Checklist

- Every service the program calls has its `<MODULE>_URL`, and every secret is set or in
  `$EYE_SECRETS`.
- Every contract the program serves has its port free, and its callers can reach it.
- Every event the program waits for from another service is taken (`eye run: taking …`), and the
  service that sends it lists the program in its `<MODULE>_EVENTS_URL`.
- `main` is safe to run on every start, and jobs it starts have keys.
- Commands that change something have `@key`, so a step that runs again does nothing twice.
- The journal is on a disk that survives a redeploy, and it is backed up.
- One `eye run` per journal.

## See also

- [The eye command](/reference/cli#run): every flag of `eye run` and `eye runs`.
- [Programs and concurrency](/reference/language/programs-and-concurrency): `main`, modules and `eye run`.
- [Time, secrets, tests](/reference/language/time-secrets-tests): secrets and `env.get`.
- [Jobs and waiting](/guide/jobs-and-waiting): what a step saves.
- [Talking to other services](/guide/contracts) and [AI calls](/guide/ai-calls): the services the
  settings point at.
