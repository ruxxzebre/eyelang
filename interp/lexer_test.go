package interp

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"eyelang/check"
	"eyelang/ir"
	"eyelang/syntax"
)

// TestLexerInEyelang runs the lexer that testdata/programs/lexer writes in
// Eyelang on every file of the conformance suite (testdata/syntax) and on the
// source of every test of the tree-sitter corpus, and checks that it finds the
// tokens syntax.Lex finds: the same kinds, spans and flags. The corpus files
// are cut into their tests' sources, as the conformance suite reads them,
// since the headers and expected trees around them aren't Eyelang.
func TestLexerInEyelang(t *testing.T) {
	prog, diags := check.Program("../testdata/programs/lexer")
	for _, d := range diags {
		t.Fatal(d)
	}
	names := kindNames(t)

	// The program's kinds are syntax's, in syntax's order, which the
	// benchmark's checksum (testdata/bench/lexer) counts on.
	kinds := runLexer(t, prog, "--kinds")
	if want := strings.Join(names[syntax.EOF:], " ") + "\n"; kinds != want {
		t.Fatalf("the Eyelang lexer's kinds are\n%s\nwant syntax's, in its order:\n%s", kinds, want)
	}

	files, err := filepath.Glob("../testdata/syntax/*/*.eye")
	if err != nil || len(files) < 100 {
		t.Fatalf("found %d files in testdata/syntax: %v", len(files), err)
	}
	dir := t.TempDir()
	sources := corpusSources(t)
	if len(sources) < 100 {
		t.Fatalf("found %d corpus tests", len(sources))
	}
	for name, src := range edgeCases {
		sources[name] = src
	}
	for name, src := range sources {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, path)
	}

	got := map[string]string{}
	for _, block := range strings.Split(runLexer(t, prog, files...), "file ")[1:] {
		path, toks, _ := strings.Cut(block, "\n")
		got[path] = toks
	}
	compared := 0
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var want strings.Builder
		toks, _ := syntax.Lex(src)
		compared += len(toks)
		for _, tok := range toks {
			fmt.Fprintf(&want, "%s %d %d %d\n", names[tok.Kind], tok.Span.Start, tok.Span.End, tok.Flags)
		}
		if g, ok := got[path]; !ok {
			t.Errorf("%s: the Eyelang lexer printed nothing for it", path)
		} else if g != want.String() {
			t.Errorf("%s: %s", path, firstDifference(g, want.String()))
		}
	}
	t.Logf("compared %d tokens in %d files", compared, len(files))
}

// edgeCases are inputs that the suite and the corpus have few of: errors at
// the end of the source, and the edges of the longest-match rules.
var edgeCases = map[string]string{
	"edge-strings.eye":   "\"unterminated\n\"{a\n\"\\\n$ é \"{\"{",
	"edge-multiline.eye": "let s = \"\"\"\n  a \"\" {x} \\u{41} \\\"\n  \"\"\"\nlet t = \"\"\"\n  never closed {y",
	"edge-numbers.eye":   "1.5e+3 1.e 1e5 1.5e+ 1_000 1__0 1_ 07:40:30 07:4 2026-10-1 12:34:5 0.5E-2x 99:99",
	"edge-chars.eye":     "'\\u{1F600}' '' 'ab' '\\q' '\\'' '\\u{}' ' \n'x '\\",
	"edge-escapes.eye":   "\"\\u{} \\u{1234567} \\u{10FFFF} \\u{12 \\é \\\" \\{ \\}\" \"abc\\",
	"edge-braces.eye":    "\"{\"{\"{a}\"}\"}\" \"{ {x: {y: 1}} }\" } { \"{}}\"",
	"edge-space.eye":     "a\r\nb\f\vc\t# c\r\n  ## d\n\n?? ? |> | || & &&",
	"edge-end.eye":       "\"\"\"\n  x {\"a{",
}

// runLexer runs the lexer program's main with args and returns what it
// printed.
func runLexer(t *testing.T, prog *ir.Program, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), prog, RunOptions{
		Args: append([]string{"lexer"}, args...), Stdout: &stdout, Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("the lexer program exited with %d: %s", code, stderr.String())
	}
	return stdout.String()
}

// firstDifference describes the first line where the Eyelang lexer's tokens,
// got, differ from syntax's, want.
func firstDifference(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < len(g) && i < len(w); i++ {
		if g[i] != w[i] {
			return fmt.Sprintf("token %d is %q in Eyelang, %q in syntax", i, g[i], w[i])
		}
	}
	return fmt.Sprintf("Eyelang finds %d tokens, syntax %d", len(g)-1, len(w)-1)
}

// kindNames reads syntax's token kinds from syntax/token.go: each kind's
// constant name, as the Eyelang lexer names its variants. Its literals'
// names differ, since a variant named Int would hide the type.
func kindNames(t *testing.T) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "../syntax/token.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	renamed := map[string]string{
		"Int": "IntLiteral", "Float": "FloatLiteral", "Date": "DateLiteral",
		"Time": "TimeLiteral", "Char": "CharLiteral",
	}
	var names []string
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST || len(names) > 0 {
			continue
		}
		for _, s := range g.Specs {
			for _, n := range s.(*ast.ValueSpec).Names {
				name := n.Name
				if name == "lastToken" {
					break
				}
				if r, ok := renamed[name]; ok {
					name = r
				}
				names = append(names, name)
			}
		}
	}
	if len(names) != int(syntax.KwFalse)+1 || names[syntax.Invalid] != "Invalid" {
		t.Fatalf("read %d kinds from syntax/token.go, starting %v", len(names), names[:1])
	}
	return names
}

// corpusSources returns the source of each test in the tree-sitter corpus,
// what lies between its === header and its --- line, by a file name that
// says where it is from: literals-3.eye for the third test of literals.txt.
func corpusSources(t *testing.T) map[string]string {
	t.Helper()
	files, err := filepath.Glob("../tree-sitter-eyelang/test/corpus/*.txt")
	if err != nil || len(files) == 0 {
		t.Fatalf("no corpus files: %v", err)
	}
	header := regexp.MustCompile(`(?m)^={3,}\n.*\n(?::\w+\n)*={3,}\n`)
	divider := regexp.MustCompile(`(?m)^-{3,}$`)
	out := map[string]string{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		heads := header.FindAllStringIndex(text, -1)
		for i, h := range heads {
			end := len(text)
			if i+1 < len(heads) {
				end = heads[i+1][0]
			}
			body := text[h[1]:end]
			if sep := divider.FindStringIndex(body); sep != nil {
				body = body[:sep[0]]
			}
			name := fmt.Sprintf("%s-%d.eye", strings.TrimSuffix(filepath.Base(file), ".txt"), i+1)
			out[name] = strings.Trim(body, "\n") + "\n"
		}
	}
	return out
}
