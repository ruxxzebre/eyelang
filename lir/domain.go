package lir

import (
	"sort"

	"eyelang/ir"
)

// The domain constructs, lowered each by its own rule (SPECS.md §22, step
// 5). Entities, jobs, views and contracts stay declarations in the typed IR;
// here they become functions of known kinds and calls into the runtime:
//
//   - An entity method is a function whose first parameter is the entity's
//     state; it reads and writes that state's fields, and the runtime saves
//     the state it leaves, in the transaction around the call. A call from
//     elsewhere asks the runtime: entity.call, with the instance to run.
//   - A job's body is a function whose durable steps, its `!` calls, are
//     marked; the runtime records each and checkpoints between statements.
//   - A view is the function that computes its entities.
//   - A served operation is a function the runtime calls for a request; a
//     call of another service's operation asks the runtime: op.call.
//   - Starting a job, publishing an event, reading an entity's state and
//     reconciling a command are runtime calls too.

// root is a body the program is lowered from, and what kind it is.
type root struct {
	fn   *ir.Func
	kind Kind
}

// roots lists every body of every module, in source order, with its kind.
// Generic ones are lowered only at the types their callers use.
func roots(prog *ir.Program) []root {
	var out []root
	for _, m := range prog.Modules {
		var rs []root
		add := func(f *ir.Func, k Kind) {
			if f != nil && f.Body != nil {
				rs = append(rs, root{f, k})
			}
		}
		for _, f := range m.Funcs {
			add(f, KindFunc)
		}
		for _, g := range m.Globals {
			add(g.Init, KindGlobal)
		}
		for _, t := range m.Types {
			for _, f := range t.Methods {
				if f.Receiver == t {
					add(f, KindFunc)
				}
			}
		}
		for _, e := range m.Entities {
			for _, f := range e.Methods {
				add(f, KindEntity)
			}
			for _, s := range e.States {
				add(s.Default, KindGlobal)
			}
			for _, mg := range e.Migrations {
				add(mg.Func, KindMigration)
			}
		}
		for _, j := range m.Jobs {
			add(j.Func, KindJob)
			for _, t := range j.Triggers {
				add(t.Schedule, KindGlobal)
			}
			add(j.Deadline, KindGlobal)
		}
		for _, v := range m.Views {
			add(v.Func, KindView)
		}
		for _, ct := range m.Contracts {
			for _, op := range ct.Ops {
				add(op.Impl, KindServe)
			}
		}
		for _, t := range m.Tests {
			add(t.Func, KindTest)
		}
		for _, impl := range m.Impls {
			if impl.For == nil || impl.For.Decl != nil || impl.For.Entity != nil {
				continue // a declared type's impl methods are among its methods
			}
			for _, f := range impl.Methods {
				if impl.Trait == nil || impl.Trait.Methods[f.Name] != f {
					add(f, KindFunc)
				}
			}
		}
		sort.SliceStable(rs, func(i, j int) bool {
			a, b := rs[i].fn.Pos, rs[j].fn.Pos
			if a.File != b.File && a.File != nil && b.File != nil {
				return a.File.Path < b.File.Path
			}
			return a.Offset < b.Offset
		})
		out = append(out, rs...)
	}
	return out
}

// kindOf is the kind of a body called by name, for an instance a call asks
// for.
func kindOf(f *ir.Func) Kind {
	if f.Entity != nil {
		return KindEntity
	}
	return KindFunc
}
