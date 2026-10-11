package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"eyelang/ir"
)

// TestTypeBefore checks that the receiver before a `.` the user is typing
// has its type, though the text doesn't parse yet.
func TestTypeBefore(t *testing.T) {
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
		decl, ok := s.typeBefore(d, dot)
		if box, isType := decl.(*ir.Type); !ok || !isType || box.Name != "Box" {
			t.Errorf("%q: the receiver's type is %v, %v; want Box", typing, decl, ok)
		}
	}
}
