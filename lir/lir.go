// Package lir is the low IR (ROADMAP 7.2): the typed IR made monomorphic and
// flat, the form C is printed from (docs/internals/compilation.md, section
// 5).
//
// A program is a set of function instances, one per generic function and
// the types it is used at, reached from the program's roots: main, tests,
// jobs, entity methods, views, served operations, migrations, schedules and
// module-level lets. Each instance is a list of blocks; each block holds
// instructions that name every value they make, and ends in a jump, a
// branch, a return or a stop. No type in it has a type variable.
//
// Calls are resolved: to an instance, a built-in, the runtime (an entity
// call, a contract's operation, a job's start), a closure's value or a dyn
// value's table. Entities, jobs, views and contracts are lowered by passes of
// their own (domain.go).
//
// Count (count.go) makes reference counting explicit (7.5): Dup and Drop
// instructions, the parameters each function owns, updates in place and
// fields taken out of a record about to be rebuilt. Layouts (layout.go)
// says how each type's values are laid out (7.13), which also decides
// which values are counted.
package lir

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"eyelang/ir"
)

// Program is a whole program in low IR.
type Program struct {
	Funcs []*Func // in the order they were reached
	// Dyn lists, for each type converted to a dyn Trait, the instances that
	// answer the trait's methods: a dyn value's table.
	Dyn []*DynTable
	// MainErrShow is the instance of Show that writes main's error, when
	// main returns a Result whose error type has an impl of Show; without
	// one, the error is written in its derived form (SPECS.md §13).
	MainErrShow *Func
	// Layouts says how each type's values are laid out; Count sets it.
	Layouts *Layouts
}

// Kind says what a function instance is, which the runtime needs to know.
type Kind int

const (
	KindFunc      Kind = iota // a function or a type's method
	KindLambda                // a lambda, lifted: its captures come first
	KindEntity                // an entity method: the state comes first, and is written back
	KindJob                   // a job's body: its durable steps are marked
	KindView                  // a view's pipeline
	KindServe                 // a served contract operation
	KindTest                  // a test's body
	KindGlobal                // a module-level let, a state default or a schedule
	KindMigration             // an entity migration, from an old row
)

var kindNames = [...]string{"fn", "lambda", "entity", "job", "view", "serve", "test", "global", "migration"}

func (k Kind) String() string { return kindNames[k] }

// Func is one instance of a function at one set of types.
type Func struct {
	Name string   // unique in the program: module.name, with its types when generic
	Src  *ir.Func // what it was made from
	Kind Kind
	// Params are its parameters: a lambda's captures first, then a method's
	// self or an entity method's state, then those it declares. A call
	// passes Absent for one it leaves out, and the function's first blocks
	// give it its default.
	Params []*Value
	Result ir.Ty
	Blocks []*Block // the first is where it starts
	// Modes says how it takes each parameter, once Count has decided:
	// borrowed, owned or mut. Nil before.
	Modes  []Mode
	values int
}

// Mode is how a function takes a parameter of a counted type (Count).
type Mode uint8

const (
	// Borrowed: the caller keeps its reference for the call, and the
	// function only reads the value, or dups what it keeps.
	Borrowed Mode = iota
	// Owned: the caller gives the function a reference, which the function
	// drops or passes on; an update of the value is in place when the
	// reference is the only one.
	Owned
	// Mut: a `mut` parameter, the address of the caller's variable, whose
	// reference stays the caller's.
	Mut
)

var modeNames = [...]string{"borrowed", "owned", "mut"}

func (m Mode) String() string { return modeNames[m] }

// Value is a named value: a variable, a parameter or an intermediate.
type Value struct {
	ID   int
	Name string // the variable's name, empty for an intermediate
	Ty   ir.Ty
}

func (v *Value) String() string {
	if v == nil {
		return "_"
	}
	if v.Name != "" {
		return fmt.Sprintf("%%%d.%s", v.ID, v.Name)
	}
	return fmt.Sprintf("%%%d", v.ID)
}

// Block is a straight run of instructions and how it ends.
type Block struct {
	ID     int
	Instrs []Instr
	Term   Term
}

// Instr is one instruction. Most make a value, Dst.
type Instr interface{ isInstr() }

type (
	// Const is a constant: the same values as ir.Lit, Absent for a left-out
	// argument, or Unit.
	Const struct {
		Dst   *Value
		Value any
	}
	// Move copies a value into a variable. It takes Src's reference, unless
	// Borrow, set by Count, says that Dst only borrows what Src borrows: a
	// `let` of a field or of a borrowed parameter, valid while that is.
	Move struct {
		Dst, Src *Value
		Borrow   bool
	}
	// Call calls an instance. Durable marks a `!` step of a job. Mut lists
	// the arguments passed as `mut`, by their index: the callee changes the
	// caller's variable.
	Call struct {
		Dst     *Value
		Fn      *Func
		Args    []*Value
		Mut     []int
		Durable bool
	}
	// Builtin calls a built-in function, by its name, with named arguments
	// as written: the back end specialises it by its arguments' types.
	//
	// InPlace, set by Count, says that the first argument's last use is
	// this one and the built-in takes it: an update (add, insert, remove,
	// concat) changes it in place when it holds the only reference, and
	// copies it otherwise. Without it every argument is borrowed.
	Builtin struct {
		Dst     *Value
		Name    string
		Args    []*Value
		Names   []string // each argument's name, "" for a positional one
		Durable bool
		InPlace bool
	}
	// Runtime asks the runtime: an entity's state or method, a contract's
	// operation, a job's start, an event, a fake. Target is what it names.
	Runtime struct {
		Dst     *Value
		Op      string // "entity.call", "entity.read", "op.call", "job.start", …
		Target  any    // *Func, *ir.Entity, *ir.Op, *ir.Job, *ir.Event, …
		Args    []*Value
		Names   []string
		Durable bool
	}
	// CallValue calls a closure.
	CallValue struct {
		Dst     *Value
		Callee  *Value
		Args    []*Value
		Durable bool
	}
	// CallDyn calls a dyn value's method through its table.
	CallDyn struct {
		Dst    *Value
		Recv   *Value
		Trait  *ir.Trait
		Method string
		Args   []*Value
	}
	// Op is a built-in operator on built-in types: + - * / % == != < <= >
	// >= ! and unary -. InPlace, set by Count, is as for Builtin: `+` on a
	// String, List or Set takes its first operand and appends to it.
	Op struct {
		Dst     *Value
		Op      string
		Args    []*Value
		InPlace bool
	}
	// Derived calls a derived trait's method for a type: show, eq, cmp or
	// hash, which the back end writes for each type.
	Derived struct {
		Dst    *Value
		Method string
		Args   []*Value
	}
	// MakeRecord, MakeVariant, MakeTuple, MakeList, MakeMap and MakeSet
	// build values; MakeClosure a lifted lambda with its captures; MakeDyn a
	// dyn value from a value and its type's table.
	MakeRecord struct {
		Dst    *Value
		Type   *ir.Type
		Names  []string
		Fields []*Value
	}
	MakeVariant struct {
		Dst     *Value
		Variant *ir.Variant
		Args    []*Value
	}
	MakeTuple struct {
		Dst   *Value
		Elems []*Value
	}
	MakeList struct {
		Dst   *Value
		Elems []*Value
	}
	MakeMap struct {
		Dst          *Value
		Keys, Values []*Value
	}
	MakeSet struct {
		Dst   *Value
		Elems []*Value
	}
	MakeClosure struct {
		Dst      *Value
		Fn       *Func
		Captures []*Value
	}
	MakeDyn struct {
		Dst   *Value
		Src   *Value
		Table *DynTable
	}
	// Field reads a record's field; TupleGet a tuple's element; Payload a
	// variant's field, once its tag is known. What they read is borrowed
	// from Src (Count): valid while Src is, without a count of its own.
	//
	// Take, set by Count, moves the field out instead: Src is about to be
	// rebuilt with a new value for it (an Update or a SetPlace later in the
	// block), so when Src holds its record inline, or the only reference to
	// it, the field is moved out and left empty (zero, which drops to
	// nothing) for the update to fill; otherwise the field is dup'ed. The
	// value is owned, so an update of it can be in place.
	Field struct {
		Dst  *Value
		Src  *Value
		Name string
		Take bool
	}
	TupleGet struct {
		Dst   *Value
		Src   *Value
		Index int
	}
	Payload struct {
		Dst     *Value
		Src     *Value
		Variant *ir.Variant
		Index   int
	}
	// IsVariant tests a variant's tag.
	IsVariant struct {
		Dst     *Value
		Src     *Value
		Variant *ir.Variant
	}
	// Update makes a record with some fields replaced: `{ r with f: v }`.
	// InPlace, set by Count, says that this is Src's last use: the update
	// takes Src and changes it in place when it holds the only reference,
	// dropping the fields it replaces. Without it Src is borrowed and
	// copied.
	Update struct {
		Dst     *Value
		Src     *Value
		Names   []string
		Fields  []*Value
		InPlace bool
	}
	// SetPlace writes a variable's value, or a field or an element inside
	// it: an element of a list, which must be there, or a map's value for a
	// key, which is inserted or replaced (SPECS.md §7).
	SetPlace struct {
		Var  *Value
		Path []Step
		Src  *Value
	}
	// Global reads a module-level let, computed once by its instance.
	Global struct {
		Dst  *Value
		Init *Func
	}
	// At marks where in the source the instructions after it come from,
	// until the next At: a fault there names this position, and the C back
	// end writes a #line directive for it. Every block whose instructions
	// can fault starts with one. Fprint leaves it out.
	At struct{ Pos ir.Pos }
	// Dup adds a reference to a counted value, which an instruction after
	// it takes: a value passed on while it is still needed, or a borrowed
	// one that is kept (Count).
	Dup struct{ V *Value }
	// Drop gives up a counted value's reference: after its last use, or on
	// the way into code that doesn't use it (Count).
	Drop struct{ V *Value }
)

func (*Const) isInstr()       {}
func (*Move) isInstr()        {}
func (*Call) isInstr()        {}
func (*Builtin) isInstr()     {}
func (*Runtime) isInstr()     {}
func (*CallValue) isInstr()   {}
func (*CallDyn) isInstr()     {}
func (*Op) isInstr()          {}
func (*Derived) isInstr()     {}
func (*MakeRecord) isInstr()  {}
func (*MakeVariant) isInstr() {}
func (*MakeTuple) isInstr()   {}
func (*MakeList) isInstr()    {}
func (*MakeMap) isInstr()     {}
func (*MakeSet) isInstr()     {}
func (*MakeClosure) isInstr() {}
func (*MakeDyn) isInstr()     {}
func (*Field) isInstr()       {}
func (*TupleGet) isInstr()    {}
func (*Payload) isInstr()     {}
func (*IsVariant) isInstr()   {}
func (*Update) isInstr()      {}
func (*SetPlace) isInstr()    {}
func (*Global) isInstr()      {}
func (*At) isInstr()          {}
func (*Dup) isInstr()         {}
func (*Drop) isInstr()        {}

// Step is one step of a SetPlace's path: a field, or the element at Index.
type Step struct {
	Field string // "" for an element
	Index *Value // nil for a field
}

// FieldSteps are the steps of a path of field names.
func FieldSteps(path []ir.Step) []Step {
	steps := make([]Step, len(path))
	for i, s := range path {
		steps[i] = Step{Field: s.Field}
	}
	return steps
}

// Absent is the value a call passes for a parameter it leaves out.
type Absent struct{}

// Term ends a block.
type Term interface{ isTerm() }

type (
	Jump   struct{ To *Block }
	Branch struct {
		Cond       *Value
		Then, Else *Block
	}
	Return struct{ Value *Value }
	// Stop ends the program's run of this code with a fault: a match no arm
	// took, an `unreachable`.
	Stop struct{ Why string }
)

func (*Jump) isTerm()   {}
func (*Branch) isTerm() {}
func (*Return) isTerm() {}
func (*Stop) isTerm()   {}

// DynTable is a type's impl of a trait, as a dyn value's table.
type DynTable struct {
	Type    ir.Ty
	Trait   *ir.Trait
	Methods map[string]*Func
}

// Own is the part of a program made from the program's own modules, not
// from the prelude, the standard modules and the store: what `eye check
// --lir` prints.
func Own(p *Program, prog *ir.Program) *Program {
	own := &Program{}
	mine := func(f *ir.Func) bool {
		return f != nil && f.Module != nil && (f.Module.Dir != "" || f.Module == prog.Root)
	}
	for _, f := range p.Funcs {
		if mine(f.Src) {
			own.Funcs = append(own.Funcs, f)
		}
	}
	for _, d := range p.Dyn {
		if c, ok := d.Type.(*ir.TCon); ok && c.Decl != nil && c.Decl.Module != nil && (c.Decl.Module.Dir != "" || c.Decl.Module == prog.Root) {
			own.Dyn = append(own.Dyn, d)
		}
	}
	return own
}

// ---------------------------------------------------------------- printing

// Fprint writes the program, one instance after another.
func Fprint(w io.Writer, p *Program) {
	for i, f := range p.Funcs {
		if i > 0 {
			fmt.Fprintln(w)
		}
		FprintFunc(w, f)
	}
	for _, d := range p.Dyn {
		names := make([]string, 0, len(d.Methods))
		for n := range d.Methods {
			names = append(names, n)
		}
		sort.Strings(names)
		fmt.Fprintf(w, "\ndyn %s for %s\n", d.Trait.Name, d.Type)
		for _, n := range names {
			fmt.Fprintf(w, "  %s = %s\n", n, d.Methods[n].Name)
		}
	}
}

// FprintFunc writes one instance.
func FprintFunc(w io.Writer, f *Func) {
	var params []string
	for i, p := range f.Params {
		s := p.String() + ": " + tyString(p.Ty)
		if i < len(f.Modes) && f.Modes[i] != Borrowed {
			s += " " + f.Modes[i].String()
		}
		params = append(params, s)
	}
	fmt.Fprintf(w, "%s %s(%s) -> %s\n", f.Kind, f.Name, strings.Join(params, ", "), tyString(f.Result))
	for _, b := range f.Blocks {
		fmt.Fprintf(w, "b%d:\n", b.ID)
		for _, in := range b.Instrs {
			if _, at := in.(*At); at {
				continue
			}
			fmt.Fprintf(w, "  %s\n", instrString(in))
		}
		fmt.Fprintf(w, "  %s\n", termString(b.Term))
	}
}

func tyString(t ir.Ty) string {
	if t == nil {
		return "?"
	}
	return t.String()
}

func values(vs []*Value) string {
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = v.String()
	}
	return strings.Join(parts, ", ")
}

func named(vs []*Value, names []string) string {
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = v.String()
		if i < len(names) && names[i] != "" {
			parts[i] = names[i] + ": " + parts[i]
		}
	}
	return strings.Join(parts, ", ")
}

func dst(v *Value) string {
	if v == nil {
		return ""
	}
	return v.String() + ": " + tyString(v.Ty) + " = "
}

func inPlace(in bool) string {
	if in {
		return "inplace "
	}
	return ""
}

func step(durable bool) string {
	if durable {
		return "!"
	}
	return ""
}

func instrString(in Instr) string {
	switch x := in.(type) {
	case *Const:
		switch v := x.Value.(type) {
		case string:
			return dst(x.Dst) + fmt.Sprintf("%q", v)
		case Absent:
			return dst(x.Dst) + "absent"
		case ir.Unit:
			return dst(x.Dst) + "()"
		}
		return dst(x.Dst) + fmt.Sprint(x.Value)
	case *Move:
		if x.Borrow {
			return dst(x.Dst) + "borrow " + x.Src.String()
		}
		return dst(x.Dst) + x.Src.String()
	case *Call:
		return dst(x.Dst) + "call" + step(x.Durable) + " " + x.Fn.Name + "(" + values(x.Args) + ")"
	case *Builtin:
		return dst(x.Dst) + inPlace(x.InPlace) + "builtin" + step(x.Durable) + " " + x.Name + "(" + named(x.Args, x.Names) + ")"
	case *Runtime:
		return dst(x.Dst) + "runtime" + step(x.Durable) + " " + x.Op + " " + targetString(x.Target) + "(" + named(x.Args, x.Names) + ")"
	case *CallValue:
		return dst(x.Dst) + "callvalue" + step(x.Durable) + " " + x.Callee.String() + "(" + values(x.Args) + ")"
	case *CallDyn:
		return dst(x.Dst) + "calldyn " + x.Recv.String() + "." + x.Trait.Name + "." + x.Method + "(" + values(x.Args) + ")"
	case *Op:
		return dst(x.Dst) + inPlace(x.InPlace) + "op " + x.Op + " " + values(x.Args)
	case *Derived:
		return dst(x.Dst) + "derived " + x.Method + "(" + values(x.Args) + ")"
	case *MakeRecord:
		name := "record"
		if x.Type != nil {
			name = x.Type.Name
		}
		return dst(x.Dst) + name + " {" + named(x.Fields, x.Names) + "}"
	case *MakeVariant:
		if len(x.Args) == 0 {
			return dst(x.Dst) + x.Variant.Name
		}
		return dst(x.Dst) + x.Variant.Name + "(" + values(x.Args) + ")"
	case *MakeTuple:
		return dst(x.Dst) + "(" + values(x.Elems) + ")"
	case *MakeList:
		return dst(x.Dst) + "[" + values(x.Elems) + "]"
	case *MakeMap:
		parts := make([]string, len(x.Keys))
		for i := range x.Keys {
			parts[i] = x.Keys[i].String() + ": " + x.Values[i].String()
		}
		return dst(x.Dst) + "map {" + strings.Join(parts, ", ") + "}"
	case *MakeSet:
		return dst(x.Dst) + "set {" + values(x.Elems) + "}"
	case *MakeClosure:
		return dst(x.Dst) + "closure " + x.Fn.Name + "[" + values(x.Captures) + "]"
	case *MakeDyn:
		return dst(x.Dst) + "dyn " + x.Table.Trait.Name + " " + x.Src.String()
	case *Field:
		if x.Take {
			return dst(x.Dst) + "take " + x.Src.String() + "." + x.Name
		}
		return dst(x.Dst) + x.Src.String() + "." + x.Name
	case *TupleGet:
		return dst(x.Dst) + fmt.Sprintf("%s.%d", x.Src, x.Index)
	case *Payload:
		return dst(x.Dst) + fmt.Sprintf("%s as %s.%d", x.Src, x.Variant.Name, x.Index)
	case *IsVariant:
		return dst(x.Dst) + x.Src.String() + " is " + x.Variant.Name
	case *Update:
		return dst(x.Dst) + inPlace(x.InPlace) + "{" + x.Src.String() + " with " + named(x.Fields, x.Names) + "}"
	case *SetPlace:
		path := x.Var.String()
		for _, p := range x.Path {
			if p.Index != nil {
				path += "[" + p.Index.String() + "]"
				continue
			}
			path += "." + p.Field
		}
		return "set " + path + " = " + x.Src.String()
	case *Global:
		return dst(x.Dst) + "global " + x.Init.Name
	case *At:
		return "at " + x.Pos.String()
	case *Dup:
		return "dup " + x.V.String()
	case *Drop:
		return "drop " + x.V.String()
	}
	return fmt.Sprintf("%T", in)
}

func targetString(t any) string {
	switch t := t.(type) {
	case *Func:
		return t.Name
	case *ir.Entity:
		return t.Name
	case *ir.Op:
		return t.Contract.Name + "." + t.Name
	case *ir.Job:
		return t.Name
	case *ir.Event:
		return t.Contract.Name + "." + t.Name
	case *ir.Variant:
		return t.Name
	case *ir.View:
		return t.Name
	case string:
		return t
	case nil:
		return "-"
	}
	return fmt.Sprintf("%T", t)
}

func termString(t Term) string {
	switch x := t.(type) {
	case *Jump:
		return fmt.Sprintf("jump b%d", x.To.ID)
	case *Branch:
		return fmt.Sprintf("branch %s b%d b%d", x.Cond, x.Then.ID, x.Else.ID)
	case *Return:
		return "return " + x.Value.String()
	case *Stop:
		return "stop: " + x.Why
	case nil:
		return "(no end)"
	}
	return fmt.Sprintf("%T", t)
}
