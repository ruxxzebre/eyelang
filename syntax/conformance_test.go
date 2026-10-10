package syntax

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// The conformance suite checks that this package and the tree-sitter grammar
// agree. Every file in testdata/syntax, and every input of the tree-sitter
// corpus, is parsed by both. They must agree on whether it parses, and for a
// file that parses, their trees must be the same token for token.
//
// tree-sitter-cli and a C compiler are needed for the tree-sitter half. Without
// them the test checks this parser only, unless EYE_CONFORMANCE is set, as it
// is in CI, in which case it fails.

type conformanceCase struct {
	name  string
	path  string
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
			cases = append(cases, conformanceCase{name: dir + "/" + filepath.Base(f), path: f, valid: dir == "valid"})
		}
	}
	tmp := t.TempDir()
	for i, c := range corpusCases(t) {
		path := filepath.Join(tmp, fmt.Sprintf("corpus-%03d.eye", i))
		if err := os.WriteFile(path, []byte(c.input), 0o644); err != nil {
			t.Fatal(err)
		}
		cases = append(cases, conformanceCase{name: "corpus/" + c.file + "/" + c.name, path: path, valid: !c.wantError})
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

	lib := buildGrammar(t)
	type result struct {
		sexp string
		ok   bool
		err  error
	}
	results := make([]result, len(cases))
	if lib != "" {
		var wg sync.WaitGroup
		sem := make(chan struct{}, runtime.NumCPU())
		for i, c := range cases {
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				sexp, ok, err := treeSitterParse(lib, c.path)
				results[i] = result{sexp, ok, err}
			}()
		}
		wg.Wait()
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src, err := os.ReadFile(c.path)
			if err != nil {
				t.Fatal(err)
			}
			f := Parse(src)
			goOK := len(f.Diagnostics) == 0 && !f.Root.HasError()
			if goOK != c.valid {
				t.Errorf("eye parses it: %v, want %v; diagnostics: %v", goOK, c.valid, f.Diagnostics)
			}
			if lib == "" {
				return
			}
			r := results[i]
			if r.err != nil {
				t.Fatalf("tree-sitter: %v", r.err)
			}
			if r.ok != goOK {
				t.Errorf("tree-sitter parses it: %v, eye: %v\n%s", r.ok, goOK, src)
				return
			}
			if !goOK {
				return
			}
			if got := f.Root.SExpr(f.Src, SExprOptions{Fields: true, Tokens: true, Text: true}); got != r.sexp {
				t.Errorf("trees differ\n%s\n  eye: %s\n   ts: %s\n%s", src, got, r.sexp, firstDifference(got, r.sexp))
			}
		})
	}
}

// buildGrammar compiles tree-sitter-eyelang into a shared library and returns
// its path, or "" when the tree-sitter half of the suite can't run.
func buildGrammar(t *testing.T) string {
	t.Helper()
	required := os.Getenv("EYE_CONFORMANCE") != ""
	skip := func(why string) string {
		if required {
			t.Fatalf("EYE_CONFORMANCE is set, but %s", why)
		}
		t.Logf("comparing with tree-sitter skipped: %s", why)
		return ""
	}
	if _, err := exec.LookPath("tree-sitter"); err != nil {
		return skip("tree-sitter is not installed (npm install -g tree-sitter-cli)")
	}
	if _, err := os.Stat("../tree-sitter-eyelang/src/parser.c"); err != nil {
		return skip("the parser isn't generated (run task generate)")
	}
	lib := filepath.Join(t.TempDir(), "eyelang.so")
	out, err := exec.Command("tree-sitter", "build", "-o", lib, "../tree-sitter-eyelang").CombinedOutput()
	if err != nil {
		return skip(fmt.Sprintf("tree-sitter build failed: %v\n%s", err, out))
	}
	return lib
}

// treeSitterParse parses a file with tree-sitter and returns its tree in the
// form SExpr prints with Fields and Tokens, and whether it parsed without
// errors.
func treeSitterParse(lib, path string) (string, bool, error) {
	cmd := exec.Command("tree-sitter", "parse", "--lib-path", lib, "--lang-name", "eyelang", "--xml", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	ok := err == nil
	if _, isExit := err.(*exec.ExitError); err != nil && !isExit {
		return "", false, err
	}
	out := stdout.Bytes()
	start := bytes.Index(out, []byte("<source_file"))
	end := bytes.LastIndex(out, []byte("</source_file>"))
	if start < 0 || end < 0 {
		return "", false, fmt.Errorf("no tree in the output:\n%s%s", out, stderr.Bytes())
	}
	sexp, err := xmlToSExpr(out[start : end+len("</source_file>")])
	return sexp, ok, err
}

// namedLeaves are the named nodes whose text is their content.
var namedLeaves = map[string]bool{
	"identifier": true, "type_identifier": true, "integer": true, "float": true, "date": true,
	"time": true, "char": true, "string_content": true, "escape_sequence": true, "unit_name": true,
	"comment": true,
}

type xmlNode struct {
	name, field string
	parts       []any // *xmlNode or string (anonymous tokens, or a leaf's text)
	text        strings.Builder
}

// xmlToSExpr converts tree-sitter's XML output to the normalised S-expression.
// Anonymous tokens are the text between elements; comments are dropped.
func xmlToSExpr(data []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var stack []*xmlNode
	var root *xmlNode
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			n := &xmlNode{name: tok.Name.Local}
			for _, a := range tok.Attr {
				if a.Name.Local == "field" {
					n.field = a.Value
				}
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.flushTokens()
				parent.parts = append(parent.parts, n)
			} else {
				root = n
			}
			stack = append(stack, n)
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text.Write(tok)
			}
		case xml.EndElement:
			n := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if namedLeaves[n.name] {
				n.parts = []any{n.text.String()}
			} else {
				n.flushTokens()
			}
		}
	}
	if root == nil {
		return "", fmt.Errorf("empty tree")
	}
	var b strings.Builder
	root.write(&b)
	return b.String(), nil
}

func (n *xmlNode) flushTokens() {
	for _, tok := range strings.Fields(n.text.String()) {
		n.parts = append(n.parts, tok)
	}
	n.text.Reset()
}

func (n *xmlNode) write(b *strings.Builder) {
	if n.field != "" {
		b.WriteString(n.field + ": ")
	}
	b.WriteString("(" + n.name)
	if namedLeaves[n.name] {
		b.WriteString(" " + strconv.Quote(n.parts[0].(string)) + ")")
		return
	}
	for _, p := range n.parts {
		switch p := p.(type) {
		case string:
			b.WriteString(" " + strconv.Quote(p))
		case *xmlNode:
			if p.name == "comment" {
				continue
			}
			b.WriteString(" ")
			p.write(b)
		}
	}
	b.WriteString(")")
}

// firstDifference shows where two S-expressions start to differ.
func firstDifference(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	from := max(0, i-60)
	return fmt.Sprintf("first difference at byte %d:\n  eye: …%s\n   ts: …%s", i, a[from:min(len(a), i+40)], b[from:min(len(b), i+40)])
}
