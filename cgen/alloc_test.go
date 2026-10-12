package cgen

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

var allocLine = regexp.MustCompile(`(?m)^allocs ([0-9]+) frees ([0-9]+) faulted ([0-9]+)$`)

// counts runs a binary with EYE_ALLOCS set and gives the objects it made,
// freed, and left behind in failing tests, which it writes at exit.
func counts(t *testing.T, exe string, args ...string) (allocs, frees, faulted uint64) {
	t.Helper()
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), "EYE_ALLOCS=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal(err)
		}
	}
	m := allocLine.FindStringSubmatch(stderr.String())
	if m == nil {
		t.Fatalf("%s wrote no counts of objects:\n%s%s", exe, stdout.String(), stderr.String())
	}
	n := func(s string) uint64 {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	return n(m[1]), n(m[2]), n(m[3])
}

// TestAllocationsBalance runs the native test binary of every program eye
// build compiles, and a main, with the runtime's counts of objects
// (EYE_ALLOCS): every object a test makes is freed by the time the program
// exits. A failing test's fault jumps out of its code without dropping what
// it holds, so the objects failing tests leave are counted on their own,
// and only those may stay (7.12 frees them).
func TestAllocationsBalance(t *testing.T) {
	needZig(t)
	if HostTarget() == "" {
		t.Skip("eye build doesn't target this machine")
	}
	for dir, prog := range compilable(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()
			allocs, frees, faulted := counts(t, build(t, prog, "", true, true), "-v")
			t.Logf("%d objects made, %d freed, %d left by failing tests", allocs, frees, faulted)
			if frees > allocs || allocs-frees != faulted {
				t.Errorf("%d objects made, %d freed: %d not freed, of which failing tests left %d", allocs, frees, int64(allocs)-int64(frees), faulted)
			}
		})
	}
	t.Run("main", func(t *testing.T) {
		t.Parallel()
		exe := build(t, load(t, "testdata/hello"), "", false, false)
		for _, args := range [][]string{{"a"}, {"a", "b"}} { // main gives Ok, then Err
			allocs, frees, faulted := counts(t, exe, args...)
			if allocs == 0 || allocs != frees || faulted != 0 {
				t.Errorf("with %q: %d objects made, %d freed, %d left by faults", args, allocs, frees, faulted)
			}
		}
	})
}
