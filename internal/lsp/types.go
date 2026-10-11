package lsp

import (
	"eyelang/check"
	"eyelang/syntax"
)

// checkedBefore checks the program for the members of the receiver before
// the `.` at offset dot: res.Index.TypeAt(path, dot) is the declaration of
// the receiver's type, and res.Lookup finds a module or type it names. While
// the user types `x.` or `x.na`, the file doesn't parse, and the checker
// skips a file with syntax errors; so it is checked with the `.` and the
// name after it left out, which keeps every offset before the `.` where it
// was. ok is false when the text still doesn't parse that way, so that no
// stale result is used.
func (s *server) checkedBefore(d *document, dot int) (res *check.Result, path string, ok bool) {
	path, ok = uriPath(d.uri)
	if !ok || dot <= 0 || dot >= len(d.text) || d.text[dot] != '.' {
		return nil, "", false
	}
	end := dot + 1
	for end < len(d.text) && isIdentByte(d.text[end]) {
		end++
	}
	text := append(append([]byte{}, d.text[:dot]...), d.text[end:]...)
	if len(syntax.Parse(text).Diagnostics) > 0 {
		return nil, "", false
	}
	return s.checkText(path, text), path, true
}
