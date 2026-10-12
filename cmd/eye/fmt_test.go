package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFmtDirectoryWithIncompleteLambda(t *testing.T) {
	for _, args := range [][]string{{"fmt", "--check"}, {"fmt"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			const broken = "let f = fn x 1\n"
			dir := program(t, map[string]string{
				"broken.eye": broken,
				"good.eye":   "fn  good()=1\n",
			})
			var stdout, stderr bytes.Buffer
			if code := run(append(args, dir), &stdout, &stderr); code != exitErrors {
				t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
			}
			if !strings.Contains(stderr.String(), "broken.eye:1:14: expected ':' or '{'") {
				t.Fatalf("missing syntax diagnostic: %s", stderr.String())
			}
			if src, err := os.ReadFile(filepath.Join(dir, "broken.eye")); err != nil || string(src) != broken {
				t.Fatalf("broken source changed: %q, %v", src, err)
			}
			want := "fn good() = 1\n"
			if len(args) > 1 {
				want = "fn  good()=1\n"
				if !strings.Contains(stdout.String(), "good.eye") {
					t.Fatalf("valid file wasn't checked: %s", stdout.String())
				}
			}
			if src, err := os.ReadFile(filepath.Join(dir, "good.eye")); err != nil || string(src) != want {
				t.Fatalf("valid source = %q, want %q: %v", src, want, err)
			}
		})
	}
}

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
