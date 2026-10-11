package interp

import (
	"encoding/binary"
	"syscall"
)

// hostMemory returns the bytes of memory the host has, or 0 if unknown.
func hostMemory() int64 {
	s, err := syscall.Sysctl("hw.memsize")
	if err != nil {
		return 0
	}
	b := []byte(s)
	for len(b) < 8 {
		b = append(b, 0) // Sysctl drops trailing zero bytes
	}
	return int64(binary.LittleEndian.Uint64(b[:8]))
}
