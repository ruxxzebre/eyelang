package main

import (
	"bytes"
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

func TestUsageListsEveryCommand(t *testing.T) {
	var out bytes.Buffer
	usage(&out)
	for _, c := range commands {
		if !strings.Contains(out.String(), c.name) {
			t.Errorf("usage does not mention %q:\n%s", c.name, out.String())
		}
	}
}
