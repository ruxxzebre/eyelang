package cgen

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"eyelang/check"
	"eyelang/interp"
	"eyelang/ir"
	"eyelang/lir"
)

// load checks the program in dir.
func load(t *testing.T, dir string) *ir.Program {
	t.Helper()
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		t.Fatalf("%s doesn't check: %v", dir, diags)
	}
	return prog
}

// needZig skips a test that compiles C in -short mode. Otherwise it runs,
// and fetches the pinned zig the first time, as CI does.
func needZig(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("compiles C with zig")
	}
}

// build compiles the program in dir into a binary in a temporary
// directory, for the target given ("" for this machine).
func build(t *testing.T, prog *ir.Program, target string, tests, debug bool) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "prog")
	if err := Build(prog, BuildOptions{Out: out, Target: target, Tests: tests, Debug: debug}); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(target, "windows/") {
		out += ".exe"
	}
	return out
}

var durations = regexp.MustCompile(` \([0-9.]+(µs|ms|s|m[0-9.]+s|h[0-9m.]+s)\)$`)

// interpreted is what `eye test -v` writes for the program, without the
// tests' durations.
func interpreted(prog *ir.Program) string {
	var b strings.Builder
	indent := func(s string) {
		for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
			b.WriteString("      " + line + "\n")
		}
	}
	results := interp.RunTests(prog, "")
	failed := 0
	for _, r := range results {
		if r.Failure == "" {
			fmt.Fprintf(&b, "ok    %s\n", r.Name)
		} else {
			failed++
			fmt.Fprintf(&b, "FAIL  %s\n", r.Name)
			indent(r.Failure)
			b.WriteString("\n")
		}
		if r.Log != "" {
			indent(r.Log)
		}
	}
	switch {
	case len(results) == 0:
		b.WriteString("no tests to run\n")
	case failed > 0:
		fmt.Fprintf(&b, "%d of %d tests failed\n", failed, len(results))
	default:
		fmt.Fprintf(&b, "%d tests passed\n", len(results))
	}
	return b.String()
}

// native runs a test binary with -v and gives its output without the
// durations.
func native(t *testing.T, exe string) string {
	t.Helper()
	cmd := exec.Command(exe, "-v")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		var exit *exec.ExitError
		if err != nil && !errors.As(err, &exit) {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Minute):
		_ = cmd.Process.Kill()
		t.Fatalf("%s didn't finish:\n%s", exe, out.String())
	}
	lines := strings.Split(out.String(), "\n")
	for i, l := range lines {
		l = durations.ReplaceAllString(l, "")
		if strings.HasSuffix(l, ")") && strings.Contains(l, " tests passed (") {
			l = l[:strings.Index(l, " (")]
		}
		lines[i] = l
	}
	return strings.Join(lines, "\n")
}

// compilable checks every program in testdata/programs and cgen/testdata
// and gives those whose C eye build generates, by directory: a program
// joins the cross-check as soon as it compiles. The ones that must be
// among them are listed, so that a program that stops compiling fails
// instead of dropping out quietly.
func compilable(t *testing.T) map[string]*ir.Program {
	t.Helper()
	var dirs []string
	for _, pattern := range []string{"../testdata/programs/*", "testdata/*"} {
		found, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range found {
			if info, err := os.Stat(d); err == nil && info.IsDir() {
				dirs = append(dirs, d)
			}
		}
	}
	out := map[string]*ir.Program{}
	for _, dir := range dirs {
		prog := load(t, dir)
		low, errs := lir.Lower(prog)
		if len(errs) > 0 {
			continue
		}
		if _, err := Generate(prog, low, Options{Tests: true}); err != nil {
			continue
		}
		out[dir] = prog
	}
	for _, must := range []string{
		"../testdata/programs/core",
		"../testdata/programs/traits",
		"../testdata/programs/lexer",
		"testdata/failures",
		"testdata/hello",
		"testdata/hosts",
		"testdata/inplace",
		"testdata/numbers",
		"testdata/shows",
		"testdata/times",
		"testdata/updates",
	} {
		if out[must] == nil {
			t.Errorf("eye build no longer compiles %s", must)
		}
	}
	return out
}

// TestTestsPassAsInterpreted builds the tests of every program eye build
// compiles as native binaries for this machine, at both levels (-O0 -g, as
// eye build -g and the debugger use, and -O2, as releases ship), and runs
// them: each test passes or fails as under eye test, with the same messages
// and the same output, line for line. The core language's program is ROADMAP
// 7.4's measure; both levels are 7.8's.
func TestTestsPassAsInterpreted(t *testing.T) {
	needZig(t)
	if HostTarget() == "" {
		t.Skip("eye build doesn't target this machine")
	}
	for dir, prog := range compilable(t) {
		want := sync.OnceValue(func() string { return interpreted(prog) })
		name := filepath.Base(dir)
		for _, level := range []struct {
			name  string
			debug bool
		}{{"O0", true}, {"O2", false}} {
			t.Run(name+"/"+level.name, func(t *testing.T) {
				t.Parallel()
				got := native(t, build(t, prog, "", true, level.debug))
				if got != want() {
					t.Errorf("the native tests at -%s differ from eye test's\n%s", level.name, lineDiff(got, want()))
				}
			})
		}
	}
}

// lineDiff names the first line where the native output and eye test's
// differ, and shows both in full.
func lineDiff(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	first := min(len(g), len(w))
	for i := 0; i < len(g) && i < len(w); i++ {
		if g[i] != w[i] {
			first = i
			break
		}
	}
	return fmt.Sprintf("line %d first differs\n--- native\n%s\n--- eye test\n%s", first+1, got, want)
}

// TestMain runs a program's main: its output, and an Err from main on
// stderr with exit status 1.
func TestMainBinary(t *testing.T) {
	needZig(t)
	if HostTarget() == "" {
		t.Skip("eye build doesn't target this machine")
	}
	exe := build(t, load(t, "testdata/hello"), "", false, false)
	run := func(args ...string) (string, string, int) {
		cmd := exec.Command(exe, args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		code := 0
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return stdout.String(), stderr.String(), code
	}
	out, errOut, code := run("a")
	if out != "hello from 2 argument(s)\n0: a\n" || errOut != "" || code != 0 {
		t.Errorf("with one argument: stdout %q, stderr %q, exit %d", out, errOut, code)
	}
	out, errOut, code = run("a", "b")
	if out != "hello from 3 argument(s)\n0: a\n1: b\n" || errOut != "tired after 3 h\n" || code != 1 {
		t.Errorf("with two arguments: stdout %q, stderr %q, exit %d", out, errOut, code)
	}
}

// TestLexerBinary builds testdata/programs/lexer, Eyelang's lexer in
// Eyelang, and runs it on the conformance suite: the compiled lexer prints
// the tokens the interpreted one does, which interp's lexer_test.go checks
// against syntax/lexer.go.
func TestLexerBinary(t *testing.T) {
	needZig(t)
	if HostTarget() == "" {
		t.Skip("eye build doesn't target this machine")
	}
	files, err := filepath.Glob("../testdata/syntax/*/*.eye")
	if err != nil || len(files) < 100 {
		t.Fatalf("found %d files in testdata/syntax: %v", len(files), err)
	}
	prog := load(t, "../testdata/programs/lexer")
	var want, errs bytes.Buffer
	if code := interp.Run(context.Background(), prog, interp.RunOptions{
		Args: append([]string{"lexer"}, files...), Stdout: &want, Stderr: &errs,
	}); code != 0 {
		t.Fatalf("the interpreted lexer exited with %d: %s", code, errs.String())
	}
	cmd := exec.Command(build(t, prog, "", false, false), files...)
	var got bytes.Buffer
	cmd.Stdout, cmd.Stderr = &got, &errs
	if err := cmd.Run(); err != nil {
		t.Fatalf("the compiled lexer: %v: %s", err, errs.String())
	}
	if got.String() != want.String() {
		g, w := strings.Split(got.String(), "\n"), strings.Split(want.String(), "\n")
		for i := 0; i < len(g) && i < len(w); i++ {
			if g[i] != w[i] {
				t.Fatalf("line %d is %q compiled, %q interpreted", i+1, g[i], w[i])
			}
		}
		t.Fatalf("the compiled lexer printed %d lines, the interpreted one %d", len(g), len(w))
	}
}

// TestCrossCompile builds the core language's tests for every release
// target: zig cross-compiles and links each, and the binary is the
// target's kind of executable.
func TestCrossCompile(t *testing.T) {
	needZig(t)
	prog := load(t, "../testdata/programs/core")
	magic := map[string][]byte{
		"linux":   []byte("\x7fELF"),
		"darwin":  {0xcf, 0xfa, 0xed, 0xfe},
		"windows": []byte("MZ"),
	}
	for _, target := range TargetNames() {
		t.Run(strings.ReplaceAll(target, "/", "-"), func(t *testing.T) {
			t.Parallel()
			exe := build(t, prog, target, true, false)
			data, err := os.ReadFile(exe)
			if err != nil {
				t.Fatal(err)
			}
			want := magic[strings.Split(target, "/")[0]]
			if !bytes.HasPrefix(data, want) {
				t.Errorf("%s starts with % x, not % x", exe, data[:min(len(data), 4)], want)
			}
		})
	}
}

// generate writes the C of the program in dir.
func generate(t *testing.T, dir string, opts Options) (map[string]string, error) {
	t.Helper()
	prog := load(t, dir)
	low, errs := lir.Lower(prog)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	files, err := Generate(prog, low, opts)
	out := map[string]string{}
	for _, f := range files {
		out[f.Name] = string(f.Src)
	}
	return out, err
}

// TestLineDirectives checks that the C points at the .eye lines it comes
// from, so that debuggers and crash reports name them.
func TestLineDirectives(t *testing.T) {
	files, err := generate(t, "../testdata/programs/core", Options{Tests: true})
	if err != nil {
		t.Fatal(err)
	}
	c := files["m_main.c"]
	if c == "" {
		t.Fatalf("no m_main.c among %d files", len(files))
	}
	for _, want := range []string{`values.eye"`, `strings.eye"`, `iter.eye"`} {
		if !regexp.MustCompile(`(?m)^#line [0-9]+ "[^"]*` + regexp.QuoteMeta(want)).MatchString(c) {
			t.Errorf("m_main.c has no #line directive for %s", want)
		}
	}
	if _, ok := files["m_tally.c"]; !ok {
		t.Errorf("the module tally has no file of its own: %v", keys(files))
	}
}

func keys(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestInPlaceUpdates checks that a variable updated by itself is changed by
// one call that may work in place, not by a copy.
func TestInPlaceUpdates(t *testing.T) {
	files, err := generate(t, "testdata/inplace", Options{Tests: true})
	if err != nil {
		t.Fatal(err)
	}
	c := files["m_main.c"]
	for _, call := range []string{"eyo_str_append(", "eyo_concat(", "eyo_list_concat(", "eyo_list_add(", "eyo_set_add(", "eyo_map_insert("} {
		if !strings.Contains(c, call) {
			t.Errorf("the C has no call of %s", call)
		}
	}
}

// TestUnsupported checks what eye build says of a program that uses what the
// C back end doesn't compile yet.
func TestUnsupported(t *testing.T) {
	_, err := generate(t, "../testdata/programs/entities", Options{Tests: true})
	if err == nil {
		t.Fatal("the C of a program with entities was generated")
	}
	for _, want := range []string{"entities.eye:", "the program declares entities", "doesn't compile yet", "eye test"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error doesn't say %q:\n%v", want, err)
		}
	}
}
