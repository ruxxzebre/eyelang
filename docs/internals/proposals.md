---
title: Proposals
---

# Proposals

Decisions for the user, drafted on 2026-10-11 against `dev` at 1cc429d. Each section gives the
problem with an example from the repository, two or three options, a recommendation, and what
changes if the recommendation is accepted.

**Decided 2026-10-11: the user accepted every recommendation.** SPECS.md's decision log has a row
for each, its rules and examples changed with them, and ROADMAP.md lists the work each needs. This
page keeps the reasoning. One fact changed after it was written: `eye` did not move to tree-sitter
and cgo. The hand-written parser stays, tree-sitter is used only in tests, and `eye` itself stays
free of cgo; the sections below say what that changes.

The first five sections are open questions from SPECS.md §24. The sixth, module roots in
`eye.toml`, came from deploying the lab's digest. The last three are places where
SPECS.md, ROADMAP.md and the code disagree.

Nothing here needs new syntax. Where an example shows code as it would be after a change, the
code already parses today; only the rules about it would change.

## Q17: approvals answered by id, and who may answer

**Decided 2026-10-11: accepted** (option 1). SPECS.md §15 and the decision log's "Answering
approvals".

### The problem

A run that waits at `approval!` publishes `approvals.requested`, and the lab's `approving` module
asks the operator on Notify and passes the first press on
([`testdata/lab/approving/approving.eye`](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/approving/approving.eye)):

```eye
approvals.answered.publish!({ what: r.what, approved: press.choice == yes.id, by: "{press.by}" })
```

The runtime takes an answer for every waiting approval whose text *contains* the answer's `what`
(`approval` in `interp/jobs.go`: `strings.Contains(what, s)`). Two consequences:

- **Answers match by text, not by request.** Two runs that both wait for "Restart the router?"
  are answered by one press. An answer whose `what` is "Restart" answers "Restart the router?"
  and "Restart the NAS?" together. `eye approve "Restart"` does the same from the command line.
- **Anyone's answer counts.** `approval!("…", from: operator, within: 1 h)` says whom to ask, but
  the runtime never compares the answer's `by` with `from`. If the routing job sent the question
  to a group chat, or another job published an answer, the approval would stand. Nothing makes
  the routing job check `press.by`, and the lab's doesn't. An `Approval` is the one value that
  stands for a human's consent (SPECS §5), so this is the lab's most important security gap.

A third, smaller point: an `Approval` can't be read. The runtime knows who approved and what
(`ApprovalVal{What, By}`), but the checker refuses `ok.by`, so an audit log can't say who agreed.

### Options

1. **Answer by id, and check who answers.** `approvals.Answer` names its request's `id`. The
   runtime refuses an answer whose `by` isn't the request's `from`, and logs it. Text matching
   stays for people at a command line and in tests, and refuses when it matches more than one
   waiting approval. `Approval` gets `what` and `by` fields.
2. **Answer by id; who may answer stays the program's job.** As 1, without the `by` check. The
   routing job must check `press.by` itself, and the docs say so.
3. **Keep text; add only the `by` check.** The smallest change, and two runs asking the same
   thing are still answered together.

### Recommendation

Option 1. The obvious program should be the safe one: a routing job that forgets to check who
pressed the button must not grant consent. With ids, an answer can't reach a request it wasn't
meant for. The lab's routing job becomes:

```eye
approvals.answered.publish!({ id: r.id, approved: press.choice == yes.id, by: "{press.by}" })
```

`from` and `by` are compared as text, as both are shown today (`from: who` is any value, and
`approvals.Request.from` is its text).

### What changes

- SPECS §15 ("Approvals reach people through the program") and open question 17: answers by id;
  `by` must be `from`; text only from the command line and tests, refused when ambiguous; the
  decision log gets a row.
- `check/std/approvals.eye`: `Answer` gets `id` and loses `what`; its doc comments change.
- `interp/jobs.go` (`approval`): match by id, compare `by` with `from`. `interp/inbox.go` and
  `cmd/eye/approve.go`: `eye approve TEXT` finds the waiting approvals in the journal and refuses
  when TEXT matches more than one, listing them with their ids; `eye approve --id ID` answers one.
- `approve(…)` and `decline(…)` in tests: still by text, failing the test when the text matches
  more than one approval. `approve(…, by:)` already exists and is checked against `from`.
- `Approval` gets `what: String` and `by: String`, readable but still made only by `approval!`.
- The lab: `approving.eye` publishes the id; `access.eye`'s tests already pass `by: operator`.
- Docs: the guide's approvals pages and the tutorial's approvals chapter.

## Q19 and Q22: which actions are effects

**Decided 2026-10-11: accepted** (option 1). SPECS.md §12 and the decision log's "Recorded steps".

### The problem

SPECS §12's rule is that in a job, every call with an outside effect is a `!` call: it is a
recorded step. Three actions are recorded steps in the runtime but aren't effects in the checker,
and printing is an effect that isn't a step.

- **Publishing.** SPECS §17 says a job publishes with `publish!`, and the interpreter faults a run
  that doesn't ("in a job, publishing is a durable step: write publish!(…)", `interp/eval.go`).
  But `eye check` accepts it. This program checks cleanly, and its run stops at run time:

  ```eye
  import bus

  job j() {
    bus.pinged.publish({ n: 1 })
    sleep!(1 min)
  }
  ```

  A public function that publishes passes without `uses {publish …}`, although SPECS §12's table
  lists `publish T`.
- **Starting a child run.** `digest.start(day)` in a job is recorded as a step, so a resumed run
  doesn't start the child again (SPECS §15). It is written without `!`.
- **Calling an `extern fn`.** SPECS §12 lists `ffi` among the outside effects, but it isn't
  inferred: an extern call in a job needs no `!`, and `open!(…)` on one is refused as "neither
  waits nor reaches outside the program". `secret S` and `consume T` aren't inferred either.
- **Printing.** `print` has the effect `stdio`, which by the rule makes it a step, yet it isn't
  one, and `log.info` has no effect at all. The interpreter keeps a replayed run quiet so that
  output isn't repeated (`TestReplayDoesntRepeatOutput`).

### Options

1. **`!` marks every recorded step.** The effects SPECS §12's table lists are inferred: `publish T`,
   `consume T`, `secret S`, `ffi`. In a job, publishing, starting a child run and calling an
   extern are `!` steps: `bus.pinged.publish!(…)`, `digest.start!(day)`, `open!(path)`. Output
   (`print`, `log`) stays outside the rule: it is not a step, `print` keeps `stdio` for signatures,
   and logging is not an effect.
2. **Fix only what disagrees with SPECS.** Infer `publish T`, `ffi`, `secret S` and `consume T`,
   and have the checker require `publish!`, as the runtime does. Starting a child run stays
   without `!`, as a documented exception.
3. **Output becomes a step too.** As 1, and `print!` and `log.info!` in jobs, with `log` an
   effect. Every recorded thing is marked, but every log line in a job carries `!`, and every
   function that logs needs `uses {log}`.

### Recommendation

Option 1. The decision log says every `!` is a checkpoint, and SPECS §1 promises that "every
point where a job can pause is marked with `!`". Starting a child run commits a step, so it is
such a point; leaving it unmarked is the one exception a reader has to remember. Output is
different: it is a side channel for people, not part of a run's meaning, and marking it would
make every job noisy (rule 6 of CLAUDE.md, "strict, but never clumsy"). Once jobs resume from
saved frames (see the [compilation design](/internals/compilation)), nothing is replayed, and quiet
replays stop mattering.

### What changes

- SPECS §12: the table's effects are inferred, and the `!` rule says "every recorded step:
  outside effects, entity calls, publishing and starting a child run; output is not a step".
  Open questions 19 and 22 are closed with a decision-log row.
- SPECS §13, §15 and §19: `digest.start(day)` in jobs becomes `digest.start!(day)`, and
  `write.start(day).join!()` becomes `write.start!(day).join!()`. `start` in an entity method or
  a test stays without `!`.
- The checker: infer the four effects (track B's item 8), require `publish!` and `start!` in
  jobs, and refuse `!` on them elsewhere.
- `eye fix` (or the formatter) adds the `!` to existing code. The lab, the tour, the docs'
  programs and `testdata/programs` change.
- Every public function that publishes, reads a secret or calls an extern lists it in `uses`.

## Q21: `Handle` against `extern type`, and issue #42

**Decided 2026-10-11: accepted** (option 1). SPECS.md §18 and the decision log's "Foreign values";
issue #42 has the text below.

### The problem

Two features name a value that foreign code owns:

- `Handle(name)`, from SPECS §5 and §18: `extern fn snapshot(camera: Handle(Camera)) -> Bytes`
  (`testdata/syntax/valid/extern.eye`). `name` is any type, used only as a label.
- `extern type Db`, from §18, which came later: a type of its own, so a `Db` can't be passed
  where a `Socket` is expected, with type parameters if needed.

They differ where it matters. A `Handle` can't be held across a `!`. A value of an `extern type`
can, today:

```eye
extern type Db

@host("x.open")
extern fn open(path: String) -> Db

@host("x.close")
extern fn close(d: Db)

job j() {
  let d = open("x")
  sleep!(1 min)
  close(d)
}
```

This checks cleanly. The same job with `Handle(Db)` is refused: "d is a Handle, which can't be
saved, and it is still needed after a waiting call". Issue #42 (6.1, extern functions) still
says foreign objects cross the boundary as `Handle(name)`.

### Options

1. **Remove `Handle`; `extern type` is the one way.** A program-declared `extern type` has no
   `Save`, `Show`, `Eq`, `Hash` or `Schema`, so it can't be saved, printed, compared or asked of
   a model. The built-in types declared as `extern type` in `ir/builtin.eye` (`Int`, `List(a)`…)
   keep the traits they have.
2. **Keep both.** `Handle(name)` for a quick foreign object without declaring a type, `extern
   type` for a named one. Two ways to do one thing, and `Handle(Camera)` already needs `Camera`
   declared.
3. **Keep `Handle` as the only way,** and drop `extern type`. `extern type` already carries the
   built-ins' declarations, so this undoes a decision (the decision log's "Built-in docs").

### Recommendation

Option 1. One obvious way, a name that checks, and the saving rule applied to it.

### What changes

- SPECS §5 (special types), §11 (what has no `Show`, `Save` or `Schema`), §18 and §19 (schemas)
  say `extern type` instead of `Handle`. Open question 21 closes with a decision-log row.
- `ir/builtin.eye` loses `Handle`; `check/effects.go` (`unsaveable`) and `check/traits.go` treat
  a program's extern types as unsaveable and without derived traits. A test shows the job above
  refused.
- `testdata/syntax/valid/extern.eye` uses an `extern type`.
- Issue #42's body. Proposed text for its third point and its "Done when":

  > - Values crossing the boundary: `Bool`, `Byte`, fixed-width integers, `Float`, `Bytes` and
  >   `String` (copied), and values of an `extern type`, which foreign code owns: only extern
  >   functions make or take one, and it can't be saved, so it can't be live across a `!`.
  >
  > **Done when:** a test embeds the interpreter in a Go program that registers a host function,
  > and another calls `crc32` from zlib; both check that a wrong signature is reported rather than
  > crashing where that is detectable; and holding a value of an `extern type` across a `!` is a
  > compile error.

  Its second point, "loads a C function at run time without cgo", stays: `eye` is free of cgo
  (see [contradiction 3](#contradiction-3-the-typed-ir-in-specs-section-22-against-roadmap)).

## Q9 and Q4: long runs, and code changes while runs are paused

**Decided 2026-10-11: accepted.** Long runs: saved frames, in the interpreter in v0.3 (ROADMAP
task 7.3). Code changes: detect and refuse, with the replay check first. SPECS.md §15 and the
decision log's "Long runs" and "Job code changes".

### The problem

**Long runs.** The lab's train watcher (`testdata/lab/trains/trains.eye`) never ends while
someone has a train pinned:

```eye
@on(enters(travelling))
@priority(interactive)
job watch(t: Ref(Traveller)) {
  while true {
    let me = t.read!()
    if me.pins == [] && me.aboard == None { return }
    let at = now!()
    # … irishrail.movement!, t.remind!, notify.send! …
    sleep!(45 s)
  }
}
```

Each loop takes four or more steps, so a run keeps over 7,700 a day. A run resumes by replaying
every step it took, so each wake costs more than the last, and the run's whole life costs the
square of its age. `eye run` warns past 1,000 steps.

**Code changes.** Replay matches steps by position only (`replay` in `interp/jobs.go`; the step's
recorded call name isn't compared). Suppose a new version of `watch` reads the clock before the
entity. A watcher paused under the old version replays its first recorded result, a `Traveller`
snapshot, into `at`. Nothing notices; the run goes wrong somewhere later. Entities have
`@version` and `migrate from`, and `eye run` refuses rows it can't migrate. Paused runs have
nothing like it.

### Options

For long runs:

1. **Save live variables at each `!`** (SPECS §15's model, and what compiled jobs will do; see
   the [compilation design](/internals/compilation), section 4). A run's record stays
   the size of its live variables, so the watcher costs the same on its tenth day as on its first.
   Do it in the interpreter in v0.3 (task 7.3 pulled forward), or wait for v0.4.
2. **Restart from inside.** A loop ends by starting its job again (`watch.start(t)`, then
   `return`), so each run stays short. It works today, without a language change; open question
   9 asks whether the language should offer it (`continue`-as-new).
3. **Keep the history, compact the cost.** Not possible with replay: every step is needed to get
   back to where the run was.

For code changes:

1. **Detect, and refuse to start.** Each resume point gets an identity: the function, the place
   of the `!`, and the names and types of the variables live there. `eye run` refuses to start
   when a paused run's resume point no longer exists or changed, as it does for entity rows, and
   says which job and how many runs. The author keeps the old job under its name until its runs
   finish and gives the new code a new name, or ends the old runs. Until frames exist, replay
   compares each recorded step's call name with the call it replays, and faults the run ("the
   code of watch changed under this run at step 12") instead of decoding the wrong value.
2. **Finish old runs on old code.** The runner keeps the version that started each run and runs it
   there, as Temporal's worker versioning does. But entities are shared and migrate forward, so
   old code would read state shaped for new code.
3. **Job migrations.** A `migrate` for a job's resume points, mapping an old frame to a new one.
   Powerful, and much to specify; no program has needed it yet.

### Recommendation

- **Long runs:** option 1, done in the interpreter as soon as the IR pass exists (task 7.3),
  without new syntax. Until then the lab's watcher uses option 2, which is ordinary code. The
  retention times question 9 also raises can stay fixed until a program needs to set them.
- **Code changes:** option 1 now: the replay check is small and turns a silent error into a clear
  one. The start-up check comes with frames. Option 3 waits for a program that needs it; option 2
  is rejected because entities migrate forward.

### What changes

- SPECS §15: a paused run resumes only at a resume point that still exists with the same live
  variables; `eye run` refuses otherwise. Open question 4 closes with a decision-log row; open
  question 9's `continue`-as-new sub-question closes as "not needed".
- `interp/jobs.go`: `replay` compares the step's `Call` with the call being replayed.
- With frames: the run record keeps a resume point's identity; `eye run` checks it at start, as
  `TestRunMigratesStoredEntities` does for entities; `eye runs` names runs whose code changed.
- ROADMAP: task 7.3 (jobs as state machines) moves into v0.3, with 7.1 before it.
- The lab: `watch` restarts itself until frames land.

## Q15: typed choices

**Decided 2026-10-11: accepted** (option 2). SPECS.md §11 and the decision log's "Typed choices".
The question about labels below was not decided: it stays in open question 15, and until it is,
a label is the variant's `Show`.

### The problem

The lab's NetBird admission asks the operator with three buttons, then matches the answer as a
string ([`testdata/lab/access/access.eye`](https://github.com/ruxxzebre/eyelang/blob/dev/testdata/lab/access/access.eye#L151)):

```eye
let step = match answer.choice {
  "guest" => Admit(user: user.id, groups: toSet(["guests"]))
  "member" => Admit(user: user.id, groups: toSet([people, "members"]))
  _ => Reject(user: user.id)
}
```

A typo in `"member"`, or a renamed button, compiles and silently rejects someone the operator
let in. The `_` arm is needed because a string has infinitely many values, so the checker can't
say that a button was forgotten either. The buttons are records whose ids repeat the strings:
`let member: Choice = { id: "member", label: "Member" }`.

### Options

1. **Compare with the constants.** `if answer.choice == member.id then … else if …`. Names are
   checked, so a typo in a constant's name fails, but the chain isn't exhaustive and still ends
   in a catch-all. No change to the language.
2. **Derive conversions for enums.** A variant type whose variants have no fields (an enum) gets
   two functions on the type, derived like `Show`: `T.variants()`, every variant in order, and
   `T.parse(s)`, `Some` of the variant named `s`, else `None`. A variant's `##` comment is its
   label. No syntax: these are calls on a type, as `Lexer.new` is.
3. **Typed contracts.** Notify's `ask` and `Answered` become generic over the choice type, so the
   answer arrives as that type. Contracts and events have no type parameters today, and Notify
   receives strings on the wire anyway.

### Recommendation

Option 2. The lab's code becomes:

```eye
## How someone new is let in.
type Admission =
  ## Guest
  | Guest
  ## Member
  | Member
  ## Refuse
  | Refuse

let step = match Admission.parse(answer.choice) {
  Some(Guest) => Admit(user: user.id, groups: toSet(["guests"]))
  Some(Member) => Admit(user: user.id, groups: toSet([people, "members"]))
  Some(Refuse) | None => Reject(user: user.id)
}
```

A misspelt `Some(Membr)` is an undefined variant, and a forgotten `Some(Member)` makes the match
non-exhaustive. The buttons come from the type, so they can't drift from the match. A `Choice`
helper in the lab's `notify` module builds them from `Admission.variants()` and the labels.

**Question:** should the label be the `##` comment, as open question 15 suggests, or a derived
`Show` with an impl where the text differs? `##` comments are documentation; using them as user
interface text mixes two jobs. A `Show` impl is the existing way to choose how a value is written.

### What changes

- SPECS §5 or §11: enums get `variants()` and `parse(…)`, derived. Open question 15 closes with a
  decision-log row.
- The checker and interpreter: the two derived functions (the checker already derives traits for
  types whose parts have them).
- The lab: `access.eye` and `approving.eye` use enums for their buttons; `trains.eye`'s
  "Boarded" and "Not taking it" buttons too.
- Docs: the guide's contracts page, where questions and buttons are shown.

## Module roots in eye.toml

**Decided 2026-10-11: accepted** (option 1). SPECS.md §13 and the decision log's "Module roots".

### The problem

`eye run` runs every job of every module a program imports. So the lab can't be deployed one
workload at a time: running `testdata/lab` would start the notifier, the train reminders and the
NetBird sync as well as the digest. The digest's deploy is therefore a program of its own,
`testdata/lab/deploy/digest`: an `eye.toml`, a `digest.eye` that only says `import digest`, and
seven symbolic links (`digest`, `home`, `notify`, `telegram`, `miniflux`, `web`,
`flaresolverr`) to the lab's module directories. The README copies it with `cp -RL` to follow
them. Links are easy to break, don't work the same on Windows, and must list every module the
digest imports, directly or not.

`eye.toml` marks a program's root and is empty today; what it holds beyond that is open question 5.

### Options

1. **`eye.toml` names more directories to find modules in.** `import digest` looks below the
   program's own directory, then below each directory listed, in order. The deploy program
   becomes its `eye.toml`, saying `modules = ["../.."]`, and its `digest.eye`.
2. **`eye run` takes the modules whose jobs run.** `eye run testdata/lab --jobs digest` runs only
   the jobs of `digest` and what it imports. No deploy program is needed, but what runs in
   production is then in a command line rather than in the repository.
3. **Wait for packages** (open question 5), and make the lab's shared modules a package the
   digest depends on. The right end state for separate repositories; much more than this needs.

### Recommendation

Option 1. The deploy stays a program in the repository that says what it is, `eye check` and the
editor find its modules by the same rule, and it is the first thing `eye.toml` holds, which open
question 5 can grow from. A listed directory must be inside the repository, or the build's
context, so that a copy of the program has everything it needs.

### What changes

- SPECS §13 ("A program's root is marked by `eye.toml`"): the `modules` key, with the search
  order. Open question 5 notes that `eye.toml` now holds one key. A decision-log row.
- The checker's module lookup (`check/index.go`) reads the key; a test shows an import found
  through it, and an error naming the directories searched when none has the module.
- `testdata/lab/deploy/digest` loses its links, and the README's deploy steps copy the lab
  instead of following links.

## Contradiction 1: jobs as processes against one runner per store

**Decided 2026-10-11: accepted** (option 1). ROADMAP.md's v0.4 row and issue #8 say "jobs compiled
to state machines; a waiting run is a record in the store".

### The problem

- ROADMAP's v0.4 row says "jobs run as short-lived processes", and issue #8 says "jobs compiled to
  native state machines that run as short-lived processes".
- SPECS §1 says "a job that is waiting is a saved record in the store, not a process or a thread".
- Open question 28: one runner writes a store at a time, held by a file lock (the journal) or an
  advisory lock (Postgres). Each runner keeps the whole store in memory. Several writers need
  runs claimed with leases that expire, and the in-memory copy would no longer be the whole truth.

If each run were its own process writing the store, v0.4 would need open question 28 answered
first, and the store would have to stop living in one runner's memory.

### Options

1. **"Process" means the runner.** v0.4's runner is a native process that executes runs as
   compiled state machines. A waiting run is a record, not a process. One runner per store
   stays; open question 28 stays separate.
2. **Worker processes under one runner.** The runner stays the only writer. To execute a run, it
   hands the run's frames to a short-lived worker process, which runs to the next checkpoint and
   sends the commit back. A crash in C code (`ffi`) kills one run, not the runner, and `@memory`
   can be the worker's memory, enforced by the operating system. Each step pays for a process
   start and a copy of the state.
3. **Each run a process that writes the store.** Needs leases, claims and a store that isn't held
   in one memory: open question 28 first, and with it most of a distributed runner.

### Recommendation

Option 1 for v0.4. It keeps every promise SPECS makes (waiting costs no memory, resuming takes
milliseconds), and phase 7's exit test (idle cost and resume time of the digest) measures it.
Option 2 can come later for runs that call C code or need hard memory limits, and needs no
language change. Option 3 belongs with open question 28 and with pipelines compiled to SQL, which
both need a store that doesn't live in one runner's memory.

### What changes

- ROADMAP: the v0.4 row and "What's next" item 5 say "jobs compiled to state machines; a waiting
  run is a record in the store". Phase 7's exit test stays as it is.
- Issue #8's goal is reworded the same way.
- SPECS needs no change; open question 28 stays open.

## Contradiction 2: when a job's state is saved, SPECS section 15 against section 22 and the code

**Decided 2026-10-11: accepted** (option 3). SPECS.md §15 states the guarantee now; it drops the
replay sentence when task 7.3 lands.

### The problem

- SPECS §15: "At each `!` the job's live variables are saved, and the call's result is recorded.
  A resumed or restarted run continues from the last checkpoint."
- SPECS §22, step 5: "The interpreter resumes a job by running it again from the start: each step
  it already took returns its recorded result."
- The code does what §22 says: `jobRun` holds a log of steps, and resuming replays them.

The guarantees a program sees are the same either way: a recorded call never runs again, and a
resumed run sees the values it saw. But the two differ where people notice: a run's cost grows
with its age under replay (Q9); a code change shifts replayed steps (Q4); output must be kept
quiet on replay (Q22); `@memory` measures the step log (Q26).

### Options

1. **Make §15 state the guarantee, not the mechanism.** "A resumed run continues from its last
   checkpoint: no recorded call runs again, and it sees the values it saw. The interpreter does
   this by replaying the run's steps; compiled jobs save their live variables (section 22)."
2. **Make §15 true now.** Jobs resume from saved frames in the interpreter (task 7.3, pulled
   forward), and §22 step 5 changes instead.
3. **Both, in order.** Option 1 now, so the spec is true today; option 2 when task 7.3 lands, and
   §15 then drops the sentence about replay.

### Recommendation

Option 3. The spec should never describe code that doesn't exist, and the user decides
separately when frames arrive (see Q9 and Q4 above).

### What changes

- SPECS §15's first bullet, as in option 1. Open question 9 and §15's "Memory" bullet already say
  replay is the interpreter's way; they stay consistent.
- Later, with task 7.3: §15 drops the replay sentence, §22 step 5 says the interpreter resumes
  from frames.

## Contradiction 3: the typed IR in SPECS section 22 against ROADMAP

**Decided 2026-10-11: accepted** (option 1). SPECS.md §22 describes the IR as built, with the typed
IR as a step of its own. Since `eye` stays free of cgo, "without cgo" stays in step 5 and in issue
#42: the interpreter loads C libraries at run time without it.

### The problem

SPECS §22 step 5 reads: "Typed IR, plus a reference interpreter on that IR … `extern` functions
from the Go host or from C libraries loaded at run time (without cgo). `entity`, `job`, `contract`
and `view` are each lowered into core IR by their own pass." Three things in it are not what was
built or planned:

- **Typed IR.** ROADMAP moved typed IR to phase 7 (7.1, issue #28), and the interpreter runs the
  untyped IR the thin slice built. ROADMAP's phase 3 says so; §22 doesn't.
- **Domain constructs.** Entities, jobs, views and contracts are declarations in `ir/`, and the
  interpreter runs them directly. Nothing lowers them into core IR.
- **Without cgo.** When this was written, track A was expected to move the release build to cgo
  with `zig cc` and replace the hand-written parser with tree-sitter. It didn't: the hand-written
  parser stays, tree-sitter is used only in tests, and `eye` itself stays free of cgo. So "without
  cgo" is still a constraint, and §22's parsing part stands as it is.

### Options

1. **Rewrite step 5 to what was built, and say where the rest goes.** Step 5 becomes "an untyped,
   resolved IR, and a reference interpreter on it", with entities, jobs, views and contracts as
   declarations of the IR. A new step before code generation says "typed IR (7.1), then
   lowering of the domain constructs and specialisation", as the
   [compilation design](/internals/compilation) proposes in its section 5.
2. **Keep §22 as the plan, and make ROADMAP follow it.** Type the IR now and lower the domain
   constructs before v0.3. A large change with no program asking for it yet.

### Recommendation

Option 1. §22 should describe the order things are built in. The compilation design proposes
that the interpreter run the typed IR too once 7.1 lands, so the end state is close to what step 5
describes, reached in a different order. For C libraries, `@library` should use whatever the
build has. `eye`'s build has no cgo, so the interpreter loads a C function at run time without it,
and a compiled program links it directly.

### What changes

- SPECS §22: step 5 and a new step before step 7 as in option 1; step 7 links the
  [compilation design](https://github.com/ruxxzebre/eyelang/blob/dev/docs/internals/compilation.md).
  The "without cgo" clause stays, since `eye` is free of cgo.
- Issue #28's body: the IR is typed for compilation, not for phase 4, and its "Done when" is the
  one the compilation design proposes for 7.1.
- Issue #42's body keeps "loads a C function at run time without cgo".
