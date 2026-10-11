package check

import (
	"fmt"
	"sort"
	"strings"

	"eyelang/ir"
	"eyelang/syntax"
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
		// A public function states its effects, and so does a private one
		// with a uses clause: its callers count what it states.
		var fns []*ir.Func
		for _, f := range m.Funcs {
			if (f.Pub || f.HasUses) && !f.Extern && f.Body != nil {
				fns = append(fns, f)
			}
		}
		for _, t := range m.Types {
			for _, f := range t.Methods {
				if (f.Pub || f.HasUses) && f.Body != nil && f.Receiver == t {
					fns = append(fns, f)
				}
			}
		}
		sort.Slice(fns, func(i, j int) bool { return fns[i].Pos.Offset < fns[j].Pos.Offset })
		for _, f := range fns {
			c.checkUses(f, all[f])
		}
		c.checkImplEffects(m, all)
	}
	c.checkMigrationEffects(all)
	c.checkFnLimits(all)
	c.checkStepResults(all)
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
	// A program's extern fn without a uses clause runs foreign code; one
	// with a clause, as the standard library's are, has what it states.
	for _, m := range c.prog.Modules {
		if m.Dir == "" && m != c.prog.Root {
			continue
		}
		for _, f := range m.Funcs {
			if f.Extern && !f.HasUses {
				all[f] = map[string]bool{"ffi": true}
			}
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

// checkImplEffects reports an impl's method that does more than its trait's
// method states: a call through the trait, as on a dyn value, counts only
// what the trait says (SPECS.md §12).
func (c *checker) checkImplEffects(m *ir.Module, all map[*ir.Func]map[string]bool) {
	for _, impl := range m.Impls {
		if impl.Trait == nil {
			continue
		}
		var names []string
		for name := range impl.Methods {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			f, sig := impl.Methods[name], impl.Trait.Methods[name]
			if sig == nil || f == sig || f.Body == nil {
				continue // not the trait's, or its default
			}
			var extra []string
			for e := range all[f] {
				if !covered(sig.Uses, e) {
					extra = append(extra, e)
				}
			}
			if len(extra) > 0 {
				sort.Strings(extra)
				list := strings.Join(extra, ", ")
				c.typeErrorf(f.Pos, "%s uses %s, which %s.%s doesn't list, and a call through the trait counts only what the trait says: add uses {%s} to the trait's method", f.Name, list, impl.Trait.Name, name, list)
			}
		}
	}
}

// checkUses reports effects a public function has but doesn't state, and
// records what `eye fmt` writes in a public function's clause.
func (c *checker) checkUses(f *ir.Func, effects map[string]bool) {
	var extra []string
	for e := range effects {
		if !covered(f.Uses, e) {
			extra = append(extra, e)
		}
	}
	sort.Strings(extra)
	list := strings.Join(extra, ", ")
	var reported []Diagnostic
	switch {
	case len(extra) == 0:
	case !f.HasUses:
		reported = c.reportUses(f.Pos, "%s uses %s, which its signature doesn't list: run eye fmt, or add uses {%s}", f.Name, list, list)
	case f.Pub:
		reported = c.reportUses(f.Pos, "%s uses %s, which its uses clause doesn't list: run eye fmt, or add it", f.Name, list)
	default:
		c.typeErrorf(f.Pos, "%s uses %s, which its uses clause doesn't list", f.Name, list)
	}
	if !f.Pub {
		return // a private function's clause is a limit its author sets
	}
	// The clause keeps what it lists that covers an effect, as `net` covers
	// `net Notify.send`, and gains what nothing covers.
	var keep []string
	for _, u := range f.Uses {
		for e := range effects {
			if covered([]string{u}, e) {
				keep = append(keep, u)
				break
			}
		}
	}
	if len(extra) == 0 && len(keep) == len(f.Uses) && (len(keep) > 0 || !f.HasUses) {
		return // it says what the body does
	}
	want := append(keep, extra...)
	sort.Strings(want)
	c.signatures = append(c.signatures, Signature{Func: f, Effects: want, reported: reported})
}

// reportUses reports a uses clause that misses an effect, and returns the
// diagnostic, which `eye fmt` fixes.
func (c *checker) reportUses(at ir.Pos, format string, args ...any) []Diagnostic {
	c.typeErrorf(at, format, args...)
	return []Diagnostic{{Pos: at, Message: fmt.Sprintf(format, args...)}}
}

// Signature is a public function whose uses clause doesn't say what its
// body does, and the effects the clause should list, sorted: what it lists
// that still covers an effect, and every effect nothing covers. An empty
// list means the function is pure and needs no clause.
type Signature struct {
	Func     *ir.Func
	Effects  []string
	reported []Diagnostic
}

// Signatures lists the public functions of the program's own modules whose
// uses clause `eye fmt` rewrites. ok is false when the program has errors
// other than the effects those clauses miss, since its effects may then be
// wrong.
func (r *Result) Signatures() (sigs []Signature, ok bool) {
	fixed := map[Diagnostic]bool{}
	for _, s := range r.c.signatures {
		for _, d := range s.reported {
			fixed[d] = true
		}
	}
	for _, d := range r.Diagnostics {
		if !fixed[d] {
			return nil, false
		}
	}
	return r.c.signatures, true
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

// effectKinds are the effects a uses clause may list (SPECS.md §12), and
// whether each names what it touches: `net Notify.send`, `entity Order`.
var effectKinds = map[string]bool{
	"wait": false, "net": true, "entity": true, "clock": false, "random": false, "fs": false,
	"process": false, "stdio": false, "env": false, "ffi": false, "secret": true, "publish": true,
	"consume": true,
}

// effectExample shows how each effect that names something names it.
var effectExample = map[string]string{
	"net": "net Notify.send", "entity": "entity Order", "publish": "publish Bus.pinged",
	"consume": "consume Bus.pinged", "secret": `secret "telegram-bot-token"`,
}

// checkEffectNames checks a uses clause's effects: each is one SPECS.md §12
// lists, and the contract, operation or entity it names exists.
func (c *checker) checkEffectNames(m *module, f *ir.File, d *syntax.Node) {
	clause := d.ChildByField(syntax.FieldEffects)
	if clause == nil {
		return
	}
	for _, e := range clause.NamedChildren() {
		kindNode, target := e.ChildByField(syntax.FieldKind), e.ChildByField(syntax.FieldTarget)
		kind := text(f, kindNode)
		named, ok := effectKinds[kind]
		switch {
		case !ok:
			var kinds []string
			for k := range effectKinds {
				kinds = append(kinds, k)
			}
			sort.Strings(kinds)
			c.errorf(pos(f, kindNode), "%s isn't an effect: an effect is %s", kind, orList(kinds))
			continue
		case target == nil:
			continue // `net` covers every operation, `entity` every entity
		case !named:
			c.errorf(pos(f, target), "%s names nothing: write uses {%s}", kind, kind)
			continue
		case kind == "secret":
			name := text(f, target)
			switch {
			case target.Kind != syntax.String:
				c.errorf(pos(f, target), `secret names a secret as secret(…) is passed it, in quotes: secret "telegram-bot-token"`)
			case strings.HasPrefix(name, `"""`):
				c.errorf(pos(f, target), `a secret's name is on one line: write it as "%s"`, strings.TrimSpace(strings.Trim(name, `"`)))
			case strings.ContainsAny(name, `{\`):
				c.errorf(pos(f, target), `a secret's name is plain text, as secret(…) is passed it: secret "telegram-bot-token"`)
			}
			continue
		case target.Kind == syntax.String:
			c.errorf(pos(f, target), "%s names what it touches without quotes, as %s", kind, effectExample[kind])
			continue
		}
		name := strings.Join(strings.Fields(text(f, target)), "")
		switch kind {
		case "publish", "consume":
			ctName, evName, _ := strings.Cut(name, ".")
			ct := c.contractIn(m, ctName)
			switch {
			case ct == nil:
				c.errorf(pos(f, target), "%s names a contract's event, as %s, and there is no contract %s here", kind, effectExample[kind], ctName)
			case evName != "" && ct.Events[evName] == nil:
				c.errorf(pos(f, target), "contract %s has no event %s", ctName, evName)
			}
		case "net":
			ctName, opName, hasOp := strings.Cut(name, ".")
			ct := c.contractIn(m, ctName)
			switch {
			case ct == nil:
				c.errorf(pos(f, target), "net names a contract and its operation, as net Notify.send, and there is no contract %s here", ctName)
			case hasOp && ct.Ops[opName] == nil:
				c.errorf(pos(f, target), "contract %s has no operation %s", ctName, opName)
			}
		case "entity":
			if c.entityIn(m, name) == nil {
				c.errorf(pos(f, target), "entity names an entity, and there is no entity %s here", name)
			}
		}
	}
}

// contractIn finds a contract by name in a module or the modules it
// imports.
func (c *checker) contractIn(m *module, name string) *ir.Contract {
	if ct, ok := m.types[name].(*ir.Contract); ok {
		return ct
	}
	for _, imp := range m.imports {
		if ct, ok := imp.types[name].(*ir.Contract); ok {
			return ct
		}
	}
	return nil
}

// entityIn finds an entity by name in a module or the modules it imports.
func (c *checker) entityIn(m *module, name string) *ir.Entity {
	if e, ok := m.types[name].(*ir.Entity); ok {
		return e
	}
	for _, imp := range m.imports {
		if e, ok := imp.types[name].(*ir.Entity); ok {
			return e
		}
	}
	return nil
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

// taskBuiltins are the built-in functions that run the functions given to
// them as tasks. taskGroup runs its function as the group's first task, so
// the functions it spawns are inside one too.
var taskBuiltins = map[string]bool{"parallelMap": true, "both": true, "race": true, "timeout": true, "taskGroup": true}

// outside reports the first effect in a set that reaches outside the
// program, which in a job makes a call a durable step (SPECS.md §12):
// another service, the clock, randomness, the environment, files, other
// processes, foreign code, entities, secrets and events. Standard output and
// waiting aren't among them: waiting is reported on its own, and output is
// not a step (open question 22).
func outside(effects map[string]bool) string {
	var found []string
	for e := range effects {
		kind, _, _ := strings.Cut(e, " ")
		switch kind {
		case "net", "clock", "random", "env", "fs", "process", "ffi", "entity", "secret", "publish", "consume":
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
		if r, ok := e.(*ir.Ref); ok {
			c.namedStep(r, job, all, task)
			return true
		}
		call, ok := e.(*ir.Call)
		if !ok {
			return true
		}
		r, ok := call.Func.(*ir.Ref)
		if !ok {
			return true
		}
		// The callee is called here, not named as a value: only its
		// arguments are visited.
		args := func() bool {
			for _, a := range call.Args {
				walk(a.Value, false, func(x ir.Expr, waiting bool) bool { return visit(x, waiting, task) })
			}
			return false
		}
		var name, effect string
		step := false
		switch d := r.Decl.(type) {
		case ir.Builtin:
			name = d.Name
			// A built-in's uses clause in ir/builtin.eye says whether it
			// waits or reaches outside. The lowerer reports sleep without
			// `!` already, with its own message.
			effect, step = c.builtinStep(d.Name)
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
			return args() // contract operations and entities are checked as they are lowered
		}
		switch {
		case task && step:
			c.typeErrorf(call.Pos, "a task in a job can only compute, and %s uses %s: start a child run for this step instead", name, effect)
		case !waiting && step && effect != "wait":
			c.typeErrorf(call.Pos, "in a job, %s uses %s, so calling it is a durable step: write %s!(…)", name, effect, name)
		case waiting && !step:
			c.typeErrorf(call.Pos, "%s neither waits nor reaches outside the program, so it isn't a durable step: write %s(…) without !", name, name)
		}
		return args()
	}
	walk(job.Body, false, func(x ir.Expr, waiting bool) bool { return visit(x, waiting, false) })
}

// namedStep reports a function named as a value in a job when calling it
// would be a durable step: wherever it is called, such as inside map, its
// calls aren't recorded. A lambda that calls it with ! is.
func (c *checker) namedStep(r *ir.Ref, job *ir.Func, all map[*ir.Func]map[string]bool, task bool) {
	var name, effect string
	switch d := r.Decl.(type) {
	case *ir.Func:
		name = d.Name
		set := effectsOf(d, all)
		if effect = outside(set); set["wait"] {
			effect = "wait"
		}
	case *ir.Op:
		name, effect = d.Name, "net "+d.Contract.Name+"."+d.Name
	case ir.Builtin:
		name = d.Name
		effect, _ = c.builtinStep(d.Name)
	}
	if effect == "" {
		return
	}
	if task {
		c.typeErrorf(r.Pos, "a task in a job can only compute, and %s uses %s: start a child run for this step instead", name, effect)
		return
	}
	text := r.Pos.Text()
	c.typeErrorf(r.Pos, "in a job, %s uses %s, so each call of it is a durable step, and named as a value its calls aren't recorded: pass a lambda that calls it with !, as fn …: %s!(…)", text, effect, text)
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
// value of an extern type, which foreign code owns, or a function. It
// returns "" if t can be saved.
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
			case "Secret", "TaskGroup", "Sender", "Receiver":
				return x.name
			}
		}
		for _, a := range x.args {
			if s := unsaveable(a); s != "" {
				return s
			}
		}
		if x.decl != nil && x.decl.Extern {
			return x.decl.Name // foreign code owns it (SPECS.md §18)
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
// stepResult is a `!` call in a job and the type of what it returns.
type stepResult struct {
	job *ir.Func
	w   *ir.Waiting
	t   typ
}

// checkStepResults reports a step whose result can't be saved: a step's
// result is recorded, so a resumed run sees the same value. A call of a
// function that itself waits isn't one step, and its result isn't
// recorded.
func (c *checker) checkStepResults(all map[*ir.Func]map[string]bool) {
	for _, s := range c.ts.steps {
		if call, ok := s.w.Call.(*ir.Call); ok {
			if r, ok := call.Func.(*ir.Ref); ok {
				if g, ok := r.Decl.(*ir.Func); ok && effectsOf(g, all)["wait"] {
					continue
				}
			}
		}
		reported := false // as a missing Schema, say
		for _, d := range c.diags {
			reported = reported || d.Pos == s.w.Pos
		}
		if what := c.unsaveable(s.t); what != "" && !reported {
			c.typeErrorf(s.w.Pos, "this durable step's result is recorded, and a %s can't be saved: use it in a function that returns what can be, and call that with !", what)
		}
	}
}

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
