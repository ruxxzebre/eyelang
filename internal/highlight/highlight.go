// Package highlight renders Eyelang source as HTML with its tokens wrapped in
// spans, using the parser rather than patterns, so contextual keywords and
// waiting calls are told apart the way the compiler sees them. The spec page
// uses it.
package highlight

import (
	"html"
	"strings"

	"eyelang/syntax"
)

// Classes of the spans HTML emits.
const (
	Keyword   = "k" // let, fn, type, entity, then, …
	Comment   = "c"
	String    = "s" // strings and characters
	Number    = "n" // numbers, dates, times, durations and sizes
	Type      = "t" // type, variant, entity and contract names
	Attribute = "a" // @name
	Waiting   = "w" // a waiting call's name and its `!`
)

// HTML returns src as HTML-escaped text with spans of the classes above. A
// list of statements, which doesn't parse at the top level, is highlighted
// as the body of a function.
func HTML(src string) string {
	f := syntax.Parse([]byte(src))
	prefix := 0
	if len(f.Diagnostics) > 0 {
		const wrap = "fn example() {\n"
		wrapped := syntax.Parse([]byte(wrap + src + "\n}\n"))
		if len(wrapped.Diagnostics) == 0 {
			f, prefix = wrapped, len(wrap)
		}
	}
	var out strings.Builder
	h := &highlighter{from: prefix, to: prefix + len(src), span: func(start, end int, class string) {
		text := html.EscapeString(string(f.Src[start:end]))
		if class == "" {
			out.WriteString(text)
			return
		}
		out.WriteString(`<span class="` + class + `">` + text + `</span>`)
	}}
	h.walk(f.Root, nil, nil)
	return out.String()
}

// Span is a part of a file of one of the classes above, in byte offsets.
type Span struct {
	Start, End int
	Class      string
}

// Spans returns the parts of a whole file that HTML would wrap in spans, in
// order: an editor's highlighting, from the same parser.
func Spans(src []byte) []Span {
	var out []Span
	h := &highlighter{from: 0, to: len(src), span: func(start, end int, class string) {
		if class != "" {
			out = append(out, Span{start, end, class})
		}
	}}
	h.walk(syntax.Parse(src).Root, nil, nil)
	return out
}

type highlighter struct {
	from, to int // the part of the source that came from the caller
	span     func(start, end int, class string)
}

// emit gives src[start:end], clipped to the caller's text, with its class.
func (h *highlighter) emit(start, end int, class string) {
	start, end = max(start, h.from), min(end, h.to)
	if start >= end {
		return
	}
	h.span(start, end, class)
}

func (h *highlighter) walk(n, parent, grand *syntax.Node) {
	if n.IsToken() {
		for _, t := range n.Leading {
			class := ""
			if t.Kind == syntax.Comment {
				class = Comment
			}
			h.emit(t.Span.Start, t.Span.End, class)
		}
		h.emit(n.Span.Start, n.Span.End, classify(n, parent, grand))
		return
	}
	for _, c := range n.Children {
		h.walk(c, n, parent)
	}
}

func classify(n, parent, grand *syntax.Node) string {
	switch n.Kind {
	case syntax.Keyword, syntax.KwLet, syntax.KwVar, syntax.KwMut, syntax.KwFn, syntax.KwPub, syntax.KwReturn,
		syntax.KwIf, syntax.KwElse, syntax.KwMatch, syntax.KwFor, syntax.KwIn, syntax.KwWhile, syntax.KwBreak,
		syntax.KwContinue, syntax.KwTrue, syntax.KwFalse:
		return Keyword
	case syntax.StringStart, syntax.StringText, syntax.StringEscape, syntax.StringEnd, syntax.Char,
		syntax.InterpOpen, syntax.InterpClose:
		return String
	case syntax.Int, syntax.Float, syntax.Date, syntax.Time, syntax.UnitName:
		return Number
	case syntax.TypeIdent:
		return Type
	case syntax.At:
		return Attribute
	case syntax.Bang:
		if parent != nil && parent.Kind == syntax.WaitingCallExpression {
			return Waiting
		}
	case syntax.Ident:
		switch {
		case parent == nil:
		case parent.Kind == syntax.Attribute && n.Field == syntax.FieldName:
			return Attribute
		case parent.Kind == syntax.WaitingCallExpression && n.Field == syntax.FieldFunction:
			return Waiting
		case parent.Kind == syntax.FieldExpression && n.Field == syntax.FieldField &&
			grand != nil && grand.Kind == syntax.WaitingCallExpression:
			return Waiting
		}
	}
	return ""
}
