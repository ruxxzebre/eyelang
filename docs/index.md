---
layout: home
title: Eyelang
titleTemplate: false

hero:
  name: Eyelang
  text: A typed language for programs that outlive their process.
  tagline: Jobs wait for days, survive restarts and send each request once, by key. No queue, no cron, no retry library, and tests that run days in milliseconds.
  actions:
    - theme: brand
      text: Quickstart
      link: /quickstart
    - theme: alt
      text: Playground
      link: /playground
    - theme: alt
      text: Why Eyelang
      link: /why
    - theme: alt
      text: Tutorial
      link: /tutorial/

---

## A job that waits

An invoice goes out, and three days later the customer gets one reminder, unless they have paid by
then. The invoice is an *entity*: a record with an identity, kept in the program's store and
changed only by its own methods. Issuing it starts a *job*, a function that runs in the background
and can pause:

<<< ./quickstart/code/remind/main.eye#invoice

<<< ./quickstart/code/remind/main.eye#job

Every call that ends in `!` is a durable *step*: it runs once, and its result is saved. So
`sleep!(3 days)` holds no thread, and the run carries on after a restart. The *key* on
`mail.send!` makes a second send with the same key do nothing, and when a reply is lost,
`.reconcile!()` asks the mail service what happened instead of sending again. `mail` is a
*contract*: a typed description of another service's operations.

## The compiler knows it's a job

A job that is resumed after a restart must see the values it saw before. Change
`Invoice(id).read!()` to `Invoice(id)`, so the job reads the invoice directly instead of through a
recorded step, and the program no longer compiles:

```text
main.eye:26:6: in a job, invoice.paid reads Invoice without recording it, so a resumed run could see another value: read a snapshot with invoice.read!() and take paid from it
```

Workflow engines find this mistake when a run replays. In Eyelang it doesn't build.

## Three days, tested in milliseconds

The test moves a fake clock, and the mail service answers from recorded replies:

<<< ./quickstart/code/remind/main.eye#test

The [quickstart](/quickstart) runs this program on your machine in a few minutes, and the
[playground](/playground) runs it in your browser.

## What you no longer write

- **The queue.** A job is started from the code that needs it, in the same transaction, and its
  runs are kept in the program's own store.
- **The cron table.** `@on(daily(07:40, in: zone))` on a job, daylight saving included.
- **The `next_run_at` column.** `sleep!(3 days)` or `sleepUntil!(until)` in the job itself.
- **The polling loop.** `next!(answered, within: 6 h)` waits for the event, and gives `None` when
  the time runs out.
- **The hand-made idempotency table.** A `key:` on a job start never starts a second run. On a
  command, the key goes to the service, which keeps what each key did, as most payment and
  messaging APIs do.
- **The retry wrapper.** For about a minute, the runtime repeats what is safe to repeat, such as a
  request that never reached the service. A lost reply to a command is `Uncertain` and is settled
  with `.reconcile!()`, never sent blindly again. An operation that declares its errors gets them
  at once, and a policy of your own is a short waiting function.
- **The test that sleeps or mocks the clock.** `advance(3 days)` moves a fake clock, and
  `fakes(mail, from: "recordings/")` replays recorded replies, so a test gives the same answer
  every time.

## What you get

- **Waiting is one line.** `sleep!`, `next!(event, within: …)` and `@on(daily(…))` replace the
  queue and the cron. A waiting run is a record on disk; the process keeps a handle of about 140
  bytes for it.
- **Replays can't go wrong.** Reading stored state in a job outside a step, or holding a value
  that can't be saved across one, is a compile error.
- **State has one owner.** Only an entity's own methods change its data, one transaction at a
  time, with no locks.
- **Outside calls are typed and settle once.** Contracts type every call, keys stop repeats, and a
  lost reply is a value the code must handle.
- **Days of behaviour test in milliseconds.** A fake clock and recorded replies; the approvals
  tutorial's eight tests take about 5 ms.

Underneath is a small, strict language: no null and no exceptions (failure is a `Result`), values
instead of shared objects, exhaustive `match`, and effects in public signatures. The
[tour](/reference/language/a-tour) shows all of it on one page, and [Why Eyelang](/why) explains
each promise.

::: info What runs today
The reference interpreter runs nearly all of the language, entities, jobs and contracts included.
`eye build` compiles the core language to a native binary; programs with jobs run in the
interpreter until native code covers them. The language changes whenever real programs show a
better way, until v1.0. [What runs today](/reference/status) lists each feature.
:::
