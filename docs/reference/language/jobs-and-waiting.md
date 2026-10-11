---
title: "Jobs and waiting"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Jobs and waiting

A job is a durable function: it can pause for any length of time and holds no memory while paused.

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

- **`!` marks a checkpoint.** Every recorded step in a job is a `!` call ([section 12](/reference/language/effects-and-signatures)). A resumed
  or restarted run continues from its last checkpoint: no recorded call runs again, and it sees
  the values it saw. The interpreter does this by replaying the run's steps, each returning its
  recorded result; from v0.3 it saves the job's live variables at each `!` instead, as compiled
  jobs do (section 22).
- **A paused run resumes only where its code still is.** Each `!` is a resume point, named by its
  function, its place and the names and types of the variables live there. When the code changes
  under a paused run so that its resume point is gone or holds different variables, `eye run`
  refuses to start, as it does for entity rows no migration can bring forward, and says which job
  and how many runs: `2 runs of watch are paused at a step that changed; finish them on the old
  code, or end them with eye runs`. To change a job whose runs are paused, keep the old job under
  its name until its runs finish and give the new code a new name, or end the old runs. Until
  runs have frames, a replayed step whose call differs from the one recorded faults the run
  instead: `the code of watch changed under this run at step 12`.
- **Long runs.** A run's saved state is its live variables, so a job that loops for weeks costs no
  more on its last day than on its first. While the interpreter still replays (until v0.3), each
  wake replays every step so far, so a loop that never ends should end its run and start a new
  one now and then; the start is a recorded step, so a crash doesn't start the new run twice:

  ```eye
  job watch(t: Ref(Traveller)) {
    for _ in 0..80 {                         # about an hour of 45-second loops
      let me = t.read!()
      if me.pins == [] && me.aboard == None { return }
      sleep!(45 s)
    }
    watch.start!(t)                          # carry on in a fresh run
  }
  ```
- **Only values that can be saved** (types with `Save`) may be live across a `!`. Holding a socket,
  file handle or `Secret` across a checkpoint is a compile error.
- **Entity calls from jobs** (`World(w).markAsleep!()`) commit in the same store transaction as the
  checkpoint, so they happen exactly once.
- **Where runs live.** Entities, runs and checkpoints are kept by a store, chosen when the program is
  started or embedded. The default store keeps them in memory and is itself written in Eyelang
  (`MemoryStore` in `check/runtime/store.eye`, part of every program): it is an immutable value, each
  transaction is a pure function from one store value to the next, and the runtime holds the only
  reference, so compiled code updates it in place. (The interpreter has no reference counts, so it
  copies what changes; the store keeps runs in buckets of 64 and rows by entity type to keep that
  small.) Until traits are checked, `Store` is the Go interface the runtime calls, and
  `MemoryStore` has the methods a `Store` trait will ask for. It is what tests, the REPL and
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
  | `all!([a, b])`, `first!([a, b])` | every one, or whichever finishes first, of several waits |

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
    forgets to check who pressed a button therefore can't grant consent.
  - **People at a command line, and tests, answer by text.** `eye approve TEXT` and
    `eye decline TEXT` answer through the inbox of the program's `eye run` the one waiting
    approval whose text contains `TEXT`; when several do, they answer none and list them with
    their ids, and `eye approve --id ID` answers one. Tests answer with `approve(…)` and
    `decline(…)`, which fail the test when their text matches more than one waiting approval.
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
  (`DeadlinePassed`, `OverBudget`, `OutOfMemory`, `Unavailable`, `Unreachable`, `Killed`) can't be
  caught where it happens, so code that must report one starts the work as a child run and joins
  it:

  ```eye
  fn written(day: Date) -> Result(Article, DigestError) =
    match write.start!(day).join!() {
      Completed(result) => result
      Failed(fault) => Err(Faulted(fault))
    }
  ```
- **Starting jobs:** `irelandDigest.start(day)` returns a `Run(a)`. It works from entity methods (as
  part of the transaction), from `main` and tests, and from jobs, where it starts a child run and
  is written `irelandDigest.start!(day)`: the start is a recorded step, so a resumed run doesn't
  start the child again. Scheduled starts take
  `after:` or `at:`: `sleepWhenIdle.start(w, after: 20 min)`,
  `irelandDigest.start(day, at: 07:40 in "Europe/Dublin")`. With `key:`, a job starts once per key:
  a later start with the same key returns the run the first one started, finished or not.
- **Attributes:**

  | Attribute | Meaning |
  |---|---|
  | `@on(trigger)` | `daily(…)`, `weekdays(…)`, `every(15 min)`, `cron("…", in: zone)`, `enters(view)`, `leaves(view)`, `event(contract.event)`. A schedule passes the job the date of the firing in the schedule's zone, if the job takes one; a view, the entity's `Ref`; an event, the event |
  | `@memory(size)` | budget admitted before the run starts; the runtime enforces it |
  | `@priority(p)` | `interactive`, `normal`, `background` or `batch`; under memory pressure, lower priorities pause at their next checkpoint |
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
