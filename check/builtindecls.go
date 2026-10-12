package check

import (
	"strconv"

	"eyelang/ir"
	"eyelang/syntax"
)

// The built-in functions are declared in ir/builtin.eye, and a call of one
// is checked against its declaration as a call of any function is: its
// parameters, defaults, `where` clause and result. Its `uses` clause gives
// its effects, and `uses {wait}` makes it a durable step in a job.
//
// goRules are the built-ins one signature can't describe yet, which
// builtinWith types in Go: the collection functions, which take any kind of
// collection until a trait can say so (SPECS.md §8); those that take any
// number of arguments; and the rest for the reason given. Their declarations
// still give their effects.
var goRules = map[string]bool{
	// Any number of values, each of which must have Show.
	"print": true, "log.debug": true, "log.info": true, "log.warn": true, "log.error": true,
	// Any number of contract modules.
	"fakes": true,
	// Its capacity must be named, with an error that says why.
	"channel": true,
	// Views and triggers aren't values yet: the attribute @on checks them.
	"enters": true, "leaves": true,
	// The collection functions.
	"size": true, "isEmpty": true, "add": true, "remove": true, "contains": true, "insert": true,
	"get": true, "first": true, "last": true, "map": true, "filter": true, "find": true, "any": true,
	"all": true, "count": true, "reduce": true, "sum": true, "average": true, "enumerate": true, "drop": true,
	"take": true, "reverse": true, "sort": true, "keys": true, "values": true, "entries": true,
	"toList": true, "toSet": true, "join": true, "flatMap": true, "zip": true, "parallelMap": true, "iter": true,
	// The methods of the fixed-width integers, which take any of the five
	// types (fixedRule), and toInt, which also takes a Float.
	"wrappingAdd": true, "wrappingSub": true, "wrappingMul": true, "checkedAdd": true,
	"checkedSub": true, "checkedMul": true, "and": true, "or": true, "xor": true, "not": true,
	"shiftLeft": true, "shiftRight": true, "toInt": true,
}

// builtinDecls are the declarations of ir/builtin.eye, by the name a call
// uses: `log.info` for a function of a built-in module.
type builtinDecls struct {
	funcs map[string]*ir.Func // the signatures calls are checked against
	uses  map[string][]string // every built-in's uses clause
	rules map[string][]fparam // the parameters of goRules, by name, without types
}

// loadBuiltins lowers the signatures in ir/builtin.eye. Its types are the
// ones the checker knows by name, such as String and List, and the prelude's;
// its own `extern type` lines only document them.
func (c *checker) loadBuiltins() {
	f := parseFile("<builtin>", ir.BuiltinSource)
	m := &module{
		mod: &ir.Module{Path: "builtin", Name: "builtin", Files: []*ir.File{f},
			Types: map[string]*ir.Type{}, Funcs: map[string]*ir.Func{}},
		values: map[string]any{},
		types:  map[string]any{},
	}
	c.builtins = builtinDecls{funcs: map[string]*ir.Func{}, uses: map[string][]string{}, rules: map[string][]fparam{}}
	for _, d := range f.Tree.Root.Children {
		if d.Kind != syntax.ExternDeclaration {
			continue
		}
		name := text(f, d.ChildByField(syntax.FieldName))
		host := ""
		if args, _, ok := attribute(f, d, "host"); ok && len(args) == 1 {
			host = c.stringArg(f, args[0])
			name = host
		}
		c.builtins.uses[name], _ = usesOf(f, d)
		if goRules[name] {
			c.builtins.rules[name] = ruleParams(f, d)
			continue
		}
		fn := &ir.Func{Name: name, Module: m.mod, Pub: true, Pos: pos(f, d), Extern: true, Host: host}
		c.lowerFunc(m, f, d, fn, ctxPlain, nil, nil)
		c.builtins.funcs[name] = fn
	}
}

// builtinEffects adds what calling a built-in does to the function being
// checked. Waiting is among them: outside a job, sleep blocks, and the
// caller's signature says that it can hold its own caller up (SPECS.md §12).
func (x *fnCheck) builtinEffects(name string, args []ir.Arg) {
	for _, u := range x.c.builtins.uses[name] {
		x.effect(namedEffect(u, args))
	}
}

// namedEffect names what a built-in's effect touches, from its first
// argument where that says: secret("telegram-bot-token") uses
// secret "telegram-bot-token", and next!(bus.pinged, …) consume Bus.pinged.
// Otherwise the effect is the bare kind, which covers all of its kind.
func namedEffect(kind string, args []ir.Arg) string {
	if len(args) == 0 {
		return kind
	}
	switch kind {
	case "secret":
		if l, ok := args[0].Value.(*ir.Lit); ok {
			if s, ok := l.Value.(string); ok {
				return kind + " " + strconv.Quote(s)
			}
		}
	case "consume":
		if ev := eventOf(args[0].Value); ev != nil {
			return kind + " " + ev.Contract.Name + "." + ev.Name
		}
	}
	return kind
}

// eventOf is the contract event an expression names, as bus.pinged does.
func eventOf(e ir.Expr) *ir.Event {
	if r, ok := e.(*ir.Ref); ok {
		if ev, ok := r.Decl.(*ir.Event); ok && ev.Contract != nil {
			return ev
		}
	}
	return nil
}

// declaredBuiltin checks a call of a built-in against its declaration.
// first, when set, is the type of the first argument, already checked: the
// receiver of `x.f(…)`.
func (x *fnCheck) declaredBuiltin(fn *ir.Func, args []ir.Arg, first typ, at ir.Pos) typ {
	f := x.c.funcType(fn, nil)
	if first != nil && len(f.params) > 0 && len(args) > 0 {
		x.expect(args[0].Value.Position(), f.params[0].t, first)
		f = &tfun{params: f.params[1:], ret: f.ret, bounds: f.bounds}
		args = args[1:]
	}
	x.args(f, args, fn.Name, at)
	return f.ret
}

// builtinStep says whether a call of a built-in is a durable step in a job,
// and the effect that makes it one, as its uses clause says.
func (c *checker) builtinStep(name string) (effect string, step bool) {
	set := map[string]bool{}
	for _, u := range c.builtins.uses[name] {
		set[u] = true
	}
	if set["wait"] {
		return "wait", true
	}
	effect = outside(set)
	return effect, effect != ""
}
