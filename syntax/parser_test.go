package syntax

import (
	"os"
	"path/filepath"
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

// TestErrorRecovery checks that independent errors are each reported once and
// that the parser keeps going after each.
func TestErrorRecovery(t *testing.T) {
	src := `fn first() {
  let a = (1 +
}

fn second() = [1, 2

type Third = | A(Int | B

fn fourth() = ok
`
	f := Parse([]byte(src))
	if got := string(f.Root.Source(f.Src)); got != src {
		t.Fatalf("the tree does not reproduce the source:\n%s", got)
	}
	var lines []int
	for _, d := range f.Diagnostics {
		lines = append(lines, f.Lines.Position(d.Span.Start).Line)
	}
	if len(f.Diagnostics) != 3 {
		t.Errorf("got %d diagnostics on lines %v, want 3:\n%v", len(f.Diagnostics), lines, f.Diagnostics)
	}
	// The declaration after the errors still parses.
	last := f.Root.Children[len(f.Root.Children)-2]
	if last.Kind != FunctionDeclaration || last.HasError() {
		t.Errorf("last declaration = %s, want a clean function_declaration", last.SExpr(f.Src, SExprOptions{Fields: true}))
	}
}

// FuzzParse feeds the parser mutations of the corpus, the spec's examples and
// every Eyelang file in the repository, and checks, for each input, that
//
//   - the parser neither panics nor gets stuck: it runs out of fuel instead
//     of hanging;
//   - the tree reproduces the source byte for byte, errors and all;
//   - a tree with an error has a diagnostic;
//   - the parser and the tree-sitter grammar agree on whether the input
//     parses, and when it does, their trees are equal, node for node.
//
// `go test` runs the seeds and every input saved in testdata/fuzz, so a
// failure found once stays a test; `task fuzz` searches for new ones.
func FuzzParse(f *testing.F) {
	for _, src := range exampleInputs(f) {
		f.Add(src)
	}
	files := eyeFiles(f)
	invalid, _ := filepath.Glob(filepath.Join("..", "testdata", "syntax", "invalid", "*.eye"))
	for _, path := range append(files, invalid...) {
		src, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(string(src))
	}
	f.Fuzz(func(t *testing.T, src string) {
		file := Parse([]byte(src))
		if got := string(file.Root.Source(file.Src)); got != src {
			t.Fatalf("the tree does not reproduce the source\n%s", firstDifference(src, got))
		}
		if file.Root.HasError() && len(file.Diagnostics) == 0 {
			t.Fatalf("the tree has an error but no diagnostic was reported:\n%s",
				file.Root.SExpr(file.Src, SExprOptions{Fields: true, Tokens: true, Text: true}))
		}
		if diff := agree([]byte(src)); diff != "" {
			t.Fatalf("%s\ninput: %q", diff, src)
		}
	})
}

func TestTypedViews(t *testing.T) {
	src := []byte("pub fn area(w: Float, h: Float) -> Float = w * h\n")
	f := Parse(src)
	fn, ok := AsFunctionDeclaration(f.Root.Children[0])
	if !ok {
		t.Fatalf("first item is %s, want function_declaration", f.Root.Children[0].Kind)
	}
	if got := fn.Name().Text(src); got != "area" {
		t.Errorf("Name() = %q, want area", got)
	}
	params, _ := AsParameters(fn.Parameters())
	if got := len(params.Items()); got != 2 {
		t.Errorf("%d parameters, want 2", got)
	}
	body, ok := AsBinaryExpression(fn.Body())
	if !ok || body.Operator().Text(src) != "*" || !body.Left().Kind.IsExpression() {
		t.Errorf("body = %s, want w * h", fn.Body().SExpr(src, SExprOptions{Tokens: true, Text: true}))
	}
	if _, ok := AsJobDeclaration(f.Root.Children[0]); ok {
		t.Error("a function declaration is viewed as a job")
	}
}
