package parser

import (
	"eyelang/ast"
	"eyelang/lexer"
	"eyelang/token"
	"fmt"
	"strconv"
)

// Precedences
const (
	LOWEST = iota
	EQUALS
	LESSGREATER
	// binds looser than arithmetic and tighter than comparison:
	// `a + b |> f` is `(a + b) |> f`, `x |> f == y` is `(x |> f) == y`
	PIPE
	SUM
	PRODUCT
	PREFIX
	CALL
	INDEX
)

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

// TODO
type ParsingError struct {
	// Type string
	// Trace string
	Message string
	Token   token.Token
}

func (pe *ParsingError) String() string {
	return fmt.Sprintf("%s %s", pe.Message, pe.Token.PositionString())
}

type Parser struct {
	l         *lexer.Lexer
	curToken  token.Token
	peekToken token.Token
	errors    []ParsingError

	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn
}

var precedences = map[token.TokenType]int{
	token.EQ:       EQUALS,
	token.NOT_EQ:   EQUALS,
	token.LT:       LESSGREATER,
	token.LT_EQ:    LESSGREATER,
	token.GT:       LESSGREATER,
	token.GT_EQ:    LESSGREATER,
	token.PLUS:     SUM,
	token.MINUS:    SUM,
	token.MOD:      PRODUCT,
	token.SLASH:    PRODUCT,
	token.ASTERISK: PRODUCT,
	token.LPAREN:   CALL,
	token.PIPE:     PIPE,
	token.LBRACKET: INDEX,
	token.DOT:      INDEX,
}

type prefixParserTable = []struct {
	tokenType token.TokenType
	fn        prefixParseFn
}

type infixParserTable = []struct {
	tokenType token.TokenType
	fn        infixParseFn
}

func New(l *lexer.Lexer) *Parser {
	// t := tracer.New()
	p := &Parser{l: l, errors: []ParsingError{}}

	p.prefixParseFns = make(map[token.TokenType]prefixParseFn)
	p.infixParseFns = make(map[token.TokenType]infixParseFn)
	p.registerPrefixes(prefixParserTable{
		{token.FUNCTION, p.parseFunctionLiteral},
		{token.IDENT, p.parseIdentifier},
		{token.INT, p.parseIntegerLiteral},
		{token.FLOAT, p.parseFloatLiteral},
		{token.IF, p.parseIfExpression},
		{token.LPAREN, p.parseGroupedExpression},
		{token.TRUE, p.parseBooleanExpression},
		{token.FALSE, p.parseBooleanExpression},
		{token.BANG, p.parsePrefixExpression},
		{token.MINUS, p.parsePrefixExpression},
		{token.STRING, p.parseStringLiteral},
		{token.LBRACKET, p.parseArrayLiteral},
		{token.LBRACE, p.parseHashLiteral},
	})
	p.registerInfixes(infixParserTable{
		{token.LPAREN, p.parseCallExpression},
		{token.ASTERISK, p.parseInfixExpression},
		{token.EQ, p.parseInfixExpression},
		{token.NOT_EQ, p.parseInfixExpression},
		{token.GT_EQ, p.parseInfixExpression},
		{token.LT_EQ, p.parseInfixExpression},
		{token.LT, p.parseInfixExpression},
		{token.GT, p.parseInfixExpression},
		{token.PLUS, p.parseInfixExpression},
		{token.MOD, p.parseInfixExpression},
		{token.MINUS, p.parseInfixExpression},
		{token.SLASH, p.parseInfixExpression},
		{token.LBRACKET, p.parseIndexExpression},
		{token.DOT, p.parseInfixExpression},
		{token.PIPE, p.parseInfixExpression},
	})

	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) Errors() []ParsingError {
	return p.errors
}

func (p *Parser) newError(format string, a ...interface{}) {
	p.errors = append(p.errors, ParsingError{
		Message: fmt.Sprintf(format, a...),
		Token:   p.curToken,
	})
}

func (p *Parser) replaceLastError(err ParsingError) {
	p.errors[len(p.errors)-1] = err
}

func (p *Parser) peekError(t token.TokenType) {
	msg := fmt.Sprintf("expected next token to be '%s', got='%s' instead. %s",
		t,
		p.peekToken.Type,
		p.peekToken.PositionString(),
	)
	p.errors = append(p.errors, ParsingError{
		Message: msg,
		Token:   p.peekToken,
	})
}

func (p *Parser) registerPrefixes(prefixTable prefixParserTable) {
	for _, prefix := range prefixTable {
		p.prefixParseFns[prefix.tokenType] = prefix.fn
	}
}

func (p *Parser) registerInfixes(infixTable infixParserTable) {
	for _, prefix := range infixTable {
		p.infixParseFns[prefix.tokenType] = prefix.fn
	}
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) parseLetStatement() *ast.LetStatement {
	defer untrace(trace("parseLetStatement"))

	stmt := &ast.LetStatement{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// expect peek advanced token too
	// TODO: make function names more explicit
	if !p.expectPeek(token.ASSIGN) {
		return nil
	}

	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
	// for !p.curTokenIs(token.SEMICOLON) {
	// 	p.nextToken()
	// }
}

func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	defer untrace(trace("parseReturnStatement"))

	stmt := &ast.ReturnStatement{Token: p.curToken}

	p.nextToken()

	stmt.ReturnValue = p.parseExpression(LOWEST)
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

// parse params like list
func (p *Parser) parseShorthandFunctionLiteral(lit *ast.FunctionLiteral) ast.Expression {
	params := []*ast.Identifier{}

	p.nextToken()

	if p.curTokenIs(token.LPAREN) {
		params = p.parseIdentifierList(token.RPAREN)
		if params == nil {
			p.newError("expected list of expressions")
			return nil
		}
		for _, param := range params {
			if param == nil {
				p.newError("expected list of expressions")
				return nil
			}
		}
		p.nextToken()
	} else {
		for !p.curTokenIs(token.COLON) && !p.curTokenIs(token.EOF) {
			if !p.curTokenIs(token.IDENT) {
				p.newError("expected identifier as function parameter. got='%s' %s", p.curToken.Literal, p.curToken.PositionString())
				return nil
			}
			param := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
			params = append(params, param)
			p.nextToken()
			if p.curTokenIs(token.COMMA) {
				p.nextToken()
			}
		}
	}

	if !p.curTokenIs(token.COLON) && !p.curTokenIs(token.LBRACE) {
		p.newError("expected colon after shorthand function literal parameters")
		return nil
	}

	seen := map[string]bool{}
	for _, param := range params {
		if seen[param.Value] {
			p.newError("duplicate function parameter '%s' %s", param.Value, param.Token.PositionString())
			return nil
		}
		seen[param.Value] = true
	}

	lit.Parameters = params

	switch {
	case p.curTokenIs(token.LBRACE):
		lit.Body = p.parseBlockStatement()
	case p.curTokenIs(token.COLON):
		p.nextToken()

		var expr ast.Expression
		if p.curTokenIs(token.LBRACE) {
			lit.Body = p.parseBlockStatement()
			return lit
		} else if p.curToken.Literal == "@" && p.peekTokenIs(token.LBRACE) {
			p.nextToken()
			expr = p.parseHashLiteral()
		} else {
			expr = p.parseExpression(LOWEST)
		}
		// TODO: eww that's ugly
		lit.Body = &ast.BlockStatement{
			Token: p.curToken,
			Statements: []ast.Statement{
				&ast.ExpressionStatement{
					Token:      p.curToken,
					Expression: expr,
				},
			},
		}
	}

	// p.nextToken()
	return lit
}

func (p *Parser) parseFunctionLiteral() ast.Expression {
	lit := &ast.FunctionLiteral{
		Token: p.curToken,
	}

	return p.parseShorthandFunctionLiteral(lit)

	// TODO: preserve expectPeek
	// if !p.peekTokenIs(token.LPAREN) {
	// }
	// p.nextToken()
	//
	// lit.Parameters = p.parseFunctionParams()
	//
	// if !p.expectPeek(token.LBRACE) {
	// 	return nil
	// }
	//
	// lit.Body = p.parseBlockStatement()
	//
	// return lit
}

func (p *Parser) parseIfExpression() ast.Expression {
	ifexpr := &ast.IfExpression{
		Token: p.curToken,
	}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	p.nextToken()

	ifexpr.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	ifexpr.Consequence = p.parseBlockStatement()

	if p.peekTokenIs(token.ELSE) {
		p.nextToken()

		if !p.expectPeek(token.LBRACE) {
			return nil
		}

		ifexpr.Alternative = p.parseBlockStatement()
	}

	return ifexpr
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{
		Token: p.curToken,
	}
	block.Statements = []ast.Statement{}

	p.nextToken()

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}

		p.nextToken()
	}

	if p.curTokenIs(token.EOF) {
		p.newError("expected '}' to close block opened at %s", block.Token.PositionString())
	}

	return block
}

func (p *Parser) parseIdentifier() ast.Expression {
	defer untrace(trace("parseIdentifier > " + p.curToken.Literal))

	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseGroupedExpression() ast.Expression {
	defer untrace(trace("parseGroupedExpression"))

	// Our current token is '('
	p.nextToken()

	// passing LOWEST to BOOST precedence of enclosed expression
	// '(' is really sticky
	exp := p.parseExpression(LOWEST)

	// so after that peek token should be ')'
	if !p.expectPeek(token.RPAREN) {
		msg := fmt.Sprintf("expected closing grouped expression. got='%s' instead %s", p.peekToken.Literal, p.peekToken.PositionString())
		p.replaceLastError(ParsingError{
			Message: msg,
			Token:   p.peekToken,
		})
		return nil
	}

	return exp
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	defer untrace(trace("parseIntegerLiteral"))

	lit := &ast.FloatLiteral{Token: p.curToken}

	value, err := strconv.ParseFloat(p.curToken.Literal, 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as float %s", p.curToken.Literal, p.curToken.PositionString())
		p.errors = append(p.errors, ParsingError{
			Message: msg,
			Token:   p.curToken,
		})
		return nil
	}

	lit.Value = value
	return lit
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	defer untrace(trace("parseIntegerLiteral"))

	lit := &ast.IntegerLiteral{Token: p.curToken}

	value, err := strconv.ParseInt(p.curToken.Literal, 0, 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as integer %s", p.curToken.Literal, p.curToken.PositionString())
		p.errors = append(p.errors, ParsingError{
			Message: msg,
			Token:   p.curToken,
		})
		return nil
	}

	lit.Value = value
	return lit
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	defer untrace(trace("parseExpression > " + p.curToken.Literal))

	prefix, ok := p.prefixParseFns[p.curToken.Type]
	if !ok {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	leftExp := prefix()

	for !p.peekTokenIs(token.SEMICOLON) && precedence < p.peekPrecedence() {
		infix, ok := p.infixParseFns[p.peekToken.Type]
		if !ok {
			return leftExp
		}
		p.nextToken()
		leftExp = infix(leftExp)
	}

	return leftExp
}

func (p *Parser) parseBooleanExpression() ast.Expression {
	defer untrace(trace("parseBooleanExpression"))
	return &ast.Boolean{Token: p.curToken, Value: p.curTokenIs(token.TRUE)}
}

func (p *Parser) parseStringLiteral() ast.Expression {
	defer untrace(trace("parseStringLiteral"))
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	defer untrace(trace("parsePrefixExpression"))

	expression := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}

	p.nextToken()
	expression.Right = p.parseExpression(PREFIX)

	return expression
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	defer untrace(trace("parseInfixExpression"))

	expression := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}
	precedence := p.curPrecedence()
	p.nextToken()
	expression.Right = p.parseExpression(precedence)

	return expression
}

func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	defer untrace(trace("parseCallExpression"))

	exp := &ast.CallExpression{Token: p.curToken, Function: function}
	exp.Arguments = p.parseExpressionList(token.RPAREN)
	return exp
}

func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {
	defer untrace(trace("parseExpressionStatement " + p.curToken.Literal))

	stmt := &ast.ExpressionStatement{Token: p.curToken}
	stmt.Expression = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) noPrefixParseFnError(t token.TokenType) {
	msg := fmt.Sprintf("no prefix parse function for %s found %s", t, p.curToken.PositionString())
	if t == token.ILLEGAL {
		msg = fmt.Sprintf("illegal token '%s' %s", p.curToken.Literal, p.curToken.PositionString())
	}
	p.errors = append(p.errors, ParsingError{
		Message: msg,
		Token:   p.curToken,
	})
}

func (p *Parser) parseStatement() ast.Statement {
	defer untrace(trace("parseStatement"))

	switch p.curToken.Type {
	case token.LET:
		return p.parseLetStatement()
	case token.RETURN:
		return p.parseReturnStatement()
	default:
		return p.parseExpressionStatement()
	}
}

func (p *Parser) ParseProgram() *ast.Program {
	defer untrace(trace("ParseProgram"))

	program := &ast.Program{}
	program.Statements = []ast.Statement{}

	for !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}

	return program
}

func (p *Parser) parseIdentifierList(end token.TokenType) []*ast.Identifier {
	expressions := p.parseExpressionList(end)
	identifiers := []*ast.Identifier{}
	// identifiers := make([]*ast.Identifier, len(expressions))
	for _, exp := range expressions {
		ident, ok := exp.(*ast.Identifier)
		if !ok {
			p.newError("expected identifier")
			return nil
		}
		identifiers = append(identifiers, ident)
	}
	return identifiers
}

func (p *Parser) parseExpressionList(end token.TokenType) []ast.Expression {
	list := []ast.Expression{}

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken()
	list = append(list, p.parseExpression(LOWEST))

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		list = append(list, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(end) {
		// TODO: error
		return nil
	}

	return list
}

func (p *Parser) parseArrayLiteral() ast.Expression {
	defer untrace(trace("parsing array literal " + p.curToken.PositionString()))
	array := &ast.ArrayLiteral{Token: p.curToken}
	array.Elements = p.parseExpressionList(token.RBRACKET)
	return array
}

func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	defer untrace(trace("parsing index expression"))

	ie := &ast.IndexExpression{Token: p.curToken, Left: left}
	p.nextToken()
	ie.Index = p.parseExpression(LOWEST)

	if !p.expectPeek(token.RBRACKET) {
		// TODO: error
		return nil
	}

	return ie
}

func (p *Parser) parseHashLiteral() ast.Expression {
	hash := &ast.HashLiteral{Token: p.curToken}
	hash.Pairs = map[ast.Expression]ast.Expression{}

	for !p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		key := p.parseExpression(LOWEST)

		if !p.expectPeek(token.COLON) {
			return nil
		}

		p.nextToken()
		value := p.parseExpression(LOWEST)

		hash.Pairs[key] = value

		if !p.peekTokenIs(token.RBRACE) && !p.expectPeek(token.COMMA) {
			// TODO parsing error
			p.newError("expected %s or %s. got=%s", token.RBRACE, token.COMMA, p.curToken.Type)
			return nil
		}
	}

	if !p.expectPeek(token.RBRACE) {
		// TODO parsing error
		return nil
	}

	return hash
}
