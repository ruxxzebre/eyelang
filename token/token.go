package token

import "fmt"

type TokenType string

type Token struct {
	Type     TokenType
	Literal  string
	Position struct {
		Row int
		Col int
	}
}

func (tok *Token) PositionString() string {
	return fmt.Sprintf("(row: %d, column: %d)", tok.Position.Row, tok.Position.Col)
}

func (tok *Token) UpdatePosition(row, col int) {
	tok.Position.Col = col
	tok.Position.Row = row
}

const (
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"

	// Identifiers + Literals
	IDENT  = "IDENT" // add, foobar, x, y
	INT    = "INT"
	STRING = "STRING"

	// Operators
	ASSIGN   = "="
	PLUS     = "+"
	MINUS    = "-"
	MOD      = "%"
	BANG     = "!"
	ASTERISK = "*"
	SLASH    = "/"
	LT       = "<"
	GT       = ">"
	EQ       = "=="
	NOT_EQ   = "!="
	// PIPE     = "|" // pipe operator for function composition
	// example of pipe operator: (let f(x) = x + 1 | g(x) = x * 2 | f(g(2)) = 6)???

	// Delimiters
	COMMA     = ","
	COLON     = ":"
	SEMICOLON = ";"

	LPAREN   = "("
	RPAREN   = ")"
	LBRACE   = "{"
	RBRACE   = "}"
	LBRACKET = "["
	RBRACKET = "]"

	// (currently) Scope Acess Operator
	DOT     = "."
	COMMENT = "#"

	// Keywords
	FUNCTION = "FUNCTION"
	LET      = "LET"
	IF       = "IF"
	RETURN   = "RETURN"
	TRUE     = "TRUE"
	FALSE    = "FALSE"
	ELSE     = "ELSE"
)

var keywords = map[string]TokenType{
	"fn":     FUNCTION,
	"let":    LET,
	"true":   TRUE,
	"false":  FALSE,
	"if":     IF,
	"else":   ELSE,
	"return": RETURN,
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
