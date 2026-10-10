package syntax

import (
	"strconv"
	"strings"
)

// File is a parsed source file.
type File struct {
	Src         []byte
	Root        *Node // a SourceFile node
	Lines       *Lines
	Diagnostics []Diagnostic // lexer and parser errors, in source order
}

// Node is a node of the lossless syntax tree. A token is a leaf node: it has
// no children, its Span covers its text, and Leading holds the whitespace and
// comments before it. Every byte of the source belongs to exactly one token
// or trivia token, so printing the leaves in order reproduces the source.
type Node struct {
	Kind     Kind
	Field    Field      // the node's role in its parent
	Flags    TokenFlags // tokens only
	Span     Span       // from the first token's text to the last token's text
	Leading  []Token    // tokens only: trivia before the token
	Children []*Node
}

// IsToken reports whether the node is a token rather than an inner node.
func (n *Node) IsToken() bool { return n.Kind.IsToken() || n.Kind == Keyword || n.Kind == UnitName }

// IsMissing reports whether the token was inserted by the parser or lexer to
// complete the tree and has no text.
func (n *Node) IsMissing() bool { return n.Flags&Missing != 0 }

// Text returns the source text of the node's span, without leading trivia.
func (n *Node) Text(src []byte) string { return string(src[n.Span.Start:n.Span.End]) }

// ChildByField returns the first child with the given field, or nil.
func (n *Node) ChildByField(f Field) *Node {
	for _, c := range n.Children {
		if c.Field == f {
			return c
		}
	}
	return nil
}

// ChildrenByField returns every child with the given field.
func (n *Node) ChildrenByField(f Field) []*Node {
	var out []*Node
	for _, c := range n.Children {
		if c.Field == f {
			out = append(out, c)
		}
	}
	return out
}

// unfielded returns the named children that have no field.
func (n *Node) unfielded() []*Node {
	var out []*Node
	for _, c := range n.Children {
		if c.Field == NoField && c.Kind.IsNamed() {
			out = append(out, c)
		}
	}
	return out
}

// NamedChildren returns the children that are named nodes, as tree-sitter
// defines them.
func (n *Node) NamedChildren() []*Node {
	var out []*Node
	for _, c := range n.Children {
		if c.Kind.IsNamed() {
			out = append(out, c)
		}
	}
	return out
}

// Walk calls visit for n and its descendants in source order. If visit
// returns false, Walk skips that node's children.
func (n *Node) Walk(visit func(*Node) bool) {
	if !visit(n) {
		return
	}
	for _, c := range n.Children {
		c.Walk(visit)
	}
}

// Source reproduces the text the tree was parsed from, trivia included.
func (n *Node) Source(src []byte) []byte {
	var out []byte
	n.Walk(func(m *Node) bool {
		if m.IsToken() {
			for _, t := range m.Leading {
				out = append(out, src[t.Span.Start:t.Span.End]...)
			}
			out = append(out, src[m.Span.Start:m.Span.End]...)
		}
		return true
	})
	return out
}

// HasError reports whether the tree contains an error node, an illegal token
// or a missing token.
func (n *Node) HasError() bool {
	found := false
	n.Walk(func(m *Node) bool {
		if m.Kind == ErrorNode || m.Kind == Illegal || m.IsMissing() {
			found = true
		}
		return !found
	})
	return found
}

// SExprOptions select what SExpr prints.
type SExprOptions struct {
	// Tokens includes anonymous tokens as quoted strings and the text of
	// named leaves: (binary_expression left: (identifier "a") "+" …).
	Tokens bool
	// Fields prefixes children with their field name.
	Fields bool
	// Indent breaks the output into lines, one named node per line.
	Indent bool
}

// SExpr prints the tree as an S-expression. With Fields and Tokens it is the
// normalised form that the conformance suite compares with tree-sitter's
// output; without either it matches tree-sitter's corpus test format.
// Comments are trivia and never appear; the EOF token is left out.
func (n *Node) SExpr(src []byte, opts SExprOptions) string {
	var b strings.Builder
	n.writeSExpr(&b, src, opts, 0)
	return b.String()
}

func (n *Node) writeSExpr(b *strings.Builder, src []byte, opts SExprOptions, depth int) {
	if opts.Fields && n.Field != NoField && n.Kind.IsNamed() {
		b.WriteString(n.Field.String())
		b.WriteString(": ")
	}
	if !n.Kind.IsNamed() {
		b.WriteString(strconv.Quote(n.Text(src)))
		return
	}
	b.WriteString("(")
	if n.IsMissing() {
		b.WriteString("MISSING ")
	}
	b.WriteString(n.Kind.String())
	if n.IsToken() {
		if opts.Tokens {
			b.WriteString(" ")
			b.WriteString(strconv.Quote(n.Text(src)))
		}
		b.WriteString(")")
		return
	}
	for _, c := range n.Children {
		if c.Kind == EOF || (!opts.Tokens && !c.Kind.IsNamed()) {
			continue
		}
		if opts.Indent && c.Kind.IsNamed() {
			b.WriteString("\n")
			b.WriteString(strings.Repeat("  ", depth+1))
		} else {
			b.WriteString(" ")
		}
		c.writeSExpr(b, src, opts, depth+1)
	}
	b.WriteString(")")
}
