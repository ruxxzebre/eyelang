---
title: "Jobs and waiting"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Jobs and waiting

A job is a durable function: it can pause for any length of time and holds no thread while paused,
only its saved record.

```eye
@on(daily(07:40, in: "Europe/Dublin"))
@priority(background)
@memory(64 MiB)
@deadline(13:40, in: "Europe/Dublin")
job irelandDigest(day: Date) -> Result(Delivery, DigestError) {
  let items = miniflux.unread!("ireland", since: day - 1 day)?
  let text = reserve!(256 MiB, fn {            # waits until the host grants the memory
    summarise!(items)
  })
  Ok(notify.send!(owner, text, key: "ireland-{day}").reconcile!())
}
```

- **`!` marks a step; a checkpoint is where a run resumes.** Every recorded step in a job is a
  `!` call ([section 12](/reference/language/effects-and-signatures)). A checkpoint is the run's saved place. A resumed
  or restarted run continues from its last checkpoint: no recorded call runs again, and it sees
  the values it saw. The interpreter keeps a checkpoint at the start of each statement of the
  job's body, the statements inside its loops, ifs and matches included, once the run has taken
  steps since the last: the job's variables, the statement, and where each loop around it is.
  A run that wakes restores them and replays only the steps since, each returning its recorded
  result. A statement that calls a function with `!` steps of its own replays that function's
  steps, so a loop that never ends belongs in the job's body, not in a function it calls once.
  While a variable holds what can't be saved, the run keeps its earlier checkpoint. Compiled jobs
  save their live variables at each `!` (section 22).
- **A paused run resumes only where its code still is.** A checkpoint names the statement it
  resumes at by its place in the job and its code, with types; the job must keep as many
  variables. When the code changes under a paused run so that the statement is gone or reads
  differently, `eye run` refuses to start, as it does for entity rows no migration can bring
  forward, and says which job and how many runs: `the code of a job changed where its runs are
  paused, so they can't resume: watch (2 paused runs)`. A change elsewhere in the job is fine. To
  change a job where its runs are paused, keep the old job under its name until its runs finish
  and give the new code a new name, or end the old runs. A step replayed after the checkpoint
  whose call differs from the one recorded, as when a function the statement calls changed,
  faults the run: `the code of watch changed while this run was paused: its step 12 called
  notify.send, and the code now calls sleep there`.
- **Long runs.** A run's saved state is its variables and the steps of the statement it is in,
  so a job that loops for weeks, such as the lab's train watcher, costs no more on its last day
  than on its first. `eye runs` keeps the last 20 steps a checkpoint dropped, and counts the
  rest.
- **Only values that can be saved** (types with `Save`) may be live across a `!`. Holding a socket,
  file handle or `Secret` across a `!` is a compile error.
- **A job reads stored state only in steps.** A resumed run must see what it saw, so every read
  of entities in a job is a recorded step: `w.read!()` gives one entity's snapshot, and
  `idleWorlds.read!()` a view's snapshots, as a list, as `World.all.read!()` gives every
  `World`'s. The run records the list, and a replay returns it, even when the entities have
  changed since. A view, `World.all` or a state field read any other way in a job is an error
  that names the step to write: `in a job, reading idleWorlds isn't recorded, so a resumed run
  could see other entities: read it with idleWorlds.read!(), a recorded step that gives a list
  of snapshots`. A waiting function follows the same rule, since it runs in the job's run.
  Outside jobs a view and `World.all` are those lists already, and `read()` gives the same.
- **Entity calls from jobs** (`World(w).markAsleep!()`) commit in the same store transaction that
  records the step in the run, so they happen exactly once.
- **Where runs live.** Entities, runs and checkpoints are kept by a store, chosen when the program is
  started or embedded. The default store keeps them in memory and is itself written in Eyelang
  (`MemoryStore` in `check/runtime/store.eye`, part of every program): it is an immutable value, each
  transaction is a pure function from one store value to the next, and the runtime holds the only
  reference, so compiled code updates it in place. (The interpreter has no reference counts, so it
  copies what changes; the store keeps runs in buckets of 64 and rows by entity type to keep that
  small.) The trait `Store(s)` in the same file says what a store does, and `MemoryStore`
  implements it; the runtime, written in Go, calls the store through a Go interface with the
  same methods. It is what tests, the REPL and
  embedders use; on its own it doesn't survive a restart. For one machine, the journal (question 9)
  appends each commit to a file and syncs it before the commit counts, which is what `eye run`
  uses by default. The Postgres store (`eye run --store postgres://…`, or `$EYE_STORE`) keeps the
  same state in a database, so it survives the loss of the machine: each commit is one SQL
  transaction, written before it counts, and a commit made while the database is down or not
  answering changes nothing and is tried again, so the runner carries on once the database is back.
  One runner writes a store at a time, held by an advisory lock; one database holds several
  programs' stores, one per schema. Both wrap the in-memory store, and programs don't change when
  the store does.
- **Calls to other services** carry a key. If the outcome is `Uncertain`, `.reconcile!()` asks the
  owner with the same key until it knows. Nothing is resent automatically.
- **Waiting constructs** (all library functions, all marked with `!`):

  | Call | Waits for |
  |---|---|
  | `sleep!(20 min)`, `sleepUntil!(07:45 in "Europe/Dublin")` | time |
  | `poll!(every: 1 min, within: 6 h, fn { miniflux.ready!() })` | a service that can't say when it is ready; `false` if time runs out |
  | `approval!("delete 3 backups", from: operator, within: 3 days)` | a human; returns `Approved(Approval) \| Declined \| Expired` |
  | `next!(notify.delivered, where: fn e: e.key == k, within: 1 h)` | an event; `None` if time runs out |
  | `reserve!(1 GiB, fn { … })` | memory from the host, released when the function returns |
  | `lock!("ci", for: 30 min, fn { … })` | a named lock that records its owner and expiry |
  | `run.join!()` | a job started earlier: `Completed(value)`, or `Failed(fault)` if it stopped first |
  | `allDone!([a, b])`, `firstDone!([a, b])` | every one, or whichever finishes first, of several runs |

- **Joining several runs.** `allDone!` and `firstDone!` take a list of runs, `List(Run(a))`, which
  a job started with `start!`. `allDone!(runs)` waits until every one has finished and gives their
  outcomes, a `List(Outcome(a))` in the list's order. `firstDone!(runs)` waits until one has
  finished and gives its `Outcome(a)`; if several had finished before it was called, the one that
  finished first. It stops the others, which end as `Failed(Killed)` with the steps they took, so
  a wait that lost doesn't go on: an approval that timed out no longer waits for its answer.
  `all` and `first` are only the collection functions: `all!(runs)` and `first!(runs)` are
  errors that name the wait to write, since `!` marks a call as a step and never changes which
  function a name means. A timeout is a run that sleeps:

  ```eye
  job ask(what: String) -> Decision { approval!(what, from: operator, within: 3 days) }

  job timer(d: Duration) -> Decision {
    sleep!(d)
    Expired
  }

  job ship(build: String) -> Result((), fs.FsError) {
    match firstDone!([ask.start!("ship {build}?"), timer.start!(1 h)]) {
      Completed(Approved(ok)) => deploy!(build, ok)
      Completed(_) | Failed(_) => {
        log.info("{build} wasn't approved within the hour")
        Ok(())
      }
    }
  }
  ```
- **Locks.** `lock!(name, for: d, fn { … })` waits until no other run holds the lock of that name,
  takes it for `d`, runs the function, whose `!` steps are the run's own, and gives the lock back
  when the function returns, with its result. The store keeps who holds each lock and until when,
  in the commit that records the step, so a restart neither frees a lock nor grants it twice. A
  run that faults, or that `firstDone!` stops, gives its locks back.
  - **A lock runs out.** When its time is up while the holder is still waiting inside the
    function, the holder ends as `Failed(LockExpired(name))` and the next run that asks takes the
    lock, so the work it guards never runs beside another holder. A holder that is computing when
    the time comes ends when it next waits inside the function. To report it, run the locked work
    as a child run and join it.
  - A run that asks for a lock it already holds faults: locks don't nest.

- **`next!` misses nothing.** It counts every matching event published since the run's previous
  step, so an answer that comes while the run is still sending its question, or sleeping, is found.
  `poll!` keeps only its outcome, so a long poll doesn't grow the run's history.
- **Approvals reach people through the program.** When a run starts waiting at `approval!`, the
  runtime publishes `approvals.requested`, an event of the standard `approvals` module, with the
  request's `id`, `what`, `from` and `until`. A program that imports `approvals` routes it with a
  job of its own, through Notify's buttons or anything else, and answers with
  `approvals.answered.publish!({ id, approved, by })`.
  - **An answer goes to the request it names.** Its `id` is the request's, so two runs that ask
    the same thing are answered one by one.
  - **Only the person asked may answer.** `by` must be the request's `from`, both as text; the
    runtime refuses any other answer, logs it, and the run keeps waiting. A routing job that
    forgets to check who pressed a button therefore can't grant consent. The check trusts the
    `by` it is given: events have no `@allow`, so any module of the program can publish
    `approvals.answered` with `by: r.from` and grant consent without a person. It guards against
    a press by the wrong person, not against the program's own code (open question 35).
  - **People at a command line, and tests, answer by text.** `eye approve TEXT` and
    `eye decline TEXT` answer through the inbox of the program's `eye run` the one waiting
    approval whose text contains `TEXT`, as the person it asks; when several do, they answer none
    and list them with their ids, and `eye approve --id ID` answers one. Tests answer with
    `approve(…)` and `decline(…)`, as the person asked unless `by:` names someone else; they fail
    the test when their text matches no waiting approval, or more than one.
  - **An `Approval` says what was approved and by whom**: `ok.what` and `ok.by`, for audit logs.
    It is still made only by `approval!`.

  ```eye
  import approvals

  @on(event(approvals.requested))
  job ask(r: approvals.Request) {
    let left = r.until - now!()
    let asked = notify.ask!(to: operator, text: r.what, choices: [yes, no], within: left,
      key: "approval-{r.id}").reconcile!()
    let Asking(question) = asked else { return }
    let Some(press) = next!(notify.answered, where: fn a: a.question == question, within: left) else {
      return
    }
    approvals.answered.publish!({ id: r.id, approved: press.choice == yes.id, by: "{press.by}" })
  }
  ```
- **A run's outcome** is `Outcome(a) = Completed(a) | Failed(Fault)`, from the prelude. A fault
  (`DeadlinePassed`, `OverBudget`, `OutOfMemory`, `Unavailable`, `Unreachable`, `Killed`,
  `LockExpired`) can't be
  caught where it happens, so code that must report one starts the work as a child run and joins
  it:

  ```eye
  fn written(day: Date) -> Result(Article, DigestError) =
    match write.start!(day).join!() {
      Completed(result) => result
      Failed(fault) => Err(Faulted(fault))
    }
  ```

  Every other fault the runtime meets ends the run as `Unreachable`, with a message that says
  what happened: a request the service refused (a 4xx reply that `status:` doesn't name, section
  17), a replayed step whose call differs from the recorded one, a call a test's fakes have no
  exchange for, and a dollar `@budget` on a model whose price isn't known ([section 19](/reference/language/ai-models-and-agents)). These are
  bugs in the program or its configuration, so like `unreachable` they fail the test they happen
  in, while the other five faults come from a limit or a service, not a bug, and don't (section
  20).
- **Starting jobs:** `irelandDigest.start(day)` returns a `Run(a)`. It works from entity methods (as
  part of the transaction), from `main` and tests, and from jobs, where it starts a child run and
  is written `irelandDigest.start!(day)`: the start is a recorded step, so a resumed run doesn't
  start the child again. Starting a job is the effect `start J` ([section 12](/reference/language/effects-and-signatures)), so a function that
  starts one is a step in a job too, `kick!()`. Scheduled starts take
  `after:` or `at:`: `sleepWhenIdle.start(w, after: 20 min)`,
  `irelandDigest.start(day, at: 07:40 in "Europe/Dublin")`. With `key:`, a job starts once per key:
  a later start with the same key returns the run the first one started, finished or not.
- **Reading a run's outcome without waiting.** Only a job waits for a run, with `run.join!()`. A
  test, `main` and the functions they call read where it got to with `run.outcome()`, an
  `Option(Outcome(a))`: `None` while the run is still going (queued, sleeping or waiting), and
  `Some(Completed(value))` or `Some(Failed(fault))` once it has ended. A test's runs move only
  when it calls `advance(…)` or publishes an event ([section 20](/reference/language/time-secrets-tests)), so `outcome()` read after one of
  them sees what that did:

  ```eye
  job brew(cups: Int) -> Int {
    sleep!(4 min)
    cups
  }

  test "a brew is ready after four minutes" {
    let run = brew.start(2)
    expect(run.outcome() == None)
    advance(5 min)
    expect(run.outcome() == Some(Completed(2)))
  }
  ```

  `join` on a run is always a wait, never the strings' `join`, so `run.join()` without `!` is an
  error that names the call to write: in a job, `in a job, joining a run is a durable step that
  waits for it: write run.join!()`; elsewhere, `a test can't wait for a run: read where it got to
  with run.outcome(), which is None while the run is still going (only a job waits, with
  join!())`. In a job, `outcome()` is an error too, since what it reads isn't recorded: `in a job,
  a run's outcome is read with run.join!(), a step that waits for it: outcome() isn't recorded,
  so a resumed run could see another`.
- **Attributes:**

  | Attribute | Meaning |
  |---|---|
  | `@on(trigger)` | `daily(…)`, `weekdays(…)`, `every(15 min)`, `cron("…", in: zone)`, `enters(view)`, `leaves(view)`, `event(contract.event)`. A schedule passes the job the date of the firing in the schedule's zone, if the job takes one; a view, the entity's `Ref`; an event, the event |
  | `@memory(size)` | budget admitted before the run starts; the runtime enforces it |
  | `@priority(p)` | `interactive`, `normal`, `background` or `batch`; under memory pressure, a run of a lower priority doesn't start or resume while one of a higher priority waits for memory, and a run that is executing isn't paused (open question 26) |
  | `@deadline(duration)`, `@deadline(13:40, in: zone)` | how long after its start, or by what time, a run must finish; after that it ends as `Failed(DeadlinePassed)`, even while it sleeps or waits |
  | `@concurrency(n, by: param)` | at most `n` runs at once, or per value of the parameter named. A run over the limit waits in a queue and begins when a run of its group finishes, in the order they were started; a waiting run keeps its place, so `@concurrency(1)` asks one question at a time, in order. A scheduled start that finds the limit reached is skipped, not queued |
  | `@limit(n, per: duration, by: param)` | at most `n` runs begin in any window of that length, optionally per value of a parameter: `@limit(1, per: 1 s, by: chat)`. A run over the rate waits until the window lets it begin |

  All of them are enforced by the runtime (`@budget` in [section 19](/reference/language/ai-models-and-agents)).
- **Memory.** A runner has a memory limit (`eye run --memory-limit 512MiB`, the host's memory
  without one, 1 GiB in tests). A run holds its `@memory` only while it executes, and
  `reserve!(size, fn { … })` holds `size` until `fn` returns, across `fn`'s own waits. A run executes,
  and a `reserve!` is granted, only when the memory fits beside what is held and no run of a
  higher `@priority` waits for memory; otherwise it waits at the checkpoint it reached, so under
  pressure lower priorities pause and none overtakes a higher one. What is reserved is kept with
  the runs, so a restart neither forgets nor repeats a grant. A run whose `@memory` is more than
  the runner has, or that holds more than its `@memory`, ends as `Failed(OutOfMemory)`; the
  reference interpreter counts what it keeps for the run, its saved state with each step's result
  (open question 26).
