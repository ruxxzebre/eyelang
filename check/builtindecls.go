package check

import (
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
	"all": true, "count": true, "reduce": true, "sum": true, "enumerate": true, "drop": true,
	"take": true, "reverse": true, "sort": true, "keys": true, "values": true, "entries": true,
	"toList": true, "toSet": true, "join": true, "flatMap": true, "zip": true, "parallelMap": true,
}

// builtinDecls are the declarations of ir/builtin.eye, by the name a call
// uses: `log.info` for a function of a built-in module.
type builtinDecls struct {
	funcs map[string]*ir.Func // the signatures calls are checked against
	uses  map[string][]string // every built-in's uses clause
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
	c.builtins = builtinDecls{funcs: map[string]*ir.Func{}, uses: map[string][]string{}}
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
			continue
		}
		fn := &ir.Func{Name: name, Module: m.mod, Pub: true, Pos: pos(f, d), Extern: true, Host: host}
		c.lowerFunc(m, f, d, fn, ctxPlain, nil, nil)
		c.builtins.funcs[name] = fn
	}
}

// builtinEffects adds what calling a built-in does to the function being
// checked. Waiting isn't among them: outside a job, sleep blocks, and in a
// job the `!` the call needs says that it waits.
func (x *fnCheck) builtinEffects(name string) {
	for _, u := range x.c.builtins.uses[name] {
		if u != "wait" {
			x.top.effect(u)
		}
	}
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
