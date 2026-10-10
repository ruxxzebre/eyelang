package lsp

import (
	"os"
	"sort"
	"strings"
	"time"
	"unicode"

	"eyelang/check"
	"eyelang/syntax"
)

// keyword describes a keyword or an operator for an editor: what it means
// in a sentence, the section of SPECS.md that defines it, and, where the
// construct it belongs to isn't the best place, where eye implements it.
type keyword struct {
	summary string
	spec    string         // the start of the SPECS.md heading, as "## 17."
	impl    []check.GoText // the checker's code for it; the first is its definition
}

// keywords are the words and operators of the language. Where impl is
// empty, the implementation is found from the construct the token belongs
// to: `contract` belongs to a contract_declaration, which the checker reads
// where it handles syntax.ContractDeclaration.
var keywords = map[string]keyword{
	"let":      {summary: "Binds a name to a value that doesn't change. A pattern can take the value apart, and `else` handles a value that doesn't match.", spec: "## 6."},
	"var":      {summary: "Declares a variable that can change, by assignment or by passing it as `mut`.", spec: "## 7."},
	"mut":      {summary: "Marks a parameter the function may change, and the argument passed for it: `f(mut x)`. The change is written back to the caller's variable.", spec: "## 7.", impl: []check.GoText{{File: "check/lower.go", Text: `hasToken(f, p, "mut")`}}},
	"fn":       {summary: "Declares a function, or writes a lambda: `fn x: x + 1`.", spec: "## 6."},
	"pub":      {summary: "Makes a declaration visible to the modules that import this one. Without it, a declaration is private to its module.", spec: "## 13.", impl: []check.GoText{{File: "check/check.go", Text: `hasToken(f, d, "pub")`}}},
	"opaque":   {summary: "Exports a type but not its fields: other modules can name the type but can't build or take apart its values.", spec: "## 5.", impl: []check.GoText{{File: "syntax/parse_decl.go", Text: "opaque = p.bumpAs(Keyword)"}}},
	"return":   {summary: "Leaves the function with a value. A block's last expression is its value, so `return` is only needed to leave early.", spec: "## 6."},
	"if":       {summary: "Chooses between two branches. It is an expression: `if c then a else b` has a value.", spec: "## 6."},
	"then":     {summary: "Separates an `if`'s condition from a branch that is one expression: `if c then a else b`.", spec: "## 6."},
	"else":     {summary: "The branch an `if` takes when the condition is false, or what a `let` does when its pattern doesn't match.", spec: "## 6."},
	"match":    {summary: "Chooses a branch by the shape of a value. The checker makes sure every case is covered.", spec: "## 9."},
	"for":      {summary: "Runs a block for each element of a collection: `for x in xs { … }`.", spec: "## 6."},
	"in":       {summary: "Separates a `for` loop's pattern from the collection it walks.", spec: "## 6."},
	"while":    {summary: "Runs a block for as long as a condition holds.", spec: "## 6."},
	"break":    {summary: "Leaves the innermost loop.", spec: "## 6."},
	"continue": {summary: "Goes on to the next round of the innermost loop.", spec: "## 6."},
	"true":     {summary: "The Bool that holds.", spec: "## 5."},
	"false":    {summary: "The Bool that doesn't hold.", spec: "## 5."},
	"import":   {summary: "Uses another module: `import lab.notify` names the directory lab/notify below the program's root, and `import m.{a, B}` brings names in unqualified.", spec: "## 13.", impl: []check.GoText{{File: "check/check.go", Text: "func (c *checker) importDecl("}}},
	"type":     {summary: "Declares a type: a record `{ … }`, variants `| A | B(x: Int)`, or an alias.", spec: "## 5.", impl: []check.GoText{{File: "check/check.go", Text: "func (c *checker) typeDecl("}}},
	"trait":    {summary: "Declares a set of methods that types can implement; a bound `where t: Trait` asks for them.", spec: "## 11.", impl: []check.GoText{{File: "check/traits.go", Text: "func (c *checker) traitDecl("}}},
	"impl":     {summary: "Gives a type its methods, implements a trait for a type, or serves a contract's operations.", spec: "## 8.", impl: []check.GoText{{File: "check/traits.go", Text: "func (c *checker) implDecl("}}},
	"where":    {summary: "States the traits a type variable must implement: `where a: Ord`.", spec: "## 11."},
	"dyn":      {summary: "A value whose type is known only at run time, through a trait: the method it calls is looked up then.", spec: "## 11."},
	"entity":   {summary: "Declares an entity: shared state with an identity, changed only by its own methods, each one a transaction.", spec: "## 14.", impl: []check.GoText{{File: "check/check.go", Text: "func (c *checker) entityDecl("}}},
	"state":    {summary: "A field of an entity's state, kept by the runtime and changed only by the entity's methods.", spec: "## 14."},
	"migrate":  {summary: "Says how an entity's state of an earlier version becomes the current one.", spec: "## 14."},
	"from":     {summary: "Names the version a migration starts from.", spec: "## 14."},
	"job":      {summary: "Declares a job: a durable function that can wait, for minutes or weeks, at its `!` steps and survives restarts.", spec: "## 15."},
	"view":     {summary: "Declares a view: a pipeline over entities that the runtime keeps up to date, so jobs can start when an entity enters or leaves it.", spec: "## 16."},
	"contract": {summary: "Describes another service: its commands, queries and events, and how they go over HTTP. Calls to it are typed, keyed and reconciled.", spec: "## 17.", impl: []check.GoText{{File: "check/check.go", Text: "func (c *checker) contractDecl("}}},
	"command":  {summary: "An operation of a contract that changes something in the other service. With @key it can be sent again safely.", spec: "## 17."},
	"query":    {summary: "An operation of a contract that only reads from the other service.", spec: "## 17."},
	"event":    {summary: "Something a contract's service publishes, which jobs can wait for or be started by.", spec: "## 17."},
	"on":       {summary: "Names the topic an event is published on.", spec: "## 17."},
	"extern":   {summary: "Declares a function that is implemented outside Eyelang: in C, in Go, or by the interpreter (@host).", spec: "## 18."},
	"uses":     {summary: "Lists the effects a function may have, such as net or clock. Public functions state theirs; inside, they are inferred.", spec: "## 12.", impl: []check.GoText{{File: "check/lower.go", Text: "func usesOf("}}},
	"with":     {summary: "In a test, applies a handler to the rest of the block: `with clock(…)`, `with fakes(…)`.", spec: "## 20."},
	"test":     {summary: "Declares a test. Tests run deterministically: the clock is fake and services are replayed from recordings.", spec: "## 20."},
	"_":        {summary: "A placeholder: a pattern that matches anything and binds nothing, or where a pipe puts its value: `x |> f(a, _)`.", spec: "## 9."},

	"!":   {summary: "Marks a durable step in a job: `sleep!(20 min)`, `notify.send!(…)`. The run may pause here, and the result is saved. Before an expression, `!x` is not.", spec: "## 15."},
	"?":   {summary: "Returns early with an `Err` or `None`, wrapping the error into the function's own error type; otherwise gives the value inside.", spec: "## 10."},
	"??":  {summary: "The value inside an `Option` or `Result`, or the default on the right when there is none.", spec: "## 10.", impl: []check.GoText{{File: "check/lower.go", Text: `case "??":`}}},
	"|>":  {summary: "Pipes a value into a call: `x |> f(a)` is `f(x, a)`, and `_` puts it elsewhere.", spec: "## 6.", impl: []check.GoText{{File: "check/lower.go", Text: "func (l *lowerer) pipe("}}},
	"=>":  {summary: "Separates a match arm's pattern from its value.", spec: "## 9."},
	"->":  {summary: "Separates a function's parameters from its return type.", spec: "## 6."},
	"@":   {summary: "Starts an attribute, such as @on(…) or @key: checked against its declaration, like a call.", spec: "## 2.", impl: []check.GoText{{File: "check/attributes.go", Text: "var attributes ="}}},
	"..":  {summary: "A range that leaves out its end, `0..n`; in a pattern, the rest of a list or record.", spec: "## 5."},
	"..=": {summary: "A range that includes its end: `1..=10`.", spec: "## 5."},
}

// keywordDoc returns what the keyword or operator word means, and the
// section of SPECS.md that defines it.
func (s *server) keywordDoc(word string) (string, location, bool) {
	k, ok := keywords[word]
	if !ok {
		return "", location{}, false
	}
	loc, _ := s.specLocation(k.spec)
	return k.summary, loc, true
}

// specLocation finds the heading of SPECS.md that starts with heading.
func (s *server) specLocation(heading string) (location, bool) {
	if heading == "" {
		return location{}, false
	}
	return s.findText("SPECS.md", heading, false, true)
}

// keywordAt returns the keyword or operator token at offset, and the node it
// belongs to.
func keywordAt(d *document, offset int) (tok, parent *syntax.Node) {
	var walk func(n *syntax.Node) bool
	walk = func(n *syntax.Node) bool {
		for _, c := range n.Children {
			if offset < c.Span.Start || offset > c.Span.End {
				continue
			}
			if c.IsToken() {
				if offset == c.Span.End && c.Span.End > c.Span.Start && !isWordEnd(d, c) {
					continue // the cursor is after it, at the next token
				}
				if c.Kind == syntax.Keyword || c.Kind.IsKeyword() || isOperator(c.Kind) {
					tok, parent = c, n
					return true
				}
				continue
			}
			if walk(c) {
				return true
			}
		}
		return false
	}
	walk(d.file.Root)
	return tok, parent
}

// isWordEnd reports whether the token ends where a word does, so a cursor
// just after it still means it.
func isWordEnd(d *document, n *syntax.Node) bool {
	if n.Span.End >= len(d.text) {
		return true
	}
	r := rune(d.text[n.Span.End])
	return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
}

// isOperator reports whether a token is punctuation or an operator rather
// than a name, a literal or a part of a string.
func isOperator(k syntax.Kind) bool {
	return k.IsToken() && k >= syntax.LParen && !k.IsKeyword() && k != syntax.Keyword
}

// keywordDefinition is where eye gives the keyword or operator at offset
// its meaning: the checker's code for it.
func (s *server) keywordDefinition(d *document, offset int) (location, bool) {
	locs := s.keywordImplementations(d, offset)
	if len(locs) == 0 {
		return location{}, false
	}
	return locs[0], true
}

// keywordImplementations are the places that implement the keyword or
// operator at offset, the checker's first: the checker, the interpreter,
// the Go parser and the grammar.
func (s *server) keywordImplementations(d *document, offset int) []location {
	tok, parent := keywordAt(d, offset)
	if tok == nil {
		return nil
	}
	word := tok.Text(d.text)
	var out []location
	seen := map[location]bool{}
	add := func(loc location, ok bool) {
		if ok && !seen[loc] {
			seen[loc] = true
			out = append(out, loc)
		}
	}
	for _, g := range keywords[word].impl {
		add(s.goTextLocation(g))
	}
	kind := goName(parent.Kind.String())
	for _, f := range s.sourceFiles("check", "check.go", "lower.go") {
		if loc, ok := s.findText(f, "syntax."+kind, true, false); ok {
			add(loc, true)
			break
		}
	}
	if irName, ok := irNodes[parent.Kind]; ok {
		for _, f := range s.sourceFiles("interp", "eval.go", "jobs.go", "services.go") {
			if loc, ok := s.findText(f, "*ir."+irName, true, false); ok {
				add(loc, true)
				break
			}
		}
	}
	for _, f := range s.sourceFiles("syntax", "parse_decl.go", "parse_expr.go", "parse_stmt.go", "parse_type.go", "parser.go") {
		if strings.HasSuffix(f, "_gen.go") {
			continue
		}
		if loc, ok := s.findText(f, kind, true, false); ok {
			add(loc, true)
			break
		}
	}
	add(s.findText("tree-sitter-eyelang/grammar.js", parent.Kind.String()+": $ =>", false, false))
	return out
}

// irNodes are the IR nodes the interpreter runs for each construct.
var irNodes = map[syntax.Kind]string{
	syntax.ContractDeclaration: "Contract", syntax.CommandDeclaration: "Op", syntax.QueryDeclaration: "Op",
	syntax.EventDeclaration: "Event", syntax.JobDeclaration: "Job", syntax.EntityDeclaration: "Entity",
	syntax.ViewDeclaration: "View", syntax.TestDeclaration: "Test", syntax.WaitingCallExpression: "Waiting",
	syntax.TryExpression: "Try", syntax.MatchExpression: "Match", syntax.IfExpression: "If",
	syntax.ForStatement: "For", syntax.WhileStatement: "While", syntax.ReturnStatement: "Return",
	syntax.BreakStatement: "Break", syntax.ContinueStatement: "Continue", syntax.LetStatement: "Let",
	syntax.VarStatement: "Let", syntax.Lambda: "Lambda", syntax.WithStatement: "With",
	syntax.BinaryExpression: "Binary", syntax.UnaryExpression: "Unary", syntax.RangeExpression: "Range",
	syntax.Assignment: "Assign", syntax.Record: "Record", syntax.RecordUpdate: "RecordUpdate",
	syntax.List: "List", syntax.Map: "Map", syntax.IndexExpression: "Index",
}

// goName turns a node name into its Go constant: contract_declaration is
// ContractDeclaration.
func goName(node string) string {
	var b strings.Builder
	for _, part := range strings.Split(node, "_") {
		if part != "" {
			b.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
	}
	return b.String()
}

// sourceFiles lists the Go files of one of eye's packages, by path in the
// repository, the given ones first and tests left out.
func (s *server) sourceFiles(pkg string, first ...string) []string {
	var out []string
	seen := map[string]bool{}
	for _, f := range first {
		out = append(out, pkg+"/"+f)
		seen[f] = true
	}
	dir, ok := s.sourceFile(check.GoFile(pkg))
	if !ok {
		return out
	}
	entries, _ := os.ReadDir(dir)
	var rest []string
	for _, e := range entries {
		n := e.Name()
		if strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") && !seen[n] && n != "index.go" {
			rest = append(rest, pkg+"/"+n)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

// findText finds the first line of a file of eye's source, by its path in
// the repository, that holds text: as a whole word when word is set, and at
// the start of the line when start is.
func (s *server) findText(rel, text string, word, start bool) (location, bool) {
	path, ok := s.sourceFile(check.GoFile(rel))
	if !ok {
		return location{}, false
	}
	lines, ok := s.lines(path)
	if !ok {
		return location{}, false
	}
	for i, line := range lines {
		c := strings.Index(line, text)
		for c >= 0 && word && c+len(text) < len(line) && isIdentByte(line[c+len(text)]) {
			next := strings.Index(line[c+1:], text)
			if next < 0 {
				c = -1
				break
			}
			c += 1 + next
		}
		if c < 0 || start && c != 0 {
			continue
		}
		at := Position{Line: i, Character: utf16Len([]byte(line[:c]))}
		end := Position{Line: i, Character: at.Character + utf16Len([]byte(text))}
		return location{URI: pathURI(path), Range: Range{Start: at, End: end}}, true
	}
	return location{}, false
}

func isIdentByte(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

// goSites are the lines of the checker's and the interpreter's Go that name
// something the prelude or a standard module declares, in quotes:
// `Kind: "Unavailable"`, `Variants["Uncertain"]`. That is where the
// runtime makes or reads it.
func (s *server) goSites(name string) []location {
	var out []location
	quoted := `"` + name + `"`
	for _, pkg := range []string{"interp", "check"} {
		for _, rel := range s.sourceFiles(pkg) {
			path, ok := s.sourceFile(check.GoFile(rel))
			if !ok {
				continue
			}
			lines, ok := s.lines(path)
			if !ok {
				continue
			}
			for i, line := range lines {
				if c := strings.Index(line, quoted); c >= 0 && !strings.HasPrefix(strings.TrimSpace(line), "//") {
					at := Position{Line: i, Character: utf16Len([]byte(line[:c]))}
					end := Position{Line: i, Character: at.Character + utf16Len([]byte(quoted))}
					out = append(out, location{URI: pathURI(path), Range: Range{Start: at, End: end}})
				}
			}
		}
	}
	return out
}

// sourceText is a file of eye's source as last read.
type sourceText struct {
	modified time.Time
	lines    []string
}

// lines returns the lines of a file of eye's source, read again only when
// the file has changed: finding a keyword's places reads the same files
// many times.
func (s *server) lines(path string) ([]string, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	if t, ok := s.sources[path]; ok && t.modified.Equal(info.ModTime()) {
		return t.lines, true
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	if s.sources == nil {
		s.sources = map[string]sourceText{}
	}
	t := sourceText{modified: info.ModTime(), lines: strings.Split(string(src), "\n")}
	s.sources[path] = t
	return t.lines, true
}
