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

A private function's effects are inferred and never need writing.

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

**`!` in jobs.** Inside a job, every call with an outside effect (`net`, `fs`, `process`, `clock`,
`random`, `env`, `ffi`, `secret` and `entity`) is a `!` call: it runs once and its result is
recorded, so a resumed run never repeats it and sees the same values. That holds for a function of
the program too: `stamp!()`, where `fn stamp() = now()`, is one step, and calling it without `!` is
an error, `in a job, stamp uses clock, so calling it is a durable step: write stamp!(…)`. Entity
calls are `!` calls too (`w.read!()`, `w.markAsleep!()`): each commits in the same transaction as
its checkpoint ([section 15](/reference/language/jobs-and-waiting)). Calls that only compute are ordinary, and `!` on one is an error.
Outside jobs the same functions are called without `!` ([section 13](/reference/language/programs-and-concurrency)). Printing and logging are not
steps (open question 22).
