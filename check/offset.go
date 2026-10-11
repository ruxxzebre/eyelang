package check

// offsetHint is what a type error adds when an Int stands where a string's
// Offset goes, at any depth (`Range(Int)` for `Range(Offset)`): where
// positions come from, since a count of characters isn't one (SPECS.md §5).
func (c *checker) offsetHint(want, got typ) string {
	offset := c.prelude.mod.Types["Offset"]
	var walk func(w, g typ) bool
	walk = func(w, g typ) bool {
		wc, ok1 := prune(w).(*tcon)
		g = defaultLiteral(g)
		gc, ok2 := prune(g).(*tcon)
		if !ok1 || !ok2 {
			return false
		}
		if offset != nil && wc.decl == offset {
			return gc.decl == nil && gc.name == "Int"
		}
		if !sameCon(wc, gc) || len(wc.args) != len(gc.args) {
			return false
		}
		for i := range wc.args {
			if walk(wc.args[i], gc.args[i]) {
				return true
			}
		}
		return false
	}
	if !walk(want, got) {
		return ""
	}
	return ": a position in a string is an Offset, not an Int; it comes from s.start(), s.end(), s.indexOf(…), s.after(…) or s.before(…)"
}
