package check

import (
	"fmt"
	"sort"
	"strings"

	"eyelang/ir"
	"eyelang/syntax"
)

// Traits (SPECS.md §11): declarations, `impl Trait for Type`, the bounds on
// type variables (`where a: Ord`, or a trait as a parameter's type), and
// whether a type has a trait. Eq, Ord, Show, Hash, Save and Schema are
// derived: a type has them when its parts do. The operator traits (Add, Sub,
// Mul, Div, Rem, Neg) come with the built-in combinations of §11, and any
// type can implement them.
//
// Coherence (open question 2, decided): an impl lives in the module that
// declares the trait or the module that declares the type, so two modules
// can't give one type two meanings of a trait.

// derived are the traits a type has when its parts have them.
var derived = map[string]bool{"Eq": true, "Ord": true, "Show": true, "Hash": true, "Save": true, "Schema": true}

// opTraits are the operator traits, by operator.
var opTraits = map[string]string{"+": "Add", "-": "Sub", "*": "Mul", "/": "Div", "%": "Rem"}

// traitDecl collects `trait Name(params) { fn … }`.
func (c *checker) traitDecl(m *module, f *ir.File, d *syntax.Node) {
	nameNode := d.ChildByField(syntax.FieldName)
	tr := &ir.Trait{Name: text(f, nameNode), Module: m.mod, Methods: map[string]*ir.Func{}, Pos: pos(f, d), Doc: docComment(f, d)}
	if tp := d.ChildByField(syntax.FieldTypeParameters); tp != nil {
		for _, p := range tp.NamedChildren() {
			tr.Params = append(tr.Params, text(f, p))
		}
	}
	m.mod.Traits[tr.Name] = tr
	c.define(m, f, nameNode, tr.Name, tr)
	for _, member := range d.ChildByField(syntax.FieldBody).NamedChildren() {
		if member.Kind != syntax.FunctionSignature && member.Kind != syntax.FunctionDeclaration {
			continue
		}
		mn := member.ChildByField(syntax.FieldName)
		fn := &ir.Func{Name: text(f, mn), Module: m.mod, Pos: pos(f, member)}
		if params := member.ChildByField(syntax.FieldParameters).NamedChildren(); len(params) > 0 {
			if first := params[0].ChildByField(syntax.FieldName); first != nil && text(f, first) == "self" {
				fn.HasSelf = true
				fn.MutSelf = hasToken(f, params[0], "mut")
			}
		}
		if !fn.HasSelf {
			c.errorf(pos(f, mn), "%s.%s takes self first: a trait's methods are called on a value", tr.Name, fn.Name)
		}
		if _, dup := tr.Methods[fn.Name]; dup {
			c.errorf(pos(f, mn), "%s.%s is declared twice", tr.Name, fn.Name)
			continue
		}
		tr.Methods[fn.Name] = fn
		tr.Order = append(tr.Order, fn.Name)
		member := member
		m.decls = append(m.decls, func() { c.lowerFunc(m, f, member, fn, ctxPlain, nil, nil) })
	}
}

// implDecl collects `impl Type { … }`, `impl Trait for Type { … }` and
// `impl Contract { … }`.
func (c *checker) implDecl(m *module, f *ir.File, d *syntax.Node) {
	typeNode := d.ChildByField(syntax.FieldType)
	traitNode := d.ChildByField(syntax.FieldTrait)
	if ct := c.contractNamed(m, f, typeNode); ct != nil && traitNode == nil {
		c.use(pos(f, typeNode), ct)
		c.serve(m, f, d, ct)
		return
	}
	name := text(f, typeNode)
	if typeNode.Kind == syntax.GenericType {
		name = text(f, typeNode.ChildByField(syntax.FieldName))
	}
	if traitNode != nil {
		entity := name
		if name == "Ref" {
			entity = "Ref(…)"
			if args := typeNode.ChildByField(syntax.FieldArguments); args != nil && len(args.NamedChildren()) == 1 {
				entity = text(f, args.NamedChildren()[0])
			}
		}
		if _, ok := c.lookupType(m, entity).(*ir.Entity); ok || name == "Ref" {
			// An entity's methods are transactions; what a trait method on
			// one would be isn't decided (ROADMAP G.6).
			c.errorf(pos(f, typeNode), "an entity can't implement a trait yet, so impl %s for %s does nothing: implement it for a record that holds the entity's Ref, such as type %sLabel = { ref: Ref(%s) }",
				text(f, traitNode), text(f, typeNode), entity, entity)
			return
		}
	}
	t := m.mod.Types[name]
	if t == nil {
		if v, ok := m.types[name].(*ir.Variant); ok && v.Type.Module == m.mod {
			t = v.Type
		}
	}
	if t == nil && (traitNode == nil || !builtinTypes[name] && c.lookupType(m, name) == nil) {
		if traitNode == nil {
			c.errorf(pos(f, typeNode), "impl for %s, which isn't a type declared in this module", name)
		} else {
			c.errorf(pos(f, typeNode), "impl for %s, which isn't a type", name)
		}
		return
	}
	if t != nil {
		if typeNode.Kind == syntax.GenericType {
			c.use(pos(f, typeNode.ChildByField(syntax.FieldName)), t)
		} else {
			c.use(pos(f, typeNode), t)
		}
	}
	var impl *ir.Impl
	if traitNode != nil {
		impl = &ir.Impl{Module: m.mod, Methods: map[string]*ir.Func{}, Pos: pos(f, d)}
		m.mod.Impls = append(m.mod.Impls, impl)
		if t != nil {
			t.Impls = append(t.Impls, impl)
		}
	}
	for _, member := range d.ChildByField(syntax.FieldBody).NamedChildren() {
		if member.Kind != syntax.FunctionDeclaration {
			continue
		}
		mn := member.ChildByField(syntax.FieldName)
		fn := &ir.Func{Name: text(f, mn), Module: m.mod, Receiver: t, Pub: hasToken(f, member, "pub"), Pos: pos(f, member)}
		if params := member.ChildByField(syntax.FieldParameters).NamedChildren(); len(params) > 0 {
			if first := params[0].ChildByField(syntax.FieldName); first != nil && text(f, first) == "self" {
				fn.HasSelf = true
				fn.MutSelf = hasToken(f, params[0], "mut")
			}
		}
		switch {
		case impl != nil:
			if _, dup := impl.Methods[fn.Name]; dup {
				c.errorf(pos(f, mn), "%s is declared twice in this impl", fn.Name)
				continue
			}
			impl.Methods[fn.Name] = fn
			c.public[fn] = true // a trait's method is as public as the trait
			if t != nil {
				if _, taken := t.Methods[fn.Name]; !taken {
					t.Methods[fn.Name] = fn
				}
			}
		default:
			if _, dup := t.Methods[fn.Name]; dup {
				c.errorf(pos(f, mn), "%s.%s is declared twice", t.Name, fn.Name)
			}
			t.Methods[fn.Name] = fn
		}
		member := member
		m.decls = append(m.decls, func() { c.lowerFunc(m, f, member, fn, ctxPlain, nil, t) })
	}
	if impl == nil {
		return
	}
	m.decls = append(m.decls, func() { c.implTrait(m, f, d, impl, t) })
}

// implTrait resolves an impl's trait once every module is collected, and
// checks the impl against it.
func (c *checker) implTrait(m *module, f *ir.File, d *syntax.Node, impl *ir.Impl, t *ir.Type) {
	traitNode := d.ChildByField(syntax.FieldTrait)
	typeNode := d.ChildByField(syntax.FieldType)
	te := c.typeExpr(m, f, traitNode, nil)
	if te.Trait == nil {
		if te.Decl != nil || te.Entity != nil || builtinTypes[te.Name] {
			c.errorf(pos(f, traitNode), "%s isn't a trait, so it can't be implemented; impl %s { … } gives a type methods", te.Name, text(f, typeNode))
		}
		return
	}
	tr := te.Trait
	impl.Trait, impl.Args = tr, te.Args
	impl.For = c.typeExpr(m, f, typeNode, nil)
	at := pos(f, d)
	if len(impl.Args) != len(tr.Params) {
		c.errorf(pos(f, traitNode), "%s takes %d type arguments, and this impl gives %d: impl %s for %s", tr.Name, len(tr.Params), len(impl.Args), traitText(tr), text(f, typeNode))
	}
	// Coherence: the impl is in the trait's module or the type's.
	if tr.Module != m.mod && (t == nil || t.Module != m.mod) {
		c.errorf(at, "impl %s for %s belongs in the module that declares %s or the one that declares %s", tr.Name, impl.For.Name, tr.Name, impl.For.Name)
	}
	if t != nil {
		for _, other := range t.Impls {
			if other != impl && other.Trait == tr && sameTypeArgs(other.Args, impl.Args) {
				c.errorf(at, "%s implements %s twice", t.Name, traitText2(tr, impl.Args))
			}
		}
	}
	for name, fn := range impl.Methods {
		if _, ok := tr.Methods[name]; !ok {
			c.errorf(fn.Pos, "%s isn't a method of %s, so it doesn't belong in impl %s for %s; put it in impl %s { … }", name, tr.Name, tr.Name, impl.For.Name, impl.For.Name)
		}
	}
	var missing []string
	for _, name := range tr.Order {
		tm := tr.Methods[name]
		if _, ok := impl.Methods[name]; ok {
			continue
		}
		if tm.Body != nil {
			impl.Methods[name] = tm // the trait's default
			if t != nil {
				if _, taken := t.Methods[name]; !taken {
					t.Methods[name] = tm
				}
			}
			continue
		}
		missing = append(missing, name)
	}
	if len(missing) > 0 {
		c.errorf(at, "impl %s for %s doesn't implement %s, which %s needs", tr.Name, impl.For.Name, strings.Join(missing, ", "), tr.Name)
	}
}

func traitText(tr *ir.Trait) string {
	if len(tr.Params) == 0 {
		return tr.Name
	}
	return tr.Name + "(" + strings.Join(tr.Params, ", ") + ")"
}

func traitText2(tr *ir.Trait, args []*ir.TypeExpr) string {
	if len(args) == 0 {
		return tr.Name
	}
	var parts []string
	for _, a := range args {
		parts = append(parts, typeExprText(a))
	}
	return tr.Name + "(" + strings.Join(parts, ", ") + ")"
}

func typeExprText(t *ir.TypeExpr) string {
	if t == nil {
		return "_"
	}
	if t.Name == "fn" && t.Decl == nil {
		var parts []string
		for _, a := range t.Args {
			parts = append(parts, typeExprText(a))
		}
		s := "fn(" + strings.Join(parts, ", ") + ")"
		if t.Result != nil {
			s += " -> " + typeExprText(t.Result)
		}
		if t.HasUses {
			s += " uses {" + strings.Join(t.Uses, ", ") + "}"
		}
		return s
	}
	if len(t.Args) == 0 {
		return t.Name
	}
	var parts []string
	for _, a := range t.Args {
		parts = append(parts, typeExprText(a))
	}
	return t.Name + "(" + strings.Join(parts, ", ") + ")"
}

func sameTypeArgs(a, b []*ir.TypeExpr) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if typeExprText(a[i]) != typeExprText(b[i]) {
			return false
		}
	}
	return true
}

// lowerBounds reads a function's bounds: a trait written as a parameter's
// type stands for a new type variable with that trait, and `where` lists the
// rest. A trait anywhere else a type goes is an error.
func (c *checker) lowerBounds(m *module, f *ir.File, d *syntax.Node, fn *ir.Func) {
	used := map[string]bool{}
	for _, p := range fn.Params {
		collectVars(p.Type, used)
	}
	collectVars(fn.Return, used)
	for _, p := range fn.Params {
		if p.Type == nil {
			continue
		}
		if tr := p.Type.Trait; tr != nil {
			v := strings.ToLower(tr.Name[:1]) + tr.Name[1:]
			for n := 2; used[v]; n++ {
				v = fmt.Sprintf("%s%d", strings.ToLower(tr.Name[:1])+tr.Name[1:], n)
			}
			used[v] = true
			fn.Bounds = append(fn.Bounds, ir.Bound{Var: v, Trait: tr, Args: p.Type.Args, Pos: fn.Pos})
			p.Type = &ir.TypeExpr{Name: v}
			continue
		}
		if tr := nestedTrait(p.Type); tr != nil {
			c.errorf(fn.Pos, "%s's parameter %s has %s inside its type, and %s is a trait: name a type variable and require it, as `List(t) where t: %s`, or write dyn %s for values of different types", fn.Name, p.Name, tr.Name, tr.Name, tr.Name, tr.Name)
		}
	}
	if tr := nestedTrait(fn.Return); tr != nil {
		c.errorf(fn.Pos, "%s returns a %s, and %s is a trait: a function returns a type, so write dyn %s for a value whose type is chosen at run time", fn.Name, tr.Name, tr.Name, tr.Name)
	}
	w := d.ChildByField(syntax.FieldRequirements)
	if w == nil {
		return
	}
	for _, r := range w.NamedChildren() {
		nameNode := r.ChildByField(syntax.FieldName)
		name := text(f, nameNode)
		if !used[name] {
			c.errorf(pos(f, nameNode), "where names %s, which isn't a type variable of %s", name, fn.Name)
		}
		for _, tn := range r.ChildrenByField(syntax.FieldTrait) {
			te := c.typeExpr(m, f, tn, nil)
			if te.Trait == nil {
				if te.Decl != nil || te.Entity != nil || builtinTypes[te.Name] {
					c.errorf(pos(f, tn), "%s isn't a trait, so it can't be required of %s", te.Name, name)
				}
				continue
			}
			fn.Bounds = append(fn.Bounds, ir.Bound{Var: name, Trait: te.Trait, Args: te.Args, Pos: pos(f, tn)})
		}
	}
}

// collectVars adds the type variables a type mentions.
func collectVars(t *ir.TypeExpr, into map[string]bool) {
	if t == nil {
		return
	}
	if t.Decl == nil && t.Entity == nil && t.Trait == nil && t.Name != "" && t.Name[0] >= 'a' && t.Name[0] <= 'z' &&
		t.Name != "record" && t.Name != "tuple" && t.Name != "fn" && t.Name != "dyn" {
		into[t.Name] = true
	}
	for _, a := range t.Args {
		collectVars(a, into)
	}
	collectVars(t.Result, into)
}

// nestedTrait finds a trait written where a type goes inside t.
func nestedTrait(t *ir.TypeExpr) *ir.Trait {
	if t == nil {
		return nil
	}
	if t.Name == "dyn" {
		return nil // dyn Trait is a type (v0.2)
	}
	parts := t.Args
	if t.Result != nil {
		parts = append(parts[:len(parts):len(parts)], t.Result)
	}
	for _, a := range parts {
		if a.Trait != nil {
			return a.Trait
		}
		if tr := nestedTrait(a); tr != nil {
			return tr
		}
	}
	return nil
}

// ---------------------------------------------------------------- whether a type has a trait

// cbound is a trait with its type arguments, as types.
type cbound struct {
	trait *ir.Trait
	args  []typ
}

// requirement is a trait a type must have, settled when the body's types
// are known.
type requirement struct {
	t     typ
	b     cbound
	at    ir.Pos
	where string // what asks for it, for the message: "sort", "comparing with <"
	role  string // the callee's type variable it is required of, for a call
}

// require records that t must have the trait.
func (x *fnCheck) require(t typ, tr *ir.Trait, args []typ, at ir.Pos, where string) {
	if tr == nil {
		return
	}
	x.top.reqs = append(x.top.reqs, requirement{t: t, b: cbound{trait: tr, args: args}, at: at, where: where})
}

// requireNamed records a requirement of one of the prelude's traits.
func (x *fnCheck) requireNamed(t typ, name string, at ir.Pos, where string) {
	x.require(t, x.c.preludeTrait(name), nil, at, where)
}

func (c *checker) preludeTrait(name string) *ir.Trait {
	if c.prelude == nil {
		return nil
	}
	return c.prelude.mod.Traits[name]
}

// settleRequirements reports each requirement the body's types don't
// meet. One on a type nothing has decided yet, such as an untyped
// parameter's, waits until every body is checked: a later call may decide
// it (settleDeferred).
func (x *fnCheck) settleRequirements() {
	for _, r := range x.reqs {
		free := map[*tvar]bool{}
		freeVars(defaultLiteral(r.t), free)
		for _, a := range r.b.args {
			freeVars(a, free)
		}
		if len(free) > 0 {
			x.c.ts.deferred = append(x.c.ts.deferred, r)
			continue
		}
		x.c.settle(r)
	}
}

// settleDeferred settles the requirements that waited for the whole
// program. A type still undecided may be any type, so it meets them.
func (c *checker) settleDeferred() {
	for _, r := range c.ts.deferred {
		c.settle(r)
	}
}

// settle reports a requirement the types don't meet.
func (c *checker) settle(r requirement) {
	t := defaultLiteral(r.t)
	ok, why := c.has(t, r.b, map[*ir.Type]bool{})
	if ok {
		return
	}
	need := traitName(r.b)
	var msg string
	if r.role != "" {
		msg = fmt.Sprintf("%s needs %s: %s, and %s is %s here, which has no %s", r.where, r.role, need, r.role, show(t), r.b.trait.Name)
	} else {
		msg = fmt.Sprintf("%s needs %s, and %s has no %s", r.where, need, show(t), r.b.trait.Name)
	}
	if why != "" {
		msg += ": " + why
	}
	if v, isVar := prune(t).(*tvar); isVar && v.rigid {
		msg = fmt.Sprintf("%s needs %s, and %s may be any type: require it, as `where %s: %s`", r.where, need, v.name, v.name, need)
	}
	c.typeErrorf(r.at, "%s", msg)
}

func traitName(b cbound) string {
	if len(b.args) == 0 {
		return b.trait.Name
	}
	var parts []string
	for _, a := range b.args {
		parts = append(parts, show(defaultLiteral(a)))
	}
	return b.trait.Name + "(" + strings.Join(parts, ", ") + ")"
}

// has reports whether t has the trait, and if not, why not when the reason
// lies in its parts.
func (c *checker) has(t typ, b cbound, seen map[*ir.Type]bool) (bool, string) {
	tr := b.trait
	isDerived := derived[tr.Name] && tr.Module == c.prelude.mod
	switch x := prune(t).(type) {
	case tunknown, tdiscard:
		return true, ""
	case *tvar:
		if !x.rigid {
			return true, "" // nothing decided its type: any type will do
		}
		for _, have := range c.ts.bounds[x] {
			if have.trait == tr && c.unifyArgs(have.args, b.args) {
				return true, ""
			}
		}
		return false, ""
	case *tfun:
		if isDerived {
			return false, "a function has none of Eq, Ord, Show, Hash, Save and Schema"
		}
		return false, ""
	case *ttuple:
		if isDerived {
			for _, e := range x.elems {
				if ok, why := c.has(e, b, seen); !ok {
					return false, partWhy("an element", e, b, why)
				}
			}
			return true, ""
		}
		return false, ""
	case *tjob, *tentity, tmodule:
		return false, ""
	case *tdyn:
		// A dyn value has its trait and nothing else: what its type is
		// otherwise is known only at run time.
		if x.trait == tr && c.unifyArgs(x.args, b.args) {
			return true, ""
		}
		return false, fmt.Sprintf("a dyn %s has only the methods of %s", x.trait.Name, x.trait.Name)
	case *tcon:
		if c.explicitImpl(x, b) {
			return true, ""
		}
		switch {
		case x.entity != nil:
			if !isDerived {
				return false, ""
			}
			for _, s := range x.entity.States {
				st := c.fromExpr(s.Type, typeEnv{}, false)
				if ok, why := c.has(st, b, seen); !ok {
					return false, partWhy("its state "+s.Name, st, b, why)
				}
			}
			return true, ""
		case x.decl != nil:
			if !isDerived {
				return false, ""
			}
			if x.decl.Extern {
				return false, fmt.Sprintf("%s is an extern type, whose values foreign code owns, so it has none of Eq, Ord, Show, Hash, Save and Schema", x.decl.Name)
			}
			if seen[x.decl] {
				return true, "" // a recursive type: its parts are being checked
			}
			seen[x.decl] = true
			defer delete(seen, x.decl)
			env := envOf(x)
			if x.decl.Alias != nil {
				for i, name := range x.decl.Alias.Fields {
					if i < len(x.decl.Alias.Args) {
						ft := c.fromExpr(x.decl.Alias.Args[i], env, false)
						if ok, why := c.has(ft, b, seen); !ok {
							return false, partWhy("its field "+name, ft, b, why)
						}
					}
				}
				if len(x.decl.Alias.Fields) == 0 {
					return c.has(c.fromExpr(x.decl.Alias, env, false), b, seen)
				}
				return true, ""
			}
			for _, v := range x.decl.Variants {
				for _, field := range v.Fields {
					ft := c.fromExpr(field.Type, env, false)
					if ok, why := c.has(ft, b, seen); !ok {
						return false, partWhy(v.Name+"'s field", ft, b, why)
					}
				}
			}
			return true, ""
		}
		return c.builtinHas(x, b, seen)
	}
	return false, ""
}

func partWhy(part string, t typ, b cbound, why string) string {
	s := fmt.Sprintf("%s is %s, which has no %s", part, show(t), b.trait.Name)
	if why != "" && !strings.Contains(why, "has no") {
		s += " (" + why + ")"
	}
	return s
}

// unifyArgs makes a requirement's trait arguments those of a bound that
// meets it.
func (c *checker) unifyArgs(have, want []typ) bool {
	if len(have) != len(want) {
		return false
	}
	for i := range have {
		if c.unify(want[i], have[i]) != nil {
			return false
		}
	}
	return true
}

// explicitImpl reports whether a named type has an `impl` of the trait whose
// arguments fit the requirement's.
func (c *checker) explicitImpl(x *tcon, b cbound) bool {
	for _, m := range c.prog.Modules {
		for _, impl := range m.Impls {
			if impl.Trait != b.trait || impl.For == nil {
				continue
			}
			if !implFor(impl, x) {
				continue
			}
			env := envOf(x)
			var args []typ
			for _, a := range impl.Args {
				args = append(args, c.fromExpr(a, env, false))
			}
			if len(b.args) == 0 || c.unifyArgs(args, b.args) {
				return true
			}
		}
	}
	return false
}

// implFor reports whether an impl is for the type x.
func implFor(impl *ir.Impl, x *tcon) bool {
	switch {
	case x.decl != nil:
		return impl.For.Decl == x.decl
	case x.entity != nil:
		return impl.For.Entity == x.entity && impl.For.Name != "Ref"
	}
	return impl.For.Decl == nil && impl.For.Entity == nil && impl.For.Name == x.name
}

// implMethod finds a method an impl gives a built-in type, as `impl Show
// for Money` does a declared one.
func (c *checker) implMethod(x *tcon, name string) *ir.Func {
	for _, m := range c.prog.Modules {
		for _, impl := range m.Impls {
			if impl.For != nil && implFor(impl, x) {
				if fn, ok := impl.Methods[name]; ok && fn.HasSelf {
					return fn
				}
			}
		}
	}
	return nil
}

// builtinHas says which traits the built-in types have.
func (c *checker) builtinHas(x *tcon, b cbound, seen map[*ir.Type]bool) (bool, string) {
	name := b.trait.Name
	if b.trait.Module != c.prelude.mod {
		return false, ""
	}
	if !derived[name] {
		if op := opOf(name); op != "" && len(b.args) == 2 {
			return c.builtinOp(x, op, b.args)
		}
		if name == "Neg" && len(b.args) == 1 {
			switch x.name {
			case "Int", "Float", "Duration":
				return c.unify(b.args[0], x) == nil, ""
			}
		}
		return false, ""
	}
	parts := func(why string) (bool, string) {
		for _, a := range x.args {
			if ok, w := c.has(a, b, seen); !ok {
				return false, partWhy(why, a, b, w)
			}
		}
		return true, ""
	}
	switch x.name {
	case "Int", "Float", "Bool", "Char", "String", "Byte", "Bytes", "Int32", "Int64", "UInt32", "UInt64",
		"Duration", "Size", "Date", "Time", "Instant", "Zone", "Key", "()", "NonZero":
		return true, ""
	case "Secret":
		return false, "a Secret can't be shown, saved or compared, so it never leaks"
	case "Iter":
		return false, "an Iter holds the functions of its pipeline, so it has none of Eq, Ord, Show, Hash, Save and Schema: collect it with toList"
	case "List", "Option", "Result":
		return parts("an element")
	case "Set", "Map", "Range":
		if name == "Ord" {
			return false, ""
		}
		return parts("an element")
	case "Ref":
		if name == "Schema" {
			return false, ""
		}
		return true, ""
	case "Run", "Approval", "Version":
		return name != "Schema" && name != "Ord", ""
	}
	return false, ""
}

func opOf(trait string) string {
	for op, t := range opTraits {
		if t == trait {
			return op
		}
	}
	return ""
}

// builtinOp says whether a built-in type has an operator trait with these
// arguments: the combinations of SPECS.md §11.
func (c *checker) builtinOp(x *tcon, op string, args []typ) (bool, string) {
	rhs, out := args[0], args[1]
	try := func(r, o typ) bool {
		if !equalTypes(prune(rhs), r) {
			if _, isVar := prune(rhs).(*tvar); !isVar {
				return false
			}
		}
		return c.unify(rhs, r) == nil && c.unify(out, o) == nil
	}
	switch x.name {
	case "Int", "Float":
		return try(x, x), ""
	case "String", "List":
		return op == "+" && try(x, x), ""
	case "Duration":
		if op == "*" || op == "/" {
			return try(tInt, tDuration), ""
		}
		return (op == "+" || op == "-") && try(tDuration, tDuration), ""
	case "Size":
		if op == "*" || op == "/" {
			return try(tInt, tSize), ""
		}
		return (op == "+" || op == "-") && try(tSize, tSize), ""
	case "Instant":
		if op == "-" && try(tInstant, tDuration) {
			return true, ""
		}
		return (op == "+" || op == "-") && try(tDuration, tInstant), ""
	case "Date":
		return (op == "+" || op == "-") && try(tDuration, tDate), ""
	}
	return false, ""
}

// ---------------------------------------------------------------- impls in the type pass

// fillImplMethods gives an impl's methods the types its trait declares
// where the impl leaves them out, as a contract's operations do for a
// served contract: `fn name(self) = self.nick` takes its result from the
// trait.
func (c *checker) fillImplMethods() {
	for _, m := range c.prog.Modules {
		for _, impl := range m.Impls {
			if impl.Trait == nil {
				continue
			}
			sub := map[string]*ir.TypeExpr{}
			for i, p := range impl.Trait.Params {
				if i < len(impl.Args) {
					sub[p] = impl.Args[i]
				}
			}
			for name, fn := range impl.Methods {
				tm := impl.Trait.Methods[name]
				if tm == nil || tm == fn {
					continue
				}
				if len(fn.Params) != len(tm.Params) || fn.HasSelf != tm.HasSelf {
					c.typeErrorf(fn.Pos, "%s takes %s in %s, and this one takes %s", name, plural(len(tm.Params), "argument")+selfNote(tm), impl.Trait.Name, plural(len(fn.Params), "argument")+selfNote(fn))
					continue
				}
				for i, p := range fn.Params {
					want := substType(tm.Params[i].Type, sub)
					if p.Type == nil {
						p.Type = want
					} else if want != nil && typeExprText(p.Type) != typeExprText(want) && !isVarName(want.Name) {
						c.typeErrorf(fn.Pos, "%s's parameter %s is %s here, and %s says %s", name, p.Name, typeExprText(p.Type), traitText2(impl.Trait, impl.Args), typeExprText(want))
					}
				}
				want := substType(tm.Return, sub)
				if fn.Return == nil {
					fn.Return = want
				} else if want != nil && typeExprText(fn.Return) != typeExprText(want) && !isVarName(want.Name) {
					c.typeErrorf(fn.Pos, "%s returns %s here, and %s says %s", name, typeExprText(fn.Return), traitText2(impl.Trait, impl.Args), typeExprText(want))
				}
			}
		}
	}
}

func selfNote(fn *ir.Func) string {
	if fn.HasSelf {
		return " besides self"
	}
	return ""
}

func isVarName(n string) bool { return n != "" && n[0] >= 'a' && n[0] <= 'z' }

// substType replaces a trait's type parameters with an impl's arguments.
func substType(t *ir.TypeExpr, sub map[string]*ir.TypeExpr) *ir.TypeExpr {
	if t == nil {
		return nil
	}
	if t.Decl == nil && t.Entity == nil && len(t.Args) == 0 {
		if r, ok := sub[t.Name]; ok {
			return r
		}
	}
	cp := *t
	cp.Args = nil
	for _, a := range t.Args {
		cp.Args = append(cp.Args, substType(a, sub))
	}
	cp.Result = substType(t.Result, sub)
	return &cp
}

// opImpl finds a declared type's impl of an operator's trait whose right-hand
// type fits r, and returns its right-hand and result types and the method
// that answers the operator.
func (x *fnCheck) opImpl(k *tcon, op string, r typ) (rhs, out typ, method *ir.Func, found bool) {
	c := x.c
	name := opTraits[op]
	var candidates []*ir.Impl
	for _, m := range c.prog.Modules {
		for _, impl := range m.Impls {
			if impl.Trait != nil && impl.Trait.Name == name && impl.Trait.Module == c.prelude.mod && impl.For != nil && implFor(impl, k) && len(impl.Args) == 2 {
				candidates = append(candidates, impl)
			}
		}
	}
	if len(candidates) == 0 {
		return nil, nil, nil, false
	}
	env := envOf(k)
	pick := candidates[0]
	if len(candidates) > 1 {
		for _, impl := range candidates {
			if equalTypes(prune(c.fromExpr(impl.Args[0], env, false)), prune(r)) {
				pick = impl
			}
		}
	}
	return c.fromExpr(pick.Args[0], env, false), c.fromExpr(pick.Args[1], env, false), pick.Methods[strings.ToLower(name)], true
}

// ---------------------------------------------------------------- dyn Trait

// checkDyns reports each `dyn Trait` whose trait can't be used that way: one
// with a method that has type variables of its own, which a call through
// dyn couldn't choose (SPECS.md §8).
func (c *checker) checkDyns() {
	seen := map[ir.Pos]bool{}
	for _, d := range c.dyns {
		if seen[d.at] {
			continue // a declaration lowered twice, as the REPL does
		}
		seen[d.at] = true
		if d.args != len(d.trait.Params) {
			c.errorf(d.at, "%s takes %d type arguments, and this dyn gives %d: dyn %s", d.trait.Name, len(d.trait.Params), d.args, traitText(d.trait))
			continue
		}
		if name, vars := genericMethod(d.trait); name != "" {
			c.errorf(d.at, "dyn %s can't be used: its method %s has type variables of its own (%s), which a call through dyn can't choose", d.trait.Name, name, strings.Join(vars, ", "))
		}
	}
}

// genericMethod returns the first method of a trait, in source order, whose
// signature has type variables besides the trait's parameters, and them.
func genericMethod(tr *ir.Trait) (string, []string) {
	own := map[string]bool{}
	for _, p := range tr.Params {
		own[p] = true
	}
	for _, name := range tr.Order {
		m := tr.Methods[name]
		vars := map[string]bool{}
		for _, p := range m.Params {
			collectVars(p.Type, vars)
		}
		collectVars(m.Return, vars)
		var extra []string
		for v := range vars {
			if !own[v] {
				extra = append(extra, v)
			}
		}
		if len(extra) > 0 {
			sort.Strings(extra)
			return name, extra
		}
	}
	return "", nil
}
