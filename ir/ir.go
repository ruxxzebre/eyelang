// Package ir is the form programs take after checking: every name is resolved
// to the declaration or local variable it means, pipes and string
// interpolation are spelled out as calls, and locals live in numbered slots.
// The interpreter runs it.
//
// The checker resolves names, then infers and checks types and effects over
// this form. It keeps what later stages need: the type a call's result was
// inferred to have (Call.Expected), from which a model call's schema comes.
package ir

import (
	_ "embed"
	"strconv"
	"strings"

	"eyelang/syntax"
)

// Program is a checked program: the module in the program's directory and
// every module it imports.
type Program struct {
	Root    *Module
	Prelude *Module
	// Store is the in-memory store, written in Eyelang (check/runtime/store.eye),
	// which the runtime keeps a program's entities, runs and events in.
	Store   *Module
	Modules []*Module // every module, the prelude first and the root last
}

// Pos is a place in a source file, for messages at run time.
type Pos struct {
	File   *File
	Offset int
	End    int // where the expression's text ends, when known
}

// Text returns the source text of the position's span.
func (p Pos) Text() string {
	if p.File == nil || p.End <= p.Offset {
		return ""
	}
	return string(p.File.Src[p.Offset:p.End])
}

// File is one source file of a module.
type File struct {
	Path string
	Src  []byte
	Tree *syntax.File
}

// String gives the position as path:line:column.
func (p Pos) String() string {
	if p.File == nil {
		return "<builtin>"
	}
	at := p.File.Tree.Lines.Position(p.Offset)
	return p.File.Path + ":" + strconv.Itoa(at.Line) + ":" + strconv.Itoa(at.Column)
}

// Module is a directory of source files that share one namespace.
type Module struct {
	Path  string // import path: "" for the program's own module, "zeropod", "lab.notify"
	Name  string // the last part of the path, the name it is imported as
	Dir   string
	Files []*File

	Globals   []*Global // module-level `let`s, in source order
	Types     map[string]*Type
	Variants  map[string]*Variant // constructors by name
	Funcs     map[string]*Func
	Entities  map[string]*Entity
	Jobs      map[string]*Job
	Views     map[string]*View
	Contracts map[string]*Contract
	Traits    map[string]*Trait
	Impls     []*Impl // the module's `impl Trait for Type` blocks
	Tests     []*Test
	Main      *Func
}

// Trait is a trait declaration: the methods its implementations have, by
// name. A method with a body is a default, which an impl may leave out.
type Trait struct {
	Name    string
	Module  *Module
	Params  []string // its type parameters: Add(rhs, out)
	Methods map[string]*Func
	Order   []string // method names in source order
	Doc     string
	Pos     Pos
}

// Impl is `impl Trait(args) for Type { … }`.
type Impl struct {
	Trait   *Trait
	Args    []*TypeExpr // the trait's type arguments
	For     *TypeExpr   // the type it is for
	Module  *Module
	Methods map[string]*Func // the impl's own methods, and the trait's defaults it takes
	Pos     Pos
}

// Bound is a requirement on a type variable: `where a: Ord`, or a trait used
// as a parameter's type, `fn label(x: Named)`.
type Bound struct {
	Var   string
	Trait *Trait
	Args  []*TypeExpr
	Pos   Pos
}

// Global is a module-level `let`. Its value is computed once, before the
// program runs, in source order.
type Global struct {
	Name   string
	Module *Module
	Init   *Func     // no parameters; returns the value
	Index  int       // position in Module.Globals
	Type   *TypeExpr // as written, or nil
	Pub    bool
	Pos    Pos
}

// Type is a type declaration with variants, a record type, or an alias.
type Type struct {
	Name     string
	Module   *Module
	Params   []string   // type variables
	Variants []*Variant // for a variant type
	Fields   []string   // for a record type
	Alias    *TypeExpr  // for an alias
	Extern   bool       // `extern type`: foreign code owns its values
	Methods  map[string]*Func
	Impls    []*Impl // the traits it implements
	Pos      Pos
	// Doc is the `##` comment above the declaration, and FieldDocs the ones
	// above a record's fields: model calls use them to describe the schema.
	Doc       string
	FieldDocs map[string]string
}

// Newtype reports whether the type has one variant, named like the type, with
// one positional field: `type WorldId = | WorldId(String)`. Such a value shows
// as its field.
func (t *Type) Newtype() bool {
	return len(t.Variants) == 1 && t.Variants[0].Name == t.Name && len(t.Variants[0].Fields) == 1 &&
		t.Variants[0].Fields[0].Name == ""
}

// Variant is one constructor of a variant type.
type Variant struct {
	Name   string
	Type   *Type
	Fields []VariantField
	Doc    string
}

// VariantField is a payload field; Name is empty for a positional field.
type VariantField struct {
	Name string
	Type *TypeExpr
}

// TypeExpr is a type as written: a name with arguments.
type TypeExpr struct {
	Name   string  // Int, List, Phase, a type variable, "()" for unit, "fn", "tuple", "record"
	Decl   *Type   // the declaration, when the name is a declared type
	Entity *Entity // for Ref(Entity) and entity names
	Trait  *Trait  // a trait where a type is written: short for a bound
	Args   []*TypeExpr
	Fields []string // record type field names, matching Args
	// A function type's result, nil when it returns (); its parameters are
	// Args. Uses lists the effects its uses clause states, and HasUses is
	// set when it has one.
	Result  *TypeExpr
	Uses    []string
	HasUses bool
}

// Func is a function, method, lambda, job body, test body or initializer.
type Func struct {
	Host     string // for an extern fn: the host function that runs it, from @host("…")
	Name     string
	Module   *Module
	Params   []*Param
	Slots    int  // locals, parameters included
	Body     Expr // nil for a signature
	Entity   *Entity
	Receiver *Type // for methods in `impl Type`
	HasSelf  bool  // a method that takes self
	MutSelf  bool
	Pub      bool
	Return   *TypeExpr
	Captures []Capture // for lambdas: values copied in when the lambda is made
	// Uses lists the effects its signature states, such as "net Notify.send";
	// HasUses is set when it has a uses clause, even an empty one.
	Uses    []string
	HasUses bool
	Extern  bool
	// Waits is set by the checker when calling it may pause: its body has a
	// `!` step, or calls a function that waits.
	Waits bool
	// Bounds are the requirements on its type variables, from `where` and
	// from traits used as parameter types.
	Bounds []Bound
	// Budget is its @budget: what the model calls made while it runs may
	// spend. A job's is on its Func.
	Budget *Budget
	Pos    Pos
}

// Budget is @budget(usd:, tokens:); a negative limit is one it doesn't set.
type Budget struct {
	USD    float64
	Tokens int64
}

// Param is a function parameter.
type Param struct {
	Name    string
	Slot    int
	Mut     bool
	Default Expr // evaluated in the callee's frame, nil if required
	Type    *TypeExpr
}

// Capture copies a value from the enclosing frame into a lambda's slot.
type Capture struct {
	From Expr // a Local, Self or SelfField of the enclosing function
	Slot int
}

// Entity is an entity declaration.
type Entity struct {
	Name    string
	Module  *Module
	ID      *Param
	States  []*State
	Methods map[string]*Func
	Pos     Pos

	// Version is the @version of the entity's stored shape, 1 without one.
	Version int
	// Migrations, oldest first, each turn a row saved at one version into
	// the state at the next.
	Migrations []*Migration
}

// Migration is `migrate from N (old) { … }`: a function of one parameter,
// the row as saved at version From, that gives the state at From+1.
type Migration struct {
	From int
	Func *Func
	Pos  Pos
}

// State is a `state` field of an entity.
type State struct {
	Name    string
	Type    *TypeExpr
	Default *Func // nil if the field has no default
}

// Job is a job declaration.
type Job struct {
	Name     string
	Module   *Module
	Func     *Func
	Triggers []Trigger
	Pos      Pos

	Concurrency *Concurrency // @concurrency
	Limit       *RateLimit   // @limit
	Deadline    *Func        // @deadline: a Duration from the run's start, or an Instant
	Memory      Size         // @memory: what a run may hold; 0 without one
	Priority    Priority     // @priority; Normal without one
}

// Priority is a job's @priority: which runs go first when memory is short.
type Priority int

// The priorities, lowest first.
const (
	Batch Priority = iota - 2
	Background
	Normal
	Interactive
)

// PriorityNames are the words @priority takes.
var PriorityNames = map[string]Priority{"batch": Batch, "background": Background, "normal": Normal, "interactive": Interactive}

func (p Priority) String() string {
	for name, q := range PriorityNames {
		if q == p {
			return name
		}
	}
	return "normal"
}

// Concurrency is @concurrency(n, by: param): at most N runs at once, for
// each value of the parameter at index By, or for the job when By is -1.
type Concurrency struct {
	N  int
	By int
}

// RateLimit is @limit(n, per: duration, by: param): at most N runs start in
// any window of Per, for each value of the parameter at index By, or for the
// job when By is -1.
type RateLimit struct {
	N   int
	Per Duration
	By  int
}

// Trigger starts a job: `@on(enters(view))` or `@on(leaves(view))`.
type Trigger struct {
	Kind     string // "enters", "leaves", "event" or "schedule"
	View     *View
	Event    *Event
	Schedule *Func // computes the Schedule: daily(07:40, in: dublin), every(15 min)
}

// View is a named pipeline over entities, kept up to date by the runtime.
type View struct {
	Name   string
	Module *Module
	Func   *Func // no parameters; returns the matching snapshots
	Entity *Entity
	Pos    Pos
}

// Contract describes another service.
type Contract struct {
	Name   string
	Module *Module
	Ops    map[string]*Op
	Events map[string]*Event
	HTTP   ContractHTTP // from @http(names: …) and @auth(…) on the contract
}

// ContractHTTP is what a contract's operations share over HTTP.
type ContractHTTP struct {
	Snake      bool   // names are snake_case on the wire: dedupeKey is dedupe_key, Normal is "normal"
	AuthHeader string // the header that carries the credential: Authorization, X-Auth-Token
	AuthBearer bool   // the credential is sent as "Bearer <secret>"
	AuthSecret string // the secret that holds the credential
}

// OpHTTP is how one operation goes over HTTP: @http("POST /v1/send", status: {…}).
type OpHTTP struct {
	Method string
	Path   string         // with :name where an argument goes in the path
	Status map[int]string // the variant a reply with that status decodes as
}

// Event is an event a contract publishes: `event answered: Answered on "notify.answers"`.
type Event struct {
	Name     string
	Contract *Contract
	Type     *TypeExpr
	Topic    string
	Key      string // the field that tells one event from another, from @key(field)
}

// Op is a command or query of a contract.
type Op struct {
	Name      string
	Contract  *Contract
	Command   bool
	Params    []*Param
	Return    *TypeExpr
	Reconcile string // the query that resolves an Uncertain outcome
	Key       bool   // @key: the command takes a `key: Key` argument
	KeyHeader string // @key(…, header: "…"): the HTTP header that carries the key
	KeyField  string // @key(…, field: "…"): a body field that carries it too
	HTTP      *OpHTTP
	Impl      *Func // the function that serves it, when this program serves the contract
	Pos       Pos
}

// Test is a `test "name" { … }` block.
type Test struct {
	Name   string
	Module *Module
	Func   *Func
	Pos    Pos
}

// ---------------------------------------------------------------- expressions

// Expr is an expression or statement. Statements evaluate to `()`.
type Expr interface{ Position() Pos }

// At carries an expression's position.
type At struct{ Pos Pos }

// Position returns where the expression is.
func (a At) Position() Pos { return a.Pos }

type (
	// Lit is a constant: *big.Int, float64, string, bool, rune, Unit,
	// Duration, Size, Date, TimeOfDay or Instant.
	Lit struct {
		At
		Value any
	}
	// EmptyBraces is `{}`: an empty record, map or set.
	EmptyBraces struct{ At }
	// Local reads a slot of the current frame.
	Local struct {
		At
		Name string
		Slot int
	}
	// Self is the receiver of a method; in an entity method, the state.
	Self struct{ At }
	// SelfField reads a state field in an entity method: `players`.
	SelfField struct {
		At
		Name string
	}
	// Ref names a declaration: *Func, *Global, *Variant, *Entity, *Job,
	// *View, *Op, *Module, *Type or a Builtin.
	Ref struct {
		At
		Decl any
	}
	// Call calls a function value with arguments. Expected is the type the
	// checker inferred for its result, with every type variable resolved, or
	// nil where a part of it isn't known: for `ai.generate!(…)` it is
	// Result(T, AiError), and T is the type the reply is decoded into.
	Call struct {
		At
		Func     Expr
		Args     []Arg
		Expected *TypeExpr
	}
	// MethodCall calls a method on a value, found by the value's kind at run
	// time: entity methods, `start` on a job, `impl` methods, built-in methods.
	// RecvPlace is set when the receiver is a variable or a field of one, so
	// a `mut self` method can write its change back.
	MethodCall struct {
		At
		Recv      Expr
		Name      string
		Args      []Arg
		RecvPlace *Place
	}
	// Waiting is a waiting call, `f!(x)`: a durable step in a job.
	Waiting struct {
		At
		Call Expr // a Call or MethodCall
	}
	// Field reads a record field, a snapshot field or a state field of a reference.
	Field struct {
		At
		Value Expr
		Name  string
	}
	Index struct {
		At
		Value, Index Expr
	}
	Unary struct {
		At
		Op string
		X  Expr
	}
	Binary struct {
		At
		Op   string
		L, R Expr
	}
	If struct {
		At
		Cond, Then, Else Expr // Else may be nil
	}
	Match struct {
		At
		Value Expr
		Arms  []Arm
	}
	// Block runs its expressions in order; its value is the last one's,
	// or () if it ends with a statement.
	Block struct {
		At
		Exprs []Expr
		Value bool // the last expression gives the block's value
	}
	Let struct {
		At
		Pattern Pattern
		Type    *TypeExpr // as written: `let x: T = …`, or nil
		Value   Expr
		Else    Expr // for `let … else`, nil otherwise
	}
	Assign struct {
		At
		Target Place
		Op     string // "=", "+=", …
		Value  Expr
	}
	For struct {
		At
		Pattern Pattern
		Iter    Expr
		Body    Expr
	}
	While struct {
		At
		Cond, Body Expr
	}
	Break    struct{ At }
	Continue struct{ At }
	Return   struct {
		At
		Value Expr // nil for `return`
	}
	// Lambda makes a closure of Func, copying the captured values.
	Lambda struct {
		At
		Func *Func
	}
	Record struct {
		At
		Names  []string
		Values []Expr
		Type   *Type // the record type it is, once the checker knows
	}
	RecordUpdate struct {
		At
		Base   Expr
		Names  []string
		Values []Expr
	}
	List struct {
		At
		Elems []Expr
	}
	Map struct {
		At
		Keys, Values []Expr
	}
	Tuple struct {
		At
		Elems []Expr
	}
	// InZone is a time in a zone named by a value: `07:40 in dublin`, or
	// with a date, `2026-10-12 07:40 in dublin`. Instant's Zone is empty.
	InZone struct {
		At
		Instant Instant
		Zone    Expr
	}
	// Interpolate joins the shown values of its parts into a string.
	Interpolate struct {
		At
		Parts []Expr
	}
	// Try is postfix `?`. Wrap is the caller's error type, for wrapping the
	// error in the variant that holds its type.
	Try struct {
		At
		X    Expr
		Wrap *Type
	}
	// Default is `x ?? y`.
	Default struct {
		At
		X, Or Expr
	}
	Range struct {
		At
		Start, End Expr // either may be nil in a slice
		Inclusive  bool
	}
	// With applies a handler to the rest of a test: `with clock(…)`.
	With struct {
		At
		Value Expr
	}
)

// Arg is an argument of a call.
type Arg struct {
	Name  string // for a named argument
	Value Expr
	Mut   bool // `mut x`: the callee may change it, and the change is written back
	Place Place
}

// Arm is a match arm.
type Arm struct {
	Pattern Pattern
	Guard   Expr
	Body    Expr
}

// Place is something that can be assigned: a local or the receiver (an
// entity's state, or `self` in a `mut self` method), then field names.
type Place struct {
	Self bool // the receiver, rather than the local in Slot
	Slot int
	Path []string
	Pos  Pos
}

// Builtin is a function provided by the runtime.
type Builtin struct{ Name string }

// Unit is the value ().
type Unit struct{}

// Duration is a length of time in nanoseconds.
type Duration int64

// Size is a number of bytes.
type Size int64

// Date is a calendar date.
type Date struct{ Year, Month, Day int }

// TimeOfDay is a time of day.
type TimeOfDay struct{ Hour, Minute, Second int }

// Instant is a date and time in a zone, as written; the interpreter resolves
// the zone.
type Instant struct {
	Date Date
	Time TimeOfDay
	Zone string
}

// ---------------------------------------------------------------- patterns

// Pattern is a pattern in `let`, `match`, `for` or a parameter.
type Pattern interface{ isPattern() }

type (
	PWild struct{}
	PBind struct {
		Name string
		Slot int
	}
	PLit struct{ Value any }
	// PVariant matches a variant. Names are set for named fields; Rest is `..`.
	PVariant struct {
		Variant *Variant
		Args    []Pattern
		Names   []string
		Rest    bool
	}
	PRecord struct {
		Names []string
		Pats  []Pattern
		Rest  bool
	}
	PTuple struct{ Pats []Pattern }
	// PList matches a list; Rest is the index of `..` or -1, and RestPat the
	// pattern bound to the rest, if any.
	PList struct {
		Pats    []Pattern
		Rest    int
		RestPat Pattern
	}
	PRange struct {
		Lo, Hi    any
		Inclusive bool
	}
	POr struct{ Alts []Pattern }
	// PEmpty matches an empty collection: `{}` in a pattern position.
	PEmpty struct{}
)

func (PWild) isPattern()    {}
func (PBind) isPattern()    {}
func (PLit) isPattern()     {}
func (PVariant) isPattern() {}
func (PRecord) isPattern()  {}
func (PTuple) isPattern()   {}
func (PList) isPattern()    {}
func (PRange) isPattern()   {}
func (POr) isPattern()      {}
func (PEmpty) isPattern()   {}

// BuiltinDoc documents a built-in function for `eye doc`.
type BuiltinDoc struct {
	Name, Signature, Doc string
	Group                string
}

// BuiltinDocs documents every function in Builtins and BuiltinModules, in
// the order the reference lists them. It is read from builtin.eye, where each
// built-in is declared with its doc comment.
var BuiltinDocs, BuiltinTypeDocs = readBuiltins(BuiltinSource)

// BuiltinSource is builtin.eye: the built-in types and functions, declared
// with `extern type` and `extern fn` and documented (SPECS.md section 18).
// Tools show it as the built-ins' declarations.
//
//go:embed builtin.eye
var BuiltinSource []byte

// readBuiltins reads the declarations of builtin.eye: its functions, and its
// types, each with its group, signature and doc comment.
func readBuiltins(src []byte) (funcs, types []BuiltinDoc) {
	f := syntax.Parse(src)
	if len(f.Diagnostics) > 0 {
		panic("ir/builtin.eye doesn't parse: " + f.Diagnostics[0].Message)
	}
	group := ""
	for _, d := range f.Root.Children {
		group = builtinGroup(d, src, group)
		name := d.ChildByField(syntax.FieldName)
		if name == nil {
			continue
		}
		doc := syntax.DocComment(d, src)
		switch d.Kind {
		case syntax.ExternTypeDeclaration:
			sig := "type " + string(src[name.Span.Start:d.Span.End])
			types = append(types, BuiltinDoc{Name: name.Text(src), Signature: sig, Doc: doc, Group: group})
		case syntax.ExternDeclaration:
			n := name.Text(src)
			for _, attr := range d.Children {
				if attr.Kind == syntax.Attribute && attr.ChildByField(syntax.FieldName).Text(src) == "host" {
					if args := attr.ChildByField(syntax.FieldArguments); args != nil && len(args.NamedChildren()) == 1 {
						n = strings.Trim(args.NamedChildren()[0].Text(src), `"`) // log.info
					}
				}
			}
			sig := "fn " + n + string(src[name.Span.End:d.Span.End])
			funcs = append(funcs, BuiltinDoc{Name: n, Signature: sig, Doc: doc, Group: group})
		}
	}
	return funcs, types
}

// builtinGroup returns the group a declaration of builtin.eye is in: the
// last `# == Group` line above it, or the group before.
func builtinGroup(d *syntax.Node, src []byte, group string) string {
	first := d
	for !first.IsToken() && len(first.Children) > 0 {
		first = first.Children[0]
	}
	for _, t := range first.Leading {
		if t.Kind != syntax.Comment {
			continue
		}
		if g, ok := strings.CutPrefix(string(src[t.Span.Start:t.Span.End]), "# == "); ok {
			group = strings.TrimSpace(g)
		}
	}
	return group
}

// Builtins are the functions every module can call without importing them.
// The interpreter provides them; a test checks the two lists agree.
var Builtins = func() []string {
	var names []string
	for _, d := range BuiltinDocs {
		if !strings.Contains(d.Name, ".") {
			names = append(names, d.Name)
		}
	}
	return names
}()

// BuiltinModules are the standard modules every module can use without
// importing them, with their functions.
var BuiltinModules = map[string][]string{
	"log": {"debug", "info", "warn", "error"},
	"env": {"get"},
}
