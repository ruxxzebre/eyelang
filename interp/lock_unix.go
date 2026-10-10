//go:build unix

package interp

import (
	"os"
	"syscall"
)

// lockFile takes an exclusive lock on f, or fails at once if another process
// holds one. The lock goes when the process does, however it ends.
func lockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

// freeSpace returns the bytes free to this user on dir's disk, or -1.
func freeSpace(dir string) int64 {
	var st syscall.Statfs_t
	if syscall.Statfs(dir, &st) != nil {
		return -1
	}
	return int64(st.Bavail) * int64(st.Bsize) //nolint:unconvert // Bsize's type differs between systems
}
