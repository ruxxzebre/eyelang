package ffi

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestVersioned(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"libq.so.1.2", "libq.so.1", "libq.so.2", "libq.so.x", "libqq.so.3"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for _, p := range versioned(dir, "libq.so.") {
		got = append(got, filepath.Base(p))
	}
	if want := []string{"libq.so.2", "libq.so.1", "libq.so.1.2"}; !slices.Equal(got, want) {
		t.Errorf("versioned = %q, want %q", got, want)
	}
}

func TestCandidates(t *testing.T) {
	for _, name := range []string{"libz.so.1", "/opt/lib/libz.so", "z.dll"} {
		if got := Candidates(name); !slices.Equal(got, []string{name}) {
			t.Errorf("Candidates(%q) = %q, want only the name", name, got)
		}
	}
	if runtime.GOOS == "linux" {
		got := Candidates("z")
		if len(got) == 0 || got[0] != "libz.so" {
			t.Errorf("Candidates(z) = %q, want libz.so first", got)
		}
	}
}

func TestFunc(t *testing.T) {
	l, err := Open("z")
	if err != nil {
		t.Skipf("no zlib to load: %v", err)
	}
	crc, err := l.Func("crc32", reflect.TypeFor[func(uint64, []byte, uint32) uint64]())
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("hello")
	out := crc.Call([]reflect.Value{reflect.ValueOf(uint64(0)), reflect.ValueOf(data), reflect.ValueOf(uint32(len(data)))})
	if got := out[0].Uint(); got != 907060870 {
		t.Errorf("crc32(hello) = %d, want 907060870", got)
	}
	if _, err := l.Func("crc32", reflect.TypeFor[func(...uint64) uint64]()); err == nil || !strings.Contains(err.Error(), "at most one result") {
		t.Errorf("a variadic Go type: %v", err)
	}
	if _, err := l.Func("noSuchSymbol", reflect.TypeFor[func()]()); err == nil || !strings.HasSuffix(err.Error(), "has no symbol noSuchSymbol") {
		t.Errorf("a missing symbol: %v", err)
	}
	if _, err := Open("eyelang-no-such-library"); err == nil || !strings.HasPrefix(err.Error(), `can't load the library "eyelang-no-such-library": `) {
		t.Errorf("a missing library: %v", err)
	}
}
