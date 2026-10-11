package syntax

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// eyeFiles lists every .eye file in the repository that should parse.
func eyeFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, root := range []string{"testdata/syntax/valid", "testdata/programs", "testdata/lab", "testdata/fmt", "docs", "check", "ir"} {
		err := filepath.WalkDir(filepath.Join("..", root), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && (d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			if strings.HasSuffix(path, ".eye") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return files
}

// TestConverterMatches checks that the tree from tree-sitter, converted, is
// the hand-written parser's tree: every node's kind, field, span, flags and
// leading trivia, on every file that parses.
func TestConverterMatches(t *testing.T) {
	files := eyeFiles(t)
	checked := 0
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want := Parse(src)
		if len(want.Diagnostics) > 0 {
			continue // testdata/fmt has inputs with errors
		}
		got, tree := parseTS(src)
		tree.Close()
		checked++
		if diff := sameTree(want.Root, got.Root, src, "source_file"); diff != "" {
			t.Errorf("%s: %s", path, diff)
		}
	}
	if checked < 150 {
		t.Errorf("only %d files compared", checked)
	}
}

// sameTree returns the first difference between two trees, or "".
func sameTree(a, b *Node, src []byte, path string) string {
	describe := func(n *Node) string {
		return fmt.Sprintf("%s field=%s flags=%d span=%v leading=%v children=%d %q",
			n.Kind, n.Field, n.Flags, n.Span, n.Leading, len(n.Children), clip(n.Text(src)))
	}
	if a.Kind != b.Kind || a.Field != b.Field || a.Flags != b.Flags || a.Span != b.Span ||
		fmt.Sprint(a.Leading) != fmt.Sprint(b.Leading) || len(a.Children) != len(b.Children) {
		return fmt.Sprintf("at %s\n  hand: %s\n  ts:   %s%s", path, describe(a), describe(b), childList(a, b, src))
	}
	for i := range a.Children {
		if d := sameTree(a.Children[i], b.Children[i], src, path+" > "+a.Children[i].Kind.String()); d != "" {
			return d
		}
	}
	return ""
}

func childList(a, b *Node, src []byte) string {
	list := func(n *Node) string {
		var s []string
		for _, c := range n.Children {
			s = append(s, fmt.Sprintf("%s%q", c.Kind, clip(c.Text(src))))
		}
		return strings.Join(s, " ")
	}
	return "\n  hand children: " + list(a) + "\n  ts children:   " + list(b)
}

func clip(s string) string {
	if len(s) > 40 {
		return s[:40] + "…"
	}
	return s
}
