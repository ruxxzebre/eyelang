//go:build windows

package ffi

import (
	"syscall"

	"github.com/ebitengine/purego"
)

func open(path string) (uintptr, error) {
	h, err := syscall.LoadLibrary(path)
	return uintptr(h), err
}

func lookup(handle uintptr, symbol string) (uintptr, error) {
	return syscall.GetProcAddress(syscall.Handle(handle), symbol)
}

func register(fptr any, addr uintptr) { purego.RegisterFunc(fptr, addr) }
