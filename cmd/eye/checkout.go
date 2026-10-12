package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type checkoutState struct {
	Version  int
	Started  int64
	Env      string
	Revision string
	Binary   string
	External bool
}

var replaceDirective = regexp.MustCompile(`(?m)^\s*replace[\s(]`)

// prepareCheckout is the native counterpart of scripts/eye. A rebuilt
// executable must be relaunched; an unchanged checkout needs no subprocess.
func prepareCheckout(root string, bootstrap bool, stdout, stderr io.Writer) (bool, error) {
	path := filepath.Join(root, "bin", ".eye-checkout.json")
	binary := filepath.Join(root, "bin", "eye")
	env, err := checkoutEnvironment(root)
	if err != nil {
		return false, err
	}
	revision := checkoutRevision(root)
	var state checkoutState
	data, _ := os.ReadFile(path)
	if !bootstrap && json.Unmarshal(data, &state) == nil && state.Version == 1 &&
		state.Env == env && state.Revision == revision && state.Binary == fileIdentity(binary) && !state.External {
		changed, err := checkoutChanged(root, state.Started)
		if err == nil && !changed {
			return false, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		return false, err
	}
	started := time.Now().UnixNano()
	if !bootstrap {
		cmd := exec.Command("go", "build", "-o", "bin/eye", "./cmd/eye")
		cmd.Dir, cmd.Stdout, cmd.Stderr = root, stdout, stderr
		if err := cmd.Run(); err != nil {
			return false, fmt.Errorf("rebuilding eye: %w", err)
		}
	} else if info, err := os.Stat(binary); err == nil {
		started = info.ModTime().UnixNano()
	}
	cmd := exec.Command("go", "env", "-json", "GOWORK")
	cmd.Dir = root
	settings, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("reading Go build settings: %w", err)
	}
	var config struct{ GOWORK string }
	if err := json.Unmarshal(settings, &config); err != nil {
		return false, err
	}
	mod, _ := os.ReadFile(filepath.Join(root, "go.mod"))
	state = checkoutState{1, started, env, revision, fileIdentity(binary),
		config.GOWORK != "" && config.GOWORK != "off" || replaceDirective.Match(mod)}
	data, _ = json.Marshal(state)
	if err := atomicFile(path, data); err != nil {
		return false, err
	}
	return !bootstrap, nil
}

func fileIdentity(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s:%d:%d", path, info.Size(), info.ModTime().UnixNano())
}

func checkoutEnvironment(root string) (string, error) {
	h := sha256.New()
	for _, name := range strings.Fields("PATH GOOS GOARCH CGO_ENABLED GOFLAGS GOEXPERIMENT GOTOOLCHAIN GOROOT GOWORK GO111MODULE GOENV GOCACHEPROG GOFIPS140 GOPATH GOMODCACHE CC CXX PKG_CONFIG CGO_CFLAGS CGO_CPPFLAGS CGO_CXXFLAGS CGO_FFLAGS CGO_LDFLAGS GOAMD64 GOARM GOARM64 GO386 GORISCV64 GOMIPS GOMIPS64 GOPPC64 GOWASM ASDF_GOLANG_VERSION") {
		fmt.Fprintf(h, "%s=%s\x00", name, os.Getenv(name))
	}
	goPath, err := exec.LookPath("go")
	if err != nil {
		return "", err
	}
	fmt.Fprint(h, fileIdentity(goPath))
	config := os.Getenv("GOENV")
	if config == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			config = filepath.Join(dir, "go", "env")
		}
	}
	if config != "off" {
		data, _ := os.ReadFile(config)
		fmt.Fprintf(h, "%s\x00%s\x00", config, data)
	}
	// A toolchain manager's shim may keep its time when its selection changes.
	for dir := root; ; dir = filepath.Dir(dir) {
		data, _ := os.ReadFile(filepath.Join(dir, ".tool-versions"))
		fmt.Fprintf(h, "%s\x00%s\x00", dir, data)
		if filepath.Dir(dir) == dir {
			break
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func checkoutRevision(root string) string {
	dir := filepath.Join(root, ".git")
	if data, err := os.ReadFile(dir); err == nil && strings.HasPrefix(string(data), "gitdir: ") {
		gitDir := strings.TrimSpace(strings.TrimPrefix(string(data), "gitdir: "))
		if filepath.IsAbs(gitDir) {
			dir = gitDir
		} else {
			dir = filepath.Join(root, gitDir)
		}
	}
	head, _ := os.ReadFile(filepath.Join(dir, "HEAD"))
	if !strings.HasPrefix(string(head), "ref: ") {
		return string(head)
	}
	if common, err := os.ReadFile(filepath.Join(dir, "commondir")); err == nil {
		commonDir := strings.TrimSpace(string(common))
		if filepath.IsAbs(commonDir) {
			dir = commonDir
		} else {
			dir = filepath.Join(dir, commonDir)
		}
	}
	ref := strings.TrimSpace(strings.TrimPrefix(string(head), "ref: "))
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(ref)))
	if err != nil {
		data, _ = os.ReadFile(filepath.Join(dir, "packed-refs"))
	}
	return string(head) + string(data)
}

func checkoutChanged(root string, started int64) (bool, error) {
	changed := false
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != root && entry.IsDir() && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules" || entry.Name() == "bin") {
			return filepath.SkipDir
		}
		if entry.Name() == ".DS_Store" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.ModTime().UnixNano() > started {
			changed = true
			return fs.SkipAll
		}
		return nil
	})
	return changed, err
}

func atomicFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".eye-tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
