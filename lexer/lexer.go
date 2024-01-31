package lexer

import (
	"eyelang/token"
	"unicode"
	// "unicode/utf8"
)

type Lexer struct {
	input        []rune
	position     int
	readPosition int
	char         rune
	textPosition struct {
		row int
		col int
	}
}

func New(input string) *Lexer {
	l := &Lexer{input: []rune(input)}
	l.textPosition.col = -1

	l.readChar()
	return l
}

func isDigit(char rune) bool {
	return unicode.IsDigit(char)
}

var emoticons = &unicode.RangeTable{
	R32: []unicode.Range32{
		{Lo: 0x1F600, Hi: 0x1F64F, Stride: 1},
	},
}

func isEmoji(char rune) bool {
	return unicode.Is(emoticons, char)
}

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.char = 0
	} else {
		l.char = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition++
	l.textPosition.col++
}

func (l *Lexer) readNumber() string {
	prevPos := l.position
	for isDigit(l.char) {
		l.readChar()
	}
	return string(l.input[prevPos:l.position])
}

func isSymbol(char rune) bool {
	return unicode.IsLetter(char) || isEmoji(char)
}

func (l *Lexer) readIdentifier() string {
	prevPos := l.position
	for isSymbol(l.char) {
		l.readChar()
	}
	return string(l.input[prevPos:l.position])
}

func newToken(tokenType token.TokenType, ch rune) token.Token {
	return token.Token{Type: tokenType, Literal: string(ch)}
}

func (l *Lexer) skipWhitespace() {
	for l.char == ' ' || l.char == '\t' || l.char == '\n' || l.char == '\r' {
		if l.char == '\n' {
			l.textPosition.row++
			l.textPosition.col = -1
		}
		l.readChar()
	}
}

func (l *Lexer) peekChar() rune {
	if l.readPosition >= len(l.input) {
		return 0
	} else {
		return l.input[l.readPosition]
	}
}

func (l *Lexer) NextToken() token.Token {
	var tok token.Token

	l.skipWhitespace()

	initialPosition := l.textPosition

	switch l.char {
	case '=':
		if l.peekChar() == '=' {
			char := l.char
			l.readChar()
			literal := string(char) + string(l.char)
			tok = token.Token{Type: token.EQ, Literal: literal}
		} else {
			tok = newToken(token.ASSIGN, l.char)
		}
	case ';':
		tok = newToken(token.SEMICOLON, l.char)
	case '+':
		tok = newToken(token.PLUS, l.char)
	case '-':
		tok = newToken(token.MINUS, l.char)
	case '<':
		tok = newToken(token.LT, l.char)
	case '>':
		tok = newToken(token.GT, l.char)
	case '/':
		tok = newToken(token.SLASH, l.char)
	case '*':
		tok = newToken(token.ASTERISK, l.char)
	case '!':
		if l.peekChar() == '=' {
			char := l.char
			l.readChar()
			literal := string(char) + string(l.char)
			tok = token.Token{Type: token.NOT_EQ, Literal: literal}
		} else {
			tok = newToken(token.BANG, l.char)
		}
	case ',':
		tok = newToken(token.COMMA, l.char)
	case '(':
		tok = newToken(token.LPAREN, l.char)
	case ')':
		tok = newToken(token.RPAREN, l.char)
	case '{':
		tok = newToken(token.LBRACE, l.char)
	case '}':
		tok = newToken(token.RBRACE, l.char)
	case '[':
		tok = newToken(token.LBRACKET, l.char)
	case ']':
		tok = newToken(token.RBRACKET, l.char)
	case ':':
		tok = newToken(token.COLON, l.char)
	case '"':
		tok.Type = token.STRING
		tok.Literal = l.readString()
	case 0:
		tok.Literal = ""
		tok.Type = token.EOF
	default:
		if isSymbol(l.char) {
			tok.Literal = l.readIdentifier()
			tok.Type = token.LookupIdent(tok.Literal)
			tok.UpdatePosition(initialPosition.row, initialPosition.col)
			return tok
		} else if isDigit(l.char) {
			tok.Literal = l.readNumber()
			tok.Type = token.INT
			tok.UpdatePosition(initialPosition.row, initialPosition.col)
			return tok
		} else {
			tok = newToken(token.ILLEGAL, l.char)
		}
	}

	tok.UpdatePosition(initialPosition.row, initialPosition.col)
	l.readChar()
	return tok
}

func (l *Lexer) readString() string {
	position := l.position + 1
	for {
		l.readChar()
		if l.char == '"' {
			break
		}
		if l.char == 0 {
			// TODO: throw error
			break
		}
	}
	return string(l.input[position:l.position])
}
