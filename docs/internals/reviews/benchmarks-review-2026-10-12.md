# What the benchmarks say, 2026-10-12

The owner asked for native code that is faster than Go and uses less memory, and for a real
program to measure it with. 7.7 built the suite ([Benchmarks against Go](../benchmarks.md)) and
S.1 wrote Eyelang's lexer in Eyelang, which is the suite's ninth benchmark. This review reads the
numbers and the experience of writing the lexer, and asks one question of each gap: **is it a
cost of how we generate code today, or a cost of the language's rules?** Only the second kind
argues for changing syntax, grammar, the standard library or the design philosophy.

Section 6 gives a recommendation on each of the native plan's open decisions. **The owner
accepted them on 2026-10-12**, with the cheap fixes (7.14), values laid out by type (7.13), B′
hosting and the lexer's five language changes (G.20); ROADMAP has the tasks.

## The short version

- **No language rule stands between Eyelang and Go's speed.** Every gap the profiles show comes
  from the C back end and its runtime. Two hand-written C experiments show it: fib with
  Eyelang's own `Int` rule (a tagged small integer, checked for overflow) runs as fast as Go, and
  binary trees with reference counting, a static `Leaf` and a simple free list run faster than
  Go.
- **The root cause is one assumption in the back end:** every value is one machine word, and
  everything that isn't a small number is a separate heap object, decided while printing C. The
  low IR is already monomorphic, so it knows every type. Values should be laid out by type:
  enums as small integers, small records as C structs, `Option` without a box, lists of records
  as arrays of structs, slices that share bytes.
- **The plan's order needs one change.** The plan put reference counting (7.5a) first. That still
  matters, but it can't reach Go alone: deciding which values are counted depends on the layout,
  so the layout should come first or together. Three cheap fixes come before both: compiling the
  runtime with the program (20–25% on fib, measured), a size-class allocator instead of musl's
  `malloc`, and static values for variants without fields.
- **The lexer asked for five small language changes**, all in line with the pillars: hex
  literals, string patterns over `Bytes`, `xs[i] = v` on a variable, `fs.readBytes`, and a
  variant's number. It also found one checker bug, since fixed. None is fundamental.
- **The design philosophy holds.** Values without aliasing are what *permit* the fast layout
  (Go can't put a shared object in a register; Eyelang can, since nothing shares it). Reference
  counting is not the bottleneck. `Int` with arbitrary precision costs about Go's speed on
  integer code, not more.

## 1. The numbers

Recorded on 2026-10-12 (4-core Xeon, Go 1.27.2, zig 0.17.0; fastest of 5 runs, peak RSS).

| Benchmark | Native ms | Go ms | Time × Go | Native MB | Go MB | Memory × Go | Interpreter ms | Interpreter MB |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| fib(37) | 455 | 96 | 4.74× | 0.16 | 2.03 | 0.08× | 7,382 | 18.4 |
| mandelbrot, 1600² | 278 | 118 | 2.34× | 0.16 | 1.97 | 0.08× | 26,318 | 23.5 |
| trees, depth 16 | 1,051 | 468 | 2.25× | 80.2 | 13.0 | 6.17× | 7,749 | 50.3 |
| records, 30M `with` | 1,346 | 284 | 4.75× | 0.16 | 1.97 | 0.08× | 9,761 | 21.1 |
| strings, 1M parts | 378 | 188 | 2.01× | 83.0 | 43.4 | 1.91× | 3,323 | 733.7 |
| lists, 1M | 179 | 162 | 1.10× | 42.3 | 46.5 | 0.91× | 3,312 | 131.7 |
| maps, 5M lookups | 548 | 97 | 5.67× | 1.18 | 2.53 | 0.47× | 16,824 | 31.9 |
| logs, 200k lines | 383 | 263 | 1.46× | 54.7 | 59.9 | 0.91× | 1,671 | 81.8 |
| lexer, 7.5 MB of Eyelang | 1,786 | 388 | 4.61× | 478.0 | 191.8 | 2.49× | 28,247 | 1,798.8 |

Native code takes 1.1–5.7× Go's time, 2.8× on the geometric mean, and uses less memory in six of
nine (0.57× on the geometric mean). Four of those six win only on start-up: a static binary starts
at 0.16 MB and a Go program at 2 MB. That floor is real and useful, since a program that mostly
waits is mostly floor, but it isn't evidence about how we hold data. Trees, strings and the lexer
hold data and use more memory than Go.

The interpreter takes 4–95× native code's time. Its memory starts at about 18 MB, the Go runtime
and the checked program, and grows to 734 MB on strings and 1.8 GB on the lexer, 3.8× native
code's and 9.4× Go's there; on trees it holds less than native code does today (50 MB against 80),
since Go's collector frees each tree soon after it is checked. After R.6 it is in CPython's range, and the lab,
which is bound by input and output, runs its 31 tests in under 200 ms. The interpreter is not
what limits the lab today.

## 2. Two experiments: what the rules cost

The profiles (callgrind on the `-O2` binaries, in the native plan's
[Against Go](./native-plan-2026-10-12.md#against-go)) say where the time goes. They don't say how
far a better back end could get. Two small C programs, compiled with the pinned zig at `-O2` for
static musl as `eye build` does, answer that for the two rules most likely to cost something.

**`Int` has arbitrary precision.** fib(37), written three ways:

| | ms |
|---|---:|
| Plain `int64`, as Go does it | 54–66 |
| `int64` with an overflow check on `+` | 90–115 |
| Eyelang's rule: a tagged word, small values inline, an overflow branch to a slow path | 93–118 |
| Go | 96 |
| `eye build` today | 455 |

The rule costs about 1.6–1.9× against plain C, because the overflow check stops the C compiler's
rearranging of the recursion, and lands at Go's speed. The tag itself costs almost nothing beyond
the check. So almost all of today's 4.7× is the back end: a `dup`/`drop` on every move, `<` as a
call into another file, every local dropped at return. Compiling the emitted C as one unit with
link-time optimisation alone takes fib from about 600 to about 450 ms on the same machine.

**Reference counting instead of a tracing collector.** The trees benchmark in C with reference
counts: 24-byte nodes (count and two children), `Leaf` a static value, each tree freed as soon as
it is checked:

| | ms | MB |
|---|---:|---:|
| musl's `malloc` and `free` | 461–573 | 16 |
| A free list for the node size | 313–425 | 16 |
| Go | 468 | 13 |
| `eye build` today | 1,051 | 80 |

Reference counting with a size-class allocator beats Go's collector on its best benchmark. What
costs today is a fresh object for every `Leaf`, 40-byte nodes, trees that live until their
variable is reassigned or the function returns, and musl's slow `malloc`.

## 3. Back-end costs, and what removes each

These are costs of the current code generation, not of the language. Each line says what removes
it, roughly by payoff.

| Cost | Where it shows | Remedy |
|---|---|---|
| Every value is one word; records, variants and `Option` are heap objects | records 4.8×, maps 5.7×, lexer 4.6× and 2.5× memory, trees | **Values laid out by type** (new task, section 5): field-less enums as integers, small records as C structs passed by value, `Option(T)` without a box, `List(Record)` as an array of structs, `Bytes` and `String` slices that share storage |
| A `dup`/`drop` on every move, every local dropped at return | fib 60% of instructions, trees' 80 MB | 7.5a: counting in the low IR, drop after last use, borrowed parameters |
| `{ p with … }` copies because `p` is still live | records | 7.5a's "in place when unique and dead" |
| The runtime is compiled file by file, so nothing inlines (`ey_int_cmp`, `divmod`) | fib, mandelbrot, maps | One translation unit, or LTO: 20–25% on fib, measured |
| musl's `malloc` | trees, lexer (allocation is about a third of the lexer's instructions) | A size-class allocator in the runtime: 30% on trees, measured |
| A field-less variant allocates each time | trees, lexer (each token's kind) | A static value per variant (until enums become integers) |
| `"{n}"` formats through `snprintf`; `split` copies parts | strings | Integer formatting in the runtime; substrings that share bytes |
| `sort`, hashing and equality go through type descriptors | lists 42%, maps | Specialise them per type, as the low IR already specialises functions |
| `xs += [x]` builds a one-element list | lexer | Lower it to an append |
| Passing a named function builds a closure | lexer | A static closure for a function without captures |
| `/` and `%` by a constant call `divmod` | lists, maps, the lexer's checksum | Constant divisors inline (the C compiler does the rest) |

None of these needs a language change. Most are small; the layout is the large one.

## 4. Language costs, and what the lexer asked for

### Rules that cost something

- **`Int` with arbitrary precision.** About Go's speed on integer code (section 2), so it doesn't
  stop the bar on most programs. A tight integer loop can still lose to Go by the overflow check.
  If that matters later, the low IR can prove some values small (a loop counter bounded by a
  list's size) and drop the check. The fixed-width types (G.1) already exist for code that wants
  wrapping. **No change recommended.**
- **Indexing returns `Option`.** Free once `Option` has no box: a branch, like Go's bounds check.
  **No change.**
- **No element assignment.** `xs[i] = v` is an error, so code that changes one element rebuilds
  the list, which is linear where Go is constant. This one is a rule, and it costs real
  algorithms. See below.

### What writing the lexer asked for

S.1's lexer is about 400 lines of Eyelang against 557 of Go, gives the same 24,959 tokens on the
conformance suite and corpus, and reads well: byte patterns such as
`['.', '.', '=', ..] => Some((DotDotEq, 3))` say what Go's switch statements spell out. What got in
the way is open question 43. Recommendations:

| Ask | Kind | Recommendation |
|---|---|---|
| `xs[i] = v` on a `var` or `mut` list (and `m[k] = v` on a map) | Rule, small syntax | **Yes.** It is local, visible mutation, as `p.x = v` already is for records, and with uniqueness it is in place. Removes the linear rebuild |
| Hex (and binary) literals: `0xC2..=0xDF` | Grammar | **Yes.** The notation is universal; byte-level code is unreadable in decimal |
| A string literal as a pattern over `Bytes`: `"continue"` instead of eight characters | Typing rule, no new syntax | **Yes.** The literal already exists; a pattern over `Bytes` accepts it |
| `fs.readBytes`, and `Bytes` from a `List(Byte)` | Library | **Yes.** Bytes that aren't text must be readable and testable |
| A variant's number (Go's `int(kind)`) | Library (a derived method) | **Yes, as a derived method on enums without fields**, not syntax. Name to choose |
| A variant named like a built-in type (`Int`) takes the type's name over in its module | Checker bug | **Fixed** in 104fa0d: a variant isn't a type |
| `if let`, escaping `{` in test sources | Ergonomics | No change now; `match` covers `if let`, and raw strings are open question territory |

### The design philosophy, checked

- **Pillar 1, values without aliasing.** The benchmarks strengthen it. Because no value is shared,
  the compiler may copy a small record into registers, store a list of records inline and share a
  string's bytes between slices, without anything able to notice. These are the layout
  changes section 3 needs. Go can't do the first for a pointer it has handed out.
- **Performance by construction: reference counting, in place, no collector.** Holds, with an
  allocator to match (section 2). The cost model a reader sees doesn't change.
- **A static binary on musl.** Holds, but musl's `malloc` and `printf` are slow, so the runtime
  brings its own allocator and number formatting rather than relying on the C library.
- **Go as the bar.** A fair bar for time. For memory it is fair on data the program holds; the
  start-up floor favours Eyelang, and the bar for a program with entities or jobs under B′
  includes the Go runner, as the plan says.

## 5. Assumptions that turned out wrong

1. **"Perceus will make it fast."** compilation.md and the plan treat precise reference counting
   (7.5) as the main performance step. It is necessary, but the larger cost is the uniform
   one-word representation. With it, even perfect counting leaves records, `Option`, enums and
   lists of records on the heap.
2. **"Counting first, layout later."** The low IR decides which values are counted from their
   types. Adding `Dup`/`Drop` before the layout is decided means doing it twice.
3. **"musl is fine."** It is fine for portability and a static file, not for allocation-heavy
   code.
4. **"Each runtime file compiled once and cached."** It makes rebuilds fast and stops inlining
   across files. LTO or one unit costs build time once per program and gains 20–25%.
5. **"The interpreter is the bottleneck."** After R.6 it isn't, for the lab. Native code matters
   for computation-heavy programs and for the self-hosted compiler, and its start-up floor
   (0.16 MB, 0.3 ms) matters for many small waiting programs.

**Proposed new task: 7.13, values laid out by type.** The C back end and the low IR choose each
type's representation: field-less enums as integers, small records as structs by value,
`Option(T)` without a box, lists of fixed-size values as arrays, string and byte slices sharing
storage. Done when records, maps and the lexer are within Go's time and the lexer's memory is
below Go's, with every cross-check passing.

## 6. The native plan's decisions, in the light of the numbers

| # | Decision | Recommendation now |
|---|---|---|
| 1 | Split 7.5 into 7.5a (counting in the low IR) and 7.5b (7.12: reuse, fault regions) | **Yes, and put 7.13 (layout) before or with 7.5a.** Before both, one small task for the cheap fixes: one translation unit or LTO, a size-class allocator, static field-less variants, integer formatting, constant division |
| 2 | Hosting for 7.6 | **B′.** The benchmarks don't touch it. A crossing under B′ costs about 35 ns; the lexer spends about 780 ns of compiled code per token today, and a job's crossings are its entity calls and steps, which wait on input and output anyway. 7.10's count stays useful for the record, not for the choice |
| 3 | The store and tasks stay in the Go runner | **Yes.** Nothing in the numbers asks otherwise |
| 4 | v0.4's exit test | **Keep "the digest runs as a native job",** and record the interpreted digest's idle memory and resume time now (R.7 gives them) as the numbers to beat |
| 5 | Order | **7.8 and the cheap fixes now** (no decision needed beyond this), **then 7.13 with 7.5a, then 7.9, 7.10, 7.6; 7.11 tracks the bar throughout; 7.12 last.** Phase 6's exit test (6.6, 6.8) is independent and waits on the homelab |
| 6 | A `wasm32-wasi` target | **Later.** Unchanged by the numbers |
| 7 | The bar: per benchmark or on the mean | **Per benchmark, for time and for memory on data held.** Count the start-up floor separately, so a win on it isn't read as a win on layout |

And for open question 43: the five changes in section 4, as one task with a spec change first,
independent of the native work. The lexer is their test: it gets shorter and reads better.

## Where the evidence is

- The suite: `testdata/bench`, `go run ./tools/bench` (`--write`, `--check`), results in
  [Benchmarks against Go](../benchmarks.md) and `testdata/bench/results.json`.
- The lexer: `testdata/programs/lexer`, checked by `interp/lexer_test.go` and natively by
  `cgen`'s `TestLexerBinary`.
- The profiles: the native plan's [Against Go](./native-plan-2026-10-12.md#against-go).
- The two C experiments of section 2 are small enough to repeat: fib and trees written by hand,
  built with `go run ./tools/zig cc -target x86_64-linux-musl -O2 -fno-sanitize=undefined`, and the
  emitted C of `testdata/bench/fib` built with and without `-flto`.
