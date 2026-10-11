package ir

import (
	"strings"
	"testing"

	"eyelang/syntax"
)

// TestBuiltinFile checks builtin.eye: every function and type has a group
// and a doc comment whose first sentence stands on its own, and no name is
// declared twice.
func TestBuiltinFile(t *testing.T) {
	if len(BuiltinDocs) < 50 || len(BuiltinTypeDocs) < 30 {
		t.Fatalf("builtin.eye declares %d functions and %d types", len(BuiltinDocs), len(BuiltinTypeDocs))
	}
	seen := map[string]bool{}
	for _, d := range append(append([]BuiltinDoc{}, BuiltinDocs...), BuiltinTypeDocs...) {
		if seen[d.Name] {
			t.Errorf("%s is declared twice", d.Name)
		}
		seen[d.Name] = true
		if d.Group == "" {
			t.Errorf("%s has no group: put it under a `# == Group` line", d.Name)
		}
		summary := syntax.Summary(d.Doc)
		if summary == "" || !strings.HasSuffix(summary, ".") {
			t.Errorf("%s: its doc should start with a sentence, found %q", d.Name, summary)
		}
	}
	for _, name := range Builtins {
		if !seen[name] {
			t.Errorf("%s is a built-in without a declaration in builtin.eye", name)
		}
	}
}
