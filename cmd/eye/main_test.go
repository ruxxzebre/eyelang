package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type runCase struct {
	name       string
	args       []string
	wantCode   int
	wantStdout string
	wantStderr string
}

func TestRun(t *testing.T) {
	tests := []runCase{
		{name: "no arguments shows usage", args: nil, wantCode: 0, wantStdout: "usage: eye <command>"},
		{name: "help", args: []string{"help"}, wantCode: 0, wantStdout: "usage: eye <command>"},
		{name: "--help", args: []string{"--help"}, wantCode: 0, wantStdout: "usage: eye <command>"},
		{name: "version", args: []string{"version"}, wantCode: 0, wantStdout: "eye 0.0.0-dev"},
		{name: "unknown command", args: []string{"frobnicate"}, wantCode: exitUsage, wantStderr: `unknown command "frobnicate"`},
	}
	for _, c := range commands {
		if c.run != nil {
			continue
		}
		tests = append(tests, runCase{
			name:       c.name + " is not implemented yet",
			args:       []string{c.name, "file.eye"},
			wantCode:   exitUsage,
			wantStderr: "eye " + c.name + ": not implemented yet",
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestParse(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.eye")
	bad := filepath.Join(dir, "bad.eye")
	if err := os.WriteFile(good, []byte("let a = 1 + 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("let a = (1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []runCase{
		{name: "tree", args: []string{"parse", good}, wantCode: 0,
			wantStdout: "(source_file\n  (let_statement\n    pattern: (identifier \"a\")"},
		{name: "sexp", args: []string{"parse", "--sexp", good}, wantCode: 0,
			wantStdout: `(source_file (let_statement "let" pattern: (identifier "a") "=" value: (binary_expression left: (integer "1") "+" right: (integer "2"))))`},
		{name: "syntax error", args: []string{"parse", bad}, wantCode: exitErrors,
			wantStderr: "bad.eye:1:11: expected ')'"},
		{name: "missing file", args: []string{"parse", filepath.Join(dir, "none.eye")}, wantCode: exitErrors,
			wantStderr: "no such file"},
		{name: "no files", args: []string{"parse"}, wantCode: exitUsage, wantStderr: "usage: eye parse"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, tt.wantCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestFmt(t *testing.T) {
	dir := t.TempDir()
	messy := filepath.Join(dir, "messy.eye")
	tidy := filepath.Join(dir, "tidy.eye")
	broken := filepath.Join(dir, "broken.eye")
	write := func(path, src string) {
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(messy, "fn  f( x:Int )=x+1\n")
	write(tidy, "fn f(x: Int) = x + 1\n")
	write(broken, "fn f( {\n")

	var stdout, stderr bytes.Buffer
	if code := run([]string{"fmt", "--check", dir}, &stdout, &stderr); code != exitErrors {
		t.Errorf("fmt --check exit code = %d, want %d", code, exitErrors)
	}
	if !strings.Contains(stdout.String(), "messy.eye") || strings.Contains(stdout.String(), "tidy.eye") {
		t.Errorf("fmt --check listed %q, want only messy.eye", stdout.String())
	}
	if !strings.Contains(stderr.String(), "broken.eye:1:7") {
		t.Errorf("fmt --check stderr = %q, want the syntax error in broken.eye", stderr.String())
	}
	if data, _ := os.ReadFile(messy); string(data) != "fn  f( x:Int )=x+1\n" {
		t.Errorf("fmt --check changed messy.eye")
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"fmt", messy, tidy}, &stdout, &stderr); code != 0 {
		t.Errorf("fmt exit code = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if data, _ := os.ReadFile(messy); string(data) != "fn f(x: Int) = x + 1\n" {
		t.Errorf("fmt wrote %q", data)
	}

	stdin = strings.NewReader("let  a=1\n")
	defer func() { stdin = os.Stdin }()
	stdout.Reset()
	if code := run([]string{"fmt"}, &stdout, &stderr); code != 0 || stdout.String() != "let a = 1\n" {
		t.Errorf("fmt from stdin: exit %d, output %q", code, stdout.String())
	}
}

func TestCheckSyntax(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, src := range map[string]string{
		"good.eye":       "let a = 1\n",
		"sub/bad.eye":    "let a = (1\nlet b = 2 +\n",
		"sub/ignore.txt": "not eyelang {",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"check", "--syntax", filepath.Join(dir, "good.eye")}, &stdout, &stderr); code != 0 || stdout.Len() > 0 {
		t.Errorf("check of a good file: exit %d, output %q %q", code, stdout.String(), stderr.String())
	}

	t.Chdir(dir)
	stdout.Reset()
	if code := run([]string{"check", "--syntax", "."}, &stdout, &stderr); code != exitErrors {
		t.Errorf("check exit code = %d, want %d", code, exitErrors)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], filepath.Join("sub", "bad.eye")+":1:") || !strings.HasPrefix(lines[1], filepath.Join("sub", "bad.eye")+":2:") {
		t.Errorf("check reported %q, want two errors in sub/bad.eye, in order", stdout.String())
	}

	stderr.Reset()
	if code := run([]string{"check", "--syntax", "missing.eye"}, &stdout, &stderr); code != exitErrors || !strings.Contains(stderr.String(), "missing.eye") {
		t.Errorf("check of a missing file: exit %d, stderr %q", code, stderr.String())
	}
}

// program writes a program's files into a new directory and returns it.
func program(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, src := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCheckProgram(t *testing.T) {
	good := program(t, map[string]string{"a.eye": "fn double(n) = n * 2\n", "b.eye": "let four = double(2)\n"})
	bad := program(t, map[string]string{"a.eye": "fn f() = missing(1)\n\njob j() {\n  sleep(1 min)\n}\n"})

	var stdout, stderr bytes.Buffer
	if code := run([]string{"check", good}, &stdout, &stderr); code != 0 {
		t.Errorf("check of a good program: exit %d, %s%s", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	if code := run([]string{"check", filepath.Join(bad, "a.eye")}, &stdout, &stderr); code != exitErrors {
		t.Errorf("check of a bad program: exit %d", code)
	}
	for _, want := range []string{"a.eye:1:10: undefined name missing", "a.eye:4:3: in a job, sleep is a durable step: write sleep!(…)"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("check reported %q, want %q", stdout.String(), want)
		}
	}
}

func TestTestCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"test", "../../testdata/programs/tour"}, &stdout, &stderr); code != 0 {
		t.Fatalf("eye test of the tour: exit %d\n%s%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok    an idle world sleeps after 20 minutes") || !strings.Contains(stdout.String(), "3 tests passed") {
		t.Errorf("eye test printed %q", stdout.String())
	}

	failing := program(t, map[string]string{"t.eye": "test \"arithmetic\" {\n  log.info(\"checking\")\n  expect(1 + 1 == 3)\n}\n"})
	stdout.Reset()
	if code := run([]string{"test", failing}, &stdout, &stderr); code != exitErrors {
		t.Errorf("a failing test: exit %d", code)
	}
	for _, want := range []string{"FAIL  arithmetic", "t.eye:3:3: expected 1 + 1 == 3, but 1 + 1 is 2", "info: checking", "1 of 1 tests failed"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("eye test printed %q, want %q", stdout.String(), want)
		}
	}

	stdout.Reset()
	if code := run([]string{"test", "--run", "nothing matches this", "../../testdata/programs/tour"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "no tests to run") {
		t.Errorf("eye test --run: exit %d, %q", code, stdout.String())
	}
}

func TestRunCommand(t *testing.T) {
	dir := program(t, map[string]string{"main.eye": `pub fn main(args: List(String)) -> Result((), String) uses {stdio} {
  let names = args |> drop(1)
  if names |> isEmpty { return Err("who?") }
  for name in names { print("hello, {name}") }
  Ok(())
}
`})
	var stdout, stderr bytes.Buffer
	if code := run([]string{"run", dir, "--", "Ann", "Bo"}, &stdout, &stderr); code != 0 || stdout.String() != "hello, Ann\nhello, Bo\n" {
		t.Errorf("eye run: exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	if code := run([]string{"run", dir}, &stdout, &stderr); code != exitErrors || !strings.Contains(stderr.String(), "who?") {
		t.Errorf("eye run returning Err: exit %d, stderr %q", code, stderr.String())
	}
}

func TestLSP(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	exit := `{"jsonrpc":"2.0","method":"exit"}`
	stdin = strings.NewReader(fmt.Sprintf("Content-Length: %d\r\n\r\n%sContent-Length: %d\r\n\r\n%s", len(body), body, len(exit), exit))
	defer func() { stdin = os.Stdin }()
	var stdout, stderr bytes.Buffer
	if code := run([]string{"lsp"}, &stdout, &stderr); code != 0 {
		t.Fatalf("lsp exit code = %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"semanticTokensProvider"`) {
		t.Errorf("lsp didn't answer initialize: %q", stdout.String())
	}
}

// TestLSPProbe answers one definition and one references request from the
// command line, in a program of two modules.
func TestLSPProbe(t *testing.T) {
	dir := t.TempDir()
	for name, src := range map[string]string{
		"eye.toml":          "",
		"main.eye":          "import shapes\n\nfn main() {\n  print(shapes.size())\n}\n",
		"shapes/shapes.eye": "pub fn size() -> Int = 2\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	cases := []struct{ args, want string }{
		{"--definition main.eye:4:17", "shapes/shapes.eye:1:8\n"},
		{"--references shapes/shapes.eye:1:8", "main.eye:4:16\nshapes/shapes.eye:1:8\n"},
		{"--definition main.eye:2:1", "nothing\n"},
	}
	for _, c := range cases {
		var stdout, stderr bytes.Buffer
		if code := run(append([]string{"lsp"}, strings.Fields(c.args)...), &stdout, &stderr); code != 0 {
			t.Fatalf("eye lsp %s: exit %d (stderr: %s)", c.args, code, stderr.String())
		}
		lines := strings.SplitAfter(stdout.String(), "\n")
		sort.Strings(lines)
		if got := strings.Join(lines, ""); got != c.want {
			t.Errorf("eye lsp %s:\n%s\nwant:\n%s", c.args, got, c.want)
		}
	}
	for at, want := range map[string]string{
		"main.eye:4":    "FILE:LINE:COLUMN",
		"main.eye:99:1": "has 6 lines",
		"main.eye:4:40": "line 4 has 22 characters",
	} {
		var stdout, stderr bytes.Buffer
		if code := run([]string{"lsp", "--definition", at}, &stdout, &stderr); code != exitErrors ||
			!strings.Contains(stderr.String(), want) {
			t.Errorf("--definition %s: exit %d, stderr %q, want %q", at, code, stderr.String(), want)
		}
	}
}

func TestUsageListsEveryCommand(t *testing.T) {
	var out bytes.Buffer
	usage(&out)
	for _, c := range commands {
		if !strings.Contains(out.String(), c.name) {
			t.Errorf("usage does not mention %q:\n%s", c.name, out.String())
		}
	}
}
