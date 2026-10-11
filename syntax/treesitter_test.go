package syntax

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTreeSitterAgrees checks that the compiled-in grammar accepts every
// valid conformance file and rejects every invalid one.
func TestTreeSitterAgrees(t *testing.T) {
	for _, dir := range []string{"valid", "invalid"} {
		files, _ := filepath.Glob(filepath.Join("..", "testdata", "syntax", dir, "*.eye"))
		if len(files) == 0 {
			t.Fatalf("no files in testdata/syntax/%s", dir)
		}
		for _, f := range files {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			tree := tsParse(src)
			if bad := tree.RootNode().HasError(); bad != (dir == "invalid") {
				t.Errorf("%s: tree-sitter reports an error: %v", f, bad)
			}
			tree.Close()
		}
	}
}
