//go:build unix

package benchmarks

import (
	"os"
	"runtime"
	"syscall"
)

// peakMemory is the most memory a finished process held. getrusage gives
// bytes on macOS and kibibytes elsewhere.
func peakMemory(state *os.ProcessState) int64 {
	usage, ok := state.SysUsage().(*syscall.Rusage)
	if !ok {
		return 0
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "ios" {
		return usage.Maxrss
	}
	return usage.Maxrss * 1024
}
