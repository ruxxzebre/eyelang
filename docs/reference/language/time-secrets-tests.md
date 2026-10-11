---
title: "Time, secrets, tests"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Time, secrets, tests

**Time.** Schedules carry their zone: `daily(07:40, in: "Europe/Dublin")`, and `weekdays(…)` is the
same from Monday to Friday. `every(15 min)` fires at each multiple of its interval since midnight
UTC: :00, :15, :30, :45. A time that a daylight-saving change skips fires as far after the change
as it was meant to be (`01:30` on the night the clock jumps from 01:00 to 02:00 fires at 02:30),
and a time a change repeats fires the first time it comes, so there are never two UTC crons and
an hour check. (The policies are to be choosable, as `onGap:` and `onOverlap:`; these two are the
defaults and the only ones so far.) A schedule counts from when the program first starts; each
firing is saved with the run it starts, so a restart neither repeats nor skips one. Dates in
another zone are asked for by name, never taken from the host's clock:

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
`approve("delete 3 backups")` or `decline(…)` answers the `approval!` whose text contains it, and
fails the test when several do.
`expect(cond)` fails the test, and for a comparison shows the value it got:
`expected w.phase == Asleep, but w.phase is Running`. A run that stops on a bug (`unreachable`, a
fake with no recorded answer) fails the test too; one that ends as an outcome the program
declared, such as `Failed(DeadlinePassed)` from its `@deadline`, doesn't, and the test checks it
by joining the run.
