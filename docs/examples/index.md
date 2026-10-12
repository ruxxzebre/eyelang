---
title: Examples
---

# Examples

Each example below is a whole program for a problem most backend developers have met: a reminder
before a trial ends, webhooks to a partner who is sometimes down, an uptime check, a morning
report, a refund that needs a person's yes. Each shows a different thing Eyelang does for you, and
each has tests that pass. The tests run days of waiting in milliseconds, and they answer calls to
other services from recorded replies instead of the network.

Open any of them in the [playground](/playground) and press **Test** to run its tests in your
browser. The source of each is in
[docs/examples](https://github.com/ruxxzebre/eyelang/tree/dev/docs/examples).

A few words used below:

- An **entity** is a piece of state with one owner, such as one account. Its data changes only
  through its own methods, each a transaction.
- A **job** is a function that can wait for minutes or days. Each step marked `!` is saved, so a run
  that waits is kept on disk, not in memory, and carries on after a restart.
- A **view** is a list of entities kept up to date as they change. A job can start when an entity
  enters or leaves it.
- A **contract** describes another service's API: its types and operations. A **key** on a call
  lets the service see a repeat and act once.

## Trial ending: an email three days before

A sign-up gets a 14-day trial and one email three days before it ends, unless they upgrade first.
Nothing polls a `trial_ends_at` column: the job sleeps until the time comes. Because a trial can be
extended while the job sleeps, the job reads the account again each time it wakes. An upgrade ends
the run, and an extension sends it back to sleep.

<<< ./trial-ending/main.eye#remind

The tests sign up, move the clock with `advance(5 days)`, upgrade or extend, and check what was
sent. [Open it in the playground](/playground#example=examples/trial-ending).

## Webhook delivery: keys, lost replies and backoff

The shop tells a partner about each order event. Three things can go wrong, and each has its
answer. An event queued twice is delivered once, because the delivery job starts once per key. A
reply lost on the way back is `Uncertain`, and `.reconcile!()` asks the partner what happened
instead of sending again. A partner that is down is tried again on a schedule the program writes
for itself. Eyelang has no retry library: the policy is a list of waits and a short waiting
function, and each `sleep!` in it is a saved step.

<<< ./webhook-delivery/main.eye#retry

<<< ./webhook-delivery/main.eye#deliver

The tests swap the partner's recorded replies in the middle of a test, from "down" to "up", to
check that the delivery gets through at 09:36 and not before.
[Open it in the playground](/playground#example=examples/webhook-delivery).

## Uptime monitor: probe every minute, alert once

Every minute each site gets an HTTP GET. Two failures in a row put the site in the view `down`.
The team hears once when a site enters the view and once when it leaves, so there is no
"already alerted" flag to keep in step. `@concurrency(1)` stops a slow round of probes from piling
up behind the next.

<<< ./uptime-monitor/main.eye#monitor

The tests fake `http.get` from recordings of a site that is up and one that answers 503, and check
that ten failed probes give one message.
[Open it in the playground](/playground#example=examples/uptime-monitor).

## Daily report: 08:00 in New York, every weekday

At 08:00 New York time on weekdays, the warehouse team hears how many paid orders wait to ship. The
schedule names its zone, so the report stays at 08:00 when the clocks change, with no second
schedule and no check for summer time. The post's key names the day, so a restart never posts the
report twice.

<<< ./daily-report/main.eye#report

The test runs from a Friday to the Monday after the clocks go back, and checks that Monday's report
comes at 13:00 UTC, an hour later than Friday's.
[Open it in the playground](/playground#example=examples/daily-report).

## Refund approval: a person says yes first

Support asks for a refund, finance has a day to answer, and then the head of finance has a day.
The payment provider's `refund` is `@destructive`, so it takes an `Approval`, and only
`approval!(…)` makes one. A refund without a person's yes doesn't compile. Only the person asked
can answer, and the run waits on disk for the two days, through restarts.

<<< ./refund-approval/main.eye#review

The tests answer as finance, as someone who wasn't asked, and not at all.
[Open it in the playground](/playground#example=examples/refund-approval).

## Log summary: the plain language

Not every program waits. This one reads a web server's access log and prints a table of requests,
failures and latency per path. It shows the core language on its own: records and variant types,
errors as values with `Result` and `?`, a generic `percentile` that works on anything that can be
ordered, a trait for table rows, and pipes. A line it can't read is reported with its line number,
and the rest are still counted.

<<< ./log-summary/main.eye#table

Press **Run** in the playground to see the table for a short sample log.
[Open it in the playground](/playground#example=examples/log-summary).

## A shop and its bank: two programs, one contract

The shop charges customers through the bank's `Payments` contract, and the bank, a second program
in `bank/`, serves it. In the playground, **Run** starts both and shows each call as it crosses
from one to the other. The same order charged twice is paid once, because the key names the order.

<<< ./shop-and-bank/main.eye#checkout

[Open it in the playground](/playground#example=examples/shop-and-bank).

## More

- The [tutorial](/tutorial/) builds a program step by step, from values to a job that waits.
- The [guide](/guide/) explains each part of the language with a program of its own.
