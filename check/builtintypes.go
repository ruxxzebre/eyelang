package check

import (
	"strings"

	"eyelang/ir"
	"eyelang/syntax"
)

// The types of the built-in functions one signature can't describe yet
// (goRules): collection functions work on several kinds of collection
// (SPECS.md §8 calls them functions over a collection), which one signature
// can't say until traits are checked, so each has a rule here. Every other
// built-in is checked against its declaration in ir/builtin.eye.

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

// builtinWith checks a call of a built-in function: against its declaration
// in ir/builtin.eye, or by its rule here when it is one of goRules. first,
// when set, is the type of the first argument, already checked: the receiver
// of `x.f(…)`.
func (x *fnCheck) builtinWith(name string, args []ir.Arg, first typ, want typ, at ir.Pos) typ {
	c := x.c
	x.builtinEffects(name, args)
	if fn := c.builtins.funcs[name]; fn != nil {
		return x.declaredBuiltin(fn, args, first, at)
	}
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
	case "print", "log.debug", "log.info", "log.warn", "log.error":
		for i := range b.pos {
			x.requireNamed(b.arg(i, nil), "Show", b.pos[i].Value.Position(), name)
		}
		return tUnit
	case "fakes":
		b.name("from", tString)
		return tHandler
	case "channel":
		a := c.fresh()
		if b.name("capacity", tInt) == nil {
			c.typeErrorf(at, "channel needs a capacity: how many values it holds before a put waits, as channel(capacity: 64)")
		}
		return &ttuple{elems: []typ{con("Sender", a), con("Receiver", a)}}
	}

	// Collection functions: the first argument says which kind.
	x.ruleShape(name, b, at)
	coll := b.arg(0, nil)
	if coll == nil {
		return c.fresh()
	}
	if v, ok := prune(coll).(*tvar); ok && v.lit == litBraces {
		// `{} |> add(x)`: empty braces are a set, or a map to the map
		// functions.
		switch name {
		case "insert", "get", "keys", "values", "entries":
			c.unify(con("Map", c.fresh(), c.fresh()), coll)
		default:
			c.unify(con("Set", c.fresh()), coll)
		}
	}
	kind, elem := x.collection(coll)
	x.collectionKind(name, kind, coll, b.pos[0].Value.Position())
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
	case "parallelMap":
		out := c.fresh()
		b.arg(1, fn(out, elem))
		b.name("limit", tInt)
		return list(out)
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
		x.require(elem, c.preludeTrait("Add"), []typ{elem, elem}, at, "sum")
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
		if name == "sort" {
			x.requireNamed(elem, "Ord", at, "sort")
		}
		return list(elem)
	case "toSet":
		x.requireNamed(elem, "Hash", at, "toSet")
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

// ruleParams reads the parameters of a built-in declaration that goRules
// types: their names and whether they have defaults.
func ruleParams(f *ir.File, d *syntax.Node) []fparam {
	var out []fparam
	for _, p := range d.ChildByField(syntax.FieldParameters).NamedChildren() {
		out = append(out, fparam{name: text(f, p.ChildByField(syntax.FieldName)), hasDefault: p.ChildByField(syntax.FieldDefault) != nil})
	}
	return out
}

// optionalRuleParams are parameters that a goRules built-in may go without,
// as its documentation says, though a function type can't have a default:
// `count` without a test is the size.
var optionalRuleParams = map[string]bool{"count.test": true}

// ruleShape checks a call of a goRules built-in against its declared
// parameters: not too many arguments, none it doesn't name, and every one
// it needs.
func (x *fnCheck) ruleShape(name string, b *builtinArgs, at ir.Pos) {
	params, ok := x.c.builtins.rules[name]
	if !ok {
		return
	}
	if len(b.pos) > len(params) {
		x.c.typeErrorf(b.pos[len(params)].Value.Position(), "too many arguments to %s: it takes %d", name, len(params))
	}
	given := map[string]bool{}
	for i := range b.pos {
		if i < len(params) {
			given[params[i].name] = true
		}
	}
	for n, a := range b.named {
		found := false
		for _, p := range params {
			found = found || p.name == n
		}
		if !found {
			x.c.typeErrorf(a.Value.Position(), "%s has no parameter %s", name, n)
		}
		given[n] = true
	}
	for _, p := range params {
		if !given[p.name] && !p.hasDefault && !optionalRuleParams[name+"."+p.name] {
			x.c.typeErrorf(at, "%s needs an argument for %s", name, p.name)
		}
	}
}

// collectionKinds are the kinds of collection each collection function
// takes; the others take every kind that has elements.
var collectionKinds = map[string][]string{
	"add":     {"List", "Set"},
	"remove":  {"Set", "Map"},
	"insert":  {"Map"},
	"keys":    {"Map"},
	"values":  {"Map"},
	"entries": {"Map"},
	"get":     {"Map", "List", "Bytes"},
	"map":     {"List", "Set", "Map", "Range", "String", "Iter", "Bytes", "Option", "Result"},
}

var elementKinds = []string{"List", "Set", "Map", "Range", "String", "Iter", "Bytes"}

// collectionKind reports a collection function given what isn't a kind of
// collection it takes.
func (x *fnCheck) collectionKind(name, kind string, coll typ, at ir.Pos) {
	kinds, ok := collectionKinds[name]
	if !ok {
		kinds = elementKinds
	}
	for _, k := range kinds {
		if k == kind {
			return
		}
	}
	switch t := defaultLiteral(coll).(type) {
	case tunknown:
		return
	case *tvar:
		if !t.rigid {
			x.c.typeErrorf(at, "the type of %s isn't known here, so %s can't tell what kind of collection it is: write its type where it is declared", at.Text(), name)
			return
		}
	}
	x.c.typeErrorf(at, "%s takes %s, and this is %s", name, orList(articled(kinds)), show(defaultLiteral(coll)))
}

// articled puts "a" or "an" before each kind's name.
func articled(kinds []string) []string {
	out := make([]string, len(kinds))
	for i, k := range kinds {
		out[i] = "a " + k
		if strings.ContainsRune("AEIOU", rune(k[0])) {
			out[i] = "an " + k
		}
	}
	return out
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
		if k.decl == x.c.ts.result && len(k.args) == 2 {
			return "Result", k.args[0]
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
