---
title: Writing Eyelang
outline: [2, 3]
---

# Writing Eyelang

This page is for coding agents, and for people in a hurry. Read it once before you write
Eyelang. It covers the syntax, the four marks the checker enforces, and the rules that programs
which wait and keep state must follow. The last section lists the errors you will meet, each with
the change that fixes it.

The [language reference](/reference/language/) states every rule in full, and wins when the two
disagree. The examples come from one
[program](https://github.com/ruxxzebre/eyelang/tree/dev/docs/writing-eyelang/code/tickets) that
`eye check` and `eye test` pass.

## The program loop

Run these three after every change. Each takes a program's directory; `eye fmt` and `eye check`
print nothing when all is well:

```sh
eye fmt DIR     # the one style; also writes each public signature's uses clause
eye check DIR   # names, types, exhaustive matches, effects, and where ! goes
eye test DIR    # every test block, each with an empty store and a fake clock
```

- An error reads `file:line:column: message`. The message says what to write; the
  [table below](#common-errors-and-their-fixes) shows the common ones.
- `eye fix DIR` adds the `!` that a durable step is missing, and prints each change.
- `eye run DIR` runs `main`, then keeps the program's jobs going until nothing is left to wait
  for. It keeps state in `DIR/.eye/journal`; `--memory` keeps nothing. `eye runs DIR` lists the
  runs and what each waits for.
- In the Eyelang repository itself, call `./bin/eye` or `go run ./cmd/eye`: direnv puts the main
  checkout's `eye` on the `PATH`.

**Layout.**

- An `eye.toml` file marks a program's root. It may be empty. Its one key is `modules`, more
  directories to find imports in: `modules = ["../.."]`.
- One directory is one module. Its `.eye` files share one namespace and one set of imports.
- `import notify` loads the directory `notify/` below the root; its public names are then
  `notify.send`, `notify.Delivery`. `import lab.notify` loads `lab/notify/` and is still used as
  `notify.…`. `import notify.{Delivery, UserId}` brings names in without the prefix.
- The standard modules `ai`, `approvals`, `fs`, `http`, `process`, `random` and `regex` need an
  `import`. `print`, `log` and `env` are built in.
- Only `pub` names leave a module. `pub opaque type` exports a type without its fields.

## Syntax at a glance

```eye
type Shape = Circle(radius: Float) | Rect(w: Float, h: Float)
type Priority = Low | Normal | High     # variants without fields
type User = { name: String, age: Int }  # a record, nominal

let dublin: Zone = "Europe/Dublin"  # module-level let, computed once

fn area(s: Shape) -> Float = match s {  # `=` and one expression
  Circle(r) => 3.14 * r * r
  Rect(w, h) => w * h
}

fn adults(users: List(User)) -> String {  # a block: its last expression is the value
  var count = 0                           # var: local and mutable
  for u in users {
    if u.age >= 18 then count += 1
  }
  let names = users |> filter(fn u: u.age >= 18) |> map(fn u: u.name) |> join(", ")
  "{count} adults: {names}"  # interpolation needs Show
}

fn older(u: User) -> User = { u with age: u.age + 1 }

fn firstName(users: List(User)) -> String {
  let Some(u) = users[0] else { return "nobody" }  # the else block must leave
  u.name
}

fn label(p: Priority) -> String = match p {
  High => "urgent"
  Low | Normal => "routine"
}
```

Literals for time and memory are part of the syntax:

```eye
let delay: Duration = 3 days + 4 h
let lunch = 12:30                       # a time of day
let launch = 2026-10-12                 # a Date
let opens = 2026-10-12 09:00 in dublin  # an Instant
let budget = 64 MiB                     # a Size
```

Tests sit next to the code, and `expect` fails a test:

```eye
test "a rectangle's area is its width times its height" {
  expect(area(Rect(w: 2.0, h: 3.0)) == 6.0)
  expect(adults([{ name: "Ann", age: 30 }, { name: "Bo", age: 9 }]) == "1 adults: Ann")
  expect(older({ name: "Bo", age: 9 }).age == 10 && firstName([]) == "nobody")
  expect(opens + delay > opens && label(High) == "urgent")
}
```

What differs from the languages you know:

- **Type arguments take parentheses:** `List(Int)`, `Map(String, Int)`, `Option(a)`,
  `Result(T, E)`. Never `<…>`. A lower-case name in a type is a type variable.
- **Lambdas:** `fn x: x + 1`, `fn (a, b): a + b`, `fn { … }` with no parameters. `return` inside a
  lambda is an error: give the lambda's value instead.
- **`if`** is `if c then a else b` for one expression, and `if c { … }` for a block.
- **There is no `if let`.** Use `match`, or `let Pattern = value else { … }`, whose block must
  leave with `return`, `break`, `continue` or `unreachable(…)`.
- **Pipes and methods are one call.** `xs |> f(a)` is `f(xs, a)`, and so is `xs.f(a)` unless
  the type has a method or a function field named `f`. `_` puts the value elsewhere:
  `2 |> sub(10, _)` is `sub(10, 2)`.
- **`name:` alone** passes the variable of that name: `send!(to:, text:, key:)`.
- **A record literal takes the type expected** from a parameter, a return type or an annotation.
  `let x = { name: "Cy", age: 5 }` with nothing to say which record is an error.
- **Collections:** `{"a": 1}` is a map, `{}` an empty map, set or record, and `toSet([a, b])` a
  set. Tuples are taken apart with `let (a, b) = pair`; there is no `.0`.
- **Strings:** `{expr}` interpolates, `\{` is a literal brace, and `"""` starts a multi-line
  string. `#` starts a comment, and `##` a doc comment.
- **Lines:** a line break ends a statement, unless the next line starts with a binary operator,
  `|>`, `.`, `?`, `,`, `:`, `=`, `->` or `else`. A line that starts with `-` or `!` is a new
  statement (only `->` and `!=` continue), so put a subtraction's `-` at the end of the line.
- **Ranges:** `0..n` leaves out `n`, and `0..=n` includes it.
- **Times** have two digits each: `07:40`, never `7:40`. Units: `ms s min h day days week weeks`,
  and `B KiB MiB GiB`.
- **`match` is exhaustive.** `_ => …` covers the rest.

## The four marks

Four marks show where code costs something or reaches outside: `!`, `mut`, `uses` and `dyn`.

### `!`: a durable step

A *job* is a function that can pause for days and survive restarts. Each *run* of it records its
*steps*, the calls marked `!`, so a resumed run gets the recorded results instead of calling
again.

- **Where `!` is required:** in a job, on every call that waits or reaches outside the program.
  - Waits: `sleep!`, `sleepUntil!`, `next!`, `approval!`, `poll!`, `reserve!`, `lock!`,
    `run.join!()`, `allDone!` and `firstDone!`.
  - Outside effects: `now!()`, `random`, `fs`, `process`, `env`, foreign functions, and calls to
    other services (`notify.send!(…)`). A secret can't be a step's result: read it inside a
    function the job calls with `!`, and use it there.
  - State: entity calls (`Ticket(id).close!()`) and reads (`t.read!()`, `overdue.read!()`,
    `Ticket.all.read!()`).
  - Publishing an event (`notify.replied.publish!(…)`) and starting a child run
    (`escalate.start!(id)`).
- **Calls that only compute take no `!`.** `double!(2)` is an error. `print` and `log.info` are
  not steps either: no `!`.
- **A waiting function** is any function with a `!` step in it, such as `tell` below. A job calls
  it with `!`, as `tell!(…)`, and its steps become the job's own. Neither `main` nor a test can
  call one: start a job that does.
- **Outside a job, a wait blocks.** In `main`, a test or a plain function, `sleep(1 s)` without
  `!` holds the caller until it returns; in a test it moves the test's clock. A job can't call a
  function that waits this way. `next`, `approval` and `reserve` work only in jobs and waiting
  functions.
- **`!` outside a job or a waiting function is an error.** In `main`, call `sleep(1 s)`.
- **Only values that can be saved live across a `!`.** A `Secret`, an `Iter`, a `dyn` value, a
  channel or an `extern type` value can't be held across one: get it again after the `!`, or use
  it inside a function called as one step.
- `f!(x)` is written with no space before `!` or `(`.

### `mut`: a change the caller can see

```eye
type Counter = { hits: Int, last: Option(Instant) }

fn record(mut c: Counter, at: Instant) {  # may change the caller's counter
  c.hits += 1
  c.last = Some(at)
}

fn tally(times: List(Instant)) -> Int {
  var counter: Counter = { hits: 0, last: None }
  for t in times {
    record(mut counter, t)  # mut at the call too
  }
  counter.hits
}
```

- A `mut` parameter is passed as `mut x`, and `x` must be a `var` or a `mut` parameter. A `let`
  never changes.
- A `mut self` method is called on a `var` without `mut`: `counter.reset()`.
- **No aliasing.** One variable can't be passed as `mut` twice in a call, or passed as `mut` and
  read in the same call.
- **Values, not references.** Assigning or passing a list, map or record never shares later
  changes. An update happens in place when nothing else refers to the value.
- **Only a callback reads a `var`.** A lambda given straight to a call, as to `filter`, may read a
  `var` but not assign it. A lambda kept in a `let`, a field or a list can't capture a `var`: copy
  the value into a `let` first.
- `m[key] = v` sets a map's key. `m[key] += v` is an error: write `m[key] = (m[key] ?? 0) + v`.
- `xs[i] = v` out of range is a fault; a list grows with `xs += [v]`.

### `uses`: the effects in a signature

- **You must write it** on a `pub fn`, a `pub fn` in an entity, and a `pub job`. A public function
  without `uses` is pure, and the checker holds it to that. Private functions and jobs that aren't
  `pub` never need one.
- **`eye fmt DIR` writes the clauses for you** from what each body does: it adds what is missing
  and drops what is gone. It does this only for a directory whose program has no other errors; a
  single file is formatted for syntax only. A private function's `uses` is a limit you set, and
  `eye fmt` leaves it as written.
- The effects: `wait`; `net Notify.send` (or `net` for every operation); `entity Ticket`;
  `start escalate`; `publish Notify.replied` and `consume Notify.replied`;
  `secret "telegram-bot-token"`; `clock`; `random`; `fs`; `process`; `stdio` (`print`); `env`;
  `ffi`. `log` is not an effect.
- A function that takes a callback lists only its own effects. The call adds the callback's.
- A function kept in a field or a list states its effects in its type:
  `run: fn(Event) uses {net Notify.send}`.

### `dyn`: a type chosen at run time

```eye
trait Sink {
  fn write(self, line: String) -> String
}

type Shout = { prefix: String }
type Tagged = { tag: String }

impl Sink for Shout {
  fn write(self, line) = "{self.prefix}{line |> upper}"
}

impl Sink for Tagged {
  fn write(self, line) = "[{self.tag}] {line}"
}

fn sinkFor(loud: Bool) -> dyn Sink {  # dyn: the type is chosen at run time
  let shout: Shout = { prefix: "! " }
  let tagged: Tagged = { tag: "log" }
  if loud then shout else tagged
}
```

- A call on a `dyn Trait` looks up the method at run time. Write `dyn` only when the type really
  differs at run time; `fn label(x: Named)` with a trait as the parameter's type is generic and
  costs nothing.
- A `dyn` value has its trait's methods and nothing else: it can't be printed, compared or held
  across a `!`.

## Errors and totality

There are no exceptions and no null. A failure that can happen is a value.

```eye
type ConfigError =
  | Unreadable(fs.FsError)
  | BadPort(String)
  | Missing(String)

fn port(path: String) -> Result(Int, ConfigError) uses {fs} {
  let text = fs.readText(path)?  # FsError: one variant holds it, so ? wraps it
  if text |> trim == "" { return Err(Missing(path)) }
  let n = parseInt(text |> trim).mapErr(BadPort)?  # two variants hold a String: say which
  Ok(n)
}
```

- **`?` returns the error** to the caller. When the caller's error type has exactly one variant
  that holds the error's type, `?` wraps it there. Otherwise convert it: `.mapErr(Variant)?` or
  `.mapErr(fn e: …)?`.
- **`?` on an `Option`** returns `None`, so it needs a function that returns an `Option`.
  Elsewhere give a default with `??`, or match it.
- **A `Result` can't be dropped.** Use it, match it, pass it on with `?`, or write
  `let _ = …` to drop it on purpose.
- **`Problem`**, from the standard library, suits programs that only report errors: `?` turns any
  error that has `Show` into one, and `.context("reading {path}")` adds a line.
- **`unreachable("why")`** stops the program, or the job's run, as a fault. Use it only for a case
  the code has already ruled out.

Every function is total, so the types say where a value may be missing:

```eye
fn third(xs: List(Int)) -> Int = xs[2] ?? 0  # indexing gives an Option

fn mean(total: Int, count: Int) -> Int = match nonzero(count) {
  Some(n) => total / n  # / takes a NonZero(Int)
  None => 0
}

fn perHundred(failed: Int, sent: Int) -> Int = (failed * 100).div(sent) ?? 0

fn nextByte(b: Byte) -> Byte = b.wrappingAdd(1)         # 255 wraps to 0
fn grow(n: UInt32) -> Option(UInt32) = n.checkedAdd(1)  # None when it doesn't fit
```

- `xs[i]`, `m[key]`, `first(xs)` and `parseInt(s)` give an `Option` or a `Result`. A slice,
  `xs[2..5]`, is clamped and never fails.
- `/` and `%` take a `NonZero(Int)`. A literal other than `0` is one; for anything else, match
  `nonzero(n)`, or use `a.div(b)` and `a.rem(b)`, which give `None` for `0`.
  `xs.average()` is an `Option(Float)`.
- `Int` has arbitrary precision and never overflows. Nothing converts implicitly: `1 + 2.0` is an
  error.
- `Byte`, `Int32`, `Int64`, `UInt32` and `UInt64` have no `+`, `-`, `*` or `/`. Their methods say
  what happens at the edge: `wrappingAdd`, `checkedAdd` (an `Option`), `and`, `or`, `xor`,
  `shiftLeft`, `toInt()`, and `Int32.from(n)`, an `Option`.

## Entities, jobs, views, events, schedules

- An **entity** is state with an identity, such as one `Ticket`. Only its own methods change its
  state, each call as one transaction. `Ticket(id)` is a reference; the entity exists from its
  first use.
- A **job** is a durable function, and each start of it is a **run**.
- A **view** is a named pipeline over entities, kept up to date as transactions commit.
- An **event** is a message a contract declares; jobs start on it or wait for it.

```eye
## A ticket: its state, changed only by its own methods, one transaction
## at a time.
entity Ticket(id: String) {
  state title: String = ""
  state open: Bool = false
  state escalated: Bool = false

  ## Opens the ticket and starts its escalation, in one transaction.
  pub fn raise(t: String) uses {start escalate} {
    if open { return }
    title = t
    open = true
    escalate.start(id, key: "escalate-{id}")
  }

  pub fn close() { open = false }

  pub fn markEscalated() { escalated = true }
}
```

```eye
## Tickets that were escalated and are still open, kept up to date as
## Ticket transactions commit.
view overdue = Ticket.all |> filter(fn t: t.open && t.escalated)
```

```eye
## A waiting function: its `!` steps are the calling job's own steps.
fn tell(to: UserId, text: String, key: Key) {
  match notify.send!(to:, text:, key:).reconcile!() {
    Accepted(_) => ()
    Refused(_) => log.warn("Notify refused {key}")
  }
}
```

```eye
## Escalates a ticket that nobody answered within four hours.
job escalate(id: String) {
  let reply = next!(notify.replied, where: fn r: r.ticket == id, within: 4 h)
  if reply != None { return }
  let ticket = Ticket(id).read!()  # a snapshot, read in a step
  if !ticket.open { return }
  tell!(oncall, "{ticket.title} has waited four hours", "escalate-{id}")
  Ticket(id).markEscalated!()
}

## A reply closes its ticket.
@on(event(notify.replied))
job closeOnReply(r: notify.Reply) {
  Ticket(r.ticket).close!()
}

## Each morning, the person on call hears how many tickets are overdue.
@on(daily(09:00, in: dublin))
job morning(day: Date) {
  let late = overdue.read!()
  if late == [] { return }
  tell!(oncall, "{late |> size} tickets are overdue", "morning-{day}")
}
```

```eye
## Raises one ticket: `eye run . -- t-1 "Disk full on db-2"`. eye run runs
## main on every start; the key in raise starts the job once.
pub fn main(args: List(String)) -> Result((), String) uses {entity Ticket, start escalate} {
  let [_, id, title] = args else { return Err("usage: eye run . -- ID TITLE") }
  Ticket(id).raise(title)
  Ok(())
}
```

The rules agents get wrong:

- **Methods can't call the network or sleep.** An entity method is a transaction, and the runtime
  may run it again after a conflict. So it can't use `net`, `fs`, `process`, `clock`, `random`,
  `env`, `ffi`, `secret` or `wait`, itself or through what it calls. It may change its state, call
  other entities' methods, start jobs and publish events, which happen only if it commits, and it
  may `print` and `log`. A method that needs the time takes it as an argument, which a job reads
  with `now!()`. A served contract operation and a view's pipeline follow the same rule.
- **Jobs read state only through `!` steps.** `Ticket(id).read!()` gives a snapshot,
  `overdue.read!()` and `Ticket.all.read!()` a list of them. `Ticket(id).title`, or a view read
  without `read!()`, is an error in a job and in a waiting function. The identity, `t.id`, is
  fine anywhere. Outside jobs, `Ticket(id).title` reads the entity directly.
- **Start jobs from `main` with a key.** `eye run` runs `main` every time it starts, so a start in
  `main` needs `key:`, which starts the run once: `escalate.start(id, key: "escalate-{id}")`. The
  rule follows calls, so it also applies to a function or method `main` calls. In a job, a start
  is a step: `escalate.start!(id)`. `after: 20 min` and `at: 07:40 in dublin` delay a start.
- **A command's outcome can't be dropped,** like a `Result`: match it, or write `let _ = …`.
- **Only a job waits for a run.** In a job, `run.join!()`. In `main` and tests, `run.outcome()`
  gives `None` while the run is going and `Some(Completed(v))` or `Some(Failed(fault))` after.
- **Triggers** are `@on(daily(07:40, in: zone))`, `weekdays(…)`, `every(15 min)`,
  `cron("0 9 * * mon-fri", in: zone)`, `enters(view)`, `leaves(view)` and
  `event(module.event)`. The job takes no parameter or one `Date` for a schedule, a `Ref(E)` for
  a view, and the event's type for an event.
- **Limits are attributes:** `@deadline(1 h)`, `@concurrency(1)`, `@limit(5, per: 1 min)`,
  `@memory(64 MiB)` and `@priority(background)`. The runtime enforces them.
- **A paused run resumes where its code still is.** If you change a job's statement where runs
  are paused, `eye run` refuses to start. Keep the old job under its name until its runs finish,
  and give the new code a new name.
- **Entities:** a state field's type must be saveable, and it can't be named `version`.
  `end(ok)` ends an entity and needs an `Approval`, which only `approval!` makes. Adding a state
  field with a default needs nothing more; renaming one, or adding one without a default, raises
  `@version(N)` and needs a `migrate from` the old version.

## Contracts

A *contract* describes another service's interface: its commands (which change something),
queries (which only read) and events. Its module's operations are called by name.

```eye
contract Notify {
  ## Sends a message to one person. Each key is sent at most once.
  @key(required)
  @reconcile(sent)
  command send(to: UserId, text: String) -> Delivery

  ## What happened to the message sent with this key, if it arrived.
  query sent(key: Key) -> Option(Delivery)

  ## Someone answered about a ticket.
  event replied: Reply on "notify.replies"
}
```

- **`@key`.** A contract decides whether its commands need a *key*, an idempotency key that stops
  a repeat from doing the work twice. A command marked `@key(required)` takes `key:` as its last
  argument. Build keys from what makes the work unique: `"escalate-{id}"`, `"morning-{day}"`.
- **`Uncertain`.** Every command can also come back `Uncertain(key)`: its reply was lost, so the
  service may or may not have acted. Until it is reconciled, or a `match` handles
  `Uncertain(_)`, the result can't be used as the declared type.
- **`.reconcile!()`** exists where the contract declares `@reconcile(query)`. It asks the query
  with the same key what happened; if the command never arrived, it sends it again with the same
  key, which can't do it twice. A service that keeps what each key did names the command itself,
  `@reconcile(send)`. A command without `@reconcile` has no `.reconcile!()`: match
  `Uncertain(_)` and decide, usually by asking a person.

  ```eye
  fn receipt(id: InvoiceId, phone: String, text: String) {
    match sms.text!(phone, text) {
      Queued(_) | BadNumber => ()
      Uncertain(_) => Invoice(id).unsure!()
    }
  }
  ```
- **Retries.** There is no retry library and no retry attribute. The runtime retries what is
  safe: a request that never reached the service is sent again for about a minute (queries,
  `http.get`, `ai.generate`, and operations that declare no failures). An operation that declares
  `http.HttpError` gets the failure at once. A lost reply to a command is `Uncertain`, settled by
  `.reconcile!()`, never blindly re-sent. A custom policy is a short waiting function: section
  15 of the spec, [Jobs and waiting](/reference/language/jobs-and-waiting), shows `retry`.
- `@fresh` on a command adds `basedOn: Version` and the outcomes `Stale` and `Expired`.
  `@destructive` adds an `approval: Approval` argument.
- `@port(n)` and `@allow(module, …)` say where a served contract listens and which modules may
  call it. `@http("POST /v1/send")` and `@auth(bearer: "secret-name")` say how it goes over HTTP.
  A caller finds the service at the URL in `NOTIFY_URL`, named after the module.
- A program serves a contract with `impl notify.Notify { fn send(to, text, key) = … }`, one
  function per operation. Each is one transaction, under the same rules as an entity method.

## Tests

```eye
test "an unanswered ticket is escalated after four hours, once" {
  let sent = fakes(notify, from: "recordings/")
  with sent
  with clock(2026-10-12 10:00 in dublin)
  Ticket("t-1").raise("Disk full on db-2")
  advance(3 h)
  expect(!Ticket("t-1").escalated)
  advance(1 h)
  expect(Ticket("t-1").escalated)
  let [message] = sent.calls(notify.send) else { unreachable("sent more or less than once") }
  expect(message.key == "escalate-t-1")
}

test "a reply closes the ticket, and nothing is sent" {
  let sent = fakes(notify, from: "recordings/")
  with sent
  Ticket("t-1").raise("Disk full on db-2")
  advance(1 h)
  notify.replied.publish({ ticket: "t-1", by: UserId("ann"), text: "on it" })
  advance(4 h)
  expect(!Ticket("t-1").open && !Ticket("t-1").escalated)
  expect(sent.calls(notify.send) == [])
}
```

- **Each test starts fresh:** an empty in-memory store, an empty directory of its own for `fs`,
  and the same random seed. `eye test` also runs the tests of the modules the program imports from
  its own directory.
- **`with clock(…)`** sets the start time; without it, the clock starts at 2026-01-01 00:00 UTC.
  `with` works only in tests, and applies to the rest of the block.
- **`advance(d)`** moves the clock. Every run that wakes on the way, and every schedule that
  fires, runs before `advance` returns, so the next line sees the result. Schedules fire only for
  jobs in the test's own module.
- A run started by a call runs as soon as the call returns, until it finishes or waits. So does a
  run that an event the test publishes starts or wakes: `notify.replied.publish(…)` stands in for
  the service. `approve("text")` and `decline("text")` answer an `approval!`.
- **`fakes(notify, from: "recordings/")`** answers the module's calls from
  `recordings/notify.json`. `ai`, `http` and `process` are faked the same way. A call that no
  exchange answers fails the test. A test without `fakes` for a service can reach the real
  service, so give every test that calls one its fakes.
- **`sent.calls(notify.send)`** lists the calls the fakes answered, oldest first, each with its
  arguments and `key`.
- A run that faults on a bug fails the test. A run that ends as a declared outcome, such as
  `Failed(DeadlinePassed)`, doesn't: check it with `run.outcome()`.

A recording is a JSON file per contract module, with the replies in the
[JSON shape](/reference/language/contracts-talking-to-other-services) the service uses:

```json
{
  "contract": "Notify",
  "exchanges": [
    { "op": "send", "key": "escalate-t-1", "result": { "kind": "Accepted", "id": "m-1" } },
    { "op": "send", "key": "escalate-t-2", "result": { "kind": "Uncertain", "key": "escalate-t-2" } },
    { "op": "sent", "key": "escalate-t-2", "result": { "kind": "Accepted", "id": "m-2" } },
    { "op": "send", "key": "morning-2026-10-13", "result": { "kind": "Accepted", "id": "m-3" } }
  ]
}
```

**How recordings are made.**

- `eye test --live notify --record DIR` runs the tests against the real service and writes what
  it answered into the file the `fakes(…)` reads. Only tests that pass write. Every secret's value
  is replaced by `<secret name>`. Review the change with the rest of the diff.
- Or write the file by hand, from a reply you saw. A lost reply is
  `{ "kind": "Uncertain", "key": "…" }`.
- The recordings are also what `eye check` trusts about a service. Reading a field that no
  recording contains is an error. Record an answer that has it, or mark the function, job, test
  or type `@unwitnessed("why")`. A pattern such as `Refused(_)` reads nothing, so it needs no mark.

## Words that mean different things

| Word | Meanings |
|---|---|
| `start` | `j.start(…)` starts a run of job `j`. `s.start()` is a string's first position, and `r.start` a range's start. |
| `end` | In an entity's own method, `end(ok)` ends the entity. Elsewhere `s.end()` is a string's end position, and `r.end` a range's end. |
| `first` | `first(xs)` is a collection's first element, an `Option`. Waiting for whichever of several runs finishes first is `firstDone!`, and for all of them `allDone!`. `first!` and `all!` are errors. |
| `join` | `xs \|> join(", ")` joins strings. `run.join!()` waits for a run's outcome, in a job. `run.join()` without `!` is an error; outside a job, read `run.outcome()`. |
| `key` | An idempotency key, `key:` on a call or `@key` on an operation. A map's keys are ordinary values. |
| `version` | A snapshot's `version` counts the transactions that changed the entity. `@version(N)` numbers the shape of its stored state. |

## Common errors and their fixes

Each message is copied as `eye check` prints it, for the code named in the second column.

| Error | What to change |
|---|---|
| `area uses fs, which its signature doesn't list: run eye fmt, or add uses {fs}` | Run `eye fmt DIR`, which writes the `uses` clause of `pub fn area`. |
| `in a job, now uses clock, so calling it is a durable step: write now!(…)` | Add `!`: `now!()`, or run `eye fix DIR`. |
| `in a job, an entity call is a durable step: write bump!(…)` | `c.bump!()`, or run `eye fix DIR`. |
| `in a job, a call to another service is a durable step: write text!(…)` | `sms.text!(…)`. |
| `in a job, c.n reads Counter without recording it, so a resumed run could see another value: read a snapshot with c.read!() and take n from it` | `let me = c.read!()`, then `me.n`. |
| `in a job, reading big isn't recorded, so a resumed run could see other entities: read it with big.read!(), a recorded step that gives a list of snapshots` | `let rows = big.read!()`. |
| `now uses clock, which an entity method can't: it runs as a transaction, which may run again after a conflict, so what reaches outside the program or waits belongs in a job, which the method can start` | Take the time as a parameter, read in a job with `now!()`, or start a job that does the outside work. |
| ``later has durable steps, which only a job records, so main can't call it: start a job that does`` | Start a job that calls `later!()`. A test gets a like error: start the job, then `advance(…)`. |
| ``pause waits through calls without !, which block outside a job, so a run couldn't record its wait: write the waiting calls in pause with !, as sleep!(…)`` | Inside `pause`, write `sleep!(…)`; it becomes a waiting function, called as `pause!()`. |
| ``double neither waits nor reaches outside the program, so it isn't a durable step: write double(…) without !`` | Drop the `!`. |
| ``eye run runs main every time it starts, so this would start ring again after each restart; give the run a key, which starts it once: ring.start(…, key: "…")`` | `ring.start("kitchen", key: "ring-kitchen")`. |
| `this drops a Result((), FsError), so a failure would go unnoticed: use it, match it, pass its error on with ?, or discard it with let _ = …` | Add `?`, match it, or `let _ = fs.writeText(…)` if a failure doesn't matter. |
| `this drops Charge, a command's outcome, which may be Uncertain, so a refusal would go unnoticed: match it, or discard it with let _ = …` | Match the outcome, usually after `.reconcile!()`. |
| `this command's reply may have been lost, so it may be Uncertain(key): reconcile it with .reconcile!(), or match it with an Uncertain(_) arm` | `payments.charge!(…).reconcile!()`, or add an `Uncertain(_)` arm. |
| `this command names no @reconcile query, so its Uncertain outcome can't be reconciled: match Uncertain(key) instead` | Remove `.reconcile!()` and match `Uncertain(_)`. |
| `` this match doesn't handle Waking: add an arm for it, or `_ => …` `` | Add the arm. Prefer naming each variant to `_`, so a new variant is caught. |
| `` this pattern doesn't match None: add `else { … }` for what it doesn't match `` | `let Some(n) = x else { return … }`. |
| `` which record type is this? Nothing here says; write it where the value is bound, as `let x: T = …` `` | `let x: User = { … }`. |
| `bump changes its argument n, so the call says so: write mut n` | `bump(mut n)`. |
| `n can't change: it is a let or a parameter; declare it with var, or as a mut parameter` | `var n = 1`. |
| `a closure can't capture seen, a var, unless it is a callback given straight to a call; copy it into a let first` | `let current = seen`, and capture `current`. |
| `dividing by Int fails when it is 0, so / and % take a NonZero(Int): match nonzero(count), which is Some(n) unless it is 0, or divide by a literal other than 0` | `match nonzero(count) { … }`, or `total.div(count) ?? 0`. |
| `Option(Int) + Int isn't defined: Option(Int) doesn't implement Add(Int, …)` | Indexing gives an `Option`: `(xs[0] ?? 0) + 1`. |
| `? on an Option returns its None from next, but next returns Int: give a default with ?? value, match the None, or return an Option` | `(x ?? 0) + 1`, or make the function return an `Option`. |
| `a test can't wait for a run: read where it got to with run.outcome(), which is None while the run is still going (only a job waits, with join!())` | `advance(…)`, then `expect(run.outcome() == Some(Completed(…)))`. |
| `` token is a Secret, which can't be saved, and it is still needed after a waiting call: get it again after the `!` `` | Read and use the secret inside one function that the job calls with `!`. |
| `undefined name fs` | `import fs`. |
| `expected an expression, found 'let'` | There is no `if let`: use `match` or `let … else`. |
| `return inside a lambda would leave only the lambda; give the lambda's value instead` | `fn x: if x < 0 then 0 else x`. |
