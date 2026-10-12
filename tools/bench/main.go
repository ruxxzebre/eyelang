// Command bench runs the benchmarks in testdata/bench: each is an Eyelang
// program and the same algorithm in Go. It compiles the Eyelang program
// natively (eye build, -O2) and the Go one with go build, runs each several
// times, checks that they print the same thing, and prints native code's
// time and peak memory against Go's, with the interpreter's beside them.
//
// Run it from the repository root:
//
//	go run ./tools/bench            # the table
//	go run ./tools/bench --write    # also docs/internals/benchmarks.md and testdata/bench/results.json
//	go run ./tools/bench --check    # fail when native code got slower or larger against Go than recorded
//
// Go is the baseline: a ratio is native over Go, so below 1 beats Go.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// A benchmark is a directory under testdata/bench with an Eyelang program
// whose main does the work and a main.go doing the same in Go. main.go is
// built with `go build` from the repository root, inside its module, so it
// may import the repository's packages (give it `//go:build ignore` then, so
// `go build ./...` leaves it out; files under testdata are left out anyway).
// Both get the same arguments; most take an optional size and run at full
// size without one. Paths in arguments are relative to the repository root,
// where every binary runs.
type benchmark struct {
	Name  string   // the directory
	What  string   // what it exercises
	Args  []string // the arguments of a measured run
	Quick []string // the arguments of a quick run, which only checks the outputs
}

// suite is every benchmark, in the order the table lists them.
var suite = []benchmark{
	{"fib", "calls and integers: naive fib(37)", nil, []string{"20"}},
	{"mandelbrot", "floats: a 1600×1600 Mandelbrot set, 100 iterations", nil, []string{"40"}},
	{"trees", "allocation: binary trees of depth 4 to 16", nil, []string{"6"}},
	{"records", "records: `with` updates of a 5-field record in a loop", nil, []string{"1000"}},
	{"strings", "strings: build a 1M-item string with `+=`, then split it", nil, []string{"1000"}},
	{"lists", "lists: map, filter and sum over 1M ints; sort 1M", nil, []string{"1000"}},
	{"maps", "maps: count 5M words over 5,000 keys", nil, []string{"1000"}},
	{"logs", "text: generate a 200k-line log, parse it, aggregate per service", nil, []string{"500"}},
}

// Thresholds of --check: how much worse than recorded a ratio may get
// before it counts as a regression rather than noise.
const (
	timeMargin = 0.25
	rssMargin  = 0.15
	rssFloorKB = 1024
	// retries is how many more times --check measures a benchmark that
	// looks worse, before it calls it a regression.
	retries = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("bench", flag.ContinueOnError)
	fs.SetOutput(stderr)
	runs := fs.Int("runs", 5, "how many times to run each binary")
	only := fs.String("only", "", "run only these benchmarks, comma-separated")
	quick := fs.Bool("quick", false, "small sizes and one run: check that everything builds and the outputs match")
	noInterp := fs.Bool("no-interp", false, "skip the interpreter (eye run --memory)")
	interpRuns := fs.Int("interp-runs", 1, "how many times to run the interpreter, which is slow")
	write := fs.Bool("write", false, "write the results to "+pagePath+" and "+resultsPath)
	check := fs.Bool("check", false, "compare with "+resultsPath+" and fail on a regression")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: go run ./tools/bench [--runs n] [--only a,b] [--quick] [--no-interp] [--write | --check]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *quick && (*write || *check) {
		fmt.Fprintln(stderr, "bench: --quick runs small sizes, which can't be written or checked")
		return 2
	}
	if *write && *only != "" {
		fmt.Fprintln(stderr, "bench: --write records the whole suite; leave out --only")
		return 2
	}
	benches, err := pick(*only)
	if err != nil {
		fmt.Fprintln(stderr, "bench:", err)
		return 2
	}
	root, err := findRoot()
	if err != nil {
		fmt.Fprintln(stderr, "bench:", err)
		return 1
	}
	opts := options{Root: root, Runs: *runs, Quick: *quick, Interp: !*noInterp, InterpRuns: *interpRuns, Log: stderr}
	if *quick {
		opts.Runs, opts.InterpRuns = 1, 1
	}
	if opts.Runs < 1 || opts.InterpRuns < 1 {
		fmt.Fprintln(stderr, "bench: --runs and --interp-runs must be at least 1")
		return 2
	}
	results, err := measure(benches, opts)
	if err != nil {
		fmt.Fprintln(stderr, "bench:", err)
		return 1
	}
	if *quick {
		for _, r := range results {
			fmt.Fprintf(stdout, "ok    %s\n", r.Name)
		}
		return 0
	}
	status := 0
	var report string
	if *check {
		recorded, err := readResults(root)
		if err != nil {
			fmt.Fprintln(stderr, "bench:", err)
			return 1
		}
		var failed []string
		report, failed = compare(recorded, results)
		// A slow stretch of a shared machine looks like a regression; a
		// real one shows again.
		for attempt := 0; attempt < retries && len(failed) > 0; attempt++ {
			fmt.Fprintf(stderr, "measuring again: %s\n", strings.Join(failed, ", "))
			again, err := pick(strings.Join(failed, ","))
			if err != nil {
				fmt.Fprintln(stderr, "bench:", err)
				return 1
			}
			retry := opts
			retry.Interp = false
			more, err := measure(again, retry)
			if err != nil {
				fmt.Fprintln(stderr, "bench:", err)
				return 1
			}
			for _, m := range more {
				for i := range results {
					if results[i].Name == m.Name {
						results[i] = best(results[i], m)
					}
				}
			}
			report, failed = compare(recorded, results)
		}
		if len(failed) > 0 {
			status = 1
		}
	}
	env := describe(root)
	fmt.Fprint(stdout, table(results, opts.Interp))
	fmt.Fprintln(stdout)
	fmt.Fprint(stdout, winners(results))
	if report != "" {
		fmt.Fprintln(stdout)
		fmt.Fprint(stdout, report)
	}
	if *write {
		if err := writeResults(root, env, results, opts); err != nil {
			fmt.Fprintln(stderr, "bench:", err)
			return 1
		}
		fmt.Fprintf(stdout, "\nwrote %s and %s\n", pagePath, resultsPath)
	}
	return status
}

// pick gives the benchmarks named in a comma-separated list, or all of them.
func pick(only string) ([]benchmark, error) {
	if only == "" {
		return suite, nil
	}
	var out []benchmark
	for _, name := range strings.Split(only, ",") {
		found := false
		for _, b := range suite {
			if b.Name == strings.TrimSpace(name) {
				out = append(out, b)
				found = true
			}
		}
		if !found {
			var names []string
			for _, b := range suite {
				names = append(names, b.Name)
			}
			return nil, fmt.Errorf("no benchmark %q: there are %s", name, strings.Join(names, ", "))
		}
	}
	return out, nil
}
