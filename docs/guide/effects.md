---
title: Effects
---

# Effects

A function's signature usually says what it takes and what it returns, but not what it does on the
way: whether it reads the clock, calls another service, or changes stored data. That is what a
reviewer most needs to know, and it is easy to hide. A helper three calls down starts posting to a
chat service, and the diff shows one new line in a file nobody opened.

In Eyelang the compiler works out what each function does, its **effects**, and every public
function states them with `uses`. A new network call, a new service, or a read of the clock changes
a signature, and the change shows in review.

The code on this page is a [real program](https://github.com/ruxxzebre/eyelang/tree/dev/docs/guide/code/effects):
a shop's orders, a rule for when an order is late, and a job that tells the team about late orders
every evening. Its tests run with `eye test docs/guide/code/effects`.

[Open it in the playground](/playground#example=guide/effects) to run it, test it and change it in your browser.

## The effects

| Effect | Comes from |
|---|---|
| `wait` | A `!` call: a step that may pause, such as `sleep!`, `next!` or a call to a waiting function; and outside a job, a call that blocks, such as `sleep(1 s)` |
| `net Chat.post` | Calling operation `post` of contract `Chat` |
| `net` | Calling a service without a contract: `http.get`, `ai.generate` |
| `entity Order` | Reading or changing entities of type `Order` |
| `publish Bus.pinged` | Publishing event `pinged` of contract `Bus`: `bus.pinged.publish(…)` |
| `consume Bus.pinged` | Waiting for that event: `next!(bus.pinged, …)` |
| `start digest` | Starting a run of job `digest`: `digest.start(day)` |
| `secret "chat-token"` | Reading a secret: `secret("chat-token")` |
| `ffi` | Calling an `extern fn` of the program, which runs foreign code |
| `clock` | `now()` |
| `stdio` | `print` |
| `env` | `env.get(…)` |

A broader name covers the narrower ones: `net Chat` covers every operation of `Chat`, and `net`
covers every call to another service; `publish` covers every event, `start` every job, and
`secret` every secret. Prefer the narrow name. `uses {net Chat.post}` tells a reader exactly which
operation a function can call; `uses {net}` tells them almost nothing.

Logging (`log.info` and the rest) isn't an effect.

## Public functions state them

A public function without `uses` is **pure**: it computes its result from its arguments and does
nothing else. Public entity methods and public jobs are held to the same rule: a method that
starts a job says `uses {start …}`, and one that calls another entity `uses {entity …}`. The
compiler holds them to that:

<<< ./code/effects/main.eye#pure

If `isLate` read the clock itself, by calling `now()` instead of taking `at`, the checker would
stop it:

```text
main.eye:27:1: isLate uses clock, which its signature doesn't list: run eye fmt, or add uses {clock}
```

Taking the time as an argument is the better design anyway: a test can pass any time it likes, and
the function can be used for "late as of last Friday" as well as "late now". Push effects to the
edges and keep the rules pure.

A function that does read the clock and the orders says so:

<<< ./code/effects/main.eye#uses

Leave one out and the checker names it:

```text
main.eye:33:1: lateOrders uses entity Order, which its uses clause doesn't list: run eye fmt, or add it
```

You rarely write these clauses yourself. `eye fmt` on the program's directory checks the program
and writes each public function's `uses` from what its body does: it adds what is missing and drops
what the body no longer does. A file formatted on its own, as an editor does on save, is formatted
for syntax only, so run `eye fmt .` before you commit. A program with other errors keeps its
clauses as they are until those errors are fixed.

## Private functions are inferred

Inside a module, effects are worked out for you. A private function never writes `uses`; its
effects are whatever its body does, and whatever the functions it calls do. The first public
function on the path states them all:

<<< ./code/effects/main.eye#inferred

`post` calls `chat.post!`, so its effects are `wait` and `net Chat.post`. (Its
`@unwitnessed("…")` is about the recordings, not effects: no recording has Chat refusing a post,
so reading `Refused(error)` needs the mark, as [Testing](/guide/testing#what-the-recordings-show) explains.) `tellLate` calls `post`,
so it has the same effects, and states them. If someone later makes `tellLate` read the channel's
history first, through `post` or directly, the signature has to change:

```text
main.eye:60:1: tellLate uses net Chat.history, which its uses clause doesn't list: run eye fmt, or add it
```

`eye fmt` writes the new clause, and the change shows in the diff, where a reviewer sees it:

```diff
-pub fn tellLate(late: List(OrderId), key: Key) uses {wait, net Chat.post} {
+pub fn tellLate(late: List(OrderId), key: Key) uses {net Chat.history, net Chat.post, wait} {
```

This is the split to aim for: inference inside a module, where the code is read together, and
stated effects at public boundaries, where other code depends on them. Editors show the inferred
effects too: hovering over `post` lists `wait` and `net Chat.post` as its effects, as inferred.

## Callbacks pass their effects to the caller

A function that takes a function states only its own effects. Calling it adds the effects of the
function passed in, where the call is written:

<<< ./code/effects/main.eye#callback

`eachLate` reads the clock and the orders, and that is all its signature says. The `print` is in
the lambda that `printLate` passes, so `stdio` is `printLate`'s effect. Leave it out and the
checker puts it where it belongs:

```text
main.eye:75:1: printLate uses stdio, which its uses clause doesn't list: run eye fmt, or add it
```

That is how `map`, `filter`, `parallelMap` and `timeout` stay pure while working with any function,
and why every public signature still lists everything its body can do.

A function kept in data, in a record's field, a list or a result, may run long after it was
given, so its effects can't pass to a caller. Its type states them instead, as in
`run: fn(Event) uses {net Notify.send}`, and calling it has those effects. A function type without
`uses` kept in data is pure, and storing a function that does more is an error at the place it is
stored:

```text
main.eye:8:35: loud uses stdio, which its type here, fn(Int) -> Int, doesn't list: a function kept in data does only what its type says, so add uses {stdio} to the type
```

## Waiting, and `!`

`wait` is the effect of a function that can pause: for a timer, an event, a person, or a reply from
another service. Every such call is written with `!`, so a reader can find every point where code
can stop or touch the outside world by looking for `!`.

In a **job**, each `!` is a step whose result is saved before the job goes on. If the program
restarts, the job resumes after the last saved step, and steps already taken aren't taken again:

<<< ./code/effects/main.eye#job

A function that waits is called with `!` too. Call `tellLate` without it and the checker says:

```text
main.eye:87:3: tellLate waits, so calling it is a durable step: write tellLate!(…)
```

In a job, every call that reaches outside the program is a step as well, and is written with `!`:
a call with an outside effect (`now!()`, `lateOrders!()`, `chat.post!(…)`, an `extern fn`), an
entity call (`Order(id).flag!()`), which commits in the same transaction as the job's checkpoint,
publishing an event (`bus.pinged.publish!(…)`) and starting a child run (`digest.start!(day)`).
Leave the `!` out and the checker says which:

```text
main.eye:5:3: in a job, publishing is a durable step: write publish!(…)
```

Output isn't a step: `print` and `log.info` are written without `!` in a job too.

Waiting calls can appear only where waiting makes sense:

| Where | Waiting calls | What the checker says otherwise |
|---|---|---|
| A job | Yes, each one a saved step | |
| Any other function | Yes; it becomes a waiting function, which only jobs and other waiting functions can call | |
| An entity method | No: a method is a transaction, which stays inside the program | `an entity method is a transaction and can't wait; start a job for this instead` |
| Serving a contract's operation | No: it is a transaction | `serving an operation is a transaction and can't wait; start a job for this instead` |
| `main` | No: `main` isn't durable | `` `!` marks a durable step, and only jobs have them; in main, call it without `!` `` |
| A test | No | `a test can't make waiting calls; start a job, then advance(…) the clock` |

A waiting function can't be called from `main` or a test at all, with `!` or without. Start a job
that does the waiting instead. A test then moves its clock with
`advance(…)`, as the program's test does with the evening job. See
[Jobs and waiting](/guide/jobs-and-waiting).

Outside a job, `sleep(1 s)` without `!` blocks: `main` waits for a second, and a test's clock
moves by one. That still waits, so the call keeps `wait` in the caller's effects, and so does a
call of a function that blocks: `pub fn pause() uses {wait} { sleep(1 s) }`. A job can't call
`pause`, with `!` or without, since its run couldn't record that wait:

```text
pause waits through calls without !, which block outside a job, so a run couldn't record its wait: write the waiting calls in pause with !, as sleep!(…)
```

## Starting jobs

Starting a job is the effect `start J`, named after the job. In a job, a function that starts one
is a step, written with `!`, so a resumed run doesn't start it again. And `main`, which runs every
time `eye run` starts, may start a job only with a key, which starts it once: the checker follows
the calls `main` makes, entity methods included, and refuses one that starts a job without a key:

```text
eye run runs main every time it starts, and kick starts ring without a key, so each restart would start it again; give that start a key, which starts it once: ring.start(…, key: "…")
```

## What can't be held across a `!`

A job's state is saved at every `!`, so everything it still needs after a `!` must be something
that can be saved. A `Secret` or a value of an `extern type`, which foreign code owns, can't. The
same goes for a waiting function, whose variables, parameters included, are part of the run's
state:

```text
main.eye:95:11: token is a Secret, which can't be saved, and it is still needed after a waiting call: get it again after the `!`
```

A function is saved as its code and what it captured, so a lambda written in the job, or a
function declared with `fn`, can be held across a `!` as long as what it captures can be saved.
A waiting function can keep a callback it is given, such as `retry!(3, fn: ping(host))`; the
call is an error when the lambda captures what can't be saved:

```text
main.eye:14:64: retry keeps call across a waiting call, so the function given for it is saved with the run, and this one can't be, since it captures token, which is a Secret: give it a function that captures only what can be saved
```

A job's arguments are saved too:

```text
main.eye:93:1: hold's argument token is a Secret, which can't be saved, and a job's arguments are saved with its run: pass what can be saved instead, and get the rest in the job
```

A step's result is saved too, so a step can't return one of them. In a job, read a secret, or use
a foreign value, inside a function that returns what can be saved, and call that function as one
step:

```text
main.eye:9:11: this durable step's result is recorded, and a Secret can't be saved: use it in a function that returns what can be, and call that with !
```

Better still, let a contract's `@auth` send the secret, so the code never holds it at all (see
[Talking to other services](/guide/contracts#over-http)).

## Reading effects in review

When you review a change, the effects tell you where to look:

- A public signature that gains an effect is a change in what the code is allowed to do. Ask
  whether it should.
- A pure public function stays pure: if it compiles without `uses`, it touches nothing.
- Every `!` in a job is a point where the job can pause, restart, or call out. A new `!` is a new
  step; check that what it does is safe to have saved.
- `net C.op` names the exact operations a function can call. A new one means a new dependency on
  another service.

## See also

- [Effects and signatures](/reference/language/effects-and-signatures), the exact rules.
- [Jobs and waiting](/guide/jobs-and-waiting): what a `!` step saves, and how a run resumes.
- [Talking to other services](/guide/contracts): where `net C.op` comes from.
- [All together: approvals](/tutorial/approvals): effects in a larger program.
