package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"eyelang/cgen"
	"eyelang/check"
)

// options say how to measure.
type options struct {
	Root   string // the repository
	Runs   int    // runs of each binary
	Quick  bool   // run each benchmark at its quick size
	Interp bool   // also run the interpreter
	// InterpRuns is how many of the runs include the interpreter, which
	// takes 10 to 100 times as long as native code.
	InterpRuns int
	Log        io.Writer // progress
}

// A sample is what the runs of one binary measured.
type sample struct {
	MinMS    float64 `json:"min_ms"`
	MedianMS float64 `json:"median_ms"`
	RSSKB    int64   `json:"rss_kb"` // the median of the runs' peak resident memory
}

// A result is one benchmark's measurements.
type result struct {
	Name   string
	What   string
	Native sample
	Go     sample
	Interp *sample // nil when the interpreter was skipped
}

// TimeRatio is native code's fastest run over Go's: below 1 beats Go.
func (r result) TimeRatio() float64 { return r.Native.MinMS / r.Go.MinMS }

// RSSRatio is native code's peak memory over Go's: below 1 beats Go.
func (r result) RSSRatio() float64 { return float64(r.Native.RSSKB) / float64(r.Go.RSSKB) }

// measure builds and runs each benchmark.
func measure(benches []benchmark, opts options) ([]result, error) {
	if cgen.HostTarget() == "" {
		return nil, errors.New("eye build doesn't target this machine")
	}
	work, err := os.MkdirTemp("", "eye-bench-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)
	launcher, err := buildLauncher(work)
	if err != nil {
		return nil, err
	}
	eye := ""
	if opts.Interp {
		eye = filepath.Join(work, "eye")
		fmt.Fprintln(opts.Log, "building eye")
		if err := goBuild(opts.Root, eye, "./cmd/eye"); err != nil {
			return nil, err
		}
	}
	var out []result
	for _, b := range benches {
		r, err := measureOne(b, opts, work, launcher, eye)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", b.Name, err)
		}
		out = append(out, r)
	}
	return out, nil
}

func measureOne(b benchmark, opts options, work, launcher, eye string) (result, error) {
	dir := filepath.Join(opts.Root, "testdata", "bench", b.Name)
	fmt.Fprintf(opts.Log, "%s: building\n", b.Name)
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		var msgs []string
		for _, d := range diags {
			msgs = append(msgs, fmt.Sprint(d))
		}
		return result{}, fmt.Errorf("the Eyelang program doesn't check:\n%s", strings.Join(msgs, "\n"))
	}
	native := filepath.Join(work, b.Name+".native")
	if err := cgen.Build(prog, cgen.BuildOptions{Out: native, Progress: opts.Log}); err != nil {
		return result{}, fmt.Errorf("eye build: %w", err)
	}
	gobin := filepath.Join(work, b.Name+".go")
	if err := goBuild(opts.Root, gobin, filepath.Join("testdata", "bench", b.Name, "main.go")); err != nil {
		return result{}, err
	}
	args := b.Args
	if opts.Quick {
		args = b.Quick
	}
	commands := map[string][]string{
		"native": append([]string{native}, args...),
		"go":     append([]string{gobin}, args...),
	}
	order := []string{"native", "go"}
	if eye != "" {
		commands["interp"] = append([]string{eye, "run", "--memory", dir, "--"}, args...)
		order = append(order, "interp")
	}
	times := map[string][]float64{}
	rss := map[string][]int64{}
	outputs := map[string]string{}
	fmt.Fprintf(opts.Log, "%s: running %d times\n", b.Name, opts.Runs)
	// The binaries take turns, so a slower stretch of the machine falls on
	// each of them alike.
	for i := 0; i < opts.Runs; i++ {
		for _, which := range order {
			if which == "interp" && i >= opts.InterpRuns {
				continue
			}
			ms, kb, stdout, err := runOnce(launcher, commands[which], opts.Root)
			if err != nil {
				return result{}, fmt.Errorf("%s: %w", which, err)
			}
			if prev, ok := outputs[which]; ok && prev != stdout {
				return result{}, fmt.Errorf("%s printed something else on run %d", which, i+1)
			}
			outputs[which] = stdout
			times[which] = append(times[which], ms)
			rss[which] = append(rss[which], kb)
		}
	}
	for _, which := range order[1:] {
		if outputs[which] != outputs["native"] {
			return result{}, fmt.Errorf("native code and %s print different results\n--- native\n%s--- %s\n%s",
				which, outputs["native"], which, outputs[which])
		}
	}
	r := result{
		Name:   b.Name,
		What:   b.What,
		Native: summarise(times["native"], rss["native"]),
		Go:     summarise(times["go"], rss["go"]),
	}
	if eye != "" {
		s := summarise(times["interp"], rss["interp"])
		r.Interp = &s
	}
	return r, nil
}

func summarise(times []float64, rss []int64) sample {
	ts := append([]float64(nil), times...)
	sort.Float64s(ts)
	rs := append([]int64(nil), rss...)
	sort.Slice(rs, func(i, j int) bool { return rs[i] < rs[j] })
	return sample{MinMS: ts[0], MedianMS: ts[len(ts)/2], RSSKB: rs[len(rs)/2]}
}

// goBuild builds a Go package or file into out, from the repository.
func goBuild(root, out, what string) error {
	cmd := exec.Command("go", "build", "-o", out, what)
	cmd.Dir = root
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go build %s: %v\n%s", what, err, b)
	}
	return nil
}

// findRoot is the repository's root: the nearest directory up from here
// with a go.mod.
func findRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "testdata", "bench")); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("run it inside the repository: no go.mod with testdata/bench above here")
		}
		dir = parent
	}
}
