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
| `publish T`, `consume T` | sending or receiving events | Derives message-queue permissions |
| `entity E` | reading or changing entities of type `E` | Database grants; includes the type in backups |
| `secret S` | using secret `S` | Delivers that secret only to code that uses it |
| `clock`, `random` | `now()`, `random()` | Lets tests replace them |
| `fs`, `process`, `stdio`, `env` | files, running programs, standard input and output, environment variables | Lets embedders and sandboxes grant them one by one |
| `ffi` | calling an `extern fn` ([section 18](/reference/language/calling-c-and-go)) | Marks code the compiler can't check |

Every effect in the table is inferred, and a private function's effects never need writing. A
public function that publishes an event, reads a secret or calls an `extern fn` lists it in
`uses` as the table names it (`publish T`, `secret S`, `ffi`), as one that calls a service lists
`net C.op`.

- **Pure means no `uses`.** A public function without a `uses` clause is pure, and the compiler
  holds it to that: if `area` read a file, the error would say `area uses fs, which its signature
  doesn't list: add uses {fs}`. Editors show the inferred effects of private functions.
- **Callbacks pass their effects to the call.** A function that takes a function lists only its own
  effects; calling it adds the effects of the function passed in, where the call is written. `map`
  is pure, and `paths |> map(fs.readText)` uses `fs`:

  ```eye
  pub fn sizes(paths: List(String)) -> List(Int) uses {fs} =
    paths |> map(fn p: fs.readText(p) |> size)    # fs comes from the lambda, not from map
  ```

  So `retry`, `timeout`, `race`, `parallelMap` and `reserve!` work with any function, and every
  public signature still lists everything its body can do.
- **Functions stored in data state their effects.** A function type without `uses` is pure; one
  kept in a field or a collection, and called later, says what it may do:

  ```eye
  type Handler = { name: String, run: fn(Event) uses {net Notify.send} }
  ```

  In a function's return type, the `uses` after a function type belongs to that function type.

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

```eye
import bus

job ping(day: Date) {
  bus.pinged.publish!({ n: 1 })              # a step: published when it commits
  let child = digest.start!(day)             # a step: a resumed run doesn't start it twice
  log.info("started {day}")                  # not a step
  sleep!(1 min)
}
```
