# Performance, the idea, and a package manager, 2026-10-12

An analysis by a reviewing agent of `dev` at `3b91ced`, 2026-10-12. Nothing in the repository was
changed. Benchmarks, profiles and three small prototype patches were made in a scratch copy
(`/tmp/eyebench`, not committed). The numbers come from a shared 4-vCPU Linux VM with 15 GB of RAM,
using Go 1.27.2, CPython 3.13.16 and Node 22.22. Other sessions were running on the same machine,
so repeated runs of one benchmark differed by up to about 30%. Guesses are marked as guesses. What
the coordinating session concluded from it is in
[conclusions-2026-10-12.md](./conclusions-2026-10-12.md), section 6.

## Executive summary

- **Plain computation is acceptable for now.** The interpreter is 2–9× slower than CPython on
  loops, calls, records and trees (fib(30): 750 ms against 86 ms). For programs that mostly wait,
  this does not matter yet.
- **The real problems are slowdowns that grow with the input, not the constant speed.** Six were
  found. Each turns an ordinary program from milliseconds into seconds or minutes:
  1. **Building a string with `+=` is quadratic.** 100k appends take 18 s; CPython takes 10 ms.
  2. **Changing the value of an existing map key copies the whole map.** 20k updates on 10k keys
     take 6–11 s; CPython takes 1.4 ms.
  3. **Waking waiting runs is quadratic.** 1k runs take 0.33 s and 10k take 25.8 s. This is the
     scale the "many entities, jobs that mostly wait" pitch promises.
  4. **One entity write costs time proportional to the number of entities of that type.** It is
     0.75 ms with 1k entities and 5.8 ms with 40k, because the store copies the per-type row map
     on every commit.
  5. **`eye check` takes 9.4 s and allocates 7 GB on a single `match` over a binary tree.** This is
     in the exhaustiveness checker.
  6. **Every store commit copies the event list.**

  Fixes for 3, 5 and 6 were prototyped in about 30 lines. The tree check went from 9.4 s to 13 ms.
  10k waiting runs went from 25.8 s to 4.2 s. `go test ./check ./interp` still passed.
- **The memory claims need a qualifier.**
  - **True:** a waiting run holds no goroutine. Resuming takes well under a millisecond, plus one
    fsync. A looping job's record stays the same size (2.1 KiB) for its whole life.
  - **False today:** "A waiting job holds no memory at all" (SPECS §1). In `eye run`, each waiting
    run costs about 1.5–1.75 KB of RSS plus about 0.6–0.85 KB of journal. The journal and Postgres
    stores both hold the whole state in RAM. Nothing in the v0.4 plan changes that.
- **The C + Perceus + specialisation plan is sound for computation.** Expect roughly 50–200× over
  the interpreter on CPU-bound code (a guess). It does not fix the slowdowns above, because they
  live in the Go runner and the store, and v0.4 keeps both. Fix the algorithms in the runner first.
  Then decide how the runner hosts C code (the analysis leans towards linking the runner into each
  program as a static library). And let waiting runs leave RAM.
- **The idea holds up.** Effects and `!` checked by the compiler for durable code, single-writer
  entities, typed reconciliation, and deterministic tests are a real step past Temporal, Restate
  and Durable Objects. What users will hit first:
  - the slowdowns above;
  - deploys refused because a job changed under paused runs, with no migration path yet;
  - one runner per store, holding everything in RAM;
  - small papercuts: `NonZero` division, `??` that works only on `Option`, record literals that
    need a named type;
  - no packages, so programs can't share a contract (open question 18).
- **A package manager is feasible and cheap if it reuses existing infrastructure.** Recommended
  order:
  - **P0, now:** `path` and `git` dependencies, an `eye.lock` with content hashes, and vendoring
    through the existing `modules` key. This answers open question 18 at once.
  - **P1:** npm's registry as transport, used only through `eye`'s own client. It never runs
    install scripts. Resolution uses minimal version selection (Go's way of choosing versions),
    not npm's semver ranges, and the publish step diffs the API to enforce semver, as Elm does.
    Contract packages ship the generated TypeScript in the same tarball.
  - **P2:** Nix as an optional output (`eye gen nix`), never a requirement.
  - **Later, only if adoption warrants it:** a registry of our own.

  The lockfile records the **effects** each dependency's public API uses, so an update that adds
  `net`, `ffi` or `process` shows in the diff. No mainstream package manager can do that.

---

# Part 1: The language today

## 1.1 Interpreter speed: core language

Each benchmark was a test timed by `eye test -v`, with Python, Node and Go equivalents. Times are
in milliseconds.

| Benchmark | Eyelang interp | CPython 3.13 | Node 22 | Go | Interp ÷ CPython |
|---|---:|---:|---:|---:|---:|
| `while` loop, 1M iterations | 87 | 47 | 3.6 | 0.24 | 1.9× |
| `for i in 0..n`, `% 7`, 1M | 66 | 37 | 1.9 | – | 1.8× |
| fib(25) | 59 | 8.0 | 1.2 | 0.28 | 7× |
| fib(30) | 750 | 86 | 5.9 | 3.0 | 8.7× |
| `out += "x{i},"`, 100k | **18,083** | 9.6 | 16.4 | 13,855 (`+=`) / 6.9 (Builder) | **1,900×** |
| … at 10k / 20k / 40k | 154 / 595 / 2,727 | | | | quadratic |
| Same text built as `List` + `join`, 100k | 155 | | | | the workaround |
| `xs += [i]`, 100k | 14 | 4.0 | 2.2 | – | 3.5× |
| Map counter: 100k updates over 100 keys | **836** | 6.0 | 2.5 | – | 140× |
| Map counter: 20k updates over 10k keys | **6,271–10,840** | 1.4 | 1.0 | 2.2 | ~5,000× |
| … 10k / 20k / 40k updates (5k / 10k / 20k keys) | 2,924 / 10,840 / 30,715 | | | | quadratic |
| Map: 100k inserts, all new keys | 169 | | | | linear |
| `{ p with x: …, y: … }`, 1M | 455 | 173 | 14.8 | 0.35 | 2.6× |
| `p.x += 1; p.y += i`, 1M | 449–562 | 88 | 3.0 | – | 5× |
| Build and sum a binary tree, depth 18 (262k nodes) | 437 | 125 | 22 | 9.5 | 3.5× |
| `toList \|> map \|> filter \|> sum`, 1M | 648–735 | 54 | 62 | – | 12× |
| `p == Asleep` on a variant, 1M | 486 | – | – | – | – |

**How to read this.** Constant-factor speed is between a slow CPython and three times slower,
which is normal for a tree-walking interpreter over Go `any` values.

The two quadratic rows are different:

- **Strings:** `+=` on a Go string copies the whole string. Go has the same problem, which is why
  `strings.Builder` exists. CPython avoids it by appending in place when the string's reference
  count is one, and Node uses ropes.
- **Maps:** `insert` on an existing key copies the map's whole index. SPECS §5 and §7 promise
  in-place updates for uniquely held values. Today that holds for list appends and for set or map
  inserts of new keys (amortised O(1) through a shared backing array). It does not hold for string
  appends, map replacements or record field assignment.

## 1.2 Interpreter speed: entities, jobs and the runtime

Each row was a test on the test clock.

| Workload | Time | Per item |
|---|---:|---:|
| 10k distinct entities, one method call each | 243 ms | 24 µs |
| One entity, 10k calls | 168 ms | 17 µs |
| Update one entity 2,000 times when its type has 1k / 10k / 40k entities | 1.58 / 4.92 / 12.7 s | **0.75 / 2.3 / 5.8 ms** |
| One run wakes 1,000 / 5,000 times (`while` + `sleep!`) | 305 ms / 1.0 s | 0.2–0.3 ms per wake, linear |
| Start N runs that each sleep 1 h (start only), N = 2k / 4k | 124 / 219 ms | 55 µs per start, linear |
| Start N runs, then wake them all, N = 1k / 2k / 4k / 10k | 0.33 / 1.16 / 4.27 / **25.8 s** | **quadratic** |
| Same, runs that touch no entity, 10k | 19.5 s | |
| `parallelMap` over 1k / 10k tasks that each `sleep(1 min)` | 11 ms / 751 ms | superlinear |
| `eye check testdata/lab` (2,157 lines, plus 1,226 lines of prelude, std and store) | 24 ms per check, 13 MB allocated | about 140k lines/s |
| `eye check` on one `match` over `Leaf \| Node(left: Tree, value: Int, right: Tree)` | **9.4 s, 7 GB allocated** | |
| `eye test testdata/lab` (30 tests) | 277 ms | |

C.6's exit test ("10,000 waiting tasks within a memory budget") is about tasks. The run-level
numbers above say the same at a larger scale: the runtime does not handle 10k waiting things today.

## 1.3 Hot spots, with file:line (dev 3b91ced)

All of these were found with `pprof` on a small harness that calls `check.Program` and
`interp.RunTests`.

**Slowdowns that grow with the input (fix first):**

1. **Exhaustiveness: `check/exhaust.go:68`** (in `missing`, from `:48`). The checker splits a column
   into constructors even when no row tests that column, which is the case for `left` and `right`
   bound by name. With two recursive fields, the search is exponential up to the `depth > 32` cap.
   - Maranget's algorithm uses the default matrix here.
   - **Prototype:** if every row's head is a wildcard or a binding, recurse on the default matrix.
     9.4 s became 13 ms, and `go test ./check` passed.
2. **The waiting index: `interp/store.go:181`** (`indexed.sleeping`). It copies and sorts every
   waiting run's id, and two call sites run it on every wake:
   - `interp/runtime.go:1023` (`nextSleeper`);
   - `interp/runtime.go:410` (`wake`, which also scans every waiting run to find the few waiting
     for one event).
   - That makes waking N runs cost O(N² log N).
3. **The store's event list: `check/runtime/store.eye:159`**
   (`s.events = (s.events |> drop(c.dropEvents)) + c.events`). `drop` makes a list with no spare
   room, so `+` copies every retained event on every commit, even an empty one.
   - **Prototype** of 2 and 3: scan the map without sorting, sort only the runs waiting for that
     event, and touch `events` only when the commit changes them. 10k waiting runs went from 25.8 s
     to 7.0 s to 4.2 s. Runs that touch no entity went from 19.5 s to 3.4 s. `go test ./interp`
     passed.
   - A min-heap on `wakeAt` and an index by event would make each wake O(log N); a guess is under
     1 s for 10k.
4. **Entity rows: `check/runtime/store.eye:152` plus `interp/value.go:261`.**
   `s.rows |> insert(w.entity, rows |> insert(w.key, row))` replaces a key in the per-type map.
   `Map.with` handles a replacement by building a new shelf and running `maps.Clone` on the index
   (`value.go:187`), so every entity write is O(entities of that type).
   - Runs avoid this with buckets of 64 (`store.eye:157`); rows have no such fix.
5. **Map replace in user code: `interp/value.go:261–268`.** Same cause, so the map-counter idiom is
   quadratic.
6. **String append: `interp/eval.go:995`.** `x + y` on Go strings.

**Constant factors (the call and loop path):**

- `interp/eval.go:108–205`: `evalArgs`, `callFunc` and `bind` allocate five slices per call (args,
  frame slots, `bound`, `paramOf`, `final`), plus the frame. In fib, `makeslice` and `mallocgc` are
  about 30% of CPU.
- `interp/eval.go:207`: `runBody` runs a `defer`/`recover` per call, because `return` is a panic.
  `interp/eval.go:1242`: `loopTurn` runs a `defer` plus a closure per loop iteration, for `break`
  and `continue`.
- `interp/eval.go:554–563`: `ref` allocates a new `&Closure` for every reference to a named
  function, and a new `&Variant` for every use of a fieldless variant such as `Asleep`.
- `interp/value.go:564–580`: `equal` falls back to `keyOf` (`value.go:437`), which builds a string,
  for anything but `Int`, `String` and `Bool`. Comparing `p == Asleep` builds two strings containing
  the module path; that is 40% of the variant benchmark.
- `interp/value.go:296`: `Record.with` copies both `Names` and `Vals` on every field assignment.
  `Names` never changes and could be shared.
- `interp/eval.go:954`: `binary` dispatches on operator strings.
- `Int` values are `int64` in an `any`. Go boxes values of 256 and above, so arithmetic allocates.

**The journal:**

- Each commit is JSON that holds each run record as base64 of JSON, which adds about 33%.
- The run record includes the 20-step `Trail` for `eye runs` (`interp/checkpoint.go:492`),
  rewritten in full on every commit. In the looping job measured, each commit was about 2.05 KB;
  from the decoded record, an estimated four fifths of it is the trail.
- One run takes three fsync'd commits to reach its first sleep: started as `ready`, step recorded,
  then `sleeping`.

## 1.4 Memory: the claims against measurements

`eye run` on a small program. RSS was read from `/proc` once the program was idle.

| Measurement | Value |
|---|---|
| Idle runner, one waiting run | 18.1 MB RSS (`--memory`: 17.7 MB); the `eye` binary is 22 MB |
| 5,000 waiting runs (journal) | 25.7 MB, so **about 1.5 KB per waiting run** |
| 20,000 waiting runs (`--memory`) | 52.7 MB, so **about 1.75 KB per waiting run** |
| 10,000 entities with 2 fields | +7 MB, so **about 0.7 KB per entity** |
| Journal for one run, up to its first sleep | 845 B over 3 commits; about 590 B per run after compaction at 5k runs |
| Journal per entity | about 108 B |
| Restart with 5k runs and 10k entities (3.8 MB journal) | under 1 s of CPU, 38.5 MB RSS |
| Starting 5k keyed runs from `main` | 4.5 s on the journal, 1.0 s with `--memory`: about 0.7 ms of fsync per commit, with no group commit |
| A job looping every 50 ms for 10 s | 385 steps; the live record stays at 2.1 KiB; the journal grows by about 2 KB per commit and compacts at 4× live past 1 MiB |

| Claim | Where | Verdict |
|---|---|---|
| "A waiting job holds no memory at all" | SPECS §1 | **No.** It holds no goroutine or stack (true, and status.md says exactly that). But each waiting run costs about 1.5–1.75 KB of RSS, because every store keeps all state in RAM; the Postgres store reads everything when it opens. Suggested wording: "costs its saved record, about 1 KB, and no thread". |
| "Resuming one takes milliseconds" | SPECS §1 | **Yes**, and better: 0.2–0.3 ms of CPU per wake, plus about 1 ms of fsync. |
| A never-ending run "costs the same on its tenth day" | SPECS §15 | **Yes.** The record is flat at 2.1 KiB and the journal compacts. |
| "An update … that nothing else refers to happens in place" | SPECS §5, §7 | **Partly.** True for list appends and new map or set keys. Not true for map replacements, string `+=` or record field assignment. |
| "Thousands of waiting tasks cost little memory" | SPECS §13 | Not measured for memory. Time is superlinear: 10k tasks take 751 ms, 1k take 11 ms. |
| compilation.md "What exists today": "`xs += [x]` in a loop is quadratic"; `typeOf` | compilation.md | **Stale.** Appends are linear now (100k in 14 ms) and records know their type. |

## 1.5 Does the compilation plan deliver the performance pillars?

**What it gets right:**

- C through `zig cc` keeps the toolchain uniform and the bootstrap seed natural.
- Perceus fits a language with no aliasing, no references and no cycles. Eyelang is closer to the
  Perceus ideal than Koka or Lean, because `mut` arguments are checked as unique at the call site.
- Whole-program specialisation is simple while there are no packages.
- Saving frames at each `!` is the right model for jobs; the interpreter's statement-level
  checkpoints already proved the idea.

**Compiled computation will be fast.** Guess: within 2–3× of Go, so 50–200× over the interpreter on
CPU-bound code. It also fixes the quadratic string and map cases, provided strings and maps get
capacity growth and a unique-owner fast path.

**The biggest risks, in order:**

1. **The bottleneck that matters is not in the C.** Every slowdown that grows with scale in section
   1.3 is in the Go runner or in `MemoryStore`. The pillar "waiting costs no memory" also needs a
   waiting run's record to leave RAM, and nothing in the plan does that. v0.4 keeps the Go runner
   and the all-in-RAM store, so the digest would run natively and still cost about 1.5 KB of RAM
   per waiting run, with quadratic waking.
   - **Recommendation:** before 7.4, give the runner a timer heap, an index by event, O(1) commits
     and run records paged out of RAM. In RAM, keep only `(id, wakeAt, event)`, about 40 B (guess),
     and load the record on wake.
2. **Hosting C from a cgo-free Go runner.** Loading the program with dlopen in pure Go (purego
   style) means every entity call, `!` step and store access is a C-to-Go callback through assembly
   trampolines, with limits on how many callbacks can exist and real cost per call.
   - The alternative is to link the runner into the program as a static library
     (`-buildmode=c-archive`, cgo only at release time). That gives a single binary and ordinary
     cgo calls (tens of ns each). `eye` itself stays cgo-free.
   - The analysis leans towards the static library, and the decision is blocking 7.6.
3. **Scope.** A core runtime in C (big integers, insertion-ordered maps and sets, strings with
   `Offset`, formatting, time and zones, `fs`, `process`), a Perceus pass and a state-machine split
   is several thousand lines of subtle code. Guess: 8–15k lines.
   - Prior art moved slowly: Koka and Lean took years, and Roc rewrote its compiler (in Zig) in
     2025.
   - Mitigation: run every program's tests on both the interpreter and the compiled build, the way
     the parser conformance suite does.
4. **Whole-program specialisation against per-module C files and caching.** Instances depend on
   every use, so per-module object caching needs instances emitted by content hash. Once packages
   exist, specialisation crosses package boundaries, which is fine because packages ship source.
5. **Toolchain weight.** The pinned zig 0.17 is 411 MB unpacked, beside a 22 MB `eye`. Fine for
   `eye build`, heavy for someone who only wants `eye test`; keep it downloaded on demand, as
   `tools/zig` does. Zig is pre-1.0, but `zig cc` is clang underneath and has been stable in
   practice.
6. **Two representations of a paused run.** Interpreter checkpoints are per statement, compiled
   frames are per `!`, and both are keyed by code hash. The plan to replay old logs once is right;
   it needs a test that moves a paused run from the interpreter to compiled code.

## 1.6 Large wins available in the interpreter now

Ordered by payoff divided by effort. The estimates are guesses unless marked as measured.

| Change | Payoff | Effort |
|---|---|---|
| Default matrix in exhaustiveness (`exhaust.go`) | 9.4 s to 13 ms on tree matches (measured) | 15 lines |
| Timer min-heap and per-event index in `indexed`; skip `events` on commits that don't change them | 10k waiting runs: 25.8 s to 4.2 s (measured, partial) to under 1 s with the heap | about 80 lines |
| Persistent hash map (HAMT) for `Map` and `Set`, or a "newest version may overwrite" stamp extended to replacements | Map counters and entity writes become O(log n). Fixes both the user idiom and `store.eye`'s row map | 300–500 lines |
| String append that shares spare room, like `List.appended` (a byte slice plus an atomic `used`), or a rope | `+=` on strings becomes amortised O(1) | about 100 lines |
| Call path: precompute the argument-to-parameter binding (lir already resolves it), cache `Closure` and fieldless `Variant` values per declaration, return and break by flag instead of panic, fast paths in `equal`, share `Record.Names` | 2–3× on call- and record-heavy code | 1–2 days |
| Journal: store run records as `json.RawMessage` instead of base64, keep the trail out of every commit, group-commit bursts such as the starts from `main` | Commits about 3–5× smaller; `main` starting 5k runs goes from 4.5 s to about 1 s | about 150 lines |
| Later, if v0.4 slips: interpret lir (register slots, typed operations) | 3–10× on computation | weeks |

---

## 1.7 The idea, compared with existing systems

| System | What it shares with Eyelang | Where Eyelang is better | Where it is reinventing or behind |
|---|---|---|---|
| **Temporal** | Durable functions, timers, signals, child workflows | Determinism is checked by the compiler (effects plus `!`) instead of by SDK convention or a sandbox. Saving live variables avoids history limits and continue-as-new. A code change under paused runs is detected, not discovered later. | Temporal's versioning, scale-out, visibility UI and ecosystem. Eyelang refuses to start on a changed job but offers no migration yet. |
| **Restate** | Journaled handlers plus keyed single-writer "virtual objects": almost exactly jobs plus entities | Types, effects and reconciliation in the language; no separate server; deterministic tests with recorded fakes | Restate scales out, keeps state outside RAM, and works from five languages. Eyelang is close to "Restate as a language" with one runner. |
| **Durable Objects** | Single-threaded objects with transactional storage, alarms | Methods that can't wait remove input/output-gate subtleties; views and triggers across objects | DOs load on demand across a global network. Eyelang loads every entity into one process's RAM. |
| **Orleans** | Virtual actors that exist on first use | Methods can't wait, so no re-entrancy deadlocks (since G.9 fixed the lost write) | Placement, activation and deactivation, clustering |
| **Unison** | Code identity by hash (Eyelang's resume-point hash is a small version of this) | Ordinary text files and git, which agents and reviewers handle well | Unison's content addressing would solve package versions and job code upgrades more completely |
| **Gleam** | Friendly typed functional core, lower-case type variables | Effects, durability, contracts | Gleam runs on BEAM's runtime and ecosystem, and **publishes to Hex**: a precedent for reusing another ecosystem's registry |
| **Koka** | Perceus, effect inference | Simpler effects (capabilities, not handlers), easier to read | Koka's effect handlers are more general; `with fakes` is a handler in disguise |
| **Roc** | Platforms provide effects, reference counting with in-place updates, full specialisation | Built-in durable domain | Roc's multi-year compiler effort and rewrite is a warning about scope |
| **Elixir/OTP** | Processes for long-lived state, supervision | Durable by default; a crash loses nothing | Distribution, hot upgrades, decades of runtime maturity |
| **Ballerina** | Network-aware types, services and clients as language constructs | Durability, effects, a smaller language | Ballerina's limited adoption is the cautionary tale for integration-domain languages |
| **Dark** | Language plus runtime plus infrastructure in one | Plain text, git, LSP: no editor lock-in, which hurt Dark | Same risk: a whole platform maintained by a very small team |

**Where Eyelang is genuinely new:**

1. Compile-time-checked determinism for durable code, through effect inference and the visible `!`.
2. Single-writer entities with value semantics, so nothing is shared mutably.
3. Exactly-once at the boundary made explicit in types: `Uncertain`, `@reconcile`, keys, `@fresh`.
4. Contracts that also derive network policy and allowed callers.
5. Deterministic tests with a test clock and recordings.
6. A language deliberately shaped for coding agents: canonical formatting, `eye fmt` writing
   `uses` clauses, one way to do each thing.

**Where it reinvents:**

- the durable execution engine;
- the actor runtime;
- the schema-migration system;
- cron.

Each is done well, but it is one team carrying what Temporal, Restate and Orleans each spend whole
companies on.

**What users will hit first, in likely order:**

1. **The slowdowns in 1.3.** A string built with `+=`, a word count in a map, a binary tree in a
   lexer or checker. All three are what an agent writes first.
2. **Papercuts in the first ten minutes,** hit while writing the benchmarks: `%` needs
   `NonZero(Int)`; `??` works on `Option` but not `Result`; `0..n |> toList` parses as
   `0..(n |> toList)`; `let x = { … }` needs a named type.
3. **Deploying a changed job with paused runs is refused,** with no `migrate` for resume points and
   no drain tooling.
4. **One runner per store, all in RAM.** Fine for a homelab; the ceiling arrives at about
   10^5–10^6 entities or runs (guess, from 0.7–1.75 KB each and the per-type write cost).
5. **No packages.** The lab is one program so its parts can share Notify (open question 18).

---

# Part 2: Could Eyelang have its own package manager?

## 2.1 Starting point and constraints

- **Modules are directories.** `import lab.notify` names `lab/notify` under the program root or
  under a directory listed in `eye.toml`'s `modules`. So the program root is the module namespace.
- **Open question 5** asks for an `eye.lock` with hashes "so Nix needs no placeholder hash", and
  asks whether there should be a registry or git only. Open question 18 asks how separate programs
  share a contract.
- **Things that already help:**
  - `modules` is a ready-made vendoring hook.
  - `tools/zig` already pins a binary by version and SHA-256 per platform, which is a package
    manager in miniature.
  - Public signatures already state their effects; `eye fmt` writes them.
  - Task 6.4 will generate TypeScript and Go from contracts.
- **Pillars that constrain the design:**
  - deterministic tests, so fetching must be hermetic and work offline from a cache;
  - visible costs and effects;
  - agents as users, so commands must be non-interactive with diffable output;
  - a cgo-free single `eye` binary;
  - "strict but never clumsy".

## 2.2 The options

### A. npm's registry as transport (the owner's idea)

`eye` acts as its own small npm client:

1. Fetch the abbreviated packument (`GET https://registry.npmjs.org/@scope/name` with
   `Accept: application/vnd.npm.install-v1+json`).
2. Choose a version.
3. Download the `.tgz`.
4. Check `dist.integrity` (SHA-512).
5. Extract only `.eye`, `eye.toml`, recordings and docs.

It never runs `npm` and never runs lifecycle scripts.

How each concern works:

- **Publishing:** `eye pack` writes a tarball with a generated `package.json` (name, version, an
  `"eyelang"` field, `files`). `npm publish --provenance` from CI with trusted publishing (OIDC)
  uploads it.
  - npm revoked all classic tokens on 2025-12-09. Write-capable granular tokens are capped at 90
    days, and `npm login` gives 2-hour sessions, so CI with OIDC is the sane path. Leaving
    publishing to the npm CLI gets provenance (Sigstore) for free.
- **Resolution:** npm semver ranges plus a lockfile, or minimal version selection over npm
  versions. The second treats each `^1.2` as "at least 1.2.0, major 1"; packuments carry each
  version's dependencies, so this works.
- **Integrity:** SHA-512 SRI comes from the registry and is copied into `eye.lock`. Nix's `fetchurl`
  accepts it directly, which removes the placeholder-hash dance that open question 5 worries about.
- **Caching and offline:** a content-addressed cache (`~/.cache/eyelang/pkg/sha512-…`);
  `eye vendor` copies dependencies into a directory listed in `modules`.
- **Private packages:**
  - npm private scopes need a paid organisation;
  - GitHub Packages (`npm.pkg.github.com`) needs authentication even to read;
  - Verdaccio, Artifactory or Nexus self-hosted, all of which speak the same protocol;
  - workable, but more friction than git for a homelab.
- **Security:**
  - The worst npm attacks (Shai-Hulud in September 2025, the chalk/debug compromise) spread through
    **install scripts** and stolen tokens. An `eye` that never runs scripts removes the first vector
    entirely.
  - Typosquatting and account takeover remain. Mitigations: require scoped names (the scope is the
    publisher's identity, as a domain is for Go), a minimum release age before `eye add` accepts a
    new version, and showing the publisher and provenance.
  - Agents add "slopsquatting": a hallucinated package name that an attacker has registered.
    `eye add` should refuse unscoped, very new or never-downloaded names without `--yes`.
- **Policy:** npm's Open Source Terms require packages to be "functionally compatible with the npm
  command-line client" and forbid using the registry "as a general purpose database". An Eyelang
  tarball with a valid `package.json` is installable by npm, and a contract package with generated
  TypeScript is genuinely useful to JavaScript users. The reading here is that this is allowed but
  at npm's discretion; it is not settled. Keep the source pluggable: `eye.lock` records URL plus
  hash, so a mirror can serve the same tarballs.
- **Cost:** no hosting. An npm read client is about 1.5k lines of Go (guess), with no node needed to
  consume packages. Publishing uses the npm CLI in CI.
- **Shared namespace with JavaScript packages:** acceptable when names are scoped (`@lab/notify`).
  Discovery needs a convention: the `"eyelang"` field plus a keyword.

**The natural fit: contract packages.** One npm package `@lab/notify` holds `notify.eye` and the
TypeScript types and client generated by task 6.4. An Eyelang program depends on it through `eye`;
a TypeScript service depends on it through `npm i`. Both read the same versioned contract, which
is what SPECS §17 promises. Go callers would still want a Go module, which would come from the same
build, published to a git tag.

### B. Go-modules style: VCS plus checksums plus MVS

- **Publishing:** push a git tag; there is no registry.
- **Identity:** a URL in `eye.toml` with a local alias, so imports stay short.
- **Resolution:** MVS (Russ Cox, 2018). Builds are reproducible without a lockfile, and an upgrade
  never happens by accident.
- **Integrity:** a lockfile of hashes. Go adds `sum.golang.org`, a transparency log that makes a tag
  rewrite detectable globally. Running such a log, and a proxy for availability and immutable tags,
  is real operations work. Without them you get trust on first use per lockfile, which is what most
  git-based package managers have.
- **Private packages:** trivial, through git credentials.
- **Offline:** cache or vendor.
- **Security:** no scripts. Typosquatting is mostly replaced by trust in URLs.
- **Cost:** a git fetch implementation, or shelling out to `git`. The repository's tarball and
  `codeload` URLs also work without git.
- Since Eyelang packages are source (`.eye` files), this needs no build infrastructure at all.

### C. A registry of our own

- **Examples:** crates.io, Hex, JSR. JSR also serves its packages through an npm-compatible
  endpoint, which is a neat hybrid.
- **What it gives:** a clean namespace, search, Eyelang-aware documents (`eye doc` per version),
  registry-enforced semver, effect manifests and yanking.
- **What it costs:** an account system, moderation, abuse handling, malware response and storage,
  plus an on-call team forever. crates.io and npm both have full-time staff for this.
- **Verdict:** not before v1.0 adoption justifies it, and design A and B so that a later registry
  can mirror them.

### D. Nix flakes as the lockfile and build layer

- `flake.lock` pins inputs.
- Derivations build hermetically.
- `cache.nixos.org` provides zig, Postgres and C libraries for every nixpkgs platform.

What it would buy:

- **Reproducible native dependencies.** The C libraries behind `@library` (zlib, sqlite, libpq) and
  Postgres for tests, all hermetic and pinned.
- **Dev shells.** `nix develop` gives zig, Postgres and node for a contributor in one step.
- **Hermetic `eye build`.**
- **Binary caches** for the native parts.

What it wouldn't buy, and what it costs:

- Eyelang packages are tiny source trees, so Nix adds nothing for them beyond what a lockfile with
  hashes gives.
- **Adoption cost:** a multi-GB store, a steep learning curve, and slow evaluation.
- **Platforms:** macOS works (the Determinate installer). Windows works only through WSL2, and
  Windows is one of Eyelang's five release targets.
- **Status:** upstream Nix still labels flakes experimental, while Determinate Nix markets them as
  stable. No upstream change by 2026 could be confirmed.
- **The pillars:** making Nix a requirement would break "a single binary, developer experience
  ships with the language".

**Verdict:** Nix should be optional and come second. `eye gen nix` turns `eye.toml` plus `eye.lock`
into a flake:

- one `fetchurl` per dependency, using the SRI hash from the lock, so there is no placeholder;
- zig from the hashes in `tools/zig` or from nixpkgs;
- `[native]` libraries mapped to nixpkgs attributes;
- a dev shell with Postgres for `--store` tests.

This is the gomod2nix or `importNpmLock` pattern. It works only if the lock stores a hash Nix
understands:

- SHA-512 or SHA-256 of the tarball, for registry and URL sources;
- the NAR SHA-256, for git and directory sources; computing it in Go takes about 200 lines.

**Native dependencies without Nix.** Since `eye` ships `zig cc`, a package can ship the C sources of
its `@library` and a short recipe. `eye` compiles them with zig for the host, as a shared library
the interpreter loads, or for the target, linked into `eye build`. The result is cached by hash.
This is Zig's own package model, and it covers Windows. Nix stays the route for large system
libraries (OpenSSL, libpq) and for teams that already use it.

### E. Hybrids

- **Recommended:** sources are pluggable (`path`, `git`, `npm`, later `registry`); resolution is
  always MVS; every source is pinned by content hash in `eye.lock`; Nix is an optional output.
- **The Roc variant:** a URL with the content hash inside it, with no lockfile at all. Elegant, but
  it loses version ranges and upgrades.

## 2.3 Fit with Eyelang's pillars

- **Effects across package boundaries (the distinctive feature).** A package's public functions
  state `uses`, so `eye` can compute each dependency's effect footprint and record it in
  `eye.lock`:

  ```toml
  [[package]]
  name = "@lab/notify"
  version = "1.4.2"
  source = "npm"
  integrity = "sha512-…"
  effects = ["net Notify.send", "net Notify.receipt"]
  stateful = false
  ```

  - An update that adds `ffi`, `process`, `fs` or a new `net` operation shows in the lockfile diff
    that a reviewer or an agent reads.
  - `eye.toml` can deny effects per dependency (`deny = ["ffi", "process"]`) and make violations
    compile errors. This is what LavaMoat or Deno's permissions do for npm, but checked statically
    and for free.
- **Semver enforced, as Elm does.** `eye publish` diffs the public API against the previous version
  and refuses a wrong bump. Breaking changes include:
  - a removed or changed signature;
  - an added effect on a `pub fn`;
  - **a new variant on a public type**, since every exhaustive `match` downstream breaks; this is
    how SPECS §9 wants a new outcome to reach every caller;
  - a contract change that breaks the wire format.

  Elm does this for types; Eyelang can do it for types, effects and contracts.
- **Deterministic tests.** `eye test` never touches the network: the lock plus the cache or
  `vendor/`. Recordings shipped in a contract package make its fakes reusable by every consumer.
- **Durable state and upgrades (a hazard unique to Eyelang).** A dependency that defines entities
  or waiting functions puts its code into the program's resume points and stored rows.
  - An `eye update` that changes such code can make `eye run` refuse to start (open questions 4 and
    23) or need migrations.
  - The lock should mark such packages `stateful`.
  - `eye update` should check the new lock against the journal or store and say which paused runs
    or rows it would strand, before the deploy rather than at start-up.
- **Contracts shared between services in different languages.** npm is the natural home for the
  TypeScript side (6.4) and git tags for Go. One `eye publish` could produce both from one
  contract.
- **Agents.** Non-interactive `eye add`, `eye update` and `eye why PKG`, with machine-readable output
  (`--json`); a lockfile diff that explains itself (versions, hashes, effects); refusal of
  unverifiable names.
- **The single cgo-free binary.** HTTP, tar and gzip, SHA-512, MVS and NAR hashing are all pure Go.
  `tools/zig`'s download, check and cache code is already the pattern to reuse; zig itself could
  become `[toolchain] zig = "0.17.0"` in `eye.toml`, resolved the same way.

## 2.4 Recommendation and phases

| Phase | What | Answers | Effort (guess) |
|---|---|---|---|
| **P0 (now)** | `[dependencies]` in `eye.toml` with `path` and `git` sources (tag or rev, subdirectory) and a **local alias as the import root**: `notify = { git = "https://github.com/…/lab", rev = "v1.2.0", dir = "notify" }` makes `import notify` work. `eye.lock` holds the exact rev, a NAR SHA-256 and an effects summary. A content-addressed cache. `eye vendor`, which writes into a `modules` directory. MVS from day one. | Open question 18 (programs share Notify), most of 5 | 1.5–2k lines of Go, 1–2 weeks |
| **P1** | The `npm:` source: `eye add npm:@scope/name@^1`, MVS over npm versions, SHA-512 integrity checks, no scripts ever, scoped names only, a minimum release age. `eye pack` plus `npm publish --provenance` through trusted publishing. API-diff semver enforcement at pack time. Contract packages carry the generated TypeScript (after 6.4). | The rest of 5 ("registry or git?": both, through one lock) | 2–3k lines, plus the 6.4 output |
| **P2** | `eye gen nix` (a flake from the lock, no placeholder hashes). `[native]` libraries built from source by `zig cc`, or mapped to system or nixpkgs libraries. A Postgres dev shell. Effect `deny` lists. The `stateful` check against the journal in `eye update`. | 5's Nix clause; parts of 4, 23 and 35 | 1–1.5k lines |
| **P3 (after v1.0, if demand)** | Our own index or registry that mirrors npm and git sources, with `eye doc` per version and a checksum transparency log | | months, plus ongoing operations |

**New open questions this raises for SPECS:**

1. **Import names for dependencies.** Is the `eye.toml` alias the import root, or does it mirror the
   package name? Recommended: the alias, for short imports, with identity kept in `eye.toml`.
2. **May packages declare entities and jobs?** Whose store, which migrations, and what happens to
   resume points on update? Recommended: allow it, flagged `stateful`, with the update check above.
3. **Effect policy for dependencies.** Should there be `deny` lists, and should `ffi` in a
   dependency need explicit consent?
4. **Allow lists across packages.** Open question 35's event guard and `@allow` across packages: a
   package can't name the program's modules, so `eye.toml` needs a place to grant them.
5. **Other things for `eye.toml`.** The store (open question 27), model prices (open question 30)
   and the toolchain pin (zig).

**Bottom line:** yes. Start with P0 now, since it is small and unblocks open question 18 for the
lab. Treat npm strictly as a CDN with integrity hashes, read by `eye` and never by npm. Use the
lock's effect summaries and enforced semver as the feature that sets Eyelang's package manager
apart. Keep Nix optional and generated.

---

## Sources for Part 2

- [npm Open Source Terms](https://docs.npmjs.com/policies/open-source-terms)
- [npm classic tokens revoked, 2025-12-09](https://github.blog/changelog/2025-12-09-npm-classic-tokens-revoked-session-based-auth-and-cli-token-management-now-available/)
- [Granular token changes, 2025-11-05](https://github.blog/changelog/2025-11-05-npm-security-update-classic-token-creation-disabled-and-granular-token-changes/)
- [GitHub's plan for a more secure npm supply chain](https://github.blog/security/supply-chain-security/our-plan-for-a-more-secure-npm-supply-chain/)
- [GitHub mandates 2FA and short-lived tokens](https://thehackernews.com/2025/09/github-mandates-2fa-and-short-lived.html)
- [JSR usage policy](https://jsr.io/docs/usage-policy)
- [Fake font packages abusing npm as a CDN](https://www.endorlabs.com/learn/how-fake-font-packages-abused-npm-as-a-cdn)
- [Determinate Systems, "Experimental does not mean unstable"](https://determinate.systems/blog/experimental-does-not-mean-unstable)
- [Why are flakes still experimental?](https://discourse.nixos.org/t/why-are-flakes-still-experimental/29317)
- [Determinate Nix 3.0](https://discourse.nixos.org/t/determinate-nix-3-0/61202/8)

Facts about Gleam on Hex, Go's MVS and sumdb, Elm's enforced semver, Roc's hashed URLs and Nix's
`fetchurl` accepting SRI hashes are from the reviewer's own knowledge and were not re-checked
online.
