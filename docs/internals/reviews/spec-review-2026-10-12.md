# SPECS.md review, 2026-10-12

A broad, critical review of SPECS.md as a language specification, written by a reviewing agent
on 2026-10-12 against `dev` at `2020c6e` (SPECS.md was 2,051 lines). Line numbers are SPECS.md's
at that commit unless a file is named; they drift as SPECS changes. Findings marked
**[verified]** were reproduced with a scratch program; findings marked *(unsure)* are
inferences. The conclusions drawn from it, and what to do about each finding, are in
[conclusions-2026-10-12.md](./conclusions-2026-10-12.md).

Some findings were fixed after the review ran: G.4 to G.6 (`Type.f()` through a module, a
self-naming alias, an `impl` on an entity), C.3 (schedules), 6.3 (`@fresh` and `@destructive`,
which now gives `Version` a type) and 6.13 (serving contracts over HTTP) landed the same day.

---

## Top 10 findings, ranked

### 1. Entity methods can have outside effects: the spec contradicts itself, and the code follows the permissive side [verified]

- **The spec says both things:**
  - §12, L831–833: "Outside jobs the same functions are called without `!`: in `main` …, in tests,
    **and in entity methods and served operations**."
  - §14, L1014–1018: "**Methods never wait.** They can't use `!`, so they can't call other
    services; those calls belong in jobs… Because entity methods have no outside side effects,
    the runtime can safely retry one after a conflict."
- **What the code does:** these all pass `eye check`, and the test runs:
  - an entity method calling `notify.send(…)` (a contract command);
  - `fs.writeText`, `now()` and `random.int` in an entity method;
  - `sleep(1 s)` in an entity method, which blocks while the entity is held.
- **Why it matters:** this breaks pillar 2 (atomic, retryable methods) and pillar 4 (keyed calls,
  never blindly retried). If a conflict retries the transaction, the network call or file write
  happens twice.
- **Fix:** keep §14's rule and make §12 say "in `main`, in tests; in entity methods and served
  operations only starting a job and publishing, which commit with the transaction". Have the
  checker refuse `net`, `fs`, `process`, `clock`, `random`, `env`, `ffi`, `secret` and `wait` in
  entity methods and in `impl Contract` functions. If reading the clock inside a transaction is
  wanted, decide it separately.

### 2. Reading an entity in a job without a step: a resumed run sees a different value [verified]

- **The promise:** L1062–1063 says a resumed run "sees the values it saw". L1019–1023 says a job
  reads an entity with `w.read!()`. `w.read()` without `!` in a job is correctly refused.
- **What the code does:**
  - A field read on a `Ref` (`let before = w.phase`) in a job compiles, and so do reads of a view
    or entity set (`busy |> size`, `W.all |> size`).
  - None of these is a recorded step.
  - In a test, a job read `w.phase` (Asleep), slept 10 minutes while the test changed it, then
    woke. The run replayed from its start, since no checkpoint had been taken yet, re-read the
    field and recorded `Running`. A run that keeps its own values should have recorded `Asleep`.
- **The spec is silent:** it never says whether `ref.field` is legal (the tour's test uses
  `w.phase` at L209, outside a job). It also never says how a job reads a view.
- **Fix:**
  - State that `ref.field` outside jobs is shorthand for `ref.read().field`.
  - In a job, make `ref.field`, view reads and `E.all` either errors ("read it with
    `w.read!()`") or recorded steps.

### 3. Nested entity calls that come back to an entity lose a write [verified]

- **The spec:** L1005–1006 says "A method may call methods on other entities, and the whole call
  tree commits or rolls back together." It says nothing about a call tree that returns to an
  entity already in it.
- **The test:** `A.ping` does `n += 1` and then calls `B(id).pong()`. `B.pong` calls
  `A(id).bump()`, which also does `n += 1`. Afterwards `A("x").read().n` is **1, not 2**. One
  update is silently lost.
- **Fix:** define re-entry. Either the inner call sees and updates the outer call's in-progress
  state, or calling back into an entity already in the call tree is an error with a message. Add
  a test either way.

### 4. Effects missing from public signatures [verified]

Pillar 5 and L777–779 say "A public function without a `uses` clause is pure, and the compiler
holds it to that". These all pass `eye check`:

- **Entity methods:** `pub fn` entity methods never need `uses`, even with `fs`, `clock` and
  `random` in the body. `impl T` methods do need it, and callers of entity methods do get the
  effects.
- **Public jobs:** a `pub job` that does `fs.readText!`, `now!()` and `sleep!` needs no `uses`.
  The grammar allows a `uses` clause on jobs, but the spec never says whether one is required.
- **Starting a job:** this is not an effect. `pub fn starter() -> Run(()) { worker.start(1) }`
  passes as "pure"; §12's table has no row for it.
- **Calling a waiting function without `!`:** this drops `wait`. `pub fn pause() { sleep(1 s) }`
  passes as pure, and `main` can call `sleep(1 s)` without listing anything. (`ir/builtin.eye`
  says `sleep` blocks outside jobs; SPECS never does.)
- **The key rule for jobs started from `main`:** the rule at L888–890 checks only `main`'s own
  body. `fn kick() { ring.start("kitchen") }` called from `main` passes, and starts a new run on
  every restart.

**Fix:** decide and write down whether entity methods and jobs carry `uses`. Add an effect for
starting jobs (for example `start J`), which also lets the key rule follow calls. Say that a
call to a function with `uses {wait}` outside a job either keeps the effect or is refused.

### 5. `!` outside jobs and "waiting functions" are never defined [verified]

- **Row 104 (L104):** "`!` marks durable steps and is used only in jobs."
- **§12, L752–754:** the effects example is
  `pub fn summarise(…) uses {wait, net Llm.complete} { llm.complete!(…) }`, an ordinary function
  with a `!` call.
- **§12, L759:** `wait` is "allowed only in jobs and **waiting functions**". The term is never
  defined.
- **Elsewhere:** L1067 ("a function with `!` steps of its own") and open question 23 also assume
  such functions exist.
- **What the code does:** `!` is accepted in any function, `pub fn waiter() uses {wait} { sleep!(1 s) }`
  included. A job must then call it as `waiter!()`.
- **Fix:** add a short rule to §12. A *waiting function* is one whose body has `!` steps. It has
  `wait` in its effects, and only jobs and other waiting functions may call it, with `!`. Change
  row 104 to "only in jobs and the functions they call". Say what `main` may do with such a
  function.

### 6. Ending an entity is undefined, and `end(ok)` collides with a built-in [verified]

- **The collision:** L995's `pub fn retire(ok: Approval) { end(ok) }` resolves to the string
  function `end(s: String) -> Offset` (`ir/builtin.eye` L442). The error is "expected String,
  found Approval".
- **The missing rules:** the spec never says what an ended entity is. Entities are virtual (row
  81, L1003: "exists on first use"), so the next `Player(id)` would recreate it from defaults.
  That means deletion with an Approval doesn't actually delete anything.
- **Fix:** use a name that can't collide, such as a method `self.end(ok)` or `retire`. Then
  define:
  - a tombstone: a call on an ended entity returns an error or faults;
  - whether its `Ref` stays a valid value;
  - that views fire `leaves`;
  - what happens to stored rows and backups;
  - who checks the `Approval` (its `what`?).

### 7. Results and command outcomes can be silently thrown away [verified]

- **The tour does it:** L192 `notify.send!(owner, …, key: …)` stands alone as a statement, so a
  `Refused` or `Uncertain` reply goes nowhere.
- **The code allows it everywhere:** `fs.writeText("a", "b")` as a statement in `main` compiles.
  L1278–1280 only stops an `Uncertain` from being *passed on*; it can still be dropped.
- **Why it matters:** this is the easy-but-wrong program that pillar 4 and "How to decide" item 1
  warn about.
- **Fix:** a statement whose value is a `Result` or a command's outcome must be used, matched or
  explicitly discarded (`let _ = …`). Put the rule in §10 and §17 and update the tour.

### 8. Stale text that contradicts the code or the rest of the spec

| Where | Says | Reality |
|---|---|---|
| Row 96 (L96), §16 L1214 | Pipelines over entity sets "compile to SQL" | Not built. ROADMAP "What's next" recommends saying they run in memory. |
| §22 L1722–1724 | "The interpreter resumes a job by running it again from the start" | Since 7.3 it resumes from statement checkpoints (§15 L1063, row 131). |
| Row 80 (L80), §15 heading L1061 | "Every `!` call in a job is also a **checkpoint**" | §15 L1063–1065: a checkpoint is taken at statements, once steps have been taken. |
| §15 L1096–1097 | "Until traits are checked, `Store` is the Go interface…" | Traits are checked; `check/runtime/store.eye` L55 declares `trait Store(s)`. |
| Q24 L1972–1973 | "The decision log's own example of `dyn` is choosing a store" | Row 110's example is `fn notifier(config) -> dyn Notifier`. |
| `@priority` row L1183 | "lower priorities pause at their next checkpoint" | Q26 L1986–1988: nothing is paused mid-run. |
| Row 117 (L117) | "(open question 32)" for string positions | Q32 is time-travel debugging. |
| L3 | "decisions come from design sessions on 2026-10-10" | Many rows are from 2026-10-11. |
| §21 L1657–1679 | "Proposed handling", "Delete now", "Before deleting anything, tag…" | All done; Eyelang 1 is gone. |

**Fix:** do one sweep. Collapse §21 to two lines, and move dated and implementation notes to
`docs/internals`.

### 9. Examples with real errors (not just missing context) [verified with `eye check`]

- **§8 L553** `List(dyn Handler)`: the checker says "dyn takes a trait, and Handler is a type".
  `Handler` is a built-in type (`ir/builtin.eye` L136, the type `with` takes), and §12 L803 also
  declares `type Handler = {…}`. Three meanings for one name.
- **§10 L614–618** `BadNumber(ParseError)`: there is no `ParseError`; `parseInt` returns
  `Result(Int, String)`.
- **§10 L614–618 and §13 L864** `FsError` unqualified: it must be `fs.FsError`. §13 fails even
  with `import fs`; §10 doesn't import `fs` at all.
- **§11 L697** `map(fn a: { id: "{a}", label: "{a}" })`: "which record type is this?" The example
  breaks §5's nominal-record rule (L342–351).
- **§12 L793–794** `fs.readText(p) |> size`: `size` of a `Result` is an error.
- **§13 L956** `fs.lines(path)`: no such function in `check/std/fs.eye`.
- **§15 L1155** publishes `{ id, approved, by }`, but `approvals.Answer` has `what` and no `id`
  (C.8/#71 isn't built). Mark it as the target design until it lands.
- **§19 L1591–1592** `codex.start!(…).result!()?`: nothing gives the result a type, and §19's own
  rule (L1509–1511) says "When nothing says, write it".
- **§19 L1558** `tools: [searchIssues, label]`: a list of functions with different types, which no
  rule here can type.
- **§11 L655** `self.nick` on `Player`, but §14 L986 says `nick` was renamed to `name`. (`impl` on
  an entity was also ignored at the time; it is an error since G.6.)
- **§15 L1088** `World(w).markAsleep!()`, where `w` is already a `Ref(World)` in the tour. It
  should be `w.markAsleep!()`.
- **§9 L574** `receipt.age`, but §5 L353 makes `receipt` a `ReceiptId`.
- **Tour, L160:** `wake.start(id)` uses a job that is never shown. It, `PlayerId`, `SleepError`
  and `SleepRefused` live in `testdata/programs/tour/worlds.eye`; the spec should say so.
- **Tour, L188:** the comment "exactly once: same transaction as the checkpoint" sits right under
  `zeropod.checkpoint!`, so a reader takes "checkpoint" to mean zeropod's. See the terminology
  notes below.
- **Inconsistent examples across sections:**
  - Three incompatible Notify contracts: `send(to, text)` at L1229;
    `send(event, title, priority, to)` at L1343; `ask`/`answered` at L1149–1152.
  - `summarise` is a pub fn at L752, a job at L1487, and `summarise!(items)` returning text at
    L1055.

### 10. Rules the code chose that SPECS never records [verified]

- **Built-in collection functions work as methods:** `xs.first()`, `s.size()` and `s |> chars` all
  compile. This contradicts L532–534 ("`x |> f(a)` calls an ordinary function; `x.f(a)` calls a
  method") and L1010–1011. User functions don't work this way (`3.twice()` is "Int has no method
  twice"). It also gives built-ins two spellings, against "one obvious way".
- **Imported variants work unprefixed everywhere:** `fn t() = Blocked` compiles. §9 L597–599
  grants this only to patterns.
- **Entity methods' lambdas may capture state fields** (`filter(fn p: p != owner)`). §7 L491–492
  forbids capturing `mut` parameters, and entity methods are `mut self` (L498, L1007).
- **A `state version` field is silently replaced** by the snapshot's transaction `version`:
  `read().version` gave 1, not 8. There is no checker error.
- **Faults beyond the six listed (L1158–1159) become `Unreachable(msg)`** (`interp/jobs.go`
  L183–189). That covers a 4xx reply (L1359), a replay mismatch (L1079), a missing fake, and an
  unknown model price.
- **Some standard modules need `import`, others don't:** `fs`, `random`, `process`, `http`, `ai`
  and `approvals` do; `env` and `log` are ambient. The spec doesn't say which; §10 and §12 use
  `fs` and `random` without importing them.
- **Integer ranges don't wrap, time ranges do:** `5..2` over `Int` is empty, while `23:00..08:00`
  wraps (L362). `(0..=5).end` is 5, so the `end` field doesn't say whether a range includes it.

**Fix:** record each in its section, or change the code.

---

## Full lists by category

### 1. Contradictions

**Inside the spec**

- Entity methods and outside effects: §12 L831–833 against §14 L1014–1018 (top 1).
- `!` only in jobs (row 104) against waiting functions (L752, L759, L1067, Q23) (top 5).
- What a checkpoint is: row 80 and L1061 against L1063 and row 131 (top 8).
- Resume by replay from the start (§22 L1722) against checkpoints (§15) (top 8).
- Function types without `uses`:
  - L799: "A function type without `uses` is pure".
  - L812–814: "A parameter's function type without `uses`… takes a callback, which may do
    anything".
  - The same `fn(a) -> b` means different things by position, and L799's sentence is wrong as
    written. (`grammar.js` L823 repeats "is pure".)
- The newline rule:
  - Row 119 continues a line that starts with "an operator, `.`, `,`, `:`, `=` or `else`".
  - §4 L296 adds `?` and `->`.
- Braces:
  - Row 120 lists what opens a block.
  - §4 L312–313 adds `let … else` and `test "…"`.
- Impls:
  - §8 L525 and §11 L666 say an `impl` goes only in the module of the type or trait.
  - §17 L1312 serves `impl notify.Notify` from another module (the lab does this in
    `notifier.eye`). The exception for contracts is never stated.
- Ports:
  - L1248–1249 / row 94: "Callers never write a port; they get it from the contract, so no call
    can go to a wrong or undeclared one".
  - L1362–1363: the service's URL, which carries a port, comes from `NOTIFY_URL`.
  - Which one wins?
- `@destructive`:
  - On a command, it adds an `Approval` parameter (L1302).
  - On a function used as a model tool, it pauses for a person (L1550, L1564).
  - Same attribute, two meanings.
- `@priority`'s table row (L1183) against Q26's honest description.
- `@memory` and "All of them are enforced by the runtime" (L1188) against Q26.
- Q24's premise against row 110.

**Spec against code (verified, beyond the top 10)**

- `?` wrapping: with two variants that both hold `String`, the message says "none of its variants
  holds a String" instead of "several". This is a small checker bug.
- `@fresh`'s `basedOn: Version` (L1300): there was no `Version` type; the snapshot's `version` was
  an `Int`. (6.3 has since made `Version` a type.)
- Approvals by `id` and only by `from` (row 128, L1130–1139) aren't built. The prelude says
  answers match by text; status.md marks it partial (#71). The spec text should say it is decided
  and not yet built.
- `check/prelude.eye` L98 still says "a Secret, **a Handle** and a function have none"; `Handle`
  is gone (row 130).

### 2. Gaps (rules a reader or implementer needs)

- **Ended entities** (top 6).
- **Re-entrant entity calls** (top 3).
- **Reading entities and views in jobs** (top 2).
- **Retrying a transaction after a conflict:** what happens to effects it already did? (This only
  matters if top 1 isn't fixed.)
- **Which effects public items must state:** entity methods, jobs, `start`, wait-effect calls
  without `!` (top 4).
- **Discarded values:** no rule at all (top 7).
- **Faults:**
  - which `Fault` a 4xx reply, a replay mismatch, a missing fake or an unknown price produces;
  - what a fault means outside a job: a query unavailable for a minute (L1292–1293) or a 4xx in
    `main`, a test or an entity method — "the run ends", but there is no run.
- **Events have no caller guard.** `@allow` covers commands and queries only (L1253), so any
  module may publish any contract's event, `approvals.answered` included, with any `by`.
- **A module with two contracts:** L1303 says how "a module that declares one contract" offers its
  operations. Two contracts would make `module.op` and `module.event` ambiguous.
- **`main`'s arguments:** L865 uses `args |> drop(1)`, but what is `args[0]` under
  `eye run DIR -- …`?
- **Arithmetic on built-in types:** L741–743 lists combinations "that make sense" without saying
  the list is complete.
  - `Date - Duration` is used at L1053 but isn't listed; `Duration / Int`, `Int * Duration` and
    `Duration - Duration` work.
  - `Duration * Float` fails (`expected Int, found Float`) even though `1.5 h` is a literal.
  - **Fix:** give a full table.
- **Slices:** what type is `xs[2..5]`, and what does sharing storage mean under value semantics
  (L364–366)?
- **Strings:** `s.charAt(i)` returns `Option(Char)` (`None` at `s.end()`); L371 says "gives the
  character there".
- **`if c then x` with no `else`** has type `()`. That's reasonable, but unstated.
- **Record literals:** a record's type can come from later use (`var counter = { hits: 0, last:
  None }` at L473 checks). L390–393 grants this to string literals only, and L351 suggests it's
  an error.
- **Keywords:** `then` is missing from §4's lists (L275–287). `grammar.js` L511 makes it
  contextual after `if`.
- **The grammar sketch (§23)** lacks:
  - `extern type` (L1771, L1777);
  - `where` on `extern fn`, which §18 L1445 allows;
  - `uses` on function types (L1845);
  - `_` and tuple patterns as lambda parameters (L338, L1809).
- **`@destructive` model tools** pause "until someone approves" (L1565), but whom? `approval!`
  needs `from:`.
- **Tests and approvals:** the test helper `approve(what, by: who = "test")` (`ir/builtin.eye`
  L194) will be refused once "only `from` may answer" lands, unless the job asked "test".
- **`eye test` and `eye.toml`'s `modules`:** L1638–1639 runs "the modules it imports from its own
  directory included". Are modules found through `modules` directories included?
- **Secrets:** which comes first, the environment variable or the file under `$EYE_SECRETS`
  (L1626–1627)?
- **Approval identity:** comparing as text (L1132) leaves open how a `UserId("paul")` compares
  with a Telegram chat id.

### 3. Unclear or ambiguous rules (two readings each)

- **Ranges, L360–361:** "fields `start` and `end`". Either `end` is always exclusive, or it is
  whatever was written (the code: `(0..=5).end == 5`). Is `5..2` empty or wrapping? It is empty
  for `Int` and wraps for times.
- **L536–537:** "A lower-case name followed by `.` is a module". Either everywhere (which would
  make `w.read!()` a module path) or only in types. It should say "in a type".
- **L1608–1609:** "`every(15 min)` fires at each multiple of its interval since midnight UTC".
  Either it resets each midnight (then `every(7 min)` has a 5-minute gap at 00:00) or it counts
  since the epoch.
- **L1184:** `@deadline(13:40, in: zone)`. For a run that starts at 14:00, either today's 13:40
  (it fails at once) or tomorrow's.
- **L1114 and L1124:** `poll!(…, fn { miniflux.ready!() })` "keeps only its outcome". Either the
  inner `!` calls are steps, contradicting "keeps only its outcome", or they aren't steps,
  contradicting "`!` marks every recorded step".
- **L1132:** "`by` must be the request's `from`, both as text". Either the `Show` form or the JSON
  form, and either way two different identity types can collide or never match.
- **L812–816:** whether a function type nested inside a parameter type (`Option(fn() -> a)`,
  `List(fn() -> a)` as in `race`) counts as a callback. `race` says yes; the rule is phrased about
  "a parameter's function type".
- **L911:** "each must stay inside the repository". What if the program isn't in git?
- **L1365–1367:** "Reading a field that no recording contains is a compile error". Which
  recordings? The `from:` path is a string the test passes at run time, so the compiler can't
  know it without new rules.
- **L1170:** a job can be started "from entity methods, from `main` and tests, and from jobs".
  What about plain functions? (They can, which is how the key rule is bypassed; see top 4.)

### 4. Examples

All of these are listed under top 9. In addition:

- **§13 L929–930:** `let (user, orders) = both(…)`. The extracted file is checked as module level
  ("a module-level let binds one name"), which conflicts with L1766–1767: "Every `eye` example …
  is a whole file or the statements of a function body". The tool should wrap statement-only
  examples, or the example should be marked.
- **§15 L1149:** `choices: [yes, no]` with `press.choice == yes.id` compares strings, while row 133
  and §11 say choices should be matched as typed variants (`Admission.parse`).
- **The fragments are fine as they are.** ROADMAP C.7 says 4 of 33 examples check; there are 38
  example files now, with 6 clean in isolation. The real errors are the ones in top 9.

### 5. Design risks against the pillars and "How to decide"

1. **Entity methods with side effects** (top 1). Example:
   `pub fn join(p) { …; notify.send(owner, "joined", key: …) }` compiles today, runs inside the
   transaction, and repeats on retry.
2. **Dropped outcomes** (top 7). The tour's own `notify.send!(…)` drops `Refused` and `Uncertain`.
3. **Nominal record literals in lambdas and pipelines.** The obvious data program fails:
   ```eye
   let rows = users |> map(fn u: { name: u.name, age: u.age })   # error: which record type?
   ```
   §11's own `buttons` example fails the same way. Q25 is open; this is evidence that it hurts
   ordinary code. Options: let a lambda's body take the element type from where the result goes,
   or allow `User { … }` at the literal.
4. **Closures can't read a `var`** (L411, L491) [verified]. This is a very common loop shape:
   ```eye
   var seen: Set(String) = {}
   for item in feed {
     let fresh = item.links |> filter(fn l: !(seen |> contains(l)))   # error: copy into a let
     seen = seen + toSet(fresh)
   }
   ```
   Under value semantics, capturing the current *value* is sound. Allow reading a `var` in a
   callback that doesn't escape, or capture by value. (Rule 6 in "How to decide": strict, but
   clumsy here.)
5. **Division needs `NonZero`.** `sum / (xs |> size)` needs a `match nonzero(…)` every time an
   average is taken. Consider a `div` that returns an `Option`, or `/` on `NonZero` produced by
   `size` of a non-empty list.
6. **A rule shaped by the interpreter.** L1067–1068 says "a loop that never ends belongs in the
   job's body, not in a function it calls once". The natural refactor (extracting `watchLoop!()`)
   makes a run grow without bound, and compiled code won't have this limit. Mark it as a
   limitation of the interpreter, not a language rule, or save the frames of called functions
   too.
7. **Approvals' safety rests on the routing job's honesty.** The obvious shortcut
   `approvals.answered.publish!({ id: r.id, approved: true, by: r.from })` passes the
   `by == from` check. Combined with events having no `@allow`, any module can grant consent.
   Consider a typed identity supplied by the contract that delivers the press, rather than text
   the program writes.
8. **Concurrency in jobs is unvalidated.** Tasks in jobs "only compute" (L976–979), so fetching 20
   feeds in parallel in a job needs 20 child runs and `all!`, which isn't built. The lab uses no
   concurrency at all (`parallelMap`, `both`, `all!` and `first!` have no hits in `testdata/lab`).
   This needs a real workload before it is settled.
9. **`main` runs again on every start** (L883–890). Only direct, keyless job starts are guarded.
   Entity calls and publishes repeat, and so do starts through a helper. Example:
   `pub fn main() { Ledger("x").credit(10) }` credits on every restart.
10. **Hidden memory cost of slices.** Slices "share storage" (L364), so a 10-byte slice keeps a
    100 MB `Bytes` alive. Pillar: lean on memory, costs visible.
11. **Overloaded names** cause real collisions:
    - `end`: a string offset, a range field, ending an entity;
    - `start`: a job, a string, a range;
    - `first` and `all`: collection functions, and the waits `first!`/`all!` (status: "resolved
      as the collection functions");
    - `join`: joining strings, and `run.join!()`;
    - `version`: `@version` and the snapshot field (and a state field);
    - `Handler`: three meanings.
    `first!` breaks the rule that `!` only marks the same call as a step. Add a glossary and
    rename the waits (e.g. `allDone!`, `firstDone!`).
12. **Three timeout conventions:** `poll!` gives `false`, `next!` gives `None`, `approval!` gives
    `Expired`. Pick one shape.
13. **Function types mean different things by position** (contradiction above). This trips
    readers and agents.
14. **Strings count characters:** `size`, `take` and `drop` on strings cost O(n) with no sign at
    the call. This is minor.

### 6. Open questions 1–34

| # | State |
|---|---|
| 1 | Time conditions in views: **open**. |
| 2 | Coherence: **decided** (row 101). The text "decided… no" no longer states the question; rephrase or drop it. |
| 3 | Exposing entities and jobs as contracts: **open**. |
| 4 | Job code upgrades: **decided and built** (row 132, status ✓). Drop it, keeping the "migrate for resume points" note as a new question if wanted. |
| 5 | Packages: **partly decided** (`eye.toml` root and `modules`); lock and registry still **open**. |
| 6 | REPL and jobs: **decided and built**. Drop it. |
| 7 | Macros after v1.0: **parked**, not a question for now. Move it to a "later" list. |
| 8 | Multi-line strings: **decided and built**. Drop it. |
| 9 | Journal: **decided and built**. The long-runs bullet says "decided and done". Only the retention settings remain **open**; the multi-runner bullet duplicates Q28. Trim it. |
| 10 | Streaming model output: **open**. |
| 11 | Prompt template files: **open**. |
| 12 | Request-scoped values via `with`: **open**. |
| 13 | `eye fmt` and the two `if` forms: **open**. |
| 14 | `if let`: **open**. |
| 15 | Typed choices: **decided and built**. Labels (`##` or `Show`) still **open**; keep only that part. |
| 16 | What a fake received: **decided and built**. "Unused exchanges fail a test?" still **open**. |
| 17 | Approvals: **decided, not built** (#71/C.8). Say so. |
| 18 | Contracts shared between programs: **open**, folded into Q5. |
| 19 | Which actions are effects: **decided and built** (row 129). Drop it, but see top 4: starting a job is still not an effect. |
| 20 | Collection function types: **decided**. The code additionally lets them be called as methods (top 10). |
| 21 | `extern type` replaces `Handle`: **decided** (the checker is built; nothing runs foreign code yet). Drop it. |
| 22 | Printing in jobs: **decided and built**. Drop it. |
| 23 | Where `!` goes on a helper: **open**, and it depends on defining waiting functions (top 5). |
| 24 | `Self` and `dyn Store`: **open**, but its premise is **stale** (row 110 has no store example). |
| 25 | Naming a record's type at its literal: **open**, and the spec's own §11 example now argues for it. |
| 26 | What `@memory` measures: **open**. The attribute table contradicts it. |
| 27 | One flag for the store: **open**. |
| 28 | Several runners on one database: **open**. |
| 29 | Shape of task groups: **open**. |
| 30 | Model prices and routing: **open**. |
| 31 | `Option` parameters with defaults: **open**. |
| 32 | Time-travel debugging: **open** (wanted, no task). It is also wrongly cited by row 117. |
| 33 | Effects of a function that is only stored: **open**. |
| 34 | Online playground: **open** (wanted, no task). |

**Missing questions to add:** ending entities; re-entrant calls; must-use results; reading
entities and views in jobs; the effect of starting a job; whether `uses` is required on entity
methods and jobs.

### 7. Structure and writing

- **The decision log has 60 rows of very mixed size.** CLI flags, environment-variable names and
  lab history sit in the "Why" column, and the longest rows run about 600 characters. Split it
  into core language decisions and runtime/tooling decisions, and move the "From the lab"
  histories to the design pages.
- **The open questions are mostly closed.** About 14 of the 34 are decided. Move decided ones
  into the log, keep their residual parts as new numbered questions, and stop renumbering (row
  117's stale reference shows the cost).
- **Implementation detail sits in normative text:** §15 "Where runs live" (buckets of 64, advisory
  locks, schemas), §19 provider environment variables, the dates in row 131 and §15. Move these
  to `docs/internals` and leave rules in SPECS.
- **§21 is obsolete** and **§22 duplicates ROADMAP** and is partly stale. Keep §22's parsing
  decision and point to ROADMAP and compilation.md for the rest.
- **Version tags:** "From v0.2" (rows 109–110, L546, L948, L1842) tags things already built before
  v0.2 ships, while row 96's v0.2 feature (SQL) isn't built. Use status.md for "what's built" and
  keep SPECS free of version tags, or keep them accurate.
- **There is no glossary.** "Checkpoint" means a job save point and also zeropod's operation.
  "Run", "step", "key", "version" and "Handler" each need one definition.
- **The tour** should say what it takes from `worlds.eye`, and the examples should use one Notify
  contract consistently.
- **Outside SPECS but found on the way:**
  - status.md L225 has two table rows joined on one line (`…durable_test.go) || \`sleep!\`…`), so
    the `sleep!` row doesn't render.
  - `check/prelude.eye` L98 mentions `Handle`.
  - `grammar.js` L823 says "a function type without `uses` is pure".

---

**Files referred to:** `SPECS.md`, `docs/reference/status.md`, `ROADMAP.md`, `ir/builtin.eye`,
`check/prelude.eye`, `check/std/{fs,approvals}.eye`, `check/runtime/store.eye`, `interp/jobs.go`,
`testdata/programs/tour/worlds.eye`, `tree-sitter-eyelang/grammar.js`.
