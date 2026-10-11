package lsp

import (
	"bytes"
	"slices"

	"eyelang/syntax"
)

// completionItem is an LSP completion item.
type completionItem struct {
	Label         string        `json:"label"`
	Kind          int           `json:"kind"`
	Detail        string        `json:"detail,omitempty"`
	Documentation *markupString `json:"documentation,omitempty"`
}

type markupString struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// LSP completion item kinds.
const (
	completionText    = 1
	completionKeyword = 14
)

// The keywords that can start a line in each kind of body, and the ones that
// can follow `pub` there.
var (
	itemStarts = map[scope][]string{
		scopeFile:     {"import", "pub", "fn", "extern", "type", "entity", "job", "view", "trait", "impl", "contract", "test", "let"},
		scopeEntity:   {"state", "pub", "fn", "migrate"},
		scopeTrait:    {"fn"},
		scopeImpl:     {"fn"},
		scopeContract: {"command", "query", "event"},
		scopeBlock:    {"let", "var", "if", "match", "for", "while", "with", "return", "break", "continue", "fn", "true", "false"},
	}
	afterPub = map[scope][]string{
		scopeFile:   {"fn", "extern", "type", "opaque", "entity", "job", "view", "trait", "contract", "let"},
		scopeEntity: {"fn"},
	}
	// Keywords that start an expression, offered after the start of a line
	// in a block.
	expressionStarts = []string{"if", "match", "fn", "true", "false"}
)

// scope is the kind of body the cursor is in.
type scope int

const (
	scopeFile scope = iota
	scopeEntity
	scopeTrait
	scopeImpl
	scopeContract
	scopeBlock // a function, job or test body, or any braces inside one
	scopeOther // a record type's fields, or braces we can't place
)

// completion offers the keywords that fit at p. It reads tokens rather than
// the tree, because the line being typed is rarely valid yet. Where a name
// fits as well, it also offers the names written in the file: VS Code shows
// its own list of the file's words only when the server offers nothing.
func (s *server) completion(d *document, p Position) (any, *responseError) {
	offset := d.offset(p)
	words, names := keywordsAt(d.text, offset)
	items := []completionItem{}
	for _, w := range words {
		item := completionItem{Label: w, Kind: completionKeyword}
		if summary, _, ok := s.keywordDoc(w); ok {
			item.Documentation = &markupString{Kind: "markdown", Value: summary}
		}
		items = append(items, item)
	}
	if names {
		for _, n := range namesIn(d.text, offset) {
			items = append(items, completionItem{Label: n, Kind: completionText})
		}
	}
	return items, nil
}

// frame is one open brace, paren or bracket before the cursor.
type frame struct {
	scope scope
	paren bool        // ( or [ rather than {
	head  []string    // the words of the line being read in this body, attributes left out
	after syntax.Kind // the last significant token in this body
	fresh bool        // the next token starts a line of this body
}

// keywordsAt returns the keywords that may be typed at offset in src, and
// whether a name may be typed there too: in a body, where an expression
// can start.
func keywordsAt(src []byte, offset int) (words []string, names bool) {
	tokens, _ := syntax.Lex(src)
	stack := []*frame{{scope: scopeFile, fresh: true}}
	// An attribute is skipped: attr is 1 after its `@`, 2 after its name,
	// and 3 inside its arguments, which attrDepth counts the parens of.
	attr, attrDepth := 0, 0
	end, typed := 0, offset // the end of the last token read, and where the word being typed starts
	for _, t := range tokens {
		if t.Kind == syntax.EOF || t.Span.Start >= offset {
			break
		}
		if t.Span.End >= offset && (t.Kind == syntax.Comment || isStringPart(t.Kind)) {
			if t.Kind != syntax.StringEnd || t.Span.End > offset {
				return nil, false // inside a comment or a string
			}
		}
		if t.Span.End == offset && isWord(t.Kind) || t.Span.End > offset && !t.Kind.IsTrivia() {
			typed = t.Span.Start // the word being typed, or a token the cursor is inside
			break
		}
		if t.Kind.IsTrivia() {
			continue
		}
		end = t.Span.End
		top := stack[len(stack)-1]
		if !top.paren && t.Flags&syntax.NewlineBefore != 0 && !continues(top.after) {
			top.fresh = true
		}
		if top.fresh {
			top.head, top.fresh = nil, false
		}
		switch {
		case attr == 1:
			attr = 2
			continue
		case attr == 2 && t.Kind == syntax.LParen:
			attr, attrDepth = 3, 1
			continue
		case attr == 3:
			switch t.Kind {
			case syntax.LParen:
				attrDepth++
			case syntax.RParen:
				attrDepth--
			}
			if attrDepth == 0 {
				attr = 0
			}
			continue
		}
		attr = 0
		switch t.Kind {
		case syntax.At:
			attr = 1
		case syntax.LBrace:
			stack = append(stack, &frame{scope: bodyScope(top), fresh: true})
		case syntax.InterpOpen:
			stack = append(stack, &frame{scope: scopeBlock, paren: true})
		case syntax.LParen, syntax.LBracket:
			stack = append(stack, &frame{scope: top.scope, paren: true})
		case syntax.RBrace, syntax.InterpClose, syntax.RParen, syntax.RBracket:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
			top = stack[len(stack)-1]
		case syntax.Semicolon:
			top.fresh = true
		default:
			if isWord(t.Kind) {
				top.head = append(top.head, string(src[t.Span.Start:t.Span.End]))
			} else {
				top.head = append(top.head, "")
			}
		}
		if t.Kind == syntax.RBrace && !top.paren {
			top.head = append(top.head, "}")
		}
		top.after = t.Kind
	}
	if attr == 1 || attr == 3 {
		return nil, false // an attribute's name or arguments
	}
	top := stack[len(stack)-1]
	if !top.paren && bytes.IndexByte(src[end:typed], '\n') >= 0 && !continues(top.after) {
		top.fresh = true
	}
	if top.paren {
		if top.scope == scopeBlock {
			return expressionStarts, true
		}
		return nil, false
	}
	head := top.head
	if top.fresh {
		head = nil
	}
	switch {
	case len(head) == 0:
		return itemStarts[top.scope], top.scope == scopeBlock
	case top.scope == scopeBlock:
		if len(head) == 1 && slices.Contains([]string{"let", "var", "for", "fn"}, head[0]) {
			return nil, false // a new name comes next
		}
		if head[len(head)-1] == "}" {
			return append([]string{"else"}, expressionStarts...), true
		}
		return expressionStarts, true
	case slices.Equal(head, []string{"pub"}):
		return afterPub[top.scope], false
	case top.scope != scopeFile:
		if top.scope == scopeEntity && slices.Equal(head, []string{"migrate"}) {
			return []string{"from"}, false
		}
	case slices.Equal(head, []string{"pub", "opaque"}):
		return []string{"type"}, false
	case slices.Equal(head, []string{"extern"}) || slices.Equal(head, []string{"pub", "extern"}):
		return []string{"fn", "type"}, false
	}
	return nil, false
}

// namesIn returns the names written in src, each once, leaving out the one
// that ends at offset, which is being typed.
func namesIn(src []byte, offset int) []string {
	tokens, _ := syntax.Lex(src)
	seen := map[string]bool{}
	var out []string
	for _, t := range tokens {
		if t.Kind != syntax.Ident && t.Kind != syntax.TypeIdent || t.Span.End == offset {
			continue
		}
		if name := string(src[t.Span.Start:t.Span.End]); !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// bodyScope is the scope of a `{` read in top: a body named by the line's
// first keyword at the declaration level, a block below it.
func bodyScope(top *frame) scope {
	switch top.scope {
	case scopeBlock:
		return scopeBlock
	case scopeOther, scopeContract:
		return scopeOther
	}
	for _, w := range top.head {
		switch w {
		case "entity":
			return scopeEntity
		case "trait":
			return scopeTrait
		case "impl":
			return scopeImpl
		case "contract":
			return scopeContract
		case "type", "import":
			return scopeOther
		case "fn", "job", "test", "migrate", "view", "let":
			return scopeBlock
		}
	}
	return scopeBlock
}

// continues reports whether a line ending in a token of kind k goes on to
// the next line, as `type T =` or `a ->` do.
func continues(k syntax.Kind) bool {
	switch k {
	case syntax.Assign, syntax.Arrow, syntax.FatArrow, syntax.Comma, syntax.Dot, syntax.Colon,
		syntax.PlusEq, syntax.MinusEq, syntax.StarEq, syntax.SlashEq, syntax.PercentEq:
		return true
	}
	return false
}

func isWord(k syntax.Kind) bool {
	return k == syntax.Ident || k == syntax.TypeIdent || k.IsKeyword()
}

func isStringPart(k syntax.Kind) bool {
	switch k {
	case syntax.StringStart, syntax.StringText, syntax.StringEscape, syntax.StringEnd:
		return true
	}
	return false
}
