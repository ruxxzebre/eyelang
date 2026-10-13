package benchmarks

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProgramBuilds checks the generated program the build benchmarks use:
// it checks clean, is formatted, and prints the same compiled as it does
// in the interpreter. It runs with go test ./... and builds natively, so
// -short skips it.
func TestProgramBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("builds eye and a native binary")
	}
	eye := eyeBinary(t)
	dir := t.TempDir()
	program(t, dir, 1)
	cache := t.TempDir()
	for _, args := range [][]string{{"check", "."}, {"fmt", "--check", "--cold", "."}} {
		if got := runEye(t, eye, dir, cache, args...); got.code != 0 {
			t.Fatalf("eye %s: exit %d\n%s%s", strings.Join(args, " "), got.code, got.stdout, got.stderr)
		}
	}
	interpreted := runEye(t, eye, dir, cache, "run", "--memory", ".")
	binary := filepath.Join(t.TempDir(), "program")
	built := runEye(t, eye, dir, cache, "build", "-o", binary, ".")
	if built.code != 0 {
		t.Fatalf("eye build: exit %d\n%s", built.code, built.stderr)
	}
	compiled := runEye(t, binary, dir, cache)
	if interpreted.code != 0 || compiled.code != 0 || compiled.stdout != interpreted.stdout {
		t.Fatalf("eye run printed %q (exit %d), the binary %q (exit %d)", interpreted.stdout, interpreted.code, compiled.stdout, compiled.code)
	}
}

// BenchmarkCheck times `eye check` on one generated program of
// EYE_BENCH_LINES lines (100,000 unless set).
func BenchmarkCheck(b *testing.B) {
	eye := eyeBinary(b)
	dir := b.TempDir()
	_, lines := program(b, dir, benchLines(b, 100_000))
	cache := b.TempDir()
	measure(b, lines, func() fmtRun { return runEye(b, eye, dir, cache, "check", ".") })
}

// BenchmarkBuild times `eye build` on one generated program of
// EYE_BENCH_BUILD_LINES lines (10,000 unless set: the C compile grows
// faster than the program, and 100,000 lines take minutes):
//
//   - emit-c: eye's own part, checking the program and writing its C
//   - debug: a whole build with -g, unoptimised
//   - release: a whole build, optimised, as eye build makes by default
//
// Each build starts from an object cache that holds the runtime's objects
// but not the program's, so it compiles the program and nothing else.
func BenchmarkBuild(b *testing.B) {
	eye := eyeBinary(b)
	lines := buildLines(b)
	dir := b.TempDir()
	_, lines = program(b, dir, lines)
	runtime := runtimeCache(b, eye)
	out := filepath.Join(b.TempDir(), "program")
	b.Run("emit-c", func(b *testing.B) {
		measure(b, lines, func() fmtRun {
			b.StopTimer()
			c := filepath.Join(b.TempDir(), "c")
			b.StartTimer()
			return runEye(b, eye, dir, runtime, "build", "--emit-c", c, ".")
		})
	})
	for _, mode := range []struct {
		name string
		args []string
	}{{"debug", []string{"-g"}}, {"release", nil}} {
		b.Run(mode.name, func(b *testing.B) {
			measure(b, lines, func() fmtRun {
				b.StopTimer()
				cache := copyCache(b, runtime)
				b.StartTimer()
				return runEyeEnv(b, eye, dir, runtime, []string{"EYE_BUILD_CACHE=" + cache},
					append(append([]string{"build"}, mode.args...), "-o", out, ".")...)
			})
		})
	}
}

// buildLines is the size of the program the build benchmarks compile.
func buildLines(tb testing.TB) int {
	tb.Helper()
	if os.Getenv("EYE_BENCH_BUILD_LINES") == "" {
		return 10_000
	}
	tb.Setenv("EYE_BENCH_LINES", os.Getenv("EYE_BENCH_BUILD_LINES"))
	return benchLines(tb, 10_000)
}

// runtimeCache returns a home whose object cache, objects, holds the
// runtime's objects in both build modes, built once by compiling hello.
func runtimeCache(tb testing.TB, eye string) string {
	tb.Helper()
	home := tb.TempDir()
	dir := tb.TempDir()
	// Unlike the generated program, so none of its objects are the program's.
	hello := "pub fn main(args: List(String)) uses {stdio} {\n  print(\"hello\")\n}\n"
	for name, data := range map[string]string{"eye.toml": "", "hello.eye": hello} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			tb.Fatal(err)
		}
	}
	cache := filepath.Join(home, "objects")
	for _, args := range [][]string{{"build", "-o", filepath.Join(dir, "p"), "."}, {"build", "-g", "-o", filepath.Join(dir, "p"), "."}} {
		if got := runEyeEnv(tb, eye, dir, home, []string{"EYE_BUILD_CACHE=" + cache}, args...); got.code != 0 {
			tb.Fatalf("eye %s: exit %d\n%s", strings.Join(args, " "), got.code, got.stderr)
		}
	}
	return home
}

// copyCache copies home's runtime objects into a fresh object cache.
func copyCache(tb testing.TB, home string) string {
	tb.Helper()
	dst := filepath.Join(tb.TempDir(), "objects")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		tb.Fatal(err)
	}
	src := filepath.Join(home, "objects")
	entries, err := os.ReadDir(src)
	if err != nil {
		tb.Fatal(err)
	}
	for _, e := range entries {
		in, err := os.Open(filepath.Join(src, e.Name()))
		if err != nil {
			tb.Fatal(err)
		}
		out, err := os.Create(filepath.Join(dst, e.Name()))
		if err != nil {
			tb.Fatal(err)
		}
		if _, err := io.Copy(out, in); err != nil {
			tb.Fatal(err)
		}
		in.Close()
		if err := out.Close(); err != nil {
			tb.Fatal(err)
		}
	}
	return dst
}
