# Benchmarks

Measurements of `eye` on inputs much larger than its tests use. The inputs are
generated into temporary directories each run, from the real programs under
`testdata/programs` and `testdata/lab`, and removed afterwards; nothing large
is committed.

## What is here

| Benchmark | Measures |
|---|---|
| `BenchmarkFmtCheck/cold` | `eye fmt --check --cold .`: every program checked, every file formatted |
| `BenchmarkFmtCheck/miss` | `eye fmt --check .` with no saved result: the same work, then the result saved |
| `BenchmarkFmtCheck/warm` | `eye fmt --check .` with a saved result: the inputs reread and the output replayed |

Each runs `eye` as a whole process, startup included, and reports `ns/op`,
`lines/s` and `peak-MiB`, the most memory one run held.

`TestFmtCheckSameOutput` runs a small corpus through a cold, a first and a
warm check and fails unless all three print the same. It runs with
`go test ./...`, so the benchmark can't silently break; `-short` skips it.

## Running

```sh
# 100,000 lines, the default
go test ./benchmarks -run '^$' -bench FmtCheck -benchtime 5x

# a million lines; each cold run takes several seconds
EYE_BENCH_LINES=1000000 go test ./benchmarks -run '^$' -bench FmtCheck -benchtime 3x
```

| Variable | Effect |
|---|---|
| `EYE_BENCH_LINES` | Size of the corpus in lines, rounded up to whole copies of the seed programs (about 6,300 lines each) |
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

`eye fmt --check --cold .` on a million lines (9,440 files, 4,480 programs,
21,920 modules), on a 10-core Apple Silicon Mac, 2026-10-13:

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
