// Package ffi loads C functions from shared libraries at run time, without
// cgo. The interpreter calls a program's @library extern fns through it
// (SPECS.md §18), and the Go runner can load compiled programs with it
// (docs/internals/compilation.md, section 1).
//
// It uses github.com/ebitengine/purego, which calls the C library's own
// dlopen and dlsym, and C functions with the C calling convention, from Go.
// Linux has no system call that loads a shared library: dlopen lives in the
// C library and its dynamic linker, so a program without cgo either carries
// its own ELF loader, which would also have to set up libc for the library
// it loads, or calls the system's dlopen. purego does the second, and on
// Linux it makes the Go runtime start threads the way C expects. The cost is
// that a binary importing this package is linked dynamically on Linux, so
// the released eye, which is static, imports it only when built with
// `-tags clib` (cmd/eye/clib.go); on macOS and Windows it costs nothing.
package ffi

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Library is a shared library loaded into the process. It stays loaded
// until the process ends.
type Library struct {
	Name   string // as asked for: "z", "libz.so.1", or a path
	Path   string // what was loaded
	handle uintptr
}

// Open loads a library: a path, a file name such as libz.so.1, which the
// system's linker looks up, or a short name such as z, which is looked up
// as the system names libraries: libz.so, then the newest libz.so.N in the
// usual directories on Linux, libz.dylib on macOS, z.dll on Windows.
func Open(name string) (*Library, error) {
	if name == "" {
		return nil, errors.New("a library needs a name")
	}
	var tried []string
	for _, path := range Candidates(name) {
		h, err := open(path)
		if err == nil {
			return &Library{Name: name, Path: path, handle: h}, nil
		}
		tried = append(tried, err.Error())
	}
	return nil, fmt.Errorf("can't load the library %q: %s", name, strings.Join(tried, "; "))
}

// Func returns the C function symbol as a Go function of type typ, whose
// parameters and result are the C types purego maps them to: bool, the
// sized integers, float64 for double, string for a NUL-terminated char*
// (copied both ways), uintptr or unsafe.Pointer for a pointer, and a slice
// for a pointer to its first element. Nothing can check that typ matches
// the C function; a mismatch gives wrong values, or crashes.
func (l *Library) Func(symbol string, typ reflect.Type) (fn reflect.Value, err error) {
	if typ.Kind() != reflect.Func || typ.IsVariadic() || typ.NumOut() > 1 {
		return reflect.Value{}, fmt.Errorf("%s: a C function's Go type is a func with at most one result, not %s", symbol, typ)
	}
	addr, err := lookup(l.handle, symbol)
	if err != nil || addr == 0 {
		// dlerror's text names the file and the symbol again.
		return reflect.Value{}, fmt.Errorf("%s has no symbol %s", l.Path, symbol)
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s can't be called as %s: %v", symbol, typ, r)
		}
	}()
	p := reflect.New(typ)
	register(p.Interface(), addr)
	return p.Elem(), nil
}

var (
	loadedMu sync.Mutex
	loaded   = map[string]*Library{}
)

// Load returns the C function symbol of library as a Go function of type
// typ, loading each library once: what the interpreter takes to load
// @library extern fns (interp.UseCLoader).
func Load(library, symbol string, typ reflect.Type) (reflect.Value, error) {
	loadedMu.Lock()
	l, ok := loaded[library]
	if !ok {
		var err error
		if l, err = Open(library); err != nil {
			loadedMu.Unlock()
			return reflect.Value{}, err
		}
		loaded[library] = l
	}
	loadedMu.Unlock()
	return l.Func(symbol, typ)
}

// Candidates lists the files Open tries for a library name, in order.
func Candidates(name string) []string {
	if strings.ContainsRune(name, '/') || strings.ContainsRune(name, filepath.Separator) ||
		strings.Contains(name, ".so") || strings.HasSuffix(name, ".dylib") || strings.HasSuffix(name, ".dll") {
		return []string{name}
	}
	switch runtime.GOOS {
	case "darwin":
		out := []string{"lib" + name + ".dylib"}
		for _, dir := range []string{"/opt/homebrew/lib", "/usr/local/lib"} {
			out = append(out, filepath.Join(dir, "lib"+name+".dylib"))
		}
		return out
	case "windows":
		return []string{name + ".dll", "lib" + name + ".dll"}
	}
	// The unversioned libz.so is usually only there with the library's
	// development files, so the versioned ones are looked for too.
	out := []string{"lib" + name + ".so"}
	for _, dir := range searchDirs() {
		out = append(out, versioned(dir, "lib"+name+".so.")...)
	}
	return out
}

// versioned lists dir's files named prefix followed by a version, such as
// libz.so.1 and libz.so.1.3, the newest major version first, and within one
// the shortest name, which is the one programs link against.
func versioned(dir, prefix string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, prefix+"*"))
	type file struct {
		path  string
		major int
	}
	var files []file
	for _, m := range matches {
		v := strings.TrimPrefix(filepath.Base(m), prefix)
		major, err := strconv.Atoi(strings.SplitN(v, ".", 2)[0])
		if err != nil {
			continue
		}
		files = append(files, file{m, major})
	}
	sort.SliceStable(files, func(i, j int) bool {
		if files[i].major != files[j].major {
			return files[i].major > files[j].major
		}
		return len(files[i].path) < len(files[j].path)
	})
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.path
	}
	return out
}

// searchDirs are the directories the system's linker usually searches on
// Linux and the BSDs: LD_LIBRARY_PATH first.
func searchDirs() []string {
	var dirs []string
	for _, d := range filepath.SplitList(os.Getenv("LD_LIBRARY_PATH")) {
		if d != "" {
			dirs = append(dirs, d)
		}
	}
	if t, ok := triplets[runtime.GOARCH]; ok {
		dirs = append(dirs, "/lib/"+t, "/usr/lib/"+t)
	}
	return append(dirs, "/lib64", "/usr/lib64", "/lib", "/usr/lib", "/usr/local/lib")
}

// triplets are the multiarch directory names Debian and Ubuntu use.
var triplets = map[string]string{
	"amd64":   "x86_64-linux-gnu",
	"arm64":   "aarch64-linux-gnu",
	"386":     "i386-linux-gnu",
	"arm":     "arm-linux-gnueabihf",
	"riscv64": "riscv64-linux-gnu",
	"ppc64le": "powerpc64le-linux-gnu",
	"s390x":   "s390x-linux-gnu",
	"loong64": "loongarch64-linux-gnu",
}
