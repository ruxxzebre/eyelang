package syntax

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The conformance suite checks that this package and the tree-sitter grammar
// agree. Every file in testdata/syntax, and every input of the tree-sitter
// corpus, is parsed by both. They must agree on whether it parses, and for a
// file that parses, their trees must be the same node for node: kinds,
// fields, spans, flags and trivia (oracle_test.go converts tree-sitter's
// tree). The grammar is compiled into the test, so a C compiler is needed,
// and no tree-sitter CLI.

type conformanceCase struct {
	name  string
	src   []byte
	valid bool
}

func conformanceCases(t *testing.T) []conformanceCase {
	t.Helper()
	var cases []conformanceCase
	for _, dir := range []string{"valid", "invalid"} {
		files, err := filepath.Glob(filepath.Join("..", "testdata", "syntax", dir, "*.eye"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			cases = append(cases, conformanceCase{name: dir + "/" + filepath.Base(f), src: src, valid: dir == "valid"})
		}
	}
	for _, c := range corpusCases(t) {
		cases = append(cases, conformanceCase{name: "corpus/" + c.file + "/" + c.name, src: []byte(c.input), valid: !c.wantError})
	}
	return cases
}

func TestConformance(t *testing.T) {
	cases := conformanceCases(t)
	valid, invalid := 0, 0
	for _, c := range cases {
		if strings.HasPrefix(c.name, "corpus/") {
			continue
		}
		if c.valid {
			valid++
		} else {
			invalid++
		}
	}
	if valid < 40 || invalid < 20 {
		t.Errorf("testdata/syntax has %d valid and %d invalid files; the suite needs at least 40 and 20", valid, invalid)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := agree(c.src); diff != "" {
				t.Error(diff)
			}
			f := Parse(c.src)
			if ok := len(f.Diagnostics) == 0 && !f.Root.HasError(); ok != c.valid {
				t.Errorf("eye parses it: %v, want %v; diagnostics: %v", ok, c.valid, f.Diagnostics)
			}
		})
	}
}

// agree parses src with both parsers and returns how they disagree, or "".
// They must agree on whether src parses; if it does, their trees must be
// equal, node for node.
func agree(src []byte) string {
	f := Parse(src)
	eyeOK := len(f.Diagnostics) == 0 && !f.Root.HasError()
	if tsOK := tsAccepts(src); tsOK != eyeOK {
		return fmt.Sprintf("tree-sitter parses it: %v, eye: %v; eye's diagnostics: %v", tsOK, eyeOK, f.Diagnostics)
	}
	if !eyeOK {
		return ""
	}
	got, tree := parseTS(src)
	tree.Close()
	return sameTree(f.Root, got.Root, src, "source_file")
}

// firstDifference shows where two strings start to differ.
func firstDifference(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	from := max(0, i-60)
	return fmt.Sprintf("first difference at byte %d:\n  eye: …%s\n   ts: …%s", i, a[from:min(len(a), i+40)], b[from:min(len(b), i+40)])
}
