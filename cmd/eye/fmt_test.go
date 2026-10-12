package main

import (
	"path/filepath"
	"reflect"
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
