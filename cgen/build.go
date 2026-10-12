package cgen

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"eyelang/internal/zig"
	"eyelang/ir"
	"eyelang/lir"
)

// The core runtime's C, compiled with every program.
//
//go:embed runtime/*.c runtime/*.h
var runtimeFS embed.FS

// Runtime returns the runtime's files.
func Runtime() []File {
	var out []File
	entries, _ := fs.ReadDir(runtimeFS, "runtime")
	for _, e := range entries {
		src, _ := runtimeFS.ReadFile("runtime/" + e.Name())
		out = append(out, File{Name: e.Name(), Src: src})
	}
	return out
}

// Targets are the platforms eye build compiles for, as GOOS/GOARCH, and
// the zig target of each: the release targets (.github/workflows/release.yml).
// Linux binaries are linked statically against musl.
var Targets = map[string]string{
	"linux/amd64":   "x86_64-linux-musl",
	"linux/arm64":   "aarch64-linux-musl",
	"darwin/amd64":  "x86_64-macos",
	"darwin/arm64":  "aarch64-macos",
	"windows/amd64": "x86_64-windows-gnu",
}

// TargetNames lists Targets' names in order.
func TargetNames() []string {
	var names []string
	for n := range Targets {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// HostTarget is the target of the machine eye runs on, or "" when it isn't
// one of Targets.
func HostTarget() string {
	t := runtime.GOOS + "/" + runtime.GOARCH
	if _, ok := Targets[t]; ok {
		return t
	}
	return ""
}

// BuildOptions says what to build and how.
type BuildOptions struct {
	Out    string // the binary to write
	Target string // one of Targets; "" for the machine eye runs on
	Tests  bool   // a binary that runs the program's tests
	Debug  bool   // -O0 with debug information, as for tests, instead of -O2
	// EmitC, if set, is a directory to write the C into, instead of
	// compiling it.
	EmitC string
	// Runner links the program into the Go runner (runner.go): the
	// runner is the binary's main, and calls the program's C through cgo.
	Runner bool
	// Archive writes a static library to Out instead of a binary, its
	// main renamed eye_main for the runner to call.
	Archive bool
	// Progress, if set, hears about fetching zig.
	Progress io.Writer
}

// Build compiles a checked program to a native binary: it lowers it, writes
// its C and the runtime's into a temporary directory, compiles each file
// with zig cc, reusing object files from the cache by the hash of what
// made them, and links them.
func Build(prog *ir.Program, opts BuildOptions) error {
	low, errs := lir.Lower(prog)
	if len(errs) > 0 {
		return joinErrors(errs)
	}
	files, err := Generate(prog, low, Options{Tests: opts.Tests})
	if err != nil {
		return err
	}
	files = append(Runtime(), files...)
	if opts.EmitC != "" {
		return WriteFiles(opts.EmitC, files)
	}
	if opts.Runner {
		return buildRunner(files, opts)
	}
	return Compile(files, opts)
}

// WriteFiles writes C files into dir, making it if needed.
func WriteFiles(dir string, files []File) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f.Name), f.Src, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Compile compiles C files, the runtime's among them, and links them into
// opts.Out.
func Compile(files []File, opts BuildOptions) error {
	target := opts.Target
	if target == "" {
		target = HostTarget()
	}
	triple, ok := Targets[target]
	if target != "" && !ok {
		return fmt.Errorf("eye build doesn't know the target %q: it builds for %s", target, strings.Join(TargetNames(), ", "))
	}
	exe, err := zig.Path(opts.Progress)
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "eye-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	if err := WriteFiles(work, files); err != nil {
		return err
	}
	cache, err := objectCache()
	if err != nil {
		return err
	}
	flags := []string{"-std=c11", "-fwrapv", "-fno-strict-aliasing", "-fno-sanitize=undefined"}
	if opts.Debug {
		flags = append(flags, "-O0", "-g")
	} else {
		flags = append(flags, "-O2")
	}
	if triple != "" {
		flags = append(flags, "-target", triple)
	}
	if opts.Archive {
		flags = append(flags, "-Dmain=eye_main")
	}
	units := compileUnits(files, !opts.Debug)
	for _, u := range units {
		if err := os.WriteFile(filepath.Join(work, u.Name), u.Src, 0o644); err != nil {
			return err
		}
	}
	var objs []string
	var mu sync.Mutex
	var failures []string
	var wg sync.WaitGroup
	sem := make(chan struct{}, max(runtime.NumCPU(), 2))
	for _, u := range units {
		f := u.File
		// The hash covers everything that makes the object: zig's version,
		// the flags and target, the file and every file it may include.
		h := sha256.New()
		fmt.Fprintf(h, "zig %s\x00%s\x00%s\x00", zig.Version, strings.Join(flags, " "), f.Name)
		h.Write(f.Src)
		for _, d := range u.deps {
			h.Write([]byte(d.Name + "\x00"))
			h.Write(d.Src)
		}
		obj := filepath.Join(cache, hex.EncodeToString(h.Sum(nil))+".o")
		objs = append(objs, obj)
		if _, err := os.Stat(obj); err == nil {
			continue
		}
		wg.Add(1)
		go func(f File, obj string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			tmp := filepath.Join(work, f.Name+".o")
			args := append([]string{"cc"}, flags...)
			args = append(args, "-c", filepath.Join(work, f.Name), "-o", tmp)
			if strings.HasPrefix(f.Name, "m_") || strings.HasPrefix(f.Name, "program") || f.Name == unitName {
				args = append(args, "-w") // the generated C has unused labels and values
			}
			if out, err := exec.Command(exe, args...).CombinedOutput(); err != nil {
				mu.Lock()
				failures = append(failures, fmt.Sprintf("compiling %s: %v\n%s", f.Name, err, out))
				mu.Unlock()
				return
			}
			if err := os.Rename(tmp, obj); err != nil {
				data, rerr := os.ReadFile(tmp)
				if rerr == nil {
					err = os.WriteFile(obj, data, 0o644)
				}
				if err != nil {
					mu.Lock()
					failures = append(failures, err.Error())
					mu.Unlock()
				}
			}
		}(f, obj)
	}
	wg.Wait()
	if len(failures) > 0 {
		sort.Strings(failures)
		return fmt.Errorf("%s", strings.Join(failures, "\n"))
	}
	out := opts.Out
	if opts.Archive {
		_ = os.Remove(out) // ar adds to an archive that is there
		args := append([]string{"ar", "rcs", out}, objs...)
		if outb, err := exec.Command(exe, args...).CombinedOutput(); err != nil {
			return fmt.Errorf("archiving %s: %v\n%s", out, err, outb)
		}
		prune(cache, objs)
		return nil
	}
	if strings.HasPrefix(target, "windows/") && !strings.HasSuffix(out, ".exe") {
		out += ".exe"
	}
	args := []string{"cc"}
	if triple != "" {
		args = append(args, "-target", triple)
	}
	args = append(args, "-o", out)
	args = append(args, objs...)
	args = append(args, "-lm")
	if outb, err := exec.Command(exe, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("linking %s: %v\n%s", out, err, outb)
	}
	prune(cache, objs)
	return nil
}

// unitName is the file that includes the runtime's C and the program's, to
// compile them as one translation unit.
const unitName = "eye_unit.c"

// A unit is a C file to compile, and the files it may include.
type unit struct {
	File
	deps []File
}

// compileUnits says what to compile. A build at -O2 compiles the runtime
// and the program as one translation unit, so that the runtime's small
// functions (comparisons, division, allocation, freeing) inline into the
// program's code; its object is cached as a whole, so a rebuild of an
// unchanged program only links. A debug build compiles each file on its
// own, in parallel, and reuses the runtime's objects across programs.
func compileUnits(files []File, one bool) []unit {
	var headers []File
	for _, f := range files {
		if strings.HasSuffix(f.Name, ".h") {
			headers = append(headers, f)
		}
	}
	if !one {
		var units []unit
		for _, f := range files {
			if strings.HasSuffix(f.Name, ".c") {
				units = append(units, unit{f, headers})
			}
		}
		return units
	}
	var b strings.Builder
	b.WriteString("/* Generated by eye build: the runtime and the program as one translation unit. */\n")
	// sys.c asks for POSIX's declarations, which must come before any
	// system header.
	b.WriteString("#define _POSIX_C_SOURCE 200809L\n")
	for _, f := range files {
		if strings.HasSuffix(f.Name, ".c") {
			fmt.Fprintf(&b, "#include \"%s\"\n", f.Name)
		}
	}
	return []unit{{File{Name: unitName, Src: []byte(b.String())}, files}}
}

// prune marks the objects a build used as used now, and removes those no
// build has used for 30 days, so the cache holds what recent builds need.
func prune(cache string, used []string) {
	now := time.Now()
	for _, o := range used {
		_ = os.Chtimes(o, now, now)
	}
	entries, err := os.ReadDir(cache)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".o") {
			continue
		}
		if info, err := e.Info(); err == nil && now.Sub(info.ModTime()) > 30*24*time.Hour {
			_ = os.Remove(filepath.Join(cache, e.Name()))
		}
	}
}

// objectCache is where compiled object files are kept by hash: the user's
// cache directory, or EYE_BUILD_CACHE when it is set.
func objectCache() (string, error) {
	dir := os.Getenv("EYE_BUILD_CACHE")
	if dir == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(cache, "eyelang", "build")
	}
	return dir, os.MkdirAll(dir, 0o755)
}
