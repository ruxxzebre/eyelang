package syntax

import (
	"errors"
	"strings"
)

// Format returns src in Eyelang's one canonical style. Like gofmt, it keeps
// the line breaks the author chose and normalises everything else:
//
//   - indentation is two spaces per open bracket, plus two for a line that
//     continues a statement;
//   - spaces between tokens follow fixed rules (`f(x)`, `a + b`, `x: Int`,
//     `{ a: 1 }`, `0..n`);
//   - `;` between statements becomes a line break, and a block, body or match
//     that spans lines puts its braces on their own lines;
//   - a list that ends on its own line gets a trailing comma, and one that
//     doesn't loses it;
//   - comments stay where they are, and trailing comments on consecutive lines
//     are aligned;
//   - at most one blank line in a row, none after `{` or before `}`, and one
//     newline at the end.
//
// Formatting never changes what a program means: the tree of the result is
// the tree of src, apart from separators and trailing commas. A file with
// syntax errors isn't formatted.
func Format(src []byte) ([]byte, error) {
	f := Parse(src)
	if len(f.Diagnostics) > 0 {
		return nil, errors.New("the file has syntax errors")
	}
	fm := &formatter{src: src}
	fm.collect(f.Root, nil)
	fm.plan(f.Root)
	return fm.print(), nil
}

// ftoken is a token with what the formatter needs to know about it.
type ftoken struct {
	n      *Node
	parent *Node
	text   string

	newlines int        // line breaks before the token in the source, comments included
	comments []fcomment // comments before the token

	drop      bool // a separator the formatter leaves out
	addComma  bool // a trailing comma goes after this token
	breakLine bool // the token starts a line in the output
	joinLine  bool // the token stays on the previous line though the source broke it

	elementEnd int  // for the first token of a statement, member or arm: index of its last token
	declStart  bool // starts an attribute, or the declaration after its attributes
}

type fcomment struct {
	text    string
	ownLine bool // on a line of its own, rather than after code
	blank   bool // a blank line comes before it
}

type formatter struct {
	src    []byte
	tokens []*ftoken
	index  map[*Node]int // token node to its index in tokens
}

// collect flattens the tree into tokens in source order, with their parents
// and leading comments.
func (fm *formatter) collect(n, parent *Node) {
	if fm.index == nil {
		fm.index = map[*Node]int{}
	}
	if n.IsToken() {
		if n.Kind == EOF && len(n.Leading) == 0 {
			return
		}
		t := &ftoken{n: n, parent: parent, text: n.Text(fm.src), elementEnd: -1}
		lines := 0
		for _, tr := range n.Leading {
			text := string(fm.src[tr.Span.Start:tr.Span.End])
			if tr.Kind == Whitespace {
				lines += strings.Count(text, "\n")
				continue
			}
			first := len(t.comments) == 0
			t.comments = append(t.comments, fcomment{
				text:    strings.TrimRight(text, " \t\r"),
				ownLine: !(first && lines == 0) || len(fm.tokens) == 0,
				blank:   lines >= 2,
			})
			lines = 0
		}
		t.newlines = lines
		if len(t.comments) > 0 {
			t.newlines = max(lines, 1)
		}
		fm.index[n] = len(fm.tokens)
		fm.tokens = append(fm.tokens, t)
		return
	}
	for _, c := range n.Children {
		fm.collect(c, n)
	}
}

// Nodes whose children are separated by line breaks.
var lineLists = map[Kind]bool{
	SourceFile: true, Block: true, EntityBody: true, TraitBody: true, ImplBody: true,
	ContractBody: true, MatchBody: true,
}

// Nodes whose children are separated by commas between brackets.
var commaLists = map[Kind]bool{
	Arguments: true, Parameters: true, List: true, Record: true, RecordUpdate: true, Map: true,
	Tuple: true, TupleType: true, TypeArguments: true, VariantPayload: true, PatternArguments: true,
	RecordPattern: true, ListPattern: true, TuplePattern: true, UsesClause: true,
	ImportDeclaration: true, TypeParameters: true, FunctionTypeParameters: true, RecordType: true,
}

// plan decides which tokens start lines, which separators go, and where
// trailing commas belong.
func (fm *formatter) plan(n *Node) {
	if n.IsToken() {
		return
	}
	for _, c := range n.Children {
		fm.plan(c)
	}
	// A declaration's attributes and the declaration itself each start a line
	// at the declaration's own indentation, not as a continuation.
	for i, c := range n.Children {
		if c.Kind == Attribute {
			if t := fm.firstTokenOf(c); t != nil {
				t.declStart = true
			}
			if i+1 < len(n.Children) && n.Children[i+1].Kind != Attribute {
				if t := fm.firstTokenOf(n.Children[i+1]); t != nil {
					t.declStart = true
				}
			}
		}
	}
	switch {
	case lineLists[n.Kind]:
		fm.planLines(n)
	case commaLists[n.Kind]:
		fm.planCommas(n)
	case n.Kind == IfExpression:
		// `}` and `else` share a line.
		for _, c := range n.Children {
			if c.Kind == KwElse {
				if i, ok := fm.index[c]; ok && i > 0 && fm.tokens[i-1].n.Kind == RBrace && len(fm.tokens[i].comments) == 0 {
					fm.tokens[i].joinLine = true
				}
			}
		}
	}
}

func (fm *formatter) planLines(n *Node) {
	var open, close *ftoken
	inner := n.Children
	if n.Kind != SourceFile {
		open, close = fm.tok(inner[0]), fm.tok(inner[len(inner)-1])
		inner = inner[1 : len(inner)-1]
	}
	multiline := false
	for i, c := range inner {
		if c.Kind == Semicolon || (c.Kind == Comma && n.Kind == MatchBody) {
			sep := fm.tok(c)
			next := fm.firstToken(inner, i+1)
			if c.Kind == Semicolon || next == nil || next.newlines > 0 {
				sep.drop = true
				if next != nil {
					next.breakLine = true
				}
			}
			if c.Kind == Semicolon {
				multiline = true
			}
			continue
		}
		first := fm.firstTokenOf(c)
		if first != nil {
			if first.newlines > 0 {
				multiline = true
			}
			first.elementEnd = fm.index[lastTokenOf(c)]
		}
		if fm.hasLineBreak(c) {
			multiline = true
		}
	}
	if close != nil && (close.newlines > 0 || len(close.comments) > 0) {
		multiline = true
	}
	if open == nil || !multiline {
		return
	}
	if next := fm.firstToken(inner, 0); next != nil {
		next.breakLine = true
	}
	close.breakLine = true
}

func (fm *formatter) planCommas(n *Node) {
	kids := n.Children
	closeNode := kids[len(kids)-1]
	if closeNode.Kind != RParen && closeNode.Kind != RBracket && closeNode.Kind != RBrace {
		return
	}
	close := fm.tok(closeNode)
	if close == nil || close.n.IsMissing() {
		return
	}
	prev := kids[len(kids)-2]
	if prev.IsToken() && (prev.Kind == LParen || prev.Kind == LBracket || prev.Kind == LBrace || prev.Kind == DotBrace) {
		return // an empty list
	}
	onOwnLine := close.newlines > 0 || len(close.comments) > 0
	if prev.Kind == Comma {
		if !onOwnLine {
			fm.tok(prev).drop = true
		}
		return
	}
	if onOwnLine {
		fm.lastToken(prev).addComma = true
	}
}

func (fm *formatter) tok(n *Node) *ftoken {
	if i, ok := fm.index[n]; ok {
		return fm.tokens[i]
	}
	return nil
}

func (fm *formatter) firstTokenOf(n *Node) *ftoken {
	if n.IsToken() {
		return fm.tok(n)
	}
	for _, c := range n.Children {
		if t := fm.firstTokenOf(c); t != nil {
			return t
		}
	}
	return nil
}

func lastTokenOf(n *Node) *Node {
	for !n.IsToken() && len(n.Children) > 0 {
		n = n.Children[len(n.Children)-1]
	}
	return n
}

func (fm *formatter) lastToken(n *Node) *ftoken { return fm.tok(lastTokenOf(n)) }

func (fm *formatter) firstToken(nodes []*Node, from int) *ftoken {
	for _, c := range nodes[from:] {
		if t := fm.firstTokenOf(c); t != nil {
			return t
		}
	}
	return nil
}

// hasLineBreak reports whether a line break separates any two tokens of n.
func (fm *formatter) hasLineBreak(n *Node) bool {
	first, last := fm.firstTokenOf(n), fm.lastToken(n)
	if first == nil || last == nil {
		return false
	}
	for i := fm.index[first.n] + 1; i <= fm.index[last.n]; i++ {
		if fm.tokens[i].newlines > 0 || fm.tokens[i].breakLine {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- printing

type fline struct {
	indent  int
	code    strings.Builder
	comment string
	blank   bool // a blank line comes before it
	empty   bool // holds only a comment
}

type bracket struct {
	lineIndent    int // indentation of the line the bracket opened on
	contentIndent int // indentation of the lines inside it
}

type element struct {
	end    int // index of the element's last token
	depth  int // bracket depth where the element started
	first  int // index of its first token
	indent int // indentation of its first line
}

func (fm *formatter) print() []byte {
	var lines []*fline
	var brackets []bracket
	var elements []element
	var cur *fline
	var prev *ftoken

	newLine := func(indent int, blank bool) *fline {
		l := &fline{indent: indent, blank: blank && len(lines) > 0}
		lines = append(lines, l)
		return l
	}
	contentIndent := func() int {
		if len(brackets) == 0 {
			return 0
		}
		return brackets[len(brackets)-1].contentIndent
	}

	for i, t := range fm.tokens {
		for len(elements) > 0 && elements[len(elements)-1].end < i {
			elements = elements[:len(elements)-1]
		}
		closer := isCloser(t.n.Kind)

		// The indentation a line starting with this token gets.
		indent := contentIndent()
		if closer && len(brackets) > 0 {
			indent = brackets[len(brackets)-1].lineIndent
		} else if len(elements) > 0 {
			e := elements[len(elements)-1]
			if e.first != i && e.depth == len(brackets) && !t.declStart {
				indent = e.indent + 2
			}
		}

		ownLine := 0 // own-line comments emitted before the token
		for _, c := range t.comments {
			if !c.ownLine && cur != nil {
				cur.comment = c.text
				continue
			}
			ci := indent
			if closer {
				ci = contentIndent()
			}
			l := newLine(ci, c.blank && !(ownLine == 0 && afterOpener(prev)))
			l.comment, l.empty = c.text, true
			cur = nil
			ownLine++
		}

		if t.n.Kind == EOF {
			break
		}
		if t.drop {
			continue
		}

		starts := cur == nil || t.breakLine || (t.newlines > 0 && !t.joinLine)
		if starts {
			blank := t.newlines >= 2 && !closer && (!afterOpener(prev) || ownLine > 0)
			cur = newLine(indent, blank)
		} else if prev != nil && space(prev, t) {
			cur.code.WriteByte(' ')
		}
		cur.code.WriteString(t.text)
		if t.addComma {
			cur.code.WriteByte(',')
		}

		if t.elementEnd >= 0 {
			elements = append(elements, element{end: t.elementEnd, depth: len(brackets), first: i, indent: cur.indent})
		}
		switch {
		case isOpener(t.n.Kind):
			brackets = append(brackets, bracket{lineIndent: cur.indent, contentIndent: openContent(brackets, cur.indent)})
		case closer && len(brackets) > 0:
			brackets = brackets[:len(brackets)-1]
		}
		prev = t
	}
	return render(lines)
}

// openContent gives the indentation inside a new bracket. Brackets opened on
// the same line share one level, so `f(fn {` indents its block once.
func openContent(brackets []bracket, lineIndent int) int {
	if len(brackets) > 0 {
		top := brackets[len(brackets)-1]
		if top.lineIndent == lineIndent && top.contentIndent > lineIndent {
			return top.contentIndent
		}
	}
	return lineIndent + 2
}

func afterOpener(t *ftoken) bool { return t == nil || isOpener(t.n.Kind) }

func isOpener(k Kind) bool {
	return k == LParen || k == LBracket || k == LBrace || k == DotBrace || k == InterpOpen
}

func isCloser(k Kind) bool {
	return k == RParen || k == RBracket || k == RBrace || k == InterpClose
}

// render writes the lines, aligning the trailing comments of consecutive
// lines.
func render(lines []*fline) []byte {
	var b strings.Builder
	for i := 0; i < len(lines); {
		// A run of consecutive lines with code and a trailing comment.
		j, width := i, 0
		for j < len(lines) && !lines[j].empty && lines[j].comment != "" && (j == i || !lines[j].blank) {
			width = max(width, lines[j].indent+lines[j].code.Len())
			j++
		}
		if j == i {
			j = i + 1
		}
		for _, l := range lines[i:j] {
			if l.blank {
				b.WriteByte('\n')
			}
			code := strings.Repeat(" ", l.indent) + l.code.String()
			if l.empty {
				code = strings.Repeat(" ", l.indent)
			}
			b.WriteString(code)
			if l.comment != "" {
				if !l.empty {
					b.WriteString(strings.Repeat(" ", max(width-len(code), 0)+2))
				}
				b.WriteString(l.comment)
			}
			b.WriteByte('\n')
		}
		i = j
	}
	return []byte(b.String())
}

// space reports whether a space separates two tokens on one line.
func space(a, b *ftoken) bool {
	ak, bk := a.n.Kind, b.n.Kind
	ap, bp := a.parent.Kind, b.parent.Kind
	switch {
	// Inside strings nothing is added; interpolations hug their braces.
	case ak == StringStart || bk == StringEnd,
		ak == StringText || ak == StringEscape || ak == InterpClose || ak == Illegal && ap == String,
		bk == StringText || bk == StringEscape || bk == InterpOpen || bk == Illegal && bp == String,
		ak == InterpOpen || bk == InterpClose:
		return false
	case bk == Comma || bk == Semicolon || bk == Colon:
		return false
	case ak == LParen || ak == LBracket || ak == At || ak == DotBrace || bk == RParen || bk == RBracket:
		return false
	case ak == Dot || bk == Dot || bk == DotBrace:
		return false
	case ak == LBrace && bk == RBrace:
		return false
	case ak == LBrace && ap == UsesClause, bk == RBrace && (bp == UsesClause || bp == ImportDeclaration):
		return false
	case bk == Question && bp == TryExpression, bk == Bang && bp == WaitingCallExpression,
		ak == Bang && ap == WaitingCallExpression:
		return false
	case (ak == Minus || ak == Bang) && (ap == UnaryExpression || ap == NegativeLiteral):
		return false
	case (ak == DotDot || ak == DotDotEq) && (ap == RangeExpression || ap == RangePattern || ap == RestPattern),
		(bk == DotDot || bk == DotDotEq) && (bp == RangeExpression || bp == RangePattern):
		return false
	case ak == KwFn && ap == FunctionType:
		return false
	case bk == LBracket && bp == IndexExpression:
		return false
	case bk == LParen:
		switch b.parent.Kind {
		case Arguments, TypeArguments, VariantPayload, PatternArguments, TypeParameters, FunctionTypeParameters:
			return false
		case Parameters:
			return ap == Lambda // `fn (a, b): …`, but `fn f(a, b)`
		}
	}
	return true
}
