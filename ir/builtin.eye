# The built-ins: the types and functions every module has without importing
# anything. The interpreter implements them; this file declares them, with
# their documentation. The checker checks each call against its declaration,
# and its `uses` clause gives the call's effects; `uses {wait}` makes it a
# durable step in a job. Tools read it too: hover, Go to Declaration and
# `eye doc`. Programs never import it (SPECS.md section 18).
#
# The functions one signature can't describe yet, such as the collection
# functions, are checked by rules in check/builtintypes.go (goRules in
# check/builtindecls.go); their declarations here document them.
#
# A line `# == Group` starts a group of the reference. An `@host("log.info")`
# names a function of a built-in module.

# == Types

## A whole number of any size: arithmetic never overflows.
pub extern type Int

## A number that isn't 0: what `/` and `%` take as a divisor, so dividing can't fail. `nonzero(n)`
## makes one.
pub extern type NonZero(a)

## A 64-bit floating-point number.
pub extern type Float

## `true` or `false`.
pub extern type Bool

## One Unicode character: `'a'`, `'\n'`.
pub extern type Char

## Text, in UTF-8. Strings interpolate with `{…}`: `"{name} has {n} items"`.
pub extern type String

## A whole number from 0 to 255, for binary formats; arithmetic on it is explicit.
pub extern type Byte

## A sequence of bytes, for binary data, hashing and foreign functions.
pub extern type Bytes

## A 32-bit whole number, for binary formats and foreign functions; arithmetic on it is explicit:
## `a.wrappingAdd(b)`, `a.checkedAdd(b)`.
pub extern type Int32

## A 64-bit whole number, for binary formats and foreign functions; arithmetic on it is explicit.
pub extern type Int64

## A 32-bit whole number without a sign, for binary formats and foreign functions.
pub extern type UInt32

## A 64-bit whole number without a sign, for binary formats and foreign functions.
pub extern type UInt64

## A moment in time, the same everywhere: `2026-10-10 07:40 in dublin`.
pub extern type Instant

## A day of the calendar, without a time or a zone: `2026-10-10`.
pub extern type Date

## A time of day, without a date or a zone: `07:40`. A range of times may wrap midnight:
## `23:00..08:00`.
pub extern type Time

## A length of time: `20 min`, `3 days`.
pub extern type Duration

## A time zone, by its IANA name: `"Europe/Dublin"`.
pub extern type Zone

## When a job runs by the clock: `daily(07:40, in: dublin)`, `every(15 min)`.
pub extern type Schedule

## An amount of memory: `64 MiB`.
pub extern type Size

## A growable array. Like every collection it is a value: passing one never shares later changes.
pub extern type List(a)

## Keys and their values, in the order the keys were added.
pub extern type Map(k, v)

## Values without duplicates.
pub extern type Set(a)

## A lazy sequence: `xs.iter()` gives one whose pipelines build no lists in between, and `toList`
## collects it.
pub extern type Iter(a)

## The values from `start` up to `end`: `0..n` leaves `end` out and `0..=n` includes it.
pub extern type Range(a)

## A reference to an entity: an identity, not a pointer, so it can be stored, sent and saved, and
## a `Ref(World)` can't be confused with a `Ref(Player)`.
pub extern type Ref(e)

## A started run of a job: `run.join!()` waits for its `Outcome`.
pub extern type Run(a)

## An idempotency key: a command sent again with the same key happens once.
pub extern type Key

## A value that can't be printed or saved, such as a token: `secret("notify-token")`.
pub extern type Secret(a)

## A person's approval, which only `approval!(…)` gives: ending an entity, or a `@destructive`
## command, needs one.
pub extern type Approval

## The version of something another service keeps, which a `@fresh` command checks is still
## current.
pub extern type Version

## An error with its context, for programs that only report errors: `?` turns any error with
## `Show` into one, and `.context("reading {path}")` adds a line.
pub extern type Problem

## An object foreign code owns, which can't be saved (SPECS.md section 18).
pub extern type Handle(name)

## Something a contract's service publishes: jobs wait for one with `next!(…)` or start on one
## with `@on(event(…))`.
pub extern type Event(a)

## A running task group, as `taskGroup(fn group { … })` gives it: `group.spawn(fn { … })` starts
## another task, which the group waits for. It can't be saved, so it can't be live across a `!`.
pub extern type TaskGroup

## The end of a channel that puts: `send.put(x)` waits while the channel is full, and
## `send.close()` ends it, after which a put fails. It can't be saved.
pub extern type Sender(a)

## The end of a channel that takes: `for x in receive { … }` reads until the channel is closed and
## empty, and `receive.take()` gives the next value, or `None` once it is closed and empty. It
## can't be saved.
pub extern type Receiver(a)

## What the collection functions take: any of List, Set, Map, Range, Option, and String where it
## makes sense. It stands for those here; programs write the collection's own type.
pub extern type Collection(a)

## What `with` applies in a test, such as `clock(…)` or `fakes(…)`.
pub extern type Handler

## A module named as a value, as `fakes(notify, from: …)` takes it.
pub extern type Module

## What starts a job in `@on(…)`: a schedule, `enters(view)`, `leaves(view)` or `event(…)`.
pub extern type Trigger

## A view named as a value, as `enters(view)` takes it.
pub extern type View

# == Output and failure

## Writes the values, shown as string interpolation shows them and separated by spaces, and a line
## break to standard output. It takes any number of values, written as separate arguments:
## `print("took", d)`.
pub extern fn print(values: List(a)) uses {stdio}

## Stops the program, or the job's run, as a fault: `Failed(Unreachable(why))`. It is for states
## the code has already ruled out, never for failures that can happen.
pub extern fn unreachable(why: String) -> a

## Logs a line at debug level. A job that resumes doesn't log again what it logged before its last
## step.
## It takes any number of values, written as separate arguments.
@host("log.debug")
pub extern fn debug(values: List(a))

## Logs a line at info level.
## It takes any number of values, written as separate arguments.
@host("log.info")
pub extern fn info(values: List(a))

## Logs a line at warning level.
## It takes any number of values, written as separate arguments.
@host("log.warn")
pub extern fn warn(values: List(a))

## Logs a line at error level.
## It takes any number of values, written as separate arguments.
@host("log.error")
pub extern fn error(values: List(a))

# == Tests

## Fails the test if the condition is false. For a comparison it shows the value it got: `expected
## w.phase == Asleep, but w.phase is Running`.
pub extern fn expect(condition: Bool)

## Moves the test's clock forward. Each job run that wakes on the way runs, in the order of its
## wake time, before `advance` returns.
pub extern fn advance(by: Duration)

## Used as `with clock(2026-10-10 12:00 in "Europe/Dublin")`: sets the test's clock. Without it, a
## test's clock starts at 2026-01-01 00:00 UTC.
pub extern fn clock(at: Instant) -> Handler

## In a test, approves the request a job is waiting for in `approval!(…)` whose text contains what.
pub extern fn approve(what: String, by: who = "test")

## In a test, declines the request a job is waiting for in `approval!(…)` whose text contains what.
pub extern fn decline(what: String, by: who = "test")

## Used as `with fakes(notify, from: "receipts/")`: calls to the contract modules replay the
## exchanges recorded in `receipts/notify.json`, checked against the contract's types.
## It takes any number of contract modules, written as separate arguments.
pub extern fn fakes(modules: List(Module), from: String) -> Handler

# == Time

## In a job, `sleep!(20 min)` is a durable step: the run waits without holding memory, and resumes
## when the time comes, even after a restart. Outside jobs, `sleep` blocks; in tests it advances
## the clock.
pub extern fn sleep(for: Duration) uses {wait}

## In a job, `sleepUntil!(08:00 in dublin)` waits, without holding memory, until that time comes.
## Outside jobs it blocks; in tests it advances the clock.
pub extern fn sleepUntil(at: Instant) uses {wait}

# == Configuration

## The secret of that name: the environment variable of the name in upper case, with `-` and `.` as
## `_` (`notify-token` is NOTIFY_TOKEN), or the file of that name in the directory $EYE_SECRETS. It
## can't be shown or saved.
pub extern fn secret(name: String) -> Secret(String)

## The environment variable of that name, if it is set: for configuration that isn't secret, such
## as a proxy's address.
@host("env.get")
pub extern fn get(name: String) -> Option(String) uses {env}

# == Time

## Every day at that time on the zone's clock: `@on(daily(07:40, in: dublin))`. A time a
## daylight-saving change skips fires as far after the change as it was meant to be; a time it
## repeats fires the first time. A job that takes a Date gets the firing's date in the zone.
pub extern fn daily(at: Time, in: Zone) -> Schedule

## Like daily, Monday to Friday.
pub extern fn weekdays(at: Time, in: Zone) -> Schedule

## At every multiple of the interval since midnight UTC: `@on(every(15 min))` fires at :00, :15,
## :30 and :45.
pub extern fn every(interval: Duration) -> Schedule

## The current time: the test's clock in tests. In a job, `now!()` records it, so a resumed run
## sees the same time.
pub extern fn now() -> Instant uses {clock}

# == Concurrency

## Applies f to each element in tasks that run at once, at most limit of them, and returns the
## results in the elements' order. Tasks that wait, for the network or a timer, overlap. In a job,
## f may only compute.
pub extern fn parallelMap(c: Collection(a), f: fn(a) -> b, limit: Int = all) -> List(b)

## Runs a and b at once and returns both results when both have finished.
pub extern fn both(a: fn() -> x, b: fn() -> y) -> (x, y)

## Runs the functions at once and returns the result of the first to finish; the others are
## cancelled at their next wait.
pub extern fn race(fs: List(fn() -> a)) -> a

## Runs f, and returns `Some(result)` if it finishes in time, or `None`, cancelling it at its next
## wait, if it doesn't.
pub extern fn timeout(after: Duration, f: fn() -> a) -> Option(a)

## Runs f with a new task group, and returns f's result once every task the group spawned has
## finished. If a task fails, the others are cancelled at their next wait and the group fails with
## it; if every task left waits for a channel that no task will put into or close, the group fails.
pub extern fn taskGroup(f: fn(TaskGroup) -> a) -> a

## A channel that holds at most capacity values, as a `Sender` and a `Receiver`. A put waits while
## it is full, so a fast producer can't fill memory. Channels aren't durable: jobs talk through
## entities and events.
pub extern fn channel(capacity: Int) -> (Sender(a), Receiver(a))

# == Waiting

## In a job, `poll!(every: 1 min, within: 6 h, fn { miniflux.ready!() })` calls check every
## interval until it returns true, and returns false if `within` runs out first. For a service that
## can't send an event when it is ready. Only the outcome is kept, so a long poll doesn't grow the
## run's history.
pub extern fn poll(every: Duration, within: Duration, check: fn() -> Bool) -> Bool uses {wait}

## In a job, waits for the first event that matches, counting every one published since the run's
## previous step, so an answer that comes quickly isn't missed. `None` when `within` runs out.
pub extern fn next(event: Event(a), where: fn(a) -> Bool, within: Duration) -> Option(a) uses {wait}

## In a job, asks a person to approve, and waits for the answer: `Approved(approval)`, `Declined`
## or `Expired`. Only this can make an `Approval`.
pub extern fn approval(what: String, from: who, within: Duration) -> Decision uses {wait}

# == Collections

## The number of elements.
pub extern fn size(c: Collection(a)) -> Int

## Whether there are no elements.
pub extern fn isEmpty(c: Collection(a)) -> Bool

## The set with x in it; on a list, the list with x at the end.
pub extern fn add(s: Set(a), x: a) -> Set(a)

## The set without x; on a map, the map without the key x.
pub extern fn remove(s: Set(a), x: a) -> Set(a)

## Whether x is an element; for a map, a key; for a string, a substring.
pub extern fn contains(c: Collection(a), x: a) -> Bool

## The map with key set to value.
pub extern fn insert(m: Map(k, v), key: k, value: v) -> Map(k, v)

## The value for key, or `None`; on a list, the element at an index. Same as `m[key]`.
pub extern fn get(m: Map(k, v), key: k) -> Option(v)

## The first element, or `None`.
pub extern fn first(c: Collection(a)) -> Option(a)

## The last element, or `None`.
pub extern fn last(c: Collection(a)) -> Option(a)

## Applies f to each element. On an Option or a Result, applies f to the value inside `Some` or
## `Ok`.
pub extern fn map(c: Collection(a), f: fn(a) -> b) -> List(b)

## The elements for which keep is true; a set stays a set. Over `Entity.all` in a view, the runtime
## keeps the result up to date.
pub extern fn filter(c: Collection(a), keep: fn(a) -> Bool) -> List(a)

## The first element for which test is true.
pub extern fn find(c: Collection(a), test: fn(a) -> Bool) -> Option(a)

## Whether test is true for some element.
pub extern fn any(c: Collection(a), test: fn(a) -> Bool) -> Bool

## Whether test is true for every element.
pub extern fn all(c: Collection(a), test: fn(a) -> Bool) -> Bool

## How many elements test is true for; without test, the size.
pub extern fn count(c: Collection(a), test: fn(a) -> Bool) -> Int

## Combines the elements, from start, left to right.
pub extern fn reduce(c: Collection(a), start: b, f: fn(b, a) -> b) -> b

## Adds the elements: numbers, durations or sizes.
pub extern fn sum(c: Collection(a)) -> a

## Pairs each element with its index, from 0.
pub extern fn enumerate(c: Collection(a)) -> List((Int, a))

## Everything after the first n elements.
pub extern fn drop(c: Collection(a), n: Int) -> List(a)

## The first n elements.
pub extern fn take(c: Collection(a), n: Int) -> List(a)

## The elements in reverse order.
pub extern fn reverse(c: Collection(a)) -> List(a)

## The elements in ascending order; equal elements keep their order.
pub extern fn sort(c: Collection(a)) -> List(a) where a: Ord

## The keys, in the order they were added.
pub extern fn keys(m: Map(k, v)) -> List(k)

## The values, in the order their keys were added.
pub extern fn values(m: Map(k, v)) -> List(v)

## The key and value pairs.
pub extern fn entries(m: Map(k, v)) -> List((k, v))

## The elements as a list.
pub extern fn toList(c: Collection(a)) -> List(a)

## The elements as a set, without duplicates.
pub extern fn toSet(c: Collection(a)) -> Set(a)

## The shown elements, with separator between them.
pub extern fn join(c: Collection(a), separator: String) -> String

## Applies f to each element and joins the results.
pub extern fn flatMap(c: Collection(a), f: fn(a) -> Collection(b)) -> List(b)

## Pairs elements of a and b, as far as the shorter goes.
pub extern fn zip(a: Collection(a), b: Collection(b)) -> List((a, b))

# == Strings and numbers

## The lines of s, without their line breaks.
pub extern fn lines(s: String) -> List(String)

## The parts of s between separators.
pub extern fn split(s: String, separator: String) -> List(String)

## s without whitespace at either end.
pub extern fn trim(s: String) -> String

## s with its letters in lower case.
pub extern fn lower(s: String) -> String

## Whether s starts with prefix.
pub extern fn startsWith(s: String, prefix: String) -> Bool

## s with its letters in upper case.
pub extern fn upper(s: String) -> String

## v as JSON, in the shape its type's schema describes (SPECS.md §19).
pub extern fn toJson(v: a) -> String where a: Schema

## Whether r is `Ok`.
pub extern fn isOk(r: Result(a, e)) -> Bool

## Whether r is `Err`.
pub extern fn isErr(r: Result(a, e)) -> Bool

## r with f applied to its error: `parse(s).mapErr(BadNumber)?` wraps the error in the caller's
## variant.
pub extern fn mapErr(r: Result(a, e), f: fn(e) -> f) -> Result(a, f)

## Reads a whole number written in decimal.
pub extern fn parseInt(s: String) -> Result(Int, String)

## `Some(n)` unless n is 0: what `/` and `%` take as a divisor.
pub extern fn nonzero(n: Int) -> Option(NonZero(Int))

## The smaller of a and b.
pub extern fn min(a: t, b: t) -> t where t: Ord

## The larger of a and b.
pub extern fn max(a: t, b: t) -> t where t: Ord

## n without its sign.
pub extern fn abs(n: t) -> t

# == Triggers

## In `@on(enters(view))`: starts one run of the job for each entity that begins to match the view.
pub extern fn enters(view: View) -> Trigger

## In `@on(leaves(view))`: starts one run of the job for each entity that stops matching the view.
pub extern fn leaves(view: View) -> Trigger
