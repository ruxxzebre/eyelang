package check

import (
	"strings"

	"eyelang/ir"
)

// Exhaustiveness (SPECS.md §9): every match covers every value of its type,
// so a new variant, or a command's Uncertain outcome, reaches every caller.
// A pattern matrix is searched for a value no row matches, column by column,
// as in Maranget's "Warnings for pattern matching": a type with finitely many
// constructors (a variant type, Bool, (), a tuple or a record) is split into
// them; a list into its lengths; and any other type needs a row that matches
// anything. Arms with guards don't count.

// checkExhaustive reports a match that misses some value.
func (c *checker) checkExhaustive(m *ir.Match, t typ) {
	var rows [][]ir.Pattern
	for _, a := range m.Arms {
		if a.Guard == nil {
			rows = append(rows, []ir.Pattern{a.Pattern})
		}
	}
	if w := c.missing(rows, []typ{t}, 0); w != nil {
		c.typeErrorf(m.Pos, "this match doesn't handle %s: add an arm for it, or `_ => …`", w[0])
	}
}

// checkIrrefutable reports a `let` without `else` whose pattern doesn't
// match every value.
func (c *checker) checkIrrefutable(l *ir.Let, t typ) {
	if w := c.missing([][]ir.Pattern{{l.Pattern}}, []typ{t}, 0); w != nil {
		c.typeErrorf(l.Pos, "this pattern doesn't match %s: add `else { … }` for what it doesn't match", w[0])
	}
}

func wildLike(p ir.Pattern) bool {
	switch p.(type) {
	case ir.PWild, ir.PBind:
		return true
	}
	return false
}

// missing returns a value, one pattern per column, that no row matches, or
// nil if the rows match every value.
func (c *checker) missing(rows [][]ir.Pattern, types []typ, depth int) []string {
	if len(types) == 0 {
		if len(rows) > 0 {
			return nil
		}
		return []string{}
	}
	if depth > 32 {
		return nil
	}
	// Alternatives become rows of their own.
	var expanded [][]ir.Pattern
	for _, r := range rows {
		expanded = append(expanded, expandOr(r)...)
	}
	rows = expanded
	t := c.columnType(types[0], rows)
	rest := types[1:]
	prepend := func(head string, w []string) []string { return append([]string{head}, w...) }

	ctors, finite := c.constructors(t, rows)
	if finite {
		for _, k := range ctors {
			var spec [][]ir.Pattern
			for _, r := range rows {
				if sub, ok := k.specialize(r[0]); ok {
					spec = append(spec, append(sub, r[1:]...))
				}
			}
			if w := c.missing(spec, append(append([]typ{}, k.fields...), rest...), depth+1); w != nil {
				return prepend(k.show(w[:len(k.fields)]), w[len(k.fields):])
			}
		}
		return nil
	}
	if elem, ok := c.listColumn(t, rows); ok {
		longest := 0
		for _, r := range rows {
			if l, isList := r[0].(ir.PList); isList && len(l.Pats) > longest {
				longest = len(l.Pats)
			}
		}
		for n := 0; n <= longest+1; n++ {
			var spec [][]ir.Pattern
			for _, r := range rows {
				if sub, ok := specializeList(r[0], n); ok {
					spec = append(spec, append(sub, r[1:]...))
				}
			}
			fields := make([]typ, n)
			for i := range fields {
				fields[i] = elem
			}
			if w := c.missing(spec, append(fields, rest...), depth+1); w != nil {
				shown := "[" + strings.Join(w[:n], ", ")
				if n == longest+1 {
					shown += ", .."
				}
				return prepend(shown+"]", w[n:])
			}
		}
		return nil
	}
	// Infinitely many values: only rows that match anything help.
	var def [][]ir.Pattern
	for _, r := range rows {
		if wildLike(r[0]) {
			def = append(def, r[1:])
		}
	}
	if w := c.missing(def, rest, depth+1); w != nil {
		return prepend("_", w)
	}
	return nil
}

func expandOr(r []ir.Pattern) [][]ir.Pattern {
	if len(r) == 0 {
		return [][]ir.Pattern{r}
	}
	or, ok := r[0].(ir.POr)
	if !ok {
		return [][]ir.Pattern{r}
	}
	var out [][]ir.Pattern
	for _, alt := range or.Alts {
		out = append(out, expandOr(append([]ir.Pattern{alt}, r[1:]...))...)
	}
	return out
}

// columnType is the type of a column, or, where it isn't known, the type its
// patterns say.
func (c *checker) columnType(t typ, rows [][]ir.Pattern) typ {
	t = prune(t)
	switch t.(type) {
	case *tcon, *ttuple:
		return t
	}
	for _, r := range rows {
		switch p := r[0].(type) {
		case ir.PVariant:
			if p.Variant.Type != nil {
				vt, _ := c.declType(p.Variant.Type)
				return vt
			}
		case ir.PLit:
			if _, ok := p.Value.(bool); ok {
				return tBool
			}
		case ir.PTuple:
			elems := make([]typ, len(p.Pats))
			for i := range elems {
				elems[i] = c.fresh() // the rows decide
			}
			return &ttuple{elems: elems}
		}
	}
	return t
}

// ctor is one constructor of a type, with the types of its fields.
type ctor struct {
	name    string
	fields  []typ
	variant *ir.Variant
	lit     any      // for Bool and ()
	names   []string // a record's field names
	tuple   bool
}

func (k ctor) show(sub []string) string {
	switch {
	case k.tuple:
		return "(" + strings.Join(sub, ", ") + ")"
	case k.names != nil:
		var parts []string
		for i, n := range k.names {
			if sub[i] != "_" {
				parts = append(parts, n+": "+sub[i])
			}
		}
		if len(parts) == 0 {
			return "{ .. }"
		}
		return "{ " + strings.Join(parts, ", ") + ", .. }"
	case len(sub) > 0:
		return k.name + "(" + strings.Join(sub, ", ") + ")"
	}
	return k.name
}

// constructors lists a type's constructors, if it has finitely many.
func (c *checker) constructors(t typ, rows [][]ir.Pattern) ([]ctor, bool) {
	switch x := t.(type) {
	case *tcon:
		if x.decl != nil && len(x.decl.Variants) > 0 {
			env := envOf(x)
			var out []ctor
			for _, v := range x.decl.Variants {
				k := ctor{name: v.Name, variant: v}
				for _, f := range v.Fields {
					k.fields = append(k.fields, c.fromExpr(f.Type, env, false))
				}
				out = append(out, k)
			}
			if x.uncertain && c.ts.uncert != nil {
				out = append(out, ctor{name: "Uncertain", variant: c.ts.uncert, fields: []typ{tKey}})
			}
			return out, true
		}
		if x.decl == nil && x.entity == nil {
			switch x.name {
			case "Bool":
				return []ctor{{name: "true", lit: true}, {name: "false", lit: false}}, true
			case "()":
				return []ctor{{name: "()", lit: ir.Unit{}}}, true
			}
		}
		if names := c.recordNames(x, rows); names != nil {
			k := ctor{names: names}
			for _, n := range names {
				ft, ok := c.fieldType(x, n)
				if !ok {
					ft = c.fresh() // reported where the pattern is bound
				}
				k.fields = append(k.fields, ft)
			}
			return []ctor{k}, true
		}
	case *ttuple:
		return []ctor{{tuple: true, fields: x.elems}}, true
	}
	return nil, false
}

// recordNames lists the fields of a record or snapshot type, when rows match
// it with record patterns.
func (c *checker) recordNames(t *tcon, rows [][]ir.Pattern) []string {
	usesRecord := false
	for _, r := range rows {
		if _, ok := r[0].(ir.PRecord); ok {
			usesRecord = true
		}
	}
	if !usesRecord {
		return nil
	}
	switch {
	case t.decl != nil && t.decl.Alias != nil && t.decl.Alias.Name == "record":
		return t.decl.Alias.Fields
	case t.entity != nil:
		var names []string
		if t.entity.ID != nil {
			names = append(names, t.entity.ID.Name)
		}
		for _, s := range t.entity.States {
			names = append(names, s.Name)
		}
		return names
	}
	return nil
}

// specialize returns the sub-patterns of p for this constructor, or false if
// p can't match it.
func (k ctor) specialize(p ir.Pattern) ([]ir.Pattern, bool) {
	wild := func() []ir.Pattern {
		out := make([]ir.Pattern, len(k.fields))
		for i := range out {
			out[i] = ir.PWild{}
		}
		return out
	}
	if wildLike(p) {
		return wild(), true
	}
	switch p := p.(type) {
	case ir.PLit:
		return nil, k.lit != nil && p.Value == k.lit
	case ir.PTuple:
		if !k.tuple || len(p.Pats) != len(k.fields) {
			return nil, false
		}
		return p.Pats, true
	case ir.PRecord:
		if k.names == nil {
			return nil, false
		}
		out := wild()
		for i, n := range p.Names {
			for j, kn := range k.names {
				if kn == n {
					out[j] = p.Pats[i]
				}
			}
		}
		return out, true
	case ir.PVariant:
		if k.variant == nil || p.Variant.Name != k.variant.Name {
			return nil, false
		}
		out := wild()
		next := 0
		for i, sub := range p.Args {
			if p.Names[i] != "" {
				for j, f := range k.variant.Fields {
					if f.Name == p.Names[i] && j < len(out) {
						out[j] = sub
					}
				}
				continue
			}
			if next < len(out) {
				out[next] = sub
			}
			next++
		}
		return out, true
	}
	return nil, false
}

// listColumn reports whether a column holds lists, and their element type.
func (c *checker) listColumn(t typ, rows [][]ir.Pattern) (typ, bool) {
	if k, ok := t.(*tcon); ok && k.decl == nil {
		switch k.name {
		case "List":
			return k.args[0], true
		case "Bytes":
			return con("Byte"), true
		}
	}
	for _, r := range rows {
		if _, ok := r[0].(ir.PList); ok {
			return c.fresh(), true
		}
	}
	return nil, false
}

// specializeList returns the sub-patterns of p for a list of length n.
func specializeList(p ir.Pattern, n int) ([]ir.Pattern, bool) {
	wild := func(k int) []ir.Pattern {
		out := make([]ir.Pattern, k)
		for i := range out {
			out[i] = ir.PWild{}
		}
		return out
	}
	if wildLike(p) {
		return wild(n), true
	}
	l, ok := p.(ir.PList)
	if !ok {
		return nil, false
	}
	if l.Rest < 0 {
		if len(l.Pats) != n {
			return nil, false
		}
		return l.Pats, true
	}
	if len(l.Pats) > n {
		return nil, false
	}
	out := append([]ir.Pattern{}, l.Pats[:l.Rest]...)
	out = append(out, wild(n-len(l.Pats))...)
	return append(out, l.Pats[l.Rest:]...), true
}
