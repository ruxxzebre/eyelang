package check

import (
	"sort"

	"eyelang/ir"
)

// checkExterns checks the extern fns of the program's own modules (SPECS.md
// §18): each says where its code comes from, @library for a C function or
// @host for one the Go program that embeds Eyelang provides, and a C
// function's parameters and result are types that can cross to C. What a
// host function can take depends on what provides it: the interpreter's own,
// such as http.get and the REPL's, take any value, and the embedding API
// checks a Go function against its declaration when it is registered.
func (c *checker) checkExterns() {
	for _, m := range c.prog.Modules {
		if m.Dir == "" {
			continue // the prelude, the standard modules and the store
		}
		names := make([]string, 0, len(m.Funcs))
		for name, fn := range m.Funcs {
			if fn.Extern {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		for _, name := range names {
			c.checkExtern(m.Funcs[name])
		}
	}
}

func (c *checker) checkExtern(fn *ir.Func) {
	switch {
	case fn.Library != "" && fn.Host != "":
		c.errorf(fn.Pos, "%s is both @library and @host: an extern fn is either a C function or one the Go program that embeds Eyelang provides", fn.Name)
		return
	case fn.Host != "":
		return
	case fn.Library == "":
		c.errorf(fn.Pos, `%s has no body, so it needs @library("…"), naming the C library it is in, or @host("…"), naming the function the Go program that embeds Eyelang provides`, fn.Name)
		return
	}
	sig := c.funcType(fn, nil)
	for i, p := range sig.params {
		if why := c.crossesToC(p.t, false); why != "" {
			c.errorf(c.typeAt(fn.Params[i].Type, fn.Pos), "%s", why)
		}
	}
	if why := c.crossesToC(sig.ret, true); why != "" {
		c.errorf(c.typeAt(fn.Return, fn.Pos), "%s", why)
	}
}

// typeAt is where a type is written, or at when it isn't.
func (c *checker) typeAt(t *ir.TypeExpr, at ir.Pos) ir.Pos {
	if p, ok := c.typePos[t]; ok && t != nil {
		return p
	}
	return at
}

// crossesToC says why a value of type t can't cross to C as a parameter or
// the result of a @library extern fn, or "" when it can: Bool, Byte, the
// fixed-width integers, Float, String and a program's extern types go both
// ways, Bytes only to C, an Option of an extern type is a pointer that may
// be NULL, and the result may be ().
func (c *checker) crossesToC(t typ, result bool) string {
	k, ok := prune(t).(*tcon)
	if !ok {
		return cantCross(t)
	}
	if k.decl != nil && k.decl.Extern {
		return ""
	}
	if k.decl == nil && k.entity == nil {
		switch k.name {
		case "Bool", "Byte", "Int32", "Int64", "UInt32", "UInt64", "Float", "String":
			return ""
		case "Bytes":
			if result {
				return "a C function can't return Bytes: C doesn't say how many bytes a pointer points to; return an extern type instead"
			}
			return ""
		case "Int":
			return "C has no integer of Int's unbounded size: write Int64, or Int32 for a C int"
		case "()":
			if result {
				return ""
			}
		}
	}
	if k.decl != nil && k.decl == c.ts.option && len(k.args) == 1 {
		if x, ok := prune(k.args[0]).(*tcon); ok && x.decl != nil && x.decl.Extern {
			return "" // a pointer that may be NULL
		}
	}
	return cantCross(t)
}

func cantCross(t typ) string {
	return show(t) + " can't cross to C: a C function takes and returns Bool, Byte, Int32, Int64, UInt32, UInt64, Float, String and extern types, takes Bytes, and takes and returns Option of an extern type for a pointer that may be NULL"
}
