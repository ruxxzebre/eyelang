package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

type fmtRun struct {
	code           int
	stdout, stderr string
}

func runFmtCheck(t *testing.T, args ...string) fmtRun {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(append([]string{"fmt", "--check"}, args...), &stdout, &stderr)
	return fmtRun{code, stdout.String(), stderr.String()}
}

// isolateCache points the saved results at a fresh directory.
func isolateCache(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, src := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// rewrite changes a file's contents but keeps its modification time, so
// only its contents can tell the saved result is stale.
func rewrite(t *testing.T, path, src string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, time.Time{}, info.ModTime()); err != nil {
		t.Fatal(err)
	}
}

// TestFmtCheckCache checks that a warm check prints exactly what a cold one
// does, and that a change to any file the check read invalidates it.
func TestFmtCheckCache(t *testing.T) {
	isolateCache(t)
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"eye.toml":    "",
		"app/app.eye": "import lib\n\npub fn f() -> Int = lib.at()\n",
		"lib/lib.eye": "pub fn at() -> Int = 1\n",
		"bad/bad.eye": "fn f( {\n",
	})
	app := filepath.Join(dir, "app")

	cold := runFmtCheck(t, "--cold", dir)
	if cold.code != exitErrors || !strings.Contains(cold.stderr, "bad.eye") {
		t.Fatalf("cold check = %+v, want the syntax error in bad.eye", cold)
	}
	for i := range 2 {
		if warm := runFmtCheck(t, dir); warm != cold {
			t.Fatalf("check %d = %+v, want the cold result %+v", i, warm, cold)
		}
	}
	if got := runFmtCheck(t, app); got.code != 0 {
		t.Fatalf("check of app = %+v, want 0", got)
	}

	// lib is outside the directory checked, but app's uses clause depends on it.
	rewrite(t, filepath.Join(dir, "lib/lib.eye"), "pub fn at() -> Int uses {stdio} {\n  print(\"x\")\n  1\n}\n")
	if got := runFmtCheck(t, app); got.code != exitErrors || !strings.Contains(got.stdout, "app.eye") {
		t.Errorf("check after editing an import = %+v, want app.eye listed", got)
	}

	// A file's contents change while its size and time stay the same.
	tidy := "pub fn at() -> Int = 1\n"
	writeFiles(t, dir, map[string]string{"lib/lib.eye": tidy})
	if got := runFmtCheck(t, filepath.Join(dir, "lib")); got.code != 0 {
		t.Fatalf("check of lib = %+v, want 0", got)
	}
	rewrite(t, filepath.Join(dir, "lib/lib.eye"), "pub fn at() -> Int =   1\n")
	if got := runFmtCheck(t, filepath.Join(dir, "lib")); got.code != exitErrors {
		t.Errorf("check after an edit that kept the time = %+v, want lib.eye listed", got)
	}

	// A new file is checked too.
	writeFiles(t, dir, map[string]string{"lib/lib.eye": tidy})
	runFmtCheck(t, filepath.Join(dir, "lib"))
	writeFiles(t, dir, map[string]string{"lib/more.eye": "pub fn   g() = 2\n"})
	if got := runFmtCheck(t, filepath.Join(dir, "lib")); !strings.Contains(got.stdout, "more.eye") {
		t.Errorf("check after adding a file = %+v, want more.eye listed", got)
	}

	// And a removed one is no longer listed.
	if err := os.Remove(filepath.Join(dir, "lib/more.eye")); err != nil {
		t.Fatal(err)
	}
	if got := runFmtCheck(t, filepath.Join(dir, "lib")); got.code != 0 {
		t.Errorf("check after removing a file = %+v, want 0", got)
	}
}

// TestFmtCheckCacheProgram checks that a saved result notices a change to
// the program around the files: a module that appears, or an eye.toml that
// makes a directory a program of its own.
func TestFmtCheckCacheProgram(t *testing.T) {
	isolateCache(t)
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"eye.toml":    "",
		"app/app.eye": "import lib\nimport extra\n\npub fn f() -> Int = lib.at() + extra.n()\n",
		"lib/lib.eye": "pub fn at() -> Int = 1\n",
	})
	app := filepath.Join(dir, "app")
	same := func(when string) fmtRun {
		t.Helper()
		warm, cold := runFmtCheck(t, app), runFmtCheck(t, "--cold", app)
		if warm != cold {
			t.Fatalf("%s: warm check %+v, cold %+v", when, warm, cold)
		}
		return warm
	}

	if got := same("extra missing"); got.code != exitErrors || !strings.Contains(got.stderr, "has errors") {
		t.Fatalf("check with extra missing = %+v, want errors", got)
	}
	runFmtCheck(t, app)
	writeFiles(t, dir, map[string]string{"extra/extra.eye": "pub fn n() -> Int = 2\n"})
	if got := same("extra added"); got.code != 0 {
		t.Fatalf("check after adding extra = %+v, want 0", got)
	}
	runFmtCheck(t, app)
	writeFiles(t, dir, map[string]string{"app/eye.toml": ""})
	if got := same("app/eye.toml added"); got.code != exitErrors {
		t.Errorf("check after app became a program of its own = %+v, want errors", got)
	}
}

// TestFmtCheckUnwritableCache checks that a cache that can't be written
// changes nothing but speed.
func TestFmtCheckUnwritableCache(t *testing.T) {
	isolateCache(t)
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Skip(err)
	}
	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache, nil, 0o644); err != nil { // a file where the directory should be
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"messy.eye": "fn  f( x:Int )=x+1\n"})
	first := runFmtCheck(t, dir)
	if first.code != exitErrors || !strings.Contains(first.stdout, "messy.eye") {
		t.Fatalf("check = %+v, want messy.eye listed", first)
	}
	if again := runFmtCheck(t, dir); again != first {
		t.Errorf("second check = %+v, want %+v", again, first)
	}
}

// TestFmtInPlaceIgnoresCache checks that formatting files never replays a
// saved check result.
func TestFmtInPlaceIgnoresCache(t *testing.T) {
	isolateCache(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "messy.eye")
	writeFiles(t, dir, map[string]string{"messy.eye": "fn  f( x:Int )=x+1\n"})
	runFmtCheck(t, dir)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"fmt", dir}, &stdout, &stderr); code != 0 {
		t.Fatalf("fmt = %d (stderr: %s)", code, stderr.String())
	}
	if data, _ := os.ReadFile(path); string(data) != "fn f(x: Int) = x + 1\n" {
		t.Errorf("fmt wrote %q", data)
	}
	if got := runFmtCheck(t, dir); got.code != 0 {
		t.Errorf("check after formatting = %+v, want 0", got)
	}
}

// TestFmtResultValidation checks that a saved result that is damaged, or
// misses the inputs of the files it covers, is not used.
func TestFmtResultValidation(t *testing.T) {
	isolateCache(t)
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"a.eye": "fn f() = 1\n"})
	runFmtCheck(t, dir)
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	saved, _ := filepath.Glob(filepath.Join(cache, "eyelang", "fmt", "*.json"))
	if len(saved) != 1 {
		t.Fatalf("saved results = %v, want one", saved)
	}
	path := saved[0]
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var good fmtResult
	if err := json.Unmarshal(data, &good); err != nil {
		t.Fatal(err)
	}
	// The result as saved is used, so each change below is what rejects it.
	if _, ok := loadFmtResult(path, good.Files); !ok {
		t.Fatalf("loadFmtResult rejected the result as saved: %s", data)
	}
	for name, change := range map[string]func(*fmtResult){
		"old version":  func(r *fmtResult) { r.Version-- },
		"empty inputs": func(r *fmtResult) { r.Inputs = nil },
		"other files":  func(r *fmtResult) { r.Files = nil },
		"bad code":     func(r *fmtResult) { r.Code = 9 },
		"stale input":  func(r *fmtResult) { r.Inputs[0].Value = "x" },
	} {
		r := good
		r.Inputs = slices.Clone(good.Inputs)
		change(&r)
		data, _ := json.Marshal(r)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, ok := loadFmtResult(path, good.Files); ok {
			t.Errorf("%s: loadFmtResult accepted it", name)
		}
	}
	if err := os.WriteFile(path, data[:len(data)/2], 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := loadFmtResult(path, good.Files); ok {
		t.Error("truncated: loadFmtResult accepted it")
	}
	if got := runFmtCheck(t, dir); got.code != 0 {
		t.Errorf("check with a truncated saved result = %+v, want 0", got)
	}
}
