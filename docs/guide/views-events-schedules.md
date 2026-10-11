---
title: Views, events and schedules
---

# Views, events and schedules

Jobs don't only start when code calls them. A disk fills up, a message arrives, it's 08:30 on a
Monday. The usual ways to react are a cron job that scans a table every minute, a consumer loop on
a message queue, and a timer that someone has to keep in step with daylight saving. Each needs its
own bookkeeping so that it doesn't miss a change, or act on one twice.

In Eyelang, a job says what starts it with `@on(...)`:

- `enters(view)` and `leaves(view)`: an entity starts or stops matching a query.
- `event(contract.event)`: another service sends an event.
- `daily(...)`, `weekdays(...)` and `every(...)`: a schedule on a clock in a time zone.

This page shows each one, and `next!`, which waits for an event inside a run. It assumes you know
[entities](/guide/entities) and [jobs](/guide/jobs-and-waiting).

The code on this page is a [real program](https://github.com/ruxxzebre/eyelang/tree/dev/docs/guide/code/views-events-schedules)
that the test suite checks and runs. It watches disks: a monitoring service sends readings, a view
finds the disks that are nearly full, and jobs page someone and wait for an answer.

## Views

Each disk is an entity with its last reading:

<<< ./code/views-events-schedules/main.eye#disk

A **view** is a named query over an entity set, which the runtime keeps up to date as transactions
commit:

<<< ./code/views-events-schedules/main.eye#view

The view starts from `Disk.all` and filters it with an ordinary function. A view must start from an
entity set, since that's what the runtime watches:

```text
main.eye:43:1: a view is a pipeline that starts from an entity set, such as World.all
```

### Triggers: `enters` and `leaves`

`@on(enters(view))` starts one run for each entity that begins to match the view, and
`@on(leaves(view))` one for each entity that stops matching. The job gets a `Ref` to the entity:

<<< ./code/views-events-schedules/main.eye#triggers

`d.host` reads the disk's identity, which never changes. To read its state, the job takes a
snapshot with `d.read!()`, as any job does.

A disk that goes from 93% to 95% is still in the view, so it starts nothing new. It is paged once
when it fills up, and settled once when it is cleaned:

<<< ./code/views-events-schedules/main.eye#test-view

This replaces a cron job that runs every few minutes, looks for rows over a threshold, and keeps a
"paged" flag so it doesn't page twice. The view sees each change as its transaction commits, and
starts exactly one run for it.

## Events

An **event** is a message another service sends, declared in its contract with a type and a topic:

<<< ./code/views-events-schedules/monitor/monitor.eye#contract

`@on(event(monitor.readings))` starts one run for each event, with the event as its argument:

<<< ./code/views-events-schedules/main.eye#record

`@key(id)` names the field that identifies an event. A message queue may deliver the same message
twice; with a key, a repeat starts nothing:

<<< ./code/views-events-schedules/main.eye#test-events

### Publishing events

A program publishes an event in one of three places:

- **In an entity method.** `Disk.acknowledge` calls `monitor.acks.publish(...)`, and the event is
  sent when the method's transaction commits. If the transaction rolls back, nothing is sent.
- **In a job,** as a durable step, so it's sent once:

  ```eye
  monitor.acks.publish!({ host, by: "bot" })
  ```

- **In a test,** to stand in for the other service, as the tests above do. Runs that the event
  starts or wakes go before `publish` returns.

### Waiting for an event

`next!` waits, inside a run, for the first event that matches. `page` above waits up to an hour
for an acknowledgement of its host:

```eye
let ack = next!(monitor.acks, where: fn a: a.host == host, within: 1 h)
```

It returns `Some(event)`, or `None` when the hour runs out, and the `match` after it handles both.

`next!` misses nothing. It counts every matching event published since the run's previous step, so
an answer that arrives while the run is still sending its question, or while it sleeps, is found.
Code that subscribes only after it has asked can't lose a fast reply.

## Schedules

A schedule starts a run on a clock in a named time zone:

<<< ./code/views-events-schedules/main.eye#morning

<<< ./code/views-events-schedules/main.eye#compact

- `daily(at, in: zone)` fires every day at that time on the zone's clock, and `weekdays(at, in:
  zone)` Monday to Friday.
- `every(15 min)` fires at each multiple of the interval since midnight UTC: :00, :15, :30, :45.
- A job that takes a `Date` gets the date of the firing in the schedule's zone, which makes a good
  key for anything the run sends.

The zone is part of the schedule, and is checked when the program is compiled:

```text
main.eye:105:22: unknown time zone "Europe/Dubln": a zone is named as in the tz database, such as "Europe/Dublin"
```

A schedule follows its zone through daylight saving. A time that a change skips fires as far after
the change as it was meant to be: on the night Dublin's clock jumps from 01:00 to 02:00, `01:30`
fires at 02:30. A time that a change repeats fires the first time it comes. So there are never two
UTC crons and an hour check:

<<< ./code/views-events-schedules/main.eye#test-schedules

A schedule counts from when the program first starts, and each firing is saved with the run it
starts. A restart neither repeats a firing nor skips one: if the program was down when firings were
due, the schedule fires once, for the latest. A firing that finds the job's `@concurrency` limit
reached is skipped, not queued (see [Limits](/guide/jobs-and-waiting#limits)).

## Mistakes this prevents

| The bug | Why it can't happen |
|---|---|
| A threshold checked by polling misses a change, or acts on it twice | A view starts one run when an entity enters, as the change commits |
| A redelivered message is handled twice | An event's `@key` starts one run per key |
| An event is sent for a change that rolled back | An event published in a method goes out when the transaction commits |
| A reply arrives before the code listens for it | `next!` counts events since the run's previous step |
| A schedule runs twice, or not at all, at a daylight-saving change | Schedules run on the zone's clock, with set rules for skipped and repeated times |
| A firing is lost, or repeated, by a restart | Each firing is saved with its run |

::: info Not yet
A view can only be used in `@on(enters(...))` and `@on(leaves(...))` for now: reading one as a
collection, such as `nearlyFull |> map(...)`, passes the checker but fails at run time, so
`morning` filters `Disk.all` with the same function instead. `cron("…", in: zone)` is designed but
not built, and so are the `onGap:` and `onOverlap:` policies. In `eye run`, nothing outside the
program can publish an event into it yet. Views over entities run in memory; compiling them to SQL
comes with the Postgres store in v0.2. See [What runs today](/reference/status).
:::

## See also

- [Views and triggers](/reference/language/views-and-triggers) and
  [Jobs and waiting](/reference/language/jobs-and-waiting) in the reference: the triggers and what
  each passes to the job.
- [Time, secrets, tests](/reference/language/time-secrets-tests): schedules, zones and daylight
  saving.
- [Talking to other services](/guide/contracts): contracts, where events are declared.
- [Testing with time and recordings](/guide/testing): publishing events and moving the clock in
  tests.
