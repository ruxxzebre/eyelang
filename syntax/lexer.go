package syntax

import (
	"fmt"
	"unicode/utf8"
)

// Lex splits src into tokens, trivia included, ending with an EOF token.
// Concatenating the tokens' text reproduces src exactly. Lexing never stops at
// an error: illegal characters and bad escapes become Illegal tokens, an
// unterminated string gets a zero-width StringEnd marked Missing, and each
// problem is reported as a diagnostic.
//
// The lexer follows the longest-match rule of the tree-sitter grammar's lexer,
// so `2026-10-10` is a Date and `2026 - 10` is subtraction, `07:40` is a Time,
// and `!=` is one token. Which newlines end a statement is decided by the
// parser from the NewlineBefore flag.
func Lex(src []byte) ([]Token, []Diagnostic) {
	l := lexer{src: src}
	l.run()
	return l.tokens, l.diags
}

type lexer struct {
	src    []byte
	pos    int
	tokens []Token
	diags  []Diagnostic

	// modes holds one entry per string being lexed, innermost last. Inside an
	// interpolation, depth counts the `{` that are still open, so the `}` that
	// ends the interpolation can be told from the end of a record.
	modes []stringMode

	// Trivia seen since the last significant token.
	space, newline bool
}

type stringMode struct {
	start  int  // offset of the opening quote
	interp bool // inside `{…}` rather than in the string's text
	open   int  // offset of the interpolation's `{`
	depth  int
}

func (l *lexer) run() {
	for {
		if n := len(l.modes); n > 0 && !l.modes[n-1].interp {
			l.stringPart()
			continue
		}
		if l.pos >= len(l.src) {
			l.endOfInput()
			return
		}
		l.next()
	}
}

func (l *lexer) emit(kind Kind, start int) {
	tok := Token{Kind: kind, Span: Span{start, l.pos}}
	if kind.IsTrivia() {
		l.tokens = append(l.tokens, tok)
		return
	}
	if l.space {
		tok.Flags |= SpaceBefore
	}
	if l.newline {
		tok.Flags |= NewlineBefore
	}
	l.space, l.newline = false, false
	l.tokens = append(l.tokens, tok)
}

func (l *lexer) errorf(span Span, format string, args ...any) {
	l.diags = append(l.diags, Diagnostic{Span: span, Message: fmt.Sprintf(format, args...)})
}

func (l *lexer) peek(offset int) byte {
	if i := l.pos + offset; i < len(l.src) {
		return l.src[i]
	}
	return 0
}

// endOfInput closes any strings left open inside interpolations with
// zero-width tokens, so the parser sees complete strings, and emits EOF.
func (l *lexer) endOfInput() {
	for i := len(l.modes) - 1; i >= 0; i-- {
		m := l.modes[i]
		l.errorf(Span{m.open, m.open + 1}, "unterminated interpolation: missing }")
		l.emitMissing(InterpClose)
		l.emitMissing(StringEnd)
	}
	l.modes = nil
	l.emit(EOF, l.pos)
}

func (l *lexer) emitMissing(kind Kind) {
	l.emit(kind, l.pos)
	l.tokens[len(l.tokens)-1].Flags |= Missing
}

// next lexes one token outside string text.
func (l *lexer) next() {
	start := l.pos
	c := l.src[l.pos]
	switch {
	case isSpace(c):
		for l.pos < len(l.src) && isSpace(l.src[l.pos]) {
			if l.src[l.pos] == '\n' {
				l.newline = true
			}
			l.pos++
		}
		l.space = true
		l.emit(Whitespace, start)
		return
	case c == '#':
		for l.pos < len(l.src) && l.src[l.pos] != '\n' {
			l.pos++
		}
		l.space = true
		l.emit(Comment, start)
		return
	case isLower(c) || c == '_':
		l.pos = scanName(l.src, l.pos)
		if kw, ok := keywords[string(l.src[start:l.pos])]; ok {
			l.emit(kw, start)
		} else {
			l.emit(Ident, start)
		}
		return
	case isUpper(c):
		l.pos = scanName(l.src, l.pos)
		l.emit(TypeIdent, start)
		return
	case isDigit(c):
		l.number()
		return
	case c == '"':
		l.pos++
		l.emit(StringStart, start)
		l.modes = append(l.modes, stringMode{start: start})
		return
	}

	kind := l.punctuation(c)
	if kind == Illegal {
		r, size := utf8.DecodeRune(l.src[l.pos:])
		l.pos += size
		if r == utf8.RuneError && size == 1 {
			l.errorf(Span{start, l.pos}, "invalid UTF-8 byte 0x%02x", c)
		} else {
			l.errorf(Span{start, l.pos}, "unexpected character %q", r)
		}
	}
	l.emit(kind, start)
}

// punctuation consumes an operator or delimiter starting with c and returns
// its kind, or returns Illegal without consuming anything.
func (l *lexer) punctuation(c byte) Kind {
	two := func(second byte, long, short Kind) Kind {
		if l.peek(1) == second {
			l.pos += 2
			return long
		}
		l.pos++
		return short
	}
	switch c {
	case '(':
		l.pos++
		return LParen
	case ')':
		l.pos++
		return RParen
	case '[':
		l.pos++
		return LBracket
	case ']':
		l.pos++
		return RBracket
	case '{':
		l.pos++
		if n := len(l.modes); n > 0 {
			l.modes[n-1].depth++
		}
		return LBrace
	case '}':
		l.pos++
		if n := len(l.modes); n > 0 {
			m := &l.modes[n-1]
			if m.depth == 0 {
				m.interp = false
				return InterpClose
			}
			m.depth--
		}
		return RBrace
	case ',':
		l.pos++
		return Comma
	case ':':
		l.pos++
		return Colon
	case ';':
		l.pos++
		return Semicolon
	case '@':
		l.pos++
		return At
	case '.':
		switch l.peek(1) {
		case '.':
			l.pos += 2
			return DotDot
		case '{':
			l.pos += 2
			return DotBrace
		}
		l.pos++
		return Dot
	case '-':
		return two('>', Arrow, Minus)
	case '=':
		switch l.peek(1) {
		case '=':
			l.pos += 2
			return Eq
		case '>':
			l.pos += 2
			return FatArrow
		}
		l.pos++
		return Assign
	case '!':
		return two('=', NotEq, Bang)
	case '<':
		return two('=', LtEq, Lt)
	case '>':
		return two('=', GtEq, Gt)
	case '+':
		l.pos++
		return Plus
	case '*':
		l.pos++
		return Star
	case '/':
		l.pos++
		return Slash
	case '%':
		l.pos++
		return Percent
	case '?':
		return two('?', QuestionQuestion, Question)
	case '|':
		switch l.peek(1) {
		case '>':
			l.pos += 2
			return PipeGt
		case '|':
			l.pos += 2
			return OrOr
		}
		l.pos++
		return Pipe
	case '&':
		if l.peek(1) == '&' {
			l.pos += 2
			return AndAnd
		}
	}
	return Illegal
}

// number lexes an Int, Float, Date or Time, taking the longest that matches:
//
//	Int    [0-9](_?[0-9])*
//	Float  Int "." [0-9](_?[0-9])* ([eE][+-]?[0-9]+)?
//	Date   [0-9]{4} "-" [0-9]{2} "-" [0-9]{2}
//	Time   [0-9]{2} ":" [0-9]{2} (":" [0-9]{2})?
func (l *lexer) number() {
	start := l.pos
	src := l.src

	if end, ok := matchDigits(src, start, 4); ok && byteAt(src, end) == '-' {
		if end, ok := matchDigits(src, end+1, 2); ok && byteAt(src, end) == '-' {
			if end, ok := matchDigits(src, end+1, 2); ok {
				l.pos = end
				l.emit(Date, start)
				return
			}
		}
	}
	if end, ok := matchDigits(src, start, 2); ok && byteAt(src, end) == ':' {
		if end, ok := matchDigits(src, end+1, 2); ok {
			if byteAt(src, end) == ':' {
				if seconds, ok := matchDigits(src, end+1, 2); ok {
					end = seconds
				}
			}
			l.pos = end
			l.emit(Time, start)
			return
		}
	}

	end := scanDigits(src, start)
	if byteAt(src, end) == '.' && isDigit(byteAt(src, end+1)) {
		end = scanDigits(src, end+1)
		if e := byteAt(src, end); e == 'e' || e == 'E' {
			exp := end + 1
			if s := byteAt(src, exp); s == '+' || s == '-' {
				exp++
			}
			if isDigit(byteAt(src, exp)) {
				for isDigit(byteAt(src, exp)) {
					exp++
				}
				end = exp
			}
		}
		l.pos = end
		l.emit(Float, start)
		return
	}
	l.pos = end
	l.emit(Int, start)
}

// stringPart lexes one token of a string's text: text, an escape, the start of
// an interpolation, or the closing quote.
func (l *lexer) stringPart() {
	start := l.pos
	top := &l.modes[len(l.modes)-1]
	if l.pos >= len(l.src) || l.src[l.pos] == '\n' {
		// Strings stay on one line. Close this one with a zero-width quote so the
		// parser sees a complete string.
		l.errorf(Span{top.start, l.pos}, `unterminated string: missing "`)
		l.modes = l.modes[:len(l.modes)-1]
		l.emitMissing(StringEnd)
		return
	}
	switch l.src[l.pos] {
	case '"':
		l.pos++
		l.modes = l.modes[:len(l.modes)-1]
		l.emit(StringEnd, start)
	case '{':
		l.pos++
		top.interp, top.open, top.depth = true, start, 0
		l.emit(InterpOpen, start)
	case '\\':
		if end, ok := matchEscape(l.src, l.pos); ok {
			l.pos = end
			l.emit(StringEscape, start)
			return
		}
		l.pos++
		if l.pos < len(l.src) && l.src[l.pos] != '\n' {
			_, size := utf8.DecodeRune(l.src[l.pos:])
			l.pos += size
		}
		l.errorf(Span{start, l.pos}, "invalid escape sequence %q", l.src[start:l.pos])
		l.emit(Illegal, start)
	default:
		for l.pos < len(l.src) {
			c := l.src[l.pos]
			if c == '"' || c == '\\' || c == '{' || c == '\n' {
				break
			}
			l.pos++
		}
		l.emit(StringText, start)
	}
}

// matchEscape matches \n \r \t \0 \\ \" \{ \} or \u{X} with 1 to 6 hex digits.
func matchEscape(src []byte, i int) (int, bool) {
	switch byteAt(src, i+1) {
	case 'n', 'r', 't', '0', '\\', '"', '{', '}':
		return i + 2, true
	case 'u':
		if byteAt(src, i+2) != '{' {
			return 0, false
		}
		j := i + 3
		for j < len(src) && j-(i+3) < 6 && isHex(src[j]) {
			j++
		}
		if j == i+3 || byteAt(src, j) != '}' {
			return 0, false
		}
		return j + 1, true
	}
	return 0, false
}

func scanName(src []byte, i int) int {
	for i < len(src) && (isLower(src[i]) || isUpper(src[i]) || isDigit(src[i]) || src[i] == '_') {
		i++
	}
	return i
}

// scanDigits scans [0-9](_?[0-9])* starting at a digit.
func scanDigits(src []byte, i int) int {
	i++
	for {
		if isDigit(byteAt(src, i)) {
			i++
		} else if byteAt(src, i) == '_' && isDigit(byteAt(src, i+1)) {
			i += 2
		} else {
			return i
		}
	}
}

// matchDigits matches exactly n digits at i.
func matchDigits(src []byte, i, n int) (int, bool) {
	for k := 0; k < n; k++ {
		if !isDigit(byteAt(src, i+k)) {
			return 0, false
		}
	}
	return i + n, true
}

func byteAt(src []byte, i int) byte {
	if i < len(src) {
		return src[i]
	}
	return 0
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}
func isLower(c byte) bool { return c >= 'a' && c <= 'z' }
func isUpper(c byte) bool { return c >= 'A' && c <= 'Z' }
func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isHex(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
