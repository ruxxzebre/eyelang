package syntax

import (
	"fmt"
	"sort"
)

// Parse parses a source file into a lossless syntax tree. It always returns a
// tree: syntax errors become ERROR nodes and missing tokens, and are listed in
// the file's diagnostics, so one parse reports every error.
//
// The grammar is tree-sitter-eyelang/grammar.js. Declarations and statements
// are parsed by recursive descent and expressions by precedence climbing, with
// the precedence table of SPECS.md section 23.
func Parse(src []byte) *File {
	tokens, lexDiags := Lex(src)
	p := &parser{src: src, lines: NewLines(src), errLines: map[int]bool{}, errTok: -1, fuel: fuel}
	var trivia []Token
	for _, t := range tokens {
		if t.Kind.IsTrivia() {
			trivia = append(trivia, t)
			continue
		}
		p.toks = append(p.toks, t)
		p.trivia = append(p.trivia, trivia)
		trivia = nil
	}
	for _, d := range lexDiags {
		p.errLines[p.lines.Position(d.Span.Start).Line] = true
	}

	root := p.sourceFile()
	diags := append(lexDiags, p.diags...)
	sort.SliceStable(diags, func(i, j int) bool { return diags[i].Span.Start < diags[j].Span.Start })
	return &File{Src: src, Root: root, Lines: p.lines, Diagnostics: diags}
}

type parser struct {
	src    []byte
	toks   []Token   // significant tokens, ending with EOF
	trivia [][]Token // the whitespace and comments before each token
	pos    int
	lines  *Lines
	diags  []Diagnostic

	// errLines holds the lines that already have a diagnostic. Like the Go
	// compiler, the parser reports at most one error per line, so one mistake
	// doesn't produce a cascade. errTok is the token where the last error was
	// reported; a second error there, before anything is consumed, is a
	// consequence of the first and isn't reported.
	errLines map[int]bool
	errTok   int

	// nl is true where a line break may end the current statement, member or
	// match arm, which is where tree-sitter's scanner may produce _newline.
	// Delimited constructs and parts that can't end an element turn it off.
	nl bool

	fuel int // looks left before the next token is consumed
}

// ---------------------------------------------------------------- tokens

// fuel is how many times the parser may look at tokens without consuming
// one. A loop that runs out has stopped making progress: rather than hang an
// editor, the parser panics, and the fuzz test reports the input.
const fuel = 1024

// look spends fuel on one look at the tokens.
func (p *parser) look() {
	p.fuel--
	if p.fuel < 0 {
		panic(fmt.Sprintf("syntax: the parser is stuck at offset %d, %s: it looked %d times without consuming a token",
			p.toks[p.pos].Span.Start, p.toks[p.pos].Kind, fuel))
	}
}

func (p *parser) tok() Token { p.look(); return p.toks[p.pos] }

func (p *parser) kind() Kind { p.look(); return p.toks[p.pos].Kind }

func (p *parser) at(k Kind) bool { p.look(); return p.toks[p.pos].Kind == k }

// peek returns the token n positions ahead, or EOF.
func (p *parser) peek(n int) Token {
	p.look()
	if i := p.pos + n; i < len(p.toks) {
		return p.toks[i]
	}
	return p.toks[len(p.toks)-1]
}

func (p *parser) text(t Token) string { return string(p.src[t.Span.Start:t.Span.End]) }

// atWord reports whether the current token is the name w, which is how
// contextual keywords are recognised.
func (p *parser) atWord(w string) bool {
	k := p.kind()
	return (k == Ident || k == TypeIdent) && p.text(p.tok()) == w
}

// bump consumes the current token and returns it as a leaf node.
func (p *parser) bump() *Node {
	t := p.toks[p.pos]
	n := &Node{Kind: t.Kind, Flags: t.Flags, Span: t.Span, Leading: p.trivia[p.pos]}
	if t.Kind != EOF {
		p.pos++
		p.fuel = fuel
	}
	return n
}

// bumpAs consumes the current token and gives it another kind: a contextual
// keyword, a unit name, or a keyword used as a label.
func (p *parser) bumpAs(k Kind) *Node {
	n := p.bump()
	n.Kind = k
	return n
}

// prevEnd returns the end of the last consumed token.
func (p *parser) prevEnd() int {
	if p.pos == 0 {
		return 0
	}
	return p.toks[p.pos-1].Span.End
}

// missing returns a zero-width token of kind k at the end of the last
// consumed token.
func (p *parser) missing(k Kind) *Node {
	end := p.prevEnd()
	return &Node{Kind: k, Flags: Missing, Span: Span{end, end}}
}

// expect consumes a token of kind k, or reports what was expected and returns
// a missing token.
func (p *parser) expect(k Kind) *Node {
	if p.at(k) {
		return p.bump()
	}
	p.errorExpected(quoteKind(k))
	return p.missing(k)
}

// expectWord consumes the contextual keyword w.
func (p *parser) expectWord(w string) *Node {
	if p.atWord(w) {
		return p.bumpAs(Keyword)
	}
	p.errorExpected("'" + w + "'")
	return p.missing(Keyword)
}

func quoteKind(k Kind) string {
	if k.IsNamed() {
		return article(k.String())
	}
	return "'" + k.String() + "'"
}

func article(name string) string {
	switch name {
	case "identifier", "integer":
		return "an " + name
	case "type_identifier":
		return "a type name"
	case "string_content":
		return "string text"
	}
	return "a " + name
}

// ---------------------------------------------------------------- line breaks

// atBreak reports whether a line break before the current token ends the
// current element: it does where an element may end, unless the line starts
// with a token that can only continue an expression.
func (p *parser) atBreak() bool {
	t := p.tok()
	return p.nl && t.Flags&NewlineBefore != 0 && !p.continues(t)
}

// continues reports whether a line starting with t continues the previous
// line. These are the tokens src/scanner.c recognises by their first
// characters: binary operators other than `-`, `.`, `?`, `,`, `:`, `=`, `->`,
// `!=`, the assignment operators and `else`.
func (p *parser) continues(t Token) bool {
	switch t.Kind {
	case Dot, DotDot, DotDotEq, DotBrace, Comma, Colon, Assign, Eq, FatArrow, Question,
		QuestionQuestion, Pipe, PipeGt, OrOr, AndAnd, Plus, Star, Slash, Percent, Lt, LtEq, Gt,
		GtEq, NotEq, Arrow, PlusEq, MinusEq, StarEq, SlashEq, PercentEq, KwElse:
		return true
	case Illegal:
		return p.src[t.Span.Start] == '&'
	}
	return false
}

// withNewlines runs f with line breaks significant or not, and restores the
// previous setting.
func (p *parser) withNewlines(significant bool, f func()) {
	saved := p.nl
	p.nl = significant
	f()
	p.nl = saved
}

// ---------------------------------------------------------------- diagnostics

func (p *parser) errorAt(span Span, format string, args ...any) {
	line := p.lines.Position(span.Start).Line
	if p.errLines[line] {
		return
	}
	p.errLines[line] = true
	p.diags = append(p.diags, Diagnostic{Span: span, Message: fmt.Sprintf(format, args...)})
}

// errorExpected reports that something else was expected at the current
// token. An illegal token already has a diagnostic from the lexer.
func (p *parser) errorExpected(what string) {
	t := p.tok()
	if t.Kind == Illegal || p.pos == p.errTok {
		return
	}
	p.errTok = p.pos
	switch {
	case t.Kind == EOF:
		// Point just after the last token rather than at the end of the file.
		end := p.prevEnd()
		p.errorAt(Span{end, end}, "expected %s, found end of file", what)
		return
	case t.Kind != StringStart && t.Flags&NewlineBefore != 0 && (p.nl || p.atRecoveryPoint()):
		// The line ended the construct; the next line is something else.
		end := p.prevEnd()
		p.errorAt(Span{end, end}, "expected %s before the end of the line", what)
		return
	}
	p.errorAt(t.Span, "expected %s, found %s", what, p.describe(t))
}

// errorMember reports that a declaration's body expected a member where the
// current token is. A member starts on a line of its own, so the token itself
// is the mistake, not the line break before it.
func (p *parser) errorMember(what string) {
	t := p.tok()
	if t.Kind == Illegal || p.pos == p.errTok {
		return
	}
	p.errTok = p.pos
	p.errorAt(t.Span, "expected %s, found %s", what, p.describe(t))
}

// describe names a token in a diagnostic: its text, or "a string".
func (p *parser) describe(t Token) string {
	if t.Kind == StringStart {
		return "a string"
	}
	return "'" + p.text(t) + "'"
}

// declarationWords are the contextual keywords that start a declaration.
var declarationWords = map[string]bool{
	"import": true, "type": true, "trait": true, "impl": true, "entity": true, "job": true,
	"view": true, "contract": true, "test": true, "extern": true,
}

// atRecoveryPoint reports whether the current token starts a line with a
// declaration or statement keyword, where error recovery stops skipping.
func (p *parser) atRecoveryPoint() bool {
	t := p.tok()
	if t.Flags&NewlineBefore == 0 {
		return false
	}
	switch t.Kind {
	case At, KwPub, KwFn, KwLet, KwVar, KwReturn, KwFor, KwWhile:
		return true
	case Ident:
		return declarationWords[p.text(t)]
	}
	return false
}

// ---------------------------------------------------------------- building

// builder collects the children of a node under construction.
type builder struct {
	kind     Kind
	children []*Node
}

func (p *parser) start(k Kind) *builder { return &builder{kind: k} }

func (b *builder) add(nodes ...*Node) {
	for _, n := range nodes {
		if n != nil {
			b.children = append(b.children, n)
		}
	}
}

func (b *builder) field(f Field, n *Node) {
	if n != nil {
		n.Field = f
		b.children = append(b.children, n)
	}
}

func (b *builder) done(p *parser) *Node {
	return newNode(b.kind, b.children, p.prevEnd())
}

func newNode(k Kind, children []*Node, at int) *Node {
	n := &Node{Kind: k, Children: children, Span: Span{at, at}}
	if len(children) > 0 {
		n.Span = Span{children[0].Span.Start, children[len(children)-1].Span.End}
	}
	return n
}

// wrap makes n the first child of a new node of kind k, with field f.
func wrap(k Kind, f Field, n *Node) *builder {
	n.Field = f
	return &builder{kind: k, children: []*Node{n}}
}

// ---------------------------------------------------------------- recovery

// errorNode wraps skipped tokens in an ERROR node.
func errorNode(children []*Node) *Node {
	if len(children) == 0 {
		return nil
	}
	return newNode(ErrorNode, children, children[0].Span.Start)
}

// skipUntil consumes tokens until stop reports true at nesting depth zero, a
// line starts with a declaration or statement keyword, a closing delimiter
// that this skip didn't open comes up, or the file ends. Brackets, strings and
// interpolations are skipped whole.
func (p *parser) skipUntil(stop func() bool) []*Node {
	var skipped []*Node
	depth := 0
	for !p.at(EOF) {
		if depth == 0 && (stop() || p.atRecoveryPoint()) {
			break
		}
		switch p.kind() {
		case LParen, LBracket, LBrace, StringStart, InterpOpen:
			depth++
		case RParen, RBracket, RBrace, StringEnd, InterpClose:
			if depth == 0 {
				return skipped
			}
			depth--
		}
		skipped = append(skipped, p.bump())
	}
	return skipped
}

// skipLine skips to the end of the current element: a line break that would
// end it, a `;` or `,` separator, or the closing delimiter.
func (p *parser) skipLine() *Node {
	saved := p.nl
	p.nl = true
	defer func() { p.nl = saved }()
	first := true
	return errorNode(p.skipUntil(func() bool {
		if first {
			first = false
			return false
		}
		return p.atBreak() || p.at(Semicolon) || p.at(Comma)
	}))
}

// ---------------------------------------------------------------- lists

// separated parses elements separated by line breaks or sep tokens, up to close,
// as source files, blocks and declaration bodies do. Each element is parsed
// with line breaks significant.
func (p *parser) separated(b *builder, close, sep Kind, element func() *Node) {
	saved := p.nl
	defer func() { p.nl = saved }()
	for {
		for p.at(sep) {
			b.add(p.bump())
		}
		if p.at(close) || p.at(EOF) {
			return
		}
		start := p.pos
		p.nl = true
		b.add(element())
		if p.pos == start {
			// The element consumed nothing and has reported why; skip the
			// token so the loop makes progress.
			b.add(errorNode([]*Node{p.bump()}))
			continue
		}
		if p.at(sep) || p.at(close) || p.at(EOF) || p.atBreak() {
			continue
		}
		if sep == Semicolon {
			p.errorExpected("a line break or ';'")
		} else {
			p.errorExpected("a line break or ','")
		}
		b.add(p.skipLine())
	}
}

// commaList parses elements separated by commas, with an optional trailing
// comma, up to close. Line breaks inside the list are spaces. It returns the
// number of elements.
func (p *parser) commaList(b *builder, close Kind, element func() *Node) int {
	saved := p.nl
	p.nl = false
	defer func() { p.nl = saved }()
	count := 0
	for !p.at(close) && !p.at(EOF) {
		start := p.pos
		b.add(element())
		count++
		if p.at(Comma) {
			b.add(p.bump())
			continue
		}
		if p.at(close) {
			break
		}
		p.errorExpected("',' or '" + close.String() + "'")
		skipped := p.skipUntil(func() bool { return p.at(Comma) || p.at(close) })
		if len(skipped) == 0 && p.pos == start {
			break
		}
		b.add(errorNode(skipped))
		if p.at(Comma) {
			b.add(p.bump())
			continue
		}
		break
	}
	return count
}

// delimited parses open, a comma list and close into b. If the list must not
// be empty and is, it reports what was expected.
func (p *parser) delimited(b *builder, open, close Kind, what string, nonEmpty bool, element func() *Node) {
	b.add(p.expect(open))
	if n := p.commaList(b, close, element); n == 0 && nonEmpty {
		p.errorExpected(what)
	}
	b.add(p.expectClose(close))
}

// expectClose consumes the closing delimiter, skipping anything left before
// it on the way.
func (p *parser) expectClose(close Kind) *Node {
	if p.at(close) {
		return p.bump()
	}
	p.errorExpected("'" + close.String() + "'")
	return p.missing(close)
}
