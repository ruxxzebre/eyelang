//go:build darwin || freebsd || linux || netbsd

package ffi

import "github.com/ebitengine/purego"

func open(path string) (uintptr, error) {
	return purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
}

func lookup(handle uintptr, symbol string) (uintptr, error) {
	return purego.Dlsym(handle, symbol)
}

func register(fptr any, addr uintptr) { purego.RegisterFunc(fptr, addr) }
