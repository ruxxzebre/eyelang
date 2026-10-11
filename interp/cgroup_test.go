package interp

import (
	"os"
	"path/filepath"
	"testing"
)

// A runner without --memory-limit takes its cgroup's limit, such as a
// Kubernetes pod's, when that is lower than the host's memory.
func TestRunnerMemoryFromTheCgroup(t *testing.T) {
	const host = 16 << 30
	for _, c := range []struct {
		name  string
		files map[string]string
		want  int64
		from  string
	}{
		{"no cgroup files", nil, host, "the host's memory"},
		{"v2, the container's own cgroup as the root", map[string]string{
			"proc/self/cgroup":         "0::/\n",
			"sys/fs/cgroup/memory.max": "536870912\n",
		}, 512 << 20, "the cgroup's limit"},
		{"v2 without a limit", map[string]string{
			"proc/self/cgroup":         "0::/\n",
			"sys/fs/cgroup/memory.max": "max\n",
		}, host, "the host's memory"},
		{"v2, a limit above the host's memory", map[string]string{
			"proc/self/cgroup":         "0::/\n",
			"sys/fs/cgroup/memory.max": "68719476736\n",
		}, host, "the host's memory"},
		{"v2 without a cgroup namespace: the lowest limit on the way up", map[string]string{
			"proc/self/cgroup":                          "0::/kubepods/pod1/c1\n",
			"sys/fs/cgroup/kubepods/pod1/c1/memory.max": "max\n",
			"sys/fs/cgroup/kubepods/pod1/memory.max":    "402653184\n",
			"sys/fs/cgroup/kubepods/memory.max":         "8589934592\n",
		}, 384 << 20, "the cgroup's limit"},
		{"v1", map[string]string{
			"proc/self/cgroup": "12:cpu,cpuacct:/kubepods/pod1\n4:memory:/kubepods/pod1\n",
			"sys/fs/cgroup/memory/kubepods/pod1/memory.limit_in_bytes": "268435456\n",
		}, 256 << 20, "the cgroup's limit"},
		{"v1 without a limit", map[string]string{
			"proc/self/cgroup":                           "4:memory:/\n",
			"sys/fs/cgroup/memory/memory.limit_in_bytes": "9223372036854771712\n",
		}, host, "the host's memory"},
	} {
		root := t.TempDir()
		for name, data := range c.files {
			path := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if got, from := runnerMemory(root, host); got != c.want || from != c.from {
			t.Errorf("%s: %d from %s, want %d from %s", c.name, got, from, c.want, c.from)
		}
	}
	// A host whose memory isn't known takes the cgroup's.
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "sys/fs/cgroup"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "sys/fs/cgroup/memory.max"), []byte("1048576\n"), 0o644)
	if got, _ := runnerMemory(root, 0); got != 1<<20 {
		t.Errorf("with the host unknown: %d", got)
	}
}
