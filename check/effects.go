package check

import (
	"fmt"
	"sort"
	"strings"

	"eyelang/ir"
)

// Effects (SPECS.md §12) are inferred inside and stated at the boundary: a
// function's effects are what its body does (waiting, calling another
// service, touching entities, the clock, standard output) and what the
// functions it calls do; a callback's effects count where the lambda is
// written. A public function lists them with `uses`, and one without `uses`
// is pure. Publishing events, starting jobs and logging aren't effects yet.

// checkEffects checks public signatures against inferred effects, calls of
// waiting functions without `!`, and what jobs hold across a `!`.
func (c *checker) checkEffects() {
	all := c.allEffects()
	for _, m := range c.prog.Modules {
		if m.Dir == "" && m != c.prog.Root {
			continue // the prelude and the standard modules
		}
		var fns []*ir.Func
		for _, f := range m.Funcs {
			if f.Pub && !f.Extern && f.Body != nil {
				fns = append(fns, f)
			}
		}
		for _, t := range m.Types {
			for _, f := range t.Methods {
				if f.Pub && f.Body != nil {
					fns = append(fns, f)
				}
			}
		}
		sort.Slice(fns, func(i, j int) bool { return fns[i].Pos.Offset < fns[j].Pos.Offset })
		for _, f := range fns {
			c.checkUses(f, all[f])
		}
	}
	c.checkMigrationEffects(all)
	for f, set := range all {
		f.Waits = set["wait"]
	}
	for _, m := range c.prog.Modules {
		for _, f := range c.bodiesOf(m) {
			c.checkWaitingCalls(f, all)
		}
		for _, j := range m.Jobs {
			c.checkSteps(j.Func, all)
			c.checkSave(j)
		}
	}
}

// allEffects computes every function's effects: its own and those of the
// functions it calls, until nothing changes. A function with a uses clause
// has the effects it states.
func (c *checker) allEffects() map[*ir.Func]map[string]bool {
	all := map[*ir.Func]map[string]bool{}
	for f, set := range c.ts.effects {
		all[f] = map[string]bool{}
		for e := range set {
			all[f][e] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for f, callees := range c.ts.callees {
			if all[f] == nil {
				all[f] = map[string]bool{}
			}
			for _, g := range callees {
				var from map[string]bool
				if g.HasUses {
					from = map[string]bool{}
					for _, u := range g.Uses {
						from[u] = true
					}
				} else {
					from = all[g]
				}
				for e := range from {
					if !all[f][e] {
						all[f][e] = true
						changed = true
					}
				}
			}
		}
	}
	return all
}

// checkUses reports effects a public function has but doesn't state.
func (c *checker) checkUses(f *ir.Func, effects map[string]bool) {
	var extra []string
	for e := range effects {
		if !covered(f.Uses, e) {
			extra = append(extra, e)
		}
	}
	if len(extra) == 0 {
		return
	}
	sort.Strings(extra)
	list := strings.Join(extra, ", ")
	if !f.HasUses {
		c.typeErrorf(f.Pos, "%s uses %s, which its signature doesn't list: add uses {%s}", f.Name, list, list)
		return
	}
	c.typeErrorf(f.Pos, "%s uses %s, which its uses clause doesn't list", f.Name, list)
}

// covered reports whether a uses clause covers an effect: `net` covers every
// call to another service, `net Notify` every operation of Notify.
func covered(uses []string, effect string) bool {
	for _, u := range uses {
		if u == effect || strings.HasPrefix(effect, u+" ") || strings.HasPrefix(effect, u+".") {
			return true
		}
	}
	return false
}

// bodiesOf lists the functions of a module that have bodies.
func (c *checker) bodiesOf(m *ir.Module) []*ir.Func {
	var out []*ir.Func
	add := func(f *ir.Func) {
		if f != nil && f.Body != nil {
			out = append(out, f)
		}
	}
	for _, f := range m.Funcs {
		add(f)
	}
	for _, t := range m.Types {
		for _, f := range t.Methods {
			add(f)
		}
	}
	for _, e := range m.Entities {
		for _, f := range e.Methods {
			add(f)
		}
	}
	for _, j := range m.Jobs {
		add(j.Func)
	}
	for _, t := range m.Tests {
		add(t.Func)
	}
	for _, ct := range m.Contracts {
		for _, op := range ct.Ops {
			add(op.Impl)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Pos.Offset < out[j].Pos.Offset })
	return out
}

// checkWaitingCalls reports calls of waiting functions without `!`: a call
// that may pause is a durable step, and the `!` shows where.
func (c *checker) checkWaitingCalls(f *ir.Func, all map[*ir.Func]map[string]bool) {
	walk(f.Body, false, func(e ir.Expr, waiting bool) bool {
		call, ok := e.(*ir.Call)
		if !ok || waiting {
			return true
		}
		if r, ok := call.Func.(*ir.Ref); ok {
			if g, ok := r.Decl.(*ir.Func); ok && all[g]["wait"] {
				c.typeErrorf(call.Pos, "%s waits, so calling it is a durable step: write %s!(…)", g.Name, g.Name)
			}
		}
		return true
	})
}

// stepBuiltins are the built-in functions that are durable steps in a job:
// those that wait, and those that read the world outside the program. The
// lowerer reports sleep without `!` already, with its own message.
var stepBuiltins = map[string]string{
	"sleep":      "wait",
	"sleepUntil": "wait",
	"poll":       "wait",
	"next":       "wait",
	"approval":   "wait",
	"now":        "clock",
	"env.get":    "env",
}

// taskBuiltins are the built-in functions that run the functions given to
// them as tasks. taskGroup runs its function as the group's first task, so
// the functions it spawns are inside one too.
var taskBuiltins = map[string]bool{"parallelMap": true, "both": true, "race": true, "timeout": true, "taskGroup": true}

// outside reports the first effect in a set that reaches outside the
// program, which in a job makes a call a durable step (SPECS.md §12):
// another service, the clock, randomness, the environment, files, other
// processes, foreign code and entities. Standard output and waiting aren't
// among them: waiting is reported on its own, and printing is open question 22.
func outside(effects map[string]bool) string {
	var found []string
	for e := range effects {
		kind, _, _ := strings.Cut(e, " ")
		switch kind {
		case "net", "clock", "random", "env", "fs", "process", "ffi", "entity", "secret":
			found = append(found, e)
		}
	}
	if len(found) == 0 {
		return ""
	}
	sort.Strings(found)
	return found[0]
}

// effectsOf is what calling g does: what its uses clause states, if it has
// one, or else what its body was found to do.
func effectsOf(g *ir.Func, all map[*ir.Func]map[string]bool) map[string]bool {
	if !g.HasUses {
		return all[g]
	}
	set := map[string]bool{}
	for _, u := range g.Uses {
		set[u] = true
	}
	return set
}

// checkSteps holds a job to SPECS.md §12: every call with an outside effect
// is a `!` step, a call that neither waits nor reaches outside isn't one, and
// a task the job starts (parallelMap, both, race, timeout) only computes,
// since its steps couldn't be recorded in order.
func (c *checker) checkSteps(job *ir.Func, all map[*ir.Func]map[string]bool) {
	var visit func(e ir.Expr, waiting, task bool) bool
	inTask := func(e ir.Expr) {
		walk(e, false, func(x ir.Expr, waiting bool) bool { return visit(x, waiting, true) })
	}
	visit = func(e ir.Expr, waiting, task bool) bool {
		if w, ok := e.(*ir.Waiting); ok && task {
			c.typeErrorf(w.Pos, "a task in a job can only compute, since its steps couldn't be recorded: start a child run for this step instead")
			return false
		}
		call, ok := e.(*ir.Call)
		if !ok {
			return true
		}
		r, ok := call.Func.(*ir.Ref)
		if !ok {
			return true
		}
		var name, effect string
		step := false
		switch d := r.Decl.(type) {
		case ir.Builtin:
			name = d.Name
			effect, step = stepBuiltins[d.Name]
			if taskBuiltins[d.Name] {
				for _, a := range call.Args {
					if _, ok := a.Value.(*ir.Lambda); ok {
						inTask(a.Value)
					} else {
						walk(a.Value, false, func(x ir.Expr, waiting bool) bool { return visit(x, waiting, task) })
					}
				}
				return false
			}
		case *ir.Func:
			name = d.Name
			if d.Module != nil && d.Module != job.Module && d.Receiver == nil {
				// As it is written: http.get, not get.
				name = d.Module.Name + "." + d.Name
			}
			set := effectsOf(d, all)
			if set["wait"] {
				effect, step = "wait", true
			} else if effect = outside(set); effect != "" {
				step = true
			}
		default:
			return true // contract operations and entities are checked as they are lowered
		}
		switch {
		case task && step:
			c.typeErrorf(call.Pos, "a task in a job can only compute, and %s uses %s: start a child run for this step instead", name, effect)
		case !waiting && step && effect != "wait":
			c.typeErrorf(call.Pos, "in a job, %s uses %s, so calling it is a durable step: write %s!(…)", name, effect, name)
		case waiting && !step:
			c.typeErrorf(call.Pos, "%s neither waits nor reaches outside the program, so it isn't a durable step: write %s(…) without !", name, name)
		}
		return true
	}
	walk(job.Body, false, func(x ir.Expr, waiting bool) bool { return visit(x, waiting, false) })
}

// walk calls visit for e and, if visit returns true, everything in it.
// waiting is set for the call a `!` marks.
func walk(e ir.Expr, waiting bool, visit func(e ir.Expr, waiting bool) bool) {
	if e == nil || !visit(e, waiting) {
		return
	}
	w := func(x ir.Expr) { walk(x, false, visit) }
	args := func(as []ir.Arg) {
		for _, a := range as {
			w(a.Value)
		}
	}
	switch e := e.(type) {
	case *ir.Interpolate:
		for _, p := range e.Parts {
			w(p)
		}
	case *ir.InZone:
		w(e.Zone)
	case *ir.Call:
		w(e.Func)
		args(e.Args)
	case *ir.MethodCall:
		w(e.Recv)
		args(e.Args)
	case *ir.Waiting:
		walk(e.Call, true, visit)
	case *ir.Field:
		w(e.Value)
	case *ir.Index:
		w(e.Value)
		w(e.Index)
	case *ir.Unary:
		w(e.X)
	case *ir.Binary:
		w(e.L)
		w(e.R)
	case *ir.Range:
		w(e.Start)
		w(e.End)
	case *ir.If:
		w(e.Cond)
		w(e.Then)
		w(e.Else)
	case *ir.Match:
		w(e.Value)
		for _, a := range e.Arms {
			w(a.Guard)
			w(a.Body)
		}
	case *ir.Block:
		for _, x := range e.Exprs {
			w(x)
		}
	case *ir.Let:
		w(e.Value)
		w(e.Else)
	case *ir.Assign:
		w(e.Value)
	case *ir.For:
		w(e.Iter)
		w(e.Body)
	case *ir.While:
		w(e.Cond)
		w(e.Body)
	case *ir.Return:
		w(e.Value)
	case *ir.Lambda:
		w(e.Func.Body)
	case *ir.Record:
		for _, v := range e.Values {
			w(v)
		}
	case *ir.RecordUpdate:
		w(e.Base)
		for _, v := range e.Values {
			w(v)
		}
	case *ir.List:
		for _, v := range e.Elems {
			w(v)
		}
	case *ir.Map:
		for i := range e.Keys {
			w(e.Keys[i])
			w(e.Values[i])
		}
	case *ir.Tuple:
		for _, v := range e.Elems {
			w(v)
		}
	case *ir.Try:
		w(e.X)
	case *ir.Default:
		w(e.X)
		w(e.Or)
	case *ir.With:
		w(e.Value)
	}
}

// ---------------------------------------------------------------- Save across `!`

// unsaveable names what in t can't be saved at a checkpoint: a Secret, a
// Handle or a function. It returns "" if t can be saved.
func (c *checker) unsaveable(t typ) string {
	return c.unsaveableIn(t, map[*ir.Type]bool{})
}

func (c *checker) unsaveableIn(t typ, seen map[*ir.Type]bool) string {
	unsaveable := func(t typ) string { return c.unsaveableIn(t, seen) }
	fromDecl := func(_ *ir.Type, te *ir.TypeExpr, x *tcon) typ { return c.fromExpr(te, envOf(x), false) }
	switch x := prune(t).(type) {
	case *tcon:
		if x.decl == nil && x.entity == nil {
			switch x.name {
			case "Secret", "Handle", "TaskGroup", "Sender", "Receiver":
				return x.name
			}
		}
		for _, a := range x.args {
			if s := unsaveable(a); s != "" {
				return s
			}
		}
		// A declared type is saved field by field.
		if d := x.decl; d != nil && !seen[d] {
			seen[d] = true
			defer delete(seen, d)
			fields := func(names []string, types []*ir.TypeExpr) string {
				for i, n := range names {
					if i < len(types) {
						if s := unsaveable(fromDecl(d, types[i], x)); s != "" {
							return fmt.Sprintf("%s (its field %s is a %s)", d.Name, n, s)
						}
					}
				}
				return ""
			}
			if d.Alias != nil {
				if s := fields(d.Alias.Fields, d.Alias.Args); s != "" {
					return s
				}
			}
			for _, v := range d.Variants {
				for _, field := range v.Fields {
					if s := unsaveable(fromDecl(d, field.Type, x)); s != "" {
						return fmt.Sprintf("%s (%s holds a %s)", d.Name, v.Name, s)
					}
				}
			}
		}
		if e := x.entity; e != nil {
			for _, s := range e.States {
				if what := unsaveable(fromDecl(nil, s.Type, x)); what != "" {
					return fmt.Sprintf("%s (its state %s is a %s)", e.Name, s.Name, what)
				}
			}
		}
	case *ttuple:
		for _, e := range x.elems {
			if s := unsaveable(e); s != "" {
				return s
			}
		}
	case *tfun:
		return "function"
	case *tdyn:
		return "dyn " + x.trait.Name // its type, which saving needs, is known only at run time
	}
	return ""
}

// checkSave reports a value that can't be saved but is still needed after
// a `!` in a job, where the job's state is saved (SPECS.md §15). Uses are
// found in source order; a value needed again on a later turn of a loop
// isn't seen yet.
func (c *checker) checkSave(j *ir.Job) {
	slots := c.ts.jobSlots[j.Func]
	if slots == nil {
		return
	}
	for _, p := range j.Func.Params {
		if p.Slot < len(slots) {
			if what := c.unsaveable(slots[p.Slot]); what != "" {
				c.typeErrorf(j.Pos, "%s's argument %s is a %s, which can't be saved, and a job's arguments are", j.Name, p.Name, what)
			}
		}
	}
	defined := map[int]int{} // slot → where it is bound
	for _, p := range j.Func.Params {
		defined[p.Slot] = -1
	}
	var waits []int
	reported := map[int]bool{}
	use := func(slot, at int, pos ir.Pos, name string) {
		if slot >= len(slots) || reported[slot] {
			return
		}
		what := c.unsaveable(slots[slot])
		if what == "" {
			return
		}
		from, ok := defined[slot]
		if !ok {
			return
		}
		for _, w := range waits {
			if w > from && w < at {
				reported[slot] = true
				c.typeErrorf(pos, "%s is a %s, which can't be saved, and it is still needed after a waiting call: get it again after the `!`", name, what)
				return
			}
		}
	}
	visit := func(e ir.Expr, waiting bool) {
		switch e := e.(type) {
		case *ir.Waiting:
			waits = append(waits, e.Pos.End)
		case *ir.Let:
			for _, s := range bound(e.Pattern) {
				defined[s] = e.Pos.Offset
			}
		case *ir.For:
			for _, s := range bound(e.Pattern) {
				defined[s] = e.Pos.Offset
			}
		case *ir.Match:
			for _, a := range e.Arms {
				for _, s := range bound(a.Pattern) {
					defined[s] = e.Pos.Offset
				}
			}
		case *ir.Local:
			use(e.Slot, e.Pos.Offset, e.Pos, e.Name)
		}
	}
	walkJob(j.Func.Body, visit)
}

// walkJob walks a job's body for the Save check: a lambda's captures are
// uses where the lambda is written, and its body isn't the job's.
func walkJob(e ir.Expr, visit func(ir.Expr, bool)) {
	walk(e, false, func(x ir.Expr, waiting bool) bool {
		if l, ok := x.(*ir.Lambda); ok {
			for _, capt := range l.Func.Captures {
				if loc, ok := capt.From.(*ir.Local); ok {
					visit(&ir.Local{At: ir.At{Pos: l.Pos}, Name: loc.Name, Slot: loc.Slot}, false)
				}
			}
			return false
		}
		visit(x, waiting)
		return true
	})
}

// bound lists the slots a pattern binds.
func bound(p ir.Pattern) []int {
	var out []int
	switch p := p.(type) {
	case ir.PBind:
		out = append(out, p.Slot)
	case ir.PVariant:
		for _, a := range p.Args {
			out = append(out, bound(a)...)
		}
	case ir.PRecord:
		for _, a := range p.Pats {
			out = append(out, bound(a)...)
		}
	case ir.PTuple:
		for _, a := range p.Pats {
			out = append(out, bound(a)...)
		}
	case ir.PList:
		for _, a := range p.Pats {
			out = append(out, bound(a)...)
		}
		if p.RestPat != nil {
			out = append(out, bound(p.RestPat)...)
		}
	case ir.POr:
		for _, a := range p.Alts {
			out = append(out, bound(a)...)
		}
	}
	return out
}
