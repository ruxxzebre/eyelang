---
title: "Effects and signatures"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Effects and signatures

The compiler infers what each function touches. Public signatures state it with `uses`, so a new
permission always shows up in the diff.

```eye
pub fn summarise(items: List(Item)) -> String uses {wait, net Llm.complete} {
  llm.complete!(prompt(items))
}
```

| Effect | Comes from | What the compiler does with it |
|---|---|---|
| `wait` | any `!` call | Compiles the function to a resumable state machine; allowed only in jobs and waiting functions |
| `net C.op` | calling an operation on contract `C` | Derives network policy, credentials and allowed-caller checks |
| `publish C.event`, `consume C.event` | publishing contract `C`'s event (`bus.pinged.publish!(…)`), or waiting for it or starting on it (`next!(bus.pinged)`, `@on(event(bus.pinged))`) | Derives message-queue permissions |
| `entity E` | reading or changing entities of type `E` | Database grants; includes the type in backups |
| `secret "name"` | reading a secret, `secret("telegram-bot-token")` | Delivers that secret only to code that uses it |
| `clock`, `random` | `now()`, `random.int(1..=6)` | Lets tests replace them |
| `fs`, `process`, `stdio`, `env` | files, running programs, standard input and output, environment variables | Lets embedders and sandboxes grant them one by one |
| `ffi` | calling an `extern fn` ([section 18](/reference/language/calling-c-and-go)) | Marks code the compiler can't check |

Every effect in the table is inferred, and a private function's effects never need writing. A
public function that publishes an event, reads a secret or calls an `extern fn` lists it in
`uses` as the table names it, as one that calls a service lists `net C.op`:
`uses {publish Bus.pinged}`, `uses {secret "telegram-bot-token"}`, with the name as passed to
`secret(…)`, and `uses {ffi}`. A bare kind covers all of its kind: `net` every operation,
`publish` and `consume` every event, `entity` every entity, and `secret` every secret, which is
what reading a secret by a name that isn't a literal needs. An `extern fn` that states its own
`uses`, as `http.get` states `net Http.get`, has those effects instead of `ffi`.

- **Pure means no `uses`.** A public function without a `uses` clause is pure, and the compiler
  holds it to that: if `area` read a file, the error would say `area uses fs, which its signature
  doesn't list: run eye fmt, or add uses {fs}`. Editors show the inferred effects of private
  functions.
- **`eye fmt` writes the clauses.** Formatting a program's directory, `eye fmt DIR`, checks the
  program and writes each public function's `uses` from what its body does: it adds a missing
  effect and drops one the body no longer has. An entry that still covers an effect stays, as
  `net` stays while the body calls any service; a clause that changes is written in sorted order.
  A file formatted alone (`eye fmt FILE`, or an editor formatting on save) is formatted for syntax
  only, and so is a program with other errors, which `eye fmt` reports. A private function's
  clause is a limit its author sets, so `eye fmt` leaves it as written.
- **Callbacks pass their effects to the call.** A function that takes a function lists only its own
  effects; calling it adds the effects of the function passed in, where the call is written. `map`
  is pure, and `paths |> map(fs.readText)` uses `fs`:

  ```eye
  pub fn sizes(paths: List(String)) -> List(Int) uses {fs} =
    paths |> map(fn p: fs.readText(p) |> size)    # fs comes from the lambda, not from map
  ```

  So `retry`, `timeout`, `race`, `parallelMap` and `reserve!` work with any function, and every
  public signature still lists everything its body can do.
- **Functions stored in data state their effects.** A function type without `uses` is pure,
  except in a parameter's type, where it takes a callback (below). One kept in a field or a
  collection, and called later, says what it may do:

  ```eye
  type Handler = { name: String, run: fn(Event) uses {net Notify.send} }
  ```

  In a function's return type, the `uses` after a function type belongs to that function type.

  Whoever calls a stored function later counts only what its type states, so a function kept
  there may do no more: `{ name: "n", run: fn e: now() }` is an error, `this function uses clock,
  which its type here, fn(Event) uses {net Notify.send}, doesn't list`. The same holds for a
  lambda or a named function put in a list, returned, or passed where a parameter's function type
  has a `uses` clause, which is then a limit. A function type without `uses` anywhere in a
  parameter's type, as in `map`'s `fn(a) -> b` or inside `race`'s `List(fn() -> a)`, takes a
  callback, which may do anything (the parameters and result of that function type keep the
  general rule);
  so a callback can't be kept in data, nor can a lambda that calls one. To keep it, state its
  effects in the parameter's type: `fn register(run: fn(Event) uses {clock}) -> Handler`. A
  stored function given as a callback, as in `xs |> map(h.run)`, does what its type states, which
  counts where it is given.

**`!` in jobs.** Inside a job, `!` marks every recorded step: it runs once and its result is
recorded, so a resumed run never repeats it and sees the same values. The steps are:

- every call with an outside effect (`net`, `fs`, `process`, `clock`, `random`, `env`, `ffi`,
  `secret` and `entity`), an `extern fn` included (`open!(path)`). That holds for a function of
  the program too: `stamp!()`, where `fn stamp() = now()`, is one step, and calling it without
  `!` is an error, `in a job, stamp uses clock, so calling it is a durable step: write stamp!(…)`;
- entity calls (`w.read!()`, `w.markAsleep!()`), each committed in the same transaction as its
  checkpoint ([section 15](/reference/language/jobs-and-waiting));
- publishing an event, `bus.pinged.publish!({ n: 1 })`, which is sent when the step commits;
- starting a child run, `digest.start!(day)`, so a resumed run doesn't start it again.

Calls that only compute are ordinary, and `!` on one is an error. Outside jobs the same functions
are called without `!`: in `main` ([section 13](/reference/language/programs-and-concurrency)), in tests, and in entity methods and served
operations, where publishing and starting a job are part of the transaction. **Output is not a
step:** `print` has the effect `stdio`, which public signatures list, but in a job it is written
without `!` and isn't recorded, and logging (`log.info`) is not an effect at all. Output is for
people watching a run, not part of what the run computes.

**A step's result is saved.** It is recorded so that a resumed run sees the same value, so it must
be something that can be saved ([section 15](/reference/language/jobs-and-waiting)). A `Secret`, or a value of an `extern type`, is used
inside a function that returns what can be saved, and that function is called as one step:
`checksum!(path)` rather than `open!(path)` followed by more calls on the handle. An error says so:
`this durable step's result is recorded, and a Secret can't be saved: use it in a function that
returns what can be, and call that with !`.

```eye
import bus

job ping(day: Date) {
  bus.pinged.publish!({ n: 1 })              # a step: published when it commits
  let child = digest.start!(day)             # a step: a resumed run doesn't start it twice
  log.info("started {day}")                  # not a step
  sleep!(1 min)
}
```
