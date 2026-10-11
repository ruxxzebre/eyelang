package lir

import (
	"fmt"

	"eyelang/ir"
)

// Verify checks what the low IR promises: every function instance is
// monomorphic (no value, parameter or result has a type variable), every
// block ends, every jump goes to a block of its function, every call names
// an instance of the program with as many arguments as it takes, and every
// value an instruction reads is a parameter or made earlier in the
// function. It returns a description of each problem.
func Verify(p *Program) []string {
	var problems []string
	funcs := map[*Func]bool{}
	for _, f := range p.Funcs {
		funcs[f] = true
	}
	for _, f := range p.Funcs {
		bad := func(format string, args ...any) {
			problems = append(problems, f.Name+": "+fmt.Sprintf(format, args...))
		}
		mono := func(what string, t ir.Ty) {
			if t == nil {
				bad("%s has no type", what)
			} else if generic(t) {
				bad("%s is %s, which has a type variable", what, t)
			}
		}
		mono("its result", f.Result)
		made := map[*Value]bool{}
		for _, v := range f.Params {
			mono("parameter "+v.String(), v.Ty)
			made[v] = true
		}
		blocks := map[*Block]bool{}
		for _, b := range f.Blocks {
			blocks[b] = true
		}
		if len(f.Blocks) == 0 {
			bad("it has no blocks")
		}
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				if d := dstOf(in); d != nil {
					mono(d.String(), d.Ty)
					made[d] = true
				}
				switch c := in.(type) {
				case *Call:
					if !funcs[c.Fn] {
						bad("b%d calls %s, which isn't an instance of the program", b.ID, c.Fn.Name)
					} else if len(c.Args) != len(c.Fn.Params) {
						bad("b%d calls %s with %d arguments; it takes %d", b.ID, c.Fn.Name, len(c.Args), len(c.Fn.Params))
					}
				case *MakeClosure:
					if !funcs[c.Fn] {
						bad("b%d makes a closure of %s, which isn't an instance of the program", b.ID, c.Fn.Name)
					}
				case *Global:
					if !funcs[c.Init] {
						bad("b%d reads the global of %s, which isn't an instance of the program", b.ID, c.Init.Name)
					}
				}
			}
			switch t := b.Term.(type) {
			case nil:
				bad("b%d doesn't end", b.ID)
			case *Jump:
				if !blocks[t.To] {
					bad("b%d jumps out of the function", b.ID)
				}
			case *Branch:
				if !blocks[t.Then] || !blocks[t.Else] {
					bad("b%d branches out of the function", b.ID)
				}
			}
		}
		// Every value read was made: a variable is made where it is first
		// set, which a loop may do after a block that reads it, so this
		// checks the function as a whole rather than in block order.
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				for _, v := range readsOf(in) {
					if v != nil && !made[v] {
						bad("b%d reads %s, which nothing makes", b.ID, v)
					}
				}
			}
			switch t := b.Term.(type) {
			case *Branch:
				if !made[t.Cond] {
					bad("b%d branches on %s, which nothing makes", b.ID, t.Cond)
				}
			case *Return:
				if t.Value != nil && !made[t.Value] {
					bad("b%d returns %s, which nothing makes", b.ID, t.Value)
				}
			}
		}
	}
	for _, d := range p.Dyn {
		if generic(d.Type) {
			problems = append(problems, fmt.Sprintf("dyn %s for %s has a type variable", d.Trait.Name, d.Type))
		}
	}
	return problems
}

func dstOf(in Instr) *Value {
	switch x := in.(type) {
	case *Const:
		return x.Dst
	case *Move:
		return x.Dst
	case *Call:
		return x.Dst
	case *Builtin:
		return x.Dst
	case *Runtime:
		return x.Dst
	case *CallValue:
		return x.Dst
	case *CallDyn:
		return x.Dst
	case *Op:
		return x.Dst
	case *Derived:
		return x.Dst
	case *MakeRecord:
		return x.Dst
	case *MakeVariant:
		return x.Dst
	case *MakeTuple:
		return x.Dst
	case *MakeList:
		return x.Dst
	case *MakeMap:
		return x.Dst
	case *MakeSet:
		return x.Dst
	case *MakeClosure:
		return x.Dst
	case *MakeDyn:
		return x.Dst
	case *Field:
		return x.Dst
	case *TupleGet:
		return x.Dst
	case *Payload:
		return x.Dst
	case *IsVariant:
		return x.Dst
	case *Update:
		return x.Dst
	case *Global:
		return x.Dst
	case *SetPlace:
		return x.Var // a place's variable may be set first here
	}
	return nil
}

func readsOf(in Instr) []*Value {
	switch x := in.(type) {
	case *Move:
		return []*Value{x.Src}
	case *Call:
		return x.Args
	case *Builtin:
		return x.Args
	case *Runtime:
		return x.Args
	case *CallValue:
		return append([]*Value{x.Callee}, x.Args...)
	case *CallDyn:
		return append([]*Value{x.Recv}, x.Args...)
	case *Op:
		return x.Args
	case *Derived:
		return x.Args
	case *MakeRecord:
		return x.Fields
	case *MakeVariant:
		return x.Args
	case *MakeTuple:
		return x.Elems
	case *MakeList:
		return x.Elems
	case *MakeMap:
		return append(append([]*Value{}, x.Keys...), x.Values...)
	case *MakeSet:
		return x.Elems
	case *MakeClosure:
		return x.Captures
	case *MakeDyn:
		return []*Value{x.Src}
	case *Field:
		return []*Value{x.Src}
	case *TupleGet:
		return []*Value{x.Src}
	case *Payload:
		return []*Value{x.Src}
	case *IsVariant:
		return []*Value{x.Src}
	case *Update:
		return append([]*Value{x.Src}, x.Fields...)
	case *SetPlace:
		return []*Value{x.Src}
	}
	return nil
}
