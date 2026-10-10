package lsp

import (
	"sort"
	"strings"

	"eyelang/syntax"
)

// Semantic tokens refine what the editor's TextMate grammar can see: which
// names are functions, parameters, fields, types or variants, where a word
// such as `type` or `state` is a keyword, and which calls are waiting calls.
// Strings, numbers and comments are left to the grammar.

var tokenTypes = []string{
	"namespace", "type", "typeParameter", "parameter", "variable", "property", "enumMember",
	"function", "method", "keyword", "decorator", "waitingCall",
}

var tokenModifiers = []string{"declaration", "documentation"}

const (
	tokNamespace = iota
	tokType
	tokTypeParameter
	tokParameter
	tokVariable
	tokProperty
	tokEnumMember
	tokFunction
	tokMethod
	tokKeyword
	tokDecorator
	tokWaitingCall
)

const modDeclaration = 1 << 0

type semanticTokens struct {
	Data []uint32 `json:"data"`
}

type semToken struct {
	start, end int // byte offsets
	typ, mods  int
}

func semanticData(d *document) []uint32 {
	var toks []semToken
	var walk func(n, parent, grand *syntax.Node)
	walk = func(n, parent, grand *syntax.Node) {
		if n.IsToken() {
			if typ, mods, ok := classify(n, parent, grand); ok && n.Span.Len() > 0 {
				toks = append(toks, semToken{n.Span.Start, n.Span.End, typ, mods})
			}
			return
		}
		for _, c := range n.Children {
			walk(c, n, parent)
		}
	}
	walk(d.file.Root, nil, nil)
	sort.Slice(toks, func(i, j int) bool { return toks[i].start < toks[j].start })

	data := make([]uint32, 0, len(toks)*5)
	prevLine, prevChar := 0, 0
	for _, t := range toks {
		start := d.position(t.start)
		length := utf16Len(d.text[t.start:t.end])
		if strings.ContainsRune(string(d.text[t.start:t.end]), '\n') {
			continue // tokens never span lines here, but the protocol needs that
		}
		deltaLine := start.Line - prevLine
		deltaChar := start.Character
		if deltaLine == 0 {
			deltaChar -= prevChar
		}
		data = append(data, uint32(deltaLine), uint32(deltaChar), uint32(length), uint32(t.typ), uint32(t.mods))
		prevLine, prevChar = start.Line, start.Character
	}
	return data
}

// typeParents are the nodes in which a type name is a type rather than a
// variant or constructor.
var typeParents = map[syntax.Kind]bool{
	syntax.GenericType: true, syntax.QualifiedType: true, syntax.TypeArguments: true,
	syntax.TupleType: true, syntax.FunctionTypeParameters: true, syntax.FunctionType: true,
	syntax.DynType: true, syntax.Requirement: true, syntax.VariantPayload: true,
	syntax.TypeDeclaration: true, syntax.EntityDeclaration: true, syntax.TraitDeclaration: true,
	syntax.ContractDeclaration: true, syntax.ImplDeclaration: true, syntax.Parameter: true,
	syntax.FieldDeclaration: true, syntax.StateDeclaration: true, syntax.EventDeclaration: true,
	syntax.CommandDeclaration: true, syntax.QueryDeclaration: true, syntax.FunctionDeclaration: true,
	syntax.FunctionSignature: true, syntax.ExternDeclaration: true, syntax.JobDeclaration: true,
	syntax.LetStatement: true, syntax.VarStatement: true, syntax.RecordType: true,
	syntax.ImportDeclaration: true,
}

// classify gives a token's semantic type, if it has one worth sending.
func classify(n, parent, grand *syntax.Node) (typ, mods int, ok bool) {
	if parent == nil {
		return 0, 0, false
	}
	switch n.Kind {
	case syntax.Keyword:
		return tokKeyword, 0, true
	case syntax.Bang:
		if parent.Kind == syntax.WaitingCallExpression {
			return tokWaitingCall, 0, true
		}
	case syntax.At:
		if parent.Kind == syntax.Attribute {
			return tokDecorator, 0, true
		}
	case syntax.TypeIdent:
		switch {
		case parent.Kind == syntax.Variant:
			return tokEnumMember, modDeclaration, true
		case parent.Kind == syntax.FieldExpression && n.Field == syntax.FieldValue:
			return tokType, 0, true
		case n.Field == syntax.FieldName && (parent.Kind == syntax.TypeDeclaration || parent.Kind == syntax.EntityDeclaration ||
			parent.Kind == syntax.TraitDeclaration || parent.Kind == syntax.ContractDeclaration):
			return tokType, modDeclaration, true
		case typeParents[parent.Kind]:
			return tokType, 0, true
		}
		return tokEnumMember, 0, true
	case syntax.Ident:
		return classifyIdent(n, parent, grand)
	}
	return 0, 0, false
}

func classifyIdent(n, parent, grand *syntax.Node) (typ, mods int, ok bool) {
	field := n.Field
	switch parent.Kind {
	case syntax.FunctionDeclaration, syntax.FunctionSignature, syntax.ExternDeclaration, syntax.JobDeclaration,
		syntax.CommandDeclaration, syntax.QueryDeclaration:
		if field == syntax.FieldName {
			if grand != nil && (grand.Kind == syntax.ImplBody || grand.Kind == syntax.TraitBody || grand.Kind == syntax.EntityBody) {
				return tokMethod, modDeclaration, true
			}
			return tokFunction, modDeclaration, true
		}
	case syntax.Parameter:
		if field == syntax.FieldName {
			return tokParameter, modDeclaration, true
		}
	case syntax.Lambda:
		if field == syntax.FieldParameter {
			return tokParameter, modDeclaration, true
		}
	case syntax.NamedArgument:
		if field == syntax.FieldName {
			return tokParameter, 0, true
		}
	case syntax.FieldDeclaration, syntax.StateDeclaration, syntax.FieldInitializer, syntax.NamedFieldPattern,
		syntax.EventDeclaration:
		if field == syntax.FieldName {
			return tokProperty, modDeclaration, true
		}
	case syntax.ShorthandField, syntax.ShorthandFieldPattern:
		return tokProperty, 0, true
	case syntax.FieldExpression:
		if field == syntax.FieldField {
			switch {
			case grand != nil && grand.Kind == syntax.WaitingCallExpression:
				return tokWaitingCall, 0, true
			case grand != nil && grand.Kind == syntax.CallExpression:
				return tokMethod, 0, true
			}
			return tokProperty, 0, true
		}
	case syntax.CallExpression:
		if field == syntax.FieldFunction {
			return tokFunction, 0, true
		}
	case syntax.WaitingCallExpression:
		if field == syntax.FieldFunction {
			return tokWaitingCall, 0, true
		}
	case syntax.ImportPath, syntax.QualifiedType:
		if parent.Kind == syntax.ImportPath || field == syntax.FieldModule {
			return tokNamespace, 0, true
		}
	case syntax.TypeVariable, syntax.TypeParameters:
		return tokTypeParameter, 0, true
	case syntax.Requirement:
		if field == syntax.FieldName {
			return tokTypeParameter, 0, true
		}
	case syntax.Attribute:
		if field == syntax.FieldName {
			return tokDecorator, 0, true
		}
	case syntax.Effect:
		if field == syntax.FieldKind {
			return tokKeyword, 0, true
		}
	case syntax.ViewDeclaration:
		if field == syntax.FieldName {
			return tokVariable, modDeclaration, true
		}
	}
	return tokVariable, 0, true
}
