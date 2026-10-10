package lexer

import (
	"testing"

	"eyelang/token"
)

func TestNextToken(t *testing.T) {
	input := `let five = 5;
let ten = 10;

let add = fn(x, y) {
  x + y;
};

let result = add(five, ten);
!-/*5;
5 < 10 > 5;

if (5 < 10) {
	return true;
} else {
	return false;
}

10 == 10;
10 != 9;
"foo.bar"
foo.bar
"foobar"
# this is a comment, it should be ignored
# no amount of comments should affect the test below
"foo bar"
[1, 2];
{"hello": 1, "bye": "no"};
# comment at the end
2.1;
0.1;
0.000001;
`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.LET, "let"},
		{token.IDENT, "five"},
		{token.ASSIGN, "="},
		{token.INT, "5"},
		{token.SEMICOLON, ";"},
		{token.LET, "let"},
		{token.IDENT, "ten"},
		{token.ASSIGN, "="},
		{token.INT, "10"},
		{token.SEMICOLON, ";"},
		{token.LET, "let"},
		{token.IDENT, "add"},
		{token.ASSIGN, "="},
		{token.FUNCTION, "fn"},
		{token.LPAREN, "("},
		{token.IDENT, "x"},
		{token.COMMA, ","},
		{token.IDENT, "y"},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.IDENT, "x"},
		{token.PLUS, "+"},
		{token.IDENT, "y"},
		{token.SEMICOLON, ";"},
		{token.RBRACE, "}"},
		{token.SEMICOLON, ";"},
		{token.LET, "let"},
		{token.IDENT, "result"},
		{token.ASSIGN, "="},
		{token.IDENT, "add"},
		{token.LPAREN, "("},
		{token.IDENT, "five"},
		{token.COMMA, ","},
		{token.IDENT, "ten"},
		{token.RPAREN, ")"},
		{token.SEMICOLON, ";"},
		{token.BANG, "!"},
		{token.MINUS, "-"},
		{token.SLASH, "/"},
		{token.ASTERISK, "*"},
		{token.INT, "5"},
		{token.SEMICOLON, ";"},
		{token.INT, "5"},
		{token.LT, "<"},
		{token.INT, "10"},
		{token.GT, ">"},
		{token.INT, "5"},
		{token.SEMICOLON, ";"},
		{token.IF, "if"},
		{token.LPAREN, "("},
		{token.INT, "5"},
		{token.LT, "<"},
		{token.INT, "10"},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.RETURN, "return"},
		{token.TRUE, "true"},
		{token.SEMICOLON, ";"},
		{token.RBRACE, "}"},
		{token.ELSE, "else"},
		{token.LBRACE, "{"},
		{token.RETURN, "return"},
		{token.FALSE, "false"},
		{token.SEMICOLON, ";"},
		{token.RBRACE, "}"},
		{token.INT, "10"},
		{token.EQ, "=="},
		{token.INT, "10"},
		{token.SEMICOLON, ";"},
		{token.INT, "10"},
		{token.NOT_EQ, "!="},
		{token.INT, "9"},
		{token.SEMICOLON, ";"},
		{token.STRING, "foo.bar"},
		{token.IDENT, "foo"},
		{token.DOT, "."},
		{token.IDENT, "bar"},
		{token.STRING, "foobar"},
		{token.STRING, "foo bar"},
		{token.LBRACKET, "["},
		{token.INT, "1"},
		{token.COMMA, ","},
		{token.INT, "2"},
		{token.RBRACKET, "]"},
		{token.SEMICOLON, ";"},
		{token.LBRACE, "{"},
		{token.STRING, "hello"},
		{token.COLON, ":"},
		{token.INT, "1"},
		{token.COMMA, ","},
		{token.STRING, "bye"},
		{token.COLON, ":"},
		{token.STRING, "no"},
		{token.RBRACE, "}"},
		{token.SEMICOLON, ";"},
		{token.FLOAT, "2.1"},
		{token.SEMICOLON, ";"},
		{token.FLOAT, "0.1"},
		{token.SEMICOLON, ";"},
		{token.FLOAT, "0.000001"},
		{token.SEMICOLON, ";"},
		{token.EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Log(tok.Type, tok.Literal, tok.Position)
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestIdentifiers(t *testing.T) {
	input := `x1 snake_case _private ok? done! 1_000`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.IDENT, "x1"},
		{token.IDENT, "snake_case"},
		{token.IDENT, "_private"},
		{token.IDENT, "ok?"},
		{token.IDENT, "done!"},
		{token.INT, "1000"},
		{token.EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType || tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - expected=(%q, %q), got=(%q, %q)",
				i, tt.expectedType, tt.expectedLiteral, tok.Type, tok.Literal)
		}
	}
}

func TestTokenPositions(t *testing.T) {
	input := "a\n# comment\nb\n\"x\ny\" z"

	tests := []struct {
		expectedLiteral string
		row, col        int
	}{
		{"a", 0, 0},
		{"b", 2, 0},
		{"x\ny", 3, 0},
		{"z", 4, 3},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
		if tok.Position.Row != tt.row || tok.Position.Col != tt.col {
			t.Errorf("tests[%d] - %q position wrong. expected=(%d:%d), got=%s",
				i, tt.expectedLiteral, tt.row, tt.col, tok.PositionString())
		}
	}
}

func TestUnterminatedString(t *testing.T) {
	l := New(`"abc`)
	tok := l.NextToken()
	if tok.Type != token.ILLEGAL || tok.Literal != `"abc` {
		t.Fatalf("expected ILLEGAL token for unterminated string. got=(%q, %q)", tok.Type, tok.Literal)
	}
}
