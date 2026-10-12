package lir

import (
	"fmt"
	"sort"
	"strings"
)

// CheckCounts checks what Count promises, by following each function's
// paths with the number of references each value holds: a value is read
// only while it holds one, or while what it borrows from does; it is taken
// or dropped only when it holds one; a variable is set only once its old
// value is gone; every path into a block arrives with the same counts;
// and a return leaves nothing held but `mut` parameters. Code that ends in
// a fault may leak (7.12). It returns a description of each problem.
func CheckCounts(p *Program) []string {
	ls := p.Layouts
	if ls == nil {
		ls = NewLayouts()
	}
	var problems []string
	for _, f := range p.Funcs {
		problems = append(problems, checkFunc(f, ls)...)
	}
	return problems
}

type holds map[*Value]int

func (h holds) copy() holds {
	n := holds{}
	for v, k := range h {
		if k != 0 {
			n[v] = k
		}
	}
	return n
}

func (h holds) String() string {
	var parts []string
	for v, k := range h {
		if k != 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", v, k))
		}
	}
	sort.Strings(parts)
	return "{" + strings.Join(parts, " ") + "}"
}

func sameHolds(a, b holds) bool {
	for v, k := range a {
		if b[v] != k {
			return false
		}
	}
	for v, k := range b {
		if a[v] != k {
			return false
		}
	}
	return true
}

func checkFunc(f *Func, ls *Layouts) []string {
	var problems []string
	bad := func(b *Block, format string, args ...any) {
		problems = append(problems, fmt.Sprintf("%s: b%d: %s", f.Name, b.ID, fmt.Sprintf(format, args...)))
	}
	if len(f.Modes) != len(f.Params) {
		return []string{f.Name + ": it isn't counted"}
	}
	// What each value is: static, borrowed (from what), or owned.
	static := map[*Value]bool{}
	borrowed := map[*Value]bool{}
	root := map[*Value]*Value{}
	start := holds{}
	for i, p := range f.Params {
		if !ls.Counted(p.Ty) {
			continue
		}
		switch f.Modes[i] {
		case Borrowed:
			borrowed[p] = true
		default:
			start[p] = 1
		}
	}
	for _, b := range f.Blocks {
		for _, in := range b.Instrs {
			d := defOf(in)
			if d == nil || !ls.Counted(d.Ty) {
				continue
			}
			switch x := in.(type) {
			case *Const:
				static[d] = true
			case *Move:
				if x.Borrow {
					if static[x.Src] {
						static[d] = true
					} else {
						borrowed[d] = true
					}
				}
			default:
				if borrowingDef(in) {
					borrowed[d] = true
				}
			}
		}
	}
	// What a borrowed value borrows from: the source of its definition, or
	// what that borrows from.
	for _, b := range f.Blocks {
		for _, in := range b.Instrs {
			d := defOf(in)
			if d == nil || !borrowed[d] {
				continue
			}
			var src *Value
			switch x := in.(type) {
			case *Field:
				src = x.Src
			case *TupleGet:
				src = x.Src
			case *Payload:
				src = x.Src
			case *Move:
				src = x.Src
			}
			if src != nil {
				if r, ok := root[src]; ok {
					src = r
				}
				if !borrowed[src] && !static[src] {
					root[d] = src
				} else if r, ok := root[src]; ok {
					root[d] = r
				}
			}
		}
	}
	tracked := func(v *Value) bool { return v != nil && ls.Counted(v.Ty) && !static[v] }
	readable := func(h holds, v *Value) bool {
		if !tracked(v) || h[v] > 0 {
			return true
		}
		if !borrowed[v] {
			return false
		}
		r, ok := root[v]
		return !ok || h[r] > 0
	}
	mutParams := map[*Value]bool{}
	for i, p := range f.Params {
		if f.Modes[i] == Mut && tracked(p) {
			mutParams[p] = true
		}
	}
	in := map[*Block]holds{}
	in[f.Blocks[0]] = start
	work := []*Block{f.Blocks[0]}
	for len(work) > 0 {
		b := work[0]
		work = work[1:]
		h := in[b].copy()
		_, stops := b.Term.(*Stop)
		ok := true
		for _, ins := range b.Instrs {
			if !ok {
				break
			}
			switch x := ins.(type) {
			case *At:
				continue
			case *Dup:
				if !readable(h, x.V) {
					bad(b, "dup %s, which holds nothing", x.V)
					ok = false
				}
				h[x.V]++
				continue
			case *Drop:
				if h[x.V] < 1 {
					bad(b, "drop %s, which holds nothing", x.V)
					ok = false
				}
				h[x.V]--
				continue
			}
			takes, reads := takesAndReads(ins)
			for _, v := range reads {
				if !readable(h, v) {
					bad(b, "%s reads %s, which holds nothing", instrString(ins), v)
					ok = false
				}
			}
			for _, v := range takes {
				if !tracked(v) {
					continue
				}
				if h[v] < 1 {
					bad(b, "%s takes %s, which holds nothing", instrString(ins), v)
					ok = false
				}
				h[v]--
			}
			if d := defOf(ins); tracked(d) {
				if h[d] != 0 && !stops {
					bad(b, "%s sets %s, which still holds %d", instrString(ins), d, h[d])
					ok = false
				}
				if borrowed[d] {
					h[d] = 0
				} else {
					h[d] = 1
				}
			}
		}
		if !ok || stops {
			continue
		}
		switch t := b.Term.(type) {
		case *Return:
			if tracked(t.Value) {
				if h[t.Value] < 1 {
					bad(b, "return %s, which holds nothing", t.Value)
				}
				h[t.Value]--
			}
			for v, k := range h {
				want := 0
				if mutParams[v] {
					want = 1
				}
				if k != want {
					bad(b, "returns with %s holding %d", v, k)
				}
			}
		case *Branch, *Jump:
			for _, s := range succs(b) {
				if _, stop := s.Term.(*Stop); stop {
					if _, seen := in[s]; !seen {
						in[s] = h.copy()
						work = append(work, s)
					}
					continue
				}
				if prev, seen := in[s]; seen {
					if !sameHolds(prev, h) {
						bad(s, "paths arrive holding %s and %s", prev, h)
					}
					continue
				}
				in[s] = h.copy()
				work = append(work, s)
			}
		}
	}
	return problems
}

// takesAndReads splits what an instruction reads into what it takes (a
// reference passed on) and what it only reads, as Count left it.
func takesAndReads(in Instr) (takes, reads []*Value) {
	switch x := in.(type) {
	case *Move:
		if x.Borrow {
			return nil, []*Value{x.Src}
		}
		return []*Value{x.Src}, nil
	case *Call:
		for i, a := range x.Args {
			if i < len(x.Fn.Modes) && x.Fn.Modes[i] == Owned {
				takes = append(takes, a)
			} else {
				reads = append(reads, a)
			}
		}
		return takes, reads
	case *Builtin:
		if x.InPlace {
			return x.Args[:1], x.Args[1:]
		}
	case *Op:
		if x.InPlace {
			return x.Args[:1], x.Args[1:]
		}
	case *Update:
		if x.InPlace {
			return append([]*Value{x.Src}, x.Fields...), nil
		}
		return x.Fields, []*Value{x.Src}
	case *MakeRecord:
		return x.Fields, nil
	case *MakeVariant:
		return x.Args, nil
	case *MakeTuple:
		return x.Elems, nil
	case *MakeList:
		return x.Elems, nil
	case *MakeSet:
		return x.Elems, nil
	case *MakeMap:
		return append(append([]*Value(nil), x.Keys...), x.Values...), nil
	case *MakeClosure:
		return x.Captures, nil
	case *MakeDyn:
		return []*Value{x.Src}, nil
	case *SetPlace:
		reads = nil
		if len(x.Path) > 0 {
			reads = append(reads, x.Var)
		}
		for _, s := range x.Path {
			if s.Index != nil {
				reads = append(reads, s.Index)
			}
		}
		return []*Value{x.Src}, reads
	}
	return nil, readsOf(in)
}
