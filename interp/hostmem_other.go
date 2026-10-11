//go:build !darwin && !linux

package interp

// hostMemory returns the bytes of memory the host has, or 0 if unknown.
func hostMemory() int64 { return 0 }
