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
	if p.atWord("with") {
		// A contextual keyword is a keyword wherever one is allowed, so a
		// statement starting with `with` is always a with statement.
		b := p.start(WithStatement)
		b.add(p.bumpAs(Keyword))
		b.field(FieldValue, p.expr())
		return b.done(p)
	}
	switch p.kind() {
	case KwLet:
		return p.let(nil)
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
	}
	if !p.startsExpr() {
		p.errorExpected("a statement")
		return errorNode(p.skipLineNodes())
	}
	return p.assignmentOr(p.expr())
}

// branch parses what a match arm or a `then` or `else` branch holds when it
// isn't a block: an expression, or one assignment (`Ok(t) => pages += [t]`).
func (p *parser) branch() *Node { return p.assignmentOr(p.expr()) }

// assignmentOr returns the assignment that e starts, or e. The checker makes
// sure the left side is a variable or a field of one.
func (p *parser) assignmentOr(e *Node) *Node {
	switch p.kind() {
	case Assign, PlusEq, MinusEq, StarEq, SlashEq, PercentEq:
		b := wrap(Assignment, FieldLeft, e)
		b.field(FieldOperator, p.bump())
		b.field(FieldRight, p.expr())
		return b.done(p)
	}
	return e
}

// let parses `let pattern: T = value else { … }`, after `pub` at the top
// level.
func (p *parser) let(pub *Node) *Node {
	b := p.start(LetStatement)
	if pub != nil {
		b.add(pub)
	}
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
