package check

import (
	"fmt"
	"strings"

	"eyelang/ir"
)

// This file holds the checker's types: how they are written, unified and
// shown. Inference itself is in infer.go.
//
// A type is a constructor applied to arguments (Int, List(a), a declared
// type, an entity's snapshot), a function, a tuple, a type variable, or
// unknown. Unknown is what the checker uses where the spec doesn't yet say
// enough to type something (an inline record type):
// it agrees with every type, so it never causes an error.
type typ interface{ isType() }

type (
	// tcon is a named type: a built-in one (Name only), a declared one
	// (Decl), or an entity's snapshot (Entity).
	tcon struct {
		name   string
		decl   *ir.Type
		entity *ir.Entity
		args   []typ
		// A command's result: its declared variants and Uncertain(key).
		uncertain bool
		// The command names a @reconcile query, so .reconcile!() resolves it.
		reconcilable bool
	}
	tfun struct {
		params []fparam
		ret    typ
		bounds []boundUse // traits a call must give the function's type variables
	}
	ttuple struct{ elems []typ }
	tvar   struct {
		id    int
		ref   typ    // the type it stands for, once known
		name  string // a declared type variable's name, for messages
		rigid bool   // a declared type variable inside its own function: it stands for any type
		lit   litKind
	}
	tunknown struct{}
	// tjob is a job as a value: `wake.start(id)`.
	tjob struct{ job *ir.Job }
	// tentity is an entity type as a value: `World(id)`, `World.all`.
	tentity struct{ e *ir.Entity }
	// tmodule is a module as a value, as in `fakes(notify, from: "…")`.
	tmodule struct{}
	// tdyn is `dyn Trait`: a value of some type that implements the trait,
	// chosen at run time (SPECS.md §8). A value of such a type converts to
	// it where it is expected; calls on it see only the trait's methods.
	tdyn struct {
		trait *ir.Trait
		args  []typ
	}
)

func (*tdyn) isType() {}

func (*tcon) isType()    {}
func (*tfun) isType()    {}
func (*ttuple) isType()  {}
func (*tvar) isType()    {}
func (tunknown) isType() {}
func (*tjob) isType()    {}
func (*tentity) isType() {}
func (tmodule) isType()  {}

// fparam is a function's parameter, as far as calls are concerned.
type fparam struct {
	name       string
	t          typ
	hasDefault bool
	mut        bool // a mut parameter: the call passes `mut x`
}

// boundUse is a trait a call of a generic function needs a type variable to
// have: `sort` needs its elements to be Ord.
type boundUse struct {
	t  typ
	b  cbound
	fn string // the function, for messages
	v  string // its type variable's name
}

// litKind says which types a literal may still take: a string literal is a
// String, a Key or a Zone, and an integer literal an Int or a fixed-width
// integer, depending on where it goes (SPECS.md §5). A literal nothing
// decides is a String or an Int.
type litKind int

const (
	litNone litKind = iota
	litString
	litInt
)

var unknown typ = tunknown{}

func con(name string, args ...typ) *tcon { return &tcon{name: name, args: args} }

var (
	tInt      = con("Int")
	tFloat    = con("Float")
	tBool     = con("Bool")
	tChar     = con("Char")
	tString   = con("String")
	tUnit     = con("()")
	tDuration = con("Duration")
	tSize     = con("Size")
	tDate     = con("Date")
	tTime     = con("Time")
	tInstant  = con("Instant")
	tZone     = con("Zone")
	tKey      = con("Key")
	tHandler  = con("Handler")
)

// prune follows bound type variables to the type they stand for.
func prune(t typ) typ {
	for {
		v, ok := t.(*tvar)
		if !ok || v.ref == nil {
			return t
		}
		t = v.ref
	}
}

// literal types each kind of literal may take.
func (k litKind) allows(c *tcon) bool {
	switch k {
	case litString:
		return c.decl == nil && c.entity == nil && (c.name == "String" || c.name == "Key" || c.name == "Zone")
	case litInt:
		switch c.name {
		case "Int", "Int32", "Int64", "UInt32", "UInt64", "Byte":
			return c.decl == nil
		}
		return false
	}
	return true
}

// mismatch is why two types don't unify. dyn is set when a value can't be
// converted to a dyn Trait because its type doesn't implement the trait.
type mismatch struct {
	want, got typ
	dyn       *tdyn
	lacks     typ // the type that doesn't implement dyn's trait
}

// unify makes a and b the same type, binding type variables as needed. It
// reports the first pair of types that differ.
func (c *checker) unify(want, got typ) *mismatch {
	a, b := prune(want), prune(got)
	if a == b {
		return nil
	}
	if _, ok := a.(tunknown); ok {
		return nil
	}
	if _, ok := b.(tunknown); ok {
		return nil
	}
	if v, ok := a.(*tvar); ok && !v.rigid {
		return c.bind(v, b, want, got)
	}
	if v, ok := b.(*tvar); ok && !v.rigid {
		return c.bind(v, a, want, got)
	}
	// differ is the mismatch of want and got, keeping why a part of them
	// couldn't be a dyn Trait.
	differ := func(inner *mismatch) *mismatch {
		m := &mismatch{want: want, got: got}
		if inner != nil {
			m.dyn, m.lacks = inner.dyn, inner.lacks
		}
		return m
	}
	switch x := a.(type) {
	case *tvar:
		return differ(nil) // a rigid variable only matches itself
	case *tcon:
		y, ok := b.(*tcon)
		if !ok || !sameCon(x, y) || len(x.args) != len(y.args) {
			return differ(nil)
		}
		for i := range x.args {
			if m := c.unify(x.args[i], y.args[i]); m != nil {
				return differ(m)
			}
		}
		return nil
	case *ttuple:
		y, ok := b.(*ttuple)
		if !ok || len(x.elems) != len(y.elems) {
			return differ(nil)
		}
		for i := range x.elems {
			if m := c.unify(x.elems[i], y.elems[i]); m != nil {
				return differ(m)
			}
		}
		return nil
	case *tfun:
		y, ok := b.(*tfun)
		if !ok || len(x.params) != len(y.params) {
			return differ(nil)
		}
		for i := range x.params {
			// A function that is expected takes what the caller gives it,
			// so its parameters go the other way: a fn(dyn Shape) will do
			// where a fn(Square) is wanted, not the reverse.
			if m := c.unify(y.params[i].t, x.params[i].t); m != nil {
				return differ(m)
			}
		}
		if m := c.unify(x.ret, y.ret); m != nil {
			return differ(m)
		}
		return nil
	case *tjob:
		if y, ok := b.(*tjob); ok && y.job == x.job {
			return nil
		}
	case *tentity:
		if y, ok := b.(*tentity); ok && y.e == x.e {
			return nil
		}
	case tmodule:
		if _, ok := b.(tmodule); ok {
			return nil
		}
	case *tdyn:
		if y, ok := b.(*tdyn); ok {
			if y.trait != x.trait || len(x.args) != len(y.args) {
				return differ(nil)
			}
			for i := range x.args {
				if m := c.unify(x.args[i], y.args[i]); m != nil {
					return differ(m)
				}
			}
			return nil
		}
		// A value converts to dyn Trait where one is expected, if its type
		// implements the trait. Values never alias, so a List(Square) is a
		// List(dyn Shape) too.
		if ok, _ := c.has(b, cbound{trait: x.trait, args: x.args}, map[*ir.Type]bool{}); ok {
			return nil
		}
		return &mismatch{want: want, got: got, dyn: x, lacks: b}
	}
	return differ(nil)
}

// sameCon reports whether two named types are the same type. A command's
// result, with Uncertain, is still its declared type: what may be missing is
// checked where it is matched.
func sameCon(x, y *tcon) bool {
	switch {
	case x.decl != nil || y.decl != nil:
		return x.decl == y.decl
	case x.entity != nil || y.entity != nil:
		return x.entity == y.entity
	}
	return x.name == y.name
}

func (c *checker) bind(v *tvar, t typ, want, got typ) *mismatch {
	if w, ok := t.(*tvar); ok {
		if w.rigid {
			if v.lit != litNone {
				return &mismatch{want: want, got: got}
			}
			v.ref = w
			return nil
		}
		// Two variables: keep what either literal allows.
		if w.lit == litNone || w.lit == v.lit {
			w.lit = max(w.lit, v.lit)
			v.ref = w
			return nil
		}
		if v.lit == litNone {
			v.lit = w.lit // what the literal allows goes with it
			w.ref = v
			return nil
		}
		return &mismatch{want: want, got: got}
	}
	if v.lit != litNone {
		ct, ok := t.(*tcon)
		if !ok || !v.lit.allows(ct) {
			return &mismatch{want: want, got: got}
		}
	}
	if occurs(v, t) {
		return &mismatch{want: want, got: got}
	}
	v.ref = t
	return nil
}

func occurs(v *tvar, t typ) bool {
	switch x := prune(t).(type) {
	case *tvar:
		return x == v
	case *tcon:
		for _, a := range x.args {
			if occurs(v, a) {
				return true
			}
		}
	case *ttuple:
		for _, e := range x.elems {
			if occurs(v, e) {
				return true
			}
		}
	case *tfun:
		for _, p := range x.params {
			if occurs(v, p.t) {
				return true
			}
		}
		return occurs(v, x.ret)
	case *tdyn:
		for _, a := range x.args {
			if occurs(v, a) {
				return true
			}
		}
	}
	return false
}

func (c *checker) fresh() *tvar {
	c.nextVar++
	return &tvar{id: c.nextVar}
}

func (c *checker) freshLit(k litKind) *tvar {
	v := c.fresh()
	v.lit = k
	return v
}

// defaultLiterals settles the literals nothing else decided: a string
// literal is a String and an integer literal an Int.
func defaultLiteral(t typ) typ {
	t = prune(t)
	if v, ok := t.(*tvar); ok && !v.rigid {
		switch v.lit {
		case litString:
			v.ref = tString
		case litInt:
			v.ref = tInt
		}
	}
	return prune(t)
}

// show writes a type as a program would.
func show(t typ) string {
	var b strings.Builder
	names := map[*tvar]string{}
	writeType(&b, t, names)
	return b.String()
}

func writeType(b *strings.Builder, t typ, names map[*tvar]string) {
	switch x := prune(t).(type) {
	case *tcon:
		name := x.name
		if x.entity != nil {
			name = x.entity.Name
		}
		b.WriteString(name)
		if len(x.args) > 0 {
			b.WriteString("(")
			for i, a := range x.args {
				if i > 0 {
					b.WriteString(", ")
				}
				writeType(b, a, names)
			}
			b.WriteString(")")
		}
	case *tfun:
		b.WriteString("fn(")
		for i, p := range x.params {
			if i > 0 {
				b.WriteString(", ")
			}
			writeType(b, p.t, names)
		}
		b.WriteString(")")
		if r := prune(x.ret); r != tUnit {
			if rc, ok := r.(*tcon); !ok || rc.name != "()" {
				b.WriteString(" -> ")
				writeType(b, x.ret, names)
			}
		}
	case *ttuple:
		b.WriteString("(")
		for i, e := range x.elems {
			if i > 0 {
				b.WriteString(", ")
			}
			writeType(b, e, names)
		}
		b.WriteString(")")
	case *tvar:
		switch {
		case x.name != "":
			b.WriteString(x.name)
		case x.lit == litString:
			b.WriteString("String")
		case x.lit == litInt:
			b.WriteString("Int")
		default:
			n, ok := names[x]
			if !ok {
				n = string(rune('a' + len(names)%26))
				names[x] = n
			}
			b.WriteString(n)
		}
	case tunknown:
		b.WriteString("_")
	case *tjob:
		fmt.Fprintf(b, "job %s", x.job.Name)
	case *tentity:
		fmt.Fprintf(b, "entity %s", x.e.Name)
	case tmodule:
		b.WriteString("module")
	case *tdyn:
		b.WriteString("dyn ")
		b.WriteString(x.trait.Name)
		if len(x.args) > 0 {
			b.WriteString("(")
			for i, a := range x.args {
				if i > 0 {
					b.WriteString(", ")
				}
				writeType(b, a, names)
			}
			b.WriteString(")")
		}
	}
}

// ---------------------------------------------------------------- types as written

// typeEnv maps the type variables in scope to their types.
type typeEnv map[string]typ

// fromExpr turns a type as written into a type. Type variables are looked up
// in env; a name not in env is added to it as a fresh variable, rigid when
// rigid is set (a function's own type variables, inside its body).
func (c *checker) fromExpr(t *ir.TypeExpr, env typeEnv, rigid bool) typ {
	if t == nil {
		return unknown
	}
	args := func() []typ {
		out := make([]typ, len(t.Args))
		for i, a := range t.Args {
			out[i] = c.fromExpr(a, env, rigid)
		}
		return out
	}
	switch {
	case t.Decl != nil:
		d := t.Decl
		if d.Alias != nil && len(d.Fields) == 0 && len(d.Variants) == 0 {
			inner := typeEnv{}
			as := args()
			for i, p := range d.Params {
				if i < len(as) {
					inner[p] = as[i]
				}
			}
			return c.fromExpr(d.Alias, inner, false)
		}
		as := args()
		for len(as) < len(d.Params) {
			as = append(as, c.fresh())
		}
		return &tcon{name: d.Name, decl: d, args: as}
	case t.Name == "Ref" && len(t.Args) == 1:
		return con("Ref", c.fromExpr(t.Args[0], env, rigid))
	case t.Entity != nil:
		return &tcon{name: t.Entity.Name, entity: t.Entity}
	}
	switch t.Name {
	case "()":
		return tUnit
	case "tuple":
		return &ttuple{elems: args()}
	case "dyn":
		if len(t.Args) == 1 && t.Args[0].Trait != nil {
			d := &tdyn{trait: t.Args[0].Trait}
			for _, a := range t.Args[0].Args {
				d.args = append(d.args, c.fromExpr(a, env, rigid))
			}
			return d
		}
		return unknown // reported where it is written
	case "fn":
		f := &tfun{ret: tUnit}
		for _, a := range t.Args {
			f.params = append(f.params, fparam{t: c.fromExpr(a, env, rigid)})
		}
		if t.Result != nil {
			f.ret = c.fromExpr(t.Result, env, rigid)
		}
		return f
	case "record", "?":
		return unknown // not typed yet: inline record types
	case "NonZero":
		// A NonZero divisor is approximated by its number type.
		if len(t.Args) == 1 {
			return c.fromExpr(t.Args[0], env, rigid)
		}
		return tInt
	}
	if t.Name != "" && t.Name[0] >= 'a' && t.Name[0] <= 'z' {
		if v, ok := env[t.Name]; ok {
			return v
		}
		v := c.fresh()
		v.name, v.rigid = t.Name, rigid
		env[t.Name] = v
		return v
	}
	if t.Name == "" {
		return unknown
	}
	if builtinTypes[t.Name] {
		return con(t.Name, args()...)
	}
	return unknown // a contract's name, or a name already reported as undefined
}

// declArgs returns the environment of a declared type's parameters for one
// use of it, and the type itself.
func (c *checker) declType(d *ir.Type) (*tcon, typeEnv) {
	env := typeEnv{}
	t := &tcon{name: d.Name, decl: d}
	for _, p := range d.Params {
		v := c.fresh()
		env[p] = v
		t.args = append(t.args, v)
	}
	return t, env
}

// envOf returns the environment a declared type's arguments give its fields.
func envOf(t *tcon) typeEnv {
	env := typeEnv{}
	if t.decl != nil {
		for i, p := range t.decl.Params {
			if i < len(t.args) {
				env[p] = t.args[i]
			}
		}
	}
	return env
}

// fieldType returns the type of a field of a record, a snapshot or a
// variant with named fields, if it has one.
func (c *checker) fieldType(t typ, name string) (typ, bool) {
	switch x := prune(t).(type) {
	case *tcon:
		switch {
		case x.entity != nil:
			return c.stateType(x.entity, name)
		case x.name == "Ref" && x.decl == nil && len(x.args) == 1:
			if s, ok := prune(x.args[0]).(*tcon); ok && s.entity != nil {
				return c.stateType(s.entity, name)
			}
		case x.name == "Range" && x.decl == nil && len(x.args) == 1 && (name == "start" || name == "end"):
			return x.args[0], true
		case x.decl != nil && x.decl.Alias != nil:
			rec := x.decl.Alias
			for i, f := range rec.Fields {
				if f == name && i < len(rec.Args) {
					return c.fromExpr(rec.Args[i], envOf(x), false), true
				}
			}
		case x.decl != nil && len(x.decl.Variants) == 1:
			for _, f := range x.decl.Variants[0].Fields {
				if f.Name == name {
					return c.fromExpr(f.Type, envOf(x), false), true
				}
			}
		}
	}
	return nil, false
}

// stateType is the type of an entity's state field, its identity or its
// version.
func (c *checker) stateType(e *ir.Entity, name string) (typ, bool) {
	if e.ID != nil && e.ID.Name == name {
		return c.fromExpr(e.ID.Type, typeEnv{}, false), true
	}
	if name == "version" {
		return tInt, true
	}
	for _, s := range e.States {
		if s.Name == name {
			return c.fromExpr(s.Type, typeEnv{}, false), true
		}
	}
	return nil, false
}

// isRecord reports whether t is a declared record type.
func isRecord(t typ) (*tcon, bool) {
	x, ok := prune(t).(*tcon)
	if !ok || x.decl == nil || x.decl.Alias == nil || x.decl.Alias.Name != "record" {
		return nil, false
	}
	return x, true
}

// toExpr turns a type back into a type as written, for the IR. It returns
// nil if any part of the type isn't known.
func toExpr(t typ) *ir.TypeExpr {
	switch x := prune(t).(type) {
	case *tcon:
		out := &ir.TypeExpr{Name: x.name, Decl: x.decl, Entity: x.entity}
		for _, a := range x.args {
			e := toExpr(a)
			if e == nil {
				return nil
			}
			out.Args = append(out.Args, e)
		}
		if out.Name == "Ref" && len(out.Args) == 1 {
			out.Entity = out.Args[0].Entity
		}
		return out
	case *ttuple:
		out := &ir.TypeExpr{Name: "tuple"}
		for _, e := range x.elems {
			te := toExpr(e)
			if te == nil {
				return nil
			}
			out.Args = append(out.Args, te)
		}
		return out
	case *tvar:
		if x.rigid && x.name != "" {
			return &ir.TypeExpr{Name: x.name}
		}
		switch x.lit {
		case litString:
			return &ir.TypeExpr{Name: "String"}
		case litInt:
			return &ir.TypeExpr{Name: "Int"}
		}
	}
	return nil
}
