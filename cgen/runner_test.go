package cgen

import (
	"debug/elf"
	"debug/macho"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"eyelang/internal/gotool"
)

// needGo skips a test that builds through the runner in -short mode, or
// when the pinned Go can't be had, as on a machine with no network and no
// cached copy. Otherwise it fetches the pinned Go the first time.
func needGo(t *testing.T) {
	t.Helper()
	needZig(t)
	if _, err := gotool.Path(nil); err != nil {
		t.Skipf("no pinned Go: %v", err)
	}
}

// hello is a program with a main and a test.
func hello(t *testing.T) string {
	dir := t.TempDir()
	src := `pub fn main(args: List(String)) uses {stdio} {
  print("hello from the runner {args |> size}")
}

test "adds" {
  expect(1 + 1 == 2)
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.eye"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestRunnerBuild builds a program through the runner (B′, runner.go): its
// main and its tests run from the Go runner's main, through cgo, and on
// Linux the binary is static.
func TestRunnerBuild(t *testing.T) {
	needGo(t)
	prog := load(t, hello(t))
	for _, tests := range []bool{false, true} {
		out := filepath.Join(t.TempDir(), "hello")
		if err := Build(prog, BuildOptions{Out: out, Tests: tests, Runner: true}); err != nil {
			t.Fatal(err)
		}
		args := []string{"a", "b"}
		if tests {
			args = nil // a test binary takes only its own flags
		}
		got, err := exec.Command(out, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", out, err, got)
		}
		want := "hello from the runner 3\n"
		if tests {
			want = "1 test passed"
		}
		if !strings.Contains(string(got), want) {
			t.Errorf("tests=%v: the runner printed %q, want %q in it", tests, got, want)
		}
		if strings.HasPrefix(HostTarget(), "linux/") {
			checkStatic(t, out)
		}
	}
}

// TestRunnerCrossBuild builds through the runner for the other targets it
// supports: Linux binaries are static ELF for their machine, and an arm64
// macOS binary is signed, which macOS needs to run it.
func TestRunnerCrossBuild(t *testing.T) {
	needGo(t)
	prog := load(t, hello(t))
	for _, target := range []string{"linux/arm64", "linux/amd64", "darwin/arm64", "darwin/amd64"} {
		if target == HostTarget() {
			continue // TestRunnerBuild builds it
		}
		t.Run(strings.ReplaceAll(target, "/", "-"), func(t *testing.T) {
			t.Parallel()
			out := filepath.Join(t.TempDir(), "hello")
			if err := Build(prog, BuildOptions{Out: out, Target: target, Runner: true}); err != nil {
				t.Fatal(err)
			}
			switch target {
			case "linux/arm64", "linux/amd64":
				f, err := elf.Open(out)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				want := map[string]elf.Machine{"linux/arm64": elf.EM_AARCH64, "linux/amd64": elf.EM_X86_64}[target]
				if f.Machine != want {
					t.Errorf("%s is for %v, not %v", target, f.Machine, want)
				}
				checkStatic(t, out)
			default:
				f, err := macho.Open(out)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				want := map[string]macho.Cpu{"darwin/arm64": macho.CpuArm64, "darwin/amd64": macho.CpuAmd64}[target]
				if f.Cpu != want {
					t.Errorf("%s is for %v, not %v", target, f.Cpu, want)
				}
				libs, _ := f.ImportedLibraries()
				for _, l := range libs {
					if l != "/usr/lib/libSystem.B.dylib" && l != "/usr/lib/libresolv.9.dylib" {
						t.Errorf("%s links %s, which a Mac may not have", target, l)
					}
				}
				if target == "darwin/arm64" && !signed(f) {
					t.Errorf("%s isn't signed, so macOS won't run it", target)
				}
			}
		})
	}
}

// checkStatic fails if the ELF binary at path asks for a dynamic linker.
func checkStatic(t *testing.T, path string) {
	t.Helper()
	f, err := elf.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, p := range f.Progs {
		if p.Type == elf.PT_INTERP || p.Type == elf.PT_DYNAMIC {
			t.Errorf("%s is linked dynamically", path)
		}
	}
}

// signed reports whether a Mach-O file has a code signature.
func signed(f *macho.File) bool {
	const lcCodeSignature = 0x1d
	for _, l := range f.Loads {
		raw := l.Raw()
		if len(raw) >= 4 && f.ByteOrder.Uint32(raw) == lcCodeSignature {
			return true
		}
	}
	return false
}
