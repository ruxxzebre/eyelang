// Package ir is the form programs take after checking: every name is resolved
// to the declaration or local variable it means, pipes and string
// interpolation are spelled out as calls, and locals live in numbered slots.
// The interpreter runs it.
//
// Types aren't in it yet. The checker resolves names and checks where waiting
// calls may appear; type inference (phase 2) will annotate this form rather
// than replace it.
package ir

import (
	"strconv"

	"eyelang/syntax"
)

// Program is a checked program: the module in the program's directory and
// every module it imports.
type Program struct {
	Root    *Module
	Prelude *Module
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
	Tests     []*Test
	Main      *Func
}

// Global is a module-level `let`. Its value is computed once, before the
// program runs, in source order.
type Global struct {
	Name   string
	Module *Module
	Init   *Func // no parameters; returns the value
	Index  int   // position in Module.Globals
}

// Type is a type declaration with variants, a record type, or an alias.
type Type struct {
	Name     string
	Module   *Module
	Params   []string   // type variables
	Variants []*Variant // for a variant type
	Fields   []string   // for a record type
	Alias    *TypeExpr  // for an alias
	Methods  map[string]*Func
	Pos      Pos
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
}

// VariantField is a payload field; Name is empty for a positional field.
type VariantField struct {
	Name string
	Type *TypeExpr
}

// TypeExpr is a type as written: a name with arguments.
type TypeExpr struct {
	Name   string     // Int, List, Phase, a type variable, "()" for unit, "fn", "tuple", "record"
	Decl   *Type      // the declaration, when the name is a declared type
	Entity *Entity    // for Ref(Entity) and entity names
	Args   []*TypeExpr
	Fields []string // record type field names, matching Args
}

// Func is a function, method, lambda, job body, test body or initializer.
type Func struct {
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
	Pos      Pos
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
}

// Trigger starts a job: `@on(enters(view))` or `@on(leaves(view))`.
type Trigger struct {
	Kind string // "enters" or "leaves"
	View *View
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
}

// Op is a command or query of a contract.
type Op struct {
	Name      string
	Contract  *Contract
	Command   bool
	Params    []*Param
	Return    *TypeExpr
	Reconcile string // the query that resolves an Uncertain outcome
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
	// Call calls a function value with arguments.
	Call struct {
		At
		Func Expr
		Args []Arg
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

// Builtins are the functions every module can call without importing them.
// The interpreter provides them; a test checks the two lists agree.
var Builtins = []string{
	// output and failure
	"print", "unreachable",
	// tests
	"expect", "advance", "fakes", "clock",
	// time
	"sleep", "now",
	// collections, used with pipes
	"size", "isEmpty", "add", "remove", "contains", "insert", "get", "first", "last", "map",
	"filter", "find", "any", "all", "count", "reduce", "sum", "enumerate", "drop", "take",
	"reverse", "sort", "keys", "values", "entries", "toList", "toSet", "join", "flatMap", "zip",
	// strings and numbers
	"lines", "split", "trim", "parseInt", "nonzero", "min", "max", "abs",
	// view triggers, inside @on(…)
	"enters", "leaves",
}

// BuiltinModules are the standard modules every module can use without
// importing them, with their functions.
var BuiltinModules = map[string][]string{
	"log": {"debug", "info", "warn", "error"},
}
