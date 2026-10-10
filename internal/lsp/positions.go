package lsp

import (
	"unicode/utf8"

	"eyelang/syntax"
)

// Position is an LSP position: a 0-based line and a column counted in UTF-16
// code units, as VS Code and the protocol's default expect.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is an LSP range.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// document is an open file and what the server derived from it.
type document struct {
	uri     string
	version int
	text    []byte
	file    *syntax.File
}

func newDocument(uri string, version int, text []byte) *document {
	return &document{uri: uri, version: version, text: text, file: syntax.Parse(text)}
}

// position converts a byte offset to an LSP position.
func (d *document) position(offset int) Position {
	offset = max(0, min(offset, len(d.text)))
	p := d.file.Lines.Position(offset)
	start := d.file.Lines.LineStart(p.Line)
	return Position{Line: p.Line - 1, Character: utf16Len(d.text[start:offset])}
}

func (d *document) rangeOf(s syntax.Span) Range {
	return Range{Start: d.position(s.Start), End: d.position(s.End)}
}

// offset converts an LSP position to a byte offset, clamping it to the
// document.
func (d *document) offset(p Position) int {
	if p.Line < 0 {
		return 0
	}
	if p.Line >= d.file.Lines.Count() {
		return len(d.text)
	}
	i := d.file.Lines.LineStart(p.Line + 1)
	for units := 0; i < len(d.text) && d.text[i] != '\n' && units < p.Character; {
		r, size := utf8.DecodeRune(d.text[i:])
		units += utf16Units(r)
		i += size
	}
	return i
}

func utf16Len(b []byte) int {
	n := 0
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		n += utf16Units(r)
		b = b[size:]
	}
	return n
}

func utf16Units(r rune) int {
	if r >= 0x10000 {
		return 2
	}
	return 1
}
