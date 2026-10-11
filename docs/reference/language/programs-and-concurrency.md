---
title: "Programs and concurrency"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Programs and concurrency

A program starts at `pub fn main`. It is an ordinary function, not a job: its effects happen
directly, without `!`, and nothing about it is durable.

```eye
import fs

pub fn main(args: List(String)) -> Result((), FsError) uses {fs, stdio} {
  for path in args |> drop(1) {
    let text = fs.readText(path)?
    print("{path}: {text |> lines |> size} lines")
  }
  Ok(())
}
```

- `main` returns `()` or `Result((), e)`. An `Err` is printed with `Show`, and the program exits
  with status 1.
- **Files, programs and random numbers** come from the standard modules `fs`, `process` and
  `random`, each with the effect of its name. `fs.readText`, `writeText`, `appendText`, `list`,
  `makeDir` and `remove` return a `Result` whose error is an `FsError` (`NotFound(path)`,
  `NotAllowed(path)`, …), and `fs.exists` a `Bool`. `process.run("git", args: ["pull"], input:
  "", dir: None)` runs a program without a shell and gives `Ok` with its exit status and output
  for any status, `Err(NotStarted(…))` when it couldn't start. `random.int(1..=6)` (an `Option`,
  `None` for an empty range), `random.float()`, `random.pick(xs)` and `random.shuffle(xs)` draw
  numbers. A relative path is relative to the directory the program was started in.
- **`eye run` keeps a program's jobs going.** It runs `main`, if the program has one, every time
  it starts; then, if the program declares jobs, it stays up: sleeping runs wake when their time
  comes, schedules fire, and it stops when nothing is left to wait for, or on Ctrl-C or SIGTERM.
  Entities, runs and events are kept in the program's journal (`.eye/journal`), so a restart
  carries on where the last one stopped: a run that was due while the program was down runs at
  once, and a schedule that missed firings fires once, for the latest. Because `main` runs on every
  start, a job it starts takes a key, which starts it once (`ring.start("kitchen", key:
  "kitchen")`); without one, the checker refuses the call. `eye runs` lists the runs in the
  journal: their status, what each waits for and until when, and their steps.
- **`eye repl [dir]`** is an interactive session in a program, or in an empty one. An expression
  prints its value and type (`[1, 2] |> map(fn x: x * 10)` prints `[10, 20]: List(Int)`), a
  statement runs, and `let` and `var` bindings last for the session. A declaration (`fn`, `type`,
  `entity`, `job`, `view`, `impl`, `contract`, `test`, `import`) joins the session, and declaring
  a name again replaces it. Every input is checked with the whole session, so its errors are the
  compiler's, shown under the line of the input they are on. Jobs run on a clock that moves only
  with `:advance`; `:runs`, `:type`, `:doc`, `:test`, `:reset` and `:quit` are the other
  commands.
- **Modules.** A module is a directory: its `.eye` files share one namespace and one set of
  imports. A program is the module in its directory and the modules it imports. `import lab.notify`
  names the directory `lab/notify` below the program's directory; its public names are then used as
  `notify.Delivery`, or brought in by name with `import lab.notify.{Delivery}`.
- **A program's root is marked by `eye.toml`.** A file belongs to the program of the nearest
  directory, from the file upward, that holds an `eye.toml`; without one, it belongs to the
  program in its own directory. So `lab/digest/digest.eye` belongs to the program in `lab/` when
  `lab/eye.toml` exists, and its `import home` finds `lab/home`. `eye check file.eye` and the
  editor's go-to-definition use this rule. The file may be empty.
- **`modules` in `eye.toml` names more directories to find imports in.** An import is looked for
  below the program's own directory first, then below each listed directory, in order; the first
  that has it wins. Paths are relative to the `eye.toml`, and each must stay inside the repository
  (or the build's context), so that a copy of the program has everything it needs. A program
  deployed alone, such as the lab's digest, is then its `eye.toml` and a file that imports what
  it runs:

  ```toml
  # testdata/lab/deploy/digest/eye.toml
  modules = ["../.."]                      # testdata/lab, where digest, home and notify are
  ```

  An import that no directory has is an error that lists every directory searched. This is the
  one key `eye.toml` holds so far; what else it will hold is open question 5.

**Concurrency** is structured. Tasks start inside one call, and all of them finish (or are
cancelled) before that call returns, so no task outlives the code that started it.

```eye
let pages = urls |> parallelMap(fn u: http.get(u), limit: 8)    # results in input order
let (user, orders) = both(fn { users.get(id) }, fn { orders.list(id) })
let fastest = race([fn { mirrorA.get(path) }, fn { mirrorB.get(path) }])
```

- **No `async` or `await`.** Any function can run in a task. A task that waits for input or output
  is parked by the runtime while others run, so functions don't come in two colours.
- **No data races.** Tasks share only immutable values; a `var` or `mut` parameter can't be
  captured. Results come back as return values.
- **Cheap tasks.** A task's stack starts small and grows, so thousands of waiting tasks cost little
  memory.
- `race` cancels the losers at their next wait. `timeout(5 s, fn { … })` returns an `Option`:
  `None`, with the function cancelled at its next wait, if it doesn't finish in time.
  `parallelMap` runs at most `limit:` tasks at once, all of them without one. If a task fails,
  the others are cancelled and the call fails with it.
- **Waits overlap.** On the system clock, a task that waits for the network, a timer or another
  service lets the others run, so two requests of 300 ms each take 300 ms together. On a test's
  clock, tasks take turns: one runs until it waits, and the one that wakes first goes next, with the
  clock moving to its wake time. A call takes as long as its longest task, in either, and every run
  of a test is the same.
- **From v0.2, task groups and channels** cover streams and work that is discovered as it goes. A
  group starts tasks with `spawn` and returns only when all of them have finished. A channel has a
  fixed capacity, and `put` parks while it is full, so a fast producer can't fill memory:

  ```eye
  taskGroup(fn group {
    let (send, receive) = channel(capacity: 64)
    group.spawn(fn {
      for line in fs.lines(path) { send.put(line) }
      send.close()
    })
    for line in receive { handle(line) }
  })
  ```

  - `taskGroup` runs its function as the group's first task and returns that function's result
    once every task has finished. A task may spawn more, so work found on the way joins the same
    group. If a task fails, the others are cancelled at their next wait and the group fails with
    it.
  - `channel(capacity: n)` gives a `Sender(a)` and a `Receiver(a)`, and holds at most `n` values,
    at least one. `send.put(x)` parks while it is full, `send.close()` ends it, and a put after
    that fails. `for x in receive` reads until the channel is closed and empty, and
    `receive.take()` gives `Some` of the next value, or `None` once it is closed and empty.
  - A group whose tasks all wait for channels that no task will put into or close would wait
    forever, so it fails instead, at the `taskGroup` call: a producer that forgets `close()` is
    a bug the test shows, not a hang.
  - `TaskGroup`, `Sender` and `Receiver` can't be saved, so channels can't be live across a `!`;
    jobs talk through entities and events.
- **In jobs**, concurrent durable steps are child runs (`digest.start!(day)`) joined with `all!` or
  `first!`. A task a job starts with `parallelMap`, `both`, `race` or `timeout` only computes,
  since its steps couldn't be recorded in order: a `!` step in it, or a call that would need one,
  is an error.
