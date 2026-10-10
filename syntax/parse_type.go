package syntax

// typeExpr parses a type: `Int`, `List(T)`, `notify.Delivery`, `()`,
// `{ name: String }` or `fn(Int) -> Bool`.
func (p *parser) typeExpr() *Node {
	if p.atWord("dyn") {
		b := p.start(DynType)
		b.add(p.bumpAs(Keyword))
		b.field(FieldTrait, p.typeExpr())
		return b.done(p)
	}
	switch p.kind() {
	case TypeIdent:
		return p.typeArguments(p.bump())
	case Ident:
		if p.peek(1).Kind != Dot {
			b := p.start(TypeVariable)
			b.add(p.bump())
			return b.done(p)
		}
		b := p.start(QualifiedType)
		b.field(FieldModule, p.bump())
		b.add(p.bump())
		b.field(FieldName, p.expect(TypeIdent))
		return p.typeArguments(b.done(p))
	case LParen:
		// `()` is the unit type; `(A, B)` a tuple type, with at least two.
		b := p.start(UnitType)
		b.add(p.bump())
		if !p.at(RParen) {
			b.kind = TupleType
			p.withNewlines(false, func() {
				b.add(p.typeExpr())
				more := 0
				for p.at(Comma) {
					b.add(p.bump())
					if p.at(RParen) {
						break
					}
					b.add(p.typeExpr())
					more++
				}
				if more == 0 {
					p.errorExpected("',' and a second type; a tuple type has at least two")
				}
			})
		}
		b.add(p.expectClose(RParen))
		return b.done(p)
	case LBrace:
		b := p.start(RecordType)
		p.delimited(b, LBrace, RBrace, "", false, p.fieldDeclaration)
		return b.done(p)
	case KwFn:
		b := p.start(FunctionType)
		b.add(p.bump())
		params := p.start(FunctionTypeParameters)
		p.delimited(params, LParen, RParen, "", false, p.typeExpr)
		b.field(FieldParameters, params.done(p))
		if p.at(Arrow) {
			b.add(p.bump())
			b.field(FieldReturnType, p.typeExpr())
		}
		return b.done(p)
	}
	p.errorExpected("a type")
	return p.missing(TypeIdent)
}

// typeArguments parses the `(A, B)` after a type name, if there is one.
func (p *parser) typeArguments(name *Node) *Node {
	if !p.at(LParen) || p.atBreak() {
		return name
	}
	b := wrap(GenericType, FieldName, name)
	args := p.start(TypeArguments)
	p.delimited(args, LParen, RParen, "a type", true, p.typeExpr)
	b.field(FieldArguments, args.done(p))
	return b.done(p)
}

// pattern parses a pattern, including alternatives: `A | B`.
func (p *parser) pattern() *Node {
	left := p.primaryPattern()
	for p.at(Pipe) {
		b := wrap(OrPattern, FieldLeft, left)
		b.add(p.bump())
		b.field(FieldRight, p.primaryPattern())
		left = b.done(p)
	}
	return left
}

func (p *parser) primaryPattern() *Node {
	switch p.kind() {
	case Int, Char:
		return p.rangePattern(p.bump())
	case Ident, Float:
		return p.bump()
	case Underscore:
		b := p.start(Wildcard)
		b.add(p.bump())
		return b.done(p)
	case StringStart:
		return p.stringLiteral()
	case KwTrue, KwFalse:
		b := p.start(Boolean)
		b.add(p.bump())
		return b.done(p)
	case LParen:
		// `()` matches unit; `(a, b)` a tuple, with at least two elements.
		b := p.start(Unit)
		b.add(p.bump())
		if !p.at(RParen) {
			b.kind = TuplePattern
			p.withNewlines(false, func() {
				b.add(p.pattern())
				more := 0
				for p.at(Comma) {
					b.add(p.bump())
					if p.at(RParen) {
						break
					}
					b.add(p.pattern())
					more++
				}
				if more == 0 {
					p.errorExpected("',' and a second pattern; a tuple has at least two elements")
				}
			})
		}
		b.add(p.expectClose(RParen))
		return b.done(p)
	case Minus:
		b := p.start(NegativeLiteral)
		b.add(p.bump())
		if p.at(Int) || p.at(Float) {
			b.add(p.bump())
		} else {
			p.errorExpected("a number")
		}
		return p.rangePattern(b.done(p))
	case TypeIdent:
		b := p.start(VariantPattern)
		b.field(FieldName, p.bump())
		if p.at(LParen) {
			args := p.start(PatternArguments)
			p.delimited(args, LParen, RParen, "", false, func() *Node {
				switch {
				case p.at(DotDot):
					return p.restPattern()
				case p.at(Ident) && p.peek(1).Kind == Colon:
					return p.fieldPattern()
				}
				return p.pattern()
			})
			b.field(FieldArguments, args.done(p))
		}
		return b.done(p)
	case LBrace:
		b := p.start(RecordPattern)
		p.delimited(b, LBrace, RBrace, "", false, func() *Node {
			switch {
			case p.at(DotDot):
				return p.restPattern()
			case p.at(Ident) && p.peek(1).Kind == Colon:
				return p.fieldPattern()
			case p.at(Ident):
				s := p.start(ShorthandFieldPattern)
				s.add(p.bump())
				return s.done(p)
			}
			p.errorExpected("a field")
			return errorNode(p.skipUntil(func() bool { return p.at(Comma) || p.at(RBrace) }))
		})
		return b.done(p)
	case LBracket:
		b := p.start(ListPattern)
		p.delimited(b, LBracket, RBracket, "", false, func() *Node {
			if p.at(DotDot) {
				return p.restPattern()
			}
			return p.pattern()
		})
		return b.done(p)
	}
	p.errorExpected("a pattern")
	return p.missing(Ident)
}

// rangePattern parses the rest of `'a'..='z'` or `0..10` after its start, if
// a range follows. Bounds are integers, characters and negative numbers.
func (p *parser) rangePattern(start *Node) *Node {
	if !p.at(DotDot) && !p.at(DotDotEq) {
		return start
	}
	b := wrap(RangePattern, FieldStart, start)
	b.field(FieldOperator, p.bump())
	switch p.kind() {
	case Int, Char:
		b.field(FieldEnd, p.bump())
	case Minus:
		end := p.start(NegativeLiteral)
		end.add(p.bump())
		if p.at(Int) || p.at(Float) {
			end.add(p.bump())
		} else {
			p.errorExpected("a number")
		}
		b.field(FieldEnd, end.done(p))
	default:
		p.errorExpected("the end of the range")
	}
	return b.done(p)
}

func (p *parser) fieldPattern() *Node {
	b := p.start(NamedFieldPattern)
	b.field(FieldName, p.bump())
	b.add(p.bump())
	b.field(FieldPattern, p.pattern())
	return b.done(p)
}

func (p *parser) restPattern() *Node {
	b := p.start(RestPattern)
	b.add(p.bump())
	if p.at(Ident) {
		b.field(FieldName, p.bump())
	}
	return b.done(p)
}
