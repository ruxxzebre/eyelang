package cgen

import (
	"eyelang/ir"
	"eyelang/lir"
)

// Recursion too deep is a fault, not a crash (runtime/stack.c): each
// function that can call itself, directly or through others, checks at its
// start that the stack has room left, and faults with the interpreter's
// message when it hasn't. Functions that can't call themselves don't
// check: they only nest as deep as the program's calls are written, which
// the stack's margin holds.

// findRecursive marks the functions reached that are part of a cycle of
// calls: direct calls, the closures and dyn tables a function makes, the
// globals it reads, and the impls of Show its derived Shows call.
func (g *gen) findRecursive() {
	g.recursive = map[*lir.Func]bool{}
	edges := func(f *lir.Func) []*lir.Func {
		var out []*lir.Func
		add := func(c *lir.Func) {
			if c != nil && g.reach[c] {
				out = append(out, c)
			}
		}
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				switch x := in.(type) {
				case *lir.Call:
					add(x.Fn)
				case *lir.MakeClosure:
					add(x.Fn)
				case *lir.Global:
					add(x.Init)
				case *lir.MakeDyn:
					for _, m := range x.Table.Methods {
						add(m)
					}
				case *lir.Runtime:
					if fn, ok := x.Target.(*lir.Func); ok {
						add(fn)
					}
				case *lir.Derived:
					if x.Method == "show" && len(x.Args) == 1 {
						g.showsIn(x.Args[0].Ty, add)
					}
				case *lir.Builtin:
					if x.Name == "expect" && len(x.Args) == 4 {
						g.showsIn(x.Args[3].Ty, add)
					}
				}
			}
		}
		return out
	}
	// Tarjan's strongly connected components.
	index := map[*lir.Func]int{}
	low := map[*lir.Func]int{}
	on := map[*lir.Func]bool{}
	var stack []*lir.Func
	var visit func(f *lir.Func)
	visit = func(f *lir.Func) {
		index[f] = len(index)
		low[f] = index[f]
		stack = append(stack, f)
		on[f] = true
		self := false
		for _, c := range edges(f) {
			if c == f {
				self = true
			}
			if _, seen := index[c]; !seen {
				visit(c)
				low[f] = min(low[f], low[c])
			} else if on[c] {
				low[f] = min(low[f], index[c])
			}
		}
		if low[f] != index[f] {
			return
		}
		var scc []*lir.Func
		for {
			c := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			on[c] = false
			scc = append(scc, c)
			if c == f {
				break
			}
		}
		if len(scc) > 1 || self {
			for _, c := range scc {
				g.recursive[c] = true
			}
		}
	}
	for _, f := range g.funcs {
		if _, seen := index[f]; !seen {
			visit(f)
		}
	}
}

// showsIn calls add with each impl of Show reached that a derived Show of
// a value of type t calls.
func (g *gen) showsIn(t ir.Ty, add func(*lir.Func)) {
	if len(g.shown) == 0 {
		return
	}
	lir.WalkShown(t, func(u ir.Ty) bool {
		if fn := g.shown[lir.TypeKey(u)]; fn != nil {
			add(fn)
			return false
		}
		return true
	})
}

// checkDepth writes the start of a recursive function: a fault when the
// stack is full.
func (e *fn) checkDepth() {
	if e.g.recursive[e.f] {
		name := e.f.Src.Name
		e.b.WriteString("\tEY_STACK_CHECK(" + e.at + ", " + cstring(name) + ");\n")
	}
}
