package gotool

import (
	"os"
	"strings"
	"testing"
)

// TestVersionIsGoMods checks that the pinned Go is the one go.mod names,
// so a program links the runner built by the Go its tests ran with.
func TestVersionIsGoMods(t *testing.T) {
	data, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if v, ok := strings.CutPrefix(line, "go "); ok {
			if strings.TrimSpace(v) != Version {
				t.Errorf("go.mod says go %s, and internal/gotool pins %s: change them together", v, Version)
			}
			return
		}
	}
	t.Fatal("go.mod has no go line")
}

// TestArchivesNameVersion checks that every pinned download is of Version.
func TestArchivesNameVersion(t *testing.T) {
	for plat, a := range archives {
		if !strings.HasPrefix(a.file, "go"+Version+".") {
			t.Errorf("%s downloads %s, not Go %s", plat, a.file, Version)
		}
		if len(a.sha256) != 64 {
			t.Errorf("%s's checksum %q isn't a SHA-256", plat, a.sha256)
		}
	}
}
