//go:build !unix

package benchmarks

import "os"

// peakMemory is unknown here, and the benchmarks leave it out.
func peakMemory(*os.ProcessState) int64 { return 0 }
