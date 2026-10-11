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
	offset := d.offset(p)
	res := s.checked(path)
	use, ok := res.Index.At(path, offset)
	if !ok {
		return s.keywordHover(d, offset)
	}
	// Links in a doc comment are names in the module the comment is in.
	link := func(name string) (string, bool) { return "`" + name + "`", true }
	if at, ok := check.Definition(use.Target); ok && at.File != nil {
		link = func(name string) (string, bool) {
			if loc, ok := s.targetLocation(lookup(res, at.File.Path, name)); ok {
				return fmt.Sprintf("[`%s`](%s)", name, lineLink(loc)), true
			}
			return "`" + name + "`", true
		}
	}
	text := describe(use.Target, link)
	if _, local := use.Target.(check.Local); local {
		// A local variable or parameter: its name and its type, as checked.
		if t, ok := res.Index.TypeText(path, use.At.Offset, use.At.End); ok {
			text = hoverText(use.At.Text()+": "+t, "", link)
		}
	}
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
func describe(target any, link func(string) (string, bool)) string {
	switch t := target.(type) {
	case ir.Builtin:
		for _, b := range ir.BuiltinDocs {
			if b.Name == t.Name {
				return hoverText(b.Signature, b.Doc, link)
			}
		}
		return ""
	case check.BuiltinType:
		for _, b := range ir.BuiltinTypeDocs {
			if b.Name == t.Name {
				return hoverText(b.Signature, b.Doc, link)
			}
		}
		return hoverText("type "+t.Name, "A type built into the language.", link)
	case ir.EnumFn:
		if t.Name == "variants" {
			return hoverText("fn "+t.Type.Name+".variants() -> List("+t.Type.Name+")",
				"Every variant of "+t.Type.Name+", in the order declared. Derived for a type whose variants have no fields.", link)
		}
		return hoverText("fn "+t.Type.Name+".parse(name: String) -> Option("+t.Type.Name+")",
			"`Some` of the variant of "+t.Type.Name+" whose name is `name`, else `None`. Derived for a type whose variants have no fields.", link)
	case check.Local:
		return "" // its type isn't recorded yet
	case *ir.Module:
		doc := ""
		for _, f := range t.Files {
			if doc = syntax.ModuleDoc(f.Tree); doc != "" {
				break
			}
		}
		return hoverText("import "+t.Path, doc, link)
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
	return hoverText(sig, doc, link)
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

// hoverText puts a signature and a doc comment together, with the comment's
// [Name] links made by link.
func hoverText(sig, doc string, link func(string) (string, bool)) string {
	var b strings.Builder
	fmt.Fprintf(&b, "```eye\n%s\n```", sig)
	if doc != "" {
		b.WriteString("\n\n" + syntax.DocLinks(doc, link))
	}
	return b.String()
}

// lineLink is a link to a location that editors open at its line.
func lineLink(loc location) string {
	return fmt.Sprintf("%s#L%d", loc.URI, loc.Range.Start.Line+1)
}

// keywordHover describes the keyword or operator at offset: what it means,
// and a link to the section of SPECS.md that defines it.
func (s *server) keywordHover(d *document, offset int) (any, *responseError) {
	tok, _ := keywordAt(d, offset)
	if tok == nil {
		return nil, nil
	}
	word := tok.Text(d.text)
	summary, loc, ok := s.keywordDoc(word)
	if !ok {
		return nil, nil
	}
	text := "`" + word + "`: " + summary
	if loc.URI != "" {
		text += "\n\n[The rules in SPECS.md](" + lineLink(loc) + ")"
	}
	return hoverResult{Contents: markupContent{Kind: "markdown", Value: text}, Range: d.rangeOf(tok.Span)}, nil
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

// lookup finds what a doc comment's [Name] names, as Result.Lookup does, and
// also a member of a type or entity: [Departure.delay], [Shape.Circle],
// [World.join].
func lookup(res *check.Result, path, name string) any {
	if t := res.Lookup(path, name); t != nil {
		return t
	}
	owner, member, ok := strings.Cut(name, ".")
	if !ok {
		return nil
	}
	switch o := res.Lookup(path, owner).(type) {
	case *ir.Type:
		if m := o.Methods[member]; m != nil {
			return m
		}
		for _, v := range o.Variants {
			if v.Name == member {
				return v
			}
		}
		return check.Member{Of: o, Name: member}
	case *ir.Entity:
		if m := o.Methods[member]; m != nil {
			return m
		}
		return check.Member{Of: o, Name: member}
	}
	return nil
}
