package lexer

import (
	"bufio"
	"bytes"
	"eyelang/token"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
	// "unicode/utf8"
)

type Lexer struct {
	// input        []rune
	reader       *bufio.Reader
	position     int
	readPosition int
	char         rune
	textPosition struct {
		row int
		col int
	}
}

func New(input string) *Lexer {
	reader := strings.NewReader(input)
	runeReader := bufio.NewReader(reader)
	l := &Lexer{reader: runeReader}
	l.textPosition.col = -1

	l.readChar()
	return l
}

func NewFromReader(reader *bufio.Reader) *Lexer {
	l := &Lexer{reader: reader}
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
	rune, _, err := l.reader.ReadRune()

	if err == io.EOF {
		l.char = 0
	} else {
		if err != nil {
			panic(err)
		}
		l.char = rune
	}

	l.position = l.readPosition
	l.readPosition++
	l.textPosition.col++
	// if l.readPosition >= len(l.input) {
	// 	l.char = 0
	// } else {
	// 	l.char = l.input[l.readPosition]
	// }
}

func (l *Lexer) readNumber() string {
	var out bytes.Buffer
	for isDigit(l.char) {
		out.WriteRune(l.char)
		l.readChar()
	}
	return out.String()
}

func isSymbol(char rune) bool {
	return unicode.IsLetter(char) || isEmoji(char)
}

func (l *Lexer) readIdentifier() string {
	var out bytes.Buffer
	for isSymbol(l.char) {
		out.WriteRune(l.char)
		l.readChar()
	}
	if l.char == '!' || l.char == '?' {
		out.WriteRune(l.char)
		l.readChar()
	}
	return out.String()
}

const EOF = 0

func (l *Lexer) readUntil(delimiter rune) string {
	var out bytes.Buffer
	for l.char != delimiter && l.char != EOF {
		out.WriteRune(l.char)
		l.readChar()
	}
	return out.String()
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

const PEEK_BYTES = 5

func (l *Lexer) peekChar() rune {
	peekBytes, err := l.reader.Peek(PEEK_BYTES)
	if err != nil && len(peekBytes) == 0 {
		// TODO: LEXER ERROR
		return 0
	}

	rune, _ := utf8.DecodeRune(peekBytes)
	return rune
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
		if l.peekChar() == '=' {
			char := l.char
			l.readChar()
			literal := string(char) + string(l.char)
			tok = token.Token{Type: token.LT_EQ, Literal: literal}
		} else {
			tok = newToken(token.LT, l.char)
		}
	case '>':
		if l.peekChar() == '=' {
			char := l.char
			l.readChar()
			literal := string(char) + string(l.char)
			tok = token.Token{Type: token.GT_EQ, Literal: literal}
		} else {
			tok = newToken(token.GT, l.char)
		}
	case '/':
		tok = newToken(token.SLASH, l.char)
	case '*':
		tok = newToken(token.ASTERISK, l.char)
	case '.':
		tok = newToken(token.DOT, l.char)
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
	case '%':
		tok = newToken(token.MOD, l.char)
	case '#':
		comment := l.readUntil('\n')
		if comment == "" {
			tok.Type = token.EOF
			tok.Literal = ""
		} else {
			l.readChar() // skipping linebreak
			return l.NextToken()
		}
	case '"':
		tok.Type = token.STRING
		tok.Literal = l.readString()
	// TODO: support for single quotes
	// case '\'':
	// 	tok.Type = token.STRING
	// 	tok.Literal = l.readString()
	case EOF:
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
	var out bytes.Buffer

	for {
		l.readChar()
		if l.char == '"' {
			break
		}
		if l.char == 0 {
			// TODO: throw error
			break
		}
		out.WriteRune(l.char)
	}
	return out.String()
}
