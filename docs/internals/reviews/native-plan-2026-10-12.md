# Native compilation: where it stands and a plan, 2026-10-12

The owner asked to stop after the runtime tasks (R.1 to R.7) and look again before starting 7.5
(reference counting and reuse) and 7.6 (the runner hosts compiled programs). This page is that
look. It says what `eye build` can do today and what it can't, what the R tasks changed, what
is left to decide, and a proposed order of work, each step with a done-when. Nothing here is
decided until the owner says so; section 7 lists the decisions.

The facts come from a survey run on `dev` at `ed7c6be` (after R.6) on a 4-core Xeon at 2.1 GHz.
Timings there vary by up to about 1.5× between runs, so ranges are given.

## The short version

- **Native code is fast where it works.** 10–80× the interpreter per benchmark after R.6, 8× on
  the whole benchmark file, 0.3 ms to start, 658 KB for core's stripped test binary. fib(30)
  takes 19 ms, against 220 ms interpreted and 5–12 ms in Go.
- **It doesn't beat Go yet, which is the owner's bar.** On 7.7's nine benchmarks, the lexer
  among them, native code takes 1.1–5.7× Go's time and uses less memory in six
  ([Against Go](#against-go)).
- **It works on very little.** Two of the fifteen programs in `testdata/programs` compile (core
  and traits). Everything with an entity, job, view or contract stops at one check, and that is
  all of the lab.
- **It is slower than the interpreter on some ordinary code.** An update whose last use is a call
  or a `let` copies the whole value each time, so a loop over it is quadratic: 40k appends
  through a helper take 3.75 s native and 19 ms interpreted. 7.5 fixes this, and until it does,
  native code breaks pillar 2's promise.
- **The hosting choice from compilation.md has changed shape.** The single-binary way
  (`c-archive`) crashes at start-up as a static musl binary, the release's Linux format. A third
  way works: the Go runner as `main`, linking the program's C through cgo. It gives one static
  binary and the fastest calls, but `eye build` then needs a Go toolchain.
- **The pillar v0.4 is about is mostly the runner's job, not the compiler's.** "Waiting costs no
  memory" is R.7, which lives in the Go runner and store and works whichever engine runs the
  code. The lab is bound by input and output: its 31 tests take 180 ms interpreted.

**Proposal:** do 7.5 now, in a narrowed form, because native code that is quadratic where the
interpreter is linear can't ship. Before 7.6, count how often a digest run crosses between
compiled code and the runner, and decide hosting on that number. Section 6 has the steps.

## 1. What `eye build` covers

### Programs

| What | Result |
|---|---|
| `testdata/programs/core` (60 tests), `traits` (8), `cgen/testdata/*` | Compile for all five targets; tests pass natively and match `eye test -v` line for line |
| The other 13 programs in `testdata/programs` | Stop at the declaration check (`cgen/cgen.go:154-194`): entities, jobs, views or contracts |
| The lab | Stops at the same check (`digest/digest.eye:17`) |

With the check turned off in a scratch copy and every function of the program emitted, the lab
has 201 function instances:

| | Functions |
|---|---:|
| Compile today | 110 |
| Blocked only by what 7.6 hosts (entities, contracts, `!` steps, events, jobs, test fakes and clock, schedules) | 85 |
| Also need a core gap: time methods or `toJson` | 6 |
| Also need host functions (`http.get`, `ai.generate`) | 2 |

Across the 13 failing programs, 74 of 307 functions compile today.

### Core gaps in the C back end

These are missing whatever 7.6 decides, because they are core, not durable:

- **Time:** `Instant.unix`, `Instant.date(in:)`, `Instant.time(in:)`, `Time.next`, `Date.at`,
  `Date.format`, `contains` on a `Range(Time)`, a range's `.start` and `.end`. `Date.at` is
  reported as "list patterns on Date", because `at` collides with list indexing in
  `cgen/builtins.go`.
- **Data:** `toJson`, and the `Schema`/`Save` derivations (only show, eq, cmp, hash, parse and
  variants are derived).
- **Host functions:** `http.get`, `ai.generate`, `random.*`.
- **Tasks:** `both`, `race`, `timeout`, `taskGroup`, `channel` and their operations.
  `parallelMap` compiles but runs in order on one thread. Only the `tasks` program uses them;
  the lab doesn't.

### How it is checked

`TestTestsPassAsInterpreted` compares a native test binary's `-v` output with the interpreter's,
for core, traits and two of cgen's programs, built at `-O0 -g`. Nothing runs the `-O2` build
against the interpreter, and no other program, saved encoding or JSON output is compared.

compilation.md says valgrind finds no leak in core's tests. valgrind can't see the heap of the
static musl binary that `eye build` writes ("0 allocs"). A glibc build of the same C does show
no leaks and no errors (1,821 allocations, 1,817 frees, nothing lost), so the claim holds, but the
check has to be done another way (step 1 below).

## 2. Speed and size

Native at `-O2` against the interpreter, before and after R.6, in ms:

| Benchmark | Native | Interpreter after R.6 | Interpreter before R.6 |
|---|---:|---:|---:|
| `while`, 1M | 4–7 | 116–134 | 143–213 |
| fib(30) | 16–28 | 218–222 | 709–1,199 |
| `out += "x{i},"`, 100k | 11–14 | 90–91 | 132–176 |
| Map counter, 100k over 100 keys | 4–6 | 148–171 | 243–328 |
| `{ p with … }`, 1M | 25–42 | 94–105 | 585–818 |
| Binary tree, depth 18 | 49–82 | 151–180 | 474–582 |
| `toList \|> map \|> filter \|> sum`, 1M | 29–46 | 309–437 | 579–685 |
| Whole file, wall clock | 207 (50 MB) | 1,717 (128 MB) | 3,740–4,941 |

For comparison, on the same machine: Go fib(30) 5–12 ms, tree-18 15–16 ms; CPython fib(30) 144 ms.
The performance review's guess, "within 2–3× of Go, 50–200× over the interpreter", becomes about
2–5× Go and 10–80× the interpreter, since R.6 made the interpreter 3× faster on calls and records.

| | |
|---|---|
| Native `hello`, start to exit | 0.3 ms (`eye run --memory`: 14.5 ms) |
| Core's test binary, static musl, `-O2` | 3.5 MB as written, 658 KB stripped |
| Build of core, cold / warm | 5.1 s / 0.22 s |
| The pinned zig, unpacked | 411 MB |

### Where native is slower

Reference counting today is decided while C is printed (`cgen/emit.go`). Every value lives until
its function returns, and in-place updates happen only for six exact `v = op(v, …)` shapes. Any
other shape copies:

| Loop | Native | Interpreter |
|---|---:|---:|
| `xs = push(xs, i)` with `fn push(xs, x) = xs.add(x)`, 40k | 3,752 ms | 19 ms |
| `let next = m.insert(i, i); m = next`, 20k | 9,839 ms | 32 ms |
| `{ p with … }` while `p` is still live | copies every time | in place since R.6 |

The interpreter doesn't depend on the shape, because its lists share backing arrays and its maps
are persistent vectors (R.3). Native code needs 7.5 to match it.

### Against Go

The owner's bar for native code is **faster than Go, on less memory**, with Go as the baseline.
7.7 ([#110](https://github.com/ruxxzebre/eyelang/issues/110)) measures it: nine benchmarks in
`testdata/bench`, each an Eyelang program and the same algorithm as idiomatic Go, run by
`go run ./tools/bench`. The full table, the machine and how to run it are on
[Benchmarks against Go](../benchmarks.md); `--check` fails when a ratio gets worse than recorded.
Recorded on 2026-10-12 on the survey's machine, in ms and MB (time is the fastest of 5 runs, memory the peak RSS):

| Benchmark | Native | Go | Time × Go | Native MB | Go MB | Memory × Go | Interpreter |
|---|---:|---:|---:|---:|---:|---:|---:|
| fib(37) | 455 | 96 | 4.74× | 0.16 | 2.03 | 0.08× | 7,382 |
| mandelbrot, 1600² | 278 | 118 | 2.34× | 0.16 | 1.97 | 0.08× | 26,318 |
| trees, depth 16 | 1,051 | 468 | 2.25× | 80.2 | 13.0 | 6.17× | 7,749 |
| records, 30M `with` | 1,346 | 284 | 4.75× | 0.16 | 1.97 | 0.08× | 9,761 |
| strings, 1M parts | 378 | 188 | 2.01× | 83.0 | 43.4 | 1.91× | 3,323 |
| lists, 1M | 179 | 162 | 1.10× | 42.3 | 46.5 | 0.91× | 3,312 |
| maps, 5M lookups | 548 | 97 | 5.67× | 1.18 | 2.53 | 0.47× | 16,824 |
| logs, 200k lines | 383 | 263 | 1.46× | 54.7 | 59.9 | 0.91× | 1,671 |
| lexer, 7.5 MB of Eyelang | 1,786 | 388 | 4.61× | 478.0 | 191.8 | 2.49× | 28,247 |

**No benchmark beats Go on time**: native code takes 1.1–5.7× as long, 2.8× on the geometric mean.
**Six of nine use less memory** (0.57× on the geometric mean), but four of those only because a static musl binary starts at
0.16 MB and a Go program at 2 MB; lists and logs hold real data and come in just under Go, whose
heap grows to about twice what is live. Trees, strings and the lexer use more. Between a quiet and a busy
stretch of this shared machine a time ratio moved by up to 40% (fib, 3.5× to 4.9×), so the check
allows 25% and measures a benchmark that looks worse twice more before it fails.

**After 7.14** (the cheap fixes: one translation unit, a size-class allocator, static field-less
variants, integer formatting, constant division). The session ran on a different 4-core host, so
the code before and after was measured there within the hour; Go's own times there move by up to
70% with the machine's load (the lexer's from 516 to 888 ms), native code's by under 10%:

| | fib | mandelbrot | trees | records | strings | lists | maps | logs | lexer |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| Native ms, before | 600 | 361 | 1,810 | 2,359 | 618 | 342 | 1,001 | 675 | 2,918 |
| Native ms, after | 522 | 273 | 672 | 1,702 | 222 | 240 | 601 | 325 | 1,540 |
| Time × Go, before | 4.23× | 1.61× | 2.94× | 9.39× | 1.65× | 1.53× | 5.70× | 1.49× | 3.29× |
| Time × Go, after (recorded) | 3.62× | 1.20× | 1.13× | 7.08× | 0.63× | 1.38× | 3.89× | 0.94× | 2.98× |

Peak memory fell on the programs that hold data: trees 80 → 25 MB, strings 83 → 68, logs 55 →
44, the lexer 477 → 200 (1.04× Go's). The start-up floor grew from about 0.16 to 0.28 MB: the
binary's code is larger and laid out differently, and the kernel maps code in 64 KB windows.
Strings and logs now beat Go on time; 1.90× Go's time on the geometric mean, from 2.87×. What is
left is 7.13's and 7.5's: fib and records are `dup`/`drop` and boxed records.

Where each gap comes from, read from the emitted C (`eye build --emit-c`) and from callgrind on
the `-O2` binaries:

- **Every value moves through a `dup` and a `drop`.** The C copies each value into a fresh
  temporary (`t_ = v; ey_dup(t_); ey_drop(old); old = t_`), each a test of the value's tag even
  for a small integer, and every return drops every local of the function. fib spends about 100
  instructions a call, 60% of them in these inlined helpers. 7.5a's counting in the low IR
  removes most of them.
- **`Int` is a tagged word with overflow checks, and `<` is a call.** `Int` never overflows, so
  `+` and `-` test both tags and the overflow flag. Comparison is `ey_int_cmp`, a call into
  `int.c` (each runtime file is compiled on its own, so nothing in it inlines), though `eye.h`
  has an inline `ey_int_lt`; `/` and `%` call `divmod`, where Go multiplies by a reciprocal.
  mandelbrot's floats are unboxed, and its whole gap is the `Int` loop counter; in lists and maps
  `divmod` is 9% of the instructions.
- **Records and variants live on the heap, and `with` copies.** A record is an object with a
  24-byte header (count, type, tag, size) and a word per field; Go keeps the `particle` in
  registers. In records, `p = { p with … }` dups `p` before `ey_agg_own`, so the count is 2 and
  every step allocates a new record and frees the old one. 7.5a's "in place when unique and dead"
  removes the copy; keeping small records out of the heap would close the rest.
- **A variant without fields allocates.** `Leaf` is a fresh `ey_agg_new(…, 0, 0)` each time, so
  a tree has as many leaf objects as nodes, beside 40-byte nodes against Go's 16. Values also live
  until their variable is reassigned or the function returns: the stretch tree stays to the end of
  `main`, and the last loop tree stays while the next is built. trees peaks at 80 MB against Go's
  12. A static, immortal value per field-less variant needs no 7.5; drop after last use is 7.5a,
  and reusing a matched cell is 7.5b. `malloc` and the recursive, type-driven `ey_free` take 35%
  of trees' instructions; Go's collector does little in a run this short.
- **Lookups box an `Option`.** `xs[i]` and `m[k]` allocate a `Some` each time, which `??` then
  takes apart, so maps allocates and frees twice per word just to look it up, and hashes and
  compares keys through the generic `ey_hash` and `ey_eq`. Go's `counts[w]++` is one specialised
  map access.
- **Strings format through `printf` and split by copying.** An `Int` in `"{…}"` is written by
  musl's `snprintf` (20% of strings' instructions); Go uses `strconv`. `split` copies each part
  into a new string with its own header, where Go's parts share the original's bytes, which is
  strings' extra memory.
- **`sort` is generic.** A merge sort comparing through `ey_cmp` on type descriptors: 42% of
  lists' instructions. Go's `slices.Sort` is a pdqsort compiled for `int`.

What it takes to clear the bar, roughly by payoff: 7.5a (drop after last use, in place when unique)
for records, trees and every `dup`/`drop`; inlining the runtime's fast paths, by compiling the
program and runtime as one translation unit or with LTO; static field-less variants; lookups that
don't box an `Option` when `??` or a `match` takes it apart at once; integer formatting without
`printf`, substrings that share bytes, and sorts specialised by element type; then unboxed small
records. For scale, the interpreter takes 4.5–92× native code's time (mandelbrot's floats are
its worst case), and its strings run holds 847 MB.

## 3. What the R tasks changed

| Task | What it did | Measured |
|---|---|---|
| R.1 | Exhaustiveness uses the default matrix | Checking time no longer explodes on wide matches |
| R.2 | Waking waiting runs is O(log n): a timer heap and an index by event | Wake-ups no longer scan every run |
| R.3 | Map value replacement and entity writes are O(log n) | Map counter over 10k keys: 6–11 s → 40 ms |
| R.4 | String `+=` is amortised O(1) | 100k appends: 18 s → 90 ms |
| R.5 | The journal: raw JSON records, a commit writes only new steps, grouped syncs | Starting 5k runs: 3.9 s → 0.37 s; a looping job's commit: 1.9 KB → 0.45 KB |
| R.6 | The interpreter's calls, returns and records | fib(30) 2.7×, record updates 3.5–3.8×, `return`/`break` 6.5× |
| R.7 | A waiting run keeps a 48-byte handle in memory; its record loads when it wakes, and a restart reads none | 20,000 runs sleeping a day, idle RSS: 51 → 25 MB on the journal, 54 → 26 MB on Postgres (an idle runner is 18–20 MB); restart 0.67 → 0.25 s and 0.61 → 0.16 s; live heap per waiting run 685 → 133 B |

What this means for the native plan:

- **The interpreter is no longer the obvious bottleneck.** Before the R tasks, the interpreter
  was 25–100× slower than native code and quadratic on strings and maps. Now it is linear
  everywhere it should be, and calls are within about 1.5× of CPython. Native code is still
  10–80× faster on computation, but the lab does little computation.
- **Idle cost and resume time, v0.4's exit test, are runner properties.** They are what R.2, R.5
  and R.7 improved, and they stay in Go under every hosting option.

## 4. What 7.5 should be

ROADMAP 7.5 lists precise `dup`/`drop` from liveness, inferred borrowed parameters (Lean's
scheme), reuse of a matched cell for a constructor of the same size, in-place updates for any
unique value, `mut` as pointers, and a fault that frees its run's region whole. `mut` as pointers
is already done (`cgen/cgen.go:605-608`).

**Proposal: 7.5 in two parts.**

- **7.5a, needed now.** Put `Dup`, `Drop` and in-place updates into the low IR, as 7.2 promised,
  instead of deciding them while printing C. Drop a value after its last use, infer borrowed
  parameters, and update in place whenever the operand is unique at run time and dead after the
  operation, whatever the shape around it. This is what fixes section 2's slow cases. Putting it
  in the low IR means another back end (WASI, a self-hosted one) gets it for free.
- **7.5b, later.** Reuse of a matched cell, and regions freed whole on a fault. Both save
  allocations, but nothing in the repository is slow without them, and faults are rare in
  programs that pass their tests.

**7.5a is done when:**
- the loops in section 2's second table, as tests under `cgen/testdata`, run in linear time
  natively (each 10× input takes at most about 12× as long);
- a counting allocator, built into the runtime in debug builds, finds as many frees as
  allocations in every program `eye build` compiles (this replaces valgrind, which can't see the
  static musl heap);
- `eye check --lir` prints the `dup`, `drop` and in-place instructions, with golden tests.

## 5. Hosting compiled programs (7.6)

7.6's interface stays as compilation.md describes it: run `main`, run a test, call an entity
method on a state, step a job's state machine, and encode or decode each saved type. The open
question is how the Go runner and the compiled C meet. The survey built each option for real:

| Option | One file to deploy? | Static on Linux? | Cost per crossing | What `eye build` needs | Catch |
|---|---|---|---|---|---|
| **A.** The runner opens the program as a shared library through purego (`internal/ffi`) | No: runner and `.so` | No: needs a C library in the image | 290–320 ns a call, 330–380 ns a callback | zig | At most 2,000 callbacks per process, never freed; arguments must be word-sized |
| **B.** The runner built as a Go `c-archive`, linked into the program by zig | Yes | **Crashes at start-up** on static musl (`runtime.sysargs`: musl doesn't pass `argc`/`argv` to constructors), on Go 1.24 and 1.27 | 45–79 ns (on glibc) | zig, and an archive per target in the release | Would mean dynamic glibc on Linux |
| **B′.** The runner is `main`; the program's C is linked in through cgo, zig as the C compiler | Yes | Yes (4.7 MB, also cross-built for arm64) | 31–39 ns | zig and a Go toolchain | Go at build time |
| **C.** The runner rewritten in C or Eyelang | Yes | Yes | none | zig | About 12,400 lines of runner, plus an HTTP client with TLS, a Postgres client and JSON, before the first native job runs |

Every option keeps `eye` itself free of cgo, which was the condition in #79: in B′ the cgo is in
the user's build, done by a Go toolchain that `eye build` fetches and pins the way it pins zig
(Go is about a 70 MB download; zig is 411 MB unpacked).

**Which matters depends on a number nobody has measured:** how many times a digest run crosses
between compiled code and the runner. Every entity call, `!` step, encode, decode and store read
is a crossing. If a run makes tens or hundreds, A's 300 ns is nothing and its catches are the
only cost. If code crosses inside hot loops (an entity read in a `for`), B′'s 35 ns matters.

**Measured (7.10, 2026-10-12):** the interpreter counts these operations (`eye run --crossings`,
`eye test --crossings`). One morning of the digest, run on the journal against stand-ins for its
services, crosses **214 times**: 2 in `main`, 36 in `digest.morning` and 176 in `digest.write`.
Most are encodes and decodes of saved values (114 and 10), then contract operations (39 and a
reconcile), `env.get` (21) and `http.get` (16); there are 3 turns of a run, one entity call and
one entity read. At 35 ns that
is about 7.5 µs, against a run that took 267 ms on stand-ins that answer at once and takes
seconds with a real model: under 0.003% of the run. Even at A's 300 ns it would be about 64 µs.
No crossing is in a hot loop; the most frequent come once per feed, page or step. Across the
lab's 31 tests the busiest run crosses 190 times. So B′'s cost per crossing doesn't matter for
these programs, and nothing needs batching; what decides between the options is one static file,
not the 35 ns.

**Recommendation:** B′, unless the owner doesn't want `eye build` to fetch Go. It is the only
option that gives one static file, which runs in a `scratch` image with nothing beside it, and
it has the cheapest crossings. Programs with no entities or jobs keep building with zig
alone, as they do today. A is the fallback: it works now with code we have, at the cost of a
dynamic runner and two files. C is not worth it before self-hosting, and B is out on Linux until
Go's musl `c-archive` start-up is fixed.

Two smaller choices come with 7.6:

- **The store stays in the Go runner.** `check/runtime/store.eye` would need 7.5's precise
  counting to commit in place if it were compiled. Kept in the runner, compiled code reaches
  state only through entity calls, which are crossings anyway.
- **Tasks are hosted by the runner first.** `both`, `race`, `timeout`, `taskGroup` and channels
  run on Go's scheduler today. Compiling them to C means one thread, non-blocking input and output,
  and a scheduler in C (compilation.md, section 2), which only the `tasks` program needs.

## 6. Proposed order

Each step is one task with an issue, closed by its done-when.

| Step | What | Done when |
|---|---|---|
| 0 | Fix stale text: ROADMAP 7.4's "48 tests" (now 60); compilation.md's line counts (interpreter 18,127, runner about 12,400) and its valgrind sentence; the performance review's quadratic and fib numbers | The pages say what this page measured |
| 1 | Wider cross-check: every program `eye build` compiles runs at `-O0` and `-O2` against the interpreter; the benchmark file and section 2's slow loops become tests under `cgen/testdata` | `go test ./cgen` runs both levels; the slow loops are recorded as known-quadratic until 7.5a |
| 2 | 7.5a, as in section 4 | Section 4's three checks |
| 3 | Core gaps in C: the time methods, `toJson`, `Schema`/`Save`, `random.*`, `http.get`; the `Date.at` message | The lab's 6 core-blocked functions compile; `testdata/programs/limits` gets past `Instant.date` |
| 4 | Count crossings: the interpreter counts, per run, the calls that would cross the hosting boundary (entity calls, steps, encodes, decodes, store reads), and records them for one digest morning | A number in ROADMAP 7.6, and the owner's hosting decision made on it |
| 5 | 7.6 with the chosen hosting: the five interface functions, the runner calling them, the store and tasks staying in Go | The digest runs as a native job on the journal and on Postgres, with idle memory and resume time recorded beside the interpreted numbers |
| 6 | Close the speed gap with Go (7.11), driven by the 7.7 benchmarks | Native code beats Go on time on every benchmark of the suite, the lexer included, and on peak memory for every program without entities or jobs |
| 7 | 7.5b: matched-cell reuse and fault regions (7.12) | Allocation counts drop on the tree and pattern benchmarks; a failing test frees its region |

After v0.4 comes **S.2, the runner in Eyelang**. Under B′, every program with entities or jobs
carries the Go runtime, so its memory floor is Go's. Moving the runner into Eyelang, part by part
as self-hosting moves the compiler, is how job programs beat Go on memory too.

The steps have issues: 7.8 is #111, 7.9 #112, 7.10 #113, 7.11 #114, 7.12 #115 and S.2 #116.

Steps 0, 1 and 3 need no decision and can run in parallel. Step 2 needs the owner to agree to the
split. Step 5 needs the hosting decision.

**The bar for every step is "faster than Go, on less memory"**, as the owner set it (7.7). Each
step that changes native code runs `go run ./tools/bench --check`, which fails when a ratio to Go
gets worse than recorded, and records its gains with `--write`. Section 2's
[Against Go](#against-go) lists what the gaps come from; 7.5a closes the largest, and the rest are
smaller runtime and back-end tasks that come after it.

### Alternatives considered

- **Pause native work after 7.5a.** Close Phase 6's exit test (6.6 with Notify, 6.8 with agents),
  start self-hosting (S.1) and the package manager's first step, and come back to 7.6 when a
  workload needs native speed. The lab doesn't today. This is a reasonable order; it just moves
  v0.4 later.
- **Relax v0.4's exit test.** Make it "the digest's idle memory and resume time are measured"
  (which R.7 and the interpreter can meet) plus "the digest's pure functions compile natively and
  match", and leave a fully native job to v0.5. That puts v0.4's goal back on its pillar instead
  of on the engine.
- **Interpret the low IR instead of compiling it.** Not worth it: after R.6 the tree-walking
  interpreter is close enough to CPython, and a second interpreter would double what the
  cross-check must compare.
- **The playground first.** It doesn't need native code: `eye` builds for `js/wasm` and
  `wasip1`, and under Node's WASI it passes core's tests. The emitted C also compiles to
  `wasm32-wasi` with the pinned zig, once `rt.c`'s `setjmp` and `sys.c`'s `fork` get a WASI branch
  (core's tests then pass in 18 ms; 403 KB stripped). Neither blocks nor is blocked by 7.5 or 7.6.
- **Self-hosting S.1.** Runs on the interpreter and needs neither 7.5 nor 7.6. As the first large
  pure-core program, it would be a good cross-check and benchmark for native code once it exists.

## 7. Decisions for the owner

The [benchmark review](./benchmarks-review-2026-10-12.md) reads 7.7's and S.1's numbers and
updates the recommendations below; where the two differ, the review is newer. **The owner
accepted the review's recommendations on 2026-10-12**, B′ hosting among them; ROADMAP's phase 7
has the tasks and their order. Its main change:
a new task, values laid out by type (7.13), before or with 7.5a.

The owner's bar, set on 2026-10-12: native code faster than Go, and using less memory. The owner
will decide the points below once 7.7's benchmarks against Go and S.1's lexer benchmark are in,
since their numbers may change parsing or more fundamental choices.

1. **7.5 split.** Do 7.5a now (counting in the low IR, drop after last use, borrowed parameters,
   in place for any unique value) and 7.5b later? *Recommended: yes.*
2. **Hosting for 7.6.** B′ (one static binary, `eye build` fetches a pinned Go), A (purego, two
   files, dynamic runner), or wait for step 4's crossing count first? *Recommended: decide after
   step 4, leaning B′.*
3. **The store and tasks stay in the Go runner** for 7.6. *Recommended: yes.*
4. **v0.4's exit test:** keep "the digest runs as a native job", or relax it as in section 6's
   alternatives? *Recommended: keep it, but measure idle memory and resume time on the
   interpreter now (after R.7) so the native numbers have something to beat.*
5. **Order against Phase 6's exit test, S.1 and packages.** Steps 0 to 4 are small; 7.6 is not.
   *Recommended: run steps 0 to 3 now, then pick between 7.6 and Phase 6's exit test.*
6. **A `wasm32-wasi` target for `eye build`** (playground output, plugins), as a later task?
   *Recommended: later, after 7.5a, since it shares the fault path that 7.5b changes.*
7. **The bar against Go.** The owner set it: native code faster than Go and on less memory (7.7).
   Open: does it hold per benchmark or on the geometric mean, and do the runtime fixes in
   [Against Go](#against-go) that need no 7.5 (inline `<`, static field-less variants, lookups
   without a boxed `Option`, formatting without `printf`, typed sorts) become tasks now?
   *Recommended: per benchmark for time and memory, as `--check` records them; the small fixes as
   one task after 7.5a, measured by the suite.*
