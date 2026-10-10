package specdoc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTestdataUpToDate checks that testdata/syntax/valid holds every example
// of SPECS.md as it is now. Run `go run ./tools/specexamples` after changing an
// example.
func TestTestdataUpToDate(t *testing.T) {
	spec, err := os.ReadFile("../../SPECS.md")
	if err != nil {
		t.Fatal(err)
	}
	examples := Examples(spec)
	if len(examples) < 20 {
		t.Fatalf("found %d examples in SPECS.md, want at least 20", len(examples))
	}
	want := map[string]bool{}
	for _, ex := range examples {
		want[ex.Name] = true
		data, err := os.ReadFile(filepath.Join("../../testdata/syntax/valid", ex.Name))
		if err != nil {
			t.Errorf("%s is missing; run go run ./tools/specexamples", ex.Name)
			continue
		}
		if !strings.HasSuffix(string(data), ex.Source) {
			t.Errorf("%s doesn't match SPECS.md; run go run ./tools/specexamples", ex.Name)
		}
	}
	files, _ := filepath.Glob("../../testdata/syntax/valid/spec-*.eye")
	for _, f := range files {
		if !want[filepath.Base(f)] {
			t.Errorf("%s has no example in SPECS.md; run go run ./tools/specexamples", filepath.Base(f))
		}
	}
}

func TestExamplesWrapStatements(t *testing.T) {
	spec := "## 3. Tour\n\n```eye\nfn f() = 1\n```\n\n- A list item:\n\n  ```eye\n  let a = 1\n  a += 1\n  ```\n"
	examples := Examples([]byte(spec))
	if len(examples) != 2 {
		t.Fatalf("got %d examples, want 2", len(examples))
	}
	if examples[0].Wrapped || examples[0].Name != "spec-03-tour-1.eye" {
		t.Errorf("first example = %+v, want an unwrapped spec-03-tour-1.eye", examples[0])
	}
	if !examples[1].Wrapped || examples[1].Source != "fn example() {\n  let a = 1\n  a += 1\n}\n" {
		t.Errorf("second example = %+v, want it wrapped in a function", examples[1])
	}
}
