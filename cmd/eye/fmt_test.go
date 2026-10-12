package main

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEyeFilesSkipsUnrelatedDirectories(t *testing.T) {
	dir := program(t, map[string]string{
		"main.eye":                       "let a = 1\n",
		"sub/main.eye":                   "let b = 2\n",
		".claude/worktrees/old/main.eye": "let c = 3\n",
		"sub/.eye/main.eye":              "let d = 4\n",
		"sub/node_modules/pkg/main.eye":  "let e = 5\n",
	})
	for _, tt := range []struct {
		name string
		path string
		want []string
	}{
		{"recursive", dir, []string{"main.eye", "sub/main.eye"}},
		{"explicit hidden directory", filepath.Join(dir, ".claude"), []string{".claude/worktrees/old/main.eye"}},
		{"explicit dependency directory", filepath.Join(dir, "sub/node_modules"), []string{"sub/node_modules/pkg/main.eye"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			files, inDirs, err := eyeFiles([]string{tt.path})
			if err != nil {
				t.Fatal(err)
			}
			want := make([]string, len(tt.want))
			for i, path := range tt.want {
				want[i] = filepath.Join(dir, path)
			}
			if !reflect.DeepEqual(files, want) || !reflect.DeepEqual(inDirs, want) {
				t.Fatalf("files = %v, inDirs = %v; want %v", files, inDirs, want)
			}
		})
	}
}

func TestFmtStats(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // the saved results' directory
	t.Setenv("XDG_CACHE_HOME", "")
	dir := program(t, map[string]string{
		"main.eye":  "fn  f( x:Int )=x+1\n",
		"other.eye": "fn g(x: Int) = x\n\nfn h(x: Int) = x\n",
	})
	fmtStats := func(args ...string) (int, string, string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := run(append([]string{"fmt"}, args...), &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}

	// Unasked, nothing is added for a script to read.
	if code, stdout, stderr := fmtStats("--check", dir); code != exitErrors || stderr != "" || !strings.HasSuffix(stdout, "main.eye\n") {
		t.Fatalf("fmt --check: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr := fmtStats("--check", "--cold", "--stats", dir)
	if code != exitErrors || !strings.HasSuffix(stdout, "main.eye\n") {
		t.Fatalf("fmt --check --stats: exit %d, stdout %q", code, stdout)
	}
	for _, want := range []string{
		"eye fmt: checked 2 files, 4 lines in ", "; 1 not formatted\n",
		"input         1 path, 2 files, 4 lines, 54 B\n",
		"cache         off: --cold ignores saved results\n",
		"check ", "1 program, ",
		"slowest 2 to format:\n",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("fmt --check --stats stderr lacks %q:\n%s", want, stderr)
		}
	}

	// A saved result reports the counts of the run that saved it.
	fmtStats("--check", dir)
	code, stdout, stderr = fmtStats("--check", "--stats", dir)
	if code != exitErrors || !strings.HasSuffix(stdout, "main.eye\n") {
		t.Fatalf("saved fmt --check --stats: exit %d, stdout %q", code, stdout)
	}
	for _, want := range []string{"checked 2 files, 4 lines in ", "(saved result); 1 not formatted\n", "cache         hit: "} {
		if !strings.Contains(stderr, want) {
			t.Errorf("saved fmt --check --stats stderr lacks %q:\n%s", want, stderr)
		}
	}

	code, _, stderr = fmtStats("--stats", dir)
	if code != 0 || !strings.Contains(stderr, "eye fmt: formatted 2 files, 4 lines in ") || !strings.Contains(stderr, "; 1 rewritten\n") {
		t.Errorf("fmt --stats: exit %d, stderr %q", code, stderr)
	}
}

func TestCount(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 18430: "18,430", 1234567: "1,234,567", -1000: "-1,000"} {
		if got := count(n); got != want {
			t.Errorf("count(%d) = %q, want %q", n, got, want)
		}
	}
}
