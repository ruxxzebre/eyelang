package ir

import (
	"fmt"
	"strings"
)

// Ty is a resolved type, as the checker computed it: the type of an
// expression of a program that checks (Expr.Type). A TypeExpr is a type as
// written, with names still to look up; a Ty has every name resolved and
// every type variable that a use decided replaced by its type.
type Ty interface {
	isTy()
	String() string
}

type (
	// TCon is a named type applied to its arguments: a built-in type (Name
	// only, as Int or List(String)), a declared one (Decl), or an entity's
	// snapshot (Entity). Uncertain is set on a command's result, which may
	// also be Uncertain(key).
	TCon struct {
		Name      string
		Decl      *Type
		Entity    *Entity
		Args      []Ty
		Uncertain bool
	}
	// TFun is a function type and the effects it states.
	TFun struct {
		Params []Ty
		Ret    Ty
		Uses   []string
	}
	TTuple struct{ Elems []Ty }
	// TVar is a type variable of a generic function, inside that function:
	// a declared one by its name, as `a`, and one the checker inferred for
	// a private function by a name it makes up. ID tells them apart.
	TVar struct {
		ID   int
		Name string
	}
	// TDyn is `dyn Trait`.
	TDyn struct {
		Trait *Trait
		Args  []Ty
	}
	// TNever is the type of what gives no value: `return`, `break`, a
	// fault, an endless loop.
	TNever struct{}
	// TJob is a job as a value (`wake.start(id)`), TEntity an entity type as
	// a value (`World(id)`, `World.all`), and TModule a module as a value.
	TJob    struct{ Job *Job }
	TEntity struct{ Entity *Entity }
	TModule struct{}
)

func (*TCon) isTy()    {}
func (*TFun) isTy()    {}
func (*TTuple) isTy()  {}
func (*TVar) isTy()    {}
func (*TDyn) isTy()    {}
func (TNever) isTy()   {}
func (*TJob) isTy()    {}
func (*TEntity) isTy() {}
func (TModule) isTy()  {}

// String writes a type as a program would, with a made-up type variable's
// name and the effects of a function type.
func (t *TCon) String() string {
	name := t.Name
	if t.Entity != nil {
		name = t.Entity.Name
	}
	if len(t.Args) == 0 {
		return name
	}
	return name + "(" + joinTys(t.Args) + ")"
}

func (t *TFun) String() string {
	s := "fn(" + joinTys(t.Params) + ")"
	if c, ok := t.Ret.(*TCon); !ok || c.Name != "()" || c.Decl != nil {
		s += " -> " + tyString(t.Ret)
	}
	if t.Uses != nil {
		s += " uses {" + strings.Join(t.Uses, ", ") + "}"
	}
	return s
}

func (t *TTuple) String() string { return "(" + joinTys(t.Elems) + ")" }

func (t *TVar) String() string {
	if t.Name != "" {
		return t.Name
	}
	return fmt.Sprintf("t%d", t.ID)
}

func (t *TDyn) String() string {
	if len(t.Args) == 0 {
		return "dyn " + t.Trait.Name
	}
	return "dyn " + t.Trait.Name + "(" + joinTys(t.Args) + ")"
}

func (TNever) String() string     { return "Never" }
func (t *TJob) String() string    { return "job " + t.Job.Name }
func (t *TEntity) String() string { return "entity " + t.Entity.Name }
func (TModule) String() string    { return "module" }
func tyString(t Ty) string {
	if t == nil {
		return "?"
	}
	return t.String()
}

func joinTys(ts []Ty) string {
	parts := make([]string, len(ts))
	for i, t := range ts {
		parts[i] = tyString(t)
	}
	return strings.Join(parts, ", ")
}
