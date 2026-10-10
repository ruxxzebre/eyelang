package syntax

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// separators matches the tokens formatting may add or remove.
var separators = regexp.MustCompile(` ";"| ","`)

// formatCases returns every valid input of the conformance suite and the
// tree-sitter corpus.
func formatCases(t *testing.T) map[string][]byte {
	t.Helper()
	cases := map[string][]byte{}
	files, _ := filepath.Glob("../testdata/syntax/valid/*.eye")
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		cases["valid/"+filepath.Base(f)] = src
	}
	for _, c := range corpusCases(t) {
		if !c.wantError {
			cases["corpus/"+c.file+"/"+c.name] = []byte(c.input)
		}
	}
	return cases
}

// TestFormatKeepsMeaning checks, on every valid file, that the formatted
// source parses to the same tree (apart from separators and trailing commas)
// and that formatting it again changes nothing.
func TestFormatKeepsMeaning(t *testing.T) {
	for name, src := range formatCases(t) {
		t.Run(name, func(t *testing.T) {
			out, err := Format(src)
			if err != nil {
				t.Fatal(err)
			}
			f := Parse(out)
			if len(f.Diagnostics) > 0 {
				t.Fatalf("the formatted file doesn't parse: %v\n%s", f.Diagnostics, out)
			}
			opts := SExprOptions{Fields: true, Tokens: true, Text: true}
			before := separators.ReplaceAllString(Parse(src).Root.SExpr(src, opts), "")
			after := separators.ReplaceAllString(f.Root.SExpr(out, opts), "")
			if before != after {
				t.Errorf("formatting changed the tree\n%s\nbefore: %s\n after: %s\n%s", out, before, after, firstDifference(before, after))
			}
			again, err := Format(out)
			if err != nil {
				t.Fatal(err)
			}
			if string(again) != string(out) {
				t.Errorf("formatting twice changes the output\nonce:\n%s\ntwice:\n%s", out, again)
			}
			if strings.Contains(string(out), "\r") || strings.Contains(string(out), " \n") || strings.Contains(string(out), "\t") {
				t.Errorf("the output has carriage returns, tabs or trailing spaces:\n%q", out)
			}
		})
	}
}

// TestFormatGolden compares Format with the expected output in testdata/fmt:
// each name.input.eye formats to name.golden.eye, and every golden file is
// already formatted.
func TestFormatGolden(t *testing.T) {
	inputs, _ := filepath.Glob("../testdata/fmt/*.input.eye")
	if len(inputs) == 0 {
		t.Fatal("no testdata/fmt/*.input.eye files")
	}
	for _, in := range inputs {
		golden := strings.TrimSuffix(in, ".input.eye") + ".golden.eye"
		t.Run(filepath.Base(golden), func(t *testing.T) {
			src, err := os.ReadFile(in)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Format(src)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Errorf("Format(%s) differs from %s\n got:\n%s\nwant:\n%s", in, golden, got, want)
			}
			if again, _ := Format(want); string(again) != string(want) {
				t.Errorf("%s isn't formatted:\n%s", golden, again)
			}
		})
	}
}

func TestFormatRefusesErrors(t *testing.T) {
	if _, err := Format([]byte("fn f( {")); err == nil {
		t.Error("Format accepted a file with syntax errors")
	}
}

func FuzzFormat(f *testing.F) {
	for _, src := range exampleInputs(f) {
		f.Add(src)
	}
	opts := SExprOptions{Fields: true, Tokens: true, Text: true}
	f.Fuzz(func(t *testing.T, src string) {
		if len(Parse([]byte(src)).Diagnostics) > 0 {
			return
		}
		out, err := Format([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		f2 := Parse(out)
		if len(f2.Diagnostics) > 0 {
			t.Fatalf("formatted output doesn't parse: %v\ninput:\n%s\noutput:\n%s", f2.Diagnostics, src, out)
		}
		before := separators.ReplaceAllString(Parse([]byte(src)).Root.SExpr([]byte(src), opts), "")
		after := separators.ReplaceAllString(f2.Root.SExpr(out, opts), "")
		if before != after {
			t.Fatalf("formatting changed the tree\ninput:\n%q\noutput:\n%q", src, out)
		}
		if again, _ := Format(out); string(again) != string(out) {
			t.Fatalf("formatting isn't stable\ninput:\n%q\nonce:\n%q\ntwice:\n%q", src, out, again)
		}
	})
}
