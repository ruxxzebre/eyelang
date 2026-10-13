package benchmarks

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// seeds are the real programs a corpus is copied from. Each keeps its
// eye.toml, so every copy is a set of programs of its own.
var seeds = []string{"../testdata/programs", "../testdata/lab"}

type seedFile struct {
	rel  string
	data []byte
}

// corpus fills dir with copies of the seeds until it holds at least lines
// lines of Eyelang, and returns how many .eye files and lines it wrote.
// Hidden directories, such as a program's .eye journal, are left out.
func corpus(tb testing.TB, dir string, lines int) (files, total int) {
	tb.Helper()
	var seed []seedFile
	perCopy, filesPerCopy := 0, 0
	for _, root := range seeds {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if path != root && strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(filepath.Dir(root), path)
			if err != nil {
				return err
			}
			seed = append(seed, seedFile{rel, data})
			if strings.HasSuffix(path, ".eye") {
				filesPerCopy++
				perCopy += bytes.Count(data, []byte("\n"))
			}
			return nil
		})
		if err != nil {
			tb.Fatal(err)
		}
	}
	if perCopy == 0 {
		tb.Fatal("the seed programs have no lines")
	}
	copies := max(1, (lines+perCopy-1)/perCopy)
	for i := range copies {
		for _, f := range seed {
			path := filepath.Join(dir, fmt.Sprintf("c%04d", i), f.rel)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				tb.Fatal(err)
			}
			if err := os.WriteFile(path, f.data, 0o644); err != nil {
				tb.Fatal(err)
			}
		}
	}
	return copies * filesPerCopy, copies * perCopy
}

// corpusDir is where a corpus of lines lines is written: EYE_BENCH_CORPUS
// when set, kept afterwards to run eye on by hand, or else a directory
// removed when the test ends.
func corpusDir(tb testing.TB, lines int) (dir string, files, total int) {
	tb.Helper()
	dir = os.Getenv("EYE_BENCH_CORPUS")
	if dir == "" {
		dir = tb.TempDir()
	} else if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		tb.Fatalf("EYE_BENCH_CORPUS=%s isn't empty; name a new directory", dir)
	}
	files, total = corpus(tb, dir, lines)
	tb.Logf("corpus: %d files, %d lines in %s", files, total, dir)
	return dir, files, total
}

// benchLines is the corpus size: EYE_BENCH_LINES, or def.
func benchLines(tb testing.TB, def int) int {
	tb.Helper()
	s := os.Getenv("EYE_BENCH_LINES")
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(strings.ReplaceAll(s, "_", ""))
	if err != nil || n <= 0 {
		tb.Fatalf("EYE_BENCH_LINES=%q: want a number of lines, such as 1000000", s)
	}
	return n
}

// eyeBinary is the eye to measure: EYE_BENCH_EYE, such as an older build to
// compare with, or else one built from this checkout once per run.
var builtEye string

func eyeBinary(tb testing.TB) string {
	tb.Helper()
	if eye := os.Getenv("EYE_BENCH_EYE"); eye != "" {
		return eye
	}
	if builtEye == "" {
		dir, err := os.MkdirTemp("", "eye-bench-")
		if err != nil {
			tb.Fatal(err)
		}
		eye := filepath.Join(dir, "eye")
		if out, err := exec.Command("go", "build", "-o", eye, "../cmd/eye").CombinedOutput(); err != nil {
			tb.Fatalf("building eye: %v\n%s", err, out)
		}
		builtEye = eye
	}
	return builtEye
}

// zigCache is where the pinned zig is kept, as eye finds it for the user
// running the benchmarks, so runs with a home of their own don't fetch it.
func zigCache() string {
	if dir := os.Getenv("EYE_ZIG_CACHE"); dir != "" {
		return dir
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(cache, "eyelang", "zig")
}

func TestMain(m *testing.M) {
	code := m.Run()
	if builtEye != "" {
		os.RemoveAll(filepath.Dir(builtEye))
	}
	os.Exit(code)
}
