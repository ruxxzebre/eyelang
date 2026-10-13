---
title: "Time, secrets, tests"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Time, secrets, tests

**Time.** Schedules carry their zone: `daily(07:40, in: "Europe/Dublin")`, and `weekdays(…)` is the
same from Monday to Friday. `cron("*/20 7-9 * * mon-fri", in: dublin)` fires when the five fields
of a crontab line say, on the zone's clock. `every(15 min)` fires at each multiple of its interval
since midnight UTC: :00, :15, :30, :45.

`cron` takes the fields every cron takes, and nothing more:

- **Five fields:** minute (0–59), hour (0–23), day of month (1–31), month (1–12, or `jan` to
  `dec`) and day of week (0–7, where 0 and 7 are Sunday, or `sun` to `sat`). Names are in any case.
- **Each field** is `*`, a value, a range (`mon-fri`), a list (`1,15`) or a step over `*` or a
  range (`*/15`, `8-18/2`).
- **When both day fields are given, a day matches if either does,** as in every cron:
  `0 9 1 * mon` fires on the 1st and on every Monday. A day field that starts with `*`, such as
  `*/2`, narrows the other instead.
- **Not taken:** a seconds or year field, the macros (`@daily` is `daily(00:00, in: zone)`), and
  `L`, `W`, `#` and `?`. A line that is wrong, or that never fires (`0 0 30 2 *`), is an error at
  the string.

A daylight-saving change skips an hour or repeats one, and the schedules on a zone's clock
(`daily`, `weekdays` and `cron`) say what they do then with `onGap:` and `onOverlap:`. The
defaults mean there are never two UTC crons and an hour check:

| Policy | At a time the change skips or repeats, the schedule… |
|---|---|
| `onGap: Shift` (default) | fires as far after the change as it was meant to be: `01:30`, on the night the clock jumps from 01:00 to 02:00, fires at 02:30 |
| `onGap: Skip` | doesn't fire for that time that day |
| `onOverlap: First` (default) | fires the first time the clock shows it |
| `onOverlap: Second` | fires the second time |
| `onOverlap: Both` | fires both times, an hour apart: for a job that should run every hour, `cron("0 * * * *", in: dublin, onOverlap: Both)` |

A time shifted onto another firing of the same schedule fires once: with `Shift`, an hourly cron
fires every hour across the jump. The policies are the prelude's `GapPolicy` and `OverlapPolicy`.

A schedule counts from when the program first starts; each firing is saved with the run it
starts, so a restart neither repeats nor skips one; firings missed while the program was stopped
run once, for the latest, when it starts again. Dates in another zone are asked for by name, never
taken from the host's clock:

| Call | Gives |
|---|---|
| `at.date(in: dublin)`, `at.time(in: dublin)` | the date or time of day the clock in that zone shows at instant `at` |
| `08:00.next(in: dublin, after: at)` | the first instant after `at` when that clock shows 08:00 |
| `day.at(07:40, in: dublin)` | the instant that day at that time |
| `day.format("%A %-d %B")` | the date written out: `Monday 12 October` |
| `t.format("%H:%M")` | the time of day written out: `07:40`, or `7:40 AM` with `"%-I:%M %p"` |
| `at.unix()` | seconds since 1970-01-01 00:00 UTC, which many APIs take |
| `d.seconds()`, `d.millis()` | a `Duration` as a number: whole seconds or milliseconds, rounded down, an `Int` |

A `Duration` is divided as a number is: `d / n` by a `NonZero(Int)` is a `Duration`, and `d / e`
by another `Duration` is how many times `e` goes into `d`, as a `Float` (dividing by a zero
`Duration` gives an infinity, as for any `Float`). So an average wait, the share of a lead time
that has passed and a timeout an API takes in milliseconds are ordinary arithmetic:

```eye
fn averageWait(total: Duration, runs: NonZero(Int)) -> Duration = total / runs

fn percentElapsed(since: Duration, lead: Duration) -> Float = since / lead * 100.0

fn timeoutHeader(limit: Duration) -> String = "{limit.millis()}"
```

`format` takes strftime's directives: for a date `%A` (the weekday), `%B` (the month), `%d`,
`%m` and `%Y`; for a time of day `%H` (00 to 23), `%I` (01 to 12), `%M`, `%S` and `%p` (`AM` or
`PM`); `%%` is a percent sign, and `-` after `%` drops the leading zero, as in `%-d` and `%-H`. A
time's text is `07:40:00`, so `t.format("%H:%M")` is how to write it without the seconds.

**Secrets and configuration.** A secret is read by name when the program needs it: from the
environment variable of that name in upper case, with `-` and `.` as `_`, or from the file of that
name in the directory `$EYE_SECRETS`, where Kubernetes and systemd put mounted secrets. A contract
sends its secret with `@auth` ([section 17](/reference/language/contracts-talking-to-other-services)), so code rarely holds one.

```eye
let token: Secret(String) = secret("telegram-bot-token")  # TELEGRAM_BOT_TOKEN
log.info("token is {token}")               # compile error: Secret has no Show
```

Configuration that isn't secret, such as a proxy's address, comes from `env.get("VPN_PROXY_URL")`,
an `Option(String)`, with the effect `env`.

**Tests.** A `test` block is a declaration. `eye test` runs the tests of every module of the
program, the modules it imports from its own directory included. `with` applies a handler to the rest of the block:
`fakes(…)`, `clock(…)` with `advance(…)`, `memory(…)`. In tests, jobs run against an in-memory
store, so every run is deterministic and fast. Each test has an empty directory of its own, where
`fs` and `process` work with relative paths and which is removed when it ends, and `random` draws
from the same seed in every test. The clock starts at `clock(…)`'s time (2026-01-01
00:00 UTC without one) and moves only when the test calls `advance(…)`; each run that wakes on the
way runs before `advance` goes on, and so does each schedule that fires, if its job is in the
test's own module (a test of the reminders doesn't start the NetBird sync). Jobs started by a call run as soon as the call returns, until
they finish or wait, and so do runs that an event a test publishes starts or wakes:
`notify.answered.publish(answer)` stands in for a person pressing a button, and
`approve("delete 3 backups")` or `decline(…)` answers, as the person asked, the `approval!` whose
text contains it, and fails the test when none or several do.
`expect(cond)` fails the test, and for a comparison shows the value it got:
`expected w.phase == Asleep, but w.phase is Running`. A run that stops on a bug (`unreachable`, a
fake with no recorded answer) fails the test too; one that ends as an outcome the program
declared, such as `Failed(DeadlinePassed)` from its `@deadline`, doesn't, and the test checks it
with the run's `outcome()`: `expect(run.outcome() == Some(Failed(DeadlinePassed)))` ([section 15](/reference/language/jobs-and-waiting)).

**Memory in tests.** A test's runner has 1 GiB of memory, whatever the machine ([section 15](/reference/language/jobs-and-waiting)).
`with memory(64 MiB)` gives it that much instead, for the rest of the test, so a test can show a
run waiting for memory, or ending as `Failed(OutOfMemory)`, without a machine of that size. It
changes the limit that `@memory` and `reserve!` are admitted against, and nothing else: what runs
already hold stays held, and runs that wait for memory and now fit run before the next statement.
A size of `0 B` is a fault; under `eye run` the limit comes from `--memory-limit`, so `memory(…)`
works only in tests.

```eye
@memory(128 MiB)
job thumbnails() {}

test "thumbnails don't fit on a small runner" {
  with memory(64 MiB)
  let run = thumbnails.start()
  expect(run.outcome() == Some(Failed(OutOfMemory)))
}
```

**Programs a test runs.** `with fakes(process, from: "recordings/")` answers `process.run` from
`recordings/process.json` instead of running anything, so a test of a program that runs `git` or
a deploy script gives the same result on every machine. Each exchange's `"key"` is a command line,
the program and its arguments separated by spaces, with an argument that is empty or holds a space,
a quote or a backslash written as a quoted string; its `"result"` is what `process.run` returns.
An exchange without a key answers every command no other names, and a command none answers fails
the test with the key to add. As for a contract, `calls(process.run)` lists the runs the fakes
answered, oldest first, each a record of `program`, `args`, `input` and `dir`:

```eye
import process

fn pull() -> Int uses {process} = match process.run("git", args: ["pull", "--ff-only"]) {
  Ok(output) => output.status
  Err(_) => -1
}

test "pull runs git once" {
  let ran = fakes(process, from: "recordings/")
  with ran
  expect(pull() == 0)
  let [git] = ran.calls(process.run) else { unreachable("not one run") }
  expect(git.args == ["pull", "--ff-only"])
}
```

```json
{ "contract": "process",
  "exchanges": [{ "op": "run", "key": "git pull --ff-only",
                  "result": { "kind": "Ok", "value": { "status": 0, "stdout": "Already up to date.\n", "stderr": "" } } }] }
```

**Tests don't reach the network.** In a test, every call to another service is answered by a
`with fakes(…)` for its module, or goes out only because `--live` names that module: a contract's
operations, `http` and `ai`. A call that neither covers fails the test at the call, and the error
names what to add:

```text
fail  an unpaid invoice gets one reminder, three days later
      main.eye:29:9: a test doesn't reach the network: mail.send has no fake here; add with
      fakes(mail, from: "recordings/"), or run eye test --live mail
```

So a test gives the same answer on every machine and never sends a real message, and a test whose
recordings don't exist yet is run once with `--live … --record` to make them.

**Live tests.** `eye test --live ai,notify` runs the tests against the real services and models of
the modules named, instead of their recordings. A `with fakes(…)` leaves those modules out, and
their calls go where `eye run` sends them: to the address and with the secret the environment
gives, such as `NOTIFY_URL` and `NOTIFY_TOKEN` for a contract, and `AI_URL`, `ANTHROPIC_API_KEY`
or `OPENAI_API_KEY` for a model (sections 17 and 19). `calls(op)` still lists the calls the live
service answered, and the fakes of the other modules keep replaying. `--live` takes `ai`, `http`
and modules that declare one contract, by path or by name.

A test that needs a live service it can't reach is skipped, not failed: when the `with` names a
contract module whose address or `@auth` secret isn't set, or `ai` while no provider is, and when
a model call's provider has no key, or a call's operation has no `@http`. `eye test` prints `skip`
and why, and a skipped test doesn't change the exit code, which is 1 only when a test failed:

```text
ok    a reminder is due once, inside its lead (2ms)
skip  the morning reads its sources, has the briefing written, and sends it
      --live ai: no model provider is set: set AI_URL, ANTHROPIC_API_KEY or OPENAI_API_KEY
11 tests passed, 1 skipped (40ms)
```

`--record`, with `--live`, writes what the live services answered as recordings, in the file the
`with fakes(…)` would have read, so the next plain `eye test` replays them. `eye test` names each
file it writes:

- **Only a test that passed writes.** A failed or skipped test writes nothing.
- **A contract's file keeps one exchange per operation and key,** the last answer the service gave
  (for a lost reply, the answer to the send that settled it, not `Uncertain`). `http.json` keeps
  one per URL. Exchanges already in the file for other operations, keys or URLs stay, so
  re-recording one test with `--run` doesn't lose what the others replay. A file is updated in
  place, so `--record` overwrites; review the change with the rest of the diff.
- **`ai.json` holds one test's replies, in order,** with what each spent and the tools it called,
  since replies are replayed in order. It replaces the file; when several tests record into one
  file, the first that passes writes it. A provider's failure or refusal isn't written.
- **Keys and times are written as they were.** A call's key is the one it was sent with, which
  comes from the test's clock and data, so the replay sends the same. A result keeps the times the
  service answered with, and nothing says when it was recorded, so recording an unchanged answer
  again leaves the file as it was.
- **A secret is never written.** A recording holds results, never the requests or headers sent,
  and the value of every secret the test read (an `@auth` secret, a `secret(…)`, a provider's key)
  is replaced by `<secret name>` wherever it appears in an answer, such as a service's error that
  repeats the token.
