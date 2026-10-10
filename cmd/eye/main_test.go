package main

import (
	"bytes"
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

func TestUsageListsEveryCommand(t *testing.T) {
	var out bytes.Buffer
	usage(&out)
	for _, c := range commands {
		if !strings.Contains(out.String(), c.name) {
			t.Errorf("usage does not mention %q:\n%s", c.name, out.String())
		}
	}
}
