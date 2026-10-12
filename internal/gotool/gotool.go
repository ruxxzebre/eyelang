// Package gotool finds the Go toolchain this repository pins, downloading
// it once into the user's cache and checking it against the checksum below,
// as internal/zig does for zig. `eye build --runner` builds a program that
// needs the runner with it (compilation.md, "Where the runtime lives"): the
// runner, written in Go, is the program's main, and the program's C is
// linked in through cgo. `eye` itself never runs cgo; only this Go does, in
// the user's build.
//
// Version matches go.mod's go line, so the runner a program links is built
// by the Go its tests ran with. To move to a new release, change Version
// and the checksums together, from https://go.dev/dl/?mode=json&include=all.
package gotool

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Version is the Go release programs that need the runner are built with.
const Version = "1.27.2"

// archives holds each platform's download and its SHA-256, from
// go.dev/dl/?mode=json&include=all.
var archives = map[string]struct{ file, sha256 string }{
	"linux/amd64":   {"go1.27.2.linux-amd64.tar.gz", "ecbadb99091a3f46e31f5f934b068b1864eafa7995211b39eaddf76996045fe5"},
	"linux/arm64":   {"go1.27.2.linux-arm64.tar.gz", "94f3e30b8e374bc285e7dadc11e0865726b9bc6e85b841ccceaabc0214c6b7c8"},
	"darwin/amd64":  {"go1.27.2.darwin-amd64.tar.gz", "587b59182488b23aa6e5fc25110405a3e0e5b38ed2f5b2f46ed13c32aee356fe"},
	"darwin/arm64":  {"go1.27.2.darwin-arm64.tar.gz", "76812b213b1b2302c978d28fa52fa92d541704b9e7d9d5db8002c50e4018c4c5"},
	"windows/amd64": {"go1.27.2.windows-amd64.zip", "1314008898bd40df77af4b014f777f08873dbdfbcd3d92308728ee03304fe04f"},
	"windows/arm64": {"go1.27.2.windows-arm64.zip", "5c4b294f23b558619ae8a4aa94d023e3bca03f52c8398bb41b9026002b0013de"},
}

// Root returns the pinned Go's GOROOT, downloading and unpacking it first
// if the cache doesn't hold it, and saying so on progress. The cache is in
// the user's cache directory, or in EYE_GO_CACHE when it is set.
func Root(progress io.Writer) (string, error) {
	plat := runtime.GOOS + "/" + runtime.GOARCH
	a, ok := archives[plat]
	if !ok {
		return "", fmt.Errorf("no Go %s for %s in internal/gotool", Version, plat)
	}
	cache := os.Getenv("EYE_GO_CACHE")
	if cache == "" {
		dir, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		cache = filepath.Join(dir, "eyelang", "go")
	}
	dir := filepath.Join(cache, Version)
	root := filepath.Join(dir, "go")
	if _, err := os.Stat(exe(root)); err == nil {
		return root, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if progress != nil {
		fmt.Fprintf(progress, "go: fetching Go %s for %s into %s\n", Version, plat, dir)
	}
	// Unpacked beside the cache and moved into place, so that two builds
	// fetching at once never see half a toolchain.
	tmp, err := os.MkdirTemp(dir, "unpack-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	archive := filepath.Join(tmp, a.file)
	if err := download("https://go.dev/dl/"+a.file, archive, a.sha256); err != nil {
		return "", err
	}
	if err := unpack(archive, tmp); err != nil {
		return "", fmt.Errorf("unpacking %s: %w", a.file, err)
	}
	if err := os.Rename(filepath.Join(tmp, "go"), root); err != nil {
		if _, statErr := os.Stat(exe(root)); statErr != nil {
			return "", err
		}
		// Another build put it there first.
	}
	if _, err := os.Stat(exe(root)); err != nil {
		return "", fmt.Errorf("unpacked %s, but %s isn't there", a.file, exe(root))
	}
	return root, nil
}

// Path returns the pinned go command, fetching the toolchain if needed.
func Path(progress io.Writer) (string, error) {
	root, err := Root(progress)
	if err != nil {
		return "", err
	}
	return exe(root), nil
}

func exe(root string) string {
	p := filepath.Join(root, "bin", "go")
	if runtime.GOOS == "windows" {
		p += ".exe"
	}
	return p
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
		return fmt.Errorf("%s has SHA-256 %s, but internal/gotool pins %s", url, got, want)
	}
	return nil
}

// unpack extracts a .tar.gz or a .zip into dir.
func unpack(archive, dir string) error {
	if strings.HasSuffix(archive, ".zip") {
		return unzip(archive, dir)
	}
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := inside(dir, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := write(target, tr, hdr.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		}
	}
}

func unzip(archive, dir string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		target, err := inside(dir, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		src, err := f.Open()
		if err != nil {
			return err
		}
		err = write(target, src, f.Mode())
		src.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// inside is name's path under dir, or an error if name would leave it.
func inside(dir, name string) (string, error) {
	target := filepath.Join(dir, filepath.FromSlash(name))
	if !strings.HasPrefix(target, filepath.Clean(dir)+string(os.PathSeparator)) {
		return "", fmt.Errorf("the archive holds a path outside its folder: %s", name)
	}
	return target, nil
}

func write(target string, src io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm()|0o200)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	return dst.Close()
}
