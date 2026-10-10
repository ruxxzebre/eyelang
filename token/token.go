package token

import (
	"encoding/json"
	"fmt"
)

type TokenType string
type TokenCategory string

type Token struct {
	Type     TokenType `json:"type"`
	Literal  string    `json:"literal"`
	Position struct {
		Row int `json:"row"`
		Col int `json:"col"`
	} `json:"position"`
}

func (tok *Token) MarshalJSON() ([]byte, error) {
	return json.Marshal(tok.Type)
}

func (tok *Token) PositionString() string {
	return fmt.Sprintf("(%d:%d)", tok.Position.Row, tok.Position.Col)
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
	FLOAT  = "FLOAT"
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
	GT_EQ    = ">="
	LT_EQ    = "<="
	NOT_EQ   = "!="
	// AND      = "and" // TODO gt eq and stuff
	OR   = "||"
	AND  = "&&"
	PIPE = "|>"
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
