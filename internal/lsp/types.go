package lsp

import (
	"path/filepath"

	"eyelang/check"
)

// typeBefore returns the declaration of the type of the receiver before the
// `.` at offset dot, whose members completion offers: an *ir.Type, an
// *ir.Entity or a check.BuiltinType. While the user types `x.` or `x.na`,
// the file doesn't parse, and the checker skips a file with syntax errors;
// so it is checked with the `.` and the name after it left out, which keeps
// every offset before the `.` where it was.
func (s *server) typeBefore(d *document, dot int) (any, bool) {
	path, ok := uriPath(d.uri)
	if !ok || dot <= 0 || dot >= len(d.text) || d.text[dot] != '.' {
		return nil, false
	}
	end := dot + 1
	for end < len(d.text) && isIdentByte(d.text[end]) {
		end++
	}
	text := append(append([]byte{}, d.text[:dot]...), d.text[end:]...)
	root := check.Root(path)
	overlay := s.overlay(root)
	overlay[path] = text
	res := check.Check(root, check.Options{Overlay: overlay, Module: filepath.Dir(path)})
	return res.Index.TypeAt(path, dot)
}
