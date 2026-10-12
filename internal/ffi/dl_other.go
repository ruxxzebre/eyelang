//go:build !(darwin || freebsd || linux || netbsd || windows)

package ffi

import (
	"fmt"
	"runtime"
)

func open(string) (uintptr, error) {
	return 0, fmt.Errorf("loading C libraries isn't supported on %s", runtime.GOOS)
}

func lookup(uintptr, string) (uintptr, error) {
	return 0, fmt.Errorf("loading C libraries isn't supported on %s", runtime.GOOS)
}

func register(any, uintptr) {}
