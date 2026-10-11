package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"eyelang/syntax"
)

// repoServer is a server whose workspace is this repository.
func repoServer(t *testing.T) *server {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return &server{docs: map[string]*document{}, folders: []string{root}}
}

// TestKeywordTable checks that every keyword's section and implementations
// are still where the table says.
func TestKeywordTable(t *testing.T) {
	s := repoServer(t)
	for word, k := range keywords {
		if _, ok := s.specLocation(k.spec); !ok {
			t.Errorf("%s: SPECS.md has no heading %q", word, k.spec)
		}
		if k.summary == "" || !strings.HasSuffix(k.summary, ".") {
			t.Errorf("%s: the summary should be a sentence", word)
		}
		for _, g := range k.impl {
			if _, ok := s.goTextLocation(g); !ok {
				t.Errorf("%s: %s has no %q", word, g.File, g.Text)
			}
		}
	}
}

// TestEveryKeywordLeadsSomewhere checks every keyword and operator in the
// repository's programs, the prelude and std: each has a definition in eye's
// Go, and each keyword has a summary.
func TestEveryKeywordLeadsSomewhere(t *testing.T) {
	s := repoServer(t)
	var files []string
	for _, dir := range []string{"../../testdata/lab", "../../testdata/programs", "../../check"} {
		_ = filepath.WalkDir(dir, func(p string, e os.DirEntry, err error) error {
			if err == nil && !e.IsDir() && strings.HasSuffix(p, ".eye") {
				files = append(files, p)
			}
			return nil
		})
	}
	missing := map[string]bool{}
	checked := map[string]bool{}
	undocumented := map[string]bool{}
	for _, p := range files {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		d := newDocument(pathURI(p), 1, src)
		d.file.Root.Walk(func(n *syntax.Node) bool {
			isKeyword := n.Kind == syntax.Keyword || n.Kind.IsKeyword() || isOperator(n.Kind)
			if !n.IsToken() || !isKeyword {
				return true
			}
			tok, parent := keywordAt(d, n.Span.Start)
			if tok == nil {
				missing[n.Text(src)+" (not found at its own start)"] = true
				return true
			}
			key := tok.Text(src) + " in " + parent.Kind.String()
			if !checked[key] {
				checked[key] = true // each pair once: the answer is the same
				if len(s.keywordImplementations(d, n.Span.Start)) == 0 {
					missing[key] = true
				}
			}
			if tok.Kind == syntax.Keyword || tok.Kind.IsKeyword() {
				if _, ok := keywords[tok.Text(src)]; !ok {
					undocumented[tok.Text(src)] = true
				}
			}
			return true
		})
	}
	report := func(what string, set map[string]bool) {
		var list []string
		for k := range set {
			list = append(list, k)
		}
		sort.Strings(list)
		if len(list) > 0 {
			t.Errorf("%s:\n  %s", what, strings.Join(list, "\n  "))
		}
	}
	report("tokens that lead nowhere", missing)
	report("keywords without a summary", undocumented)
	if len(files) < 20 {
		t.Fatalf("checked only %d files", len(files))
	}
}

// TestKeywordsAndComments goes from a keyword to the checker's code and its
// section of SPECS.md, from a name in a comment to its declaration, and from
// a variant only the runtime makes to the Go that makes it.
func TestKeywordsAndComments(t *testing.T) {
	s := repoServer(t)
	src := "## Asks for a None, or a Shape.\ncontract Shapes {\n  query size(s: String) -> Int\n}\n"
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "k.eye"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	d := newDocument(pathURI(filepath.Join(dir, "k.eye")), 1, []byte(src))
	s.docs[d.uri] = d
	at := func(line, char int) Position { return Position{Line: line, Character: char} }
	where := func(locs []location) string {
		if len(locs) == 0 {
			return "nothing"
		}
		p, _ := uriPath(locs[0].URI)
		return fmt.Sprintf("%s:%d", filepath.Base(p), locs[0].Range.Start.Line+1)
	}
	if got, _ := s.definition(d, at(1, 2)); !strings.HasPrefix(where(got.([]location)), "check.go:") {
		t.Errorf("contract goes to %s, want check/check.go", where(got.([]location)))
	}
	if got, _ := s.declaration(d, at(1, 2)); !strings.HasPrefix(where(got.([]location)), "SPECS.md:") {
		t.Errorf("contract's declaration is %s, want SPECS.md", where(got.([]location)))
	}
	if locs := s.keywordImplementations(d, d.offset(at(1, 2))); len(locs) < 3 {
		t.Errorf("contract has %d implementations, want the checker, the parser and the grammar at least", len(locs))
	}
	if got, _ := s.definition(d, at(0, 16)); where(asLocations(got)) != "prelude.eye:"+preludeLine(t, "| None") {
		t.Errorf("None in a comment goes to %s", where(asLocations(got)))
	}
	if locs := s.goSites("Unavailable"); len(locs) == 0 || !strings.Contains(locs[0].URI, "/interp/") {
		t.Errorf("Unavailable is made in %v, want the interpreter", locs)
	}
}

func asLocations(v any) []location {
	locs, _ := v.([]location)
	return locs
}

// preludeLine is the line of the prelude that holds text.
func preludeLine(t *testing.T, text string) string {
	src, err := os.ReadFile("../../check/prelude.eye")
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(string(src), "\n") {
		if strings.Contains(line, text) {
			return fmt.Sprint(i + 1)
		}
	}
	return "?"
}
