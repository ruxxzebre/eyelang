package syntax

// The tree-sitter grammar as an oracle for the hand-written parser. Its tree
// is converted to the lossless tree this package builds, so the conformance
// suite and the fuzzer compare every node: kind, field, span, flags and
// trivia. Only tests link the grammar, so eye builds without cgo.

import (
	"sync"

	ts "github.com/tree-sitter/go-tree-sitter"

	eyelang "eyelang/tree-sitter-eyelang/src"
)

var language = sync.OnceValue(func() *ts.Language { return ts.NewLanguage(eyelang.Language()) })

// parsers are reused: making a tree-sitter parser allocates.
var parsers = sync.Pool{New: func() any {
	p := ts.NewParser()
	if err := p.SetLanguage(language()); err != nil {
		panic(err)
	}
	return p
}}

// tsParse parses src with the tree-sitter grammar. The caller closes the
// tree.
func tsParse(src []byte) *ts.Tree {
	p := parsers.Get().(*ts.Parser)
	defer parsers.Put(p)
	return p.Parse(src, nil)
}

// tsAccepts reports whether tree-sitter parses src without an error.
func tsAccepts(src []byte) bool {
	tree := tsParse(src)
	defer tree.Close()
	return !tree.RootNode().HasError()
}

// parseTS parses src with the tree-sitter grammar and converts the result to
// the lossless tree the hand-written parser builds. The grammar decides the
// structure; the lexer supplies each token's kind, flags and leading trivia,
// which tree-sitter doesn't keep. Comments, which tree-sitter puts in the
// tree as extras, are trivia here.
func parseTS(src []byte) (*File, *ts.Tree) {
	tokens, lexDiags := Lex(src)
	c := &converter{src: src}
	var trivia []Token
	for _, t := range tokens {
		if t.Kind.IsTrivia() {
			trivia = append(trivia, t)
			continue
		}
		c.toks = append(c.toks, t)
		c.trivia = append(c.trivia, trivia)
		trivia = nil
	}
	tree := tsParse(src)
	root := c.inner(tree.RootNode())
	// What tree-sitter left out of the tree, and the end of the file with the
	// trivia before it.
	eof := len(c.toks) - 1
	if rest := c.until(c.toks[eof].Span.Start); rest != nil {
		root.Children = append(root.Children, rest)
	}
	c.pos = eof
	root = newNode(SourceFile, append(root.Children, c.leaf(eof, EOF)), 0)
	return &File{Src: src, Root: root, Lines: NewLines(src), Diagnostics: lexDiags}, tree
}

type converter struct {
	src    []byte
	toks   []Token   // significant tokens, ending with EOF
	trivia [][]Token // the whitespace and comments before each token
	pos    int       // the next token not yet in the tree
}

// leaf makes token i a leaf of kind k.
func (c *converter) leaf(i int, k Kind) *Node {
	t := c.toks[i]
	return &Node{Kind: k, Flags: t.Flags, Span: t.Span, Leading: c.trivia[i]}
}

// prevEnd is the end of the last token put in the tree.
func (c *converter) prevEnd() int {
	if c.pos == 0 {
		return 0
	}
	return c.toks[c.pos-1].Span.End
}

// until puts the tokens before offset that aren't in the tree yet into an
// ERROR node, or returns nil if there are none.
func (c *converter) until(offset int) *Node {
	var skipped []*Node
	for c.pos < len(c.toks)-1 && c.toks[c.pos].Span.Start < offset {
		skipped = append(skipped, c.leaf(c.pos, c.toks[c.pos].Kind))
		c.pos++
	}
	return errorNode(skipped)
}

// node converts a tree-sitter node and what it contains.
func (c *converter) node(n *ts.Node) *Node {
	if n.ChildCount() == 0 && !n.IsError() {
		return c.token(n)
	}
	return c.inner(n)
}

// inner converts a node that isn't a token.
func (c *converter) inner(n *ts.Node) *Node {
	kind := ErrorNode
	if !n.IsError() {
		kind = nodeKind(n.Kind())
	}
	var children []*Node
	add := func(m *Node, field string) {
		if m == nil {
			return
		}
		m.Field = fieldByName(field)
		children = append(children, m)
	}
	for i := uint(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		if child.IsExtra() {
			continue // a comment: trivia
		}
		field := n.FieldNameForChild(uint32(i))
		if !child.IsMissing() {
			add(c.until(int(child.StartByte())), "")
		}
		add(c.node(child), field)
	}
	if n.IsError() {
		add(c.until(int(n.EndByte())), "")
	}
	return newNode(kind, children, c.prevEnd())
}

// token converts a tree-sitter leaf to the token the lexer read there.
func (c *converter) token(n *ts.Node) *Node {
	if n.IsMissing() {
		return &Node{Kind: missingKind(n), Flags: Missing, Span: Span{c.prevEnd(), c.prevEnd()}}
	}
	start, end := int(n.StartByte()), int(n.EndByte())
	if start == end {
		return nil // a zero-width token of the scanner, such as a line break's
	}
	if c.pos >= len(c.toks)-1 || c.toks[c.pos].Span != (Span{start, end}) {
		// tree-sitter read this text as one token and the lexer didn't: keep
		// the lexer's tokens, as an error.
		return c.until(end)
	}
	i := c.pos
	c.pos++
	if k, ok := nodeKinds()[n.Kind()]; ok && n.IsNamed() && k >= firstNode {
		// A rule that is one token, such as `continue_statement: 'continue'`:
		// tree-sitter makes it a named leaf, the tree a node holding the token.
		return newNode(k, []*Node{c.leaf(i, c.toks[i].Kind)}, 0)
	}
	return c.leaf(i, leafKind(n, c.toks[i].Kind))
}

// leafKind is the kind of a token in the tree: the lexer's, except that a
// name the grammar uses as a contextual keyword is a Keyword, a unit is a
// UnitName, and a reserved word used as a name is an identifier.
func leafKind(n *ts.Node, lexed Kind) Kind {
	if !n.IsNamed() {
		if lexed == Ident || lexed == TypeIdent {
			return Keyword
		}
		return lexed
	}
	switch n.Kind() {
	case "identifier":
		return Ident
	case "type_identifier":
		return TypeIdent
	case "unit_name":
		return UnitName
	}
	if k, ok := nodeKinds()[n.Kind()]; ok && k.IsToken() {
		return k
	}
	return lexed
}

// missingKind is the kind of a token tree-sitter inserted.
func missingKind(n *ts.Node) Kind {
	if k, ok := tokenKinds()[n.Kind()]; ok {
		return k
	}
	if !n.IsNamed() {
		return Keyword
	}
	return nodeKind(n.Kind())
}

// nodeKind is the kind of a named node of the grammar.
func nodeKind(name string) Kind {
	if k, ok := nodeKinds()[name]; ok {
		return k
	}
	return ErrorNode
}

var nodeKinds = sync.OnceValue(func() map[string]Kind {
	m := map[string]Kind{}
	for k, name := range nodeNames {
		m[name] = k
	}
	for k, name := range extraNames {
		m[name] = k
	}
	for k := Kind(0); k < lastToken; k++ {
		if k.IsNamed() {
			m[k.String()] = k
		}
	}
	return m
})

// tokenKinds are the punctuation and reserved words by their text.
var tokenKinds = sync.OnceValue(func() map[string]Kind {
	m := map[string]Kind{}
	for k := Kind(0); k < lastToken; k++ {
		if !k.IsNamed() && k != StringEnd && k != InterpOpen && k != InterpClose {
			m[k.String()] = k
		}
	}
	return m
})

var fieldsByName = sync.OnceValue(func() map[string]Field {
	m := map[string]Field{}
	for f, name := range fieldNames {
		m[name] = Field(f)
	}
	return m
})

func fieldByName(name string) Field { return fieldsByName()[name] }
