// Package codegen writes Go and TypeScript code from a program's contracts
// (SPECS.md §17, "Generated code for other languages"): for each module that
// declares contracts, its types in the one JSON shape, a client that calls
// the operations over @http, and a server stub to implement that routes
// requests the way eye run serves the contract. `eye gen go` and
// `eye gen ts` print it.
//
// Both languages are written from one model of a contract module, built
// here from the checked program with the rules of internal/wire, which the
// interpreter follows too; a parameter's default is the JSON the
// interpreter computes for it (interp.DefaultJSON).
package codegen

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"eyelang/internal/wire"
	"eyelang/interp"
	"eyelang/ir"
)

// Options are what the program doesn't say.
type Options struct {
	// Source is the program's directory as the user named it, for the
	// header comment.
	Source string
	// Package is the Go package's name; the module's name when empty.
	Package string
}

// Modules returns the modules of a program that declare contracts, sorted by
// import path: each is one generated file.
func Modules(prog *ir.Program) []*ir.Module {
	var out []*ir.Module
	for _, m := range prog.Modules {
		if len(m.Contracts) > 0 && m.Dir != "" {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// FindModule returns the module of a program with contracts whose import
// path or name is name.
func FindModule(prog *ir.Program, name string) (*ir.Module, error) {
	mods := Modules(prog)
	for _, m := range mods {
		if m.Path == name {
			return m, nil
		}
	}
	var found []*ir.Module
	for _, m := range mods {
		if m.Name == name {
			found = append(found, m)
		}
	}
	if len(found) == 1 {
		return found[0], nil
	}
	var names []string
	for _, m := range mods {
		names = append(names, m.Path)
	}
	if len(found) > 1 {
		return nil, fmt.Errorf("more than one module is named %s: give its import path, one of %s", name, strings.Join(names, ", "))
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("the program declares no contracts")
	}
	return nil, fmt.Errorf("no module %s declares a contract; the modules that do are %s", name, strings.Join(names, ", "))
}

// ---------------------------------------------------------------- the model

// unit is what one generated file holds: a module's contracts and every type
// they use.
type unit struct {
	prog      *ir.Program
	mod       *ir.Module
	snake     bool
	contracts []*contract
	types     []*named          // sorted by module and position
	byKey     map[string]*named // by typeKey
	errs      []string
}

type contract struct {
	ct       *ir.Contract
	ops      []*operation // the operations with @http, in source order
	internal []*ir.Op     // the others, reached only from inside the program
	client   string       // NotifyClient
	service  string       // NotifyService
}

type operation struct {
	op       *ir.Op
	args     string // SendArgs; empty when it has no parameters
	params   []*param
	result   *ir.TypeExpr // the type a success decodes as: T of Result(T, http.HttpError)
	httpErr  bool         // it returns Result(T, http.HttpError)
	statuses []status     // the codes `status:` maps, lowest first
}

// status is one code that `status:` maps to a variant of the result.
type status struct {
	code    int
	variant string
}

type param struct {
	p        *ir.Param
	wire     string       // its name on the wire
	t        *ir.TypeExpr // its type, for an Option the type inside
	in       string       // "path", "query" or "body"
	option   bool         // an Option: left out when None
	def      []byte       // the JSON its default sends; nil without one
	optional bool         // a call may leave it out: an Option, or one with a default
}

type kind int

const (
	kRecord  kind = iota // an object with every field
	kEnum                // variants without fields: a string
	kNewtype             // wraps one value: that value
	kUnion               // variants tagged with "kind"
)

// named is a declared type as the file uses it, with its type arguments
// filled in: generic types get a declaration for each use.
type named struct {
	key      string
	decl     *ir.Type
	kind     kind
	base     string // the name it asks for: Delivery, PageEntry
	name     string // the name it got
	doc      string
	fields   []field      // a record's
	inner    *ir.TypeExpr // a newtype's
	variants []*variant   // an enum's or a union's
}

type field struct {
	name string // as declared
	wire string
	t    *ir.TypeExpr
	doc  string
}

type variant struct {
	v      *ir.Variant
	of     *named
	name   string // a union variant's own type, or an enum's constant
	tag    string // the kind tag: the variant's name as declared
	value  string // an enum variant's string on the wire
	fields []field
	doc    string
}

// build makes the model of mod's contracts.
func build(prog *ir.Program, mod *ir.Module) (*unit, error) {
	u := &unit{prog: prog, mod: mod, byKey: map[string]*named{}}
	cts := make([]*ir.Contract, 0, len(mod.Contracts))
	for _, ct := range mod.Contracts {
		cts = append(cts, ct)
	}
	sort.Slice(cts, func(i, j int) bool { return cts[i].Name < cts[j].Name })
	for i, ct := range cts {
		if i > 0 && ct.HTTP.Snake != cts[0].HTTP.Snake {
			return nil, fmt.Errorf("%s and %s in module %s name things differently on the wire (@http(names: snake)); generated code needs one way per module", cts[0].Name, ct.Name, mod.Path)
		}
		u.snake = ct.HTTP.Snake
		c := &contract{ct: ct}
		routed, internal := wire.HTTPOps(ct)
		c.internal = internal
		for _, op := range wire.SortedOps(ct) {
			for _, p := range op.Params {
				u.use(p.Type)
			}
			if wire.HTTPErrorType(op.Return) != nil {
				// Result(T, http.HttpError) as a whole result is the
				// language's error; anywhere else it is a variant type like
				// any other.
				u.use(op.Return.Args[0])
			} else {
				u.use(op.Return)
			}
		}
		evs := make([]*ir.Event, 0, len(ct.Events))
		for _, ev := range ct.Events {
			evs = append(evs, ev)
		}
		sort.Slice(evs, func(i, j int) bool { return evs[i].Name < evs[j].Name })
		for _, ev := range evs {
			u.use(ev.Type)
		}
		for _, op := range routed {
			c.ops = append(c.ops, u.operation(op))
		}
		u.contracts = append(u.contracts, c)
	}
	u.sortTypes()
	u.allocate()
	if len(u.errs) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(u.errs, "\n"))
	}
	return u, nil
}

func (u *unit) errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	for _, e := range u.errs {
		if e == msg {
			return
		}
	}
	u.errs = append(u.errs, msg)
}

func (u *unit) operation(op *ir.Op) *operation {
	o := &operation{op: op, result: op.Return}
	if wire.HTTPErrorType(op.Return) != nil {
		o.httpErr, o.result = true, op.Return.Args[0]
	}
	inPath := map[string]bool{}
	for _, name := range wire.PathParams(op.HTTP.Path) {
		inPath[name] = true
	}
	for _, p := range op.Params {
		pp := &param{p: p, wire: wire.Name(p.Name, u.snake), t: p.Type, in: "body"}
		switch {
		case inPath[p.Name]:
			pp.in = "path"
		case wire.InQuery(op.HTTP.Method):
			pp.in = "query"
		}
		if inner := u.optionOf(p.Type); inner != nil {
			pp.option, pp.t = true, inner
		}
		def, ok, err := interp.DefaultJSON(u.prog, op, p)
		if err != nil {
			u.errorf("%s", err)
		}
		if ok {
			pp.def = def
		}
		pp.optional = pp.option || p.Default != nil
		if pp.in == "path" && pp.optional {
			u.errorf("%s.%s: %s goes in the path, so it can't be left out; it has a default or is an Option", op.Contract.Name, op.Name, p.Name)
		}
		o.params = append(o.params, pp)
	}
	for _, code := range wire.Statuses(op) {
		o.statuses = append(o.statuses, status{code: code, variant: op.HTTP.Status[code]})
	}
	return o
}

// optionOf returns T when t is Option(T).
func (u *unit) optionOf(t *ir.TypeExpr) *ir.TypeExpr {
	if t != nil && t.Decl != nil && t.Decl == u.prog.Prelude.Types["Option"] && len(t.Args) == 1 {
		return t.Args[0]
	}
	return nil
}

// typeEnv maps a generic type's variables to the types they stand for.
type typeEnv map[string]*ir.TypeExpr

// subst fills a type's variables in from env.
func subst(t *ir.TypeExpr, env typeEnv) *ir.TypeExpr {
	if t == nil || len(env) == 0 {
		return t
	}
	if t.Decl == nil && len(t.Args) == 0 {
		if r, ok := env[t.Name]; ok {
			return r
		}
		return t
	}
	c := *t
	c.Args = make([]*ir.TypeExpr, len(t.Args))
	for i, a := range t.Args {
		c.Args[i] = subst(a, env)
	}
	c.Result = subst(t.Result, env)
	return &c
}

func bind(d *ir.Type, args []*ir.TypeExpr) typeEnv {
	env := typeEnv{}
	for i, p := range d.Params {
		if i < len(args) {
			env[p] = args[i]
		}
	}
	return env
}

// typeKey names a type uniquely: the module and declaration, and its
// arguments.
func typeKey(t *ir.TypeExpr) string {
	var b strings.Builder
	if t.Decl != nil {
		if t.Decl.Module != nil {
			b.WriteString(t.Decl.Module.Path)
			b.WriteByte('.')
		}
		b.WriteString(t.Decl.Name)
	} else {
		b.WriteString(t.Name)
		for _, f := range t.Fields {
			b.WriteString(" " + f)
		}
	}
	if len(t.Args) > 0 {
		b.WriteByte('(')
		for i, a := range t.Args {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(typeKey(a))
		}
		b.WriteByte(')')
	}
	return b.String()
}

// resolve looks through aliases that aren't records: `type Tags = List(String)`
// is List(String).
func resolve(t *ir.TypeExpr) *ir.TypeExpr {
	for i := 0; i < 32 && t != nil && t.Decl != nil && t.Decl.Alias != nil && len(t.Decl.Variants) == 0 && t.Decl.Alias.Name != "record"; i++ {
		t = subst(t.Decl.Alias, bind(t.Decl, t.Args))
	}
	return t
}

// use records the types t needs, and reports what it can't write.
func (u *unit) use(t *ir.TypeExpr) {
	t = resolve(t)
	if t == nil {
		return
	}
	if t.Decl == nil {
		switch t.Name {
		case "String", "Key", "Zone", "Int", "Float", "Bool", "Duration", "Size", "Instant", "Date", "Time", "Version", "Approval", "()":
		case "List", "Set":
			for _, a := range t.Args {
				u.use(a)
			}
		case "Map":
			if len(t.Args) == 2 {
				if !u.stringOnWire(t.Args[0]) {
					u.errorf("a Map whose keys aren't strings on the wire, such as %s, has no generated type: its JSON is an array of pairs", typeString(t))
				}
				u.use(t.Args[0])
				u.use(t.Args[1])
			}
		case "tuple":
			u.errorf("a tuple, %s, has no generated type yet: use a record", typeString(t))
		case "record":
			u.errorf("an unnamed record type, %s, has no generated type: name it with `type`", typeString(t))
		default:
			u.errorf("%s has no generated type: contracts' types are records, variants, lists, sets, maps, strings, numbers, booleans, times, Version and Approval", typeString(t))
		}
		return
	}
	d := t.Decl
	if d == u.prog.Prelude.Types["Option"] {
		if len(t.Args) == 1 {
			u.use(t.Args[0])
		}
		return
	}
	key := typeKey(t)
	if _, ok := u.byKey[key]; ok {
		return
	}
	if d.Extern {
		u.errorf("%s is an extern type: foreign code owns its values, so they can't cross the wire", d.Name)
		return
	}
	env := bind(d, t.Args)
	n := &named{key: key, decl: d, doc: d.Doc, base: baseName(t)}
	u.byKey[key] = n
	switch {
	case d.Alias != nil && len(d.Variants) == 0:
		n.kind = kRecord
		for i, name := range d.Alias.Fields {
			n.fields = append(n.fields, field{name: name, wire: wire.Name(name, u.snake), t: subst(d.Alias.Args[i], env), doc: d.FieldDocs[name]})
		}
	case d.Newtype():
		n.kind = kNewtype
		n.inner = subst(d.Variants[0].Fields[0].Type, env)
	case wire.IsEnum(d):
		n.kind = kEnum
		for _, v := range d.Variants {
			n.variants = append(n.variants, &variant{v: v, of: n, tag: v.Name, value: wire.Name(v.Name, u.snake), doc: v.Doc})
		}
	default:
		n.kind = kUnion
		for _, v := range d.Variants {
			vv := &variant{v: v, of: n, tag: v.Name, doc: v.Doc}
			for i, f := range v.Fields {
				name := wire.FieldName(v, i)
				vv.fields = append(vv.fields, field{name: name, wire: wire.Name(name, u.snake), t: subst(f.Type, env)})
			}
			n.variants = append(n.variants, vv)
		}
	}
	u.types = append(u.types, n)
	for _, f := range n.fields {
		u.use(f.t)
	}
	if n.inner != nil {
		u.use(n.inner)
	}
	for _, v := range n.variants {
		for _, f := range v.fields {
			u.use(f.t)
		}
	}
}

// stringOnWire reports whether a type's values are JSON strings, so that it
// can key a JSON object.
func (u *unit) stringOnWire(t *ir.TypeExpr) bool {
	t = resolve(t)
	if t.Decl == nil {
		switch t.Name {
		case "String", "Key", "Zone":
			return true
		}
		return false
	}
	switch {
	case t.Decl.Newtype():
		return u.stringOnWire(subst(t.Decl.Variants[0].Fields[0].Type, bind(t.Decl, t.Args)))
	case wire.IsEnum(t.Decl):
		return true
	}
	return false
}

// baseName is the name a declared type asks for: its own, with its type
// arguments' names after it for a generic one.
func baseName(t *ir.TypeExpr) string {
	t = resolve(t)
	var b strings.Builder
	if t.Decl != nil {
		b.WriteString(t.Decl.Name)
	} else {
		switch t.Name {
		case "()":
			b.WriteString("Unit")
		case "tuple":
			b.WriteString("Tuple")
		default:
			b.WriteString(upperFirst(t.Name))
		}
	}
	for _, a := range t.Args {
		b.WriteString(baseName(a))
	}
	return b.String()
}

// sortTypes puts the file's types in order: the module's own first, then
// the others by import path, each in source order.
func (u *unit) sortTypes() {
	rank := func(n *named) string {
		if n.decl.Module == u.mod {
			return ""
		}
		if n.decl.Module == nil {
			return "~"
		}
		return "." + n.decl.Module.Path
	}
	sort.SliceStable(u.types, func(i, j int) bool {
		a, b := u.types[i], u.types[j]
		if rank(a) != rank(b) {
			return rank(a) < rank(b)
		}
		if a.decl.Pos.Offset != b.decl.Pos.Offset {
			return a.decl.Pos.Offset < b.decl.Pos.Offset
		}
		return a.key < b.key
	})
}

// reserved are names the generated code declares itself, in either
// language, which a type of the program can't take.
var reserved = map[string]bool{
	"Uncertain": true, "Stale": true, "Expired": true, "ErrStale": true, "ErrExpired": true,
	"HTTPError": true, "HttpError": true, "Version": true, "Approval": true, "Duration": true,
	"ClientOptions": true, "ServiceOptions": true,
}

// allocate gives every type, variant, client, service and argument record
// a name of its own. Each asks for a list of names, plainest first; when two
// ask for the same one, or it is reserved, both move on to their next.
func (u *unit) allocate() {
	type claim struct {
		set   func(string)
		names []string
		at    int
	}
	var claims []*claim
	add := func(set func(string), names ...string) {
		claims = append(claims, &claim{set: set, names: names})
	}
	for _, n := range u.types {
		mod, path := "", ""
		if n.decl.Module != nil {
			mod, path = upperFirst(n.decl.Module.Name), camelPath(n.decl.Module.Path)
		}
		add(func(s string) { n.name = s }, n.base, mod+n.base, path+n.base)
		for _, v := range n.variants {
			switch {
			case n.single():
				continue // the type's own name
			case n.kind == kEnum:
				add(func(s string) { v.name = s }, n.base+v.tag, mod+n.base+v.tag, path+n.base+v.tag)
			default:
				add(func(s string) { v.name = s }, v.tag, n.base+v.tag, mod+n.base+v.tag, path+n.base+v.tag)
			}
		}
	}
	for _, c := range u.contracts {
		add(func(s string) { c.client = s }, c.ct.Name+"Client")
		add(func(s string) { c.service = s }, c.ct.Name+"Service")
		for _, o := range c.ops {
			if len(o.params) > 0 {
				add(func(s string) { o.args = s }, upperFirst(o.op.Name)+"Args", c.ct.Name+upperFirst(o.op.Name)+"Args")
			}
		}
	}
	for round := 0; ; round++ {
		count := map[string]int{}
		for _, c := range claims {
			count[c.names[c.at]]++
		}
		moved, stuck := false, ""
		for _, c := range claims {
			name := c.names[c.at]
			if count[name] > 1 || reserved[name] {
				if c.at+1 < len(c.names) {
					c.at++
					moved = true
				} else {
					stuck = name
				}
			}
		}
		if !moved || round > 8 {
			if stuck != "" {
				u.errorf("two declarations would both be named %s in the generated code", stuck)
			}
			break
		}
	}
	for _, c := range claims {
		c.set(c.names[c.at])
	}
	for _, n := range u.types {
		if n.single() {
			n.variants[0].name = n.name
		}
	}
}

// single reports whether a type tagged with "kind" has one variant, named
// like the type: `type Cleared = | Cleared(id: String, by: String)`. Its one
// variant is the type, with no union around it.
func (n *named) single() bool {
	return n.kind == kUnion && len(n.variants) == 1 && n.variants[0].tag == n.decl.Name
}

// ---------------------------------------------------------------- shapes

// named returns the declaration t uses, or nil for a built-in type.
func (u *unit) named(t *ir.TypeExpr) *named {
	t = resolve(t)
	if t == nil || t.Decl == nil {
		return nil
	}
	return u.byKey[typeKey(t)]
}

func typeString(t *ir.TypeExpr) string { return interp.TypeString(t) }

// ---------------------------------------------------------------- names

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// camelPath is a module path as one name: lab.notify is LabNotify.
func camelPath(path string) string {
	var b strings.Builder
	for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '.' || r == '/' || r == '-' || r == '_' }) {
		b.WriteString(upperFirst(part))
	}
	return b.String()
}

// initialisms are the words Go writes in capitals.
var initialisms = map[string]bool{"id": true, "url": true, "uri": true, "http": true, "https": true, "json": true, "api": true, "ip": true, "html": true, "uuid": true, "ttl": true}

// goName is an exported Go name for an Eyelang one: feedUrl is FeedURL.
func goName(s string) string {
	var words []string
	start := 0
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			words = append(words, s[start:i])
			start = i
		}
	}
	words = append(words, s[start:])
	var b strings.Builder
	for _, w := range words {
		if initialisms[strings.ToLower(w)] {
			b.WriteString(strings.ToUpper(w))
		} else {
			b.WriteString(upperFirst(w))
		}
	}
	return b.String()
}

// docLines splits a `##` comment into lines.
func docLines(doc string) []string {
	doc = strings.TrimSpace(doc)
	if doc == "" {
		return nil
	}
	return strings.Split(doc, "\n")
}

// bearerNote is how a bearer credential goes in its header, for prose.
func bearerNote(ct *ir.Contract) string {
	if ct.HTTP.AuthBearer {
		return ", as Bearer <secret>,"
	}
	return ""
}

// internalOps says which of a contract's operations have no @http, if any.
func internalOps(c *contract) string {
	if len(c.internal) == 0 {
		return ""
	}
	var names []string
	for _, op := range c.internal {
		names = append(names, op.Name)
	}
	verb := "has"
	if len(names) > 1 {
		verb = "have"
	}
	return fmt.Sprintf("%s %s no @http, so only the program that serves %s reaches them.", joinWords(names), verb, c.ct.Name)
}

// wrap breaks paragraphs into lines of at most about 76 characters, for the
// comments the generated code writes itself. An empty paragraph is an empty
// line.
func wrap(paragraphs ...string) []string {
	var lines []string
	for _, para := range paragraphs {
		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		line := words[0]
		for _, w := range words[1:] {
			if len(line)+1+len(w) > 76 {
				lines = append(lines, line)
				line = w
			} else {
				line += " " + w
			}
		}
		lines = append(lines, line)
	}
	return lines
}

// joinWords lists names for a sentence: "a, b and c".
func joinWords(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}
