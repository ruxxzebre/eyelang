package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckoutRebuilds(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("the checkout wrapper requires a POSIX shell")
	}
	dir := program(t, map[string]string{
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
		if _, err := prepareCheckout(dir, false, io.Discard, io.Discard); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("sh", filepath.Join(dir, "bin/eye")).CombinedOutput()
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
	if rebuilt, err := prepareCheckout(dir, false, io.Discard, io.Discard); err == nil || rebuilt {
		t.Fatalf("failed rebuild reported an executable ready to relaunch: %v, %v", rebuilt, err)
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
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module wrappertest\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("two\n", 11)
	run("two\n", 11)

	// Go's own configuration file counts, though its path stays the same.
	goenv := filepath.Join(t.TempDir(), "env")
	t.Setenv("GOENV", goenv)
	run("two\n", 12)
	if err := os.WriteFile(goenv, []byte("GOFLAGS=-tags=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("two\n", 13)
	run("two\n", 13)

	// A binary replaced by hand is rebuilt from the sources.
	if err := os.WriteFile(filepath.Join(dir, "bin/eye"), []byte("#!/bin/sh\nprintf 'stale binary\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	run("two\n", 14)

	// .git is skipped by the timestamp scan, so moving a branch is seen
	// through the revision.
	for path, data := range map[string]string{".git/HEAD": "ref: refs/heads/dev\n", ".git/refs/heads/dev": "aaaa\n"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, path)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, path), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("two\n", 15)
	run("two\n", 15)
	if err := os.WriteFile(filepath.Join(dir, ".git/refs/heads/dev"), []byte("bbbb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("two\n", 16)
	run("two\n", 16)
}

func TestCheckoutRevision(t *testing.T) {
	write := func(t *testing.T, root string, files map[string]string) {
		t.Helper()
		for path, data := range files {
			path = filepath.Join(root, filepath.FromSlash(path))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	t.Run("branch", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, map[string]string{".git/HEAD": "ref: refs/heads/dev\n", ".git/refs/heads/dev": "aaaa\n"})
		before := checkoutRevision(root)
		write(t, root, map[string]string{".git/refs/heads/dev": "bbbb\n"})
		if after := checkoutRevision(root); after == before || !strings.Contains(after, "bbbb") {
			t.Errorf("revision after a commit = %q, was %q", after, before)
		}
		write(t, root, map[string]string{".git/HEAD": "ref: refs/heads/main\n", ".git/refs/heads/main": "bbbb\n"})
		if after := checkoutRevision(root); !strings.Contains(after, "refs/heads/main") {
			t.Errorf("revision after switching branches = %q", after)
		}
	})

	t.Run("detached", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, map[string]string{".git/HEAD": "cccc\n"})
		if got := checkoutRevision(root); got != "cccc\n" {
			t.Errorf("detached revision = %q", got)
		}
	})

	t.Run("packed", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, map[string]string{".git/HEAD": "ref: refs/heads/dev\n", ".git/packed-refs": "dddd refs/heads/dev\n"})
		before := checkoutRevision(root)
		if !strings.Contains(before, "dddd") {
			t.Fatalf("packed revision = %q", before)
		}
		write(t, root, map[string]string{".git/packed-refs": "eeee refs/heads/dev\n"})
		if after := checkoutRevision(root); after == before {
			t.Errorf("repacked revision unchanged: %q", after)
		}
	})

	t.Run("worktree", func(t *testing.T) {
		// A linked worktree's .git is a file naming its git directory,
		// whose commondir holds the shared refs.
		main, tree := t.TempDir(), t.TempDir()
		gitDir := filepath.Join(main, ".git", "worktrees", "tree")
		write(t, main, map[string]string{
			".git/worktrees/tree/HEAD":      "ref: refs/heads/feature\n",
			".git/worktrees/tree/commondir": "../..\n",
			".git/refs/heads/feature":       "ffff\n",
		})
		write(t, tree, map[string]string{".git": "gitdir: " + gitDir + "\n"})
		before := checkoutRevision(tree)
		if !strings.Contains(before, "ffff") {
			t.Fatalf("worktree revision = %q", before)
		}
		write(t, main, map[string]string{".git/refs/heads/feature": "9999\n"})
		if after := checkoutRevision(tree); !strings.Contains(after, "9999") {
			t.Errorf("worktree revision after a commit = %q", after)
		}
	})
}
