// Command zig runs the zig this repository pins, downloading it once into the
// user's cache and checking it against the checksum below. Every build of C
// in the repository goes through it (`scripts/zig`, `task zig -- …`, CI), so
// everyone compiles with the same zig, as `eye build` will (compilation.md,
// section 1):
//
//	go run ./tools/zig cc -O2 hello.c -o hello
//	go run ./tools/zig --path        # where the pinned zig lives, fetching it if needed
//
// To move to a new release, change version and the checksums together, from
// https://ziglang.org/download/index.json.
package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// version is the zig release the repository builds C with.
const version = "0.17.0"

// archives holds each platform's download and its SHA-256, from
// ziglang.org/download/index.json.
var archives = map[string]struct{ file, sha256 string }{
	"x86_64-linux":    {"zig-x86_64-linux-0.17.0.tar.xz", "1cbe9df9f27e6b78d14ccbca43b6703a404ef79ef1c463de901d7f088d4e2026"},
	"aarch64-linux":   {"zig-aarch64-linux-0.17.0.tar.xz", "9e8d11661d4ae3bd57702a3832781e23ad151dde5798e16a5ccd503f65234ff8"},
	"x86_64-macos":    {"zig-x86_64-macos-0.17.0.tar.xz", "4f9a1c5269aa17ebda5e6d3c2b89d6cbf36f7d2b22a0306e9ab98f25f95529c6"},
	"aarch64-macos":   {"zig-aarch64-macos-0.17.0.tar.xz", "b607e9b9234790a008116ae5bdb71c6243b84b9fb42a53a9e70fde41c06c536a"},
	"x86_64-windows":  {"zig-x86_64-windows-0.17.0.zip", "b5663f69581dcf391293fbf16c06cb80d81d806545ce618b4d0bab7f0eb8c428"},
	"aarch64-windows": {"zig-aarch64-windows-0.17.0.zip", "0a59d91fa1cb40cf068e9b0954434ce973500c7a2ea749f1e01af62cdab52d26"},
}

func main() {
	exe, err := ensure()
	if err != nil {
		fmt.Fprintln(os.Stderr, "zig:", err)
		os.Exit(1)
	}
	args := os.Args[1:]
	if len(args) == 1 && args[0] == "--path" {
		fmt.Println(exe)
		return
	}
	cmd := exec.Command(exe, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "zig:", err)
		os.Exit(1)
	}
}

// platform names this machine as zig's downloads do.
func platform() string {
	arch := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
	osName := map[string]string{"linux": "linux", "darwin": "macos", "windows": "windows"}[runtime.GOOS]
	return arch + "-" + osName
}

// ensure returns the pinned zig's executable, downloading and unpacking it
// first if the cache doesn't hold it. EYE_ZIG_CACHE moves the cache.
func ensure() (string, error) {
	plat := platform()
	a, ok := archives[plat]
	if !ok {
		return "", fmt.Errorf("no zig %s for %s/%s in tools/zig", version, runtime.GOOS, runtime.GOARCH)
	}
	root := os.Getenv("EYE_ZIG_CACHE")
	if root == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(cache, "eyelang", "zig")
	}
	dir := filepath.Join(root, version)
	exe := filepath.Join(dir, strings.TrimSuffix(strings.TrimSuffix(a.file, ".tar.xz"), ".zip"), "zig")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	if _, err := os.Stat(exe); err == nil {
		return exe, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	fmt.Fprintf(os.Stderr, "zig: fetching zig %s for %s into %s\n", version, plat, dir)
	archive := filepath.Join(dir, a.file+".part")
	defer os.Remove(archive)
	if err := download("https://ziglang.org/download/"+version+"/"+a.file, archive, a.sha256); err != nil {
		return "", err
	}
	if err := unpack(archive, a.file, dir); err != nil {
		return "", err
	}
	if _, err := os.Stat(exe); err != nil {
		return "", fmt.Errorf("unpacked %s, but %s isn't there", a.file, exe)
	}
	return exe, nil
}

// download writes url to path and checks its SHA-256 against want.
func download(url, path, want string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetching %s: %s", url, resp.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		f.Close()
		return fmt.Errorf("fetching %s: %w", url, err)
	}
	if err := f.Close(); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("%s has SHA-256 %s, but tools/zig pins %s", url, got, want)
	}
	return nil
}

// unpack extracts the archive into dir: a .zip with archive/zip, a .tar.xz
// with the system's tar, which every platform with such a download has.
func unpack(archive, name, dir string) error {
	if strings.HasSuffix(name, ".tar.xz") {
		if out, err := exec.Command("tar", "-xJf", archive, "-C", dir).CombinedOutput(); err != nil {
			return fmt.Errorf("unpacking %s: %v: %s", name, err, out)
		}
		return nil
	}
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		target := filepath.Join(dir, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(target, filepath.Clean(dir)+string(os.PathSeparator)) {
			return fmt.Errorf("%s holds a path outside its folder: %s", name, f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := extract(f, target); err != nil {
			return err
		}
	}
	return nil
}

func extract(f *zip.File, target string) error {
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode()|0o200)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	return dst.Close()
}
