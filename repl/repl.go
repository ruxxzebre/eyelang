// Package repl is Eyelang's interactive session: `eye repl` reads inputs,
// checks them with the rest of the session, runs them, and prints what they
// made, with its type.
//
// The session is a source file in the program's root module. Declarations
// typed at the prompt accumulate in it; a binding (`let`, `var`) becomes a
// module-level value read through a host function, so later inputs and
// declarations see it without computing it again; and each other input runs
// as the body of a function the session checks and calls. Every input checks
// the whole session again, so errors are the compiler's own, at the place in
// the input where they are.
package repl

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"eyelang/check"
	"eyelang/interp"
	"eyelang/ir"
	"eyelang/syntax"
)

// sessionPath is where the session's source is, as far as the checker
// knows: a file of the program's root module that exists only in memory.
func (r *REPL) sessionPath() string { return filepath.Join(r.dir, ".eye-repl-session.eye") }

// check checks the program with src as the session.
func (r *REPL) check(src string) (*ir.Program, []check.Diagnostic) {
	res := check.Check(r.dir, check.Options{Overlay: map[string][]byte{r.sessionPath(): []byte(src)}})
	return res.Program, res.Diagnostics
}

// REPL is one interactive session.
type REPL struct {
	dir   string // the program the session runs in
	temp  bool   // dir was made for the session
	out   io.Writer
	decls []decl
	binds []binding
	sess  *interp.Session
	prog  *ir.Program
}

type decl struct {
	names []string
	src   string
}

type binding struct {
	name    string
	typ     *ir.TypeExpr // nil when not known
	mutable bool
}

// New starts a session in the program in dir, whose modules the session can
// use; with dir "", in an empty program.
func New(dir string, out io.Writer) (*REPL, error) {
	r := &REPL{dir: dir, out: out}
	if dir == "" {
		d, err := os.MkdirTemp("", "eye-repl-")
		if err != nil {
			return nil, err
		}
		r.dir, r.temp = d, true
	}
	r.sess = interp.NewSession(out, out, time.Now().Truncate(time.Second))
	if diags := r.load(""); len(diags) > 0 {
		for _, d := range diags {
			fmt.Fprintln(out, d)
		}
		return nil, fmt.Errorf("the program in %s has errors", dir)
	}
	return r, nil
}

// Close removes what the session made on disk.
func (r *REPL) Close() {
	if r.temp {
		_ = os.RemoveAll(r.dir)
	}
}

// Prompt is the prompt for a new input, and Continue for a further line of
// an unfinished one.
const (
	Prompt   = "eye> "
	Continue = "...  "
)

// Unfinished reports whether an input needs more lines: a bracket, brace or
// parenthesis is still open, a multi-line string isn't closed, or the last
// line ends with an operator.
func Unfinished(input string) bool {
	if strings.Count(input, `"""`)%2 == 1 {
		return true // a multi-line string isn't closed
	}
	toks, _ := syntax.Lex([]byte(input))
	depth := 0
	var last syntax.Token
	for _, t := range toks {
		switch t.Kind {
		case syntax.LParen, syntax.LBrace, syntax.LBracket, syntax.InterpOpen:
			depth++
		case syntax.RParen, syntax.RBrace, syntax.RBracket, syntax.InterpClose:
			depth--
		}
		switch t.Kind {
		case syntax.Whitespace, syntax.Comment, syntax.EOF:
		default:
			last = t
		}
	}
	if depth > 0 {
		return true
	}
	switch strings.TrimSpace(input[last.Span.Start:last.Span.End]) {
	case "|>", "=", "+", "-", "*", "/", "&&", "||", "??", ",", "->", "=>", ".":
		return true
	}
	return false
}

// Eval runs one input and prints what it made, or what is wrong with it.
func (r *REPL) Eval(input string) (quit bool) {
	text := strings.TrimSpace(input)
	switch {
	case text == "":
		return false
	case strings.HasPrefix(text, ":"):
		return r.command(text)
	}
	if names, ok := declaration(text); ok {
		r.declare(text, names)
		return false
	}
	r.run(text, false)
	return false
}

// declaration reports whether an input is declarations (not `let`), and the
// names it declares.
func declaration(text string) ([]string, bool) {
	tree := syntax.Parse([]byte(text))
	if len(tree.Diagnostics) > 0 {
		return nil, false
	}
	var names []string
	items := 0
	for _, d := range tree.Root.NamedChildren() {
		switch d.Kind {
		case syntax.EOF:
			continue
		case syntax.LetStatement:
			return nil, false
		}
		items++
		if n := d.ChildByField(syntax.FieldName); n != nil {
			names = append(names, n.Text([]byte(text)))
		}
	}
	return names, items > 0
}

// ---------------------------------------------------------------- the session's source

// source writes the session: its declarations, a new one being tried, the
// bindings, and body as the function __input.
func (r *REPL) source(newDecl, body string) (src string, at int) {
	var b strings.Builder
	for _, d := range r.decls {
		b.WriteString(d.src)
		b.WriteString("\n\n")
	}
	if newDecl != "" {
		at = b.Len()
		b.WriteString(newDecl)
		b.WriteString("\n\n")
	}
	b.WriteString("@host(\"repl.value\")\nextern fn __value(name: String) -> a\n\n")
	b.WriteString("@host(\"repl.save\")\nextern fn __save(name: String, value: a) -> a\n\n")
	for _, x := range r.binds {
		if t := annotation(x.typ); t != "" {
			fmt.Fprintf(&b, "let %s: %s = __value(%q)\n", x.name, t, x.name)
		} else {
			fmt.Fprintf(&b, "let %s = __value(%q)\n", x.name, x.name)
		}
	}
	b.WriteString("\nfn __input() {\n")
	for _, x := range r.binds {
		if !x.mutable {
			continue
		}
		if t := annotation(x.typ); t != "" {
			fmt.Fprintf(&b, "var %s: %s = __value(%q)\n", x.name, t, x.name)
		} else {
			fmt.Fprintf(&b, "var %s = __value(%q)\n", x.name, x.name)
		}
	}
	if body != "" {
		at = b.Len()
		b.WriteString(body)
		b.WriteString("\n")
	}
	b.WriteString("()\n}\n")
	return b.String(), at
}

// load checks the session with src as its source, and on success makes the
// result the session's program.
func (r *REPL) load(src string) []check.Diagnostic {
	if src == "" {
		src, _ = r.source("", "")
	}
	prog, diags := r.check(src)
	if len(diags) > 0 {
		return diags
	}
	r.prog = prog
	if err := r.sess.Load(prog); err != nil {
		fmt.Fprintln(r.out, err)
	}
	return nil
}

// report prints diagnostics, those in the input at their line and column in
// it, under the line they are on.
func (r *REPL) report(diags []check.Diagnostic, input string, at int, toInput func(int) int) {
	lines := strings.Split(input, "\n")
	for _, d := range diags {
		if d.Pos.File == nil || d.Pos.File.Path != r.sessionPath() {
			fmt.Fprintln(r.out, d)
			continue
		}
		off := d.Pos.Offset - at
		if toInput != nil && off >= 0 {
			off = toInput(off)
		}
		if off < 0 || off > len(input) {
			fmt.Fprintf(r.out, "in the session: %s\n", d.Message)
			continue
		}
		line := strings.Count(input[:off], "\n")
		col := off - (strings.LastIndex(input[:off], "\n") + 1)
		if len(lines) > 1 {
			fmt.Fprintf(r.out, "%d:%d: %s\n", line+1, col+1, d.Message)
		} else {
			fmt.Fprintf(r.out, "%s\n", d.Message)
		}
		fmt.Fprintf(r.out, "  %s\n  %s^\n", lines[line], strings.Repeat(" ", col))
	}
}

// ---------------------------------------------------------------- declarations

func (r *REPL) declare(text string, names []string) {
	// A name declared again replaces what it named.
	old := r.decls
	var kept []decl
	for _, d := range r.decls {
		if !overlaps(d.names, names) {
			kept = append(kept, d)
		}
	}
	r.decls = kept
	src, at := r.source(text, "")
	if diags := r.load(src); len(diags) > 0 {
		r.decls = old
		r.report(diags, text, at, nil)
		return
	}
	r.decls = append(r.decls, decl{names: names, src: text})
	if len(names) > 0 {
		fmt.Fprintf(r.out, "defined %s\n", strings.Join(names, ", "))
	}
}

func overlaps(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------- inputs that run

// run checks text as the body of a function and, unless onlyType, calls it.
// A let or var it ends with becomes a binding; an expression it ends with is
// printed with its type.
func (r *REPL) run(text string, onlyType bool) {
	body, newNames, mutable, shown, toInput := r.shape(text)
	src, at := r.source("", body)
	prog, diags := r.check(src)
	if len(diags) > 0 {
		r.report(diags, text, at, toInput)
		return
	}
	fn := prog.Root.Funcs["__input"]
	types := savedTypes(fn)
	if onlyType {
		fmt.Fprintln(r.out, display(types["it"]))
		return
	}
	r.prog = prog
	if err := r.sess.Load(prog); err != nil {
		fmt.Fprintln(r.out, err)
	}
	if _, err := r.sess.Call(fn); err != nil {
		for _, n := range newNames {
			r.sess.Forget(n)
		}
		fmt.Fprintln(r.out, err)
		return
	}
	for _, n := range newNames {
		r.bind(n, types[n], mutable[n])
		v, _ := r.sess.Value(n)
		fmt.Fprintf(r.out, "%s: %s = %s\n", n, display(types[n]), interp.Show(v))
	}
	for i := range r.binds {
		if r.binds[i].mutable {
			if t := types[r.binds[i].name]; t != nil {
				r.binds[i].typ = t
			}
		}
	}
	if shown {
		if t := types["it"]; t == nil || t.Name != "()" {
			v, _ := r.sess.Value("it")
			fmt.Fprintf(r.out, "%s: %s\n", interp.Show(v), display(t))
		}
		r.sess.Forget("it")
	}
}

func (r *REPL) bind(name string, t *ir.TypeExpr, mutable bool) {
	for i, x := range r.binds {
		if x.name == name {
			r.binds = append(r.binds[:i], r.binds[i+1:]...)
			break
		}
	}
	r.binds = append(r.binds, binding{name: name, typ: t, mutable: mutable})
}

// shape turns an input into the body that runs it: an expression it ends
// with is saved as "it", and each name a let or var of it binds, and each
// var of the session, is saved after it.
func (r *REPL) shape(text string) (body string, names []string, mutable map[string]bool, shown bool, toInput func(int) int) {
	mutable = map[string]bool{}
	// Where the body's offsets are in the input: the same, until the
	// trailer, which isn't in it.
	toInput = func(off int) int {
		if off <= len(text) {
			return off
		}
		return -1
	}
	wrapped := "fn f() {\n" + text + "\n}"
	tree := syntax.Parse([]byte(wrapped))
	block := (*syntax.Node)(nil)
	for _, d := range tree.Root.NamedChildren() {
		if d.Kind == syntax.FunctionDeclaration {
			block = d.ChildByField(syntax.FieldBody)
		}
	}
	body = text
	if block != nil && len(tree.Diagnostics) == 0 {
		stmts := block.NamedChildren()
		for _, s := range stmts {
			switch s.Kind {
			case syntax.LetStatement:
				names = append(names, bound(s.ChildByField(syntax.FieldPattern), wrapped)...)
			case syntax.VarStatement:
				n := s.ChildByField(syntax.FieldName).Text([]byte(wrapped))
				names = append(names, n)
				mutable[n] = true
			}
		}
		if len(stmts) > 0 {
			last := stmts[len(stmts)-1]
			switch last.Kind {
			case syntax.LetStatement, syntax.VarStatement, syntax.Assignment, syntax.ForStatement,
				syntax.WhileStatement, syntax.WithStatement, syntax.ReturnStatement:
			default:
				const offset = len("fn f() {\n")
				start, end := last.Span.Start-offset, last.Span.End-offset
				const prefix, suffix = `__save("it", (`, "))"
				body = text[:start] + prefix + text[start:end] + suffix + text[end:]
				shown = true
				toInput = func(off int) int {
					switch {
					case off < start:
						return off
					case off < start+len(prefix):
						return start
					case off < end+len(prefix):
						return off - len(prefix)
					case off < end+len(prefix)+len(suffix):
						return end
					case off <= len(text)+len(prefix)+len(suffix):
						return off - len(prefix) - len(suffix)
					}
					return -1
				}
			}
		}
	}
	var trailer []string
	for _, n := range names {
		trailer = append(trailer, fmt.Sprintf("__save(%q, %s)", n, n))
	}
	for _, x := range r.binds {
		if x.mutable && !contains(names, x.name) {
			trailer = append(trailer, fmt.Sprintf("__save(%q, %s)", x.name, x.name))
		}
	}
	if len(trailer) > 0 {
		body += "\n" + strings.Join(trailer, "\n")
	}
	return body, names, mutable, shown, toInput
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// bound lists the names a pattern binds.
func bound(p *syntax.Node, src string) []string {
	var out []string
	var walk func(n *syntax.Node, field bool)
	walk = func(n *syntax.Node, field bool) {
		switch n.Kind {
		case syntax.Ident, syntax.ShorthandFieldPattern:
			out = append(out, n.Text([]byte(src)))
			return
		case syntax.NamedFieldPattern:
			walk(n.ChildByField(syntax.FieldPattern), true)
			return
		case syntax.RestPattern:
			if name := n.ChildByField(syntax.FieldName); name != nil {
				out = append(out, name.Text([]byte(src)))
			}
			return
		}
		for _, c := range n.NamedChildren() {
			walk(c, false)
		}
	}
	walk(p, false)
	return out
}

// savedTypes finds the types the checker inferred for the values __input
// saves, by name.
func savedTypes(fn *ir.Func) map[string]*ir.TypeExpr {
	types := map[string]*ir.TypeExpr{}
	visit(reflect.ValueOf(fn.Body), map[uintptr]bool{}, func(c *ir.Call) {
		ref, ok := c.Func.(*ir.Ref)
		if !ok {
			return
		}
		f, ok := ref.Decl.(*ir.Func)
		if !ok || f.Name != "__save" || len(c.Args) != 2 {
			return
		}
		if lit, ok := c.Args[0].Value.(*ir.Lit); ok {
			if name, ok := lit.Value.(string); ok {
				types[name] = c.Expected
			}
		}
	})
	return types
}

// visit walks an expression and calls f on each call in it. It doesn't
// follow references to declarations.
func visit(v reflect.Value, seen map[uintptr]bool, f func(*ir.Call)) {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() || seen[v.Pointer()] {
			return
		}
		seen[v.Pointer()] = true
		if c, ok := v.Interface().(*ir.Call); ok {
			f(c)
		}
		visit(v.Elem(), seen, f)
	case reflect.Interface:
		if !v.IsNil() {
			visit(v.Elem(), seen, f)
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			visit(v.Index(i), seen, f)
		}
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			switch t.Field(i).Name {
			case "Decl", "Pos", "Expected", "Type", "Module", "Wrap":
				continue
			}
			if t.Field(i).IsExported() {
				visit(v.Field(i), seen, f)
			}
		}
	}
}

// ---------------------------------------------------------------- types as text

// display writes a type for the user; annotation writes it as source the
// session can declare a binding with, or "" when it can't.
func display(t *ir.TypeExpr) string {
	if t == nil {
		return "?"
	}
	if s := annotation(t); s != "" {
		return s
	}
	return interp.TypeString(t)
}

func annotation(t *ir.TypeExpr) string {
	if t == nil {
		return ""
	}
	var args []string
	for _, a := range t.Args {
		s := annotation(a)
		if s == "" {
			return ""
		}
		args = append(args, s)
	}
	switch t.Name {
	case "()":
		return "()"
	case "tuple":
		return "(" + strings.Join(args, ", ") + ")"
	case "record", "fn", "?", "dyn", "":
		return ""
	}
	if t.Name[0] < 'A' || t.Name[0] > 'Z' {
		return "" // a type variable
	}
	name := t.Name
	var mod *ir.Module
	switch {
	case t.Decl != nil:
		mod = t.Decl.Module
	case t.Entity != nil:
		mod = t.Entity.Module
	}
	if mod != nil && mod.Path != "" && mod.Path != "prelude" {
		name = mod.Name + "." + name
	}
	if len(args) > 0 {
		name += "(" + strings.Join(args, ", ") + ")"
	}
	return name
}

// ---------------------------------------------------------------- commands

const help = `Type an expression to see its value and type, a statement to run it, or a
declaration (fn, type, entity, job, view, impl, contract, test, import) to add
it to the session. let and var bindings last for the session.

  :type expr        the type of an expression, without running it
  :doc name         the documentation of a declaration or built-in function
  :advance 20 min   move the session's clock; jobs that wake on the way run
  :runs [id]        the session's job runs, or one run's steps
  :test [text]      run the session's tests, or those whose names contain text
  :reset            start again with an empty session
  :quit             leave (or Ctrl-D)`

func (r *REPL) command(text string) (quit bool) {
	name, arg, _ := strings.Cut(text, " ")
	arg = strings.TrimSpace(arg)
	switch name {
	case ":quit", ":q":
		return true
	case ":help", ":h":
		fmt.Fprintln(r.out, help)
	case ":type", ":t":
		r.run(arg, true)
	case ":doc":
		r.doc(arg)
	case ":advance":
		r.run("advance("+arg+")", false)
		fmt.Fprintf(r.out, "now %s\n", r.sess.Now().Format("2006-01-02 15:04:05 MST"))
	case ":runs":
		runs := interp.Runs(r.prog, r.sess.Store())
		if arg != "" {
			var one []interp.RunInfo
			for _, run := range runs {
				if fmt.Sprint(run.ID) == arg {
					one = append(one, run)
				}
			}
			interp.FormatRuns(r.out, one, true, r.sess.Now())
		} else if len(runs) == 0 {
			fmt.Fprintln(r.out, "no runs")
		} else {
			interp.FormatRuns(r.out, runs, false, r.sess.Now())
		}
	case ":test":
		results := interp.RunTests(r.prog, arg)
		failed := 0
		for _, res := range results {
			if res.Failure != "" {
				failed++
				fmt.Fprintf(r.out, "FAIL  %s\n      %s\n", res.Name, res.Failure)
			} else {
				fmt.Fprintf(r.out, "ok    %s\n", res.Name)
			}
		}
		tests := "tests"
		if len(results) == 1 {
			tests = "test"
		}
		fmt.Fprintf(r.out, "%d of %d %s passed\n", len(results)-failed, len(results), tests)
	case ":reset":
		r.decls, r.binds = nil, nil
		r.sess = interp.NewSession(r.out, r.out, time.Now().Truncate(time.Second))
		r.load("")
		fmt.Fprintln(r.out, "the session is empty")
	default:
		fmt.Fprintf(r.out, "%s isn't a command; :help lists them\n", name)
	}
	return false
}

// doc prints the documentation of a built-in function, or of a declaration
// of the session or the program: its first line and its ## comment.
func (r *REPL) doc(name string) {
	for _, d := range ir.BuiltinDocs {
		if d.Name == name {
			fmt.Fprintf(r.out, "%s\n\n%s\n", d.Signature, d.Doc)
			return
		}
	}
	for _, m := range r.prog.Modules {
		var pos ir.Pos
		switch {
		case m.Funcs[name] != nil:
			pos = m.Funcs[name].Pos
		case m.Types[name] != nil:
			pos = m.Types[name].Pos
		case m.Entities[name] != nil:
			pos = m.Entities[name].Pos
		case m.Jobs[name] != nil:
			pos = m.Jobs[name].Pos
		case m.Views[name] != nil:
			pos = m.Views[name].Pos
		default:
			continue
		}
		if pos.File == nil {
			continue
		}
		fmt.Fprintln(r.out, docAt(pos.File.Src, pos.Offset))
		return
	}
	fmt.Fprintf(r.out, "no declaration or built-in named %s\n", name)
}

// docAt returns the declaration's first line, after the ## comment lines
// above it.
func docAt(src []byte, offset int) string {
	text := string(src)
	start := strings.LastIndex(text[:offset], "\n") + 1
	end := strings.IndexByte(text[offset:], '\n')
	first := text[start:]
	if end >= 0 {
		first = text[start : offset+end]
	}
	var docs []string
	lines := strings.Split(text[:start], "\n")
	for i := len(lines) - 2; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(l, "##") {
			break
		}
		docs = append(docs, strings.TrimSpace(strings.TrimPrefix(l, "##")))
	}
	for i, j := 0, len(docs)-1; i < j; i, j = i+1, j-1 {
		docs[i], docs[j] = docs[j], docs[i]
	}
	if len(docs) == 0 {
		return first
	}
	return first + "\n\n" + strings.Join(docs, "\n")
}
