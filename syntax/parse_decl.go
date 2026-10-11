package syntax

// sourceFile parses the whole file: items separated by line breaks or `;`.
func (p *parser) sourceFile() *Node {
	b := p.start(SourceFile)
	for {
		p.separated(b, EOF, Semicolon, p.item)
		if p.at(EOF) {
			break
		}
		// A closing delimiter with nothing open.
		p.errorExpected("a declaration")
		b.add(errorNode([]*Node{p.bump()}))
	}
	b.add(p.bump()) // EOF, which holds the trailing trivia
	return b.done(p)
}

// item parses a top-level declaration with its attributes and `pub`.
func (p *parser) item() *Node {
	attrs := p.attributes()
	var pub, opaque *Node
	if p.at(KwPub) {
		pub = p.bump()
		if p.atWord("opaque") {
			opaque = p.bumpAs(Keyword)
		}
	}
	word := ""
	if p.at(Ident) {
		word = p.text(p.tok())
	}
	if opaque != nil && word != "type" {
		p.errorExpected("'type'")
		return errorNode(nonNil(append(attrs, pub, opaque)))
	}
	switch {
	case p.at(KwFn):
		return p.function(attrs, pub, false)
	case word == "extern":
		return p.extern(attrs, pub)
	case word == "job":
		return p.job(attrs, pub)
	case word == "entity":
		return p.entity(attrs, pub)
	case word == "view":
		return p.view(attrs, pub)
	case word == "type":
		return p.typeDeclaration(attrs, pub, opaque)
	case word == "trait":
		return p.trait(attrs, pub)
	case word == "contract":
		return p.contract(attrs, pub)
	case word == "impl", word == "test":
		if pub != nil {
			p.errorAt(pub.Span, "%s declarations can't be pub", word)
			attrs = append(attrs, errorNode([]*Node{pub}))
		}
		if word == "impl" {
			return p.impl(attrs)
		}
		return p.test(attrs)
	case p.at(KwLet) && len(attrs) == 0:
		return p.let(pub) // `pub let` exports a module-level value
	case word == "import", p.at(KwLet):
		if len(attrs) > 0 || pub != nil {
			first := pub
			if len(attrs) > 0 {
				first = attrs[0]
			}
			p.errorAt(first.Span, "%s can't have attributes or pub", p.text(p.tok()))
			attrs = append(attrs, pub)
			return errorNode(nonNil(append(attrs, p.itemWithoutPrefix())))
		}
		return p.itemWithoutPrefix()
	}
	p.errorExpected("a declaration")
	skipped := append(nonNil(append(attrs, pub)), p.skipLineNodes()...)
	return errorNode(skipped)
}

func (p *parser) itemWithoutPrefix() *Node {
	if p.atWord("import") {
		return p.importDeclaration()
	}
	return p.let(nil)
}

func nonNil(nodes []*Node) []*Node {
	out := nodes[:0:0]
	for _, n := range nodes {
		if n != nil {
			out = append(out, n)
		}
	}
	return out
}

// skipLineNodes is skipLine without the ERROR wrapper.
func (p *parser) skipLineNodes() []*Node {
	if n := p.skipLine(); n != nil {
		return n.Children
	}
	return nil
}

// attributes parses `@name` and `@name(args)` attributes.
func (p *parser) attributes() []*Node {
	var attrs []*Node
	for p.at(At) {
		b := p.start(Attribute)
		b.add(p.bump())
		b.field(FieldName, p.expect(Ident))
		if p.at(LParen) {
			b.field(FieldArguments, p.arguments())
		}
		attrs = append(attrs, b.done(p))
	}
	return attrs
}

// declaration starts a declaration node with its attributes and `pub`.
func (p *parser) declaration(k Kind, attrs []*Node, pub *Node) *builder {
	b := p.start(k)
	b.add(attrs...)
	b.add(pub)
	return b
}

func (p *parser) importDeclaration() *Node {
	b := p.start(ImportDeclaration)
	b.add(p.bumpAs(Keyword))
	path := p.start(ImportPath)
	path.add(p.expect(Ident))
	for p.at(Dot) {
		path.add(p.bump(), p.expect(Ident))
	}
	b.field(FieldPath, path.done(p))
	if p.at(DotBrace) {
		b.add(p.bump())
		n := p.commaList(b, RBrace, func() *Node { return withField(FieldItem, p.name()) })
		if n == 0 {
			p.errorExpected("a name")
		}
		b.add(p.expectClose(RBrace))
	}
	return b.done(p)
}

func withField(f Field, n *Node) *Node {
	if n != nil {
		n.Field = f
	}
	return n
}

// name parses an identifier or a type name.
func (p *parser) name() *Node {
	if p.at(Ident) || p.at(TypeIdent) {
		return p.bump()
	}
	p.errorExpected("a name")
	return p.missing(Ident)
}

// function parses `fn name(params) -> T uses {…} body`. In a trait the body is
// optional, which makes the declaration a function_signature.
func (p *parser) function(attrs []*Node, pub *Node, inTrait bool) *Node {
	b := p.declaration(FunctionDeclaration, attrs, pub)
	b.add(p.bump())
	b.field(FieldName, p.expect(Ident))
	b.field(FieldParameters, p.parameters())

	// Outside a trait the body is required, so a line break can't end the
	// declaration early. In a trait it ends a signature.
	p.withNewlines(inTrait, func() {
		if p.at(Arrow) {
			b.add(p.bump())
			b.field(FieldReturnType, p.typeExpr())
		}
		if p.atWord("uses") && !p.atBreak() {
			b.field(FieldEffects, p.usesClause())
		}
		if p.atWord("where") && !p.atBreak() {
			b.field(FieldRequirements, p.whereClause())
		}
		switch {
		case p.at(Assign):
			b.add(p.bump())
			p.withNewlines(true, func() { b.field(FieldBody, p.expr()) })
		case p.at(LBrace) && !p.atBreak():
			b.field(FieldBody, p.block())
		case inTrait:
			b.kind = FunctionSignature
			if pub != nil {
				p.errorAt(pub.Span, "a function without a body can't be pub")
			}
		default:
			p.errorExpected("'=' or '{'")
		}
	})
	return b.done(p)
}

// extern parses `extern fn name(params) -> T uses {…} where …`, a function
// implemented by C or by the Go program that embeds Eyelang, and `extern type
// Name(params)`, a type whose values foreign code owns.
func (p *parser) extern(attrs []*Node, pub *Node) *Node {
	if t := p.peek(1); (t.Kind == Ident || t.Kind == TypeIdent) && p.text(t) == "type" {
		b := p.declaration(ExternTypeDeclaration, attrs, pub)
		b.add(p.bumpAs(Keyword))
		b.add(p.bumpAs(Keyword))
		b.field(FieldName, p.expect(TypeIdent))
		if p.at(LParen) {
			b.field(FieldTypeParameters, p.typeParameters())
		}
		return b.done(p)
	}
	b := p.declaration(ExternDeclaration, attrs, pub)
	b.add(p.bumpAs(Keyword))
	b.add(p.expect(KwFn))
	b.field(FieldName, p.expect(Ident))
	b.field(FieldParameters, p.parameters())
	if p.at(Arrow) {
		b.add(p.bump())
		b.field(FieldReturnType, p.typeExpr())
	}
	if p.atWord("uses") && !p.atBreak() {
		b.field(FieldEffects, p.usesClause())
	}
	if p.atWord("where") && !p.atBreak() {
		b.field(FieldRequirements, p.whereClause())
	}
	return b.done(p)
}

// whereClause parses `where a: Ord + Show, b: Hash`.
func (p *parser) whereClause() *Node {
	b := p.start(WhereClause)
	b.add(p.bumpAs(Keyword))
	for {
		r := p.start(Requirement)
		r.field(FieldName, p.expect(Ident))
		r.add(p.expect(Colon))
		r.field(FieldTrait, p.typeExpr())
		for p.at(Plus) {
			r.add(p.bump())
			r.field(FieldTrait, p.typeExpr())
		}
		b.add(r.done(p))
		if !p.at(Comma) {
			return b.done(p)
		}
		b.add(p.bump())
	}
}

func (p *parser) parameters() *Node {
	b := p.start(Parameters)
	p.delimited(b, LParen, RParen, "", false, p.parameter)
	return b.done(p)
}

func (p *parser) parameter() *Node {
	b := p.start(Parameter)
	if p.at(KwMut) {
		b.add(p.bump())
	}
	switch {
	case p.at(Underscore):
		w := p.start(Wildcard)
		w.add(p.bump())
		b.field(FieldName, w.done(p))
	case p.at(KwIn), p.at(KwFor):
		// The labels built-ins take, `in:` and `for:`; only an extern fn
		// may use them, which the checker sees to.
		b.field(FieldName, p.bumpAs(Ident))
	default:
		b.field(FieldName, p.expect(Ident))
	}
	if p.at(Colon) {
		b.add(p.bump())
		b.field(FieldType, p.typeExpr())
	}
	if p.at(Assign) {
		b.add(p.bump())
		b.field(FieldDefault, p.expr())
	}
	return b.done(p)
}

// usesClause parses `uses {wait, net Llm.complete, entity World}`.
func (p *parser) usesClause() *Node {
	b := p.start(UsesClause)
	b.add(p.bumpAs(Keyword))
	b.add(p.expect(LBrace))
	p.commaList(b, RBrace, p.effect)
	b.add(p.expectClose(RBrace))
	return b.done(p)
}

func (p *parser) effect() *Node {
	b := p.start(Effect)
	if !p.at(Ident) {
		p.errorExpected("an effect")
		return b.done(p)
	}
	b.field(FieldKind, p.bump())
	if p.at(Ident) || p.at(TypeIdent) {
		path := p.start(Path)
		path.add(p.bump())
		for p.at(Dot) {
			path.add(p.bump(), p.name())
		}
		b.field(FieldTarget, path.done(p))
	}
	return b.done(p)
}

func (p *parser) job(attrs []*Node, pub *Node) *Node {
	b := p.declaration(JobDeclaration, attrs, pub)
	b.add(p.bumpAs(Keyword))
	b.field(FieldName, p.expect(Ident))
	b.field(FieldParameters, p.parameters())
	p.withNewlines(false, func() {
		if p.at(Arrow) {
			b.add(p.bump())
			b.field(FieldReturnType, p.typeExpr())
		}
		if p.atWord("uses") {
			b.field(FieldEffects, p.usesClause())
		}
		b.field(FieldBody, p.block())
	})
	return b.done(p)
}

func (p *parser) entity(attrs []*Node, pub *Node) *Node {
	b := p.declaration(EntityDeclaration, attrs, pub)
	b.add(p.bumpAs(Keyword))
	b.field(FieldName, p.expect(TypeIdent))
	b.field(FieldParameters, p.parameters())
	body := p.start(EntityBody)
	body.add(p.expect(LBrace))
	p.separated(body, RBrace, Semicolon, p.entityMember)
	body.add(p.expectClose(RBrace))
	b.field(FieldBody, body.done(p))
	return b.done(p)
}

func (p *parser) entityMember() *Node {
	attrs := p.attributes()
	switch {
	case p.atWord("state"):
		b := p.declaration(StateDeclaration, attrs, nil)
		b.add(p.bumpAs(Keyword))
		b.field(FieldName, p.expect(Ident))
		b.add(p.expect(Colon))
		b.field(FieldType, p.typeExpr())
		if p.at(Assign) {
			b.add(p.bump())
			b.field(FieldDefault, p.expr())
		}
		return b.done(p)
	case p.at(KwFn), p.at(KwPub):
		var pub *Node
		if p.at(KwPub) {
			pub = p.bump()
		}
		if !p.at(KwFn) {
			p.errorExpected("'fn'")
			return errorNode(nonNil(append(attrs, pub)))
		}
		return p.function(attrs, pub, false)
	case p.atWord("migrate"):
		b := p.declaration(Migration, attrs, nil)
		b.add(p.bumpAs(Keyword))
		b.add(p.expectWord("from"))
		b.field(FieldVersion, p.expect(Int))
		b.add(p.expect(LParen))
		b.field(FieldParameter, p.expect(Ident))
		b.add(p.expectClose(RParen))
		b.field(FieldBody, p.block())
		return b.done(p)
	}
	p.errorExpected("'state', 'fn' or 'migrate'")
	return errorNode(append(attrs, p.skipLineNodes()...))
}

func (p *parser) view(attrs []*Node, pub *Node) *Node {
	b := p.declaration(ViewDeclaration, attrs, pub)
	b.add(p.bumpAs(Keyword))
	b.field(FieldName, p.expect(Ident))
	b.add(p.expect(Assign))
	b.field(FieldValue, p.expr())
	return b.done(p)
}

// typeDeclaration parses `type Name(T) = …`. The value is a variant list if it
// has a `|`, and otherwise a type: `type T = A(Int)` is an alias of a generic
// type, and `type T = | A(Int)` declares one variant.
func (p *parser) typeDeclaration(attrs []*Node, pub, opaque *Node) *Node {
	b := p.declaration(TypeDeclaration, attrs, pub)
	b.add(opaque)
	b.add(p.bumpAs(Keyword))
	b.field(FieldName, p.expect(TypeIdent))
	if p.at(LParen) {
		b.field(FieldTypeParameters, p.typeParameters())
	}
	b.add(p.expect(Assign))

	switch {
	case p.at(Pipe):
		b.field(FieldValue, p.variantList(nil))
	case p.at(TypeIdent):
		// Read `Name` or `Name(…)` as a variant, then decide.
		first := p.variant()
		if p.at(Pipe) {
			b.field(FieldValue, p.variantList(first))
		} else {
			b.field(FieldValue, p.variantAsType(first))
		}
	default:
		b.field(FieldValue, p.typeExpr())
	}
	return b.done(p)
}

// typeParameters parses `(a, b)`.
func (p *parser) typeParameters() *Node {
	params := p.start(TypeParameters)
	p.delimited(params, LParen, RParen, "a type parameter", true, func() *Node { return p.expect(Ident) })
	return params.done(p)
}

// variantList parses `A | B(Int) | C(x: Int)`, starting after first if it
// was already parsed.
func (p *parser) variantList(first *Node) *Node {
	b := p.start(VariantList)
	if first == nil {
		b.add(p.bump()) // the leading |
		b.add(p.variant())
	} else {
		b.add(first)
	}
	for p.at(Pipe) {
		b.add(p.bump())
		b.add(p.variant())
	}
	return b.done(p)
}

func (p *parser) variant() *Node {
	b := p.start(Variant)
	b.field(FieldName, p.expect(TypeIdent))
	if p.at(LParen) && !p.atBreak() {
		payload := p.start(VariantPayload)
		p.delimited(payload, LParen, RParen, "a field or type", true, func() *Node {
			if p.at(Ident) && p.peek(1).Kind == Colon {
				return p.fieldDeclaration()
			}
			return p.typeExpr()
		})
		b.field(FieldPayload, payload.done(p))
	}
	return b.done(p)
}

// variantAsType turns a lone variant back into the type it spells:
// `Name` is a type_identifier and `Name(A, B)` a generic_type.
func (p *parser) variantAsType(v *Node) *Node {
	name := v.ChildByField(FieldName)
	payload := v.ChildByField(FieldPayload)
	if payload == nil {
		name.Field = NoField
		return name
	}
	for _, c := range payload.Children {
		if c.Kind == FieldDeclaration {
			p.errorAt(c.Span, "a type argument can't have a name; to declare a variant with named fields, start the list with '|'")
			break
		}
	}
	payload.Kind = TypeArguments
	payload.Field = FieldArguments
	name.Field = FieldName
	return newNode(GenericType, []*Node{name, payload}, name.Span.Start)
}

func (p *parser) fieldDeclaration() *Node {
	b := p.start(FieldDeclaration)
	b.field(FieldName, p.expect(Ident))
	b.add(p.expect(Colon))
	b.field(FieldType, p.typeExpr())
	return b.done(p)
}

func (p *parser) trait(attrs []*Node, pub *Node) *Node {
	b := p.declaration(TraitDeclaration, attrs, pub)
	b.add(p.bumpAs(Keyword))
	b.field(FieldName, p.expect(TypeIdent))
	if p.at(LParen) {
		b.field(FieldTypeParameters, p.typeParameters())
	}
	body := p.start(TraitBody)
	body.add(p.expect(LBrace))
	p.separated(body, RBrace, Semicolon, func() *Node { return p.member(true) })
	body.add(p.expectClose(RBrace))
	b.field(FieldBody, body.done(p))
	return b.done(p)
}

// member parses a function in a trait or impl body.
func (p *parser) member(inTrait bool) *Node {
	attrs := p.attributes()
	var pub *Node
	if p.at(KwPub) {
		pub = p.bump()
	}
	if p.at(KwFn) {
		return p.function(attrs, pub, inTrait)
	}
	p.errorExpected("'fn'")
	return errorNode(append(nonNil(append(attrs, pub)), p.skipLineNodes()...))
}

// impl parses `impl Trait for Type { … }` and `impl Type { … }`.
func (p *parser) impl(attrs []*Node) *Node {
	b := p.declaration(ImplDeclaration, attrs, nil)
	b.add(p.bumpAs(Keyword))
	p.withNewlines(false, func() {
		first := p.typeExpr()
		if p.at(KwFor) {
			b.field(FieldTrait, first)
			b.add(p.bump())
			b.field(FieldType, p.typeExpr())
		} else {
			b.field(FieldType, first)
		}
	})
	body := p.start(ImplBody)
	body.add(p.expect(LBrace))
	p.separated(body, RBrace, Semicolon, func() *Node { return p.member(false) })
	body.add(p.expectClose(RBrace))
	b.field(FieldBody, body.done(p))
	return b.done(p)
}

func (p *parser) contract(attrs []*Node, pub *Node) *Node {
	b := p.declaration(ContractDeclaration, attrs, pub)
	b.add(p.bumpAs(Keyword))
	b.field(FieldName, p.expect(TypeIdent))
	body := p.start(ContractBody)
	body.add(p.expect(LBrace))
	p.separated(body, RBrace, Semicolon, p.contractMember)
	body.add(p.expectClose(RBrace))
	b.field(FieldBody, body.done(p))
	return b.done(p)
}

func (p *parser) contractMember() *Node {
	attrs := p.attributes()
	switch {
	case p.atWord("command"), p.atWord("query"):
		kind := CommandDeclaration
		if p.atWord("query") {
			kind = QueryDeclaration
		}
		b := p.declaration(kind, attrs, nil)
		b.add(p.bumpAs(Keyword))
		b.field(FieldName, p.expect(Ident))
		b.field(FieldParameters, p.parameters())
		b.add(p.expect(Arrow))
		b.field(FieldReturnType, p.typeExpr())
		return b.done(p)
	case p.atWord("event"):
		b := p.declaration(EventDeclaration, attrs, nil)
		b.add(p.bumpAs(Keyword))
		b.field(FieldName, p.expect(Ident))
		b.add(p.expect(Colon))
		p.withNewlines(false, func() { b.field(FieldType, p.typeExpr()) })
		b.add(p.expectWord("on"))
		b.field(FieldTopic, p.stringLiteral())
		return b.done(p)
	}
	p.errorExpected("'command', 'query' or 'event'")
	return errorNode(append(attrs, p.skipLineNodes()...))
}

func (p *parser) test(attrs []*Node) *Node {
	b := p.declaration(TestDeclaration, attrs, nil)
	b.add(p.bumpAs(Keyword))
	b.field(FieldName, p.stringLiteral())
	b.field(FieldBody, p.block())
	return b.done(p)
}
