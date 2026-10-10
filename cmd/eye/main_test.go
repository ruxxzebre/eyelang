package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
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

func TestCheck(t *testing.T) {
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
	if code := run([]string{"check", filepath.Join(dir, "good.eye")}, &stdout, &stderr); code != 0 || stdout.Len() > 0 {
		t.Errorf("check of a good file: exit %d, output %q %q", code, stdout.String(), stderr.String())
	}

	t.Chdir(dir)
	stdout.Reset()
	if code := run([]string{"check"}, &stdout, &stderr); code != exitErrors {
		t.Errorf("check exit code = %d, want %d", code, exitErrors)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], filepath.Join("sub", "bad.eye")+":1:") || !strings.HasPrefix(lines[1], filepath.Join("sub", "bad.eye")+":2:") {
		t.Errorf("check reported %q, want two errors in sub/bad.eye, in order", stdout.String())
	}

	stderr.Reset()
	if code := run([]string{"check", "missing.eye"}, &stdout, &stderr); code != exitErrors || !strings.Contains(stderr.String(), "missing.eye") {
		t.Errorf("check of a missing file: exit %d, stderr %q", code, stderr.String())
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

func TestUsageListsEveryCommand(t *testing.T) {
	var out bytes.Buffer
	usage(&out)
	for _, c := range commands {
		if !strings.Contains(out.String(), c.name) {
			t.Errorf("usage does not mention %q:\n%s", c.name, out.String())
		}
	}
}
