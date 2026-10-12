package check

import "eyelang/ir"

// TypeDefinitions returns the declarations of the type of what is at offset
// in the file at path, for an editor's "go to type definition": a local
// variable's, a parameter's or a field's type, the type of a call's result
// when the offset is on the function's name, the type of a record literal.
// A type made of others, such as List(Article), Option(Article) or
// Map(UserId, Article), gives the innermost declared types in it, each
// once: Article, or UserId and Article. One that holds no declared type,
// such as List(Int), gives the built-in, as BuiltinType. A Ref or an entity
// gives the entity's declaration.
func (ix *Index) TypeDefinitions(path string, offset int) []any {
	around := ix.around(path, offset)
	if u, ok := ix.At(path, offset); ok {
		if around != nil && around.at.End == u.At.End {
			return typeDecls(around.t)
		}
		switch t := u.Target.(type) {
		case Member:
			// A field where it is declared, or written in a record
			// literal, has no expression of its own: its declared type.
			if te := memberType(t); te != nil {
				return typeExprDecls(te)
			}
		case Local:
			// Where it is bound has no expression of its own, but each use
			// of it has its type.
			for _, v := range ix.Uses {
				if v.Target != any(t) {
					continue
				}
				if e := ix.exact(v.At); e != nil {
					return typeDecls(e.t)
				}
			}
			// A parameter nothing uses: the type it is declared with.
			if p := ix.param(t); p != nil && p.Type != nil {
				return typeExprDecls(p.Type)
			}
		}
	}
	if around != nil {
		return typeDecls(around.t)
	}
	return nil
}

// param is the parameter a local is, when it is one: of the function or job
// whose declaration holds where it is bound.
func (ix *Index) param(l Local) *ir.Param {
	for _, u := range ix.Uses {
		var f *ir.Func
		switch d := u.Target.(type) {
		case *ir.Func:
			f = d
		case *ir.Job:
			f = d.Func
		}
		if f == nil || f.Pos.File != l.At.File || l.At.Offset < f.Pos.Offset || l.At.End > f.Pos.End {
			continue
		}
		for _, p := range f.Params {
			if p.Name == l.At.Text() {
				return p
			}
		}
	}
	return nil
}

// around is the smallest checked expression in the file at path whose span
// holds offset.
func (ix *Index) around(path string, offset int) *typed {
	var best *typed
	for i := range ix.types {
		e := &ix.types[i]
		if e.at.File == nil || e.at.File.Path != path || offset < e.at.Offset || offset > e.at.End {
			continue
		}
		if best == nil || e.at.End-e.at.Offset < best.at.End-best.at.Offset {
			best = e
		}
	}
	return best
}

// exact is the checked expression whose span is at's.
func (ix *Index) exact(at ir.Pos) *typed {
	for i := range ix.types {
		e := &ix.types[i]
		if e.at.File == at.File && e.at.Offset == at.Offset && e.at.End == at.End {
			return e
		}
	}
	return nil
}

// memberType is the type a field is declared with.
func memberType(m Member) *ir.TypeExpr {
	switch o := m.Of.(type) {
	case *ir.Type:
		if a := o.Alias; a != nil {
			for i, f := range a.Fields {
				if f == m.Name && i < len(a.Args) {
					return a.Args[i]
				}
			}
		}
	case *ir.Variant:
		for _, f := range o.Fields {
			if f.Name == m.Name {
				return f.Type
			}
		}
	case *ir.Entity:
		if o.ID != nil && o.ID.Name == m.Name {
			return o.ID.Type
		}
		for _, s := range o.States {
			if s.Name == m.Name {
				return s.Type
			}
		}
	case *ir.Op:
		for _, p := range o.Params {
			if p.Name == m.Name {
				return p.Type
			}
		}
	}
	return nil
}

// typeDecls is typeDefinitions' answer for a checked type: the innermost
// declared types in it, or else the innermost built-in ones.
func typeDecls(t typ) []any {
	if out := innermost(t, true); len(out) > 0 {
		return out
	}
	return innermost(t, false)
}

// innermost lists the innermost types t is made of: only declared ones, or
// built-in ones too. A function's type is its result's.
func innermost(t typ, declaredOnly bool) []any {
	var out []any
	switch x := prune(defaultLiteral(t)).(type) {
	case *tcon:
		if x.entity != nil {
			return []any{x.entity}
		}
		for _, a := range x.args {
			out = appendNew(out, innermost(a, declaredOnly)...)
		}
		if len(out) > 0 {
			return out
		}
		if d, ok := typeDecl(x); ok {
			if _, builtin := d.(BuiltinType); !builtin || !declaredOnly {
				return []any{d}
			}
		}
	case *ttuple:
		for _, e := range x.elems {
			out = appendNew(out, innermost(e, declaredOnly)...)
		}
	case *tfun:
		if x.ret != nil {
			return innermost(x.ret, declaredOnly)
		}
	case *tentity:
		return []any{x.e}
	case *tdyn:
		if x.trait != nil {
			return []any{x.trait}
		}
	}
	return out
}

// typeExprDecls is typeDecls for a type as written in a declaration.
func typeExprDecls(t *ir.TypeExpr) []any {
	if out := innermostExpr(t, true); len(out) > 0 {
		return out
	}
	return innermostExpr(t, false)
}

func innermostExpr(t *ir.TypeExpr, declaredOnly bool) []any {
	if t == nil {
		return nil
	}
	if t.Entity != nil {
		return []any{t.Entity}
	}
	if t.Name == "fn" {
		return innermostExpr(t.Result, declaredOnly)
	}
	var out []any
	for _, a := range t.Args {
		out = appendNew(out, innermostExpr(a, declaredOnly)...)
	}
	switch {
	case len(out) > 0 || t.Name == "tuple" || t.Name == "record":
		return out
	case t.Decl != nil:
		return []any{t.Decl}
	case t.Trait != nil:
		return []any{t.Trait}
	case builtinTypes[t.Name] && !declaredOnly:
		return []any{BuiltinType{Name: t.Name}}
	}
	return nil
}

// appendNew appends each of more that out doesn't hold yet.
func appendNew(out []any, more ...any) []any {
	for _, m := range more {
		seen := false
		for _, o := range out {
			if o == m {
				seen = true
				break
			}
		}
		if !seen {
			out = append(out, m)
		}
	}
	return out
}
