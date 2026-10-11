package syntax

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/syntax/invalid/diagnostics.txt from the parser's output")

// TestSyntaxErrors checks the parser's diagnostics for every invalid file in
// the conformance suite against testdata/syntax/invalid/diagnostics.txt: the
// message and where it points, so a change to the parser can't make an error
// vaguer without the change showing. Run with -update to rewrite the file.
func TestSyntaxErrors(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "testdata", "syntax", "invalid", "*.eye"))
	if len(files) == 0 {
		t.Fatal("no invalid files")
	}
	var got strings.Builder
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		f := Parse(src)
		if len(f.Diagnostics) == 0 {
			t.Errorf("%s: no diagnostics", path)
		}
		for _, d := range f.Diagnostics {
			p := f.Lines.Position(d.Span.Start)
			fmt.Fprintf(&got, "%s:%d:%d: %s\n", filepath.Base(path), p.Line, p.Column, d.Message)
		}
	}
	golden := filepath.Join("..", "testdata", "syntax", "invalid", "diagnostics.txt")
	if *update {
		if err := os.WriteFile(golden, []byte(got.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != string(want) {
		gotLines, wantLines := strings.Split(got.String(), "\n"), strings.Split(string(want), "\n")
		for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
			var g, w string
			if i < len(gotLines) {
				g = gotLines[i]
			}
			if i < len(wantLines) {
				w = wantLines[i]
			}
			if g != w {
				t.Errorf("diagnostic %d:\n  got:  %s\n  want: %s", i+1, g, w)
			}
		}
	}
}
