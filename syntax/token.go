package syntax

import "strconv"

// Kind is the kind of a token or of a syntax node. Tokens and nodes share one
// numbering so that a lossless tree can hold both.
type Kind uint16

// Token kinds. The names of named tokens (identifier, integer, …) match the
// node names in tree-sitter-eyelang/grammar.js; punctuation and keywords are
// named by their text.
const (
	Invalid Kind = iota
	EOF

	// Trivia: kept in the tree, skipped by the parser.
	Whitespace
	Comment

	// Illegal is a character or escape that no token starts with. The lexer
	// reports a diagnostic for each one.
	Illegal

	Ident     // a value, function, field or module name: world, markAsleep
	TypeIdent // a type, variant, entity or contract name: World, Some
	Int       // 42, 1_000
	Float     // 1.5, 6.02e23
	Date      // 2026-10-10
	Time      // 07:40, 23:59:30
	Char      // 'a', '\n'

	StringStart  // the opening "
	StringText   // text between escapes and interpolations
	StringEscape // \n, \{, \u{1F600}
	StringEnd    // the closing "; zero width and Missing when the string is unterminated
	InterpOpen   // { inside a string
	InterpClose  // the } that ends an interpolation

	LParen    // (
	RParen    // )
	LBracket  // [
	RBracket  // ]
	LBrace    // {
	RBrace    // }
	Comma     // ,
	Colon     // :
	Semicolon // ;
	Dot       // .
	DotDot    // ..
	DotDotEq  // ..=
	DotBrace  // .{
	At        // @
	Arrow     // ->
	FatArrow  // =>
	Assign    // =
	PlusEq    // +=
	MinusEq   // -=
	StarEq    // *=
	SlashEq   // /=
	PercentEq // %=
	Eq        // ==
	NotEq     // !=
	Lt        // <
	LtEq      // <=
	Gt        // >
	GtEq      // >=
	Plus      // +
	Minus     // -
	Star      // *
	Slash     // /
	Percent   // %
	Bang      // !
	Question  // ?
	QuestionQuestion
	Pipe   // |
	PipeGt // |>
	OrOr   // ||
	AndAnd // &&
	Underscore

	// Reserved words. Every other keyword (type, entity, state, job, view,
	// test, with, where, dyn, extern, opaque, migrate, uses, command, … and the
	// unit names) is lexed as Ident or TypeIdent and recognised by the parser
	// where the grammar expects it.
	KwLet
	KwVar
	KwMut
	KwFn
	KwPub
	KwReturn
	KwIf
	KwElse
	KwMatch
	KwFor
	KwIn
	KwWhile
	KwBreak
	KwContinue
	KwTrue
	KwFalse

	lastToken
)

var tokenNames = [...]string{
	Invalid:          "invalid",
	EOF:              "end of file",
	Whitespace:       "whitespace",
	Comment:          "comment",
	Illegal:          "illegal",
	Ident:            "identifier",
	TypeIdent:        "type_identifier",
	Int:              "integer",
	Float:            "float",
	Date:             "date",
	Time:             "time",
	Char:             "char",
	StringStart:      `"`,
	StringText:       "string_content",
	StringEscape:     "escape_sequence",
	StringEnd:        `"`,
	InterpOpen:       "interpolation_open",
	InterpClose:      "interpolation_close",
	LParen:           "(",
	RParen:           ")",
	LBracket:         "[",
	RBracket:         "]",
	LBrace:           "{",
	RBrace:           "}",
	Comma:            ",",
	Colon:            ":",
	Semicolon:        ";",
	Dot:              ".",
	DotDot:           "..",
	DotDotEq:         "..=",
	DotBrace:         ".{",
	At:               "@",
	Arrow:            "->",
	FatArrow:         "=>",
	Assign:           "=",
	PlusEq:           "+=",
	MinusEq:          "-=",
	StarEq:           "*=",
	SlashEq:          "/=",
	PercentEq:        "%=",
	Eq:               "==",
	NotEq:            "!=",
	Lt:               "<",
	LtEq:             "<=",
	Gt:               ">",
	GtEq:             ">=",
	Plus:             "+",
	Minus:            "-",
	Star:             "*",
	Slash:            "/",
	Percent:          "%",
	Bang:             "!",
	Question:         "?",
	QuestionQuestion: "??",
	Pipe:             "|",
	PipeGt:           "|>",
	OrOr:             "||",
	AndAnd:           "&&",
	Underscore:       "_",
	KwLet:            "let",
	KwVar:            "var",
	KwMut:            "mut",
	KwFn:             "fn",
	KwPub:            "pub",
	KwReturn:         "return",
	KwIf:             "if",
	KwElse:           "else",
	KwMatch:          "match",
	KwFor:            "for",
	KwIn:             "in",
	KwWhile:          "while",
	KwBreak:          "break",
	KwContinue:       "continue",
	KwTrue:           "true",
	KwFalse:          "false",
}

// String returns the text of punctuation and keywords, and the tree-sitter
// node name of named tokens and nodes.
func (k Kind) String() string {
	if int(k) < len(tokenNames) && tokenNames[k] != "" {
		return tokenNames[k]
	}
	if name, ok := nodeNames[k]; ok {
		return name
	}
	if name, ok := extraNames[k]; ok {
		return name
	}
	return "kind(" + strconv.Itoa(int(k)) + ")"
}

// IsTrivia reports whether tokens of this kind are whitespace or comments.
func (k Kind) IsTrivia() bool { return k == Whitespace || k == Comment }

// IsKeyword reports whether k is a reserved word.
func (k Kind) IsKeyword() bool { return k >= KwLet && k <= KwFalse }

// IsToken reports whether k is a token kind rather than a node kind.
func (k Kind) IsToken() bool { return k < lastToken }

var keywords = map[string]Kind{
	"let":      KwLet,
	"var":      KwVar,
	"mut":      KwMut,
	"fn":       KwFn,
	"pub":      KwPub,
	"return":   KwReturn,
	"if":       KwIf,
	"else":     KwElse,
	"match":    KwMatch,
	"for":      KwFor,
	"in":       KwIn,
	"while":    KwWhile,
	"break":    KwBreak,
	"continue": KwContinue,
	"true":     KwTrue,
	"false":    KwFalse,
	"_":        Underscore,
}

// TokenFlags describe a token's surroundings.
type TokenFlags uint8

const (
	// SpaceBefore: whitespace or a comment separates the token from the
	// previous one. A waiting call's `!(` and a propagating `?` must not have it.
	SpaceBefore TokenFlags = 1 << iota
	// NewlineBefore: a line break separates the token from the previous one.
	NewlineBefore
	// Missing: the token is not in the source. The lexer adds a zero-width
	// StringEnd to close an unterminated string.
	Missing
)

// Token is one token of the source, trivia included. Concatenating the text of
// every token reproduces the source byte for byte.
type Token struct {
	Kind  Kind
	Flags TokenFlags
	Span  Span
}

// Span is a half-open range of byte offsets into the source.
type Span struct {
	Start, End int
}

// Len returns the number of bytes in the span.
func (s Span) Len() int { return s.End - s.Start }

// Diagnostic is a syntax error with the source range it is about.
type Diagnostic struct {
	Span    Span
	Message string
}
