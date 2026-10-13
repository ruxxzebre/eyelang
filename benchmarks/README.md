# Benchmarks

Measurements of `eye` on inputs much larger than its tests use. The inputs are
generated into temporary directories each run and removed afterwards; nothing
large is committed. There are two:

- **The corpus:** the real programs under `testdata/programs` and
  `testdata/lab`, copied side by side: thousands of small programs, for
  `eye fmt`.
- **The program:** one generated program of any size, modules `m0000`,
  `m0001`, … of ordinary core-language code (records, variants, matches,
  lists, loops, interpolated strings) and a `main` that calls every module,
  for `eye check` and `eye build`. It uses only what the C back end compiles,
  and every function is reachable, so a build compiles all of it.

## What is here

| Benchmark | Measures |
|---|---|
| `BenchmarkFmtCheck/cold` | `eye fmt --check --cold .`: every program checked, every file formatted |
| `BenchmarkFmtCheck/miss` | `eye fmt --check .` with no saved result: the same work, then the result saved |
| `BenchmarkFmtCheck/warm` | `eye fmt --check .` with a saved result: the inputs reread and the output replayed |
| `BenchmarkCheck` | `eye check .` on the program |
| `BenchmarkBuild/emit-c` | `eye build --emit-c`: eye's own part of a build, checking the program and writing its C |
| `BenchmarkBuild/debug` | `eye build -g`: a whole build, unoptimised |
| `BenchmarkBuild/release` | `eye build`: a whole build, optimised, as it is by default |

A build starts from an object cache that holds the runtime's objects but not
the program's, so it times compiling the program and nothing else. The pinned
zig is used from where `eye` keeps it, and fetched there first if it isn't.

Each runs `eye` as a whole process, startup included, and reports `ns/op`,
`lines/s` and `peak-MiB`, the most memory one run held.

Two tests keep the benchmarks working, and run with `go test ./...` (`-short`
skips them): `TestFmtCheckSameOutput` runs a small corpus through a cold, a
first and a warm check and fails unless all three print the same, and
`TestProgramBuilds` checks that a small generated program checks clean, is
formatted, and prints the same built natively as in the interpreter.

## Running

```sh
# 100,000 lines, the default
go test ./benchmarks -run '^$' -bench FmtCheck -benchtime 5x

# a million lines; each cold run takes several seconds
EYE_BENCH_LINES=1000000 go test ./benchmarks -run '^$' -bench FmtCheck -benchtime 3x

# eye check on a program of 100,000 lines, and eye build on one of 10,000
go test ./benchmarks -run '^$' -bench 'Check$|Build' -benchtime 2x -timeout 30m
```

A release build of 100,000 lines takes minutes, and its time grows faster
than the program: raise `EYE_BENCH_BUILD_LINES` a step at a time, and give
`go test` a `-timeout` to match.

| Variable | Effect |
|---|---|
| `EYE_BENCH_LINES` | Size of the corpus, and of the program `BenchmarkCheck` checks, in lines (100,000 by default); the corpus is rounded up to whole copies of the seed programs, about 6,300 lines each |
| `EYE_BENCH_BUILD_LINES` | Size of the program `BenchmarkBuild` compiles (10,000 by default) |
| `EYE_BENCH_EYE` | The `eye` to measure, such as an older build; otherwise one is built from this checkout |
| `EYE_BENCH_CORPUS` | An empty directory to write the corpus into and keep, to run `eye` on by hand |

The corpus is the seed programs copied side by side, `c0000/programs/…`,
`c0001/programs/…`, each copy keeping its `eye.toml` files, so every copy is a
set of programs of its own and nothing is checked twice by accident. The seed
holds one file that `eye fmt` would change, the generated
`testdata/programs/tour/tour.eye`, so a check exits 1 and lists one file per
copy; the benchmark accepts exit 1 and fails on anything above it.

## Comparing two builds

Build the older `eye` from its commit, then run the same benchmark against
both and compare with [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat):

```sh
git worktree add /tmp/eye-old <commit>
(cd /tmp/eye-old && go build -o /tmp/eye-old.bin ./cmd/eye)
git worktree remove /tmp/eye-old

EYE_BENCH_EYE=/tmp/eye-old.bin go test ./benchmarks -run '^$' -bench FmtCheck -count 6 > old.txt
go test ./benchmarks -run '^$' -bench FmtCheck -count 6 > new.txt
go run golang.org/x/perf/cmd/benchstat@latest old.txt new.txt
```

Timings move with what else the machine runs; alternate the two builds, or
use `-count`, rather than trusting one run. To see where a run spends its
time, `eye fmt --check --cold --stats .` in a kept corpus breaks it down by
phase.

## Results

All on a 10-core Apple Silicon Mac, 2026-10-13.

### eye check and eye build

How long one program takes, by its size. The targets are a million lines
compiled in at most a minute, in 10 seconds in the long term and in 1 second
at the furthest; SPECS.md open question 47 has them and the ways there.

| Lines | `eye check` | Writing the C | Whole build, `-g` | Whole build, optimised | Lines of C |
|---:|---:|---:|---:|---:|---:|
| 10,000 | 67 ms | 0.2 s | 1.1 s | 18 s | 124,000 |
| 100,000 | 0.33 s | 1.8 s | 14 s | 4 min | 1.2 million |
| 1,000,000 | 3.8 s, 2.7 GB | 29 s, 4.7 GB | not run | not run, over an hour by the trend | 11.6 million (748 MB) |

The C compile is nearly all of a build: about 12 lines of C per line of
Eyelang, in one file, which the optimiser takes in time growing faster than
the file. Building a million lines would need the C split and compiled in
parallel before it could be measured at all.

### eye fmt

`eye fmt --check --cold .` on a million lines (9,440 files, 4,480 programs,
21,920 modules):

| Build | Total | Check phase | Peak memory |
|---|---:|---:|---:|
| Programs checked one after another (`49d7575`) | 10.8 s | 8.8 s | 2.2 GB |
| Programs checked in parallel (`27287c5`) | 6.3 s | 4.4 s | 2.2 GB |
| Each file, directory and manifest read once per run | 5.3 s | 3.7 s | 2.4 GB |

The three builds ran alternately, three times each; the table gives the
middle run. At 100,000 lines: cold about 0.9 s and 270 MiB, warm about 80 ms
and 21 MiB.

Where the rest goes, from a CPU profile and `GODEBUG=gctrace=1`: the live
heap grows to about 1.1 GB, nearly all of it the parsed syntax of every file,
which the session keeps so the formatter doesn't parse each file twice.
Collecting it costs 12–14% of the CPU time. A leaner syntax tree would cut
both memory and time; reading each time zone once (`checkZones` asks the
system for every zone literal) would cut about 2%.

## Adding a benchmark

Put it here as a `Benchmark…` function in a `_test.go` file. Generate its
input with `corpus` or a generator of its own into a temporary directory;
don't commit large inputs. Report a rate (`lines/s` or similar) as well as
`ns/op`, so results at different sizes compare, and add a small `Test…`
that runs the same path quickly, so `go test ./...` keeps it working.
