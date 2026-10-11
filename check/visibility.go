package check

import (
	"fmt"

	"eyelang/ir"
)

// Module boundaries (SPECS.md §5, §13): another module uses only what a
// module marks `pub`, and a `pub opaque` type's fields and variants stay
// inside the module that declares it, so other modules make, read and take
// apart its values only through its functions and methods.

// isPub reports whether other modules may use decl. A contract's
// operations and events are what it offers, so they always may.
func (c *checker) isPub(decl any) bool {
	switch d := decl.(type) {
	case *ir.Func:
		return d.Pub || c.public[d]
	case *ir.Global:
		return d.Pub
	case *ir.Variant:
		return d.Type == nil || c.public[d.Type]
	case *ir.Job:
		return d.Func.Pub || c.public[d]
	case *ir.Op, *ir.Event, *ir.Module, ir.Builtin, nil:
		return true
	}
	return c.public[decl]
}

// moduleOf is the module that declares decl, or nil for what every module
// has.
func moduleOf(decl any) *ir.Module {
	switch d := decl.(type) {
	case *ir.Func:
		return d.Module
	case *ir.Global:
		return d.Module
	case *ir.Variant:
		if d.Type != nil {
			return d.Type.Module
		}
	case *ir.Type:
		return d.Module
	case *ir.Entity:
		return d.Module
	case *ir.Job:
		return d.Module
	case *ir.View:
		return d.Module
	case *ir.Trait:
		return d.Module
	case *ir.Contract:
		return d.Module
	}
	return nil
}

// privateUse says why module from can't use decl, or "" when it can.
func (c *checker) privateUse(from *ir.Module, decl any, name string) string {
	owner := moduleOf(decl)
	if owner == nil || owner == from {
		return ""
	}
	if v, ok := decl.(*ir.Variant); ok && v.Type != nil && c.opaque[v.Type] {
		return fmt.Sprintf("%s is a variant of %s, which %s declares opaque: other modules make and match its values only through its functions", name, v.Type.Name, c.moduleName(owner))
	}
	if c.isPub(decl) {
		return ""
	}
	return fmt.Sprintf("%s is private to %s: declare it pub there to use it here", name, c.moduleName(owner))
}

// moduleName names a module in a message: `module lab.notify`, or the
// prelude, which every module has without importing it.
func (c *checker) moduleName(m *ir.Module) string {
	if m == c.prelude.mod {
		return "the prelude"
	}
	return "module " + m.Name
}

// opaqueTo reports whether t is a type whose fields and variants module
// from can't see: a `pub opaque` type of another module.
func (c *checker) opaqueTo(from *ir.Module, t typ) (*ir.Type, bool) {
	k, ok := prune(t).(*tcon)
	if !ok || k.decl == nil || !c.opaque[k.decl] || k.decl.Module == from || from == nil {
		return nil, false
	}
	return k.decl, true
}

// opaqueMessage says that a type's insides are hidden from this module.
func (c *checker) opaqueMessage(d *ir.Type, what string) string {
	return fmt.Sprintf("%s is opaque outside %s, so its %s can't be used here: use its functions and methods", d.Name, c.moduleName(d.Module), what)
}
