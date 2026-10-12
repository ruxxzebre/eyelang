package zig

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// fakeArchive exercises the real download, checksum and extraction without
// fetching Zig. A bad entry comes after the executable, as a failed unpack
// must not leave that executable available to the next build.
func fakeArchive(t *testing.T, bad bool, beforeDownload func()) (string, *atomic.Int32) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("EYE_ZIG_CACHE", root)
	exe := "zig"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range []string{"zig-test/" + exe, "zig-test/lib/header.h"} {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(f, "complete"); err != nil {
			t.Fatal(err)
		}
	}
	if bad {
		if _, err := w.Create("../outside"); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	oldArchive := archives[platform()]
	archives[platform()] = struct{ file, sha256 string }{"zig-test.zip", fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))}
	t.Cleanup(func() { archives[platform()] = oldArchive })
	var downloads atomic.Int32
	oldClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		downloads.Add(1)
		if beforeDownload != nil {
			beforeDownload()
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(buf.Bytes()))}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = oldClient })
	return root, &downloads
}

func TestConcurrentPathDownloadsOnce(t *testing.T) {
	started, release := make(chan struct{}, 8), make(chan struct{})
	root, downloads := fakeArchive(t, false, func() {
		started <- struct{}{}
		<-release
	})
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			exe, err := Path(nil)
			if err != nil {
				t.Error(err)
				return
			}
			if data, err := os.ReadFile(filepath.Join(filepath.Dir(exe), "lib/header.h")); err != nil || string(data) != "complete" {
				t.Errorf("incomplete installation: %q, %v", data, err)
			}
		}()
	}
	<-started
	// A second request can proceed while the first is blocked only if
	// callers are installing independently into the same cache.
	select {
	case <-started:
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	wg.Wait()
	if got := downloads.Load(); got != 1 {
		t.Fatalf("downloaded %d times for concurrent builds, want once", got)
	}
	entries, err := os.ReadDir(filepath.Join(root, Version))
	if err != nil || len(entries) != 1 || entries[0].Name() != "zig-test" {
		t.Fatalf("installation left temporary files: %v, %v", entries, err)
	}
}

func TestFailedUnpackIsNotCached(t *testing.T) {
	root, downloads := fakeArchive(t, true, nil)
	for range 2 {
		if exe, err := Path(nil); err == nil || !strings.Contains(err.Error(), "outside its folder") {
			t.Fatalf("failed installation returned %q, %v", exe, err)
		}
	}
	if got := downloads.Load(); got != 2 {
		t.Fatalf("failed installation was reused: %d downloads", got)
	}
	if entries, err := os.ReadDir(filepath.Join(root, Version)); err != nil || len(entries) != 0 {
		t.Fatalf("failed installation left temporary files: %v, %v", entries, err)
	}
}

func TestConcurrentInstallerKeepsPublishedDirectory(t *testing.T) {
	var root string
	root, _ = fakeArchive(t, false, func() {
		// Another process finishes installing while this one downloads.
		lib := filepath.Join(root, Version, "zig-test", "lib")
		if err := os.MkdirAll(lib, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(lib, "header.h"), []byte("winner"), 0o644); err != nil {
			t.Fatal(err)
		}
		name := "zig"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		if err := os.WriteFile(filepath.Join(filepath.Dir(lib), name), []byte("winner"), 0o755); err != nil {
			t.Fatal(err)
		}
	})
	exe, err := Path(nil)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(filepath.Dir(exe), "lib/header.h")); err != nil || string(data) != "winner" {
		t.Fatalf("overwrote the published installation: %q, %v", data, err)
	}
}
