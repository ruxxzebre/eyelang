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
| `at.unix()` | seconds since 1970-01-01 00:00 UTC, which many APIs take |

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
by joining the run.
