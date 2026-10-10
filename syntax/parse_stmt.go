package syntax

// block parses `{ statements }`.
func (p *parser) block() *Node {
	b := p.start(Block)
	if !p.at(LBrace) {
		p.errorExpected("'{'")
		b.add(p.missing(LBrace))
		return b.done(p)
	}
	b.add(p.bump())
	p.separated(b, RBrace, Semicolon, p.statement)
	b.add(p.expectClose(RBrace))
	return b.done(p)
}

func (p *parser) statement() *Node {
	switch p.kind() {
	case KwLet:
		return p.let()
	case KwVar:
		b := p.start(VarStatement)
		b.add(p.bump())
		b.field(FieldName, p.expect(Ident))
		if p.at(Colon) {
			b.add(p.bump())
			p.withNewlines(false, func() { b.field(FieldType, p.typeExpr()) })
		}
		b.add(p.expect(Assign))
		b.field(FieldValue, p.expr())
		return b.done(p)
	case KwFor:
		b := p.start(ForStatement)
		b.add(p.bump())
		b.field(FieldPattern, p.pattern())
		b.add(p.expect(KwIn))
		p.withNewlines(false, func() { b.field(FieldValue, p.expr()) })
		b.field(FieldBody, p.block())
		return b.done(p)
	case KwWhile:
		b := p.start(WhileStatement)
		b.add(p.bump())
		p.withNewlines(false, func() { b.field(FieldCondition, p.expr()) })
		b.field(FieldBody, p.block())
		return b.done(p)
	case KwWith:
		b := p.start(WithStatement)
		b.add(p.bump())
		b.field(FieldValue, p.expr())
		return b.done(p)
	case KwReturn:
		b := p.start(ReturnStatement)
		b.add(p.bump())
		if p.startsExpr() && !p.atBreak() {
			b.field(FieldValue, p.expr())
		}
		return b.done(p)
	case KwBreak:
		b := p.start(BreakStatement)
		b.add(p.bump())
		return b.done(p)
	case KwContinue:
		b := p.start(ContinueStatement)
		b.add(p.bump())
		return b.done(p)
	case Ident:
		if p.peek(1).Kind == Assign {
			b := p.start(Assignment)
			b.field(FieldLeft, p.bump())
			b.add(p.bump())
			b.field(FieldRight, p.expr())
			return b.done(p)
		}
	}
	if !p.startsExpr() {
		p.errorExpected("a statement")
		return errorNode(p.skipLineNodes())
	}
	return p.expr()
}

// let parses `let pattern: T = value else { … }`.
func (p *parser) let() *Node {
	b := p.start(LetStatement)
	b.add(p.bump())
	b.field(FieldPattern, p.pattern())
	if p.at(Colon) {
		b.add(p.bump())
		p.withNewlines(false, func() { b.field(FieldType, p.typeExpr()) })
	}
	b.add(p.expect(Assign))
	b.field(FieldValue, p.expr())
	if p.at(KwElse) {
		b.add(p.bump())
		b.field(FieldAlternative, p.block())
	}
	return b.done(p)
}
