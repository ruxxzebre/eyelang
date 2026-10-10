package check

import (
	"eyelang/ir"
)

// The built-in functions' types. Collection functions work on several kinds
// of collection (SPECS.md §8 calls them functions over a collection), which
// one signature can't say until traits are checked, so each has a rule here.

func isBuiltin(name string) bool {
	for _, b := range ir.Builtins {
		if b == name {
			return true
		}
	}
	return false
}

// builtinArgs is a built-in call's arguments, each checked once.
type builtinArgs struct {
	x       *fnCheck
	pos     []ir.Arg
	named   map[string]ir.Arg
	types   map[int]typ
	ntypes  map[string]typ
	first   typ // the receiver's type, already checked, for `x.f(…)`
	hasRecv bool
}

func (x *fnCheck) newBuiltinArgs(args []ir.Arg, first typ) *builtinArgs {
	b := &builtinArgs{x: x, named: map[string]ir.Arg{}, types: map[int]typ{}, ntypes: map[string]typ{}}
	for _, a := range args {
		if a.Name != "" {
			b.named[a.Name] = a
			continue
		}
		b.pos = append(b.pos, a)
	}
	if first != nil {
		b.first, b.hasRecv = first, true
		b.types[0] = first
	}
	return b
}

// arg checks positional argument i against want, and returns its type, or
// nil if there is no such argument.
func (b *builtinArgs) arg(i int, want typ) typ {
	if i >= len(b.pos) {
		return nil
	}
	if t, ok := b.types[i]; ok {
		if want != nil {
			b.x.expect(b.pos[i].Value.Position(), want, t)
		}
		return t
	}
	t := b.x.check(b.pos[i].Value, want)
	b.types[i] = t
	return t
}

func (b *builtinArgs) name(n string, want typ) typ {
	a, ok := b.named[n]
	if !ok {
		return nil
	}
	if t, ok := b.ntypes[n]; ok {
		return t
	}
	t := b.x.check(a.Value, want)
	b.ntypes[n] = t
	return t
}

// rest checks the arguments no rule looked at.
func (b *builtinArgs) rest() {
	for i := range b.pos {
		b.arg(i, nil)
	}
	for n := range b.named {
		b.name(n, nil)
	}
}

func (x *fnCheck) builtin(name string, args []ir.Arg, want typ, at ir.Pos) typ {
	return x.builtinWith(name, args, nil, want, at)
}

// builtinWith checks a call of a built-in function. first, when set, is the
// type of the first argument, already checked: the receiver of `x.f(…)`.
func (x *fnCheck) builtinWith(name string, args []ir.Arg, first typ, want typ, at ir.Pos) typ {
	c := x.c
	b := x.newBuiltinArgs(args, first)
	defer b.rest()
	option := func(t typ) typ { return &tcon{name: "Option", decl: c.ts.option, args: []typ{t}} }
	result := func(t, e typ) typ { return &tcon{name: "Result", decl: c.ts.result, args: []typ{t, e}} }
	fn := func(ret typ, params ...typ) typ {
		f := &tfun{ret: ret}
		for _, p := range params {
			f.params = append(f.params, fparam{t: p})
		}
		return f
	}
	list := func(t typ) typ { return con("List", t) }
	switch name {
	case "print":
		x.top.effect("stdio")
		return tUnit
	case "log.debug", "log.info", "log.warn", "log.error":
		return tUnit
	case "unreachable":
		b.arg(0, tString)
		return c.fresh()
	case "expect":
		b.arg(0, tBool)
		return tUnit
	case "advance":
		b.arg(0, tDuration)
		return tUnit
	case "clock":
		b.arg(0, tInstant)
		return tHandler
	case "fakes":
		b.name("from", tString)
		return tHandler
	case "approve", "decline":
		b.arg(0, tString)
		return tUnit
	case "sleep":
		b.arg(0, tDuration)
		return tUnit
	case "sleepUntil":
		b.arg(0, tInstant)
		return tUnit
	case "daily", "weekdays":
		b.arg(0, con("Time"))
		b.name("in", tZone)
		return con("Schedule")
	case "every":
		b.arg(0, tDuration)
		return con("Schedule")
	case "now":
		x.top.effect("clock")
		return tInstant
	case "poll":
		b.name("every", tDuration)
		b.name("within", tDuration)
		b.arg(0, fn(tBool))
		return tBool
	case "next":
		event := c.fresh()
		b.arg(0, con("Event", event))
		b.name("where", fn(tBool, event))
		b.name("within", tDuration)
		return option(event)
	case "approval":
		b.arg(0, tString)
		b.name("within", tDuration)
		return &tcon{name: "Decision", decl: c.ts.decision}
	case "toJson":
		b.arg(0, nil)
		return tString
	case "lines", "split":
		b.arg(0, tString)
		b.arg(1, tString)
		return list(tString)
	case "startsWith":
		b.arg(0, tString)
		b.arg(1, tString)
		return tBool
	case "trim", "lower", "upper":
		b.arg(0, tString)
		return tString
	case "env.get":
		x.top.effect("env")
		b.arg(0, tString)
		return option(tString)
	case "secret":
		b.arg(0, tString)
		return con("Secret", tString)
	case "parseInt":
		b.arg(0, tString)
		return result(tInt, tString)
	case "nonzero":
		b.arg(0, tInt)
		return option(tInt)
	case "min", "max":
		t := b.arg(0, nil)
		if t == nil {
			return c.fresh()
		}
		b.arg(1, t)
		return t
	case "abs":
		if t := b.arg(0, nil); t != nil {
			return t
		}
		return c.fresh()
	case "isOk", "isErr":
		b.arg(0, result(c.fresh(), c.fresh()))
		return tBool
	case "mapErr":
		ok, e, f := c.fresh(), c.fresh(), c.fresh()
		b.arg(0, result(ok, e))
		b.arg(1, fn(f, e))
		return result(ok, f)
	}

	// Collection functions: the first argument says which kind.
	coll := b.arg(0, nil)
	if coll == nil {
		return c.fresh()
	}
	kind, elem := x.collection(coll)
	same := func() typ {
		if kind == "Set" {
			return coll
		}
		return list(elem)
	}
	switch name {
	case "size":
		return tInt
	case "isEmpty":
		return tBool
	case "count":
		b.arg(1, fn(tBool, elem))
		return tInt
	case "add":
		b.arg(1, elem)
		if kind == "" {
			return coll
		}
		return same()
	case "remove":
		if k, _, ok := mapOf(coll); ok {
			b.arg(1, k)
			return coll
		}
		b.arg(1, elem)
		return coll
	case "contains":
		switch kind {
		case "String":
			b.arg(1, tString)
		case "Map":
			k, _, _ := mapOf(coll)
			b.arg(1, k)
		default:
			b.arg(1, elem)
		}
		return tBool
	case "insert":
		k, v := typ(c.fresh()), typ(c.fresh())
		if mk, mv, ok := mapOf(coll); ok {
			k, v = mk, mv
		} else {
			x.expect(b.pos[0].Value.Position(), con("Map", k, v), coll)
		}
		b.arg(1, k)
		b.arg(2, v)
		return con("Map", k, v)
	case "get":
		if k, v, ok := mapOf(coll); ok {
			b.arg(1, k)
			return option(v)
		}
		b.arg(1, tInt)
		return option(elem)
	case "first", "last":
		return option(elem)
	case "map":
		out := c.fresh()
		switch k := prune(coll).(type) {
		case *tcon:
			switch k.decl {
			case c.ts.option:
				b.arg(1, fn(out, k.args[0]))
				return option(out)
			case c.ts.result:
				b.arg(1, fn(out, k.args[0]))
				return result(out, k.args[1])
			}
		}
		b.arg(1, fn(out, elem))
		return list(out)
	case "flatMap":
		out := c.fresh()
		b.arg(1, fn(list(out), elem))
		return list(out)
	case "filter":
		b.arg(1, fn(tBool, elem))
		return same()
	case "find":
		b.arg(1, fn(tBool, elem))
		return option(elem)
	case "any", "all":
		b.arg(1, fn(tBool, elem))
		return tBool
	case "reduce":
		acc := b.arg(1, nil)
		if acc == nil {
			acc = c.fresh()
		}
		b.arg(2, fn(acc, acc, elem))
		return acc
	case "sum":
		return elem
	case "enumerate":
		return list(&ttuple{elems: []typ{tInt, elem}})
	case "drop", "take":
		b.arg(1, tInt)
		if kind == "String" {
			return tString
		}
		return list(elem)
	case "reverse", "sort", "toList":
		return list(elem)
	case "toSet":
		return con("Set", elem)
	case "keys":
		if k, _, ok := mapOf(coll); ok {
			return list(k)
		}
		return list(c.fresh())
	case "values":
		if _, v, ok := mapOf(coll); ok {
			return list(v)
		}
		return list(c.fresh())
	case "entries":
		if k, v, ok := mapOf(coll); ok {
			return list(&ttuple{elems: []typ{k, v}})
		}
		return list(c.fresh())
	case "join":
		b.arg(1, tString)
		return tString
	case "zip":
		other := b.arg(1, nil)
		_, e2 := x.collection(orNil(other))
		return list(&ttuple{elems: []typ{elem, e2}})
	}
	return c.fresh()
}

// collection says what kind of collection t is, and the type of its
// elements: a map's are its (key, value) pairs.
func (x *fnCheck) collection(t typ) (string, typ) {
	switch k := prune(t).(type) {
	case *tcon:
		if k.decl == nil && k.entity == nil {
			switch k.name {
			case "List", "Set", "Iter", "Range":
				if len(k.args) == 1 {
					return k.name, k.args[0]
				}
			case "Map":
				if len(k.args) == 2 {
					return "Map", &ttuple{elems: []typ{k.args[0], k.args[1]}}
				}
			case "String":
				return "String", tChar
			case "Bytes":
				return "Bytes", con("Byte")
			}
		}
		if k.decl == x.c.ts.option && len(k.args) == 1 {
			return "Option", k.args[0]
		}
	case *tvar:
		if k.lit == litString {
			return "String", tChar
		}
	}
	return "", x.c.fresh()
}

func mapOf(t typ) (typ, typ, bool) {
	if k, ok := prune(t).(*tcon); ok && k.decl == nil && k.name == "Map" && len(k.args) == 2 {
		return k.args[0], k.args[1], true
	}
	return nil, nil, false
}
