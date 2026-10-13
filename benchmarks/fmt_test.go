package benchmarks

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// fmtRun is what one `eye fmt` printed and how it ended.
type fmtRun struct {
	code           int
	stdout, stderr string
	peak           int64 // bytes of memory at most, or 0 where unknown
}

// runEye runs eye in dir with its saved results kept under cache.
func runEye(tb testing.TB, eye, dir, cache string, args ...string) fmtRun {
	tb.Helper()
	return runEyeEnv(tb, eye, dir, cache, nil, args...)
}

// runEyeEnv is runEye with env added to the environment.
func runEyeEnv(tb testing.TB, eye, dir, cache string, env []string, args ...string) fmtRun {
	tb.Helper()
	cmd := exec.Command(eye, args...)
	cmd.Dir = dir
	// os.UserCacheDir is $HOME/Library/Caches on macOS and $XDG_CACHE_HOME
	// elsewhere: both point into cache, so the user's own is left alone.
	// The pinned zig stays where it is, so it isn't fetched again.
	cmd.Env = append(os.Environ(), "HOME="+cache, "XDG_CACHE_HOME="+filepath.Join(cache, "xdg"), "EYE_ZIG_CACHE="+zigCache())
	cmd.Env = append(cmd.Env, env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	run := fmtRun{stdout: stdout.String(), stderr: stderr.String()}
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		run.code = exit.ExitCode()
	default:
		tb.Fatalf("eye %v: %v", args, err)
	}
	run.peak = peakMemory(cmd.ProcessState)
	return run
}

// TestFmtCheckSameOutput runs a small corpus through a cold check, a first
// check that saves its result, and a check that replays it: all three print
// the same. It keeps the benchmark below working, and runs in CI.
func TestFmtCheckSameOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("builds eye")
	}
	eye := eyeBinary(t)
	dir, files, _ := corpusDir(t, benchLines(t, 10_000))
	cache := t.TempDir()
	cold := runEye(t, eye, dir, cache, "fmt", "--check", "--cold", ".")
	if cold.code > 1 {
		t.Fatalf("cold check exited %d: %s", cold.code, cold.stderr)
	}
	for _, name := range []string{"first", "warm"} {
		got := runEye(t, eye, dir, cache, "fmt", "--check", ".")
		if got.code != cold.code || got.stdout != cold.stdout || got.stderr != cold.stderr {
			t.Fatalf("%s check differs from the cold one:\nexit %d, stdout:\n%s\nstderr:\n%s\nwant exit %d, stdout:\n%s\nstderr:\n%s",
				name, got.code, got.stdout, got.stderr, cold.code, cold.stdout, cold.stderr)
		}
	}
	t.Logf("%d files; exit %d", files, cold.code)
}

// BenchmarkFmtCheck times `eye fmt --check .` over a corpus of
// EYE_BENCH_LINES lines (100,000 unless set), as a whole process:
//
//   - cold: --cold, so every program is checked and every file formatted
//   - miss: no saved result, so it checks, then saves one
//   - warm: a saved result, so it rereads the inputs and replays it
func BenchmarkFmtCheck(b *testing.B) {
	eye := eyeBinary(b)
	dir, _, lines := corpusDir(b, benchLines(b, 100_000))
	b.Run("cold", func(b *testing.B) {
		cache := b.TempDir()
		measure(b, lines, func() fmtRun { return runEye(b, eye, dir, cache, "fmt", "--check", "--cold", ".") })
	})
	b.Run("miss", func(b *testing.B) {
		measure(b, lines, func() fmtRun {
			b.StopTimer()
			cache, err := os.MkdirTemp(b.TempDir(), "cache")
			if err != nil {
				b.Fatal(err)
			}
			b.StartTimer()
			return runEye(b, eye, dir, cache, "fmt", "--check", ".")
		})
	})
	b.Run("warm", func(b *testing.B) {
		cache := b.TempDir()
		runEye(b, eye, dir, cache, "fmt", "--check", ".")
		measure(b, lines, func() fmtRun { return runEye(b, eye, dir, cache, "fmt", "--check", ".") })
	})
}

// measure runs fn b.N times and reports lines per second and the most
// memory one run held.
func measure(b *testing.B, lines int, fn func() fmtRun) {
	b.Helper()
	var peak int64
	for b.Loop() {
		run := fn()
		if run.code > 1 {
			b.Fatalf("eye fmt exited %d: %s", run.code, run.stderr)
		}
		peak = max(peak, run.peak)
	}
	b.ReportMetric(float64(lines)*float64(b.N)/b.Elapsed().Seconds(), "lines/s")
	if peak > 0 {
		b.ReportMetric(float64(peak)/(1<<20), "peak-MiB")
	}
}
