package syntax

// Binding strength of binary operators, loosest first, as in grammar.js.
const (
	precDefault  = 1 // ??
	precOr       = 2 // ||
	precAnd      = 3 // &&
	precEquality = 4 // == !=
	precCompare  = 5 // < <= > >=
	precRange    = 6 // .. ..=
	precPipe     = 7 // |>
	precAdd      = 8 // + -
	precMultiply = 9 // * / %
)

func binaryPrec(k Kind) (prec int, rightAssoc bool) {
	switch k {
	case QuestionQuestion:
		return precDefault, true
	case OrOr:
		return precOr, false
	case AndAnd:
		return precAnd, false
	case Eq, NotEq:
		return precEquality, false
	case Lt, LtEq, Gt, GtEq:
		return precCompare, false
	case DotDot, DotDotEq:
		return precRange, false
	case PipeGt:
		return precPipe, false
	case Plus, Minus:
		return precAdd, false
	case Star, Slash, Percent:
		return precMultiply, false
	}
	return 0, false
}

// startsExpr reports whether the current token can start an expression.
func (p *parser) startsExpr() bool {
	switch p.kind() {
	case Ident, TypeIdent, Underscore, Int, Float, Date, Time, Char, StringStart, KwTrue, KwFalse,
		LParen, LBracket, LBrace, KwFn, KwIf, KwMatch, Minus, Bang:
		return true
	}
	return false
}

// expr parses an expression.
func (p *parser) expr() *Node { return p.binary(precDefault, false) }

// binary parses operators that bind at least as tightly as minPrec. With
// openRange, a `..` directly before `]` is left for a slice like `xs[i..]`;
// that only applies at the top of an index, where no looser operator is
// waiting for its right side.
func (p *parser) binary(minPrec int, openRange bool) *Node {
	left := p.unary()
	for {
		k := p.kind()
		prec, rightAssoc := binaryPrec(k)
		if prec == 0 || prec < minPrec || p.atBreak() {
			return left
		}
		if openRange && k == DotDot && p.peek(1).Kind == RBracket {
			return left
		}
		kind, leftField, rightField := BinaryExpression, FieldLeft, FieldRight
		if k == DotDot || k == DotDotEq {
			kind, leftField, rightField = RangeExpression, FieldStart, FieldEnd
		}
		b := wrap(kind, leftField, left)
		b.field(FieldOperator, p.bump())
		next := prec + 1
		if rightAssoc {
			next = prec
		}
		b.field(rightField, p.binary(next, false))
		left = b.done(p)
	}
}

// unary parses prefix operators, which bind looser than postfix operators and
// tighter than binary ones.
func (p *parser) unary() *Node {
	switch p.kind() {
	case Minus, Bang:
		b := p.start(UnaryExpression)
		b.field(FieldOperator, p.bump())
		b.field(FieldOperand, p.unary())
		return b.done(p)
	}
	return p.postfix(p.primary(), func(*Node) bool { return true })
}

// postfix parses calls, waiting calls, field access, indexing and `?` after
// e. canEnd reports whether the expression could end with what has been
// parsed so far; only then can a line break end it.
func (p *parser) postfix(e *Node, canEnd func(*Node) bool) *Node {
	for {
		t := p.tok()
		brk := p.atBreak() && canEnd(e)
		switch {
		case t.Kind == LParen && !brk:
			b := wrap(CallExpression, FieldFunction, e)
			b.field(FieldArguments, p.arguments())
			e = b.done(p)
		case t.Kind == Bang && t.Flags&SpaceBefore == 0 &&
			p.peek(1).Kind == LParen && p.peek(1).Flags&SpaceBefore == 0:
			b := wrap(WaitingCallExpression, FieldFunction, e)
			b.add(p.bump())
			b.field(FieldArguments, p.arguments())
			e = b.done(p)
		case t.Kind == Dot:
			b := wrap(FieldExpression, FieldValue, e)
			b.add(p.bump())
			b.field(FieldField, p.name())
			e = b.done(p)
		case t.Kind == LBracket && !brk:
			b := wrap(IndexExpression, FieldValue, e)
			b.add(p.bump())
			p.withNewlines(false, func() { b.field(FieldIndex, p.index()) })
			b.add(p.expectClose(RBracket))
			e = b.done(p)
		case t.Kind == Question && t.Flags&SpaceBefore == 0:
			b := wrap(TryExpression, FieldValue, e)
			b.add(p.bump())
			e = b.done(p)
		default:
			return e
		}
	}
}

// index parses what goes between the brackets of `xs[…]`: an index, a range,
// or an open range `..n`, `..=n` or `i..`.
func (p *parser) index() *Node {
	if p.at(DotDot) || p.at(DotDotEq) {
		b := p.start(RangeExpression)
		b.field(FieldOperator, p.bump())
		b.field(FieldEnd, p.expr())
		return b.done(p)
	}
	e := p.binary(precDefault, true)
	if p.at(DotDot) && p.peek(1).Kind == RBracket {
		b := wrap(RangeExpression, FieldStart, e)
		b.field(FieldOperator, p.bump())
		return b.done(p)
	}
	return e
}

// arguments parses `(a, mut b, name: c)`. `in` and `for` may be labels.
func (p *parser) arguments() *Node {
	b := p.start(Arguments)
	p.delimited(b, LParen, RParen, "", false, func() *Node {
		switch p.kind() {
		case KwMut:
			return p.mutArgument()
		case Ident, KwIn, KwFor:
			if p.peek(1).Kind == Colon {
				arg := p.start(NamedArgument)
				arg.field(FieldName, p.bumpAs(Ident))
				arg.add(p.bump())
				if p.at(KwMut) {
					arg.field(FieldValue, p.mutArgument())
				} else {
					arg.field(FieldValue, p.expr())
				}
				return arg.done(p)
			}
		}
		return p.expr()
	})
	return b.done(p)
}

// mutArgument parses `mut x`: a variable the call may change.
func (p *parser) mutArgument() *Node {
	b := p.start(MutArgument)
	b.add(p.bump())
	b.field(FieldValue, p.expr())
	return b.done(p)
}

func (p *parser) primary() *Node {
	switch p.kind() {
	case Ident, TypeIdent, Char:
		return p.bump()
	case Underscore:
		b := p.start(Placeholder)
		b.add(p.bump())
		return b.done(p)
	case Int, Float:
		return p.number()
	case Date:
		date := p.bump()
		if !p.at(Time) || p.atBreak() {
			return date
		}
		b := wrap(Instant, FieldDate, date)
		b.field(FieldTime, p.bump())
		b.add(p.expect(KwIn))
		b.field(FieldZone, p.stringLiteral())
		return b.done(p)
	case Time:
		t := p.bump()
		if !p.at(KwIn) || p.atBreak() {
			return t
		}
		b := wrap(ZonedTime, FieldTime, t)
		b.add(p.bump())
		b.field(FieldZone, p.stringLiteral())
		return b.done(p)
	case StringStart:
		return p.stringLiteral()
	case KwTrue, KwFalse:
		b := p.start(Boolean)
		b.add(p.bump())
		return b.done(p)
	case LParen:
		if p.peek(1).Kind == RParen {
			b := p.start(Unit)
			b.add(p.bump(), p.bump())
			return b.done(p)
		}
		// `(e)` groups; `(a, b)` is a tuple, which needs at least two elements.
		b := p.start(ParenthesizedExpression)
		b.add(p.bump())
		p.withNewlines(false, func() {
			b.add(p.expr())
			if !p.at(Comma) {
				return
			}
			b.kind = Tuple
			more := 0
			for p.at(Comma) {
				b.add(p.bump())
				if p.at(RParen) {
					break
				}
				b.add(p.expr())
				more++
			}
			if more == 0 {
				p.errorExpected("a second element; a tuple has at least two")
			}
		})
		b.add(p.expectClose(RParen))
		return b.done(p)
	case LBracket:
		b := p.start(List)
		p.delimited(b, LBracket, RBracket, "", false, p.expr)
		return b.done(p)
	case LBrace:
		return p.braces()
	case KwFn:
		return p.lambda()
	case KwIf:
		return p.ifExpression()
	case KwMatch:
		return p.matchExpression()
	}
	p.errorExpected("an expression")
	if p.at(RParen) || p.at(RBracket) || p.at(RBrace) || p.at(Comma) || p.at(Semicolon) ||
		p.at(EOF) || p.at(InterpClose) || p.at(StringEnd) || p.tok().Flags&NewlineBefore != 0 {
		return p.missing(Ident)
	}
	return errorNode([]*Node{p.bump()})
}

var durationUnits = map[string]bool{
	"ms": true, "s": true, "min": true, "h": true, "day": true, "days": true, "week": true, "weeks": true,
}

var sizeUnits = map[string]bool{"B": true, "KiB": true, "MiB": true, "GiB": true}

// number parses an Int or Float and the unit that may follow it.
func (p *parser) number() *Node {
	n := p.bump()
	if p.atBreak() {
		return n
	}
	kind := Kind(0)
	switch {
	case p.at(Ident) && durationUnits[p.text(p.tok())]:
		kind = Duration
	case p.at(TypeIdent) && sizeUnits[p.text(p.tok())]:
		kind = Size
	default:
		return n
	}
	b := wrap(kind, FieldValue, n)
	b.field(FieldUnit, p.bumpAs(UnitName))
	return b.done(p)
}

// stringLiteral parses a string with its escapes and interpolations. The
// lexer always closes a string, if need be with a missing quote.
func (p *parser) stringLiteral() *Node {
	b := p.start(String)
	if !p.at(StringStart) {
		p.errorExpected("a string")
		b.add(p.missing(StringStart), p.missing(StringEnd))
		return b.done(p)
	}
	b.add(p.bump())
	for !p.at(StringEnd) && !p.at(EOF) {
		switch p.kind() {
		case StringText, StringEscape, Illegal:
			b.add(p.bump())
		case InterpOpen:
			in := p.start(Interpolation)
			in.add(p.bump())
			p.withNewlines(false, func() { in.field(FieldValue, p.expr()) })
			if !p.at(InterpClose) {
				p.errorExpected("'}'")
				in.add(errorNode(p.skipUntil(func() bool { return p.at(InterpClose) })))
			}
			in.add(p.expect(InterpClose))
			b.add(in.done(p))
		default:
			// Only reachable after a parse error inside an interpolation.
			b.add(errorNode([]*Node{p.bump()}))
		}
	}
	b.add(p.expect(StringEnd))
	return b.done(p)
}

// braces parses what a `{` starts in an expression: `{}`, a record, a record
// update or a map. A block's `{` never gets here; see block.
func (p *parser) braces() *Node {
	open := p.bump()
	if p.at(RBrace) {
		b := p.start(EmptyBraces)
		b.add(open, p.bump())
		return b.done(p)
	}
	var result *Node
	p.withNewlines(false, func() {
		if p.at(Ident) {
			switch p.peek(1).Kind {
			case Colon, Comma, RBrace:
				result = p.record(open)
				return
			}
		}
		first := p.expr()
		switch {
		case p.atWord("with"):
			b := p.start(RecordUpdate)
			b.add(open)
			b.field(FieldBase, first)
			b.add(p.bumpAs(Keyword))
			if p.commaList(b, RBrace, p.fieldInitializer) == 0 {
				p.errorExpected("a field")
			}
			b.add(p.expectClose(RBrace))
			result = b.done(p)
		case p.at(Colon) && isMapKey(first):
			b := p.start(Map)
			b.add(open)
			entry := wrap(MapEntry, FieldKey, first)
			entry.add(p.bump())
			entry.field(FieldValue, p.expr())
			b.add(entry.done(p))
			if p.at(Comma) {
				b.add(p.bump())
				p.commaList(b, RBrace, p.mapEntry)
			}
			b.add(p.expectClose(RBrace))
			result = b.done(p)
		default:
			if p.at(Colon) {
				p.errorAt(first.Span, "a map key must be a string, integer, date, time or parenthesised expression")
			} else {
				p.errorExpected("'with' or ':'")
			}
			b := p.start(ErrorNode)
			b.add(open, first)
			b.add(p.skipUntil(func() bool { return false })...)
			b.add(p.expectClose(RBrace))
			result = b.done(p)
		}
	})
	return result
}

func isMapKey(n *Node) bool {
	switch n.Kind {
	case String, Int, Date, Time, ParenthesizedExpression:
		return true
	}
	return false
}

func (p *parser) record(open *Node) *Node {
	b := p.start(Record)
	b.add(open)
	p.commaList(b, RBrace, func() *Node {
		if p.at(Ident) && p.peek(1).Kind != Colon {
			s := p.start(ShorthandField)
			s.add(p.bump())
			return s.done(p)
		}
		return p.fieldInitializer()
	})
	b.add(p.expectClose(RBrace))
	return b.done(p)
}

func (p *parser) fieldInitializer() *Node {
	b := p.start(FieldInitializer)
	b.field(FieldName, p.expect(Ident))
	b.add(p.expect(Colon))
	b.field(FieldValue, p.expr())
	return b.done(p)
}

func (p *parser) mapEntry() *Node {
	b := p.start(MapEntry)
	switch p.kind() {
	case StringStart:
		b.field(FieldKey, p.stringLiteral())
	case Int, Date, Time:
		b.field(FieldKey, p.bump())
	case LParen:
		key := p.start(ParenthesizedExpression)
		key.add(p.bump())
		key.add(p.expr())
		key.add(p.expectClose(RParen))
		b.field(FieldKey, key.done(p))
	default:
		p.errorExpected("a map key")
		return errorNode(p.skipUntil(func() bool { return p.at(Comma) || p.at(RBrace) }))
	}
	b.add(p.expect(Colon))
	b.field(FieldValue, p.expr())
	return b.done(p)
}

// lambda parses `fn x: e`, `fn (a, b): e`, `fn { … }` and `fn x { … }`. An
// expression body takes everything to its right.
func (p *parser) lambda() *Node {
	b := p.start(Lambda)
	b.add(p.bump())
	switch p.kind() {
	case Ident:
		b.field(FieldParameter, p.bump())
	case LParen:
		p.withNewlines(false, func() { b.field(FieldParameters, p.parameters()) })
	}
	switch {
	case p.at(Colon):
		b.add(p.bump())
		b.field(FieldBody, p.expr())
	case p.at(LBrace):
		b.field(FieldBody, p.block())
	default:
		p.errorExpected("':' or '{'")
	}
	return b.done(p)
}

func (p *parser) ifExpression() *Node {
	b := p.start(IfExpression)
	b.add(p.bump())
	p.withNewlines(false, func() { b.field(FieldCondition, p.expr()) })
	b.field(FieldConsequence, p.block())
	if p.at(KwElse) {
		b.add(p.bump())
		if p.at(KwIf) {
			b.field(FieldAlternative, p.ifExpression())
		} else {
			b.field(FieldAlternative, p.block())
		}
	}
	return b.done(p)
}

func (p *parser) matchExpression() *Node {
	b := p.start(MatchExpression)
	b.add(p.bump())
	p.withNewlines(false, func() { b.field(FieldValue, p.expr()) })
	body := p.start(MatchBody)
	body.add(p.expect(LBrace))
	p.separated(body, RBrace, Comma, p.matchArm)
	body.add(p.expectClose(RBrace))
	b.field(FieldBody, body.done(p))
	return b.done(p)
}

// matchArm parses `pattern if guard => value`. A value that starts with `{`
// is a block.
func (p *parser) matchArm() *Node {
	b := p.start(MatchArm)
	p.withNewlines(false, func() {
		b.field(FieldPattern, p.pattern())
		if p.at(KwIf) {
			b.add(p.bump())
			b.field(FieldGuard, p.expr())
		}
		b.add(p.expect(FatArrow))
	})
	if p.at(LBrace) {
		b.field(FieldValue, p.block())
	} else {
		b.field(FieldValue, p.expr())
	}
	return b.done(p)
}
