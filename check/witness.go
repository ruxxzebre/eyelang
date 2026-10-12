package check

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"

	"eyelang/internal/wire"
	"eyelang/ir"
)

// Fields no recording contains (SPECS.md §17, "Fakes come from
// recordings"). A program's answers from another service are known from the
// recordings its tests replay, `fakes(chat, from: "recordings/")`, not from
// what its author guesses the service sends. So a read of a field of a type
// a contract's module declares, which an operation's result holds, is an
// error when no recording of that contract has the field, unless the read is
// in a declaration marked @unwitnessed("why"), or the type is.
//
// A read is a field access, `r.receipt`, or a pattern that looks inside the
// field, `Refused(error)`; `Refused(_)` and `Refused(..)` read nothing, so a
// variant no recording has is matched without a mark, as exhaustiveness
// needs. A contract none of the program's tests replays recordings of is
// not checked: there is nothing to witness its answers. Nor is one the
// program serves, whose answers its own code makes.

// witnessKey is a field of a declared type: of the record (variant "") or
// of one of its variants, by index.
type witnessKey struct {
	decl    *ir.Type
	variant string
	field   int
}

// witnessed is what the recordings of one contract module show.
type witnessed struct {
	contract *ir.Contract
	files    []string            // the recordings read, for messages
	checked  map[*ir.Type]bool   // the module's types its operations answer with
	seen     map[witnessKey]bool // the fields some recording has
}

// checkWitnessed reports the reads of fields no recording contains.
func (c *checker) checkWitnessed() {
	byModule := c.recordings()
	if len(byModule) == 0 {
		return
	}
	// A type's fields belong to the contract module that declares it.
	byType := map[*ir.Type]*witnessed{}
	for _, w := range byModule {
		for d := range w.checked {
			byType[d] = w
		}
	}
	for _, m := range c.prog.Modules {
		if m.Dir == "" && m != c.prog.Root {
			continue // the standard library's
		}
		for _, f := range Bodies(m) {
			c.witnessReads(f, f.Body, byType)
		}
	}
}

// recordings reads the recordings the program's tests replay: each
// `fakes(m, …, from: "dir/")` whose directory is written as a string names
// dir/m.json, beside the file it is written in, for each contract module m.
func (c *checker) recordings() map[*ir.Module]*witnessed {
	out := map[*ir.Module]*witnessed{}
	read := map[string]bool{}
	for _, m := range c.prog.Modules {
		if m.Dir == "" && m != c.prog.Root {
			continue
		}
		for _, f := range Bodies(m) {
			walk(f.Body, false, func(e ir.Expr, _ bool) bool {
				call, ok := e.(*ir.Call)
				if !ok {
					return true
				}
				r, ok := call.Func.(*ir.Ref)
				if !ok || call.Pos.File == nil {
					return true
				}
				if b, isBuiltin := r.Decl.(ir.Builtin); !isBuiltin || b.Name != "fakes" {
					return true
				}
				dir := ""
				var mods []*ir.Module
				for _, a := range call.Args {
					switch a.Name {
					case "from":
						if lit, ok := a.Value.(*ir.Lit); ok {
							dir, _ = lit.Value.(string)
						}
					case "":
						if mr, ok := a.Value.(*ir.Ref); ok {
							if mod, ok := mr.Decl.(*ir.Module); ok {
								mods = append(mods, mod)
							}
						}
					}
				}
				if dir == "" {
					return true
				}
				for _, mod := range mods {
					if mod.Dir == "" || len(mod.Contracts) != 1 {
						continue // the standard library's, or no one contract to fake
					}
					path := filepath.Join(filepath.Dir(call.Pos.File.Path), dir, mod.Name+".json")
					if read[path] {
						continue
					}
					read[path] = true
					c.readRecording(out, mod, path)
				}
				return true
			})
		}
	}
	return out
}

// readRecording adds what the recording at path shows of mod's contract. A
// recording that can't be read, or doesn't fit the contract, is the test's
// to report when it runs.
func (c *checker) readRecording(out map[*ir.Module]*witnessed, mod *ir.Module, path string) {
	data, err := c.readFile(path)
	if err != nil {
		return
	}
	var rec struct {
		Contract  string `json:"contract"`
		Exchanges []struct {
			Op     string          `json:"op"`
			Result json.RawMessage `json:"result"`
		} `json:"exchanges"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if dec.Decode(&rec) != nil {
		return
	}
	var ct *ir.Contract
	for _, x := range mod.Contracts {
		ct = x
	}
	if rec.Contract != ct.Name {
		return
	}
	for _, op := range ct.Ops {
		if op.Impl != nil {
			return // the program serves it: its own code makes the answers
		}
	}
	w := out[mod]
	if w == nil {
		w = &witnessed{contract: ct, checked: map[*ir.Type]bool{}, seen: map[witnessKey]bool{}}
		for _, op := range ct.Ops {
			answers(op.Return, mod, w.checked)
		}
		out[mod] = w
	}
	w.files = append(w.files, path)
	for _, e := range rec.Exchanges {
		op, ok := ct.Ops[e.Op]
		if !ok || len(e.Result) == 0 {
			continue
		}
		var x any
		d := json.NewDecoder(bytes.NewReader(e.Result))
		d.UseNumber()
		if d.Decode(&x) != nil {
			continue
		}
		c.witness(x, op.Return, nil, w.seen)
	}
}

// answers adds the types mod declares that t holds: an operation's result,
// whose fields come from the service.
func answers(t *ir.TypeExpr, mod *ir.Module, into map[*ir.Type]bool) {
	if t == nil {
		return
	}
	for _, a := range t.Args {
		answers(a, mod, into)
	}
	answers(t.Result, mod, into)
	d := t.Decl
	if d == nil || into[d] {
		return
	}
	if d.Module == mod {
		into[d] = true
	} else {
		return // another module's type, as Option or Result: only its arguments
	}
	if d.Alias != nil {
		answers(d.Alias, mod, into)
	}
	for _, v := range d.Variants {
		for _, f := range v.Fields {
			answers(f.Type, mod, into)
		}
	}
}

// witness marks the fields of t that the recorded value x has. It follows
// the shape every value travels in (SPECS.md §17): a record is an object, a
// variant {"kind": "Sent", …}, or {"Sent": {…}} as recordings made before
// that shape was settled write it. env gives the type parameters' types.
func (c *checker) witness(x any, t *ir.TypeExpr, env map[string]*ir.TypeExpr, seen map[witnessKey]bool) {
	if t == nil {
		return
	}
	if t.Decl == nil && len(t.Args) == 0 {
		if r, ok := env[t.Name]; ok {
			t = r
		}
	}
	if d := t.Decl; d != nil {
		inner := map[string]*ir.TypeExpr{}
		for i, p := range d.Params {
			if i < len(t.Args) {
				a := t.Args[i]
				if a.Decl == nil && len(a.Args) == 0 && env[a.Name] != nil {
					a = env[a.Name]
				}
				inner[p] = a
			}
		}
		c.witnessDecl(x, d, inner, seen)
		return
	}
	arg := func(i int) *ir.TypeExpr {
		if i < len(t.Args) {
			return t.Args[i]
		}
		return nil
	}
	switch t.Name {
	case "List", "Set":
		xs, _ := x.([]any)
		for _, e := range xs {
			c.witness(e, arg(0), env, seen)
		}
	case "Map":
		switch m := x.(type) {
		case []any:
			for _, e := range m {
				switch p := e.(type) {
				case []any:
					if len(p) == 2 {
						c.witness(p[0], arg(0), env, seen)
						c.witness(p[1], arg(1), env, seen)
					}
				case map[string]any:
					c.witness(p["key"], arg(0), env, seen)
					c.witness(p["value"], arg(1), env, seen)
				}
			}
		case map[string]any:
			for _, v := range m {
				c.witness(v, arg(1), env, seen)
			}
		}
	case "tuple":
		switch xs := x.(type) {
		case []any:
			for i, e := range xs {
				c.witness(e, arg(i), env, seen)
			}
		case map[string]any:
			for i := range t.Args {
				c.witness(xs[fmt.Sprintf("value%d", i+1)], arg(i), env, seen)
			}
		}
	case "record":
		obj, _ := x.(map[string]any)
		for i, name := range t.Fields {
			if v, ok := obj[name]; ok {
				c.witness(v, arg(i), env, seen)
			}
		}
	}
}

func (c *checker) witnessDecl(x any, d *ir.Type, env map[string]*ir.TypeExpr, seen map[witnessKey]bool) {
	if d.Alias != nil && len(d.Variants) == 0 {
		if d.Alias.Name != "record" {
			c.witness(x, d.Alias, env, seen)
			return
		}
		obj, _ := x.(map[string]any)
		for i, name := range d.Alias.Fields {
			if v, ok := obj[name]; ok && i < len(d.Alias.Args) {
				seen[witnessKey{d, "", i}] = true
				c.witness(v, d.Alias.Args[i], env, seen)
			}
		}
		return
	}
	if len(d.Variants) == 0 || wire.IsEnum(d) {
		return
	}
	if x == nil {
		return // None, or no value
	}
	if d.Newtype() || d == c.prog.Prelude.Types["Option"] {
		// Its one field is the value itself: a newtype's, or Some's.
		for _, v := range d.Variants {
			if len(v.Fields) == 1 {
				seen[witnessKey{d, v.Name, 0}] = true
				c.witness(x, v.Fields[0].Type, env, seen)
			}
		}
		return
	}
	obj, ok := x.(map[string]any)
	if !ok {
		return
	}
	kind, _ := obj["kind"].(string)
	fields := obj
	if kind == "" && len(obj) == 1 {
		// {"Sent": {…}}, as older recordings write it.
		for k, v := range obj {
			kind = k
			fields, _ = v.(map[string]any)
			if fields == nil {
				fields = map[string]any{"value": v}
			}
		}
	}
	for _, v := range d.Variants {
		if v.Name != kind {
			continue
		}
		for i, f := range v.Fields {
			if val, ok := fields[wire.FieldName(v, i)]; ok {
				seen[witnessKey{d, v.Name, i}] = true
				c.witness(val, f.Type, env, seen)
			}
		}
	}
}

// witnessReads reports each read in body, which fn holds, of a field no
// recording contains.
func (c *checker) witnessReads(fn *ir.Func, body ir.Expr, byType map[*ir.Type]*witnessed) {
	walk(body, false, func(e ir.Expr, _ bool) bool {
		switch e := e.(type) {
		case *ir.Lambda:
			c.witnessReads(e.Func, e.Func.Body, byType)
			return false
		case *ir.Field:
			if t, ok := e.Value.Ty().(*ir.TCon); ok && t.Decl != nil && t.Decl.Alias != nil && t.Decl.Alias.Name == "record" {
				for i, name := range t.Decl.Alias.Fields {
					if name == e.Name {
						c.unwitnessed(byType, witnessKey{t.Decl, "", i}, e.Pos)
					}
				}
			}
		case *ir.Match:
			for _, a := range e.Arms {
				c.patternReads(fn, a.Pattern, e.Value.Ty(), e.Pos, byType)
			}
		case *ir.Let:
			c.patternReads(fn, e.Pattern, e.Value.Ty(), e.Pos, byType)
		case *ir.For:
			if t, ok := e.Iter.Ty().(*ir.TCon); ok && (t.Name == "List" || t.Name == "Set") && len(t.Args) == 1 {
				c.patternReads(fn, e.Pattern, t.Args[0], e.Pos, byType)
			}
		}
		return true
	})
}

// patternReads reports the fields pattern p looks inside, in a value of
// type t, that no recording contains. at is the match, let or for, for a
// pattern with no place of its own.
func (c *checker) patternReads(fn *ir.Func, p ir.Pattern, t ir.Ty, at ir.Pos, byType map[*ir.Type]*witnessed) {
	tc, _ := t.(*ir.TCon)
	switch p := p.(type) {
	case ir.POr:
		for _, alt := range p.Alts {
			c.patternReads(fn, alt, t, at, byType)
		}
	case ir.PTuple:
		tt, _ := t.(*ir.TTuple)
		for i, sub := range p.Pats {
			var et ir.Ty
			if tt != nil && i < len(tt.Elems) {
				et = tt.Elems[i]
			}
			c.patternReads(fn, sub, et, at, byType)
		}
	case ir.PList:
		var et ir.Ty
		if tc != nil && len(tc.Args) == 1 {
			et = tc.Args[0]
		}
		for _, sub := range p.Pats {
			c.patternReads(fn, sub, et, at, byType)
		}
	case ir.PVariant:
		v := p.Variant
		if v == nil || v.Type == nil {
			return
		}
		for j, sub := range p.Args {
			i := j
			if j < len(p.Names) && p.Names[j] != "" {
				i = -1
				for k, f := range v.Fields {
					if f.Name == p.Names[j] {
						i = k
					}
				}
			}
			if i < 0 || i >= len(v.Fields) {
				continue
			}
			if _, wild := sub.(ir.PWild); wild {
				continue
			}
			c.unwitnessed(byType, witnessKey{v.Type, v.Name, i}, c.patternAt(fn, sub, at))
			c.patternReads(fn, sub, fieldTy(tc, v.Type, v.Fields[i].Type), at, byType)
		}
	case ir.PRecord:
		if tc == nil || tc.Decl == nil || tc.Decl.Alias == nil || tc.Decl.Alias.Name != "record" {
			return
		}
		d := tc.Decl
		for j, name := range p.Names {
			for i, f := range d.Alias.Fields {
				if f != name || i >= len(d.Alias.Args) {
					continue
				}
				if _, wild := p.Pats[j].(ir.PWild); wild {
					continue
				}
				c.unwitnessed(byType, witnessKey{d, "", i}, c.patternAt(fn, p.Pats[j], at))
				c.patternReads(fn, p.Pats[j], fieldTy(tc, d, d.Alias.Args[i]), at, byType)
			}
		}
	}
}

// patternAt is where a pattern is written: a name where it is bound, a
// literal where it is, or else at.
func (c *checker) patternAt(fn *ir.Func, p ir.Pattern, at ir.Pos) ir.Pos {
	switch p := p.(type) {
	case ir.PBind:
		if pos, ok := c.slotPos[fn][p.Slot]; ok {
			return pos
		}
	case ir.PLit:
		if p.Pos.File != nil {
			return p.Pos
		}
	}
	return at
}

// fieldTy is the type of a field declared as te in d, in a value of type t.
func fieldTy(t *ir.TCon, d *ir.Type, te *ir.TypeExpr) ir.Ty {
	if te == nil {
		return nil
	}
	if te.Decl == nil && len(te.Args) == 0 && t != nil && t.Decl == d {
		for i, p := range d.Params {
			if p == te.Name && i < len(t.Args) {
				return t.Args[i]
			}
		}
	}
	out := &ir.TCon{Name: te.Name, Decl: te.Decl, Entity: te.Entity}
	for _, a := range te.Args {
		out.Args = append(out.Args, fieldTy(t, d, a))
	}
	return out
}

// unwitnessed reports a read, at at, of a field no recording contains.
func (c *checker) unwitnessed(byType map[*ir.Type]*witnessed, k witnessKey, at ir.Pos) {
	w := byType[k.decl]
	if w == nil || w.seen[k] || c.markedUnwitnessed(at) || c.markedUnwitnessed(k.decl.Pos) {
		return
	}
	field, of := "", k.decl.Name
	if k.variant == "" {
		field = k.decl.Alias.Fields[k.field]
	} else {
		for _, v := range k.decl.Variants {
			if v.Name == k.variant {
				of = v.Name
				field = v.Fields[k.field].Name
				if field == "" {
					field = fmt.Sprintf("value %d", k.field+1)
					if len(v.Fields) == 1 {
						field = "value"
					}
				}
			}
		}
	}
	files := append([]string{}, w.files...)
	sort.Strings(files)
	for i, f := range files {
		if rel, err := filepath.Rel(c.root, f); err == nil {
			files[i] = rel
		}
	}
	where := "no recording of " + w.contract.Name + " (" + files[0] + ")"
	if len(files) > 1 {
		where = fmt.Sprintf("none of the %d recordings of %s", len(files), w.contract.Name)
	}
	c.typeErrorf(at, "%s's %s is in %s, so nothing shows the service sends it: record an answer that has it, or mark where it is read @unwitnessed(\"why\")",
		of, fieldWord(field), where)
}

func fieldWord(name string) string {
	if name == "value" || len(name) > 6 && name[:6] == "value " {
		return name
	}
	return "field " + name
}

// markedUnwitnessed reports whether at is inside a declaration marked
// @unwitnessed("why").
func (c *checker) markedUnwitnessed(at ir.Pos) bool {
	for _, s := range c.unwitnessedAt {
		if s.File == at.File && at.Offset >= s.Offset && at.Offset <= s.End {
			return true
		}
	}
	return false
}
