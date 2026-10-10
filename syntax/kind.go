package syntax

//go:generate go run ./gen

// Token kinds that the parser assigns in place of what the lexer produced.
const (
	// Keyword is a contextual keyword such as `migrate`, `uses` or `with`.
	// The lexer reads it as a name; the parser renames it where the grammar
	// expects the keyword.
	Keyword Kind = lastToken + iota
	// UnitName is the unit of a duration or size: ms, min, MiB.
	UnitName

	// ErrorNode wraps input the parser skipped. The other node kinds are in
	// kind_gen.go, generated from the grammar.
	ErrorNode

	firstNode = ErrorNode
)

var extraNames = map[Kind]string{
	Keyword:  "keyword",
	UnitName: "unit_name",
}

// IsNamed reports whether nodes of this kind are named nodes in the
// tree-sitter grammar. Punctuation, keywords and quotes are anonymous.
func (k Kind) IsNamed() bool {
	switch k {
	case Ident, TypeIdent, Int, Float, Date, Time, Char, StringText, StringEscape, UnitName, Comment, Illegal:
		return true
	}
	return k >= firstNode
}

// Field is the role a node plays in its parent, named as in the grammar's
// field() calls. The fields are in kind_gen.go.
type Field uint8

func (f Field) String() string { return fieldNames[f] }
