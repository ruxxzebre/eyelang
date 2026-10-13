package interp

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// runnerMemory is the memory a runner takes when it isn't told how much it
// has: the limit of the cgroup it runs in, such as a Kubernetes pod's, when
// that is lower than the host's memory, and the host's memory otherwise.
// root is where the files are read from, "/" but in tests. It says which it
// took, for the log.
func runnerMemory(root string, host int64) (int64, string) {
	if cg := cgroupMemory(root); cg > 0 && (host <= 0 || cg < host) {
		return cg, "the cgroup's limit"
	}
	return host, hostMemoryFrom
}

// hostMemoryFrom is what runnerMemory says when it took the host's memory.
const hostMemoryFrom = "the host's memory"

// cgroupMemory returns the memory limit of the cgroup this process runs in,
// or 0 if it has none. With cgroup v2 it is the lowest memory.max from the
// process's cgroup up to the root; with v1, memory.limit_in_bytes of its
// memory cgroup. A container usually sees its own cgroup as the root.
func cgroupMemory(root string) int64 {
	var v2, v1 string // the process's cgroup in each hierarchy
	if data, err := os.ReadFile(filepath.Join(root, "proc/self/cgroup")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			parts := strings.SplitN(line, ":", 3) // 0::/kubepods/… or 4:memory:/kubepods/…
			if len(parts) != 3 {
				continue
			}
			switch {
			case parts[0] == "0" && parts[1] == "":
				v2 = parts[2]
			case slices.Contains(strings.Split(parts[1], ","), "memory"):
				v1 = parts[2]
			}
		}
	}
	var limit int64
	for dir := path.Clean("/" + v2); ; dir = path.Dir(dir) {
		if n := readLimit(filepath.Join(root, "sys/fs/cgroup", dir, "memory.max")); n > 0 && (limit == 0 || n < limit) {
			limit = n
		}
		if dir == "/" {
			break
		}
	}
	if limit > 0 {
		return limit
	}
	for _, dir := range []string{path.Clean("/" + v1), "/"} {
		// v1 says "no limit" with a number near the largest int64.
		if n := readLimit(filepath.Join(root, "sys/fs/cgroup/memory", dir, "memory.limit_in_bytes")); n > 0 && n < 1<<62 {
			return n
		}
	}
	return 0
}

// readLimit reads a cgroup's memory limit in bytes: 0 for "max", or if the
// file isn't there.
func readLimit(file string) int64 {
	data, err := os.ReadFile(file)
	if err != nil {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0
	}
	return n
}
