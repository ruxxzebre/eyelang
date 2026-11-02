package interp

import (
	"bytes"
	"reflect"
	"strconv"
	"strings"

	"eyelang/ir"
)

// A compiled program leaves to the runner what touches the outside world or
// the store: `!` steps, contract operations, entity reads, starts, events,
// logging. It doesn't call each by a function of its own. It names the call
// in the program, a NativeSite, and gives the values of the receiver and the
// arguments. The binary carries the program's source, so the runner holds
// the same typed IR, finds the call there, and evaluates a copy of it with
// those values held in place of what computed them. So each operation has
// one implementation, with its steps, its replay, its fakes and its budget,
// whichever engine ran the code around it.

// NativeSite names a call in the program's typed IR.
type NativeSite struct {
	File        string // the file, relative to the program's root
	Offset, End int    // where the call's text is
	Kind        string // what it is: "call", "method" or "field"
	Durable     bool   `json:",omitempty"` // it is a `!` step
	// Held says what each value given replaces: "r" the receiver, a number
	// the argument at that place, or "=name" the parameter of that name.
	// Whatever isn't replaced, a decl such as the job in `write.start(day)`,
	// is evaluated as written.
	Held []string `json:",omitempty"`
}

type siteKey struct {
	file        string
	offset, end int
	kind        string
}

// site finds the expression a NativeSite names, indexing the program's
// calls the first time.
func (in *Interp) site(s *NativeSite, at ir.Pos) ir.Expr {
	if in.sites == nil {
		in.sites = map[siteKey]ir.Expr{}
		seen := map[uintptr]bool{}
		walkIR(reflect.ValueOf(in.prog), seen, func(e ir.Expr) {
			kind := ir.SiteKind(e)
			p := e.Position()
			if kind == "" || p.File == nil {
				return
			}
			k := siteKey{ir.SiteFile(in.prog, p.File), p.Offset, p.End, kind}
			if _, ok := in.sites[k]; !ok {
				in.sites[k] = e
			}
		})
	}
	e, ok := in.sites[siteKey{s.File, s.Offset, s.End, s.Kind}]
	if !ok {
		in.faultf(at, "the compiled program names a %s at %s:%d, which the program it carries doesn't have", s.Kind, s.File, s.Offset)
	}
	return e
}

var (
	exprType = reflect.TypeFor[ir.Expr]()
	posType  = reflect.TypeFor[ir.Pos]()
	fileType = reflect.TypeFor[*ir.File]()
)

// walkIR visits every expression reachable from v, which may hold cycles:
// a Ref leads to the declaration it names.
func walkIR(v reflect.Value, seen map[uintptr]bool, visit func(ir.Expr)) {
	if !v.IsValid() {
		return
	}
	t := v.Type()
	if t == posType || t == fileType {
		return
	}
	switch v.Kind() {
	case reflect.Interface:
		if !v.IsNil() {
			walkIR(v.Elem(), seen, visit)
		}
	case reflect.Pointer:
		if v.IsNil() || seen[v.Pointer()] {
			return
		}
		seen[v.Pointer()] = true
		if t.Implements(exprType) && v.CanInterface() {
			visit(v.Interface().(ir.Expr))
		}
		walkIR(v.Elem(), seen, visit)
	case reflect.Struct:
		for i := range v.NumField() {
			if t.Field(i).IsExported() {
				walkIR(v.Field(i), seen, visit)
			}
		}
	case reflect.Slice:
		if !v.IsNil() {
			for i := range v.Len() {
				walkIR(v.Index(i), seen, visit)
			}
		}
	case reflect.Array:
		for i := range v.Len() {
			walkIR(v.Index(i), seen, visit)
		}
	case reflect.Map:
		for it := v.MapRange(); it.Next(); {
			walkIR(it.Value(), seen, visit)
		}
	}
}

// evalSite evaluates the call a site names with the values given.
func (in *Interp) evalSite(s *NativeSite, args [][]byte, at ir.Pos) Value {
	if len(args) != len(s.Held) {
		in.faultf(at, "the compiled program gives %d values to a call that holds %d", len(args), len(s.Held))
	}
	held := make(map[string]ir.Expr, len(args))
	for i, a := range args {
		held[s.Held[i]] = &ir.Held{V: in.decode(a, at)}
	}
	e := in.heldCopy(in.site(s, at), held, at)
	if s.Durable {
		e = &ir.Waiting{At: ir.At{Pos: e.Position(), T: e.Ty()}, Call: e}
	}
	return in.eval(&frame{fn: siteFunc}, e)
}

// siteFunc is what the frame an evaluated site runs in belongs to: a copy of
// a call reads nothing from its frame.
var siteFunc = &ir.Func{Name: "(a call of compiled code)"}

// heldCopy copies a call with the values held in place of what computed
// them.
func (in *Interp) heldCopy(e ir.Expr, held map[string]ir.Expr, at ir.Pos) ir.Expr {
	switch c := e.(type) {
	case *ir.Call:
		cp := *c
		cp.Args = heldArgs(c.Args, held)
		return &cp
	case *ir.MethodCall:
		cp := *c
		if r, ok := held["r"]; ok {
			cp.Recv = r
		}
		cp.Args = heldArgs(c.Args, held)
		return &cp
	case *ir.Field:
		cp := *c
		if r, ok := held["r"]; ok {
			cp.Value = r
		}
		return &cp
	}
	in.faultf(at, "the runner can't evaluate a %T for compiled code", e)
	return nil
}

// heldArgs gives a call's arguments: all by parameter name when compiled
// code ordered them by its parameters, and otherwise each in its place.
func heldArgs(args []ir.Arg, held map[string]ir.Expr) []ir.Arg {
	byName := false
	for k := range held {
		if k != "" && k[0] == '=' {
			byName = true
		}
	}
	if byName {
		var out []ir.Arg
		for k, v := range held {
			if k[0] == '=' {
				out = append(out, ir.Arg{Name: k[1:], Value: v})
			}
		}
		return out
	}
	out := append([]ir.Arg(nil), args...)
	for i := range out {
		if v, ok := held[strconv.Itoa(i)]; ok {
			out[i].Value = v
		}
	}
	return out
}

// Now is the runner's clock.
func (h *nativeHost) Now() int64 { return h.in.rt.now.UnixNano() }

// Site evaluates a call compiled code asks for, inside the call it was
// given to.
func (h *nativeHost) Site(site *NativeSite, args [][]byte) ([]byte, error) {
	return h.guard(func() []byte {
		return h.in.encode(h.in.evalSite(site, args, h.at), h.at)
	})
}

// Show writes a value only the runner reads, as the program's Show does.
func (h *nativeHost) Show(value []byte) (string, error) {
	out, err := h.guard(func() []byte {
		return []byte(h.in.display(h.in.decode(value, h.at)))
	})
	return string(out), err
}

// NativeFault is why compiled code stopped: where, as it was built, and why.
type NativeFault struct{ At, Msg string }

func (f *NativeFault) Error() string { return f.At + ": " + f.Msg }

// nativePos finds the place compiled code names, written as file:line:col
// where it was built, in the program this runner carries: at when it isn't
// there. The file's path differs, so it is matched by what the two end in.
func (in *Interp) nativePos(place string, at ir.Pos) ir.Pos {
	parts := strings.Split(place, ":")
	if len(parts) < 3 {
		return at
	}
	path := strings.Join(parts[:len(parts)-2], ":")
	line, err1 := strconv.Atoi(parts[len(parts)-2])
	col, err2 := strconv.Atoi(parts[len(parts)-1])
	if err1 != nil || err2 != nil {
		return at
	}
	for _, m := range in.prog.Modules {
		for _, f := range m.Files {
			rel := ir.SiteFile(in.prog, f)
			if rel != path && !strings.HasSuffix(path, "/"+rel) && !strings.HasSuffix(f.Path, "/"+path) {
				continue
			}
			offset := 0
			for l := 1; l < line && offset < len(f.Src); l++ {
				i := bytes.IndexByte(f.Src[offset:], '\n')
				if i < 0 {
					return at
				}
				offset += i + 1
			}
			return ir.Pos{File: f, Offset: offset + col - 1}
		}
	}
	return at
}
