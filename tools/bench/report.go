package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"eyelang/internal/zig"
)

const (
	pagePath    = "docs/internals/benchmarks.md"
	resultsPath = "testdata/bench/results.json"
)

// environment is the machine and the tools the results come from.
type environment struct {
	Date    string `json:"date"`
	Commit  string `json:"commit"`
	Machine string `json:"machine"`
	Go      string `json:"go"`
	Zig     string `json:"zig"`
}

// recorded is what results.json holds.
type recorded struct {
	environment
	Runs       int              `json:"runs"`
	InterpRuns int              `json:"interp_runs,omitempty"`
	Benchmarks []recordedResult `json:"benchmarks"`
}

type recordedResult struct {
	Name      string  `json:"name"`
	Native    sample  `json:"native"`
	Go        sample  `json:"go"`
	Interp    *sample `json:"interp,omitempty"`
	TimeRatio float64 `json:"time_ratio"`
	RSSRatio  float64 `json:"rss_ratio"`
}

// describe finds out the machine, Go's and zig's versions, the date and
// the commit.
func describe(root string) environment {
	env := environment{
		Date:    time.Now().Format("2006-01-02"),
		Machine: machine(),
		Go:      output(root, "go", "env", "GOVERSION"),
		Commit:  output(root, "git", "rev-parse", "--short", "HEAD"),
	}
	if exe, err := zig.Path(nil); err == nil {
		env.Zig = output(root, exe, "version")
	}
	return env
}

func output(dir, name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	b, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(b))
}

// machine names the processor, its cores and the system.
func machine() string {
	cpu := runtime.GOARCH
	if f, err := os.Open("/proc/cpuinfo"); err == nil {
		s := bufio.NewScanner(f)
		for s.Scan() {
			if k, v, ok := strings.Cut(s.Text(), ":"); ok && strings.TrimSpace(k) == "model name" {
				cpu = strings.Join(strings.Fields(v), " ")
				break
			}
		}
		f.Close()
	} else if runtime.GOOS == "darwin" {
		if b, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
			cpu = strings.TrimSpace(string(b))
		}
	}
	return fmt.Sprintf("%s, %d cores, %s/%s", cpu, runtime.NumCPU(), runtime.GOOS, runtime.GOARCH)
}

func ms(v float64) string {
	switch {
	case v < 10:
		return fmt.Sprintf("%.2f", v)
	case v < 100:
		return fmt.Sprintf("%.1f", v)
	default:
		return fmt.Sprintf("%.0f", v)
	}
}

func mb(kb int64) string {
	if kb < 10*1024 {
		return fmt.Sprintf("%.2f", float64(kb)/1024)
	}
	return fmt.Sprintf("%.1f", float64(kb)/1024)
}

func ratio(r float64) string { return fmt.Sprintf("%.2f×", r) }

// table is the results as a Markdown table.
func table(results []result, interp bool) string {
	var b strings.Builder
	b.WriteString("| Benchmark | Native ms | Go ms | Time × Go | Native MB | Go MB | Memory × Go |")
	sep := "|---|---:|---:|---:|---:|---:|---:|"
	if interp {
		b.WriteString(" Interpreter ms | Interpreter MB |")
		sep += "---:|---:|"
	}
	b.WriteString("\n" + sep + "\n")
	for _, r := range results {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s |", r.Name,
			ms(r.Native.MinMS), ms(r.Go.MinMS), ratio(r.TimeRatio()),
			mb(r.Native.RSSKB), mb(r.Go.RSSKB), ratio(r.RSSRatio()))
		if interp {
			if r.Interp != nil {
				fmt.Fprintf(&b, " %s | %s |", ms(r.Interp.MinMS), mb(r.Interp.RSSKB))
			} else {
				b.WriteString(" | |")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// winners says which benchmarks beat Go on time and on memory.
func winners(results []result) string {
	var faster, leaner []string
	for _, r := range results {
		if r.TimeRatio() < 1 {
			faster = append(faster, r.Name)
		}
		if r.RSSRatio() < 1 {
			leaner = append(leaner, r.Name)
		}
	}
	list := func(names []string) string {
		if len(names) == 0 {
			return "none yet"
		}
		return strings.Join(names, ", ")
	}
	var geoT, geoM float64
	for _, r := range results {
		geoT += math.Log(r.TimeRatio())
		geoM += math.Log(r.RSSRatio())
	}
	n := float64(len(results))
	return fmt.Sprintf("Faster than Go: %s.\nLess memory than Go: %s.\nGeometric mean: %s Go's time, %s Go's memory.\n",
		list(faster), list(leaner), ratio(math.Exp(geoT/n)), ratio(math.Exp(geoM/n)))
}

func readResults(root string) (recorded, error) {
	var rec recorded
	b, err := os.ReadFile(filepath.Join(root, resultsPath))
	if err != nil {
		return rec, fmt.Errorf("no recorded results to check against (run --write first): %w", err)
	}
	if err := json.Unmarshal(b, &rec); err != nil {
		return rec, fmt.Errorf("%s: %w", resultsPath, err)
	}
	return rec, nil
}

// worse says whether a result's time and memory are worse than recorded
// by more than the margins. Memory must also have grown by more than
// rssFloorKB, since a few pages more on a program of 200 KB is a large
// ratio but no regression.
func worse(old recordedResult, r result) (time, memory bool) {
	time = r.TimeRatio() > old.TimeRatio*(1+timeMargin)
	memory = r.RSSRatio() > old.RSSRatio*(1+rssMargin) && r.Native.RSSKB > old.Native.RSSKB+rssFloorKB
	return time, memory
}

// compare checks the results against the recorded ones: native code may
// not get slower or larger against Go by more than the margins. It gives
// its report and the benchmarks that got worse.
func compare(rec recorded, results []result) (string, []string) {
	var b strings.Builder
	fmt.Fprintf(&b, "Against the results recorded on %s at %s (as ratios to Go, time may grow %.0f%%, memory %.0f%%):\n\n",
		rec.Date, rec.Commit, timeMargin*100, rssMargin*100)
	var failed []string
	better := false
	byName := map[string]recordedResult{}
	for _, r := range rec.Benchmarks {
		byName[r.Name] = r
	}
	for _, r := range results {
		old, found := byName[r.Name]
		if !found {
			fmt.Fprintf(&b, "  %-11s not recorded: run --write to record it\n", r.Name)
			continue
		}
		timeWorse, memWorse := worse(old, r)
		if timeWorse || memWorse {
			failed = append(failed, r.Name)
		}
		verdict := func(what string, now, was, margin float64, bad bool) string {
			switch {
			case bad:
				return fmt.Sprintf("%s %s → %s WORSE", what, ratio(was), ratio(now))
			case now < was*(1-margin):
				better = true
				return fmt.Sprintf("%s %s → %s better", what, ratio(was), ratio(now))
			default:
				return fmt.Sprintf("%s %s → %s", what, ratio(was), ratio(now))
			}
		}
		fmt.Fprintf(&b, "  %-11s %-32s %s\n", r.Name,
			verdict("time", r.TimeRatio(), old.TimeRatio, timeMargin, timeWorse),
			verdict("memory", r.RSSRatio(), old.RSSRatio, rssMargin, memWorse))
	}
	b.WriteString("\n")
	switch {
	case len(failed) > 0:
		fmt.Fprintf(&b, "FAIL: native code got slower or larger against Go than recorded: %s.\n", strings.Join(failed, ", "))
	case better:
		b.WriteString("ok; some ratios are well below the recorded ones: run --write to record them.\n")
	default:
		b.WriteString("ok\n")
	}
	return b.String(), failed
}

// best keeps, of two measurements of the same benchmark, the better time
// and the better memory: a regression has to show in every attempt.
func best(a, b result) result {
	out := a
	if b.TimeRatio() < a.TimeRatio() {
		out.Native.MinMS, out.Native.MedianMS = b.Native.MinMS, b.Native.MedianMS
		out.Go.MinMS, out.Go.MedianMS = b.Go.MinMS, b.Go.MedianMS
	}
	if b.RSSRatio() < a.RSSRatio() {
		out.Native.RSSKB, out.Go.RSSKB = b.Native.RSSKB, b.Go.RSSKB
	}
	return out
}

// writeResults writes results.json and the docs page.
func writeResults(root string, env environment, results []result, opts options) error {
	rec := recorded{environment: env, Runs: opts.Runs}
	if opts.Interp {
		rec.InterpRuns = opts.InterpRuns
	}
	for _, r := range results {
		rec.Benchmarks = append(rec.Benchmarks, recordedResult{
			Name: r.Name, Native: r.Native, Go: r.Go, Interp: r.Interp,
			TimeRatio: round3(r.TimeRatio()), RSSRatio: round3(r.RSSRatio()),
		})
	}
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, resultsPath), append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, pagePath), []byte(page(env, results, opts)), 0o644)
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

// page is docs/internals/benchmarks.md.
func page(env environment, results []result, opts options) string {
	var b strings.Builder
	b.WriteString("---\ntitle: Benchmarks against Go\n---\n\n")
	b.WriteString("<!-- Written by `go run ./tools/bench --write`. Don't edit; rerun it. -->\n\n")
	b.WriteString("# Benchmarks against Go\n\n")
	b.WriteString("The bar for natively compiled Eyelang is **faster than Go, on less memory**, with Go as the ")
	b.WriteString("baseline. Each benchmark in [`testdata/bench`](https://github.com/ruxxzebre/eyelang/tree/dev/testdata/bench) ")
	b.WriteString("is an Eyelang program and the same algorithm written as idiomatic Go. The Eyelang program is ")
	b.WriteString("compiled with `eye build` (C at `-O2`, through the pinned zig) and the Go one with `go build`; ")
	b.WriteString("both print the same result, which the tool checks. The interpreter (`eye run --memory`) runs the ")
	b.WriteString("same program for comparison.\n\n")
	fmt.Fprintf(&b, "Native code and Go ran %d times each, taking turns", opts.Runs)
	if opts.Interp {
		fmt.Fprintf(&b, ", and the interpreter %d", opts.InterpRuns)
		if opts.InterpRuns == 1 {
			b.WriteString(" time")
		} else {
			b.WriteString(" times")
		}
	}
	b.WriteString(". Times are the fastest run's wall clock, start to exit; ")
	b.WriteString("memory is the median of the runs' peak resident set (max RSS, from `wait4`). A ratio is native ")
	b.WriteString("code's over Go's, so below 1.00× beats Go. A Go program starts at about 2 MB resident and a ")
	b.WriteString("native Eyelang one at about 0.2 MB, so where a benchmark holds little data, its memory ratio ")
	b.WriteString("shows that fixed cost rather than how values are stored.\n\n")
	b.WriteString("| | |\n|---|---|\n")
	fmt.Fprintf(&b, "| Machine | %s |\n| Go | %s |\n| zig | %s |\n| Date | %s |\n| Commit | `%s` |\n\n",
		env.Machine, env.Go, env.Zig, env.Date, env.Commit)
	b.WriteString("## Results\n\n")
	b.WriteString(table(results, opts.Interp))
	b.WriteString("\n")
	for _, line := range strings.Split(strings.TrimSpace(winners(results)), "\n") {
		b.WriteString(line + "  \n")
	}
	b.WriteString("\nWhere each gap comes from, and the work that must close it, is in the ")
	b.WriteString("[native compilation plan](./reviews/native-plan-2026-10-12.md#against-go).\n\n")
	b.WriteString("## The benchmarks\n\n")
	b.WriteString("| Benchmark | What it exercises |\n|---|---|\n")
	byName := map[string]string{}
	for _, s := range suite {
		byName[s.Name] = s.What
	}
	for _, r := range results {
		fmt.Fprintf(&b, "| [`%s`](https://github.com/ruxxzebre/eyelang/tree/dev/testdata/bench/%s) | %s |\n", r.Name, r.Name, byName[r.Name])
	}
	b.WriteString("\n## Running them\n\n")
	b.WriteString("From the repository root:\n\n")
	b.WriteString("```sh\n")
	b.WriteString("go run ./tools/bench              # the table: 5 runs of each binary\n")
	b.WriteString("go run ./tools/bench --only fib   # one benchmark, or a comma-separated list\n")
	b.WriteString("go run ./tools/bench --no-interp  # without the interpreter, which is the slow part\n")
	b.WriteString("go run ./tools/bench --check      # fail when native code got worse against Go\n")
	b.WriteString("go run ./tools/bench --write      # record new results: this page and testdata/bench/results.json\n")
	b.WriteString("```\n\n")
	fmt.Fprintf(&b, "`--check` reruns the suite and compares each ratio with the one recorded in ")
	fmt.Fprintf(&b, "`%s`. It fails when native code's time against Go's grew by more than %.0f%%, or its memory ", resultsPath, timeMargin*100)
	fmt.Fprintf(&b, "against Go's by more than %.0f%% and by more than %d MB. A benchmark that looks worse is measured ", rssMargin*100, rssFloorKB/1024)
	fmt.Fprintf(&b, "up to %d more times, keeping its best time and memory, so a slow stretch of the machine doesn't fail ", retries)
	b.WriteString("the check and a real regression still does. Ratios against Go on the same machine cancel most of ")
	b.WriteString("the difference between machines, but not all of it, so record new results on the machine you ")
	b.WriteString("check on. CI doesn't run the timing check, since shared runners are too noisy for it; ")
	b.WriteString("`go test ./tools/bench` (skipped under `-short`) builds every benchmark and runs it at a small ")
	b.WriteString("size, only to check that native code, Go and the interpreter print the same.\n\n")
	b.WriteString("A new benchmark is a directory under `testdata/bench` with an Eyelang program whose `main` does the ")
	b.WriteString("work and a `main.go` doing the same in Go, both taking the same arguments (usually an optional size), ")
	b.WriteString("plus a line in `tools/bench`'s suite with the arguments of a measured run and of a quick one. `main.go` ")
	b.WriteString("is built from the repository root, inside its module, so it may import the repository's packages. ")
	b.WriteString("Size a benchmark to run 100 ms to 1 s in Go.\n")
	return b.String()
}
