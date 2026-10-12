---
title: Compiling Eyelang
---

# Compiling Eyelang

This is the design for phase 7 (v0.4): native code, reference counting with in-place updates,
generic code specialised per type, and jobs that resume from saved state. SPECS.md §22 refers to
it as "the compilation analysis".

It was written on 2026-10-11 against `dev` at 1cc429d. Recommendations are marked
**Recommendation**, points that are still open are marked **Question**, and points the user has
settled are marked **Decided**, as SPECS.md's decision log records them. The decisions it asked
for are collected in [Proposals](/internals/proposals).

**Decided 2026-10-11:** the back end is C, and `eye` ships `zig` to compile the C it emits
(section 1); jobs resume from saved frames in the interpreter in v0.3, before the C back end
exists (section 4); in v0.4 a waiting run is a record in the store, not a process (section 4,
"Processes"). One fact changed after this was written: `eye` did not move to tree-sitter and cgo.
The hand-written parser stays, tree-sitter is used only in tests, and `eye` itself stays free of
cgo, which changes how the runner hosts compiled code (section 1, "Where the runtime lives").

Phase 7 is done when the morning digest runs as a native job, with its idle cost and resume time
measured and recorded in ROADMAP.md.

## What exists today

**Since this was written:** 7.1 is done. Every expression of a program that checks has its
resolved type in the IR (`ir.Ty`, read with `Expr.Ty()`, checked by `TestTypedIR`); a method
call has its `Target`, an operator on a declared type its `Impl`, and each function its
`Effects`; `eye check --ir` prints it. Function types state their effects, and a function kept in
data does no more than its type says. A record value knows its declared type (`Record.Type` in
`interp`), which it keeps when it is saved; nothing finds a type from a record's fields any more,
so a record saved before records kept their type comes back without one. 7.2 is done too: the
low IR (package `lir`, `eye check --lir`) is monomorphic, in blocks of named values, with the
domain constructs lowered to kinds of function and runtime calls; reference counts come with
7.5. 7.4 is done (2026-10-12): `eye build` compiles the core language to native code through C
(package `cgen`), described in section 1, "The C back end as built". What follows describes
`dev` at 1cc429d.

The pipeline has three stages, all in Go:

1. `syntax` parses a file into a lossless tree.
2. `check` resolves names and lowers the tree to the IR in `ir/` (`check/lower.go`). Then it
   infers types and effects over that IR (`check/infer.go`, `check/effects.go`).
3. `interp` walks the IR.

### The IR

[`ir/ir.go`](https://github.com/ruxxzebre/eyelang/blob/dev/ir/ir.go) is a resolved tree, not a
typed one:

- Every name points at its declaration (`Ref.Decl`) or a numbered slot of the current frame
  (`Local.Slot`). Pipes, `_` and interpolation are already plain calls.
- Types survive only in three places: `Call.Expected` (the inferred type of a call's result, which
  a model call's schema comes from), `Record.Type`, and `TypeExpr`, which is a type *as written*
  in a signature, not a resolved one.
- `MethodCall` is dispatched "by the value's kind at run time". A trait method, an operator on a
  declared type and a built-in method are all found when the call runs.
- Built-ins are `Builtin{Name}`. Most are declared in `ir/builtin.eye`. Eight that take functions
  (`poll`, `both`, `race`, `timeout`, `taskGroup`, `next`, `mapErr`, `reserve`) and the
  collection functions still have rules written in Go (`check/builtindecls.go`,
  `check/builtintypes.go`).
- Entities, jobs, views and contracts are declarations of their own (`ir.Entity`, `ir.Job`,
  `ir.View`, `ir.Contract`), and the interpreter runs them directly. They are not lowered to core
  IR, which SPECS §22 step 5 says they would be.

The checker computes a type for nearly every expression, then throws it away. Where it can't, it
uses `unknown`, which agrees with every type. Track B's audit counted about 39 places that
produce `unknown`, mostly in `check/infer.go`, and function types in signatures are entirely
`unknown` today.

### The interpreter

[`interp/`](https://github.com/ruxxzebre/eyelang/tree/dev/interp) is about 10,700 lines of Go
without its tests. The parts that matter for compilation:

- **Values** are Go `any`: `int64`, or `*big.Int` once an `Int` outgrows it; `string`; `*List`,
  `*Set` and `*Map` over Go slices, with sets and maps keeping insertion order; `*Record` with
  its field names; `*Variant`; `*Closure` with copied captures.
- **No reference counts.** Most updates make a new value: `Record.with` copies, and so does
  `+` on strings, except as below. Appending to a list and adding a new key to a map or set share a backing
  array and are amortised O(1), and so is `s += t` (or `s = s + t`) on a local `String`
  variable: the variable's slot holds a byte buffer with room to spare (`textBuf` in
  `interp/text.go`), and reading the variable gives a Go string over its bytes, without a copy.
  A map keeps its values in a persistent vector (`interp/vec.go`), so replacing a key's value
  copies one path of 32-way nodes, O(log n), and so does an entity write, which replaces a row
  in the store's map of its type. The store keeps runs in buckets of 64
  (`check/runtime/store.eye`).
- **A record knows its type** since 7.1 (2026-10-11): the checker writes it into the typed IR,
  and the interpreter no longer guesses it from field names.
- **`mut` is copy in, copy out.** The callee returns the changed value and the caller writes it
  back to the place (`evalCall` in `interp/eval.go`).
- **Jobs resume by replaying.** A run is its arguments and a log of steps
  (`jobRun` in `interp/runtime.go`). Resuming runs the job from the start, and each `!` step
  already taken returns its recorded result (`replay` in `interp/jobs.go`). A run parks by
  panicking with `parkSignal`. Steps are matched by position only: a step's `Call` name is kept
  for `eye runs`, not compared, so a run paused under older code replays its results into
  whatever the new code does at that position.
- **Tasks are goroutines.** `parallelMap`, `both`, `race`, `timeout` and task groups run on Go's
  scheduler, or take turns on a test's clock.
- **`@memory` counts a run's saved state**: its arguments and every step's result
  (open question 26).
- **The store is Eyelang.** `MemoryStore` in `check/runtime/store.eye` is interpreted like any
  other code. The journal and the Postgres store wrap it, and both read everything into memory
  when they open.

### What compilation needs that isn't there

| Needed | Today |
|---|---|
| A known type for every expression | Types are discarded; `unknown` in about 39 places; function types untyped |
| Calls resolved to their target | Methods, traits and operators are found at run time |
| Generic code specialised | One generic interpreter for every type |
| Values with layouts and reference counts | Go values, garbage-collected, copied on update |
| Jobs that resume where they stopped | Jobs replay from the start |
| A runtime for compiled code | The runtime is the interpreter |

## 1. The back end

Four back ends fit a compiler written in Go that later compiles itself.

| | C | QBE | Cranelift | LLVM |
|---|---|---|---|---|
| What we emit | C11 source | QBE's text IL | Cranelift IR, through its Rust API | LLVM IR, as text or through bindings |
| Code quality | As good as `clang -O2` | Well below `-O2`; few optimisations by design | About `-O1` | The best |
| Compile speed | Slow at `-O2`, fair at `-O0` | Very fast | Fast | Slowest |
| What the build needs | A C compiler: `zig cc`, `clang` or `gcc` | QBE (small C program) plus an assembler and linker | A Rust library; no stable C API | LLVM, tens of MB, a new major version each year |
| Our five release targets (linux, darwin on amd64 and arm64, windows/amd64) | All | No Windows | All | All |
| Debug info | `#line` maps C back to `.eye` lines | Little | Some | Full |
| Calling C (`@library`) | Direct call, headers optional | By ABI | By ABI | By ABI |
| From Go today | Write text | Write text | A Rust shim behind cgo | Write text, then `clang` |
| From self-hosted Eyelang | Write text | Write text | Needs FFI into Rust | Write text |
| Who uses it | Koka, Lean 4, Nim, Chicken Scheme | Hare, cproc | Wasmtime, Rust's debug backend | Rust, Swift, Zig, Crystal |

**Recommendation: emit C.** **Decided 2026-10-11:** accepted.

- **The workload doesn't need the best code.** Eyelang programs mostly wait. Native code matters
  for memory and for resume time, and C at `-O2` is far ahead of the interpreter on both.
- **The bootstrap seed is C already.** SPECS §22 says releases publish the self-hosted compiler's
  generated C, so a fresh machine can build Eyelang without an Eyelang binary. With a C back end
  the seed is just the compiler's normal output.
- **One toolchain for every target.** `zig cc` cross-compiles C to all five release targets from
  any host, so `eye build` can ship it and use it everywhere. `eye` itself needs no C compiler:
  it stays free of cgo, and only `eye build` compiles C.
- **The closest prior art is C.** Koka and Lean 4 compile functional languages with reference
  counting and in-place reuse to C. Their runtimes and papers are the model for section 2.
- **No garbage collector means no stack maps.** The usual problem with C as a target, finding GC
  roots on the stack, doesn't arise.
- **`@library` becomes a plain call.** An `extern fn` from a C library is a C prototype in the
  generated code, linked like any other symbol.

What C costs, and what to do about it:

- **Compile time.** Emit one C file per module and cache object files by content hash. Build
  with `-O0` for `eye test` and the REPL, which keep the interpreter anyway, and `-O2` for
  `eye build`.
- **Undefined behaviour.** Emit fixed-width arithmetic as unsigned operations or compiler
  builtins (`__builtin_add_overflow`), and build with `-fwrapv -fno-strict-aliasing`. `Int`
  never overflows: small values are checked and spill to a big integer.
- **Debugging.** Emit `#line` directives so that debuggers and crash reports point at `.eye`
  lines.

**Recommendation:** keep a low-level IR between the typed IR and C (section 5). C is printed from
it, and a second back end can be added later from the same IR. QBE is the likely second, for
fast debug builds, if C compile times hurt.

**Decided 2026-10-11:** `eye` ships `zig` (a large download, tens of MB) and compiles the C it
emits with `zig cc`, rather than using the system's `cc`. Shipping it makes cross-compiling and
reproducible builds work everywhere, with the same compiler on every machine; using `cc` would
have kept `eye` small.

### The C back end as built

Task 7.4 (2026-10-12) built the back end for the core language: package `cgen` and
`eye build`. It is what this section describes, as it stands before reference counting (7.5).

**What `eye build` does.** It checks the program, lowers it to low IR (`lir.Lower`), and writes
C11: one file per module (`m_main.c`, `m_tally.c`, …), `program.h` with every prototype and
type descriptor, and `program.c` with the descriptors, the `dyn` tables, the time zones the
program names, and the entry point. Every position the low IR marks (`lir.At`) becomes a
`#line` directive, so a debugger or a crash report names the `.eye` line. The runtime's C, about
4,100 lines in `cgen/runtime`, is embedded in `eye` and compiled with every program. `eye` runs
the pinned zig (`internal/zig`) as a separate process; `eye` itself has no cgo.

- `-O2` by default, `-O0 -g` with `-g`; always `-std=c11 -fwrapv -fno-strict-aliasing`.
- `--target os/arch` cross-compiles to each release target: `linux/amd64` and `linux/arm64`
  (static, against musl), `darwin/amd64`, `darwin/arm64`, `windows/amd64`.
- `--test` makes a binary that runs the program's test blocks and reports as `eye test` does,
  with `--run` and `-v`; without it the binary runs `main`, and an `Err` from `main` goes to
  stderr with exit status 1.
- Object files are cached in the user's cache directory (`EYE_BUILD_CACHE` overrides it), named
  by a SHA-256 of zig's version, the flags and target, the file and every header. A build marks
  the objects it used, and removes those no build used for 30 days.
- `--emit-c dir` writes the C instead of compiling it.
- zig is fetched once into the user's cache from ziglang.org and checked against the checksum
  `internal/zig` pins, rather than shipped inside the release archives.

**Values.** Every value is one 64-bit word, `ey_v` (`cgen/runtime/eye.h`):

- scalars are their bits: `Bool`, `Char`, `Byte` and the fixed-width integers (signed ones
  sign-extended), `Float`, `Duration` (ns), `Size`, `Date` (days since 1970), `Time` (seconds
  since midnight) and `()`;
- an `Int` is a small integer `n` as `(n << 1) | 1`, or a pointer to a big integer;
- everything else is a pointer to a heap object, which starts with a reference count and its
  type's descriptor.

A descriptor (`ey_type`) is what showing, comparing, hashing and freeing a value needs: its kind,
and its fields', elements' or variants' descriptors. The back end writes one per type the program
uses, so derived `Eq`, `Ord`, `Hash` and `Show` are runtime functions driven by descriptors, not
generated code per type. A newtype (one variant with one field, such as `Offset`) is its field.
Records, variants and tuples are one object with their fields in declared order; a variant
also holds its tag. A `dyn Trait` is an object with the value and a table of the impl's
methods. A closure is its code and its captures.

**The runtime** (`cgen/runtime`, written for this back end):

| File | What it holds |
|---|---|
| `rt.c` | allocation, freeing by descriptor, faults, `print` and `log`, running `main` or the tests |
| `int.c` | `Int`: the small fast path inline in `eye.h` with `__builtin_*_overflow`, and a big integer of 32-bit limbs (schoolbook multiplication; division a limb at a time by one limb, and by shifting and subtracting by more), written for this runtime, not vendored |
| `str.c` | UTF-8 strings and `Bytes`, `Offset` positions as byte offsets, searching, slicing, parsing numbers |
| `coll.c` | lists; sets and maps in insertion order with an open-addressing hash index; the collection functions |
| `iter.c` | lazy `Iter` values (a source and its steps), cursors that pull one element at a time through them, and the walks `for` loops use |
| `value.c` | equality, order, hashing and `Show`, exactly as the interpreter prints values |
| `fixed.c` | the fixed-width integers' methods: wrapping, checked, bits and shifts |
| `time.c` | instants, dates and zones; the zone tables come from Go's zone database, written into `program.c` for the zones the program names |
| `sys.c` | the `fs`, `process` and `stdio` primitives; in a test binary, relative paths are inside a directory of the test's own |

**Counting.** Counts are simple and not atomic, since a program runs on one thread. Each C
variable of the low IR owns one reference, dropped when the variable is set again and when the
function returns; arguments are borrowed and results owned. A parameter the function sets takes
a reference of its own first. Faults in a test `longjmp` out and leak what was live, as the plan
for faults (section 2) allows until runs get regions.

**Updates in place.** Simple counts already make the common loops linear. A variable updated by
itself (`s += t`, `s = "{s}…"`, `xs += ys`, `xs = xs.add(x)`, `seen = seen.add(x)`,
`m = m.insert(k, v)`) becomes one call that takes the variable's value and changes it in place
when its count is one, growing strings and lists by doubling, and copies it first otherwise. A
record's field assignment does the same through `ey_agg_own`. Measured on 2026-10-12 against the
interpreter's quadratic cases (performance review, section 1.1): 100k string appends take 18 ms
(18 s interpreted), 20k map updates over 10k keys 2 ms (10.8 s), 1M field updates 10 ms.
Precise counting (7.5) extends this to values whose last use is a call, not an assignment.

**Not compiled yet.** Entities, jobs, views and contracts (the runner hosts them, 7.6),
durable `!` steps, `with` in tests, foreign code (`@library`), and the clock and fakes of tests.
A program that declares one gets an error that names what it declares and says `eye run` and
`eye test` run it. `parallelMap` runs its function on one thread, in order.

**Tests.** `cgen/cgen_test.go` builds `testdata/programs/core` and `traits`, a program of failing
tests and one of in-place updates, runs them, and checks the output is `eye test`'s line for line
(durations aside); builds core for all five targets; runs a `main`. Valgrind finds no invalid
access and no leak in the core program's tests.

### Where the runtime lives

Compiled code needs a runtime, and today the runtime is the interpreter. It has two parts with
different needs.

- **The core runtime** is what every program needs: allocation, reference counts, `Int` with big
  integers, strings, lists, maps and sets, hashing, formatting, and the `fs`, `process` and
  `stdio` primitives. The self-hosted compiler needs only this.
- **The durable runner** is what programs with entities and jobs need: the scheduler, the
  journal, the Postgres store, the HTTP client, JSON, TLS, time zones, model calls and approvals.
  Today it is most of `interp/`, about 6,000 of its lines, tested by every program in the
  repository.

**Recommendation:** write the core runtime in C (and later in Eyelang over `extern` functions),
and keep the durable runner in Go. The compiled program is built as a C library with a small
interface: run `main`, run a test, call an entity method on a state, step a job's state machine,
and encode or decode each saved type. `eye` stays free of cgo, so the runner can't link that
library through cgo inside `eye`. Two ways remain:

- **The runner loads the program at run time.** `eye build` compiles the program to a shared
  library with `zig cc`, and the runner, still pure Go, opens it and calls its interface without
  cgo, with the same loader that task 6.1 builds for `@library`. Every artifact of the release
  stays free of cgo; a deployment is the runner and the program's library, two files. That
  loader is `internal/ffi`, on purego (from 2026-10-12). On Linux it makes the runner a
  dynamically linked binary, which a `scratch` image can't start, so this way needs a base image
  with the C library and its dynamic linker, such as `distroless/base`.
- **The runner is linked into the program.** The release also ships the runner as a static
  library built from Go (`go build -buildmode=c-archive`, which needs cgo when the release is
  built, though not in `eye`), and `eye build` links it with the program into one binary with
  `zig cc`. A deployment is one file; the release carries one more library per target.

**Question:** which of the two? The first reuses 6.1's loader and keeps cgo out of every build,
but not a static runner; the second gives a single binary. Both keep the points below.

- The runner's code is reused, not rewritten, and its tests keep covering it.
- Go's garbage collector stays in the process, but it manages only the runner's own buffers.
  Program values are reference-counted in the C heap, so the "no GC pauses" goal holds for them.
- Go programs that embed Eyelang (`@host`, issue #42) get compiled code through the same
  interface.
- Parts of the runner move into Eyelang later, as the self-hosting plan moves the compiler.

**Question:** is a Go host in every compiled program acceptable for v0.4? The alternative, a
runner in C or Eyelang from the start, means writing a Postgres client, an HTTP client with TLS,
and time-zone handling before the first compiled job runs.

## 2. Values and memory

### Representation

**Recommendation**, following Lean 4 and Koka:

- **Unboxed scalars:** `Bool`, `Char`, `Byte`, the fixed-width integers, `Float`, `Duration`,
  `Size`, `Date`, `Time`, `Instant`, `()`, entity references whose identity is a scalar, and
  variant types whose variants have no fields (an enum).
- **`Int`** is one machine word: a small integer tagged in the low bit, or a pointer to a
  reference-counted big integer. Arithmetic checks for overflow and spills.
- **Heap objects** carry a header with a reference count and a tag: records, variants with
  fields, closures, big integers, strings, and the buffers of lists, maps and sets.
- **Strings** are UTF-8 bytes in a counted buffer. Searching and slicing count bytes, so they
  cost nothing per step; `size`, `take` and `drop` count characters, so they walk the string
  (decision log, "String offsets"). Open question 32 chose an `Offset` type for byte offsets;
  at run time it is a machine word, so it changes nothing in this representation.
- **Lists** are a counted buffer of length and capacity. A slice (`xs[2..5]`) shares the buffer,
  as SPECS §5 says, and holds a reference to it.
- **Maps and sets** keep insertion order, as the interpreter's do, so iteration order doesn't
  change between the interpreter and compiled code.
- **Records and variants** are heap objects with their fields unboxed after specialisation.

**Question:** should small records (two or three scalar fields, such as a `Range(Int)`) be passed
by value, as Swift does, instead of allocated? It saves allocations; it costs a second
representation in the back end.

### Reference counting with in-place updates

The language was designed for this. SPECS §7 rules out aliasing, references and cycles, so
counting is enough to reclaim everything, and a value with one reference can be changed in place
without anyone seeing it.

**Recommendation: Perceus**, as Koka does (Reinking, Xie, de Moura and Leijen, *Perceus:
Garbage Free Reference Counting with Reuse*, PLDI 2021), with Lean 4's borrowed parameters
(Ullrich and de Moura, *Counting Immutable Beans*, IFL 2019):

- **Precise counting.** The compiler inserts `dup` and `drop` from ownership: a value is dropped
  right after its last use, not at the end of its scope. So in
  `let ys = xs |> add(x)` with `xs` dead afterwards, `add` receives the only reference and
  appends in place.
- **Borrowed parameters.** A parameter that a function only reads is borrowed: the caller keeps
  its reference and no count changes. Lean infers this; Eyelang can too, from the typed IR.
- **Reuse.** When a `match` takes a value apart and the arm builds a value of the same size, the
  arm reuses the memory (`{ r with f: v }` on a unique `r`, or a tree rebuilt on the way back up).
- **In place when unique.** `+=`, field assignment, `insert`, `add` and friends check the count:
  one means change in place, more means copy first. This is the copy on write SPECS §7 promises.

Three things are particular to Eyelang:

- **`mut` parameters become pointers.** A `mut` argument is passed as the address of the
  caller's variable, as Swift's `inout` and Hylo's mutable value semantics do. The no-aliasing
  rule (SPECS §7, checked today) guarantees that the variable is the only path to the value
  during the call, so uniqueness depends only on the value's count. The interpreter's copy in,
  copy out has the same meaning.
- **Nothing escapes a run except bytes.** A job's values cross a `!` only as saved bytes, entity
  state goes to the store as bytes, and events and results are encoded too. So the memory a run
  uses while it executes can be counted, and freed, as one region (see `@memory` below and
  section 6).
- **No cycles, by construction.** There are no references, closures copy what they capture, a
  lambda can't name itself, and entities are referenced by identity. No cycle collector is
  needed.

### Tasks and threads

`parallelMap`, `both`, `race`, `timeout` and task groups share immutable values between tasks.
Counting a value from two threads needs atomic operations, which are slow.

**Recommendation:** start with all tasks of a program on one thread with non-blocking I/O. SPECS
§13 promises overlapping waits, no data races and cheap tasks, and one thread keeps all three
without atomics. When parallel computation is wanted, use Koka's scheme: a value handed to
another thread is marked as shared, and only shared values pay for atomic counts.

**Question:** does SPECS mean `parallelMap` to use several cores? §13 talks about overlapping
waits, and CLAUDE.md about safe parallelism. The answer decides whether v0.4 needs the shared
marking.

### Faults

A fault (`unreachable`, a deadline, `OutOfMemory`) ends a run wherever it is. With reference
counting, the values live at that point must still be freed.

**Recommendation:** give each run's execution its own heap region. A fault frees the region
whole; no unwinding code is generated. Values that outlive the execution are bytes by then.
Module-level `let`s are computed once and never freed.

## 3. Generic code

SPECS §8 promises that generic code is specialised for the types it is used with. The interpreter
doesn't need to, since every value is a Go `any`.

**Recommendation: specialise the whole program.** A program is compiled as one unit today: its
modules, the prelude, the standard modules and the store. Packages (open question 5) don't exist
yet. So the compiler can see every use.

- Start from the roots: `main`, the tests, every job, entity method, view, served operation,
  migration and module-level `let`.
- Specialise each generic function, type and impl for each set of type arguments it is used with.
  A record's layout comes from its specialised field types.
- A `where` bound becomes a direct call to the impl that the specialised type has. Trait methods,
  operators on declared types, and derived `Eq`, `Ord`, `Show`, `Hash`, `Save` and `Schema` are
  ordinary functions, generated per type. Run-time lookups such as `typeOf` disappear.
- **`dyn Trait`** is a pointer to the value and a table of the impl's methods, plus a drop
  function. It is the only run-time dispatch, and SPECS shows it in the type.
- **Private functions** that leave their types out are generic (SPECS section 6), and are
  specialised per use like any other generic function.
- **Polymorphic recursion**, a generic function that calls itself at a bigger type, can't be
  specialised. It is rare; report it as an error at the call.
- **Collection functions** (`map`, `filter`, `size`… over lists, sets, maps, ranges, `Option`
  and strings) are intrinsics of the back end, specialised by element type, until they can be
  written in Eyelang with typed function types.

**Question:** SPECS §8 says "since values never alias, a `List(Logger)` is also a
`List(dyn Handler)`". With unboxed elements this conversion copies the list and boxes every
element: a cost at a place that doesn't show one. Should the conversion be written
(`xs |> map(fn x: x)` with the type expected), or stay implicit?

**Question:** specialising everything can grow code. The lab is small enough not to care. For
the self-hosted compiler, measure, and consider sharing one body between types with the same
layout (all reference-counted pointers, for instance), as Swift and C# do.

## 4. Jobs as state machines

### Today: replay

A job is resumed by running it again from the start. Each `!` step already taken returns its
recorded result, so nothing outside happens twice, and output from those steps isn't printed
again (`TestReplayDoesntRepeatOutput`).

Replay has three costs:

- **Time and space grow with the run's age.** A run keeps every step, and every wake replays
  them all. The lab's train watcher (`testdata/lab/trains/trains.eye`, `job watch`) loops every
  45 seconds with four or more steps a loop: over 7,700 steps a day, so the cost of its life
  grows with the square of its age. `eye run` warns past 1,000 steps (open question 9).
- **Code between steps must do the same thing every time.** It does, since anything that could
  differ is an effect and therefore a step. But it also means printing has to be kept quiet on
  replay, which is part of why open question 22 is open.
- **A code change shifts the steps.** Steps are matched by position, so a paused run whose job
  changed gets results for the wrong calls (open question 4).

SPECS §15 describes the other model: "at each `!` the job's live variables are saved". Compiled
jobs will work that way.

### The transformation

The checker already knows which functions wait: a function whose effects include `wait`
(`ir.Func.Waits`). Only those are transformed.

1. **Each waiting function becomes a state machine.** Its frame is a record of a state number and
   the variables live at each `!`. Each `!` call site is a state.
2. **A checkpoint saves the stack of frames** from the job's body down to the `!` that waits:
   the job calls `summarise!`, which calls `reserve!`, whose function calls the model. Each
   frame is saved as the name of its code, its state number and its live variables, encoded with
   `Save`.
3. **Lambdas that wait are waiting functions too.** `reserve!(256 MiB, fn { summarise!(items) })`
   and `poll!(every: 1 min, within: 6 h, fn { miniflux.ready!() })` take functions that wait. A
   function value can't be saved, but a frame doesn't save a function value: it names the code
   statically, and its captures are live variables.
4. **Waiting built-ins are states of the runtime:** a timer for `sleep!` and `sleepUntil!`, an
   event sequence number for `next!`, a request for `approval!`, the outcome for `poll!`, a
   grant for `reserve!`, a child run for `join!`.

What makes this possible is already checked: only values with `Save` may be live across a `!`
(`checkSave` in `check/effects.go`), so every frame can be encoded. One hole matters here: a value
of an `extern type` can be held across a `!` today, while a `Handle` can't. **Decided
2026-10-11** (open question 21): `Handle` goes, and a program's `extern type` can't be saved, so
the hole closes when the checker enforces it (see [Proposals](/internals/proposals)).

### The journal and exactly once

The guarantee stays the same: one commit per step. Today a step's commit holds the run with its
new step appended, the entity writes, the events and the runs it started. Compiled, the commit
holds the run's new frames instead of a longer log, with the same other parts.

- **A crash between an outside call and its commit** still repeats the call when the run resumes.
  That is why commands carry keys and are reconciled; nothing changes there.
- **A run's record stays the size of its live variables.** The watcher's loop costs the same on
  its tenth day as on its first. Open question 9's suggestion to restart the job from inside
  itself (`continue`-as-new) is no longer needed.
- **What replay recomputed must be kept.** `@budget` spending becomes a running total in the run
  record, since it can't be summed from replayed steps. `next!` needs only the sequence number of
  the run's previous step, which it has.
- **`eye runs` shows the steps.** Keep a bounded trail for it: the last steps' names and times,
  and a count of the rest.

### Doing it in the interpreter first

**Recommendation:** write the transformation as a pass over the IR, and have the interpreter run
its output for jobs before the C back end exists.

- One implementation of the state machine, tested by every job test in the repository, before
  any C is generated.
- Open question 9 is fixed for the lab in v0.3, not v0.4.
- SPECS §15 becomes true of the interpreter, which resolves the contradiction between §15 and §22
  (see [Proposals](/internals/proposals)).

The cost: a tree-walking interpreter can't jump into the middle of a loop, so the pass must
produce a flat form, blocks with jumps, and the interpreter needs an evaluator for that form,
used for waiting functions only.

**Decided 2026-10-11:** the second evaluator is worth it. The interpreter resumes jobs from frames
in v0.3 (task 7.3, with 7.1 before it), so open question 9 is fixed for the lab a release before
compiled jobs.

**Built 2026-10-11, without the second evaluator** (`interp/checkpoint.go`). The interpreter
saves one frame, the job's own, at statement boundaries instead of at each `!`. At the start of
each statement of the job's body, nested statements of `while` and `for` loops, `if`s and
`match`es included, a run that has taken steps since its last checkpoint saves its variables,
the path from the body to the statement, and each enclosing `for` loop's elements and index.
The steps before it are dropped, apart from a trail of 20 for `eye runs`. A run that wakes
restores the variables, walks the path in the ordinary tree-walking evaluator, which enters
each loop, branch and arm without evaluating what it already did, and replays only the steps
since the checkpoint. What this gives up against frames at each `!`: a function the statement
calls with `!` steps of its own replays them, since its frame isn't saved, so a loop that never
ends must be in the job's body; and a checkpoint is skipped while a variable holds what can't be
saved. Neither matters to the lab. The C back end still splits waiting functions into states,
as section 4 describes; this was the cheaper way to fix open question 9 in the interpreter. A
checkpoint names its statement by a hash of its typed IR (7.1), which is how a code change under
a paused run is caught (open question 4).

### Runs saved by older versions

Runs saved under v0.2 and v0.3 are step logs. **Recommendation:** when a new runtime finds one,
it replays the log once, with the interpreter, up to the run's current checkpoint, then saves its
frames and carries on. The interpreter stays in the repository (SPECS §22), so this costs little,
and nobody has to drain their runs before upgrading.

### Processes

ROADMAP says that in v0.4 "jobs run as short-lived processes", and issue #8 that "jobs [are]
compiled to native state machines that run as short-lived processes". SPECS §1 says that a
waiting job "is a saved record in the store, not a process or a thread". The design above runs
state machines inside one runner, which owns the store.
**Decided 2026-10-11**
([Proposals, contradiction 1](/internals/proposals#contradiction-1-jobs-as-processes-against-one-runner-per-store)):
"process" means the runner. v0.4's runner is a native process that executes runs as compiled
state machines; a waiting run is a record in the store, not a process, and one runner per store
stays (open question 28 stays open). Worker processes for runs that call C code can come later
without a language change.

## 5. The typed IR (issue #28)

**Recommendation: two levels.**

### Level 1: the typed IR

The current `ir` tree, with what the checker already computes kept instead of thrown away:

- a resolved type on every expression, in a representation of its own (not `TypeExpr`, which is
  a type as written);
- every call resolved to its target: a function, a method of an impl, a built-in, an operator's
  impl, a slot of a `dyn` table, an operation of a contract;
- for each call of a generic function, the impls its `where` bounds use;
- the effects of each function, which today live in a map inside the checker.

The interpreter runs this IR as it runs today's, and can start using the types: a record knows its
type, so `typeOf` goes away. `eye check --ir` prints it for golden tests.

**Proposed "Done when" for 7.1:** the checker writes a type on every expression, and no
expression of any program in the repository (the lab, `testdata/programs`, the docs' programs,
the tour) has `unknown` or an unresolved type variable. This is the same test as track B's second
safety net, "every program that checks is fully typed". `eye check --ir` prints the typed IR, and
golden tests cover it.

Issue #28's text predates the interpreter: it says the IR exists "so phase 4 can turn jobs into
state machines", but phase 4 built replay instead. Its body should say what is above.

### Level 2: the low IR

A second IR, built from the typed IR, that C is printed from:

- monomorphic: every type known and laid out;
- in blocks with jumps, every intermediate value named (A-normal form);
- with `dup`, `drop` and reuse explicit;
- with jobs split into states and frames.

The passes, in order:

1. Specialise generic code (section 3).
2. Lower the domain constructs: an entity method is a function over the state record inside a
   transaction; a job is a function with frames; a view is a predicate; a contract's operation is
   a call into the runner. SPECS §22 asks for one pass per construct; this is where they go.
3. Flatten into blocks and name every intermediate value.
4. Insert reference counts and reuse (section 2).
5. Split waiting functions into states (section 4).
6. Print C.

**Recommendation:** keep the domain constructs as declarations in level 1, where the interpreter
and the tools need them, and lower them only on the way to level 2.

## 6. How the open questions bear on this

**Question 23: where `!` goes on a helper.** `stamp!()`, where `fn stamp() = now()`, is one
recorded step. A helper with its own `!` steps is called with `!` too, and its steps are recorded
one by one. The compiler doesn't need the two to look different: `Waits` tells it which is which.
For the first, the call site is one state and the helper is ordinary code. For the second, the
helper is a state machine whose frame goes on the stack. The difference matters to readers, and to
code upgrades: a waiting helper's states are resume points of every job that calls it, so
changing the helper can change a paused run. No change is needed for compilation.

**Question 24: `dyn Store` and `Self`.** The `Store` trait is `Store(s)`, and `commit` returns
`s`, the store's own type, so `dyn Store` would need to say which store it is. With the runner in
Go (section 1), the store is chosen in Go, and the compiled program only calls `MemoryStore`, so
specialisation needs no `dyn Store`. If the store is ever chosen in Eyelang, it does.
**Recommendation:** make `commit` a `mut self` method. That makes the in-place update explicit
instead of relying on the compiler proving the store unique, and the trait no longer needs its
own type, so `dyn Store` works. `Self` is a bigger change and can wait for a program that needs it.

**Question 26: what `@memory` measures.** A compiled run's memory can be counted exactly, since
each run's execution has its own region (section 2). **Recommendation:** `@memory` bounds the
bytes a run's region holds while it executes; a run that waits holds nothing in memory, and its
saved frames are shown separately by `eye runs`. Until v0.4, the interpreter keeps measuring saved
state, and its documentation says that it is a stand-in. Process memory is the measure only if runs
become processes (contradiction 1).

**Question 9: long and never-ending runs.** Saved frames make a run's record as large as its live
variables, so the cost no longer grows with age (section 4).

**Question 4: code changes while runs are paused.** With frames, a paused run can resume under new
code only if its resume point still exists and its live variables still have the same names and
types. That makes the change detectable, which replay by position isn't. See
[Proposals](/internals/proposals#q9-and-q4-long-runs-and-code-changes-while-runs-are-paused).

## 7. Tasks for phase 7

ROADMAP.md lists these as phase 7's tasks, each with its issue. 7.1 and 7.3 come first and land in
v0.3, so that the interpreter resumes jobs from frames before the C back end exists.

| Task | Needs | Done when |
|---|---|---|
| 7.1 Typed IR (#28) | Function types typed, no `unknown` left (track B) | Every expression of every repository program has a known type; `eye check --ir` golden tests |
| 7.2 Low IR and specialisation | 7.1 | Every repository program lowers to monomorphic low IR; the lab's generic code is specialised; polymorphic recursion is reported |
| 7.3 Jobs as state machines | 7.1 | Every job test passes with jobs resuming from frames, in the interpreter; the train watcher's record stays the same size over a simulated week |
| 7.4 C back end and the core runtime | 7.2 | `eye build` makes a native binary of `testdata/programs/core` on the five release targets, and its tests pass |
| 7.5 Reference counting and reuse | 7.4 | A loop that appends to a list runs in linear time and allocates once per growth; a test counts allocations and frees and finds no leak in the core programs |
| 7.6 The runner hosts compiled programs, and the digest runs natively | 7.3, 7.5 | The morning digest runs as a native job on the journal and on Postgres; its idle memory and resume time are measured and recorded in ROADMAP.md |

## 8. Open points, collected

Still open:

- A Go host in every compiled program for v0.4? (section 1)
- Does the runner load the compiled program as a shared library, or link into it as a static
  library built from Go? (section 1)
- Small records by value? (section 2)
- Does `parallelMap` use several cores? (section 2)
- Is `List(Logger)` to `List(dyn Handler)` an implicit copy? (section 3)
- Code size of full specialisation, measured on the self-hosted compiler. (section 3)

Decided 2026-10-11:

- C is the back end, and `eye` ships `zig` to compile it. (section 1)
- State machines in the interpreter in v0.3. (section 4)
- "Jobs as short-lived processes" means the runner; a waiting run is a record. (section 4, and
  Proposals)
