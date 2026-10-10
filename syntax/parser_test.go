package syntax

import (
	"regexp"
	"strings"
	"testing"
)

var (
	spaces       = regexp.MustCompile(`\s+`)
	commentNodes = regexp.MustCompile(`\s*\(comment\)`)
)

// normalizeTree removes comments, which are trivia in this parser, and
// collapses whitespace in a tree printed in tree-sitter's corpus format.
func normalizeTree(s string) string {
	s = commentNodes.ReplaceAllString(s, "")
	s = spaces.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "( ", "(")
	return strings.TrimSpace(strings.ReplaceAll(s, " )", ")"))
}

// TestCorpus parses every test of the tree-sitter corpus and compares the
// tree with tree-sitter's expected one.
func TestCorpus(t *testing.T) {
	for _, c := range corpusCases(t) {
		t.Run(c.file+"/"+c.name, func(t *testing.T) {
			f := Parse([]byte(c.input))
			if got := string(f.Root.Source(f.Src)); got != c.input {
				t.Fatalf("the tree does not reproduce the source:\n%s", got)
			}
			if c.wantError {
				if len(f.Diagnostics) == 0 {
					t.Errorf("parsed without errors, want an error:\n%s\n%s", c.input,
						f.Root.SExpr(f.Src, SExprOptions{Fields: true, Indent: true}))
				}
				return
			}
			for _, d := range f.Diagnostics {
				pos := f.Lines.Position(d.Span.Start)
				t.Errorf("%d:%d: %s", pos.Line, pos.Column, d.Message)
			}
			got := normalizeTree(f.Root.SExpr(f.Src, SExprOptions{}))
			if want := normalizeTree(c.tree); got != want {
				t.Errorf("tree differs from tree-sitter's\ninput:\n%s\n got: %s\nwant: %s", c.input, got, want)
			}
		})
	}
}
