//go:build !unix

package interp

import "os"

// lockFile doesn't lock on systems without flock; one eye run per journal is
// then up to the user.
func lockFile(*os.File) error { return nil }

// freeSpace isn't known on systems without statfs.
func freeSpace(string) int64 { return -1 }
