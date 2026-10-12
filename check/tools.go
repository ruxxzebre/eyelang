package check

import (
	"fmt"

	"eyelang/ir"
)

// A model's tools (SPECS.md §19): a function declared with `fn` and named
// where an ai.Tool is expected converts to one, as a value converts to
// `dyn Trait`, so `tools: [searchIssues, label]` holds functions of
// different types. Its parameters give the tool's input schema and its
// result goes back to the model, so both need Schema. Named as a value, its
// effects count where it is named, as any function's do (§12): a public
// function that passes label lists label's effects.

// toolUse is a function named as a tool, and where.
type toolUse struct {
	fn  *ir.Func
	ref *ir.Ref
}

// isTool reports whether t is the ai module's Tool.
func isTool(t typ) bool {
	x, ok := prune(orNil(t)).(*tcon)
	if !ok || x.decl == nil || !x.decl.Extern || x.decl.Name != "Tool" {
		return false
	}
	m := x.decl.Module
	return m != nil && m.Path == "ai" && m.Dir == ""
}

// tool checks a function named where a Tool is expected, and returns the
// Tool type; it returns nil for any other expression, which is checked as
// usual.
func (x *fnCheck) tool(e ir.Expr, want typ) typ {
	if want == nil || !isTool(want) {
		return nil
	}
	r, ok := e.(*ir.Ref)
	if !ok {
		return nil
	}
	fn, ok := r.Decl.(*ir.Func)
	if !ok {
		return nil
	}
	c := x.c
	t := x.ref(r) // counts its effects here, as for any function named as a value
	c.typed(r.Pos, t)
	c.ts.exprs = append(c.ts.exprs, typedExpr{r, t})
	f, isFn := prune(t).(*tfun)
	if !isFn {
		return want
	}
	c.ts.tools = append(c.ts.tools, toolUse{fn: fn, ref: r})
	if fn.Extern || fn.Body == nil {
		c.typeErrorf(r.Pos, "%s is an extern fn, and a tool is a function declared with fn, whose ## comment and parameters describe it to the model: wrap it in one", fn.Name)
		return want
	}
	params := fn.Params
	if fn.Destructive {
		params = params[:len(params)-1] // the approval, which the runtime asks a person for
	}
	ok = true
	for i, p := range params {
		switch {
		case p.Type == nil:
			c.typeErrorf(r.Pos, "%s's parameter %s has no written type, and a tool's input schema comes from its parameters' types: write it, as %s: String", fn.Name, p.Name, p.Name)
			ok = false
		case hasTypeVar(p.Type):
			c.typeErrorf(r.Pos, "%s is generic, and a tool has one input schema: give its parameter %s a type without type variables", fn.Name, p.Name)
			ok = false
		case i < len(f.params):
			x.requireNamed(f.params[i].t, "Schema", r.Pos, fmt.Sprintf("%s's parameter %s, which the model fills in as a tool's input,", fn.Name, p.Name))
		}
	}
	if fn.Return != nil && hasTypeVar(fn.Return) {
		c.typeErrorf(r.Pos, "%s is generic, and what a tool returns goes back to the model as JSON: give its result a type without type variables", fn.Name)
		ok = false
	}
	if ok {
		x.requireNamed(f.ret, "Schema", r.Pos, fmt.Sprintf("what %s returns, which goes back to the model as a tool's result,", fn.Name))
	}
	return want
}

// hasTypeVar reports whether a type as written names a type variable.
func hasTypeVar(te *ir.TypeExpr) bool {
	if te == nil {
		return false
	}
	if te.Decl == nil && te.Entity == nil && te.Trait == nil && isVarName(te.Name) &&
		te.Name != "record" && te.Name != "tuple" && te.Name != "fn" && te.Name != "dyn" {
		return true
	}
	for _, a := range te.Args {
		if hasTypeVar(a) {
			return true
		}
	}
	return hasTypeVar(te.Result)
}

// isToolRef reports whether r names a function as a tool. Its calls are
// recorded steps of the model call, so naming it in a job is no mistake.
func (c *checker) isToolRef(r *ir.Ref) bool {
	for _, t := range c.ts.tools {
		if t.ref == r {
			return true
		}
	}
	return false
}

// checkTools reports a tool that waits: it runs inside its model call,
// which can't pause in the middle of a step.
func (c *checker) checkTools(all map[*ir.Func]map[string]bool) {
	for _, t := range c.ts.tools {
		if effectsOf(t.fn, all)["wait"] {
			c.typeErrorf(t.ref.Pos, "%s waits, and a tool runs inside its model call, which can't pause for it: pass a function that doesn't wait (a @destructive tool has the runtime ask a person for each call)", t.fn.Name)
		}
	}
}

// checkToolApprovals reports a call of ai.generate whose own tools: list
// names a @destructive tool, without approve: to ask a person for each of
// the model's calls of it.
func (c *checker) checkToolApprovals(e *ir.Call) {
	var list *ir.List
	for _, a := range e.Args {
		switch a.Name {
		case "approve":
			return
		case "tools":
			list, _ = a.Value.(*ir.List)
		}
	}
	if list == nil {
		return
	}
	for _, el := range list.Elems {
		if r, ok := el.(*ir.Ref); ok {
			if fn, ok := r.Decl.(*ir.Func); ok && fn.Destructive {
				c.typeErrorf(r.Pos, "%s is @destructive, so each call the model makes of it needs a person's approval: give ai.generate approve:, as approve: fn call: approval!(call, from: someone, within: 1 day)", fn.Name)
			}
		}
	}
}
