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

## The effects

| Effect | Comes from |
|---|---|
| `wait` | A `!` call: a step that may pause, such as `sleep!`, `next!` or a call to a waiting function |
| `net Chat.post` | Calling operation `post` of contract `Chat` |
| `net` | Calling a service without a contract: `http.get`, `ai.generate` |
| `entity Order` | Reading or changing entities of type `Order` |
| `clock` | `now()` |
| `stdio` | `print` |
| `env` | `env.get(…)` |

A broader name covers the narrower ones: `net Chat` covers every operation of `Chat`, and `net`
covers every call to another service. Prefer the narrow name. `uses {net Chat.post}` tells a reader
exactly which operation a function can call; `uses {net}` tells them almost nothing.

Logging (`log.info` and the rest), publishing events and starting jobs aren't effects.

## Public functions state them

A public function without `uses` is **pure**: it computes its result from its arguments and does
nothing else. The compiler holds it to that:

<<< ./code/effects/main.eye#pure

If `isLate` read the clock itself, by calling `now()` instead of taking `at`, the checker would
stop it:

```text
main.eye:27:1: isLate uses clock, which its signature doesn't list: add uses {clock}
```

Taking the time as an argument is the better design anyway: a test can pass any time it likes, and
the function can be used for "late as of last Friday" as well as "late now". Push effects to the
edges and keep the rules pure.

A function that does read the clock and the orders says so:

<<< ./code/effects/main.eye#uses

Leave one out and the checker names it:

```text
main.eye:33:1: lateOrders uses entity Order, which its uses clause doesn't list
```

## Private functions are inferred

Inside a module, effects are worked out for you. A private function never writes `uses`; its
effects are whatever its body does, and whatever the functions it calls do. The first public
function on the path states them all:

<<< ./code/effects/main.eye#inferred

`post` calls `chat.post!`, so its effects are `wait` and `net Chat.post`. `tellLate` calls `post`,
so it has the same effects, and states them. If someone later makes `tellLate` read the channel's
history first, through `post` or directly, the signature has to change:

```text
main.eye:59:1: tellLate uses net Chat.history, which its uses clause doesn't list
```

So the change shows in the diff, where a reviewer sees it:

```diff
-pub fn tellLate(late: List(OrderId), key: Key) uses {wait, net Chat.post} {
+pub fn tellLate(late: List(OrderId), key: Key) uses {wait, net Chat.post, net Chat.history} {
```

This is the split to aim for: inference inside a module, where the code is read together, and
stated effects at public boundaries, where other code depends on them.

## Callbacks pass their effects to the caller

A function that takes a function states only its own effects. Calling it adds the effects of the
function passed in, where the call is written:

<<< ./code/effects/main.eye#callback

`eachLate` reads the clock and the orders, and that is all its signature says. The `print` is in
the lambda that `printLate` passes, so `stdio` is `printLate`'s effect. Leave it out and the
checker puts it where it belongs:

```text
main.eye:74:1: printLate uses stdio, which its uses clause doesn't list
```

That is how `map`, `filter`, `parallelMap` and `timeout` stay pure while working with any function,
and why every public signature still lists everything its body can do.

## Waiting, and `!`

`wait` is the effect of a function that can pause: for a timer, an event, a person, or a reply from
another service. Every such call is written with `!`, so a reader can find every point where code
can stop or touch the outside world by looking for `!`.

In a **job**, each `!` is a step whose result is saved before the job goes on. If the program
restarts, the job resumes after the last saved step, and steps already taken aren't taken again:

<<< ./code/effects/main.eye#job

A function that waits is called with `!` too. Call `tellLate` without it and the checker says:

```text
main.eye:86:3: tellLate waits, so calling it is a durable step: write tellLate!(…)
```

In a job, an entity call is a step as well (`Order(id).flag!()`): it commits in the same
transaction as the job's checkpoint.

Waiting calls can appear only where waiting makes sense:

| Where | Waiting calls | What the checker says otherwise |
|---|---|---|
| A job | Yes, each one a saved step | |
| Any other function | Yes; it becomes a waiting function, which only jobs and other waiting functions can call | |
| An entity method | No: a method is a transaction | `an entity method is a transaction and can't wait; start a job for this instead` |
| Serving a contract's operation | No: it is a transaction | `serving an operation is a transaction and can't wait; start a job for this instead` |
| `main` | No: `main` isn't durable | `` `!` marks a durable step, and only jobs have them; in main, call it without `!` `` |
| A test | No | `a test can't make waiting calls; start a job, then advance(…) the clock` |

A waiting function can't be called from `main` or a test at all, with `!` or without. Start a job
that does the waiting instead. A test then moves its clock with
`advance(…)`, as the program's test does with the evening job. See
[Jobs and waiting](/guide/jobs-and-waiting).

::: info Not yet
The checker asks for `!` on calls of waiting functions, on entity calls in jobs, and on `sleep`.
It doesn't yet ask for it on other calls in a job that have an outside effect, such as `now()`,
`lateOrders()` or a contract call written as `chat.post(…)`, although the
[rule](/reference/language/effects-and-signatures) says they are steps too. Write the `!`:
`lateOrders!()`, `now!()`.
:::

## What can't be held across a `!`

A job's state is saved at every `!`, so everything it still needs after a `!` must be something
that can be saved. A `Secret`, a function or a foreign `Handle` can't:

```text
main.eye:94:11: token is a Secret, which can't be saved, and it is still needed after a waiting call: get it again after the `!`
```

A job's arguments are saved too:

```text
main.eye:92:1: hold's argument token is a Secret, which can't be saved, and a job's arguments are
```

Read a secret where you use it, after the last wait, or better, let a contract's `@auth` send it,
so the code never holds it at all (see [Talking to other services](/guide/contracts#over-http)).

## Reading effects in review

When you review a change, the effects tell you where to look:

- A public signature that gains an effect is a change in what the code is allowed to do. Ask
  whether it should.
- A pure public function stays pure: if it compiles without `uses`, it touches nothing.
- Every `!` in a job is a point where the job can pause, restart, or call out. A new `!` is a new
  step; check that what it does is safe to have saved.
- `net C.op` names the exact operations a function can call. A new one means a new dependency on
  another service.

## Not yet

- `secret S`, `publish T`, `consume T` and `ffi` can be written in `uses`, but they aren't
  inferred yet (open question 19 in the spec). `fs`, `process` and `random` have no functions yet.
- A function type kept in data can state its effects (`run: fn(Event) uses {net Notify.send}`),
  but calling it doesn't check them yet.
- Editors don't show the inferred effects of private functions yet.

## See also

- [Effects and signatures](/reference/language/effects-and-signatures), the exact rules.
- [Jobs and waiting](/guide/jobs-and-waiting): what a `!` step saves, and how a run resumes.
- [Talking to other services](/guide/contracts): where `net C.op` comes from.
- [All together: approvals](/tutorial/approvals): effects in a larger program.
