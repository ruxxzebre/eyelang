---
title: Compiling Eyelang
---

# Compiling Eyelang

For a map of the whole repository first, read [How Eyelang works](/internals/architecture). This
page goes deep on one part of it: native code. Parts of it describe the code as it was on
2026-10-11; where they differ from today, the map and [What runs today](/reference/status) are
current. For example, jobs no longer replay from their start: they resume from a checkpoint (7.3).

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

[`interp/`](https://github.com/ruxxzebre/eyelang/tree/dev/interp) is about 19,600 lines of Go
without its tests. The parts that matter for compilation:

- **Values** are Go `any`: `int64`, or `*big.Int` once an `Int` outgrows it; `string`; `*List`,
  `*Set` and `*Map` over Go slices, with sets and maps keeping insertion order; `*Record` with
  its field names; `*Variant`; `*Closure` with copied captures.
- **No reference counts.** Most updates make a new value: changing a record's field copies its
  values (the copy shares the field names), and `+` on strings copies, except as below. A
  record that only one local variable holds changes in place: a record the interpreter has just
  made and stored in the variable (`ownRec` in `interp/own.go`) takes `p.x += 1` and
  `p = { p with x: 1 }` without a copy, until the variable is read as a whole. Appending to a
  list and adding a new key to a map or set share a backing
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
  back to the place (`callDirect` in `interp/eval.go`).
- **Calls and leaving early.** A call of a function with a body evaluates its arguments
  straight into the callee's frame, which the interpreter reuses once the call returns. A
  `return`, `break`, `continue` or `?` in a statement position sets a flag in the frame that
  each enclosing statement passes up; one inside an expression panics, and the function or loop
  catches it (`interp/control.go`).
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
  other code. The journal and the Postgres store wrap it, and keep in it only the records of the
  runs that are ready: a run that waits or has ended is a handle in the Go runner's index
  (`interp/handles.go`) and a record on disk, read back when it wakes (SPECS.md §15).

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
`eye build`. It is what this section describes, with what 7.14, 7.5 (counting) and 7.13 (layouts) changed since.

**What `eye build` does.** It checks the program, lowers it to low IR (`lir.Lower`), and writes
C11: one file per module (`m_main.c`, `m_tally.c`, …), `program.h` with every prototype and
type descriptor, and `program.c` with the descriptors, the `dyn` tables, the time zones the
program names, and the entry point. Every position the low IR marks (`lir.At`) becomes a
`#line` directive, so a debugger or a crash report names the `.eye` line. The runtime's C, about
5,000 lines in `cgen/runtime`, is embedded in `eye` and compiled with every program. `eye` runs
the pinned zig (`internal/zig`) as a separate process; `eye` itself has no cgo.

- `-O2` by default, `-O0 -g` with `-g`; always `-std=c11 -fwrapv -fno-strict-aliasing`.
- `--target os/arch` cross-compiles to each release target: `linux/amd64` and `linux/arm64`
  (static, against musl), `darwin/amd64`, `darwin/arm64`, `windows/amd64`.
- `--test` makes a binary that runs the program's test blocks and reports as `eye test` does,
  with `--run` and `-v`; without it the binary runs `main`, and an `Err` from `main` goes to
  stderr with exit status 1.
- At `-O2` the runtime and the program are compiled as one translation unit (`eye_unit.c`
  includes every `.c` file), so the runtime's small functions inline into the program's code:
  comparisons, division, allocation and freeing. At `-O0` each file is compiled on its own, in
  parallel, and the runtime's objects are shared between programs.
- Object files are cached in the user's cache directory (`EYE_BUILD_CACHE` overrides it), named
  by a SHA-256 of zig's version, the flags and target, the file and every file it may include. A
  rebuild of an unchanged program only links (core: 0.3 s). A build marks the objects it used,
  and removes those no build used for 30 days.
- `--emit-c dir` writes the C instead of compiling it.
- zig is fetched once into the user's cache from ziglang.org and checked against the checksum
  `internal/zig` pins, rather than shipped inside the release archives.

**Values.** Since 7.13 each type has the layout the low IR chooses for it (section 2, "Values
laid out by type"), and the back end prints it as a C type:

- scalars are their bits in their own width: `Bool` and `Byte` 1 byte, `Char`, `Int32`,
  `UInt32` and `Time` 4, `Int64`, `UInt64`, `Float` (a `double`), `Duration`, `Size` and `Date` 8;
  `()` is one unused byte;
- an `Int` is a word: a small integer `n` as `(n << 1) | 1`, or a pointer to a big integer;
- an enum is its variant's number, a byte;
- a record, tuple or variant type of at most 64 bytes is a C struct (`ey_s12`), passed and
  returned by value, with its own inline `ey_dup_s12` and `ey_drop_s12` that touch only its
  counted fields; `Option(Int)`, `Option(String)` and `Option(Kind)` use a niche, so `None`
  costs nothing, and `Option(Float)` or a `Result` a tag in front of the variants' fields;
- a larger or recursive record or variant type is a box, a pointer to an object with its fields
  inline (`ey_b3`, or `ey_b3_v1` for a variant); a variant without fields is the immediate word
  `(n << 1) | 1`, so `Leaf` allocates nothing;
- lists, sets and maps are pointers to buffers that hold their elements inline: a `List(Token)`
  is one array of 24-byte tokens, and a slice of a list shares its buffer;
- a closure is an object with its code and its captures inline (`NAME_cl`), a `dyn Trait` an
  object with the value inline and the impl's table;
- `String`, `Bytes`, `Key`, `Zone`, `Instant`, a `Range` and the results of commands are still
  pointers to objects (`Layouts.Objects`); strings become views next.

A heap object starts with a 16-byte header: a 32-bit count (`EY_IMMORTAL` for a static one, and
a count that would overflow wraps to it), 32 bits for a boxed variant's number, and the type's
descriptor. A descriptor (`ey_type`) says how the type is laid out: size, alignment, the offset
of each field and each variant's fields, the tag or niche, and for a struct its generated dup and
drop. Derived `Eq`, `Ord`, `Hash` and `Show`, the collections and lazy iterators are runtime
functions that walk values through it, given a pointer to each value; the runtime's builtins
take their arguments by pointer when they are not a word and write their results through an
out-pointer. A newtype (one variant with one field, such as `Offset`) is its field. A closure is
called through `void code(ey_fn *self, void *out, void *const *args)`, whatever its types.

**The runtime** (`cgen/runtime`, written for this back end):

| File | What it holds |
|---|---|
| `rt.c` | allocation (free lists by 8-byte size class up to 256 bytes, carved from 256 KB chunks; `malloc` above), freeing by descriptor, counts of objects, integers in decimal, faults, `print` and `log`, running `main` or the tests |
| `int.c` | `Int`: the small fast path inline in `eye.h` with `__builtin_*_overflow`, and a big integer of 32-bit limbs (schoolbook multiplication; division a limb at a time by one limb, and by shifting and subtracting by more), written for this runtime, not vendored |
| `str.c` | UTF-8 strings and `Bytes`, `Offset` positions as byte offsets, searching, slicing, parsing numbers |
| `coll.c` | lists; sets and maps in insertion order with an open-addressing hash index whose slots keep each key's hash, so a probe compares keys only when hashes match and growing hashes nothing again, and which remembers the last key found, so `m.insert(k, (m[k] ?? 0) + 1)` looks `k` up once; `sort`, by merging, with whole numbers compared as words; the collection functions |
| `iter.c` | lazy `Iter` values (a source and its steps), cursors that pull one element at a time through them, and the walks `for` loops use |
| `value.c` | equality, order, hashing and `Show`, exactly as the interpreter prints values |
| `fixed.c` | the fixed-width integers' methods: wrapping, checked, bits and shifts |
| `time.c` | instants, dates and zones, and the time methods with one `strftime` formatter; the zone tables come from Go's zone database, written into `program.c` for the zones the program names |
| `sys.c` | the `fs`, `process` and `stdio` primitives; in a test binary, relative paths are inside a directory of the test's own |
| `random.c` | the `random` module, drawing the interpreter's numbers: Go's PCG source and `math/rand/v2`'s algorithms, from the interpreter's seed in each test |
| `stack.c` | the program's thread, with a 256 MB stack, and the fault when a recursive function finds it full: recursion too deep, as in the interpreter, not a crash |

**Counting.** Counts are not atomic, since a program runs on one thread. Since 7.5 the back end
prints the low IR's counting (section 2, "Reference counting in the low IR"): `lir.Count` runs
in `Generate`, with the back end's rule for which values are counted, and each `Dup` and `Drop`
becomes `ey_dup` and `ey_drop`; an instruction marked `inplace` calls the runtime's in-place
form (`eyo_list_add`, `eyo_map_insert`, `eyo_concat`, …), which takes its operand; a field
`take`n is moved out of its record and left empty when the record is unique; a closure's
trampoline and a `dyn` method's wrapper dup the arguments of parameters the function owns.
Nothing else in the back end decides a count. Faults in a test `longjmp` out and leak what was
live, as the plan for faults (section 2) allows until runs get regions. A value whose type
holds nothing counted (a scalar, an enum, a struct of those) is never dupped or dropped, and a
struct's dup and drop touch only its counted fields.

**Counts of objects.** The runtime counts the objects it makes and frees. With `EYE_ALLOCS` set,
a binary writes `allocs N frees M faulted K` to stderr when it exits, after dropping its globals;
`K` is what failing tests left behind.

**Updates in place.** An update whose operand's last use it is (`s += t`, `s = "{s}…"`,
`xs += ys`, `xs = xs.add(x)`, `m = m.insert(k, v)`, a set's `add`, `remove`, `{ r with … }`)
takes the operand and changes it in place when its count is one, growing strings and lists by
doubling, and copies it first otherwise; a field assignment changes an inline record in
place, and makes a boxed one unique first through `ey_box_own`. Before 7.5 only six exact `v = op(v, …)` shapes were in place; now the shape
around the update doesn't matter: the native plan's slow loops (an append through a helper, a
map insert through a `let`, `p = { p with items: p.items.add(i) }`) take 8 ms for 1M appends,
7 ms for 100k inserts and 12 ms for 1M updates, and grow linearly.

**Not compiled yet.** Entities, jobs, views and contracts (the runner hosts them, 7.6),
durable `!` steps, `with` in tests, foreign code (`@library`), and the clock and fakes of tests.
A program that declares one gets an error that names what it declares and says `eye run` and
`eye test` run it. `parallelMap` runs its function on one thread, in order.

**Tests.** `cgen/cgen_test.go` builds the tests of every program in `testdata/programs` and
`cgen/testdata` that `eye build` compiles (today core, traits, the lexer, a program of failing
tests, one of in-place updates, one of slow updates, numbers, `layouts`, which makes, takes
apart, compares, shows, collects and updates a value of each layout, and `times`, `hosts`,
`shows` and `recursion` for 7.9's time methods, `random` and `fs.lines`, impls of `Show` inside
other values and recursion too deep), at both `-O0 -g` and `-O2`, runs them,
and checks the output is `eye test`'s line for line (durations aside); a program joins as soon
as it compiles. It also builds core for all five targets and runs a `main`. `cgen/updates_test.go`
times the update loops that copied natively until 7.5 (an append through a helper, a map insert
through a `let`, a record updated from its own list) and a list taken apart by slices, which
copied until 7.13, at two sizes, and fails if one grows quadratically again. Valgrind can't check for
leaks, since it doesn't see the heap of a static musl binary; instead `cgen/alloc_test.go` runs
the test binaries of core, traits, the lexer and cgen's programs, and a `main`, with `EYE_ALLOCS`,
and checks that every object is freed by exit except those a failing test left.

### Where the runtime lives

Compiled code needs a runtime, and today the runtime is the interpreter. It has two parts with
different needs.

- **The core runtime** is what every program needs: allocation, reference counts, `Int` with big
  integers, strings, lists, maps and sets, hashing, formatting, and the `fs`, `process` and
  `stdio` primitives. The self-hosted compiler needs only this.
- **The durable runner** is what programs with entities and jobs need: the scheduler, the
  journal, the Postgres store, the HTTP client, JSON, TLS, time zones, model calls and approvals.
  Today it is most of `interp/`, about 12,700 of its lines, tested by every program in the
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

**Decided (2026-10-12): neither as written, but a third way, B′.** The second way crashes at
start-up as a static musl binary (musl doesn't pass `argc` and `argv` to constructors, which a Go
`c-archive` reads), and the first costs about 300 ns a call and a dynamic runner. In B′ the Go
runner is the program's `main`, and `eye build` links the program's C into it through cgo, with
the pinned zig as the C compiler and a pinned Go toolchain it fetches as it fetches zig: one
static binary, about 35 ns a call, `eye` itself still free of cgo, and programs with no entities
or jobs built with zig alone. The measurements are in the
[native compilation plan](reviews/native-plan-2026-10-12.md), section 5. All three keep the
points below.

- The runner's code is reused, not rewritten, and its tests keep covering it.
- Go's garbage collector stays in the process, but it manages only the runner's own buffers.
  Program values are reference-counted in the C heap, so the "no GC pauses" goal holds for them.
- Go programs that embed Eyelang (`@host`, issue #42) get compiled code through the same
  interface.
- Parts of the runner move into Eyelang later, as the self-hosting plan moves the compiler.

**Decided (2026-10-12): yes, for v0.4.** A runner in C or Eyelang from the start would mean
writing a Postgres client, an HTTP client with TLS, and time-zone handling before the first
compiled job runs. The runner moves into Eyelang after v0.4 (S.2), which is when a job program
can have no Go in it and beat Go on memory.

### Hosting compiled programs (7.6)

**Built so far (the groundwork of 7.6).** `eye build --runner` builds a program as B′, for
`linux/amd64`, `linux/arm64`, `darwin/amd64` and `darwin/arm64`. Without the flag nothing
changes. It goes like this (`cgen/runner.go`):

1. The program's C and the runtime's are compiled by zig into a static library,
   `libeyeprogram.a`, with the program's `main` renamed `eye_main` (`-Dmain=eye_main`). The
   objects are cached as for any build.
2. `eye` writes a Go module into a temporary directory: a `main` package, and the runner's Go
   source as the module `eyelang`, which `main` requires and replaces with that copy.
3. The pinned Go (`internal/gotool`, which moves with `go.mod`'s `go` line) builds it with
   `CGO_ENABLED=1` and `CC="zig cc -target …"`, linked externally. Linux binaries are static,
   against musl (`-extldflags -static`). A macOS binary links `libSystem` and `libresolv`, as
   every Go program on a Mac does. zig's macOS SDK doesn't describe `libresolv`, so the build
   writes a small text stub of it. zig signs arm64 binaries, which macOS needs.
4. The runner's `main` calls `eye_main(argc, argv)` through cgo, then ends the process with C's
   `exit`. That flushes the program's buffered output and runs its `atexit` handlers, which Go's
   `os.Exit` would skip.

A first build for a target takes about 40 s, mostly compiling Go's standard library through
cgo. After that it takes about 4 s. `go test ./cgen -run TestRunner` builds a program's `main`
and its tests this way, and cross-builds for the other targets, checking each file's headers.

**How the runner's Go source reaches a user's machine: inside `eye`.** `cgen` embeds the source
(`//go:embed hosted/*.go`), and the build writes it out as the module `eyelang` beside the
program's `main`, with `GOPROXY=off`, `GOTOOLCHAIN=local`, `GOWORK=off` and `GOENV=off`. A build
fetches Go and zig, each once, and nothing else: no module proxy, no `GOPATH`, no settings from
the user's `go env`. Today the embedded runner is the package `cgen/hosted`, which only calls
`eye_main`. It is behind the build tag `eyehosted`, so `eye`'s own build, which has no cgo,
skips it.

When 7.6 hosts entities and jobs, the runner's packages (`interp` and the stores) and their
dependencies are embedded the same way. The dependencies are vendored (`go mod vendor`) when
`eye` is built, so the build runs with `-mod=vendor`, offline. Fetching the runner's module from
a proxy instead would make a build depend on the network and on a published version matching
`eye`'s. Embedding ties the runner a program links to the `eye` that compiled it, which is what
the interface below needs.

**The interface** is between the runner (Go) and a compiled program (C), declared in
[`cgen/hosted/eye_host.h`](https://github.com/ruxxzebre/eyelang/blob/dev/cgen/hosted/eye_host.h).
Values cross it as opaque handles, `eye_val`, so 7.13's layouts can change what a handle is
without changing a signature.

- **What the program declares:** `eye_program()` gives the tables the runner needs, by index:
  - each entity, with the types of its identity, its state and its snapshot, and its methods'
    parameter and result types;
  - each job, with its parameters' types, its frame's type and its result's type;
  - the tests, the events with their types, and the host functions with theirs;
  - whether it runs `main`, with the types of `main`'s arguments and result, and the types of
    a key and a run, for starts.

  A declaration is named as the runner names
  it: one of the program's own module by its name (`Counter`), one of an imported module with
  the module's path in front (`lab.notify.Message`). `EYE_HOST_VERSION` changes with the
  interface, and the runner refuses a program compiled for another version.
- **Entry points**, compiled code the runner calls:
  - `eye_main`, built since the groundwork;
  - `eye_run_main` and `eye_run_test`;
  - `eye_call_method`, which runs a method on a state, given and replaced through a pointer;
  - `eye_job_frame`, which makes a job's first frame from its arguments;
  - `eye_step_job`, which runs a job from its frame to its next `!` and returns `EYE_WAITING`
    with an `eye_step` (sleep, an entity call, or a host function, with the step's arguments
    and the type of what it gives back), or to its end with `EYE_OK`;
  - `eye_encode`, `eye_decode`, `eye_retain`, `eye_release` and `eye_fault`.
- **Callbacks**, the runner exported from Go: `eye_go_entity_call`, `eye_go_entity_read`,
  `eye_go_start`, `eye_go_publish` and `eye_go_host`, for what finishes inside a transaction.
  Their argument arrays aren't `const`, since Go, which defines them, can't say so; the runner
  only reads them.

The rules that go with it:

- **Ownership.** Arguments are borrowed for the call. Results and replaced states are owned by
  the receiver, which releases them with `eye_release`. A step's arguments belong to the frame
  until the next call on it. On `EYE_FAULT`, a state or frame passed by pointer is the one
  given. The runner keeps no handle between calls: it keeps encodings.
- **The Go side works with encodings.** The runner never looks inside a handle. To read one, as
  a contract operation's arguments or a store write, it calls `eye_encode`, in the shape the
  interpreter uses. To make one, it calls `eye_decode`. 7.10 counted those crossings for a
  digest morning: 114 encodes and 10 decodes, about 4 µs at B′'s 35 ns.
- **Jobs return at each `!`.** A step is never a callback that blocks inside C. So a waiting
  run holds no C stack and no thread, only its encoded frame, as section 4 needs. Entity calls,
  reads, starts and publishing are callbacks, because they finish within the transaction.
- **One thread at a time per `eye_rt`.** Compiled code runs on the goroutine that called it,
  and its callbacks run there too. The runner already gives each transaction one goroutine.
- **Faults.** A fault in compiled code returns `EYE_FAULT` up through every frame. The runner
  rolls the transaction back, as it does for an interpreted fault. A fault inside a callback
  returns `EYE_FAULT` to the compiled caller, which returns it on.

**The runner's side (built).** The interpreter's runtime is the runner, unchanged: the store
(journal or Postgres), the scheduler, the R.7 handles, steps and their replay. A compiled
program plugs in through one seam, `interp.Native` (`interp/native.go`), set with
`RunOptions.Native`:

- `HostsMain`, `HostsTest`, `HostsMethod` and `HostsJob` say what the compiled program runs.
  What it doesn't, the interpreter runs, so a program can be part compiled.
- **`main` and the tests** run where the interpreter would run them (`runMain`, `runTest`), so
  `eye run`'s order and each test's fresh store and clock stay as they are.
- **Callbacks.** Every call into compiled code is given the runner as an `interp.NativeHost`:
  entity calls and reads, starts, publishing, and calls of what the runner provides (a built-in
  such as `print`, `expect` or `log.info`, or a host function such as `env.get`). They go
  through the runtime's own code (`callEntity`, `snapshot`, `start`, `publish`), so an entity
  call from a method joins the method's transaction. A Go panic must never unwind through C
  frames, and the runtime stops with one for a fault, a failed `expect`, a park or a stop. So
  each callback keeps what stopped it and returns `EYE_FAULT`, and the Go side panics with it
  again once the compiled code has returned. A failed `expect` in a compiled test reports the
  test's place, since compiled code has no expression to show.
- **An entity method** runs where the interpreter would run it, inside the transaction
  (`callIn`). Its arguments are ordered by the method's parameters, with defaults filled in, and
  encoded. The row's state goes in encoded, and the new state comes back encoded.
- **A job** runs step by step (`runNativeJob`). Each time it stops at a `!`, the run's record
  keeps the step with its frame (`jobRun.Native`) as its checkpoint. The steps since the last
  checkpoint move to the run's trail for `eye runs`, as an interpreted checkpoint moves them. The
  step is then taken by the interpreter's own code (`sleep`, `entityStep`, or a host function,
  recorded as `durable` records one), or replayed after a restart, and what it gave resumes the
  job. The step's commit saves the new
  checkpoint, so a crash before it leaves the run where it was, and running the job again from
  there gives the same step.
- With no compiled program, the interpreter's path is unchanged: one nil check per method call
  and per job turn.

`cgen/hosted/runner` (behind the tag `eyehosted`, with cgo) implements `interp.Native` over
the entry points and exports the callbacks. Its `Main` runs a program as `eye run` does, or its
tests with `--test`, as `eye test` does.

**The program travels in the binary as its source.** `cgen.ProgramSources` collects the `.eye`
files of the program's own modules, and its `eye.toml`, for the build to write into the
generated main. The runner checks them from memory when it starts. The declarations it
schedules, and whatever the compiled program leaves to the interpreter, come from that check,
and no directory is needed at run time. Carrying the checked IR instead would need an encoder
for the IR's pointer graph. Checking costs about 50 ms for the whole lab (2,200 lines, the
digest among them, process start included), once per start, which doesn't justify one.

`go test ./cgen -run TestHostedProgram` builds a hand-written C stand-in
(`cgen/hosted/testdata/counter`) into the runner through `eye build --runner`'s link step. In it:
- `main` reads an entity, starts a job and prints, all in C.
- `Counter.add` publishes an event and calls `Total.add` in its transaction.
- The job sleeps, reads `env.get!` as a host step, then counts.

The test runs it on the journal, and on Postgres when `EYE_TEST_POSTGRES` is set. It kills the
process while the job sleeps and starts it again: the job resumes, takes its steps and
finishes. The store holds the same bytes, states and events, the interpreter would write. Its
tests run with `--test`, a failing `expect` included.

## 2. Values and memory

### Values laid out by type (7.13)

The back end as first built (section 1, "The C back end as built") holds every value in one
64-bit word, and everything that isn't a scalar or a small `Int` is a separate heap object. The
benchmarks showed that this, not counting, is the larger cost
([benchmark review](reviews/benchmarks-review-2026-10-12.md), sections 3 and 5): a record, an
`Option`, an enum and every element of a `List(Token)` are objects. **Decided 2026-10-12** (the
owner accepted the review): the low IR, which is monomorphic and so knows every type, chooses
each type's representation, and the C back end prints it. Values without aliasing (SPECS §7) are
what permit this: nothing can notice that a record was copied into registers, that a list holds
its records inline, or that two strings share bytes. Go can't keep a struct it handed out a
pointer to in a register; Eyelang never hands one out.

The choice is data in the low IR, `lir.Layouts` (`lir/layout.go`): for each type, how its values
are held (`Repr`), their size and alignment, whether they are counted, each field's offset, and
how a variant type says which variant a value is. `TestLayouts` checks it for each kind of type.

| Type | Held as | Bytes | Counted |
|---|---|---:|---|
| `()`, `Never` | nothing | 0 | no |
| `Bool`, `Byte` | `bool`, `uint8_t` | 1 | no |
| `Char`, `Int32`, `UInt32`, `Time` | 32-bit integers | 4 | no |
| `Int64`, `UInt64`, `Float`, `Duration`, `Size`, `Date` | 64-bit numbers | 8 | no |
| `Int` | a tagged word: a small integer `n` as `(n << 1) \| 1`, or a pointer to a big integer | 8 | yes (only a big one has a count) |
| An enum (variants without fields) | its variant's number, `uint8_t` (`uint16_t` past 256 variants) | 1 | no |
| A record, tuple or `Range` of at most 64 bytes | a C struct, by value | ≤ 64 | when a field is |
| A variant type with fields, at most 64 bytes | a C struct: a niche or a tag, then the variants' fields over each other | ≤ 64 | when a field is |
| A larger or recursive record or variant type | a box: a pointer to a counted object; a variant without fields is an immediate word | 8 | yes |
| `String`, `Bytes`, `Key`, `Zone` | a view, `{p, len, own}` | 24 | yes |
| `List`, `Set`, `Map` | a pointer to a counted buffer whose elements are laid out inline | 8 | yes |
| `Iter`, a closure, a `dyn` value | a pointer to a counted object | 8 | yes |
| `Instant` | `{ns, zone}`, the zone a static table | 16 | no |
| A newtype (`Offset`), `NonZero(Int)` | its field | | as its field |

- **Inline up to 64 bytes** (the owner approved the cut-off and the three-word string views on 2026-10-12). A record or tuple is a C struct passed and returned by value, its
  fields ordered by alignment so that little is lost to padding. Up to 64 bytes it stays inline,
  past that it is boxed. Copying 64 bytes is a few moves, less than allocating and freeing an
  object; the C ABI passes any struct over 16 bytes in memory, so the cost is flat between 3 and 8
  words; past 8, the copies, and the dups of many counted fields, cost more than a box's one
  count. The benchmarks' records fit: records' `Particle` is 40 bytes (five `Int`s), the lexer's
  `Token` 24 and its `Lexer` 48; a record of three strings (72) is boxed.
- **Recursive types are boxed**, decided on the declarations: a type that holds itself inline,
  through its fields, a tuple or another declared type's arguments (not through a `List`, `Map`,
  `Set`, `Iter`, function or `dyn`, which are pointers already). In a boxed variant type a variant
  without fields is the immediate word `(n << 1) | 1`, so it allocates nothing, and one with
  fields is an object whose header holds `n`: `Tree`'s `Leaf` is a word and its `Node` a 24-byte
  object (header and two children), the layout of the review's hand-written experiment that beat
  Go.
- **`Option` and other variant types without a box.** A variant type with one variant with
  fields and others without uses a *niche* when it can: a value of the payload no real value
  uses. A pointer is never below 4096, an `Int`'s or a box's word is never even and below 4096,
  an enum's number never past its last variant, a `Bool` never past 1. So `Option(Int)` is 8
  bytes with `None` as 0, `Option(List(a))` a pointer that `None` leaves null, `Option(String)` a
  view whose `p` is null, `Option(Kind)` one byte, `Option(Option(Int))` 8 bytes still. Without a
  niche (`Option(Float)`) or with two variants with fields (`Result`), a tag byte comes first and
  the variants' fields lie over each other after it. `xs[i] ?? 0` and `m[k] ?? 0` allocate
  nothing.
- **Zero is empty.** All-zero bits are a value whose drop does nothing: a null pointer, an `Int`
  word of 0, a view without an owner. A field taken out of a record (below), an argument left out
  for its default, and a fresh variable rely on it.
- **Lists of fixed-size values are arrays of them.** A `List(Token)` is one buffer of 24-byte
  tokens, a `List(Byte)` one of bytes, a `Map(String, Int)` arrays of views and words; reading an
  element copies it out (and dups its counted fields). A list is one word, a pointer to a list
  object; a slice of a list (`xs[2..5]`) is a list object whose items point into its list's
  buffer and which holds that list (`owner`), so it costs one small object whatever its length,
  as SPECS section 5 says. A slice changes in place never: `+=` or `xs[i] = v` on one copies
  what it covers first. (The first design copied list slices; the spec promised sharing, and the
  `owner` field costs nothing on lists that aren't slices.)
- **Strings and bytes are views** `{p, len, own}`: where the bytes start, how many, and the
  counted buffer that holds them (null for a literal, which is static). A slice, `split`,
  `lines`, `words` and `trim` share the buffer; `s += t` appends in place when the view holds the
  buffer's only reference and ends where the buffer's bytes end. A view is three words, not
  Go's two, because there are no pointers into the middle of an object to find its count from,
  and 32-bit offsets would cap strings at 4 GB.
- **`Int` stays a tagged word.** fib with Eyelang's `Int` rule runs at Go's speed (the review,
  section 2); the tag costs almost nothing beyond the overflow check.
- **Heap objects** start with a 16-byte header (8 in the first design; the descriptor pointer
  stays while the runtime's generic code walks objects): a 32-bit count (a static object's is negative and
  never changes, and a count that would overflow becomes static, so the object lives on) and 32
  bits the object's kind uses, a boxed variant's number for one. A box holds no descriptor: its
  drop is generated for its type. The buffers of lists, maps and sets, closures and `dyn` values
  keep a pointer to their type's operations, since the runtime walks them.
- **Counted values** are those that hold an `Int`, a string, an object or a box, directly or in
  an inline field: only they get dups and drops (below). Scalars, enums, `Instant`, and records,
  tuples and variants of those are copied as bits and never counted.

How the generic runtime functions work over these layouts, given that the low IR is monomorphic:

- **Specialised per type, from the low IR, where it matters.** For each type the program uses,
  the back end writes its drop and its copy (dup), and its `eq`, `cmp` and `hash`, as C
  functions, the way it already specialises generic functions. `sort` is a pdqsort written once
  as a template in the runtime and instantiated per element type, comparing inline; a map's or
  set's probe is instantiated per key type, hashing and comparing inline. These are the paths the
  profiles named (`sort` through descriptors was 42% of lists; maps hash and compare through
  `ey_hash` and `ey_eq`). Saving and loading values for the runner (7.6) is generated per type the
  same way.
- **Descriptors where it doesn't.** A descriptor becomes a table of a type's operations: size,
  alignment, drop, copy, `eq`, `cmp`, `hash` and `show`. The runtime's generic code uses it where
  speed matters less: showing values as the interpreter does, lazy `Iter` pipelines, the
  collection functions that take a closure, and buffers it must free.
- **Closures** have one entry the runtime can call whatever the types,
  `void (*)(ey_fn *self, void *result, const void *const *args)`, beside the typed function a
  direct call uses. Arguments are borrowed through it (below).

**Decided 2026-10-12:** small records by value (the question this section used to ask): up to 64
bytes, as above.

### Reference counting in the low IR (7.5)

The language was designed for this. SPECS §7 rules out aliasing, references and cycles, so
counting is enough to reclaim everything, and a value with one reference can be changed in place
without anyone seeing it.

The scheme is **Perceus**, as Koka does (Reinking, Xie, de Moura and Leijen, *Perceus: Garbage
Free Reference Counting with Reuse*, PLDI 2021), with Lean 4's borrowed parameters (Ullrich and
de Moura, *Counting Immutable Beans*, IFL 2019). **Decided 2026-10-12:** it is a pass over the low
IR, `lir.Count` (`lir/count.go`), not decisions taken while C is printed, so that another back end
gets it too; reuse of a matched cell and freeing a run's region on a fault are 7.12. Only counted
types (above) take part.

- **Owned and borrowed values.** A value an instruction makes (a call's result, a record, a
  list) owns one reference. It is dropped right after its last use (`drop`), or passed on by that
  use, which takes it; a use that takes it while it is still needed takes a copy (`dup`). A field
  read (`Field`, `TupleGet`, `Payload`), a module-level `let`, a borrowed parameter, and a `let`
  of any of these or of a variable that stays live are *borrowed*: valid while what they borrow
  from is, which the pass keeps alive until their last use, and never dropped. A borrowed value
  that is kept is dup'ed where it is kept. Constants are static and never counted.
- **Borrowed parameters**, inferred over the whole program at once as Lean does: a parameter
  starts borrowed and becomes owned when the body keeps it (in a record, a list, a closure),
  returns it, passes it to an owned parameter or would update it in place; the rounds repeat
  until nothing changes. A parameter the function sets (its default) is owned, a `mut` one is
  the caller's variable, and a lambda's parameters are borrowed, since a closure is called
  through its value. `Func.Modes` records the result, and `eye check --lir` prints `owned` and
  `mut` after a parameter's type.
- **Liveness**, by the usual backward analysis over the blocks, where a use of a borrowed value
  is also a use of what it borrows from. A value dead on one way out of a branch and live on the
  other is dropped on the way in, in a block of its own when the target has other ways in. A
  `mut` parameter is live where the function returns. Code that ends in a fault leaks what is
  live, until runs get regions (7.12).
- **In place when unique, whatever the shape around it.** An update whose operand is owned and
  dead after it takes the operand and is marked `inplace`: a built-in's `add`, `insert`,
  `remove` or `concat`, `+` on a `String`, `List` or `Set`, and `{ r with … }`. At run time it
  changes the value in place when its count is one and copies it otherwise; this is the copy on
  write SPECS §7 promises. Since the decision comes from liveness, it doesn't matter whether the
  last use is an assignment, a call (`xs = push(xs, i)`, where `push` owns its list) or a `let`
  (`let next = m.insert(i, i); m = next`).
- **A field taken from a record about to be rebuilt.** In `p = { p with items: p.items.add(i) }`
  and `p.items = p.items.add(i)`, `p` still holds the old list while the new one is made, so the
  list's count is two and `add` would copy. When the record is rebuilt with a new value for the
  field later in the block (an update in place, or a `SetPlace` of the field) and nothing reads
  the field in between, the read becomes `take`: the field is moved out and left empty (zero)
  when the record is inline or its only reference, and dup'ed otherwise. `add` is then in place,
  and so is the update, which drops the empty field to nothing.
- **A borrowed value whose root changes** while it is still used (the variable set again, a
  `SetPlace` of an overlapping path, a `mut` argument) is dup'ed where it is read instead.
- **Built-ins and the runtime borrow** every argument but an update's operand, and give an owned
  result; what they keep (an element added to a list) they dup themselves. Closure and `dyn`
  calls borrow their arguments too; a function with owned parameters called through a closure
  or a `dyn` table is called through a wrapper that dups them.
- **`i = i + 1`**, an instruction that sets a variable it reads, goes through a new value, so
  that the old one is dropped in between.

`eye check --lir` prints the result. The update loop of `cgen/testdata/updates`:

```
b2:
  %10.i: Int = borrow %8
  %11: Int = take %5.p.count
  %12: Int = 1
  %13: Int = op + %11, %12
  drop %11
  %14: List(Int) = take %5.p.items
  %15: List(Int) = inplace builtin add(%14, %10.i)
  %16: Bag = inplace {%5.p with count: %13, items: %15}
  set %5.p = %16
```

`lir.CheckCounts` checks every program in the repository after counting
(`TestEveryProgramLowers`): it follows each path with the number of references each value holds,
and reports a value read, taken or dropped without one, a variable set while it still holds one,
paths that meet with different counts, and a return that leaves anything held. On 2026-10-12,
taking out any one of the 11,027 dups and drops the pass wrote into the repository's programs made
it report a problem, except 4 in code that ends in a fault, which may leak; `TestCheckCountsCatches`
keeps doing so for the golden program. `testdata/ir/counting` holds the
golden output of each rule. The C back end prints `Dup` and `Drop` as the type's dup and drop,
and an instruction marked `inplace` as the runtime's in-place form, which takes its operand.

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
   and `poll!(every: 1 min, within: 6 h, fn { miniflux.ready!() })` take functions that wait.
   A lambda's own frame names its code statically, and its captures are live variables. A
   function value a frame keeps across a `!`, such as a callback a waiting function was given,
   is saved the same way: the name of its code and its captures. The checker lets one be kept
   only when it sees both (SPECS §15); the interpreter's checkpoints save it so too, naming a
   lambda by its module and a hash of its typed IR.
4. **Waiting built-ins are states of the runtime:** a timer for `sleep!` and `sleepUntil!`, an
   event sequence number for `next!`, a request for `approval!`, the outcome for `poll!`, a
   grant for `reserve!`, a child run for `join!`.

What makes this possible is already checked: only values with `Save` may be live across a `!`
in a job or a waiting function, parameters included (`checkSave` in `check/effects.go`), so
every frame can be encoded; a function is saved as its code and its captures (point 3). One hole
matters here: a value
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
4. Insert reference counts (`lir.Count`, section 2) and, with 7.12, reuse; the layout of each
   type (`lir.Layouts`) decides which values are counted.
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
- Does `parallelMap` use several cores? (section 2)
- Is `List(Logger)` to `List(dyn Handler)` an implicit copy? (section 3)
- Code size of full specialisation, measured on the self-hosted compiler. (section 3)

Decided 2026-10-11:

- C is the back end, and `eye` ships `zig` to compile it. (section 1)
- State machines in the interpreter in v0.3. (section 4)
- "Jobs as short-lived processes" means the runner; a waiting run is a record. (section 4, and
  Proposals)

Decided 2026-10-12:

- Values are laid out by type, chosen in the low IR; small records by value, up to 64 bytes.
  (section 2, 7.13)
- Reference counting is a pass over the low IR. (section 2, 7.5)
