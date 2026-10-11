package ir

import (
	"fmt"
	"io"
	"math/big"
	"strings"
)

// Fprint writes a function and its body as the typed IR: one line per
// expression, indented under the expression it is part of, each with its
// type after a colon. `eye check --ir` prints it, and golden tests compare
// it. The form is for reading, and may change.
func Fprint(w io.Writer, f *Func) {
	p := &printer{w: w}
	name := f.Name
	if f.Receiver != nil {
		name = f.Receiver.Name + "." + name
	} else if f.Entity != nil {
		name = f.Entity.Name + "." + name
	}
	var params []string
	for _, prm := range f.Params {
		s := prm.Name
		if prm.Mut {
			s = "mut " + s
		}
		params = append(params, s)
	}
	head := fmt.Sprintf("fn %s(%s)", name, strings.Join(params, ", "))
	if len(f.Effects) > 0 {
		head += " uses {" + strings.Join(f.Effects, ", ") + "}"
	}
	if f.Waits {
		head += " waits"
	}
	p.line(0, head)
	for _, prm := range f.Params {
		if prm.Default != nil {
			p.line(1, "default "+prm.Name)
			p.expr(2, prm.Default)
		}
	}
	p.expr(1, f.Body)
}

type printer struct{ w io.Writer }

func (p *printer) line(depth int, s string) {
	fmt.Fprintf(p.w, "%s%s\n", strings.Repeat("  ", depth), s)
}

// node writes an expression's line: what it is, and its type.
func (p *printer) node(depth int, e Expr, what string) {
	p.line(depth, what+" : "+tyString(e.Ty()))
}

func (p *printer) args(depth int, as []Arg) {
	for _, a := range as {
		switch {
		case a.Mut:
			p.line(depth, "mut "+a.Name)
			p.expr(depth+1, a.Value)
		case a.Name != "":
			p.line(depth, a.Name+":")
			p.expr(depth+1, a.Value)
		default:
			p.expr(depth, a.Value)
		}
	}
}

func (p *printer) exprs(depth int, es []Expr) {
	for _, e := range es {
		p.expr(depth, e)
	}
}

func (p *printer) expr(depth int, e Expr) {
	if e == nil {
		return
	}
	d := depth + 1
	switch e := e.(type) {
	case *Lit:
		p.node(depth, e, "lit "+litString(e.Value))
	case *EmptyBraces:
		p.node(depth, e, "{}")
	case *Local:
		p.node(depth, e, fmt.Sprintf("local %s#%d", e.Name, e.Slot))
	case *Self:
		p.node(depth, e, "self")
	case *SelfField:
		p.node(depth, e, "self."+e.Name)
	case *Ref:
		p.node(depth, e, "ref "+declString(e.Decl))
	case *Call:
		p.node(depth, e, "call")
		p.expr(d, e.Func)
		p.args(d, e.Args)
	case *MethodCall:
		p.node(depth, e, "method ."+e.Name+" -> "+declString(e.Target))
		p.expr(d, e.Recv)
		p.args(d, e.Args)
	case *Waiting:
		p.node(depth, e, "!")
		p.expr(d, e.Call)
	case *Field:
		p.node(depth, e, "field ."+e.Name)
		p.expr(d, e.Value)
	case *Index:
		p.node(depth, e, "index")
		p.expr(d, e.Value)
		p.expr(d, e.Index)
	case *Unary:
		p.node(depth, e, "unary "+e.Op+implString(e.Impl))
		p.expr(d, e.X)
	case *Binary:
		p.node(depth, e, "binary "+e.Op+implString(e.Impl))
		p.expr(d, e.L)
		p.expr(d, e.R)
	case *Range:
		op := ".."
		if e.Inclusive {
			op = "..="
		}
		p.node(depth, e, "range "+op)
		p.expr(d, e.Start)
		p.expr(d, e.End)
	case *If:
		p.node(depth, e, "if")
		p.expr(d, e.Cond)
		p.expr(d, e.Then)
		if e.Else != nil {
			p.line(d, "else")
			p.expr(d+1, e.Else)
		}
	case *Match:
		p.node(depth, e, "match")
		p.expr(d, e.Value)
		for _, a := range e.Arms {
			p.line(d, "arm "+PatternString(a.Pattern))
			if a.Guard != nil {
				p.line(d+1, "if")
				p.expr(d+2, a.Guard)
			}
			p.expr(d+1, a.Body)
		}
	case *Block:
		p.node(depth, e, "block")
		p.exprs(d, e.Exprs)
	case *Let:
		p.node(depth, e, "let "+PatternString(e.Pattern))
		p.expr(d, e.Value)
		if e.Else != nil {
			p.line(d, "else")
			p.expr(d+1, e.Else)
		}
	case *Assign:
		p.node(depth, e, "assign "+placeString(e.Target)+" "+e.Op+implString(e.Impl))
		p.expr(d, e.Value)
	case *For:
		p.node(depth, e, "for "+PatternString(e.Pattern))
		p.expr(d, e.Iter)
		p.expr(d, e.Body)
	case *While:
		p.node(depth, e, "while")
		p.expr(d, e.Cond)
		p.expr(d, e.Body)
	case *Break:
		p.node(depth, e, "break")
	case *Continue:
		p.node(depth, e, "continue")
	case *Return:
		p.node(depth, e, "return")
		p.expr(d, e.Value)
	case *Lambda:
		var params []string
		for _, prm := range e.Func.Params {
			params = append(params, fmt.Sprintf("%s#%d", prm.Name, prm.Slot))
		}
		what := "lambda(" + strings.Join(params, ", ") + ")"
		if len(e.Func.Effects) > 0 {
			what += " uses {" + strings.Join(e.Func.Effects, ", ") + "}"
		}
		p.node(depth, e, what)
		for _, c := range e.Func.Captures {
			p.line(d, fmt.Sprintf("capture #%d", c.Slot))
			p.expr(d+1, c.From)
		}
		p.expr(d, e.Func.Body)
	case *Record:
		what := "record"
		if e.Type != nil {
			what += " " + e.Type.Name
		}
		p.node(depth, e, what)
		for i, n := range e.Names {
			p.line(d, n+":")
			p.expr(d+1, e.Values[i])
		}
	case *RecordUpdate:
		p.node(depth, e, "with")
		p.expr(d, e.Base)
		for i, n := range e.Names {
			p.line(d, n+":")
			p.expr(d+1, e.Values[i])
		}
	case *List:
		p.node(depth, e, "list")
		p.exprs(d, e.Elems)
	case *Map:
		p.node(depth, e, "map")
		for i := range e.Keys {
			p.line(d, "entry")
			p.expr(d+1, e.Keys[i])
			p.expr(d+1, e.Values[i])
		}
	case *Tuple:
		p.node(depth, e, "tuple")
		p.exprs(d, e.Elems)
	case *InZone:
		p.node(depth, e, "in zone "+litString(e.Instant))
		p.expr(d, e.Zone)
	case *Interpolate:
		p.node(depth, e, "interpolate")
		p.exprs(d, e.Parts)
	case *Try:
		p.node(depth, e, "?")
		p.expr(d, e.X)
	case *Default:
		p.node(depth, e, "default")
		p.expr(d, e.X)
		p.expr(d, e.Or)
	case *With:
		p.node(depth, e, "with handler")
		p.expr(d, e.Value)
	default:
		p.node(depth, e, fmt.Sprintf("%T", e))
	}
}

func implString(f *Func) string {
	if f == nil {
		return ""
	}
	return " -> " + funcString(f)
}

func funcString(f *Func) string {
	switch {
	case f.Receiver != nil:
		return f.Receiver.Name + "." + f.Name
	case f.Entity != nil:
		return f.Entity.Name + "." + f.Name
	}
	return f.Name
}

// declString names what a Ref or a method call's target is.
func declString(d any) string {
	switch d := d.(type) {
	case *Func:
		return "fn " + funcString(d)
	case *Global:
		return "global " + d.Name
	case *Variant:
		return "variant " + d.Name
	case *Entity:
		return "entity " + d.Name
	case *Job:
		return "job " + d.Name
	case *View:
		return "view " + d.Name
	case *Op:
		return "op " + d.Contract.Name + "." + d.Name
	case *Event:
		return "event " + d.Contract.Name + "." + d.Name
	case *Module:
		return "module " + d.Name
	case *Type:
		return "type " + d.Name
	case Builtin:
		return "builtin " + d.Name
	case RuntimeMethod:
		return "runtime " + d.Name
	case FieldCall:
		if f, ok := d.Call.Func.(*Field); ok {
			return "the function in field " + f.Name
		}
		return "a field's function"
	case nil:
		return "?"
	}
	return fmt.Sprintf("%T", d)
}

func litString(v any) string {
	switch v := v.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case rune:
		return fmt.Sprintf("%q", v)
	case *big.Int:
		return v.String()
	case Unit:
		return "()"
	}
	return fmt.Sprintf("%v", v)
}

func placeString(p Place) string {
	s := fmt.Sprintf("#%d", p.Slot)
	if p.Self {
		s = "self"
	}
	for _, f := range p.Path {
		s += "." + f
	}
	return s
}

// PatternString writes a pattern as a program would.
func PatternString(p Pattern) string {
	list := func(ps []Pattern) string {
		parts := make([]string, len(ps))
		for i, q := range ps {
			parts[i] = PatternString(q)
		}
		return strings.Join(parts, ", ")
	}
	switch p := p.(type) {
	case nil, PWild, *PWild:
		return "_"
	case PBind:
		return fmt.Sprintf("%s#%d", p.Name, p.Slot)
	case PLit:
		return litString(p.Value)
	case PVariant:
		if len(p.Args) == 0 && !p.Rest {
			return p.Variant.Name
		}
		parts := make([]string, len(p.Args))
		for i, a := range p.Args {
			parts[i] = PatternString(a)
			if i < len(p.Names) && p.Names[i] != "" {
				parts[i] = p.Names[i] + ": " + parts[i]
			}
		}
		if p.Rest {
			parts = append(parts, "..")
		}
		return p.Variant.Name + "(" + strings.Join(parts, ", ") + ")"
	case PRecord:
		parts := make([]string, len(p.Pats))
		for i, q := range p.Pats {
			parts[i] = p.Names[i] + ": " + PatternString(q)
		}
		if p.Rest {
			parts = append(parts, "..")
		}
		return "{ " + strings.Join(parts, ", ") + " }"
	case PTuple:
		return "(" + list(p.Pats) + ")"
	case PList:
		parts := make([]string, 0, len(p.Pats)+1)
		for i, q := range p.Pats {
			if i == p.Rest {
				parts = append(parts, restString(p.RestPat))
			}
			parts = append(parts, PatternString(q))
		}
		if p.Rest >= len(p.Pats) {
			parts = append(parts, restString(p.RestPat))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case PRange:
		op := ".."
		if p.Inclusive {
			op = "..="
		}
		return litString(p.Lo) + op + litString(p.Hi)
	case POr:
		parts := make([]string, len(p.Alts))
		for i, a := range p.Alts {
			parts[i] = PatternString(a)
		}
		return strings.Join(parts, " | ")
	case *PEmpty, PEmpty:
		return "{}"
	}
	return fmt.Sprintf("%T", p)
}

func restString(p Pattern) string {
	if p == nil {
		return ".."
	}
	return ".." + PatternString(p)
}
