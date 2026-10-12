package clib_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"eyelang/host"
	_ "eyelang/host/clib"
	"eyelang/internal/ffi"
)

// needZlib skips the test when the system has no zlib to load.
func needZlib(t *testing.T) {
	t.Helper()
	if _, err := ffi.Open("z"); err != nil {
		t.Skipf("no zlib to call: %v", err)
	}
}

func load(t *testing.T, dir string) *host.Program {
	t.Helper()
	p, err := host.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestZlib calls zlib's crc32, zlibVersion and gzip functions from an
// Eyelang program: Bytes and Strings go to C, integers and Strings come
// back, and a GzFile is a pointer only extern fns make and take.
func TestZlib(t *testing.T) {
	needZlib(t)
	p := load(t, "testdata/zlib")
	path := filepath.Join(t.TempDir(), "hello.gz")
	var out, errs bytes.Buffer
	code := p.Run(context.Background(), host.RunOptions{Args: []string{"zlib", path}, Stdout: &out, Stderr: &errs})
	if code != 1 {
		t.Errorf("exit status %d, want 1 from main's second write", code)
	}
	if got := out.String(); !strings.HasPrefix(got, "crc32 907060870, zlib 1.") {
		t.Errorf("printed %q", got)
	}
	if got, want := errs.String(), "can't open /no/such/dir/x.gz\n"; got != want {
		t.Errorf("stderr is %q, want %q: gzopen's NULL is None", got, want)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	text, err := io.ReadAll(r)
	if err != nil || string(text) != "hello, gzip" {
		t.Errorf("the gzip file holds %q (%v)", text, err)
	}
	for _, r := range p.Test("") {
		if r.Failure != "" {
			t.Errorf("%s: %s", r.Name, r.Failure)
		}
	}
}

// TestWrongC checks that what can be found wrong about a C function is
// reported, not a crash: a library or a symbol that isn't there, before
// main starts and at the call in a test, and NULL from a function whose
// declaration doesn't allow it. (A C function's parameter types can't be
// checked: C libraries don't record them.)
func TestWrongC(t *testing.T) {
	needZlib(t)
	p := load(t, "testdata/wrong")
	var errs bytes.Buffer
	if code := p.Run(context.Background(), host.RunOptions{Stdout: &bytes.Buffer{}, Stderr: &errs}); code != 1 {
		t.Errorf("exit status %d, want 1", code)
	}
	lines := strings.Split(strings.TrimSpace(errs.String()), "\n")
	if len(lines) != 2 ||
		!strings.HasPrefix(lines[0], `testdata/wrong/main.eye:3:1: can't load the library "eyelang-no-such-library": `) ||
		!strings.HasPrefix(lines[1], "testdata/wrong/main.eye:6:1: ") ||
		!strings.Contains(lines[1], "has no symbol noSuchZlibFunction") {
		t.Errorf("stderr:\n%s", errs.String())
	}
	want := map[string]string{
		"a library that isn't there":               `testdata/wrong/main.eye:17:10: can't load the library "eyelang-no-such-library"`,
		"a symbol the library doesn't have":        "has no symbol noSuchZlibFunction",
		"NULL where the declaration has no Option": "testdata/wrong/main.eye:25:11: gzopen returned NULL, which isn't a GzFile: if it may, declare its result Option(GzFile)",
	}
	results := p.Test("")
	if len(results) != len(want) {
		t.Fatalf("ran %d tests, want %d", len(results), len(want))
	}
	for _, r := range results {
		if !strings.Contains(r.Failure, want[r.Name]) {
			t.Errorf("%s: failed with %q, want %q", r.Name, r.Failure, want[r.Name])
		}
	}
}
