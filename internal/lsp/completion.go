package lsp

import (
	"bytes"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"eyelang/check"
	"eyelang/ir"
	"eyelang/syntax"
)

// completionItem is an LSP completion item.
type completionItem struct {
	Label         string        `json:"label"`
	Kind          int           `json:"kind"`
	Detail        string        `json:"detail,omitempty"`
	Documentation *markupString `json:"documentation,omitempty"`
	SortText      string        `json:"sortText,omitempty"`
}

type markupString struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// LSP completion item kinds.
const (
	completionText    = 1
	completionKeyword = 14
)

// The keywords that can start a line in each kind of body, and the ones that
// can follow `pub` there.
var (
	itemStarts = map[scope][]string{
		scopeFile:     {"import", "pub", "fn", "extern", "type", "entity", "job", "view", "trait", "impl", "contract", "test", "let"},
		scopeEntity:   {"state", "pub", "fn", "migrate"},
		scopeTrait:    {"fn"},
		scopeImpl:     {"fn"},
		scopeContract: {"command", "query", "event"},
		scopeBlock:    {"let", "var", "if", "match", "for", "while", "with", "return", "break", "continue", "fn", "true", "false"},
	}
	afterPub = map[scope][]string{
		scopeFile:   {"fn", "extern", "type", "opaque", "entity", "job", "view", "trait", "contract", "let"},
		scopeEntity: {"fn"},
	}
	// Keywords that start an expression, offered after the start of a line
	// in a block.
	expressionStarts = []string{"if", "match", "fn", "true", "false"}
)

// scope is the kind of body the cursor is in.
type scope int

const (
	scopeFile scope = iota
	scopeEntity
	scopeTrait
	scopeImpl
	scopeContract
	scopeBlock // a function, job or test body, or any braces inside one
	scopeOther // a record type's fields, or braces we can't place
)

// completion offers what may be typed at p. After `x.` it offers the
// members of x. Elsewhere it offers the keywords that fit, read from the
// tokens rather than the tree, because the line being typed is rarely valid
// yet; and where an expression can start, the names in scope: locals, the
// module's declarations, imports, the prelude's and the built-ins.
//
// Where it knows nothing to offer, it offers nothing, and VS Code shows its
// own list of the file's words instead.
func (s *server) completion(d *document, p Position) (any, *responseError) {
	offset := d.offset(p)
	if recv, dot, ok := receiverAt(d.text, offset); ok {
		return s.members(d, recv, dot), nil
	}
	words, names := keywordsAt(d.text, offset)
	items := []completionItem{}
	for _, w := range words {
		item := completionItem{Label: w, Kind: completionKeyword}
		if summary, _, ok := s.keywordDoc(w); ok {
			item.Documentation = &markupString{Kind: "markdown", Value: summary}
		}
		items = append(items, item)
	}
	if !names {
		return items, nil
	}
	res, path, ok := s.checkedWithoutLine(d, offset)
	if !ok {
		for _, n := range namesIn(d.text, offset) {
			items = append(items, completionItem{Label: n, Kind: completionText})
		}
		return items, nil
	}
	return append(items, scopeNames(res, path, d, offset)...), nil
}

// checkedWithoutLine checks the program with the document as the editor has
// it. The checker skips a file with syntax errors, and the line being typed
// usually has one; so when the document doesn't parse, that line is checked
// as blank, which keeps every other offset where it was.
func (s *server) checkedWithoutLine(d *document, offset int) (*check.Result, string, bool) {
	path, ok := uriPath(d.uri)
	if !ok {
		return nil, "", false
	}
	text := d.text
	if len(d.file.Diagnostics) > 0 {
		text = bytes.Clone(d.text)
		start := bytes.LastIndexByte(text[:offset], '\n') + 1
		end := len(text)
		if i := bytes.IndexByte(text[offset:], '\n'); i >= 0 {
			end = offset + i
		}
		for i := start; i < end; i++ {
			text[i] = ' '
		}
	}
	res := s.checkText(path, text)
	return res, path, res.Program != nil
}

// checkText checks the program that the file at path belongs to, with text
// as that file's source.
func (s *server) checkText(path string, text []byte) *check.Result {
	root := check.Root(path)
	overlay := s.overlay(root)
	overlay[path] = text
	return check.Check(root, check.Options{Overlay: overlay, Module: filepath.Dir(path)})
}

// frame is one open brace, paren or bracket before the cursor.
type frame struct {
	scope scope
	paren bool        // ( or [ rather than {
	head  []string    // the words of the line being read in this body, attributes left out
	after syntax.Kind // the last significant token in this body
	fresh bool        // the next token starts a line of this body
}

// keywordsAt returns the keywords that may be typed at offset in src, and
// whether a name may be typed there too: in a body, where an expression
// can start.
func keywordsAt(src []byte, offset int) (words []string, names bool) {
	tokens, _ := syntax.Lex(src)
	stack := []*frame{{scope: scopeFile, fresh: true}}
	// An attribute is skipped: attr is 1 after its `@`, 2 after its name,
	// and 3 inside its arguments, which attrDepth counts the parens of.
	attr, attrDepth := 0, 0
	end, typed := 0, offset // the end of the last token read, and where the word being typed starts
	for _, t := range tokens {
		if t.Kind == syntax.EOF || t.Span.Start >= offset {
			break
		}
		if t.Span.End >= offset && (t.Kind == syntax.Comment || isStringPart(t.Kind)) {
			if t.Kind != syntax.StringEnd || t.Span.End > offset {
				return nil, false // inside a comment or a string
			}
		}
		if t.Span.End == offset && isWord(t.Kind) || t.Span.End > offset && !t.Kind.IsTrivia() {
			typed = t.Span.Start // the word being typed, or a token the cursor is inside
			break
		}
		if t.Kind.IsTrivia() {
			continue
		}
		end = t.Span.End
		top := stack[len(stack)-1]
		if !top.paren && t.Flags&syntax.NewlineBefore != 0 && !continues(top.after) {
			top.fresh = true
		}
		if top.fresh {
			top.head, top.fresh = nil, false
		}
		switch {
		case attr == 1:
			attr = 2
			continue
		case attr == 2 && t.Kind == syntax.LParen:
			attr, attrDepth = 3, 1
			continue
		case attr == 3:
			switch t.Kind {
			case syntax.LParen:
				attrDepth++
			case syntax.RParen:
				attrDepth--
			}
			if attrDepth == 0 {
				attr = 0
			}
			continue
		}
		attr = 0
		switch t.Kind {
		case syntax.At:
			attr = 1
		case syntax.LBrace:
			stack = append(stack, &frame{scope: bodyScope(top), fresh: true})
		case syntax.InterpOpen:
			stack = append(stack, &frame{scope: scopeBlock, paren: true})
		case syntax.LParen, syntax.LBracket:
			stack = append(stack, &frame{scope: top.scope, paren: true})
		case syntax.RBrace, syntax.InterpClose, syntax.RParen, syntax.RBracket:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
			top = stack[len(stack)-1]
		case syntax.Semicolon:
			top.fresh = true
		default:
			if isWord(t.Kind) {
				top.head = append(top.head, string(src[t.Span.Start:t.Span.End]))
			} else {
				top.head = append(top.head, "")
			}
		}
		if t.Kind == syntax.RBrace && !top.paren {
			top.head = append(top.head, "}")
		}
		top.after = t.Kind
	}
	if attr == 1 || attr == 3 {
		return nil, false // an attribute's name or arguments
	}
	top := stack[len(stack)-1]
	if !top.paren && bytes.IndexByte(src[end:typed], '\n') >= 0 && !continues(top.after) {
		top.fresh = true
	}
	if top.paren {
		if top.scope == scopeBlock {
			return expressionStarts, true
		}
		return nil, false
	}
	head := top.head
	if top.fresh {
		head = nil
	}
	switch {
	case len(head) == 0:
		return itemStarts[top.scope], top.scope == scopeBlock
	case top.scope == scopeBlock:
		if len(head) == 1 && slices.Contains([]string{"let", "var", "for", "fn"}, head[0]) {
			return nil, false // a new name comes next
		}
		if head[len(head)-1] == "}" {
			return append([]string{"else"}, expressionStarts...), true
		}
		return expressionStarts, true
	case slices.Equal(head, []string{"pub"}):
		return afterPub[top.scope], false
	case top.scope != scopeFile:
		if top.scope == scopeEntity && slices.Equal(head, []string{"migrate"}) {
			return []string{"from"}, false
		}
	case slices.Equal(head, []string{"pub", "opaque"}):
		return []string{"type"}, false
	case slices.Equal(head, []string{"extern"}) || slices.Equal(head, []string{"pub", "extern"}):
		return []string{"fn", "type"}, false
	}
	return nil, false
}

// namesIn returns the names written in src, each once, leaving out the one
// that ends at offset, which is being typed.
func namesIn(src []byte, offset int) []string {
	tokens, _ := syntax.Lex(src)
	seen := map[string]bool{}
	var out []string
	for _, t := range tokens {
		if t.Kind != syntax.Ident && t.Kind != syntax.TypeIdent || t.Span.End == offset {
			continue
		}
		if name := string(src[t.Span.Start:t.Span.End]); !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// bodyScope is the scope of a `{` read in top: a body named by the line's
// first keyword at the declaration level, a block below it.
func bodyScope(top *frame) scope {
	switch top.scope {
	case scopeBlock:
		return scopeBlock
	case scopeOther, scopeContract:
		return scopeOther
	}
	for _, w := range top.head {
		switch w {
		case "entity":
			return scopeEntity
		case "trait":
			return scopeTrait
		case "impl":
			return scopeImpl
		case "contract":
			return scopeContract
		case "type", "import":
			return scopeOther
		case "fn", "job", "test", "migrate", "view", "let":
			return scopeBlock
		}
	}
	return scopeBlock
}

// continues reports whether a line ending in a token of kind k goes on to
// the next line, as `type T =` or `a ->` do.
func continues(k syntax.Kind) bool {
	switch k {
	case syntax.Assign, syntax.Arrow, syntax.FatArrow, syntax.Comma, syntax.Dot, syntax.Colon,
		syntax.PlusEq, syntax.MinusEq, syntax.StarEq, syntax.SlashEq, syntax.PercentEq:
		return true
	}
	return false
}

func isWord(k syntax.Kind) bool {
	return k == syntax.Ident || k == syntax.TypeIdent || k.IsKeyword()
}

func isStringPart(k syntax.Kind) bool {
	switch k {
	case syntax.StringStart, syntax.StringText, syntax.StringEscape, syntax.StringEnd:
		return true
	}
	return false
}

// More LSP completion item kinds.
const (
	completionMethod     = 2
	completionFunction   = 3
	completionField      = 5
	completionVariable   = 6
	completionClass      = 7
	completionInterface  = 8
	completionModule     = 9
	completionEnum       = 13
	completionEnumMember = 20
	completionStruct     = 22
	completionEvent      = 23
)

// nameList gathers completion items, each name once: the first one added
// wins, so a local hides a declaration of the same name.
type nameList struct {
	items []completionItem
	seen  map[string]bool
	rank  string // sorts locals before the module's names, and those before the prelude's
}

func (l *nameList) add(label string, kind int, target any) {
	if l.seen == nil {
		l.seen = map[string]bool{}
	}
	if label == "" || l.seen[label] {
		return
	}
	l.seen[label] = true
	item := itemFor(label, kind, target)
	item.SortText = l.rank + label
	l.items = append(l.items, item)
}

// itemFor describes a name as hover does: its signature's first line as the
// detail, and its doc comment.
func itemFor(label string, kind int, target any) completionItem {
	item := completionItem{Label: label, Kind: kind}
	sig, doc := "", ""
	switch t := target.(type) {
	case nil, check.Local:
	case ir.Builtin:
		for _, b := range ir.BuiltinDocs {
			if b.Name == t.Name {
				sig, doc = b.Signature, b.Doc
			}
		}
	case check.BuiltinType:
		for _, b := range ir.BuiltinTypeDocs {
			if b.Name == t.Name {
				sig, doc = b.Signature, b.Doc
			}
		}
	case *ir.Module:
		sig = "import " + t.Path
	default:
		if decl, src := declarationOf(target); decl != nil {
			sig, doc = signature(decl, src), syntax.DocComment(decl, src)
		}
		if v, ok := target.(*ir.Variant); ok {
			doc = v.Doc
		}
	}
	if line, _, _ := strings.Cut(sig, "\n"); line != "" {
		item.Detail = line
	}
	if doc != "" {
		item.Documentation = &markupString{Kind: "markdown", Value: doc}
	}
	return item
}

// declarationOf finds the syntax that declares target.
func declarationOf(target any) (*syntax.Node, []byte) {
	at, ok := check.Definition(target)
	if !ok || at.File == nil || at.File.Tree == nil {
		return nil, nil
	}
	return declarationAt(at.File.Tree, at.Offset), at.File.Src
}

// declaredWith reports whether target's declaration has the keyword word
// before its name, as `pub` or `opaque`.
func declaredWith(target any, word string) bool {
	if f, ok := target.(*ir.Func); ok && word == "pub" {
		return f.Pub
	}
	if g, ok := target.(*ir.Global); ok && word == "pub" {
		return g.Pub
	}
	decl, src := declarationOf(target)
	if decl == nil {
		return false
	}
	for _, c := range decl.Children {
		if c.IsToken() && c.Text(src) == word {
			return true
		}
	}
	return false
}

// scopeNames are the names that code at offset can use: its locals, then
// the module's declarations, its imports, the prelude's and the built-ins.
func scopeNames(res *check.Result, path string, d *document, offset int) []completionItem {
	l := &nameList{rank: "1"}
	for _, name := range localsAt(d.file.Root, d.text, offset) {
		l.add(name, completionVariable, nil)
	}
	l.rank = "2"
	if m := moduleOf(res.Program, path); m != nil {
		addModule(l, m, false)
	}
	l.rank = "3"
	for _, imp := range d.file.Root.Children {
		if imp.Kind != syntax.ImportDeclaration {
			continue
		}
		if p := imp.ChildByField(syntax.FieldPath); p != nil {
			name := p.Text(d.text)
			name = name[strings.LastIndexByte(name, '.')+1:]
			l.add(name, completionModule, res.Lookup(path, name))
		}
		for _, item := range imp.ChildrenByField(syntax.FieldItem) {
			name := item.Text(d.text)
			l.add(name, kindOf(res.Lookup(path, name)), res.Lookup(path, name))
		}
	}
	l.rank = "4"
	if res.Program.Prelude != nil {
		addModule(l, res.Program.Prelude, true)
	}
	for _, b := range ir.BuiltinDocs {
		if module, _, ok := strings.Cut(b.Name, "."); ok {
			if module[0] < 'A' || module[0] > 'Z' { // Int32.from is a type's
				l.add(module, completionModule, nil)
			}
			continue
		}
		l.add(b.Name, completionFunction, ir.Builtin{Name: b.Name})
	}
	for _, b := range ir.BuiltinTypeDocs {
		l.add(b.Name, completionStruct, check.BuiltinType{Name: b.Name})
	}
	return l.items
}

// members are what may follow the receiver recv and the `.` at dot. After a
// module, its public declarations; after a built-in module such as `log`,
// its functions; after a type's name, its variants and the methods called on
// the type itself. After a value, the fields and methods of its type, or the
// identity, state and methods of an entity.
func (s *server) members(d *document, recv syntax.Span, dot int) []completionItem {
	l := &nameList{rank: "1"}
	res, path, ok := s.checkedBefore(d, dot)
	if closers := unclosed(d.text, dot); !ok && closers != "" {
		// `print(x.` doesn't parse even without the `.`: close what the line
		// left open, after the name being typed.
		end := dot + 1
		for end < len(d.text) && isIdentByte(d.text[end]) {
			end++
		}
		text := slices.Concat(d.text[:end], []byte(closers), d.text[end:])
		res, path, ok = s.checkedBefore(newDocument(d.uri, d.version, text), dot)
	}
	if !ok {
		return []completionItem{}
	}
	var named any // what the receiver names, when it is a name
	if isName(d.text[recv.Start:recv.End]) {
		if use, ok := res.Index.At(path, recv.Start); ok {
			named = use.Target
		} else {
			named = res.Lookup(path, string(d.text[recv.Start:recv.End]))
		}
	}
	switch t := named.(type) {
	case *ir.Module:
		addModule(l, t, true)
	case ir.Builtin:
		for _, b := range ir.BuiltinDocs {
			if member, ok := strings.CutPrefix(b.Name, t.Name+"."); ok {
				l.add(member, completionFunction, ir.Builtin{Name: b.Name})
			}
		}
	case check.BuiltinType:
		// Functions called on a built-in type: Int32.from.
		for _, b := range ir.BuiltinDocs {
			if member, ok := strings.CutPrefix(b.Name, t.Name+"."); ok {
				l.add(member, completionFunction, ir.Builtin{Name: b.Name})
			}
		}
	case *ir.Type:
		for _, name := range slices.Sorted(maps.Keys(t.Methods)) {
			if fn := t.Methods[name]; !fn.HasSelf {
				l.add(name, completionFunction, fn)
			}
		}
		for _, v := range t.Variants {
			l.add(v.Name, completionEnumMember, v)
		}
	default:
		switch t := typeOf(res, path, dot).(type) {
		case *ir.Type:
			fields := t.Fields
			if len(t.Variants) == 1 {
				for _, f := range t.Variants[0].Fields {
					fields = append(fields, f.Name)
				}
			}
			for _, f := range fields {
				l.add(f, completionField, check.Member{Of: t, Name: f})
			}
			addMethods(l, t.Methods)
		case *ir.Entity:
			if t.ID != nil {
				l.add(t.ID.Name, completionField, check.Member{Of: t, Name: t.ID.Name})
			}
			for _, st := range t.States {
				l.add(st.Name, completionField, check.Member{Of: t, Name: st.Name})
			}
			addMethods(l, t.Methods)
		}
		// Then the functions in scope that take the value first, which
		// x.f(…) calls on it (SPECS.md §8); a method or field of the same
		// name, added above, is what the call means.
		l.rank = "2"
		funcs := res.FuncsOn(path, dot)
		for _, name := range slices.Sorted(maps.Keys(funcs)) {
			l.add(name, completionFunction, funcs[name])
		}
	}
	if l.items == nil {
		return []completionItem{}
	}
	return l.items
}

// unclosed returns what closes the parentheses and brackets that the line
// before offset opens and leaves open, innermost first: ")" for `print(x`,
// "])" for `f([a`.
func unclosed(src []byte, offset int) string {
	start := bytes.LastIndexByte(src[:offset], '\n') + 1
	tokens, _ := syntax.Lex(src[start:offset])
	var open []byte
	for _, t := range tokens {
		switch t.Kind {
		case syntax.LParen:
			open = append(open, ')')
		case syntax.LBracket:
			open = append(open, ']')
		case syntax.RParen, syntax.RBracket:
			if len(open) > 0 {
				open = open[:len(open)-1]
			}
		}
	}
	slices.Reverse(open)
	return string(open)
}

// typeOf is the declaration of the type of the expression before the `.` at
// dot, or nil when the checker doesn't know it.
func typeOf(res *check.Result, path string, dot int) any {
	t, ok := res.Index.TypeAt(path, dot)
	if !ok {
		return nil
	}
	return t
}

// addMethods adds the methods called on a value: those that take self, and
// an entity's, which all do.
func addMethods(l *nameList, methods map[string]*ir.Func) {
	for _, name := range slices.Sorted(maps.Keys(methods)) {
		if fn := methods[name]; fn.HasSelf || fn.Entity != nil {
			l.add(name, completionMethod, fn)
		}
	}
}

func isName(b []byte) bool {
	for _, c := range b {
		if !isIdentByte(c) {
			return false
		}
	}
	return len(b) > 0
}

// addModule adds a module's declarations; with onlyPub, those another module
// can use.
func addModule(l *nameList, m *ir.Module, onlyPub bool) {
	visible := func(target any) bool { return !onlyPub || declaredWith(target, "pub") }
	for _, name := range slices.Sorted(maps.Keys(m.Funcs)) {
		if f := m.Funcs[name]; visible(f) {
			l.add(name, completionFunction, f)
		}
	}
	for _, g := range m.Globals {
		if visible(g) {
			l.add(g.Name, completionVariable, g)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(m.Types)) {
		t := m.Types[name]
		if !visible(t) {
			continue
		}
		l.add(name, kindOf(t), t)
		if onlyPub && declaredWith(t, "opaque") {
			continue
		}
		for _, v := range t.Variants {
			l.add(v.Name, completionEnumMember, v)
		}
	}
	addAll(l, m.Entities, visible, completionClass)
	addAll(l, m.Jobs, visible, completionFunction)
	addAll(l, m.Views, visible, completionVariable)
	addAll(l, m.Contracts, visible, completionInterface)
	addAll(l, m.Traits, visible, completionInterface)
	// A module that holds one contract offers its operations and events as
	// its own: `notify.send!(…)`, `notify.answered`.
	if len(m.Contracts) == 1 {
		for _, ct := range m.Contracts {
			for _, name := range slices.Sorted(maps.Keys(ct.Ops)) {
				l.add(name, completionFunction, ct.Ops[name])
			}
			for _, name := range slices.Sorted(maps.Keys(ct.Events)) {
				l.add(name, completionEvent, ct.Events[name])
			}
		}
	}
}

func addAll[T any](l *nameList, decls map[string]T, visible func(any) bool, kind int) {
	for _, name := range slices.Sorted(maps.Keys(decls)) {
		if d := decls[name]; visible(d) {
			l.add(name, kind, d)
		}
	}
}

// kindOf is the completion kind of a declaration.
func kindOf(target any) int {
	switch t := target.(type) {
	case *ir.Func, *ir.Job:
		return completionFunction
	case *ir.Type:
		if len(t.Variants) > 1 {
			return completionEnum
		}
		return completionStruct
	case *ir.Variant:
		return completionEnumMember
	case *ir.Entity:
		return completionClass
	case *ir.Contract, *ir.Trait:
		return completionInterface
	case *ir.Module:
		return completionModule
	}
	return completionVariable
}

// moduleOf finds the module that the file at path belongs to.
func moduleOf(prog *ir.Program, path string) *ir.Module {
	for _, m := range prog.Modules {
		for _, f := range m.Files {
			if f.Path == path {
				return m
			}
		}
	}
	return nil
}

// localsAt returns the locals in scope at offset, innermost first: the
// parameters of the functions and lambdas around it, the bindings of the
// statements before it in each block, the pattern of a `for` or match arm it
// is in, and the state of the entity it is in.
func localsAt(root *syntax.Node, src []byte, offset int) []string {
	var outer [][]string // the names each enclosing node binds, outermost first
	for n := root; n != nil; {
		var next *syntax.Node
		for _, c := range n.Children {
			if !c.IsToken() && c.Span.Start < offset && offset <= c.Span.End {
				next = c
				break
			}
			// An arm still being typed, `Some(v) => `, ends before the cursor.
			if c.Kind == syntax.MatchArm && c.Span.End < offset && bytes.IndexByte(src[c.Span.End:offset], '\n') < 0 {
				next = c
			}
		}
		var names []string
		switch n.Kind {
		case syntax.FunctionDeclaration, syntax.JobDeclaration, syntax.Lambda, syntax.Migration:
			if ps := n.ChildByField(syntax.FieldParameters); ps != nil {
				for _, p := range ps.Children {
					if p.Kind == syntax.Parameter {
						binders(p.ChildByField(syntax.FieldName), src, &names)
					}
				}
			}
			binders(n.ChildByField(syntax.FieldParameter), src, &names)
		case syntax.EntityDeclaration:
			if ps := n.ChildByField(syntax.FieldParameters); ps != nil {
				for _, p := range ps.Children {
					if p.Kind == syntax.Parameter {
						binders(p.ChildByField(syntax.FieldName), src, &names)
					}
				}
			}
			if body := n.ChildByField(syntax.FieldBody); body != nil {
				for _, st := range body.Children {
					if st.Kind == syntax.StateDeclaration {
						binders(st.ChildByField(syntax.FieldName), src, &names)
					}
				}
			}
		case syntax.Block:
			for _, st := range n.Children {
				if st == next || st.Span.End > offset {
					break
				}
				switch st.Kind {
				case syntax.LetStatement:
					binders(st.ChildByField(syntax.FieldPattern), src, &names)
				case syntax.VarStatement:
					binders(st.ChildByField(syntax.FieldName), src, &names)
				}
			}
			slices.Reverse(names) // a later binding hides an earlier one
		case syntax.ForStatement:
			if body := n.ChildByField(syntax.FieldBody); body != nil && body.Span.Start < offset {
				binders(n.ChildByField(syntax.FieldPattern), src, &names)
			}
		case syntax.MatchArm:
			if pat := n.ChildByField(syntax.FieldPattern); pat != nil && pat.Span.End < offset {
				binders(pat, src, &names)
			}
		}
		outer = append(outer, names)
		n = next
	}
	var out []string
	for _, names := range slices.Backward(outer) {
		out = append(out, names...)
	}
	return out
}

// binders adds the names a pattern binds: its identifiers, leaving out the
// field names of `{name: pattern}`.
func binders(n *syntax.Node, src []byte, out *[]string) {
	if n == nil {
		return
	}
	if n.Kind == syntax.Ident {
		*out = append(*out, n.Text(src))
		return
	}
	for _, c := range n.Children {
		if n.Kind == syntax.NamedFieldPattern && c.Field == syntax.FieldName {
			continue
		}
		binders(c, src, out)
	}
}

// receiverAt finds the `.` that the cursor at offset follows, as in
// `notify.`, `notify.se` or `xs.first().`, outside comments and strings. It
// returns the last token of the receiver before it and the `.`'s offset.
func receiverAt(src []byte, offset int) (recv syntax.Span, dot int, ok bool) {
	tokens, _ := syntax.Lex(src)
	var sig []syntax.Token
	for _, t := range tokens {
		if t.Kind == syntax.EOF || t.Span.Start >= offset {
			break
		}
		if t.Span.End >= offset && (t.Kind == syntax.Comment || isStringPart(t.Kind)) {
			if t.Kind != syntax.StringEnd || t.Span.End > offset {
				return syntax.Span{}, 0, false
			}
		}
		if !t.Kind.IsTrivia() {
			sig = append(sig, t)
		}
	}
	if n := len(sig); n > 0 && isWord(sig[n-1].Kind) && sig[n-1].Span.End >= offset {
		sig = sig[:n-1] // the word being typed
	}
	n := len(sig)
	if n < 2 || sig[n-1].Kind != syntax.Dot {
		return syntax.Span{}, 0, false
	}
	switch r := sig[n-2]; r.Kind {
	case syntax.Ident, syntax.TypeIdent, syntax.RParen, syntax.RBracket:
		if r.Span.End == sig[n-1].Span.Start {
			return r.Span, sig[n-1].Span.Start, true
		}
	}
	return syntax.Span{}, 0, false
}
