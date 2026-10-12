package check

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"eyelang/ir"
	"eyelang/syntax"
)

// Effects (SPECS.md §12) are inferred inside and stated at the boundary: a
// function's effects are what its body does (waiting, calling another
// service, touching entities, the clock, standard output, publishing events,
// starting jobs) and what the functions it calls do; a callback's effects
// count where the lambda is written. A public function, entity method or
// job lists them with `uses`, and one without `uses` is pure. Logging isn't
// an effect.

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
		// Public entity methods and jobs state their effects too.
		for _, e := range m.Entities {
			for _, f := range e.Methods {
				if (f.Pub || f.HasUses) && f.Body != nil {
					fns = append(fns, f)
				}
			}
		}
		for _, j := range m.Jobs {
			if j.Func.Pub || j.Func.HasUses {
				fns = append(fns, j.Func)
			}
		}
		sort.Slice(fns, func(i, j int) bool { return fns[i].Pos.Offset < fns[j].Pos.Offset })
		for _, f := range fns {
			c.checkUses(f, all[f])
		}
		c.checkImplEffects(m, all)
	}
	c.checkMigrationEffects(all)
	c.checkTransactions(all)
	c.checkUnkeyedStarts()
	c.checkFnLimits(all)
	c.checkTools(all)
	c.checkStepResults(all)
	for f, set := range all {
		f.Waits = set["wait"]
	}
	for _, m := range c.prog.Modules {
		for _, f := range Bodies(m) {
			f.Effects = sortedEffects(effectsOf(f, all))
		}
		for _, f := range m.Funcs {
			f.Effects = sortedEffects(effectsOf(f, all)) // externs too
		}
	}
	for f := range c.ts.lambdaEffects {
		f.Effects = sortedEffects(c.srcEffects(fxSrc{fn: f, lambda: true}, all))
	}
	for f := range c.ts.lambdaCallees {
		f.Effects = sortedEffects(c.srcEffects(fxSrc{fn: f, lambda: true}, all))
	}
	for _, m := range c.prog.Modules {
		kinds := map[*ir.Func]fnKind{}
		for _, j := range m.Jobs {
			kinds[j.Func] = kindJob
		}
		for _, t := range m.Tests {
			kinds[t.Func] = kindTest
		}
		for _, e := range m.Entities {
			for _, f := range e.Methods {
				kinds[f] = kindEntity
			}
		}
		for _, ct := range m.Contracts {
			for _, op := range ct.Ops {
				if op.Impl != nil {
					kinds[op.Impl] = kindOperation
				}
			}
		}
		for _, f := range c.bodiesOf(m) {
			c.checkWaitingCalls(f, kinds[f], all)
			if kinds[f] == kindPlain && hasSteps(f) && m.Main != f {
				// A waiting function's steps are the job's (SPECS.md §12).
				c.checkSteps(f, "in a waiting function", all)
				// Its frame is part of the run's saved state (SPECS.md §15).
				c.checkSave(f)
			}
		}
		for _, j := range m.Jobs {
			c.checkSteps(j.Func, "in a job", all)
			c.checkJobArgs(j)
			c.checkSave(j.Func)
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
	"consume": true, "start": true,
}

// effectExample shows how each effect that names something names it.
var effectExample = map[string]string{
	"net": "net Notify.send", "entity": "entity Order", "publish": "publish Bus.pinged",
	"consume": "consume Bus.pinged", "secret": `secret "telegram-bot-token"`, "start": "start digest",
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
		case "start":
			if c.jobIn(m, name) == nil {
				c.errorf(pos(f, target), "start names a job by its name, as start digest, and the program has no job %s", name)
			}
		}
	}
}

// jobIn finds a job by name: in a module, the modules it imports, or any
// module of the program, since a function another module offers may start
// a job of a module this one doesn't import.
func (c *checker) jobIn(m *module, name string) *ir.Job {
	if j, ok := m.values[name].(*ir.Job); ok {
		return j
	}
	for _, imp := range m.imports {
		if j, ok := imp.values[name].(*ir.Job); ok {
			return j
		}
	}
	for _, other := range c.modules {
		if j, ok := other.values[name].(*ir.Job); ok {
			return j
		}
	}
	return nil
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

// hasSteps reports whether a function's body has `!` steps, which makes it
// a waiting function (SPECS.md §12).
func hasSteps(f *ir.Func) bool {
	found := false
	walk(f.Body, false, func(e ir.Expr, _ bool) bool {
		_, w := e.(*ir.Waiting)
		found = found || w
		return !found
	})
	return found
}

// jobOnlyWaits are the built-ins that wait for what only a run can wait
// for: outside a job they would fault, where sleep and poll block.
var jobOnlyWaits = map[string]bool{"next": true, "approval": true, "reserve": true}

// checkWaitingCalls holds calls that wait to SPECS.md §12. In a job or a
// waiting function every wait is a `!` step, so a call that waits without
// one, or a function that waits by blocking, is an error. Elsewhere a call
// of sleep or of a function that blocks keeps `wait` in the caller's
// effects, and a waiting function, whose steps only a run records, can't
// be called.
func (c *checker) checkWaitingCalls(f *ir.Func, kind fnKind, all map[*ir.Func]map[string]bool) {
	if kind == kindEntity || kind == kindOperation {
		return // a transaction can't wait at all (checkTransactions)
	}
	durable := kind == kindJob || hasSteps(f)
	steps := map[*ir.Func]bool{}
	// waits says whether calling g waits, and whether a run records its
	// waits: an extern fn's host does, and a waiting function's steps do.
	waits := func(g *ir.Func) (wait, recorded bool) {
		if g.Extern {
			return effectsOf(g, all)["wait"], true
		}
		if _, ok := steps[g]; !ok {
			steps[g] = hasSteps(g)
		}
		return effectsOf(g, all)["wait"], steps[g]
	}
	main := f.Module != nil && f.Module.Main == f
	walk(f.Body, false, func(e ir.Expr, waiting bool) bool {
		call, ok := e.(*ir.Call)
		if !ok {
			return true
		}
		r, ok := call.Func.(*ir.Ref)
		if !ok {
			return true
		}
		switch d := r.Decl.(type) {
		case *ir.Func:
			wait, recorded := waits(d)
			switch {
			case !wait:
			case !recorded && (durable || waiting):
				c.typeErrorf(call.Pos, "%s waits through calls without !, which block outside a job, so a run couldn't record its wait: write the waiting calls in %s with !, as sleep!(…)", d.Name, d.Name)
			case waiting || !recorded:
			case durable || kind == kindPlain && !main:
				c.typeErrorf(call.Pos, "%s waits, so calling it is a durable step: write %s!(…)", d.Name, d.Name)
			case kind == kindTest:
				c.typeErrorf(call.Pos, "%s has durable steps, which only a job records, so a test can't call it: start a job that calls it, then advance(…) the clock", d.Name)
			default:
				c.typeErrorf(call.Pos, "%s has durable steps, which only a job records, so %s can't call it: start a job that does", d.Name, f.Name)
			}
		case ir.Builtin:
			if waiting || !slices.Contains(c.builtins.uses[d.Name], "wait") {
				return true
			}
			switch {
			case kind == kindJob:
				// The lowerer reports it, even in a program with other errors.
			case durable:
				c.typeErrorf(call.Pos, "%s has durable steps, so its waits are steps too: write %s!(…)", f.Name, d.Name)
			case jobOnlyWaits[d.Name]:
				c.typeErrorf(call.Pos, "%s waits for what only a run can wait for, so only a job or a function with ! steps can call it, as %s!(…)", d.Name, d.Name)
			}
		}
		return true
	})
}

// notInTransaction is true for the effects an entity method or a served
// operation can't have (SPECS.md §14): each reaches outside the program or
// waits, and a transaction may run again after a conflict, so it would
// happen twice. Entity calls, publishing and starting jobs commit with the
// transaction, and output is for people watching, so those stay allowed.
var notInTransaction = map[string]bool{
	"net": true, "fs": true, "process": true, "clock": true, "random": true, "env": true,
	"ffi": true, "secret": true, "wait": true,
}

// checkTransactions reports what an entity method or a served operation
// does that reaches outside the program or waits, at the call that does it.
func (c *checker) checkTransactions(all map[*ir.Func]map[string]bool) {
	for _, m := range c.prog.Modules {
		if m.Dir == "" && m != c.prog.Root {
			continue
		}
		var fns []*ir.Func
		for _, e := range m.Entities {
			for _, f := range e.Methods {
				if f.Body != nil {
					fns = append(fns, f)
				}
			}
		}
		for _, ct := range m.Contracts {
			for _, op := range ct.Ops {
				if op.Impl != nil && op.Impl.Body != nil {
					fns = append(fns, op.Impl)
				}
			}
		}
		sort.Slice(fns, func(i, j int) bool { return fns[i].Pos.Offset < fns[j].Pos.Offset })
		for _, f := range fns {
			c.checkTransaction(f, all)
		}
	}
}

// checkTransaction reports the effects of one entity method or served
// operation that a transaction can't have.
func (c *checker) checkTransaction(f *ir.Func, all map[*ir.Func]map[string]bool) {
	what, starts := "an entity method can't", "which the method can start"
	if f.Entity == nil {
		what, starts = "serving an operation can't", "which the operation can start"
	}
	reported := map[string]bool{}
	report := func(at ir.Pos, name string, set map[string]bool) {
		var bad []string
		for e := range set {
			kind, _, _ := strings.Cut(e, " ")
			if notInTransaction[kind] {
				bad = append(bad, e)
				reported[e] = true
			}
		}
		if len(bad) == 0 {
			return
		}
		sort.Strings(bad)
		c.typeErrorf(at, "%s uses %s, which %s: it runs as a transaction, which may run again after a conflict, so what reaches outside the program or waits belongs in a job, %s", name, strings.Join(bad, ", "), what, starts)
	}
	// effects is what calling or naming decl does, and false for a
	// declaration whose effects are checked elsewhere: an entity's methods
	// are checked on their own.
	effects := func(decl any, args []ir.Arg) (string, map[string]bool, bool) {
		switch d := decl.(type) {
		case *ir.Func:
			name := d.Name
			if d.Module != nil && d.Module != f.Module && d.Receiver == nil {
				name = d.Module.Name + "." + d.Name // as it is written: fs.writeText
			}
			return name, effectsOf(d, all), d.Entity == nil
		case *ir.Op:
			return d.Name, map[string]bool{"net " + d.Contract.Name + "." + d.Name: true}, true
		case ir.Builtin:
			set := map[string]bool{}
			for _, u := range c.builtins.uses[d.Name] {
				set[namedEffect(u, args)] = true
			}
			return d.Name, set, true
		}
		return "", nil, false
	}
	var visit func(e ir.Expr, waiting bool) bool
	visit = func(e ir.Expr, waiting bool) bool {
		var decl any
		var args []ir.Arg
		switch e := e.(type) {
		case *ir.Waiting:
			reported["wait"] = true // the lowerer reports the `!`
			return true
		case *ir.Ref:
			decl = e.Decl // named as a value, it may be called wherever it goes
		case *ir.Call:
			r, ok := e.Func.(*ir.Ref)
			if !ok {
				return true
			}
			decl, args = r.Decl, e.Args
		case *ir.MethodCall:
			decl = e.Target
		}
		name, set, ok := effects(decl, args)
		if !ok {
			return true
		}
		if waiting {
			delete(set, "wait") // reported with the `!`
		}
		report(e.Position(), name, set)
		if call, ok := e.(*ir.Call); ok {
			// The callee is called here, not named as a value: only its
			// arguments are visited.
			for _, a := range call.Args {
				walk(a.Value, false, visit)
			}
			return false
		}
		return true
	}
	walk(f.Body, false, visit)
	// What no call above accounts for, as a function value's call, is
	// reported at the method. Another entity method's effects are reported
	// in that method.
	rest := map[string]bool{}
	add := func(set map[string]bool) {
		for e := range set {
			if !reported[e] {
				rest[e] = true
			}
		}
	}
	add(c.ts.effects[f])
	for _, g := range c.ts.callees[f] {
		if g.Entity == nil {
			add(effectsOf(g, all))
		}
	}
	report(f.Pos, f.Name, rest)
}

// checkUnkeyedStarts reports a call in main of a function that starts a
// job without a key, itself or through the functions it calls: main runs
// on every start of eye run, so the job would start again after each
// restart (SPECS.md §13). main's own starts are checked as they are found.
func (c *checker) checkUnkeyedStarts() {
	// What each function starts without a key, with what it calls.
	unkeyed := map[*ir.Func][]unkeyedStart{}
	for f, starts := range c.ts.unkeyed {
		unkeyed[f] = append([]unkeyedStart{}, starts...)
	}
	for changed := true; changed; {
		changed = false
		for f, callees := range c.ts.callees {
			for _, g := range callees {
				for _, s := range unkeyed[g] {
					if !slices.Contains(unkeyed[f], s) {
						unkeyed[f] = append(unkeyed[f], s)
						changed = true
					}
				}
			}
		}
	}
	for _, m := range c.prog.Modules {
		main := m.Main
		if main == nil || main.Body == nil || (m.Dir == "" && m != c.prog.Root) {
			continue
		}
		reported := map[*ir.Func]bool{}
		walk(main.Body, false, func(e ir.Expr, _ bool) bool {
			var g *ir.Func
			switch e := e.(type) {
			case *ir.Call:
				if r, ok := e.Func.(*ir.Ref); ok {
					g, _ = r.Decl.(*ir.Func)
				}
			case *ir.Ref:
				g, _ = e.Decl.(*ir.Func)
			case *ir.MethodCall:
				g, _ = e.Target.(*ir.Func)
			}
			starts := unkeyed[g]
			if g == nil || len(starts) == 0 || reported[g] {
				return true
			}
			reported[g] = true
			s := starts[0]
			for _, t := range starts[1:] {
				if t.job.Pos.Offset < s.job.Pos.Offset {
					s = t
				}
			}
			where := ""
			if s.in != g {
				where = " (in " + s.in.Name + ")"
			}
			c.typeErrorf(e.Position(), "eye run runs main every time it starts, and %s starts %s without a key%s, so each restart would start it again; give that start a key, which starts it once: %s.start(…, key: \"…\")", g.Name, s.job.Name, where, s.job.Name)
			return true
		})
	}
}

// taskBuiltins are the built-in functions that run the functions given to
// them as tasks. taskGroup runs its function as the group's first task, so
// the functions it spawns are inside one too.
var taskBuiltins = map[string]bool{"parallelMap": true, "both": true, "race": true, "timeout": true, "taskGroup": true}

// outside reports the first effect in a set that reaches outside the
// program, which in a job makes a call a durable step (SPECS.md §12):
// another service, the clock, randomness, the environment, files, other
// processes, foreign code, entities, secrets, events and starting jobs,
// which a resumed run must not do again. Standard output and
// waiting aren't among them: waiting is reported on its own, and output is
// not a step (open question 22).
func outside(effects map[string]bool) string {
	var found []string
	for e := range effects {
		kind, _, _ := strings.Cut(e, " ")
		switch kind {
		case "net", "clock", "random", "env", "fs", "process", "ffi", "entity", "secret", "publish", "consume", "start":
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

// checkSteps holds a job or a waiting function, as where says, to SPECS.md
// §12: every call with an outside effect is a `!` step, a call that neither
// waits nor reaches outside isn't one, and a task it starts (parallelMap,
// both, race, timeout) only computes, since its steps couldn't be recorded
// in order.
func (c *checker) checkSteps(fn *ir.Func, where string, all map[*ir.Func]map[string]bool) {
	var visit func(e ir.Expr, waiting, task bool) bool
	inTask := func(e ir.Expr) {
		walk(e, false, func(x ir.Expr, waiting bool) bool { return visit(x, waiting, true) })
	}
	visit = func(e ir.Expr, waiting, task bool) bool {
		if w, ok := e.(*ir.Waiting); ok && task {
			c.typeErrorf(w.Pos, "a task %s can only compute, since its steps couldn't be recorded: start a child run for this step instead", where)
			return false
		}
		if r, ok := e.(*ir.Ref); ok {
			if !c.isToolRef(r) { // a tool's calls are the model call's steps
				c.namedStep(r, where, all, task)
			}
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
		step, misnamedWait := false, false
		switch d := r.Decl.(type) {
		case ir.Builtin:
			name = d.Name
			// all! and first! are reported as they are type-checked, with
			// the wait to write instead.
			_, misnamedWait = collectionWaits[d.Name]
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
			if d.Module != nil && d.Module != fn.Module && d.Receiver == nil {
				// As it is written: http.get, not get.
				name = d.Module.Name + "." + d.Name
			}
			set := effectsOf(d, all)
			if set["wait"] {
				effect, step = "wait", true
			} else if effect = outside(set); effect != "" {
				step = true
			}
		case *ir.Op:
			// The lowerer reports one without `!` in a job.
			name, effect, step = d.Name, "net "+d.Contract.Name+"."+d.Name, true
			if !waiting && !task && where != "in a job" {
				c.typeErrorf(call.Pos, "%s, a call to another service is a durable step: write %s!(…)", where, name)
			}
			if !task {
				return args()
			}
		default:
			return args() // entities are checked as they are type-checked
		}
		switch {
		case task && step:
			c.typeErrorf(call.Pos, "a task %s can only compute, and %s uses %s: start a child run for this step instead", where, name, effect)
		case !waiting && step && effect != "wait":
			c.typeErrorf(call.Pos, "%s, %s uses %s, so calling it is a durable step: write %s!(…)", where, name, effect, name)
		case waiting && !step && !misnamedWait:
			c.typeErrorf(call.Pos, "%s neither waits nor reaches outside the program, so it isn't a durable step: write %s(…) without !", name, name)
		}
		return args()
	}
	walk(fn.Body, false, func(x ir.Expr, waiting bool) bool { return visit(x, waiting, false) })
}

// namedStep reports a function named as a value in a job or a waiting
// function when calling it would be a durable step: wherever it is called,
// such as inside map, its calls aren't recorded. A lambda that calls it
// with ! is.
func (c *checker) namedStep(r *ir.Ref, where string, all map[*ir.Func]map[string]bool, task bool) {
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
		c.typeErrorf(r.Pos, "a task %s can only compute, and %s uses %s: start a child run for this step instead", where, name, effect)
		return
	}
	text := r.Pos.Text()
	c.typeErrorf(r.Pos, "%s, %s uses %s, so each call of it is a durable step, and named as a value its calls aren't recorded: pass a lambda that calls it with !, as fn …: %s!(…)", where, text, effect, text)
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
		if fc, ok := e.Target.(ir.FuncCall); ok {
			// x.f(a) is the call f(x, a), which is what the checks of
			// calls look at; it holds x and a.
			walk(fc.Call, waiting, visit)
			return
		}
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
			case "Secret", "TaskGroup", "Sender", "Receiver", "Iter":
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
							return fmt.Sprintf("%s (its field %s is %s)", d.Name, n, withArticle(s))
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
						return fmt.Sprintf("%s (%s holds %s)", d.Name, v.Name, withArticle(s))
					}
				}
			}
		}
		if e := x.entity; e != nil {
			for _, s := range e.States {
				if what := unsaveable(fromDecl(nil, s.Type, x)); what != "" {
					return fmt.Sprintf("%s (its state %s is %s)", e.Name, s.Name, withArticle(what))
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

// withArticle puts "a" or "an" before what unsaveable names.
func withArticle(what string) string {
	if what != "" && strings.ContainsRune("AEIOU", rune(what[0])) {
		return "an " + what
	}
	return "a " + what
}

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
			c.typeErrorf(s.w.Pos, "this durable step's result is recorded, and %s can't be saved: use it in a function that returns what can be, and call that with !", withArticle(what))
		}
	}
}

// checkJobArgs reports a job's argument that can't be saved: a run is
// started with its arguments, and keeps them.
func (c *checker) checkJobArgs(j *ir.Job) {
	slots := c.ts.saveSlots[j.Func]
	for _, p := range j.Func.Params {
		if p.Slot < len(slots) {
			if what := c.unsaveable(slots[p.Slot]); what != "" {
				c.typeErrorf(j.Pos, "%s's argument %s is %s, which can't be saved, and a job's arguments are", j.Name, p.Name, withArticle(what))
			}
		}
	}
}

// checkSave reports a value that can't be saved but is still needed after
// a `!` in a job or a waiting function, whose frames are the run's saved
// state (SPECS.md §15). A parameter is one of its frame's variables. Uses
// are found in source order; a value needed again on a later turn of a
// loop isn't seen yet.
func (c *checker) checkSave(fn *ir.Func) {
	slots := c.ts.saveSlots[fn]
	if slots == nil {
		return
	}
	defined := map[int]int{} // slot → where it is bound
	for _, p := range fn.Params {
		defined[p.Slot] = -1
	}
	// A value is needed after a waiting call when one comes between where
	// it is bound and where it is read, or when it is bound before a loop,
	// read inside it, and the loop waits: the next turn reads it after the
	// wait.
	type use struct {
		slot, from, at int
		pos            ir.Pos
		name           string
	}
	type loop struct{ start, end int }
	var waits []int
	var uses []use
	var loops []loop
	var fors []*ir.For
	visit := func(e ir.Expr, waiting bool) {
		switch e := e.(type) {
		case *ir.While:
			loops = append(loops, loop{e.Pos.Offset, e.Pos.End})
		case *ir.Waiting:
			waits = append(waits, e.Pos.End)
		case *ir.Let:
			for _, s := range bound(e.Pattern) {
				defined[s] = e.Pos.Offset
			}
		case *ir.For:
			loops = append(loops, loop{e.Pos.Offset, e.Pos.End})
			fors = append(fors, e)
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
			if from, ok := defined[e.Slot]; ok {
				uses = append(uses, use{e.Slot, from, e.Pos.Offset, e.Pos, e.Name})
			}
		}
	}
	walkFrame(fn.Body, visit)
	waitsIn := func(start, end int) bool {
		for _, w := range waits {
			if w > start && w <= end {
				return true
			}
		}
		return false
	}
	reported := map[int]bool{}
	// A loop that waits keeps what it walks across the wait, to find its
	// place when the run resumes.
	for _, f := range fors {
		what := c.unsaveable(c.ts.forIters[f])
		body := f.Body.Position()
		if what == "" || !waitsIn(body.Offset, body.End) {
			continue
		}
		fix := "walk what can be saved"
		if t, ok := prune(c.ts.forIters[f]).(*tcon); ok && t.name == "Iter" && t.decl == nil {
			fix = "collect it with toList before the loop"
		}
		c.typeErrorf(f.Iter.Position(), "this loop walks %s, which can't be saved, and its body waits, so a resumed run couldn't find its place: %s", withArticle(what), fix)
		if l, ok := f.Iter.(*ir.Local); ok {
			reported[l.Slot] = true
		}
	}
	for _, u := range uses {
		if u.slot >= len(slots) || reported[u.slot] {
			continue
		}
		what := c.unsaveable(slots[u.slot])
		if what == "" {
			continue
		}
		if waitsIn(u.from, u.at-1) {
			reported[u.slot] = true
			c.typeErrorf(u.pos, "%s is %s, which can't be saved, and it is still needed after a waiting call: get it again after the `!`", u.name, withArticle(what))
			continue
		}
		for _, l := range loops {
			if u.from < l.start && l.start <= u.at && u.at < l.end && waitsIn(l.start, l.end) {
				reported[u.slot] = true
				c.typeErrorf(u.pos, "%s is %s, which can't be saved, and the loop it is read in waits, so its next turn needs it after a waiting call: make it inside the loop", u.name, withArticle(what))
				break
			}
		}
	}
}

// walkFrame walks a job's or a waiting function's body for the Save check:
// a lambda's captures are uses where the lambda is written, and its body
// isn't the frame's.
func walkFrame(e ir.Expr, visit func(ir.Expr, bool)) {
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
