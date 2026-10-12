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

## A whole number from 0 to 255, for binary formats; arithmetic on it is explicit:
## `b.wrappingAdd(1)`. An ASCII character literal is a Byte where one is expected: `'-'`.
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

## A lazy sequence: `xs.iter()` gives one, and `map`, `filter`, `take`, `drop`, `enumerate`, `zip`
## and `flatMap` on it give another without building lists in between. Each walk computes it
## again from its source; `toList` collects it. It can't be saved, compared or printed.
pub extern type Iter(a)

## The values from `start` up to `end`: `0..n` leaves `end` out and `0..=n` includes it.
pub extern type Range(a)

## A reference to an entity: an identity, not a pointer, so it can be stored, sent and saved, and
## a `Ref(World)` can't be confused with a `Ref(Player)`.
pub extern type Ref(e)

## A started run of a job. In a job, `run.join!()` waits for its `Outcome`; a test or `main` reads
## it with `run.outcome()`, an `Option(Outcome(a))` that is `None` while the run is still going.
pub extern type Run(a)

## An idempotency key: a command sent again with the same key happens once.
pub extern type Key

## A value that can't be printed or saved, such as a token: `secret("notify-token")`.
pub extern type Secret(a)

## A person's approval, which only `approval!(…)` gives: ending an entity, or a `@destructive`
## command or function, needs one. `ok.what` is the text that was approved and `ok.by` who approved it, the
## `from:` of `approval!` as text, for audit logs.
pub extern type Approval

## The version of something another service keeps, which a `@fresh` command checks is still
## current.
pub extern type Version

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

## What the collection functions take: any of List, Set, Map, Range, Iter, Bytes, Option, and String
## where it makes sense. It stands for those here; programs write the collection's own type.
pub extern type Collection(a)

## What `with` applies in a test, such as `clock(…)`.
pub extern type Handler

## What `fakes(…)` gives: `with` applies it, as a Handler, and `calls` lists the calls it
## answered.
pub extern type Fakes

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

## In a test, approves the one request a job is waiting for in `approval!(…)` whose text contains
## what, and fails the test when none or several do. The answer comes from by, or without it from
## the person asked; an answer from anyone else is refused, and the run keeps waiting.
pub extern fn approve(what: String, by: who = "the person asked")

## In a test, declines the one request a job is waiting for in `approval!(…)` whose text contains
## what, as `approve` approves it.
pub extern fn decline(what: String, by: who = "the person asked")

## Used as `with fakes(notify, from: "receipts/")`: calls to the contract modules replay the
## exchanges recorded in `receipts/notify.json`, checked against the contract's types.
## It takes any number of contract modules, written as separate arguments. Named first, as
## `let sent = fakes(notify, from: …)` and `with sent`, it lists the calls it answered:
## `sent.calls(notify.send)`, each a record of the call's arguments and its key.
pub extern fn fakes(modules: List(Module), from: String) -> Fakes

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
## can't be shown or saved. Reading one uses `secret "name"`, and in a job is a step: `secret!(…)`.
pub extern fn secret(name: String) -> Secret(String) uses {secret}

## The environment variable of that name, if it is set: for configuration that isn't secret, such
## as a proxy's address.
@host("env.get")
pub extern fn get(name: String) -> Option(String) uses {env}

# == Time

## Every day at that time on the zone's clock: `@on(daily(07:40, in: dublin))`. A job that takes
## a Date gets the firing's date in the zone. A time a daylight-saving change skips fires as far
## after the change as it was meant to be, unless `onGap: Skip`; a time it repeats fires the
## first time, unless `onOverlap: Second` or `Both`.
pub extern fn daily(at: Time, in: Zone, onGap: GapPolicy = Shift, onOverlap: OverlapPolicy = First) -> Schedule

## Like daily, Monday to Friday.
pub extern fn weekdays(at: Time, in: Zone, onGap: GapPolicy = Shift, onOverlap: OverlapPolicy = First) -> Schedule

## When the five fields of a crontab line say, on the zone's clock:
## `@on(cron("*/20 7-9 * * mon-fri", in: dublin))`. The fields are minute, hour, day of month,
## month (or `jan` to `dec`) and day of week (0 to 7, 0 and 7 being Sunday, or `sun` to `sat`),
## each `*`, a value, a range, a list or a step. When both day fields are given, a day matches if
## either does. Daylight-saving changes are handled as daily's are.
pub extern fn cron(fields: String, in: Zone, onGap: GapPolicy = Shift, onOverlap: OverlapPolicy = First) -> Schedule

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
pub extern fn next(event: Event(a), where: fn(a) -> Bool, within: Duration) -> Option(a) uses {wait, consume}

## In a job, `reserve!(256 MiB, fn { summarise!(items) })` waits until the runner can grant the
## memory, holds it while f runs, even across f's own waits, and gives it back when f returns,
## with f's result. Under pressure, runs of a higher `@priority` are granted memory first.
pub extern fn reserve(size: Size, f: fn() -> a) -> a uses {wait}

## In a job, asks a person to approve, and waits for the answer: `Approved(approval)`, `Declined`
## or `Expired`. Only this can make an `Approval`.
pub extern fn approval(what: String, from: who, within: Duration) -> Decision uses {wait}

## In a job, `lock!("ci", for: 30 min, fn { deploy!() })` waits until no other run holds the lock
## of that name, takes it for that long, runs f and gives the lock back when f returns, with f's
## result. The store keeps who holds a lock and until when. A run still waiting inside f when the
## time runs out ends as `Failed(LockExpired(name))`, and the next run takes the lock.
pub extern fn lock(name: String, for: Duration, f: fn() -> a) -> a uses {wait}

## In a job, `allDone!([a, b])` waits until every run in the list has finished, and gives their
## outcomes in the list's order.
pub extern fn allDone(runs: List(Run(a))) -> List(Outcome(a)) uses {wait}

## In a job, `firstDone!([a, b])` waits until one of the runs has finished, and gives its outcome.
## It stops the others, which end as `Failed(Killed)`.
pub extern fn firstDone(runs: List(Run(a))) -> Outcome(a) uses {wait}

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
## `Ok`. On an Iter, gives an Iter that applies f as it is walked; f then only computes.
pub extern fn map(c: Collection(a), f: fn(a) -> b) -> List(b)

## The elements for which keep is true; a set stays a set, and an Iter an Iter. Over `Entity.all` in
## a view, the runtime keeps the result up to date.
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

## The mean of the numbers, an Int or a Float each, as a Float, or `None` when there are none:
## `[1, 2].average()` is `Some(1.5)`. For an Int that rounds as `/` does, write
## `xs.sum().div(xs.size())`.
pub extern fn average(c: Collection(a)) -> Option(Float)

## Pairs each element with its index, from 0; on an Iter, as it is walked.
pub extern fn enumerate(c: Collection(a)) -> List((Int, a))

## Everything after the first n elements; on an Iter, an Iter that skips them.
pub extern fn drop(c: Collection(a), n: Int) -> List(a)

## The first n elements; on an Iter, an Iter that stops after them.
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

## The elements as a list: what collects an Iter.
pub extern fn toList(c: Collection(a)) -> List(a)

## The elements as a set, without duplicates.
pub extern fn toSet(c: Collection(a)) -> Set(a)

## The shown elements, with separator between them.
pub extern fn join(c: Collection(a), separator: String) -> String

## Applies f to each element and joins the results; on an Iter, as it is walked.
pub extern fn flatMap(c: Collection(a), f: fn(a) -> Collection(b)) -> List(b)

## Pairs elements of a and b, as far as the shorter goes; an Iter when either is one.
pub extern fn zip(a: Collection(a), b: Collection(b)) -> List((a, b))

## A lazy sequence of the elements, an [Iter]: `xs.iter() |> map(f) |> take(3)` applies f to three
## elements, whatever the size of xs. A map gives its (key, value) pairs, a string its characters.
pub extern fn iter(c: Collection(a)) -> Iter(a)

# == Strings and numbers

## The lines of s, without their line breaks: a line ends at `\n` or `\r\n`, and a line break at
## the end adds no empty line, so `"a\r\nb\n".lines()` is `["a", "b"]` and `"".lines()` is `[]`.
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

## Whether s ends with suffix.
pub extern fn endsWith(s: String, suffix: String) -> Bool

## s with every occurrence of old replaced by new, from left to right: `"a-b-c".replace("-", "+")`
## is `"a+b+c"`.
pub extern fn replace(s: String, old: String, new: String) -> String

## The words of s: the parts between runs of whitespace, with none empty. `"  a  b\n"` has the
## words `["a", "b"]`.
pub extern fn words(s: String) -> List(String)

## The characters of s, in order, as an [Iter]: `for c in s.chars() { … }`.
pub extern fn chars(s: String) -> Iter(Char)

## The bytes of s in UTF-8, for scanning text byte by byte: `src[i] == Some('"')`.
pub extern fn bytes(s: String) -> Bytes

## The text the bytes spell in UTF-8, or `None` if they aren't UTF-8.
pub extern fn toText(b: Bytes) -> Option(String)

## The bytes of a list, in order, as `Bytes`: for bytes that aren't text, built or changed one at a
## time. `toBytes([0xC3, 0xA9]).toText()` is `Some("é")`.
pub extern fn toBytes(xs: List(Byte)) -> Bytes

## Where the first part at or after from starts, or `None` if there is none. Without from, it
## looks from the start.
pub extern fn indexOf(s: String, part: String, from: Offset = start) -> Option(Offset)

## The part of s between two positions: `s.slice(s.start()..i)`. A range past either end is cut to
## fit; it is `None` when an end would split a character, as an [Offset] from another string may.
pub extern fn slice(s: String, range: Range(Offset)) -> Option(String)

## The position before the first character of s.
pub extern fn start(s: String) -> Offset

## The position after the last character of s: `s.slice(i..s.end())` is the rest of s from i.
pub extern fn end(s: String) -> Offset

## The position by characters after at, or `None` past the end: `s.after(i)` steps over the
## character at i.
pub extern fn after(s: String, at: Offset, by: Int = 1) -> Option(Offset)

## The position by characters before at, or `None` before the start.
pub extern fn before(s: String, at: Offset, by: Int = 1) -> Option(Offset)

## The character that starts at at, or `None` at the end or inside a character.
pub extern fn charAt(s: String, at: Offset) -> Option(Char)

## The character's Unicode code point: `'A'.code()` is 65.
pub extern fn code(c: Char) -> Int

## The character with that Unicode code point, or `None` if there is none.
pub extern fn fromCode(n: Int) -> Option(Char)

## v as JSON, in the shape its type's schema describes (SPECS.md §17). A
## NaN or an infinity, which JSON has no number for, is a fault.
pub extern fn toJson(v: a) -> String where a: Schema

## Whether r is `Ok`.
pub extern fn isOk(r: Result(a, e)) -> Bool

## Whether r is `Err`.
pub extern fn isErr(r: Result(a, e)) -> Bool

## r with f applied to its error: `parse(s).mapErr(BadNumber)?` wraps the error in the caller's
## variant.
pub extern fn mapErr(r: Result(a, e), f: fn(e) -> f) -> Result(a, f)

## r with its error turned into a [Problem] that says what was being done:
## `fs.readText(path).context("reading {path}")?`. On a Problem, it adds the line before the
## others.
pub extern fn context(r: Result(a, e), what: String) -> Result(a, Problem) where e: Show

## Reads a whole number written in decimal, or in another base from 2 to 36: `parseInt("ff",
## base: 16)` is 255. Spaces around it are ignored; the error says what was wrong.
pub extern fn parseInt(s: String, base: Int = 10) -> Result(Int, String)

## Reads a number written in decimal, with an optional fraction and exponent: `"1.5"`, `"-2"`,
## `"6.02e23"`. Spaces around it are ignored; the error says what was wrong.
pub extern fn parseFloat(s: String) -> Result(Float, String)

## The whole number nearest to x, as a Float, with halves away from zero: `round(2.5)` is `3.0`.
## [toInt] makes it an Int.
pub extern fn round(x: Float) -> Float

## The largest whole number that isn't more than x, as a Float.
pub extern fn floor(x: Float) -> Float

## The smallest whole number that isn't less than x, as a Float.
pub extern fn ceil(x: Float) -> Float

## The whole part of x, without its fraction, as an Int: `x.round().toInt()`. An infinity or NaN
## is `None`. On a fixed-width integer, `x.toInt()` is its value as an Int, which always fits,
## so it isn't an Option.
pub extern fn toInt(x: Float) -> Option(Int)

## The number n as a Float: the nearest one when n is too large to hold exactly.
pub extern fn toFloat(n: Int) -> Float

## `Some(n)` unless n is 0: what `/` and `%` take as a divisor.
pub extern fn nonzero(n: Int) -> Option(NonZero(Int))

## `Some(a / b)`, or `None` when b is 0, for a divisor that may be 0 without a `match` on
## [nonzero]: `total.div(count) ?? 0`. It rounds down, as `/` does: `div(-7, 2)` is `Some(-4)`.
pub extern fn div(a: Int, b: Int) -> Option(Int)

## `Some(a % b)`, or `None` when b is 0. The remainder has the sign of b, as with `%`:
## `rem(-7, 2)` is `Some(1)`.
pub extern fn rem(a: Int, b: Int) -> Option(Int)

## The smaller of a and b.
pub extern fn min(a: t, b: t) -> t where t: Ord

## The larger of a and b.
pub extern fn max(a: t, b: t) -> t where t: Ord

## n without its sign.
pub extern fn abs(n: t) -> t where t: Neg(t)

# == Fixed-width integers

## a + b wrapped into their type, modulo 2 to the power of its bits: for Bytes, 255 plus 1 is 0.
## a and b have the same fixed-width type: Byte, Int32, Int64, UInt32 or UInt64.
pub extern fn wrappingAdd(a: n, b: n) -> n

## a - b wrapped into their type, modulo 2 to the power of its bits: for Bytes, 0 minus 1 is 255.
pub extern fn wrappingSub(a: n, b: n) -> n

## a * b wrapped into their type, modulo 2 to the power of its bits, as hashes multiply.
pub extern fn wrappingMul(a: n, b: n) -> n

## `Some(a + b)`, or `None` when the sum doesn't fit their type.
pub extern fn checkedAdd(a: n, b: n) -> Option(n)

## `Some(a - b)`, or `None` when the difference doesn't fit their type: for Bytes, `0` minus `1`.
pub extern fn checkedSub(a: n, b: n) -> Option(n)

## `Some(a * b)`, or `None` when the product doesn't fit their type.
pub extern fn checkedMul(a: n, b: n) -> Option(n)

## The bits set in both a and b.
pub extern fn and(a: n, b: n) -> n

## The bits set in a, in b, or in both.
pub extern fn or(a: n, b: n) -> n

## The bits set in a or in b but not in both.
pub extern fn xor(a: n, b: n) -> n

## Every bit of a flipped: for a signed type, `-a - 1`.
pub extern fn not(a: n) -> n

## The bits of a moved up by `by`; bits moved past the top are lost, so `by` of the type's width
## or more gives 0. A negative `by` is a fault.
pub extern fn shiftLeft(a: n, by: Int) -> n

## The bits of a moved down by `by`, keeping the sign of a signed type: Int32 -8 shifted by 1 is
## -4. A negative `by` is a fault.
pub extern fn shiftRight(a: n, by: Int) -> n

## `Some(n)` as a Byte when n is from 0 to 255, otherwise `None`: `Byte.from(n) ?? 0`.
@host("Byte.from")
pub extern fn from(n: Int) -> Option(Byte)

## `Some(n)` as an Int32 when n is from -2147483648 to 2147483647, otherwise `None`.
@host("Int32.from")
pub extern fn from(n: Int) -> Option(Int32)

## `Some(n)` as an Int64 when it fits in 64 bits with a sign, otherwise `None`.
@host("Int64.from")
pub extern fn from(n: Int) -> Option(Int64)

## `Some(n)` as a UInt32 when n is from 0 to 4294967295, otherwise `None`.
@host("UInt32.from")
pub extern fn from(n: Int) -> Option(UInt32)

## `Some(n)` as a UInt64 when n is from 0 to 18446744073709551615, otherwise `None`.
@host("UInt64.from")
pub extern fn from(n: Int) -> Option(UInt64)

# == Triggers

## In `@on(enters(view))`: starts one run of the job for each entity that begins to match the view.
pub extern fn enters(view: View) -> Trigger

## In `@on(leaves(view))`: starts one run of the job for each entity that stops matching the view.
pub extern fn leaves(view: View) -> Trigger
