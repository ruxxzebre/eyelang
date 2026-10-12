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
// their own (domain.go). Reference counts (dup, drop, reuse) come with 7.5.
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
	values int
}

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
	// Move copies a value into a variable.
	Move struct{ Dst, Src *Value }
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
	Builtin struct {
		Dst     *Value
		Name    string
		Args    []*Value
		Names   []string // each argument's name, "" for a positional one
		Durable bool
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
	// >= ! and unary -.
	Op struct {
		Dst  *Value
		Op   string
		Args []*Value
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
	// variant's field, once its tag is known.
	Field struct {
		Dst  *Value
		Src  *Value
		Name string
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
	Update struct {
		Dst    *Value
		Src    *Value
		Names  []string
		Fields []*Value
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
	for _, p := range f.Params {
		params = append(params, p.String()+": "+tyString(p.Ty))
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
		return dst(x.Dst) + x.Src.String()
	case *Call:
		return dst(x.Dst) + "call" + step(x.Durable) + " " + x.Fn.Name + "(" + values(x.Args) + ")"
	case *Builtin:
		return dst(x.Dst) + "builtin" + step(x.Durable) + " " + x.Name + "(" + named(x.Args, x.Names) + ")"
	case *Runtime:
		return dst(x.Dst) + "runtime" + step(x.Durable) + " " + x.Op + " " + targetString(x.Target) + "(" + named(x.Args, x.Names) + ")"
	case *CallValue:
		return dst(x.Dst) + "callvalue" + step(x.Durable) + " " + x.Callee.String() + "(" + values(x.Args) + ")"
	case *CallDyn:
		return dst(x.Dst) + "calldyn " + x.Recv.String() + "." + x.Trait.Name + "." + x.Method + "(" + values(x.Args) + ")"
	case *Op:
		return dst(x.Dst) + "op " + x.Op + " " + values(x.Args)
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
		return dst(x.Dst) + x.Src.String() + "." + x.Name
	case *TupleGet:
		return dst(x.Dst) + fmt.Sprintf("%s.%d", x.Src, x.Index)
	case *Payload:
		return dst(x.Dst) + fmt.Sprintf("%s as %s.%d", x.Src, x.Variant.Name, x.Index)
	case *IsVariant:
		return dst(x.Dst) + x.Src.String() + " is " + x.Variant.Name
	case *Update:
		return dst(x.Dst) + "{" + x.Src.String() + " with " + named(x.Fields, x.Names) + "}"
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
