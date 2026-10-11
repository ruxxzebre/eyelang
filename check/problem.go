package check

import "eyelang/ir"

// intoProblem checks `x?` in a function whose error type is the prelude's
// Problem (SPECS.md §10): any error with Show becomes a Problem, so the call
// needs only that. It reports whether to is Problem and from isn't, so the
// rule applied; when from is a Problem already, `?` passes it on as is.
func (x *fnCheck) intoProblem(from, to typ, at ir.Pos) bool {
	problem := x.c.prelude.mod.Types["Problem"]
	t, ok := prune(to).(*tcon)
	if !ok || problem == nil || t.decl != problem {
		return false
	}
	if f, ok := prune(from).(*tcon); ok && f.decl == problem {
		return false
	}
	x.requireNamed(from, "Show", at, "? into a Problem")
	return true
}
