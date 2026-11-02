package lir

import "eyelang/ir"

// Compiled jobs (ROADMAP 7.6; docs/internals/compilation.md, "Compiled
// jobs"). A function with a `!` step in it stops at each one: it saves the
// variables live there and returns, and the runner calls it again with what
// the step gave. What the back end needs for that is here: which functions
// stop, and which variables are live at each place they stop.
//
// A step and every other call into the runtime is named by its Site: the
// expression of the typed IR it came from. The runner holds the same typed
// IR, so it evaluates that expression with the values compiled code gives
// it, and so keeps one implementation of each operation, its steps, its
// fakes and its replay.

// Stops reports whether f stops at a `!` step of its own.
func Stops(f *Func) bool {
	for _, b := range f.Blocks {
		for _, in := range b.Instrs {
			if Durable(in) {
				return true
			}
		}
	}
	return false
}

// Durable reports whether an instruction is a `!` step.
func Durable(in Instr) bool {
	switch x := in.(type) {
	case *Call:
		return x.Durable
	case *Builtin:
		return x.Durable
	case *Runtime:
		return x.Durable
	case *CallValue:
		return x.Durable
	}
	return false
}

// SiteOf is the expression an instruction came from, if it has one.
func SiteOf(in Instr) ir.Expr {
	switch x := in.(type) {
	case *Call:
		return x.Site
	case *Builtin:
		return x.Site
	case *Runtime:
		return x.Site
	case *CallValue:
		return x.Site
	}
	return nil
}

// PrepareSteps readies the functions that stop, before Count: a for loop
// over a list walks it by index, since a walk's place must survive in a
// frame and an Iter can't be saved. A loop over anything else keeps its
// Iter, which the back end reports if it is live across a step.
func PrepareSteps(p *Program) {
	for _, f := range p.Funcs {
		if Stops(f) {
			indexLoops(f)
		}
	}
}

func indexLoops(f *Func) {
	for _, b := range f.Blocks {
		for i := 0; i < len(b.Instrs); i++ {
			it, ok := b.Instrs[i].(*Builtin)
			if !ok || it.Name != "iter" || len(it.Args) != 1 || !isList(it.Args[0].Ty) {
				continue
			}
			if !walkedByIndex(f, it.Dst) {
				continue
			}
			list := it.Args[0]
			n := f.newValue(tyInt)
			idx := f.newValue(tyInt)
			// The list stays in a variable of its own: the walk reads it to
			// the end of the loop.
			held := f.newValue(list.Ty)
			head := []Instr{
				&Move{Dst: held, Src: list},
				&Builtin{Dst: n, Name: "size", Args: []*Value{held}},
				&Const{Dst: idx, Value: int64(0)},
			}
			b.Instrs = append(b.Instrs[:i:i], append(head, b.Instrs[i+1:]...)...)
			rewriteWalk(f, it.Dst, held, n, idx)
		}
	}
}

// walkedByIndex reports whether only iter.more and iter.next read an Iter.
func walkedByIndex(f *Func, it *Value) bool {
	for _, b := range f.Blocks {
		for _, in := range b.Instrs {
			for _, r := range readsOf(in) {
				if r != it {
					continue
				}
				x, ok := in.(*Builtin)
				if !ok || x.Name != "iter.more" && x.Name != "iter.next" {
					return false
				}
			}
		}
	}
	return true
}

func rewriteWalk(f *Func, it, list, n, idx *Value) {
	for _, b := range f.Blocks {
		for i := 0; i < len(b.Instrs); i++ {
			x, ok := b.Instrs[i].(*Builtin)
			if !ok || len(x.Args) != 1 || x.Args[0] != it {
				continue
			}
			switch x.Name {
			case "iter.more":
				b.Instrs[i] = &Op{Dst: x.Dst, Op: "<", Args: []*Value{idx, n}}
			case "iter.next":
				one := f.newValue(tyInt)
				next := []Instr{
					&Builtin{Dst: x.Dst, Name: "list.elem", Args: []*Value{list, idx}},
					&Const{Dst: one, Value: int64(1)},
					&Op{Dst: idx, Op: "+", Args: []*Value{idx, one}},
				}
				b.Instrs = append(b.Instrs[:i:i], append(next, b.Instrs[i+1:]...)...)
			}
		}
	}
}

func isList(t ir.Ty) bool {
	c, ok := Resolve(t).(*ir.TCon)
	return ok && c.Decl == nil && c.Name == "List"
}

// newValue makes an intermediate value of f.
func (f *Func) newValue(t ir.Ty) *Value {
	f.values++
	return &Value{ID: f.values, Ty: t}
}

// LiveAcross gives, for each step of f, the variables that hold a value it
// must keep across that step: read after it, and set on some way to it. The
// step's own result isn't one of them.
func LiveAcross(f *Func) map[Instr][]*Value {
	type sets struct{ in, out map[*Value]bool }
	state := map[*Block]*sets{}
	for _, b := range f.Blocks {
		state[b] = &sets{map[*Value]bool{}, map[*Value]bool{}}
	}
	transfer := func(in Instr, live map[*Value]bool) {
		if d := defOf(in); d != nil {
			delete(live, d)
		}
		for _, r := range readsOf(in) {
			if r != nil {
				live[r] = true
			}
		}
		if s, ok := in.(*SetPlace); ok && len(s.Path) > 0 {
			live[s.Var] = true
		}
	}
	termReads := func(t Term, live map[*Value]bool) {
		switch x := t.(type) {
		case *Branch:
			live[x.Cond] = true
		case *Return:
			if x.Value != nil {
				live[x.Value] = true
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for i := len(f.Blocks) - 1; i >= 0; i-- {
			b := f.Blocks[i]
			st := state[b]
			for _, s := range succs(b) {
				for v := range state[s].in {
					st.out[v] = true
				}
			}
			live := map[*Value]bool{}
			for v := range st.out {
				live[v] = true
			}
			termReads(b.Term, live)
			for j := len(b.Instrs) - 1; j >= 0; j-- {
				transfer(b.Instrs[j], live)
			}
			for v := range live {
				if !st.in[v] {
					st.in[v] = true
					changed = true
				}
			}
		}
	}
	// A value is kept only if something can have set it before the step.
	setBefore := func() map[*Block]map[*Value]bool {
		in := map[*Block]map[*Value]bool{}
		for _, b := range f.Blocks {
			in[b] = map[*Value]bool{}
		}
		if len(f.Blocks) > 0 {
			for _, p := range f.Params {
				in[f.Blocks[0]][p] = true
			}
		}
		for changed := true; changed; {
			changed = false
			for _, b := range f.Blocks {
				cur := map[*Value]bool{}
				for v := range in[b] {
					cur[v] = true
				}
				for _, ins := range b.Instrs {
					if d := dstOf(ins); d != nil {
						cur[d] = true
					}
				}
				for _, s := range succs(b) {
					for v := range cur {
						if !in[s][v] {
							in[s][v] = true
							changed = true
						}
					}
				}
			}
		}
		return in
	}()
	out := map[Instr][]*Value{}
	for _, b := range f.Blocks {
		live := map[*Value]bool{}
		for v := range state[b].out {
			live[v] = true
		}
		termReads(b.Term, live)
		before := make([]map[*Value]bool, len(b.Instrs))
		for j := len(b.Instrs) - 1; j >= 0; j-- {
			if Durable(b.Instrs[j]) {
				m := map[*Value]bool{}
				for v := range live {
					m[v] = true
				}
				before[j] = m
			}
			transfer(b.Instrs[j], live)
		}
		seen := map[*Value]bool{}
		for v := range setBefore[b] {
			seen[v] = true
		}
		for j, ins := range b.Instrs {
			if before[j] != nil {
				d := dstOf(ins)
				var vs []*Value
				for v := range before[j] {
					if v != d && seen[v] {
						vs = append(vs, v)
					}
				}
				sortValues(vs)
				out[ins] = vs
			}
			if d := dstOf(ins); d != nil {
				seen[d] = true
			}
		}
	}
	return out
}

func sortValues(vs []*Value) {
	for i := 1; i < len(vs); i++ {
		for j := i; j > 0 && vs[j-1].ID > vs[j].ID; j-- {
			vs[j-1], vs[j] = vs[j], vs[j-1]
		}
	}
}
