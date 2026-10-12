package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestQuick builds every benchmark natively and with Go and runs each, and
// the interpreter, once at a small size: they must print the same. It
// measures nothing; timing is for `go run ./tools/bench --check` on a
// quiet machine.
func TestQuick(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles C with zig and builds eye")
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--quick"}, &stdout, &stderr); code != 0 {
		t.Fatalf("bench --quick exited with %d:\n%s%s", code, stdout.String(), stderr.String())
	}
	for _, b := range suite {
		if !strings.Contains(stdout.String(), "ok    "+b.Name+"\n") {
			t.Errorf("no result for %s:\n%s", b.Name, stdout.String())
		}
	}
}

// TestSuite checks that every directory under testdata/bench is in the
// suite, and every benchmark in it has its two programs.
func TestSuite(t *testing.T) {
	root, err := findRoot()
	if err != nil {
		t.Fatal(err)
	}
	inSuite := map[string]bool{}
	for _, b := range suite {
		inSuite[b.Name] = true
		dir := filepath.Join(root, "testdata", "bench", b.Name)
		eyes, _ := filepath.Glob(filepath.Join(dir, "*.eye"))
		if len(eyes) == 0 {
			t.Errorf("%s has no Eyelang program", b.Name)
		}
		if _, err := os.Stat(filepath.Join(dir, "main.go")); err != nil {
			t.Errorf("%s has no main.go", b.Name)
		}
		if len(b.Quick) == 0 {
			t.Errorf("%s has no quick arguments", b.Name)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, "testdata", "bench"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() && !inSuite[e.Name()] {
			t.Errorf("testdata/bench/%s isn't in tools/bench's suite", e.Name())
		}
	}
}

// TestCompare checks what --check calls a regression.
func TestCompare(t *testing.T) {
	rec := recorded{environment: environment{Date: "2026-10-12", Commit: "abc"}}
	for _, name := range []string{"steady", "slower", "larger", "faster"} {
		rec.Benchmarks = append(rec.Benchmarks, recordedResult{Name: name, Native: sample{RSSKB: 50_000}, TimeRatio: 2, RSSRatio: 0.5})
	}
	rec.Benchmarks = append(rec.Benchmarks, recordedResult{Name: "small", Native: sample{RSSKB: 150}, TimeRatio: 2, RSSRatio: 0.075})
	res := func(name string, nativeMS float64, nativeKB, goKB int64) result {
		return result{Name: name, Native: sample{MinMS: nativeMS, RSSKB: nativeKB}, Go: sample{MinMS: 100, RSSKB: goKB}}
	}
	cases := []struct {
		result result
		ok     bool
		want   string
	}{
		{res("steady", 240, 56_000, 100_000), true, "ok\n"},
		{res("slower", 260, 50_000, 100_000), false, "time 2.00× → 2.60× WORSE"},
		{res("larger", 200, 60_000, 100_000), false, "memory 0.50× → 0.60× WORSE"},
		{res("faster", 100, 50_000, 100_000), true, "time 2.00× → 1.00× better"},
		{res("small", 200, 180, 2000), true, "ok\n"}, // 30 KB more is no regression
		{res("new", 100, 500, 1000), true, "new         not recorded"},
	}
	for _, c := range cases {
		report, failed := compare(rec, []result{c.result})
		if (len(failed) == 0) != c.ok || !strings.Contains(report, c.want) {
			t.Errorf("%s: failed %v, want ok %v, with %q in:\n%s", c.result.Name, failed, c.ok, c.want, report)
		}
	}
}

// TestBest checks that a retry keeps the better time and memory.
func TestBest(t *testing.T) {
	a := result{Native: sample{MinMS: 300, RSSKB: 100}, Go: sample{MinMS: 100, RSSKB: 100}}
	b := result{Native: sample{MinMS: 200, RSSKB: 200}, Go: sample{MinMS: 100, RSSKB: 100}}
	got := best(a, b)
	if got.TimeRatio() != 2 || got.RSSRatio() != 1 {
		t.Errorf("best: time %v, memory %v; want 2 and 1", got.TimeRatio(), got.RSSRatio())
	}
}
