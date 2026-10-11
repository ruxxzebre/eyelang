package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"eyelang/ir"
)

// TestCheckedBefore checks that the receiver before a `.` the user is typing
// has its type, though the text doesn't parse yet.
func TestCheckedBefore(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "eye.toml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "main.eye")
	saved := "type Box = { side: Int }\nfn f(b: Box) -> Int {\n  b.side\n}\n"
	if err := os.WriteFile(path, []byte(saved), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &server{docs: map[string]*document{}}
	for _, typing := range []string{
		"type Box = { side: Int }\nfn f(b: Box) -> Int {\n  b.\n}\n",
		"type Box = { side: Int }\nfn f(b: Box) -> Int {\n  b.si\n}\n",
	} {
		d := newDocument(pathURI(path), 2, []byte(typing))
		s.docs[d.uri] = d
		dot := len("type Box = { side: Int }\nfn f(b: Box) -> Int {\n  b")
		res, path, ok := s.checkedBefore(d, dot)
		if !ok {
			t.Errorf("%q: not checked", typing)
			continue
		}
		decl, ok := res.Index.TypeAt(path, dot)
		if box, isType := decl.(*ir.Type); !ok || !isType || box.Name != "Box" {
			t.Errorf("%q: the receiver's type is %v, %v; want Box", typing, decl, ok)
		}
	}
	// Unbalanced parentheses don't parse even without the `.`.
	d := newDocument(pathURI(path), 3, []byte("fn f(b: Box) -> Int {\n  g(b.\n}\n"))
	if _, _, ok := s.checkedBefore(d, len("fn f(b: Box) -> Int {\n  g(b")); ok {
		t.Errorf("checked a text that doesn't parse")
	}
}
