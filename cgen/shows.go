package cgen

import (
	"fmt"

	"eyelang/ir"
	"eyelang/lir"
)

// A type's own impl of Show inside other values (SPECS.md §11): the
// derived Show, the runtime's ey_show_to, writes a value whose descriptor
// has a show entry through it. The low IR names the impls a derived Show
// may need (lir.Program.Shows); walk reaches those that the program's
// derived Shows and expect's messages do, and their types' descriptors get
// a wrapper that calls the impl.

// addShows reaches the impls of Show of the types inside a value of type
// t that is written in its derived form.
func (g *gen) addShows(t ir.Ty, add func(*lir.Func)) {
	if len(g.low.Shows) == 0 {
		return
	}
	lir.WalkShown(t, func(u ir.Ty) bool {
		k := lir.TypeKey(u)
		fn := g.low.Shows[k]
		if fn == nil {
			return true
		}
		if g.shown == nil {
			g.shown = map[string]*lir.Func{}
		}
		g.shown[k] = fn
		add(fn)
		return false
	})
}

// newtypeShowDesc is the descriptor of a newtype with an impl of Show that
// a derived Show calls: its field's, with the impl.
func (g *gen) newtypeShowDesc(t ir.Ty) (string, bool) {
	r := lir.Resolve(t)
	c, ok := r.(*ir.TCon)
	if !ok || c.Decl == nil || !c.Decl.Newtype() {
		return "", false
	}
	fn := g.shown[lir.TypeKey(r)]
	if fn == nil {
		return "", false
	}
	key := "show " + lir.TypeKey(r)
	if d, ok := g.descs[key]; ok {
		return "&" + d.name, true
	}
	d := &desc{name: fmt.Sprintf("ey_d%d", len(g.descs)), ty: rep(r), show: fn}
	g.descs[key] = d
	g.descQueue = append(g.descQueue, d)
	return "&" + d.name, true
}

// showFn is the impl of Show a descriptor's show entry calls, or nil.
func (g *gen) showFn(d *desc) *lir.Func {
	if d.show != nil {
		return d.show
	}
	if c, ok := d.ty.(*ir.TCon); ok && c.Decl != nil {
		return g.shown[lir.TypeKey(d.ty)]
	}
	return nil
}

// showOf is the name of a descriptor's show entry, or "".
func (g *gen) showOf(d *desc) string {
	if g.showFn(d) == nil {
		return ""
	}
	return d.name + "_show"
}

// showWrapper writes a descriptor's show entry: it calls the impl with the
// value it is lent.
func (g *gen) showWrapper(d *desc, name string) string {
	fn := g.showFn(d)
	ct := g.ctype(d.ty)
	dup := ""
	if len(fn.Modes) > 0 && fn.Modes[0] == lir.Owned {
		dup = g.dupStmt(d.ty, "v")
	}
	args := "v"
	if hasDefaults(fn) {
		args += ", 0"
	}
	return fmt.Sprintf("static ey_v %s(const void *p) { %s v = *(const %s *)p; %s return %s(%s); }\n", name, ct, ct, dup, g.names[fn], args)
}

// scalarKinds are the kinds of the built-in types a newtype may be.
var scalarKinds = map[string]string{
	"()": "EY_UNIT", "Bool": "EY_BOOL", "Float": "EY_FLOAT", "Char": "EY_CHAR", "Byte": "EY_BYTE",
	"Int32": "EY_INT32", "Int64": "EY_INT64", "UInt32": "EY_UINT32", "UInt64": "EY_UINT64",
	"Duration": "EY_DURATION", "Size": "EY_SIZE", "Date": "EY_DATE", "Time": "EY_TIME", "Never": "EY_NEVER",
	"Int": "EY_INT", "String": "EY_STRING", "Zone": "EY_STRING", "Key": "EY_STRING", "Bytes": "EY_BYTES",
	"Instant": "EY_INSTANT", "Iter": "EY_ITER",
}

// resultOf is what a function gives, or nil for no function.
func resultOf(f *lir.Func) ir.Ty {
	if f == nil {
		return nil
	}
	return f.Result
}
