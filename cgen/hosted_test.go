package cgen

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"eyelang/interp"
	"eyelang/ir"
)

// counterDir is the program the C stand-in compiles.
const counterDir = "hosted/testdata/counter"

// buildHosted builds the runner with the C stand-in of
// hosted/testdata/counter linked in, through the path eye build --runner
// takes: zig compiles the C into an archive, and the pinned Go links it into
// a main that runs cgen/hosted/runner, with the program's sources carried
// in the binary. The runner's source is this repository's, so the build
// reads its dependencies from the module cache.
func buildHosted(t *testing.T) (string, *ir.Program) {
	t.Helper()
	needGo(t)
	prog := load(t, counterDir)
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	var files []File
	for _, name := range []string{counterDir + "/counter.c", "hosted/eye_host.h"} {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, File{Name: filepath.Base(name), Src: src})
	}
	lib := filepath.Join(work, "libcounter.a")
	if err := Compile(files, BuildOptions{Out: lib, Archive: true}); err != nil {
		t.Fatal(err)
	}
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	// The repository's requirements, so the module graph is complete
	// without looking anything up.
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	gomod := strings.Replace(string(mod), "module eyelang", "module hostedtest", 1) +
		fmt.Sprintf("\nrequire eyelang v0.0.0\n\nreplace eyelang => %s\n", root)
	sources, err := ProgramSources(prog)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeTree(work, map[string]string{
		"go.mod":     gomod,
		"go.sum":     string(sum),
		"main.go":    "package main\n\nimport (\n\t\"os\"\n\n\t\"eyelang/cgen/hosted/runner\"\n)\n\nfunc main() { os.Exit(runner.Main(sources, os.Args, os.Stdout, os.Stderr)) }\n",
		"sources.go": goSources(sources),
	}); err != nil {
		t.Fatal(err)
	}
	cache, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		t.Skipf("no module cache to build the runner from: %v", err)
	}
	out := filepath.Join(work, "counter")
	env := []string{"GOMODCACHE=" + strings.TrimSpace(string(cache)), "GOFLAGS=-mod=readonly"}
	if err := goLink(work, out, HostTarget(), "eyehosted,netgo,osusergo", []string{lib, "-lm"}, env, nil); err != nil {
		t.Fatal(err)
	}
	return out, prog
}

// goSources writes the program's sources as a Go file of the main package.
func goSources(sources map[string]string) string {
	var paths []string
	for p := range sources {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var b strings.Builder
	b.WriteString("package main\n\n// sources are the program's files, which the runner checks when it starts.\nvar sources = map[string]string{\n")
	for _, p := range paths {
		fmt.Fprintf(&b, "\t%s: %s,\n", strconv.Quote(p), strconv.Quote(sources[p]))
	}
	b.WriteString("}\n")
	return b.String()
}

// TestHostedProgram runs a compiled program in the runner: main, the
// tests, Counter.add, Total.add and the job tick run in C, through
// eye_host.h, and call back into the runner, which keeps the store, the
// schedule and the steps. The process is killed while the job sleeps;
// started again, it resumes the job, which takes its host step, counts and
// finishes.
func TestHostedProgram(t *testing.T) {
	if _, ok := runnerTargets[HostTarget()]; !ok {
		t.Skip("the runner isn't built for this machine")
	}
	exe, prog := buildHosted(t)
	t.Run("journal", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "journal")
		hostedRestart(t, exe, prog, []string{"--journal", path}, func() interp.Store {
			st, err := interp.ReadJournal(path)
			if err != nil {
				t.Fatal(err)
			}
			return st
		})
	})
	t.Run("postgres", func(t *testing.T) {
		url := hostedPostgres(t)
		hostedRestart(t, exe, prog, []string{"--store", url}, func() interp.Store {
			st, err := interp.ReadPostgres(url)
			if err != nil {
				t.Fatal(err)
			}
			return st
		})
	})
	t.Run("tests", func(t *testing.T) {
		// The C tests call Counter.add, which calls Total.add in its
		// transaction, read Total's snapshot and expect through the runner.
		out, _, err := runHosted(context.Background(), exe, []string{"--test"}, nil)
		want := "ok    counts and totals\n" +
			"FAIL  a failing expect stops the test\n" +
			"      /eye-program/main.eye:49:1: expectation failed\n" +
			"1 of 2 tests failed\n"
		if err == nil || out != want {
			t.Errorf("--test printed %q (%v), want %q", out, err, want)
		}
		if out, _, err := runHosted(context.Background(), exe, []string{"--test", "--run", "totals"}, nil); err != nil || out != "ok    counts and totals\n1 tests passed\n" {
			t.Errorf("--test --run totals printed %q (%v)", out, err)
		}
	})
}

// hostedRestart starts the program, kills it while its job sleeps, starts
// it again to finish the job, and checks the store after each.
func hostedRestart(t *testing.T, exe string, prog *ir.Program, args []string, store func() interp.Store) {
	// The first start: main, in C, reads Counter(a), starts tick, which
	// sleeps two seconds, and prints.
	cmd := exec.Command(exe, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	lines := bufio.NewScanner(stdout)
	var printed []string
	for lines.Scan() {
		printed = append(printed, lines.Text())
		if lines.Text() == "started" {
			break
		}
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	if strings.Join(printed, "\n") != "count 0\nstarted" {
		t.Fatalf("the first start printed %q\n%s", printed, stderr.String())
	}
	st := store()
	if run := onlyRun(t, prog, st); run.Status != "sleeping" {
		t.Fatalf("killed while it sleeps, tick is %s, not sleeping", run.Status)
	}
	// It sleeps at a native checkpoint, which only the compiled job makes.
	if data, _ := st.Run(1); !bytes.Contains(data, []byte(`"Native":{"Frame"`)) {
		t.Errorf("tick's record has no native checkpoint: %s", data)
	}

	// Started again: main starts nothing new (tick's key is taken), and
	// the runner wakes tick when its two seconds are up. tick reads TICK
	// with a host step, then calls Counter.add, which publishes and calls
	// Total.add in its transaction.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	out, logs, err := runHosted(ctx, exe, args, []string{"TICK=on"})
	if err != nil {
		t.Fatalf("started again: %v\n%s", err, logs)
	}
	if out != "count 0\nstarted\n" {
		t.Errorf("started again, it printed %q", out)
	}
	st = store()
	run := onlyRun(t, prog, st)
	if run.Status != "done" {
		t.Fatalf("tick is %s, not done: %s", run.Status, run.Fault)
	}
	var calls []string
	for _, s := range run.Steps {
		calls = append(calls, s.Call)
	}
	if got := strings.Join(calls, ", "); !strings.Contains(got, "env.get") || !strings.Contains(got, "Counter(a).add") {
		t.Errorf("tick's steps are %s, without env.get and Counter(a).add", got)
	}
	// The C methods gave the store the states the interpreter would.
	for key, want := range map[string]string{
		".Counter/s1:a": "R\x02\x02ids\x01a\x05counti\x02",
		".Total/s3:all": "R\x02\x02ids\x03all\x03sumi\x02",
	} {
		if row, ok := st.Load(key); !ok || string(row.State) != want {
			t.Errorf("%s is saved as %q, want %q", key, row.State, want)
		}
	}
	published := false
	for _, e := range st.Events() {
		published = published || e.Name == "bus.counted" && string(e.Data) == "N\x0bbus.Counted\x02\x02ids\x01a\x05counti\x02"
	}
	if !published {
		t.Errorf("Counter.add published no bus.counted { id: \"a\", count: 1 }: %v", st.Events())
	}

	// And main sees the count.
	if out, logs, err = runHosted(ctx, exe, args, nil); err != nil || out != "count 1\nstarted\n" {
		t.Errorf("the third start printed %q (%v)\n%s", out, err, logs)
	}
}

// runHosted runs the program to its end, and returns what it printed and
// what it logged.
func runHosted(ctx context.Context, exe string, args, env []string) (string, string, error) {
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Env = append(os.Environ(), env...)
	var out, logs bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &logs
	err := cmd.Run()
	return out.String(), logs.String(), err
}

// onlyRun is the store's one run, as eye runs shows it.
func onlyRun(t *testing.T, prog *ir.Program, st interp.Store) interp.RunInfo {
	t.Helper()
	runs := interp.Runs(prog, st)
	if len(runs) != 1 {
		t.Fatalf("the store has %d runs, want 1", len(runs))
	}
	return runs[0]
}

// hostedPostgres gives the test a schema of its own in the database
// $EYE_TEST_POSTGRES names, dropped when the test ends, and returns the
// address of a store in it. Without the variable the test is skipped.
func hostedPostgres(t *testing.T) string {
	t.Helper()
	base := os.Getenv("EYE_TEST_POSTGRES")
	if base == "" {
		t.Skip("set EYE_TEST_POSTGRES to a Postgres address to test the Postgres store")
	}
	schema := fmt.Sprintf("eyetest_hosted_%d", time.Now().UnixNano())
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn, err := pgx.Connect(ctx, base)
		if err != nil {
			return
		}
		defer conn.Close(ctx)
		_, _ = conn.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	})
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + "search_path=" + schema
}
