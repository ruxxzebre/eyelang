package cgen

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"eyelang/check"
	"eyelang/interp"
	"eyelang/ir"
)

// buildHosted builds the runner with the C stand-in of
// hosted/testdata/counter linked in, through the path eye build --runner
// takes: zig compiles the C into an archive, and the pinned Go links it into
// a main that runs cgen/hosted/runner. The runner's source is this
// repository's, so the build reads its dependencies from the module cache.
func buildHosted(t *testing.T) string {
	t.Helper()
	needGo(t)
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	var files []File
	for _, name := range []string{"hosted/testdata/counter/counter.c", "hosted/eye_host.h"} {
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
	if err := writeTree(work, map[string]string{
		"go.mod":  gomod,
		"go.sum":  string(sum),
		"main.go": "package main\n\nimport (\n\t\"os\"\n\n\t\"eyelang/cgen/hosted/runner\"\n)\n\nfunc main() { os.Exit(runner.Main(os.Args, os.Stdout, os.Stderr)) }\n",
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
	return out
}

// TestHostedProgram runs a compiled program in the runner: Counter.add and
// the job tick run in C, through eye_host.h, while the runner keeps the
// store, the schedule and the steps. The process is killed while the job
// sleeps; started again, it resumes the job, which finishes and counts.
func TestHostedProgram(t *testing.T) {
	if _, ok := runnerTargets[HostTarget()]; !ok {
		t.Skip("the runner isn't built for this machine")
	}
	exe := buildHosted(t)
	t.Run("journal", func(t *testing.T) {
		dir := counterProgram(t)
		path := filepath.Join(dir, ".eye", "journal")
		hostedRestart(t, exe, dir, nil, func() interp.Store {
			st, err := interp.ReadJournal(path)
			if err != nil {
				t.Fatal(err)
			}
			return st
		})
	})
	t.Run("postgres", func(t *testing.T) {
		url := hostedPostgres(t)
		dir := counterProgram(t)
		hostedRestart(t, exe, dir, []string{"--store", url}, func() interp.Store {
			st, err := interp.ReadPostgres(url)
			if err != nil {
				t.Fatal(err)
			}
			return st
		})
	})
}

// counterProgram copies the program the stand-in compiles into a
// directory of its own, which keeps its journal.
func counterProgram(t *testing.T) string {
	dir := t.TempDir()
	src, err := os.ReadFile("hosted/testdata/counter/main.eye")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.eye"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// hostedRestart starts the program, kills it while its job sleeps, starts
// it again to finish the job, and checks the store after each.
func hostedRestart(t *testing.T, exe, dir string, flags []string, store func() interp.Store) {
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	args := append(append([]string{}, flags...), dir)

	// The first start: main starts tick, which sleeps two seconds.
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
	// the runner wakes tick when its two seconds are up.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	out, logs, err := runHosted(ctx, exe, args)
	if err != nil {
		t.Fatalf("started again: %v\n%s", err, logs)
	}
	if out != "count 0\nstarted\n" {
		t.Errorf("started again, it printed %q", out)
	}
	st = store()
	if run := onlyRun(t, prog, st); run.Status != "done" {
		t.Fatalf("tick is %s, not done: %s", run.Status, run.Fault)
	}
	// Counter.add ran in C and gave the store the state the interpreter
	// would: { id: "a", count: 1 }.
	row, ok := st.Load(".Counter/s1:a")
	want := "R\x02\x02ids\x01a\x05counti\x02"
	if !ok || string(row.State) != want {
		t.Errorf("Counter(a) is saved as %q, want %q", row.State, want)
	}

	// And main sees the count.
	if out, logs, err = runHosted(ctx, exe, args); err != nil || out != "count 1\nstarted\n" {
		t.Errorf("the third start printed %q (%v)\n%s", out, err, logs)
	}
}

// runHosted runs the program to its end, and returns what it printed and
// what it logged.
func runHosted(ctx context.Context, exe string, args []string) (string, string, error) {
	cmd := exec.CommandContext(ctx, exe, args...)
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
