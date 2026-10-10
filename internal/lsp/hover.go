package lsp

import (
	"fmt"
	"strings"

	"eyelang/check"
	"eyelang/ir"
	"eyelang/syntax"
)

type markupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type hoverResult struct {
	Contents markupContent `json:"contents"`
	Range    Range         `json:"range"`
}

// hover answers textDocument/hover: the signature and the doc comment of
// what the name at p refers to, as Markdown. The doc comment is the same
// text eye doc publishes.
func (s *server) hover(d *document, p Position) (any, *responseError) {
	path, ok := uriPath(d.uri)
	if !ok {
		return nil, nil
	}
	use, ok := s.checked(path).Index.At(path, d.offset(p))
	if !ok {
		return nil, nil
	}
	text := describe(use.Target)
	if text == "" {
		return nil, nil
	}
	return hoverResult{
		Contents: markupContent{Kind: "markdown", Value: text},
		Range:    d.rangeOf(syntax.Span{Start: use.At.Offset, End: use.At.End}),
	}, nil
}

// describe returns the hover text for what a name refers to: its signature
// in an eye code block, then its doc comment.
func describe(target any) string {
	switch t := target.(type) {
	case ir.Builtin:
		for _, b := range ir.BuiltinDocs {
			if b.Name == t.Name {
				return hoverText(b.Signature, b.Doc)
			}
		}
		return ""
	case check.BuiltinType:
		return hoverText("type "+t.Name, "A type built into the language.")
	case check.Local:
		return "" // its type isn't recorded yet
	case *ir.Module:
		doc := ""
		for _, f := range t.Files {
			if doc = syntax.ModuleDoc(f.Tree); doc != "" {
				break
			}
		}
		return hoverText("import "+t.Path, doc)
	}
	at, ok := check.Definition(target)
	if !ok || at.File == nil || at.File.Tree == nil {
		return ""
	}
	decl := declarationAt(at.File.Tree, at.Offset)
	if decl == nil {
		return ""
	}
	src := at.File.Src
	sig := signature(decl, src)
	doc := syntax.DocComment(decl, src)
	switch t := target.(type) {
	case *ir.Variant:
		// A variant's doc comment sits above its `|`, which the checker has
		// already read.
		doc = t.Doc
		if t.Type.Name == t.Name {
			// `type UserId = | UserId(String)`: the type's doc is the variant's.
			if doc == "" {
				doc = t.Type.Doc
			}
		} else {
			doc = strings.TrimSpace(doc + "\n\nA variant of `" + t.Type.Name + "`.")
		}
	case check.Member:
		if owner := ownerName(t.Of); owner != "" {
			doc = strings.TrimSpace(doc + "\n\nA field of `" + owner + "`.")
		}
	}
	return hoverText(sig, doc)
}

func ownerName(of any) string {
	switch o := of.(type) {
	case *ir.Type:
		return o.Name
	case *ir.Entity:
		return o.Name
	}
	return ""
}

// hoverText puts a signature and a doc comment together. Links in the doc
// comment show as code: the hover already sits on the code they name.
func hoverText(sig, doc string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "```eye\n%s\n```", sig)
	if doc != "" {
		doc = syntax.DocLinks(doc, func(name string) (string, bool) { return "`" + name + "`", true })
		b.WriteString("\n\n" + doc)
	}
	return b.String()
}

// declarationAt finds the innermost node that declares the name starting at
// offset: a declaration, member, field, variant, parameter or binding.
func declarationAt(f *syntax.File, offset int) *syntax.Node {
	var found *syntax.Node
	f.Root.Walk(func(n *syntax.Node) bool {
		if n.IsToken() || offset < n.Span.Start || offset > n.Span.End {
			return false
		}
		for _, field := range []syntax.Field{syntax.FieldName, syntax.FieldPattern} {
			if c := n.ChildByField(field); c != nil && c.Span.Start == offset {
				found = n
			}
		}
		return true
	})
	return found
}

// maxSignatureLines keeps a long type or contract from filling the hover.
const maxSignatureLines = 16

// signature is how a declaration reads in a hover: a function, job, entity,
// trait or contract up to its body, anything else as written, without doc
// comments.
func signature(n *syntax.Node, src []byte) string {
	text := n.Text(src)
	if body := n.ChildByField(syntax.FieldBody); body != nil {
		switch n.Kind {
		case syntax.FunctionDeclaration, syntax.JobDeclaration, syntax.EntityDeclaration,
			syntax.TraitDeclaration, syntax.ContractDeclaration, syntax.ImplDeclaration:
			text = strings.TrimSpace(string(src[n.Span.Start:body.Span.Start]))
			text = strings.TrimSpace(strings.TrimSuffix(text, "="))
		}
	}
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "##") {
			continue
		}
		if len(lines) == maxSignatureLines {
			lines = append(lines, "  …")
			break
		}
		lines = append(lines, l)
	}
	return strings.Join(lines, "\n")
}
