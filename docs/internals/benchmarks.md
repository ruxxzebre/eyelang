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
| Commit | `a5ba1d0` |

## Results

| Benchmark | Native ms | Go ms | Time × Go | Native MB | Go MB | Memory × Go | Interpreter ms | Interpreter MB |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| fib | 233 | 138 | 1.69× | 0.27 | 1.91 | 0.14× | 9953 | 18.8 |
| mandelbrot | 249 | 217 | 1.15× | 0.27 | 1.91 | 0.14× | 29342 | 22.2 |
| trees | 316 | 482 | 0.66× | 8.39 | 12.4 | 0.68× | 10381 | 50.1 |
| records | 134 | 253 | 0.53× | 0.28 | 2.03 | 0.14× | 10455 | 21.1 |
| strings | 167 | 374 | 0.45× | 67.9 | 48.1 | 1.41× | 5894 | 930.2 |
| lists | 155 | 154 | 1.00× | 19.5 | 48.1 | 0.41× | 3542 | 134.1 |
| maps | 199 | 123 | 1.62× | 0.90 | 2.53 | 0.36× | 17388 | 22.4 |
| logs | 246 | 330 | 0.74× | 43.8 | 59.8 | 0.73× | 2194 | 79.4 |
| lexer | 561 | 870 | 0.65× | 62.8 | 191.6 | 0.33× | 30606 | 1620.8 |

Faster than Go: trees, records, strings, logs, lexer.  
Less memory than Go: fib, mandelbrot, trees, records, lists, maps, logs, lexer.  
Geometric mean: 0.85× Go's time, 0.36× Go's memory.  

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
