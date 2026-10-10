package check

import (
	"path/filepath"
	"strings"
	"testing"

	"eyelang/syntax"
)

// unresolvable are the names the index may leave without a target, with
// why. Every other identifier in the repository's programs, the prelude
// and the standard modules leads somewhere, so go-to-definition and find
// references work on it.
var unresolvable = map[string]string{
	"unix":     "an Instant method only the interpreter knows; the checker doesn't",
	"end":      "a Range's field, built into the checker",
	"name":     "self.name() in a trait's default method: self is any Named",
	"solution": "a field in a record pattern, which takes its type from the value matched",
	"nick":     "a field of an entity's older version, in a migration",
	"nickname": "a field of an entity's older version, in a migration",
	"Refused":  "a variant that two imported modules declare, matched by name",
}

// TestEveryNameResolves checks every identifier in the programs under
// testdata, and in the modules eye carries, against the index.
func TestEveryNameResolves(t *testing.T) {
	roots := []string{"../testdata/lab"}
	programs, _ := filepath.Glob("../testdata/programs/*")
	roots = append(roots, programs...)
	seen := map[string]bool{}
	for _, root := range roots {
		res := Check(root, Options{All: true})
		for _, d := range res.Diagnostics {
			t.Errorf("%s: %s", root, d)
		}
		for _, m := range res.Program.Modules {
			for _, f := range m.Files {
				if seen[f.Path] {
					continue
				}
				seen[f.Path] = true
				f.Tree.Root.Walk(func(n *syntax.Node) bool {
					if n.Kind == syntax.Effect {
						// `uses {net, wait}`: effects have no declaration.
						return false
					}
					if n.Kind != syntax.Ident && n.Kind != syntax.TypeIdent {
						return true
					}
					name := n.Text(f.Src)
					if _, ok := res.Index.At(f.Path, n.Span.Start); ok || unresolvable[name] != "" {
						return true
					}
					at := f.Tree.Lines.Position(n.Span.Start)
					t.Errorf("%s:%d:%d: %s leads nowhere", strings.TrimPrefix(f.Path, "../"), at.Line, at.Column, name)
					return true
				})
			}
		}
	}
	if len(seen) < 20 {
		t.Fatalf("checked only %d files", len(seen))
	}
}
