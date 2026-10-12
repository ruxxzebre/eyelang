package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEyeScriptRebuilds(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("the checkout wrapper requires a POSIX shell")
	}
	wrapper, err := os.ReadFile("../../scripts/eye")
	if err != nil {
		t.Fatal(err)
	}
	dir := program(t, map[string]string{
		"scripts/eye":     string(wrapper),
		"go.mod":          "module wrappertest\n",
		"cmd/eye/main.go": "#!/bin/sh\nprintf 'one\\n'\n",
		"fakebin/go": `#!/bin/sh
set -e
case "$1" in
  env) printf '{\n  "GOWORK": "%s",\n  "GOFLAGS": "%s"\n}\n' "${GOWORK:-}" "${GOFLAGS:-}" ;;
  build)
    echo build >> bin/build.log
    [ "${GOFLAGS:-}" != fail ]
    cp cmd/eye/main.go bin/eye
    chmod +x bin/eye
    ;;
  *) exit 1 ;;
esac
`,
	})
	if err := os.Chmod(filepath.Join(dir, "fakebin/go"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Join(dir, "fakebin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOWORK", "")
	run := func(want string, builds int) {
		t.Helper()
		out, err := exec.Command("sh", filepath.Join(dir, "scripts/eye")).CombinedOutput()
		if err != nil || string(out) != want {
			t.Fatalf("wrapper = %q, %v; want %q", out, err, want)
		}
		log, err := os.ReadFile(filepath.Join(dir, "bin/build.log"))
		if err != nil || strings.Count(string(log), "build\n") != builds {
			t.Fatalf("build log = %q, %v; want %d builds", log, err, builds)
		}
	}
	run("one\n", 1)
	run("one\n", 1)
	// Give source/directory writes a distinct timestamp even on coarse filesystems.
	time.Sleep(20 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "cmd/eye/main.go"), []byte("#!/bin/sh\nprintf 'two\\n'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("two\n", 2)
	run("two\n", 2)
	if err := os.WriteFile(filepath.Join(dir, "cmd/eye/extra.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	run("two\n", 3)
	if err := os.Remove(filepath.Join(dir, "cmd/eye/extra.go")); err != nil {
		t.Fatal(err)
	}
	run("two\n", 4)
	t.Setenv("GOFLAGS", "changed")
	run("two\n", 5)
	run("two\n", 5)
	t.Setenv("GOFLAGS", "fail")
	out, err := exec.Command("sh", filepath.Join(dir, "scripts/eye")).CombinedOutput()
	if err == nil || strings.Contains(string(out), "two\n") {
		t.Fatalf("failed rebuild ran the previous binary: %q, %v", out, err)
	}
	t.Setenv("GOFLAGS", "changed")
	t.Setenv("GOWORK", "/external/go.work")
	run("two\n", 7)
	run("two\n", 8)
	t.Setenv("GOWORK", "")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module wrappertest\nreplace example.com/local => ../local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("two\n", 9)
	run("two\n", 10)
}
