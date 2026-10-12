package interp

import (
	"context"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strings"

	"eyelang/ir"
)

// A program's extern fns (SPECS.md §18): @host functions, which the Go
// program that embeds Eyelang registers (Hosts), and @library functions,
// which are C functions loaded at run time without cgo (UseCLoader). The
// standard library's own @host functions are in hostFuncs: they take and
// give any value, since they are part of the interpreter.

// ForeignVal is a value of a program's extern type: foreign code made it,
// and only extern functions take it. V is the Go value a host function
// returned, or the pointer a C function returned, as a uintptr.
type ForeignVal struct {
	Type *ir.Type
	V    any
}

// CLoader loads the C function symbol from a library as a Go function of
// type fn (see internal/ffi.Load).
type CLoader func(library, symbol string, fn reflect.Type) (reflect.Value, error)

var cLoader CLoader

// UseCLoader sets how @library extern fns are loaded. The interpreter has
// no loader of its own, so that a Go program that never calls C, such as
// the released eye on Linux, stays a static binary: eyelang/host/clib sets
// one when imported.
func UseCLoader(l CLoader) { cLoader = l }

// ---------------------------------------------------------------- host functions

// Hosts are the functions a Go program that embeds Eyelang provides for a
// program's @host extern fns, by name.
type Hosts struct {
	prog  *ir.Program
	funcs map[string]reflect.Value
}

// NewHosts starts an empty set of host functions for prog.
func NewHosts(prog *ir.Program) *Hosts {
	return &Hosts{prog: prog, funcs: map[string]reflect.Value{}}
}

// Register provides fn for the extern fns declared @host(name). fn is a Go
// function whose parameters and results match the declaration's:
//
//	Bool       bool            Float   float64
//	Byte       uint8           String  string
//	Int        int             Bytes   []byte (copied)
//	Int32      int32           UInt32  uint32
//	Int64      int64           UInt64  uint64
//
// An extern type is any Go pointer, struct, map, channel, function or
// interface type, and the value stays Go's. A result of Result(T, String)
// is (T, error), or error alone for Result((), String); a result of ()
// is none. fn may take a context.Context first, which is done when the
// task that made the call is cancelled. Register reports the first
// difference from the declaration, and a name no extern fn of the program
// is declared with.
func (h *Hosts) Register(name string, fn any) error {
	if _, ok := hostFuncs[name]; ok {
		return fmt.Errorf("@host(%q) is the interpreter's own: register a name of the program's", name)
	}
	if _, ok := h.funcs[name]; ok {
		return fmt.Errorf("@host(%q) is registered twice", name)
	}
	decls := hostDecls(h.prog, name)
	if len(decls) == 0 {
		return fmt.Errorf("no extern fn of the program is declared @host(%q)", name)
	}
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func || v.IsNil() {
		return fmt.Errorf("@host(%q) needs a Go function, and this is %T", name, fn)
	}
	for _, d := range decls {
		if err := checkHost(d, v.Type()); err != nil {
			return err
		}
	}
	h.funcs[name] = v
	return nil
}

// hostDecls are the extern fns of the program's own modules declared
// @host(name).
func hostDecls(prog *ir.Program, name string) []*ir.Func {
	var out []*ir.Func
	for _, f := range programExterns(prog) {
		if f.Host == name {
			out = append(out, f)
		}
	}
	return out
}

// programExterns are the extern fns of the program's own modules, in the
// order of their positions.
func programExterns(prog *ir.Program) []*ir.Func {
	var out []*ir.Func
	for _, m := range prog.Modules {
		if m != prog.Root && m.Dir == "" {
			continue // the prelude and the standard modules
		}
		for _, f := range m.Funcs {
			if f.Extern {
				out = append(out, f)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Pos, out[j].Pos
		if a.File != b.File && a.File != nil && b.File != nil {
			return a.File.Path < b.File.Path
		}
		return a.Offset < b.Offset
	})
	return out
}

var (
	contextType = reflect.TypeFor[context.Context]()
	errorType   = reflect.TypeFor[error]()
)

// crossing is how a value of one of an extern fn's types crosses to Go or
// C: its kind, a built-in type's name or "extern", "option" (an Option of
// an extern type, C's pointer that may be NULL) or "()", and its extern
// type.
type crossing struct {
	kind string
	ext  *ir.Type
}

// crossingOf is how a value of t crosses. The checker allowed only what
// can, so anything else is "".
func crossingOf(t *ir.TypeExpr) crossing {
	for t != nil && t.Decl != nil && t.Decl.Alias != nil && len(t.Decl.Params) == 0 {
		t = t.Decl.Alias
	}
	switch {
	case t == nil || t.Name == "()" && t.Decl == nil:
		return crossing{kind: "()"}
	case t.Decl != nil && t.Decl.Extern:
		return crossing{kind: "extern", ext: t.Decl}
	case t.Decl == nil && t.Entity == nil && len(t.Args) == 0:
		return crossing{kind: t.Name}
	case t.Name == "Option" && len(t.Args) == 1:
		if inner := crossingOf(t.Args[0]); inner.kind == "extern" {
			return crossing{kind: "option", ext: inner.ext}
		}
	}
	return crossing{}
}

// goKinds are the Go kinds that hold each built-in type in a host
// function.
var goKinds = map[string]reflect.Kind{
	"Bool": reflect.Bool, "Byte": reflect.Uint8, "Int": reflect.Int,
	"Int32": reflect.Int32, "Int64": reflect.Int64, "UInt32": reflect.Uint32, "UInt64": reflect.Uint64,
	"Float": reflect.Float64, "String": reflect.String, "Bytes": reflect.Slice,
}

// goName is how Go writes the type that holds c, for messages.
func goName(c crossing) string {
	switch c.kind {
	case "Bytes":
		return "[]byte"
	case "extern":
		return "a pointer, struct, map, channel, function or interface"
	}
	if k, ok := goKinds[c.kind]; ok {
		return k.String()
	}
	return "no Go type"
}

// holds reports whether the Go type t can hold a value that crosses as c.
func holds(c crossing, t reflect.Type) bool {
	switch c.kind {
	case "extern":
		switch t.Kind() {
		case reflect.Pointer, reflect.Struct, reflect.Map, reflect.Chan, reflect.Func, reflect.Interface, reflect.UnsafePointer:
			return true
		}
		return false
	case "Bytes":
		return t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8
	}
	k, ok := goKinds[c.kind]
	return ok && t.Kind() == k
}

// checkHost checks a Go function's type against the extern fn it is
// registered for.
func checkHost(decl *ir.Func, ft reflect.Type) error {
	where := fmt.Sprintf("@host(%q) is %s at %s", decl.Host, describeExtern(decl), decl.Pos)
	// First what no Go function could match, then this one's differences.
	for _, p := range decl.Params {
		if !hostable(crossingOf(p.Type)) {
			return fmt.Errorf("%s: parameter %s is %s, which no Go type stands for: a host function takes %s",
				where, p.Name, typeString(p.Type), hostTypes)
		}
	}
	want, ok := hostResults(decl)
	if !ok {
		return fmt.Errorf("%s, and no Go function can return %s: a host function returns %s, (), or Result(T, String) as (T, error)",
			where, typeString(decl.Return), hostTypes)
	}
	if ft.IsVariadic() {
		return fmt.Errorf("%s, and the Go function is variadic", where)
	}
	in := 0
	if ft.NumIn() > 0 && ft.In(0) == contextType {
		in = 1
	}
	if got := ft.NumIn() - in; got != len(decl.Params) {
		return fmt.Errorf("%s, which takes %s, and the Go function takes %d", where, count(len(decl.Params), "argument"), got)
	}
	for i, p := range decl.Params {
		c, t := crossingOf(p.Type), ft.In(in+i)
		if !holds(c, t) {
			return fmt.Errorf("%s: parameter %s is %s, which Go holds as %s, and the Go function's parameter %d is %s",
				where, p.Name, typeString(p.Type), goName(c), in+i+1, t)
		}
	}
	var got []string
	for i := range ft.NumOut() {
		got = append(got, ft.Out(i).String())
	}
	match := ft.NumOut() == len(want)
	for i := 0; match && i < len(want); i++ {
		if want[i].kind == "error" {
			match = ft.Out(i) == errorType
		} else {
			match = holds(want[i], ft.Out(i))
		}
	}
	if !match {
		var names []string
		for _, w := range want {
			if w.kind == "error" {
				names = append(names, "error")
			} else {
				names = append(names, goName(w))
			}
		}
		return fmt.Errorf("%s, which a Go function returns as (%s), and this one returns (%s)",
			where, strings.Join(names, ", "), strings.Join(got, ", "))
	}
	return nil
}

// hostResults are the Go results that give an extern fn's result: none for
// (), (T, error) for Result(T, String), and the type's own otherwise. An
// error is a crossing of kind "error".
func hostResults(decl *ir.Func) ([]crossing, bool) {
	r := decl.Return
	for r != nil && r.Decl != nil && r.Decl.Alias != nil && len(r.Decl.Params) == 0 {
		r = r.Decl.Alias
	}
	if r != nil && r.Name == "Result" && len(r.Args) == 2 {
		if crossingOf(r.Args[1]).kind != "String" {
			return nil, false
		}
		ok := crossingOf(r.Args[0])
		if ok.kind == "()" {
			return []crossing{{kind: "error"}}, true
		}
		return []crossing{ok, {kind: "error"}}, hostable(ok)
	}
	c := crossingOf(r)
	if c.kind == "()" {
		return nil, true
	}
	return []crossing{c}, hostable(c)
}

// hostTypes are the types a host function takes and returns, for messages.
const hostTypes = "Bool, Byte, Int, Int32, Int64, UInt32, UInt64, Float, String, Bytes and extern types"

// hostable reports whether a Go type holds values that cross as c.
func hostable(c crossing) bool {
	_, basic := goKinds[c.kind]
	return basic || c.kind == "extern"
}

// describeExtern writes an extern fn's signature, for messages.
func describeExtern(f *ir.Func) string {
	var ps []string
	for _, p := range f.Params {
		ps = append(ps, p.Name+": "+typeString(p.Type))
	}
	s := f.Name + "(" + strings.Join(ps, ", ") + ")"
	if f.Return != nil {
		s += " -> " + typeString(f.Return)
	}
	return s
}

func count(n int, what string) string {
	if n == 1 {
		return "1 " + what
	}
	return fmt.Sprintf("%d %ss", n, what)
}

// hostCall makes the hostFunc that calls a registered Go function for one
// extern fn, which checkHost accepted.
func hostCall(decl *ir.Func, fn reflect.Value) hostFunc {
	ft := fn.Type()
	withCtx := ft.NumIn() > 0 && ft.In(0) == contextType
	results, _ := hostResults(decl)
	return func(in *Interp, params map[string]Value, at ir.Pos) Value {
		var args []reflect.Value
		if withCtx {
			args = append(args, reflect.Value{}) // the context, set below
		}
		for _, p := range decl.Params {
			args = append(args, in.toGo(crossingOf(p.Type), params[p.Name], ft.In(len(args)), decl, p.Name, at))
		}
		var out []reflect.Value
		var failed any
		in.blocking(func(ctx context.Context) {
			defer func() { failed = recover() }()
			if withCtx {
				args[0] = reflect.ValueOf(&ctx).Elem()
			}
			out = fn.Call(args)
		})
		if failed != nil {
			in.faultf(at, "@host(%q) panicked: %v", decl.Host, failed)
		}
		var v Value = ir.Unit{}
		for i, r := range results {
			if r.kind == "error" {
				if err, _ := out[i].Interface().(error); err != nil {
					return in.errV(err.Error())
				}
				return in.ok(v)
			}
			v = in.fromGo(r, out[i], decl, at)
		}
		return v
	}
}

// ---------------------------------------------------------------- C functions

// cTypes are the Go types purego passes as each built-in type's C type.
var cTypes = map[string]reflect.Type{
	"Bool": reflect.TypeFor[bool](), "Byte": reflect.TypeFor[uint8](),
	"Int32": reflect.TypeFor[int32](), "Int64": reflect.TypeFor[int64](),
	"UInt32": reflect.TypeFor[uint32](), "UInt64": reflect.TypeFor[uint64](),
	"Float": reflect.TypeFor[float64](), "String": reflect.TypeFor[string](),
	"Bytes": reflect.TypeFor[[]byte](),
	// A pointer: a value of an extern type, or an Option of one.
	"extern": reflect.TypeFor[uintptr](), "option": reflect.TypeFor[uintptr](),
}

// cFunc loads the C function of a @library extern fn.
func cFunc(decl *ir.Func) (hostFunc, error) {
	if cLoader == nil {
		return nil, fmt.Errorf("%s is a C function, from @library(%q), and this build can't load C libraries: build eye with -tags clib, or import eyelang/host/clib in the Go program that embeds Eyelang", decl.Name, decl.Library)
	}
	var params []reflect.Type
	for _, p := range decl.Params {
		params = append(params, cTypes[crossingOf(p.Type).kind])
	}
	ret := crossingOf(decl.Return)
	var results []reflect.Type
	if ret.kind != "()" {
		results = append(results, cTypes[ret.kind])
	}
	fn, err := cLoader(decl.Library, decl.Name, reflect.FuncOf(params, results, false))
	if err != nil {
		return nil, err
	}
	return func(in *Interp, args map[string]Value, at ir.Pos) Value {
		vals := make([]reflect.Value, len(decl.Params))
		for i, p := range decl.Params {
			vals[i] = in.toC(crossingOf(p.Type), args[p.Name], decl, p.Name, at)
		}
		var out []reflect.Value
		var failed any
		in.blocking(func(context.Context) {
			defer func() { failed = recover() }()
			out = fn.Call(vals)
		})
		if failed != nil {
			in.faultf(at, "calling %s in %s failed: %v", decl.Name, decl.Library, failed)
		}
		if ret.kind == "()" {
			return ir.Unit{}
		}
		return in.fromC(ret, out[0], decl, at)
	}, nil
}

// ---------------------------------------------------------------- values

// fixedValue is the bits of an Int that a fixed-width or Int type holds.
func fixedValue(v Value) (uint64, bool) {
	switch x := v.(type) {
	case int64:
		return uint64(x), true
	case *big.Int:
		return x.Uint64(), x.IsUint64()
	}
	return 0, false
}

// toGo is v as a value of the Go type t, for a host function's parameter.
func (in *Interp) toGo(c crossing, v Value, t reflect.Type, decl *ir.Func, param string, at ir.Pos) reflect.Value {
	if c.kind == "extern" {
		f, ok := v.(*ForeignVal)
		if !ok {
			in.faultf(at, "%s's %s isn't a value foreign code made", decl.Name, param)
		}
		g := reflect.ValueOf(f.V)
		if _, isC := f.V.(uintptr); isC {
			in.faultf(at, "%s's %s is a %s that a C function made, and @host(%q) is a Go function", decl.Name, param, f.Type.Name, decl.Host)
		}
		if !g.IsValid() {
			return reflect.Zero(t)
		}
		if !g.Type().AssignableTo(t) {
			in.faultf(at, "@host(%q) takes %s as its %s, and this %s holds a %s, which another host function made", decl.Host, t, param, f.Type.Name, g.Type())
		}
		return g
	}
	out := reflect.New(t).Elem()
	switch c.kind {
	case "Bool":
		out.SetBool(v.(bool))
	case "Byte", "UInt32", "UInt64":
		b, _ := fixedValue(v)
		out.SetUint(b)
	case "Int32", "Int64":
		b, _ := fixedValue(v)
		out.SetInt(int64(b))
	case "Int":
		n, ok := v.(int64)
		if !ok || int64(int(n)) != n {
			in.faultf(at, "%s's %s is %s, which doesn't fit a Go int", decl.Name, param, show(v))
		}
		out.SetInt(n)
	case "Float":
		out.SetFloat(v.(float64))
	case "String":
		out.SetString(v.(string))
	case "Bytes":
		out.Set(reflect.ValueOf(in.byteSlice(v, at)).Convert(t))
	}
	return out
}

// fromGo is a host function's result as a value.
func (in *Interp) fromGo(c crossing, v reflect.Value, decl *ir.Func, at ir.Pos) Value {
	switch c.kind {
	case "extern":
		return &ForeignVal{Type: c.ext, V: v.Interface()}
	case "Bool":
		return v.Bool()
	case "Byte", "UInt32", "UInt64":
		return width{64, false}.value(v.Uint())
	case "Int32", "Int64", "Int":
		return v.Int()
	case "Float":
		return v.Float()
	case "String":
		return v.String()
	case "Bytes":
		return bytesValue(v.Bytes())
	}
	in.faultf(at, "%s's result can't come back from Go", decl.Name)
	return nil
}

// toC is v as the value purego passes to C.
func (in *Interp) toC(c crossing, v Value, decl *ir.Func, param string, at ir.Pos) reflect.Value {
	pointer := func(v Value) reflect.Value {
		f, ok := v.(*ForeignVal)
		if !ok {
			in.faultf(at, "%s's %s isn't a value foreign code made", decl.Name, param)
		}
		p, ok := f.V.(uintptr)
		if !ok {
			in.faultf(at, "%s's %s is a %s that a Go host function made, and %s is a C function", decl.Name, param, f.Type.Name, decl.Name)
		}
		return reflect.ValueOf(p)
	}
	switch c.kind {
	case "extern":
		return pointer(v)
	case "option":
		if s, ok := isVariant(v, "Some"); ok {
			return pointer(s.Vals[0])
		}
		return reflect.ValueOf(uintptr(0))
	case "Bytes":
		return reflect.ValueOf(in.byteSlice(v, at))
	}
	return in.toGo(c, v, cTypes[c.kind], decl, param, at)
}

// fromC is a C function's result as a value. A NULL pointer is None for an
// Option of an extern type, and a fault for an extern type itself.
func (in *Interp) fromC(c crossing, v reflect.Value, decl *ir.Func, at ir.Pos) Value {
	switch c.kind {
	case "extern", "option":
		p := uintptr(v.Uint())
		if p == 0 && c.kind == "option" {
			return in.none()
		}
		if p == 0 {
			in.faultf(at, "%s returned NULL, which isn't a %s: if it may, declare its result Option(%s)", decl.Name, c.ext.Name, c.ext.Name)
		}
		f := &ForeignVal{Type: c.ext, V: p}
		if c.kind == "option" {
			return in.some(f)
		}
		return f
	}
	return in.fromGo(c, v, decl, at)
}

// byteSlice copies a Bytes value, a list of Ints from 0 to 255, into Go.
func (in *Interp) byteSlice(v Value, at ir.Pos) []byte {
	es := in.elems(v, at)
	b := make([]byte, len(es))
	for i, e := range es {
		n, _ := e.(int64)
		b[i] = byte(n)
	}
	return b
}

func bytesValue(b []byte) Value {
	out := make([]Value, len(b))
	for i, x := range b {
		out[i] = int64(x)
	}
	return &List{Elems: out}
}

// ---------------------------------------------------------------- linking

// externFunc finds the code that runs an extern fn: a host function the
// embedding program registered, one of the interpreter's own, or a C
// function from its library, which is loaded on its first call.
func (in *Interp) externFunc(fn *ir.Func) (hostFunc, error) {
	if h, ok := in.externs[fn]; ok {
		return h, nil
	}
	var h hostFunc
	switch {
	case fn.Library != "":
		var err error
		if h, err = cFunc(fn); err != nil {
			return nil, err
		}
	case fn.Host != "":
		if g, ok := in.hosts.lookup(fn.Host); ok {
			h = hostCall(fn, g)
		} else if h, ok = hostFuncs[fn.Host]; !ok {
			return nil, fmt.Errorf("%s is @host(%q), and nothing registered a function of that name: eye provides only the standard library's, so run the program from the Go program that embeds it (package eyelang/host)", fn.Name, fn.Host)
		}
	default:
		return nil, fmt.Errorf("%s has no body, and neither @library nor @host says where its code is", fn.Name)
	}
	if in.externs == nil {
		in.externs = map[*ir.Func]hostFunc{}
	}
	in.externs[fn] = h
	return h, nil
}

func (h *Hosts) lookup(name string) (reflect.Value, bool) {
	if h == nil {
		return reflect.Value{}, false
	}
	v, ok := h.funcs[name]
	return v, ok
}

// linkExterns finds the code of every extern fn of the program's own
// modules before the program starts, so that a missing host function, or
// a C library or symbol that isn't there, is reported then rather than at
// the first call. It returns one message for each it couldn't find.
func (in *Interp) linkExterns() []string {
	var out []string
	for _, fn := range programExterns(in.prog) {
		if _, err := in.externFunc(fn); err != nil {
			out = append(out, fmt.Sprintf("%s: %v", fn.Pos, err))
		}
	}
	return out
}
