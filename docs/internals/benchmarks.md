---
title: Benchmarks against Go
---

<!-- Written by `go run ./tools/bench --write`. Don't edit; rerun it. -->

# Benchmarks against Go

The bar for natively compiled Eyelang is **faster than Go, on less memory**, with Go as the baseline. Each benchmark in [`testdata/bench`](https://github.com/ruxxzebre/eyelang/tree/dev/testdata/bench) is an Eyelang program and the same algorithm written as idiomatic Go. The Eyelang program is compiled with `eye build` (C at `-O2`, through the pinned zig) and the Go one with `go build`; both print the same result, which the tool checks. The interpreter (`eye run --memory`) runs the same program for comparison.

Native code and Go ran 5 times each, taking turns, and the interpreter 1 time. Times are the fastest run's wall clock, start to exit; memory is the median of the runs' peak resident set (max RSS, from `wait4`). A ratio is native code's over Go's, so below 1.00× beats Go. A Go program starts at about 2 MB resident and a native Eyelang one at about 0.2 MB, so where a benchmark holds little data, its memory ratio shows that fixed cost rather than how values are stored.

| | |
|---|---|
| Machine | Intel(R) Xeon(R) Processor @ 2.80GHz, 4 cores, linux/amd64 |
| Go | go1.27.2 |
| zig | 0.17.0 |
| Date | 2026-10-12 |
| Commit | `7b2faa2` |

## Results

| Benchmark | Native ms | Go ms | Time × Go | Native MB | Go MB | Memory × Go | Interpreter ms | Interpreter MB |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| fib | 522 | 144 | 3.62× | 0.28 | 2.03 | 0.14× | 11488 | 18.7 |
| mandelbrot | 273 | 226 | 1.20× | 0.28 | 2.03 | 0.14× | 40380 | 21.5 |
| trees | 672 | 596 | 1.13× | 25.2 | 12.1 | 2.08× | 11904 | 52.9 |
| records | 1702 | 240 | 7.08× | 0.28 | 1.91 | 0.15× | 15143 | 21.3 |
| strings | 222 | 355 | 0.63× | 67.9 | 48.2 | 1.41× | 6107 | 719.3 |
| lists | 240 | 174 | 1.38× | 42.3 | 48.1 | 0.88× | 5112 | 142.3 |
| maps | 601 | 155 | 3.89× | 0.89 | 2.53 | 0.35× | 30092 | 32.7 |
| logs | 325 | 345 | 0.94× | 43.8 | 60.1 | 0.73× | 3185 | 94.1 |
| lexer | 1540 | 516 | 2.98× | 200.2 | 191.7 | 1.04× | 42339 | 1715.4 |

Faster than Go: strings, logs.  
Less memory than Go: fib, mandelbrot, records, lists, maps, logs.  
Geometric mean: 1.90× Go's time, 0.50× Go's memory.  

Where each gap comes from, and the work that must close it, is in the [native compilation plan](./reviews/native-plan-2026-10-12.md#against-go).

## The benchmarks

| Benchmark | What it exercises |
|---|---|
| [`fib`](https://github.com/ruxxzebre/eyelang/tree/dev/testdata/bench/fib) | calls and integers: naive fib(37) |
| [`mandelbrot`](https://github.com/ruxxzebre/eyelang/tree/dev/testdata/bench/mandelbrot) | floats: a 1600×1600 Mandelbrot set, 100 iterations |
| [`trees`](https://github.com/ruxxzebre/eyelang/tree/dev/testdata/bench/trees) | allocation: binary trees of depth 4 to 16 |
| [`records`](https://github.com/ruxxzebre/eyelang/tree/dev/testdata/bench/records) | records: `with` updates of a 5-field record in a loop |
| [`strings`](https://github.com/ruxxzebre/eyelang/tree/dev/testdata/bench/strings) | strings: build a 1M-item string with `+=`, then split it |
| [`lists`](https://github.com/ruxxzebre/eyelang/tree/dev/testdata/bench/lists) | lists: map, filter and sum over 1M ints; sort 1M |
| [`maps`](https://github.com/ruxxzebre/eyelang/tree/dev/testdata/bench/maps) | maps: count 5M words over 5,000 keys |
| [`logs`](https://github.com/ruxxzebre/eyelang/tree/dev/testdata/bench/logs) | text: generate a 200k-line log, parse it, aggregate per service |
| [`lexer`](https://github.com/ruxxzebre/eyelang/tree/dev/testdata/bench/lexer) | a real program: Eyelang's lexer in Eyelang against syntax/lexer.go, on 7.5 MB of Eyelang |

## Running them

From the repository root:

```sh
go run ./tools/bench              # the table: 5 runs of each binary
go run ./tools/bench --only fib   # one benchmark, or a comma-separated list
go run ./tools/bench --no-interp  # without the interpreter, which is the slow part
go run ./tools/bench --check      # fail when native code got worse against Go
go run ./tools/bench --write      # record new results: this page and testdata/bench/results.json
```

`--check` reruns the suite and compares each ratio with the one recorded in `testdata/bench/results.json`. It fails when native code's time against Go's grew by more than 25%, or its memory against Go's by more than 15% and by more than 1 MB. A benchmark that looks worse is measured up to 2 more times, keeping its best time and memory, so a slow stretch of the machine doesn't fail the check and a real regression still does. Ratios against Go on the same machine cancel most of the difference between machines, but not all of it, so record new results on the machine you check on. CI doesn't run the timing check, since shared runners are too noisy for it; `go test ./tools/bench` (skipped under `-short`) builds every benchmark and runs it at a small size, only to check that native code, Go and the interpreter print the same.

A new benchmark is a directory under `testdata/bench` with an Eyelang program whose `main` does the work and a `main.go` doing the same in Go, both taking the same arguments (usually an optional size), plus a line in `tools/bench`'s suite with the arguments of a measured run and of a quick one. `main.go` is built from the repository root, inside its module, so it may import the repository's packages. Size a benchmark to run 100 ms to 1 s in Go.
