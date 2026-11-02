package cgen

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"

	"eyelang/ir"
	"eyelang/lir"
)

// Compiled jobs and entity methods, for the Go runner to host (ROADMAP 7.6;
// docs/internals/compilation.md, "Compiled jobs").
//
// What compiled code doesn't do itself, it asks the runner to do by naming a
// site, a call in the typed IR the binary also carries, with the values of
// its receiver and arguments: every `!` step, and every call of the runtime
// (an entity's read, a job's start, a contract's operation, logging). The
// runner evaluates a copy of that call with those values held in place. So
// one implementation of each operation serves both engines.
//
// A function with a step in it stops there: it saves the variables live at
// that place as bytes, drops what it owns and returns, and each function
// that called it does the same up to the job. A waiting run is those bytes,
// with no C stack. To resume, the job is called again: each function reads
// its bytes and jumps to where it stopped.

// hostJob is a job the program hosts; hostEntity an entity, with its methods.
type hostJob struct {
	name string
	fn   *lir.Func
}

type hostEntity struct {
	name    string
	ty      ir.Ty
	methods []hostMethod
}

type hostMethod struct {
	name string
	fn   *lir.Func
}

// siteEntry is a call the runner evaluates, as eye_host.h lists it.
type siteEntry struct {
	file        string
	offset, end int
	kind        string
	durable     bool
	held        []string
}

// nativeName names a declaration as interp.Native does.
func nativeName(m *ir.Module, name string) string {
	if m == nil || m.Path == "" {
		return name
	}
	return m.Path + "." + name
}

// hostedRoots finds the program's own jobs and entity methods. The rest of
// the program, main and the tests among it, stays the interpreter's.
func (g *gen) hostedRoots() {
	bySrc := map[*ir.Func]*lir.Func{}
	for _, f := range g.low.Funcs {
		if f.Kind == lir.KindJob || f.Kind == lir.KindEntity {
			bySrc[f.Src] = f
		}
	}
	for _, m := range g.prog.Modules {
		if !g.own(m) {
			continue
		}
		var jobs, ents []string
		for n := range m.Jobs {
			jobs = append(jobs, n)
		}
		for n := range m.Entities {
			ents = append(ents, n)
		}
		sort.Strings(jobs)
		sort.Strings(ents)
		for _, n := range jobs {
			j := m.Jobs[n]
			if f := bySrc[j.Func]; f != nil {
				g.hostJobs = append(g.hostJobs, hostJob{nativeName(m, j.Name), f})
			}
		}
		for _, n := range ents {
			ent := m.Entities[n]
			he := hostEntity{name: nativeName(m, ent.Name), ty: &ir.TCon{Name: ent.Name, Entity: ent}}
			var names []string
			for mn := range ent.Methods {
				names = append(names, mn)
			}
			sort.Strings(names)
			for _, mn := range names {
				if f := bySrc[ent.Methods[mn]]; f != nil {
					he.methods = append(he.methods, hostMethod{mn, f})
				}
			}
			g.hostEntities = append(g.hostEntities, he)
		}
	}
}

// callsCompiled reports whether a call in a hosted program runs the callee's
// compiled code. A `!` call of a function that doesn't wait is a step of its
// own, whose result is recorded, and the runner evaluates it.
func (g *gen) callsCompiled(x *lir.Call) bool {
	if !x.Durable || lir.Stops(x.Fn) {
		return true
	}
	return x.Fn.Src != nil && x.Fn.Src.Waits
}

// hostedBuiltin reports whether the runner answers a built-in in a hosted
// program, since it touches what a test fakes or the run must record: the
// log, the environment, randomness.
func hostedBuiltin(name string) bool {
	switch name {
	case "print", "env.get", "now":
		return true
	}
	return strings.HasPrefix(name, "log.") || strings.HasPrefix(name, "random.")
}

// staticOps make values that are only names: nothing crosses for them, and
// a site evaluates what the program wrote.
var staticOps = map[string]bool{
	"module.value": true, "job.value": true, "op.value": true, "event.value": true,
	"entity.type": true, "type.value": true,
}

// opaqueType reports whether compiled code keeps t's values as their
// encoding, for the runner to read: a Ref, a Run, a function the runner runs.
func opaqueType(t ir.Ty) bool {
	switch x := lir.Resolve(t).(type) {
	case *ir.TFun:
		return true
	case *ir.TCon:
		// A command's result may also be Uncertain(key), Stale or Expired,
		// which the runner's value says: lir.Layouts keeps it an object.
		return x.Uncertain || x.Fresh || x.Decl == nil && x.Entity == nil && (x.Name == "Ref" || x.Name == "Run" || x.Name == "Approval")
	}
	return false
}

// opaqueData reports whether values of t are kept as their encoding and
// aren't closures, so that they share a descriptor: a string's.
func (g *gen) opaqueData(t ir.Ty) bool {
	if !g.opts.Hosted {
		return false
	}
	_, isCon := lir.Resolve(t).(*ir.TCon)
	return isCon && opaqueType(t)
}

// opaqueVariant writes a test of an opaque value's variant.
func (e *fn) opaqueVariant(x *lir.IsVariant) {
	name := variantName(x.Variant)
	e.assign(x.Dst, fmt.Sprintf("eyh_is_variant(%s, %s, %d)", e.v(x.Src), cstring(name), len(name)))
}

// opaqueField reads a field of an opaque value, a record the runner gave. It
// is decoded, as opaquePayload's is.
func (e *fn) opaqueField(x *lir.Field) {
	if e.g.layout(x.Dst.Ty).Repr == lir.ReprNone {
		return
	}
	if e.shadows[x.Dst] {
		if st := e.g.dropStmt(x.Dst.Ty, fmt.Sprintf("sh%d", x.Dst.ID)); st != "" {
			e.printf("%s", st)
		}
	}
	e.printf("eyh_field(%s, %s, %d, %s, &%s);", e.v(x.Src), cstring(x.Name), len(x.Name), e.g.saver(x.Dst.Ty).dec(), e.v(x.Dst))
	if e.shadows[x.Dst] {
		e.printf("sh%d = %s;", x.Dst.ID, e.v(x.Dst))
	}
}

// opaqueMove moves a value between a variable that keeps it as its encoding
// and one that holds it laid out: a command's result, once it is known not
// to be Uncertain. The Move takes the source, unless it only borrows.
func (e *fn) opaqueMove(x *lir.Move) {
	src := e.g.saver(x.Src.Ty)
	dst := e.g.saver(x.Dst.Ty)
	e.printf("{ ey_buf c_ = {0}; %s(&c_, &%s); ey_rd r_ = {(const uint8_t *)c_.b, (const uint8_t *)c_.b + c_.len, NULL};", src.enc(), e.v(x.Src))
	e.printf("if (!%s(&r_, &%s)) ey_fault(%s, r_.err ? r_.err : \"corrupt saved value\"); free(c_.b); }", dst.dec(), e.v(x.Dst), e.at)
	if x.Borrow {
		if e.shadows[x.Dst] {
			if st := e.g.dropStmt(x.Dst.Ty, fmt.Sprintf("sh%d", x.Dst.ID)); st != "" {
				e.printf("%s", st)
			}
			e.printf("sh%d = %s;", x.Dst.ID, e.v(x.Dst))
		}
		return
	}
	if st := e.g.dropStmt(x.Src.Ty, e.v(x.Src)); st != "" {
		e.printf("%s", st)
	}
}

// opaquePayload reads a field of an opaque value's variant. The field is
// decoded, so this function owns it, while the code takes it as borrowed from
// the value and doesn't drop it: its shadow does.
func (e *fn) opaquePayload(x *lir.Payload) {
	if e.g.layout(x.Dst.Ty).Repr == lir.ReprNone {
		return
	}
	if e.shadows[x.Dst] {
		if st := e.g.dropStmt(x.Dst.Ty, fmt.Sprintf("sh%d", x.Dst.ID)); st != "" {
			e.printf("%s", st)
		}
	}
	e.printf("eyh_payload(%s, %d, %s, &%s);", e.v(x.Src), x.Index, e.g.saver(x.Dst.Ty).dec(), e.v(x.Dst))
	if e.shadows[x.Dst] {
		e.printf("sh%d = %s;", x.Dst.ID, e.v(x.Dst))
	}
}

// opaqueClosures are the closures in f that only a site or a saved frame
// reads: the runner runs them, so they are kept as their encoding, and their
// code isn't compiled unless something else needs it.
func (g *gen) opaqueClosures(f *lir.Func) map[*lir.Value]bool {
	if !g.opts.Hosted {
		return nil
	}
	if m, ok := g.opaque[f]; ok {
		return m
	}
	if g.opaque == nil {
		g.opaque = map[*lir.Func]map[*lir.Value]bool{}
	}
	m := map[*lir.Value]bool{}
	for _, b := range f.Blocks {
		for _, in := range b.Instrs {
			if c, ok := in.(*lir.MakeClosure); ok {
				m[c.Dst] = true
			}
		}
	}
	for _, b := range f.Blocks {
		for _, in := range b.Instrs {
			for _, r := range lir.Reads(in) {
				if !m[r] {
					continue
				}
				switch x := in.(type) {
				case *lir.Dup, *lir.Drop:
				case *lir.Runtime:
					if x.Site == nil {
						delete(m, r)
					}
				case *lir.Builtin:
					if !x.Durable && !hostedBuiltin(x.Name) {
						delete(m, r)
					}
				default:
					delete(m, r)
				}
			}
		}
	}
	for _, b := range f.Blocks {
		if r, ok := b.Term.(*lir.Return); ok && r.Value != nil && m[r.Value] {
			delete(m, r.Value)
		}
	}
	g.opaque[f] = m
	return m
}

// prepareHosted works out what a function of a hosted program needs: its
// stops, its static values and its opaque closures. It writes the jump to
// where it stopped when it is resuming.
func (e *fn) prepareHosted() {
	f := e.f
	e.static = map[*lir.Value]bool{}
	for _, b := range f.Blocks {
		for _, in := range b.Instrs {
			if r, ok := in.(*lir.Runtime); ok && staticOps[r.Op] {
				e.static[r.Dst] = true
			}
		}
	}
	e.opaqueCl = e.g.opaqueClosures(f)
	e.shadows = map[*lir.Value]bool{}
	for _, b := range f.Blocks {
		for _, in := range b.Instrs {
			switch p := in.(type) {
			case *lir.Payload:
				if e.g.opaqueData(p.Src.Ty) && e.g.counted(p.Dst.Ty) {
					e.shadows[p.Dst] = true
				}
			case *lir.Field:
				if e.g.opaqueData(p.Src.Ty) && e.g.counted(p.Dst.Ty) {
					e.shadows[p.Dst] = true
				}
			case *lir.Move:
				if p.Borrow && e.g.opaqueData(p.Src.Ty) != e.g.opaqueData(p.Dst.Ty) && e.g.counted(p.Dst.Ty) {
					e.shadows[p.Dst] = true
				}
			}
		}
	}
	if lir.Stops(f) {
		e.stops = newStopInfo(e)
	}
	for _, v := range sortedValues(e.shadows) {
		fmt.Fprintf(&e.b, "\t%s sh%d = {0};\n", e.g.ctype(v.Ty), v.ID)
	}
	if e.stops != nil {
		e.stops.writeStart(e)
	}
}

func sortedValues(m map[*lir.Value]bool) []*lir.Value {
	var vs []*lir.Value
	for v := range m {
		vs = append(vs, v)
	}
	sort.Slice(vs, func(i, j int) bool { return vs[i].ID < vs[j].ID })
	return vs
}

// ---------------------------------------------------------------- stops

// stopInfo is what a function that stops knows about the places it stops.
type stopInfo struct {
	code    uint64 // a hash of the function's code, which its frames carry
	places  []*stopPlace
	byInstr map[lir.Instr]*stopPlace
	mutable map[*lir.Value]bool
}

type stopPlace struct {
	n    int // from 1
	live []*lir.Value
}

func newStopInfo(e *fn) *stopInfo {
	f := e.f
	s := &stopInfo{byInstr: map[lir.Instr]*stopPlace{}, mutable: map[*lir.Value]bool{}}
	var code strings.Builder
	lir.FprintFunc(&code, f)
	h := fnv.New64a()
	h.Write([]byte(code.String()))
	s.code = h.Sum64()
	mut := mutParams(f)
	for i, p := range f.Params {
		if i < len(mut) && mut[i] {
			s.mutable[p] = true
		}
	}
	live := lir.LiveAcross(f)
	for _, b := range f.Blocks {
		for _, in := range b.Instrs {
			if !lir.Durable(in) {
				continue
			}
			if c, ok := in.(*lir.Call); ok && !lir.Stops(c.Fn) && e.g.callsCompiled(c) {
				continue
			}
			p := &stopPlace{n: len(s.places) + 1}
			for _, v := range live[in] {
				if e.static[v] || s.mutable[v] || e.g.layout(v.Ty).Repr == lir.ReprNone {
					continue
				}
				if _, isFn := lir.Resolve(v.Ty).(*ir.TFun); isFn && !e.opaqueCl[v] {
					e.errorf("a function value is live across a step here, and the C back end can't save it yet: `eye run` runs this program")
					continue
				}
				p.live = append(p.live, v)
				if e.g.counted(v.Ty) && f.NotOwned[v] {
					e.shadows[v] = true
				}
			}
			s.places = append(s.places, p)
			s.byInstr[in] = p
		}
	}
	return s
}

// writeStart declares what resuming needs, and jumps to where the function
// stopped. A value read back that the code doesn't drop has a shadow, which
// the function drops as it leaves.
func (s *stopInfo) writeStart(e *fn) {
	e.b.WriteString("\tey_rd rd_ = {0};\n\t(void)rd_;\n")
	fmt.Fprintf(&e.b, "\tif (eyj_cur->resuming) switch (eyj_next(&rd_, UINT64_C(%d))) {\n", s.code)
	for _, p := range s.places {
		fmt.Fprintf(&e.b, "\tcase %d: goto rs%d;\n", p.n, p.n)
	}
	fmt.Fprintf(&e.b, "\tdefault: ey_fault(%s, \"a run's saved frame names a place this code doesn't have\");\n\t}\n", cstring(posString(e.f.Src.Pos)))
}

// dropShadows gives up what the function read back and the code won't drop.
func (e *fn) dropShadows() {
	for _, v := range sortedValues(e.shadows) {
		if st := e.g.dropStmt(v.Ty, fmt.Sprintf("sh%d", v.ID)); st != "" {
			e.printf("%s", st)
		}
	}
}

// step writes a `!`: a call of a function that stops, or a step the runner
// takes.
func (e *fn) step(in lir.Instr) {
	if c, ok := in.(*lir.Call); ok && e.g.callsCompiled(c) {
		if !lir.Stops(c.Fn) {
			plain := *c
			plain.Durable = false
			e.call(&plain)
			return
		}
		e.stoppingCall(c)
		return
	}
	p := e.stops.byInstr[in]
	vals, held := e.siteArgs(in)
	site := e.g.site(e, in, held)
	e.encodeArgs(vals)
	if len(vals) == 0 {
		e.printf("eyj_stop(%d, NULL, 0);", site)
	} else {
		e.printf("eyj_stop(%d, a_, %d); }", site, len(vals))
	}
	e.printf("goto sv%d;", p.n)
	e.printf("rs%d:;", p.n)
	e.restores(p)
	dst := lir.Dst(in)
	if dst == nil || e.g.layout(dst.Ty).Repr == lir.ReprNone {
		e.printf("eyj_resumed(NULL, NULL);")
		return
	}
	e.printf("eyj_resumed(%s, &%s);", e.g.saver(dst.Ty).dec(), e.v(dst))
}

// stoppingCall calls a function that may stop, and stops too if it did. To
// resume, the call is made again with nothing but its `mut` variables: the
// callee reads the rest from its own bytes.
func (e *fn) stoppingCall(c *lir.Call) {
	p := e.stops.byInstr[c]
	plain := *c
	plain.Durable = false
	e.call(&plain)
	e.printf("goto jn%d;", p.n)
	e.printf("rs%d:;", p.n)
	e.restores(p)
	mut := mutParams(c.Fn)
	var args []string
	for i, a := range c.Args {
		switch {
		case i < len(mut) && mut[i] && e.mut[a]:
			args = append(args, fmt.Sprintf("v%d", a.ID))
		case i < len(mut) && mut[i]:
			args = append(args, fmt.Sprintf("&v%d", a.ID))
		default:
			args = append(args, fmt.Sprintf("(%s){0}", e.g.ctype(c.Fn.Params[i].Ty)))
		}
	}
	if hasDefaults(c.Fn) {
		args = append(args, "UINT64_C(0)")
	}
	e.assign(c.Dst, fmt.Sprintf("%s(%s)", e.g.names[c.Fn], strings.Join(args, ", ")))
	e.printf("jn%d:;", p.n)
	e.printf("if (eyj_cur->stopping) goto sv%d;", p.n)
}

// restores reads a place's live variables back.
func (e *fn) restores(p *stopPlace) {
	for _, v := range p.live {
		e.printf("if (!eyj_get(&rd_, %s, &%s)) ey_fault(%s, rd_.err ? rd_.err : \"a run's saved frame is corrupt\");", e.g.saver(v.Ty).dec(), e.v(v), e.at)
		if e.shadows[v] {
			e.printf("sh%d = %s;", v.ID, e.v(v))
		}
	}
}

// saves writes, after the function's code, what each place does on the way
// out: it saves the live variables, drops what it owns and returns.
func (e *fn) saves() {
	if e.stops == nil {
		return
	}
	rt := e.g.ctype(e.g.resultTy(e.f))
	for _, p := range e.stops.places {
		fmt.Fprintf(&e.b, "sv%d:; {\n\tey_buf *b_ = eyj_save(UINT64_C(%d), %d);\n\t(void)b_;\n", p.n, e.stops.code, p.n)
		for _, v := range p.live {
			fmt.Fprintf(&e.b, "\teyj_put(b_, %s, &%s, sizeof %s);\n", e.g.saver(v.Ty).enc(), e.v(v), e.v(v))
		}
		for _, v := range p.live {
			if e.f.NotOwned[v] {
				continue
			}
			if st := e.g.dropStmt(v.Ty, e.v(v)); st != "" {
				fmt.Fprintf(&e.b, "\t%s\n", st)
			}
		}
		e.dropShadows()
		if e.pending.Len() > 0 {
			fmt.Fprintf(&e.b, "\t%s\n", e.pending.String())
			e.pending.Reset()
		}
		fmt.Fprintf(&e.b, "\t{ %s z_ = {0}; return z_; }\n}\n", rt)
	}
}

// ---------------------------------------------------------------- sites

// siteArgs are the values an instruction gives its site, and what each
// replaces there. A static value crosses as nothing: the site evaluates what
// the program wrote.
func (e *fn) siteArgs(in lir.Instr) ([]*lir.Value, []string) {
	site := lir.SiteOf(in)
	if site == nil {
		e.errorf("the C back end can't tell the runner which call this is (%T)", in)
		return nil, nil
	}
	var args []*lir.Value
	switch x := in.(type) {
	case *lir.Call:
		return e.paramArgs(x, site)
	case *lir.Builtin:
		args = x.Args
	case *lir.Runtime:
		args = x.Args
	case *lir.CallValue:
		e.errorf("a `!` call of a function value isn't compiled yet: `eye run` runs this program")
		return nil, nil
	}
	first := 0
	var written int
	switch c := site.(type) {
	case *ir.Call:
		written = len(c.Args)
	case *ir.MethodCall:
		first, written = 1, len(c.Args)
	case *ir.Field:
		first = 1
	default:
		e.errorf("the runner can't evaluate a %T for compiled code", site)
		return nil, nil
	}
	if len(args) != first+written {
		e.errorf("the low IR gives %d values to a call with %d in its source (%T)", len(args), first+written, in)
		return nil, nil
	}
	var vals []*lir.Value
	var held []string
	for i, a := range args {
		if a == nil || e.static[a] {
			continue
		}
		vals = append(vals, a)
		if i < first {
			held = append(held, "r")
		} else {
			held = append(held, strconv.Itoa(i-first))
		}
	}
	return vals, held
}

// paramArgs are a function call's values, which the low IR orders by the
// callee's parameters, each named so that the runner binds it the same way.
func (e *fn) paramArgs(x *lir.Call, site ir.Expr) ([]*lir.Value, []string) {
	args := x.Args
	var vals []*lir.Value
	var held []string
	if _, method := site.(*ir.MethodCall); method && len(args) > 0 {
		vals = append(vals, args[0])
		held = append(held, "r")
		args = args[1:]
	}
	if x.Fn.Src == nil || len(args) != len(x.Fn.Src.Params) {
		e.errorf("the C back end can't name the arguments of %s for the runner", x.Fn.Name)
		return nil, nil
	}
	for i, a := range args {
		if e.absent(a) {
			continue
		}
		vals = append(vals, a)
		held = append(held, "="+x.Fn.Src.Params[i].Name)
	}
	return vals, held
}

// site is the index of the site an instruction names.
func (g *gen) site(e *fn, in lir.Instr, held []string) int {
	expr := lir.SiteOf(in)
	if expr == nil {
		return -1
	}
	p := expr.Position()
	se := siteEntry{ir.SiteFile(g.prog, p.File), p.Offset, p.End, ir.SiteKind(expr), lir.Durable(in), held}
	key := fmt.Sprintf("%s:%d:%d:%s:%v:%q", se.file, se.offset, se.end, se.kind, se.durable, se.held)
	if i, ok := g.siteIndex[key]; ok {
		return i
	}
	if g.siteIndex == nil {
		g.siteIndex = map[string]int{}
	}
	g.sites = append(g.sites, se)
	g.siteIndex[key] = len(g.sites) - 1
	return len(g.sites) - 1
}

// encodeArgs opens a block with the values' handles in a_; the caller closes
// it, unless there are none.
func (e *fn) encodeArgs(vals []*lir.Value) {
	if len(vals) == 0 {
		return
	}
	e.printf("{ eye_val a_[%d];", len(vals))
	for i, v := range vals {
		e.printf("a_[%d] = eyh_enc(%s, &%s);", i, e.g.saver(v.Ty).enc(), e.v(v))
	}
}

// site writes a call the runner evaluates inside the current call.
func (e *fn) site(in lir.Instr, dst *lir.Value) {
	vals, held := e.siteArgs(in)
	site := e.g.site(e, in, held)
	dec, out := "NULL", "NULL"
	if dst != nil && e.g.layout(dst.Ty).Repr != lir.ReprNone {
		dec, out = e.g.saver(dst.Ty).dec(), "&"+e.v(dst)
	}
	e.encodeArgs(vals)
	if len(vals) == 0 {
		e.printf("eyh_site(%d, NULL, 0, %s, %s);", site, dec, out)
		return
	}
	e.printf("eyh_site(%d, a_, %d, %s, %s); }", site, len(vals), dec, out)
}

// hostedRuntime writes a call into the runtime in a hosted program. It
// reports false for what compiled code does itself: a closure over a
// built-in or a constructor.
func (e *fn) hostedRuntime(x *lir.Runtime) bool {
	switch x.Op {
	case "builtin.value", "variant.constructor", "derived.value":
		return false
	}
	if staticOps[x.Op] {
		if x.Dst != nil {
			e.zero(x.Dst)
		}
		return true
	}
	e.site(x, x.Dst)
	return true
}

// opaqueClosure writes a closure the runner runs: its encoding, the code's
// name and its captures, which it takes.
func (e *fn) opaqueClosure(x *lir.MakeClosure) {
	if x.Fn.Src == nil || x.Fn.Src.Module == nil {
		e.errorf("the C back end can't name this function for the runner")
		return
	}
	name := "lambda " + x.Fn.Src.Module.Path + "." + ir.LambdaHash(x.Fn.Src)
	e.printf("{ ey_buf c_ = {0}; ey_buf_char(&c_, 'C'); eys_text(&c_, %s, %d); eys_uvarint(&c_, %d);", cstring(name), len(name), len(x.Captures))
	for _, c := range x.Captures {
		e.printf("%s(&c_, &%s);", e.g.saver(c.Ty).enc(), e.v(c))
	}
	e.printf("%s = ey_str_new(c_.b, c_.len, &ey_t_string); free(c_.b);", e.v(x.Dst))
	for _, c := range x.Captures {
		if st := e.g.dropStmt(c.Ty, e.v(c)); st != "" {
			e.printf("%s", st)
		}
	}
	e.printf("}")
}

// ---------------------------------------------------------------- saved entities

// saveEntity writes an entity value's encoder and decoder: a record of its
// identity, a snapshot's version and its states, in the order the runner
// keeps them. A state has no version; stateSaver writes that form.
func (g *gen) saveEntity(enc, dec *strings.Builder, x *ir.TCon) {
	names, types, _ := lir.RecordFields(x)
	d := g.desc(x)
	fmt.Fprintf(enc, "\tey_buf_char(b, 'R');\n\teys_uvarint(b, %d);\n", len(names))
	fmt.Fprintf(dec, "\tuint64_t n;\n\tif (!eys_read_record(r, &n)) return false;\n")
	var fields []string
	for i, ft := range types {
		// A state has no version: a snapshot's is read, and a state's is 0.
		zero := "{0}"
		if names[i] == "version" {
			zero = "EY_SMALL(INT64_C(0))"
		}
		fmt.Fprintf(dec, "\t%s f%d = %s;\n", g.ctype(ft), i, zero)
		fields = append(fields, fmt.Sprintf("&f%d", i))
	}
	fmt.Fprintf(dec, "\tfor (uint64_t i = 0; i < n; i++) {\n\t\tconst char *name;\n\t\tint64_t len;\n\t\tif (!eys_read_text(r, &name, &len)) return false;\n")
	for i, name := range names {
		f := g.saver(types[i])
		fmt.Fprintf(enc, "\teys_text(b, %s, %d);\n\t%s(b, ey_field(%s, v, %d));\n", cstring(name), len(name), f.enc(), d, i)
		fmt.Fprintf(dec, "\t\tif (eys_text_is(name, len, %s)) {\n\t\t\tif (!%s(r, &f%d)) return false;\n\t\t\tcontinue;\n\t\t}\n", cstring(name), f.dec(), i)
	}
	dec.WriteString("\t\treturn eys_fail(r, \"the saved state has a field its entity doesn't\");\n\t}\n")
	fmt.Fprintf(dec, "\tey_tuple_make(%s, out, (const void *[]){%s});\n\treturn true;\n", d, strings.Join(fields, ", "))
}

// stateEncoder writes the encoder of an entity's state, as the store keeps
// it: without a snapshot's version.
func (g *gen) stateEncoder(he hostEntity, n int) string {
	names, types, _ := lir.RecordFields(he.ty)
	d := g.desc(he.ty)
	var b strings.Builder
	fmt.Fprintf(&b, "static void eyh_state%d(ey_buf *b, const void *v) {\n\tey_buf_char(b, 'R');\n\teys_uvarint(b, %d);\n", n, len(names)-1)
	for i, name := range names {
		if name == "version" {
			continue
		}
		f := g.saver(types[i])
		fmt.Fprintf(&b, "\teys_text(b, %s, %d);\n\t%s(b, ey_field(%s, v, %d));\n", cstring(name), len(name), f.enc(), d, i)
	}
	b.WriteString("}\n\n")
	return b.String()
}

// ---------------------------------------------------------------- entry points

// hostedEntry writes eye_host.h's entry points and tables.
func (g *gen) hostedEntry() string {
	var b strings.Builder
	maxParams := 1
	for _, j := range g.hostJobs {
		maxParams = max(maxParams, len(j.fn.Params))
	}
	for _, e := range g.hostEntities {
		for _, m := range e.methods {
			maxParams = max(maxParams, len(m.fn.Params))
		}
	}
	// The runner reads each value as its encoding, so a type's number means
	// nothing to it: every type is 0.
	fmt.Fprintf(&b, "static const int32_t eyh_types[%d] = {0};\n\n", maxParams)
	for i, he := range g.hostEntities {
		b.WriteString(g.stateEncoder(he, i))
		for k, m := range he.methods {
			b.WriteString(g.methodEntry(i, k, m))
		}
	}
	for i, j := range g.hostJobs {
		b.WriteString(g.jobEntry(i, j))
	}
	// The tables.
	for i, he := range g.hostEntities {
		fmt.Fprintf(&b, "static const eye_method_info eyh_ms%d[] = {\n", i)
		for _, m := range he.methods {
			fmt.Fprintf(&b, "\t{%s, %d, eyh_types, 0},\n", cstring(m.name), len(m.fn.Params)-1)
		}
		if len(he.methods) == 0 {
			b.WriteString("\t{NULL, 0, NULL, 0},\n")
		}
		b.WriteString("};\n")
	}
	b.WriteString("static const eye_entity_info eyh_entities[] = {\n")
	for i, he := range g.hostEntities {
		fmt.Fprintf(&b, "\t{%s, 0, 0, 0, %d, eyh_ms%d},\n", cstring(he.name), len(he.methods), i)
	}
	if len(g.hostEntities) == 0 {
		b.WriteString("\t{NULL, 0, 0, 0, 0, NULL},\n")
	}
	b.WriteString("};\nstatic const eye_job_info eyh_jobs[] = {\n")
	for _, j := range g.hostJobs {
		fmt.Fprintf(&b, "\t{%s, %d, eyh_types, 0, 0},\n", cstring(j.name), len(j.fn.Params))
	}
	if len(g.hostJobs) == 0 {
		b.WriteString("\t{NULL, 0, NULL, 0, 0},\n")
	}
	b.WriteString("};\n")
	for i, s := range g.sites {
		if len(s.held) == 0 {
			continue
		}
		fmt.Fprintf(&b, "static const char *const eyh_held%d[] = {", i)
		for _, h := range s.held {
			fmt.Fprintf(&b, "%s, ", cstring(h))
		}
		b.WriteString("};\n")
	}
	b.WriteString("static const eye_site_info eyh_sites[] = {\n")
	for i, s := range g.sites {
		held := "NULL"
		if len(s.held) > 0 {
			held = fmt.Sprintf("eyh_held%d", i)
		}
		durable := 0
		if s.durable {
			durable = 1
		}
		fmt.Fprintf(&b, "\t{%s, %d, %d, %s, %d, %d, %s},\n", cstring(s.file), s.offset, s.end, cstring(s.kind), durable, len(s.held), held)
	}
	if len(g.sites) == 0 {
		b.WriteString("\t{NULL, 0, 0, NULL, 0, 0, NULL},\n")
	}
	b.WriteString("};\n\n")
	// The program, and the dispatch to each entry.
	fmt.Fprintf(&b, "const eye_program_info *eye_program(void) {\n\tstatic eye_program_info p;\n\tp.version = EYE_HOST_VERSION;\n\tp.nentities = %d;\n\tp.entities = eyh_entities;\n\tp.njobs = %d;\n\tp.jobs = eyh_jobs;\n\tp.nsites = %d;\n\tp.sites = eyh_sites;\n\tey_zones(ey_zone_list, %d);\n\treturn &p;\n}\n\n",
		len(g.hostEntities), len(g.hostJobs), len(g.sites), g.nzones)
	// Each runs on the big stack (eyh_big), with its arguments in a struct.
	b.WriteString("typedef struct { eye_rt *rt; int32_t entity, method; eye_val *state; const eye_val *args; eye_val *result; } eyh_mcall;\n\n")
	b.WriteString("static int eyh_method(void *p) {\n\teyh_mcall *c = p;\n\tswitch (c->entity) {\n")
	for i, he := range g.hostEntities {
		fmt.Fprintf(&b, "\tcase %d:\n\t\tswitch (c->method) {\n", i)
		for k := range he.methods {
			fmt.Fprintf(&b, "\t\tcase %d: return eyh_m%d_%d(c->rt, c->state, c->args, c->result);\n", k, i, k)
		}
		b.WriteString("\t\t}\n\t\tbreak;\n")
	}
	b.WriteString("\t}\n\treturn EYE_FAULT;\n}\n\n")
	b.WriteString("eye_status eye_call_method(eye_rt *rt, int32_t entity, int32_t method, eye_val *state, const eye_val *args, int32_t nargs, eye_val *result) {\n\t(void)nargs;\n\teyh_mcall c = {rt, entity, method, state, args, result};\n\treturn (eye_status)eyh_big(eyh_method, &c);\n}\n\n")
	b.WriteString("typedef struct { eye_rt *rt; int32_t job; eye_val *frame; eye_val resumed; eye_step *step; eye_val *result; } eyh_jcall;\n\n")
	b.WriteString("static int eyh_job(void *p) {\n\teyh_jcall *c = p;\n\tswitch (c->job) {\n")
	for i := range g.hostJobs {
		fmt.Fprintf(&b, "\tcase %d: return eyh_j%d(c->rt, c->frame, c->resumed, c->step, c->result);\n", i, i)
	}
	b.WriteString("\t}\n\treturn EYE_FAULT;\n}\n\n")
	b.WriteString("eye_status eye_step_job(eye_rt *rt, int32_t job, eye_val *frame, eye_val resumed, eye_step *step, eye_val *result) {\n\teyh_jcall c = {rt, job, frame, resumed, step, result};\n\treturn (eye_status)eyh_big(eyh_job, &c);\n}\n")
	return b.String()
}

// entryArgs writes the arguments' variables, from first on, and the call.
func (g *gen) entryCall(f *lir.Func, first []string, from int) []string {
	args := append([]string(nil), first...)
	for i := from; i < len(f.Params); i++ {
		args = append(args, fmt.Sprintf("a%d", i))
	}
	if hasDefaults(f) {
		args = append(args, "UINT64_C(0)")
	}
	return args
}

// dropBorrowed drops the arguments the function only borrowed: the entry
// made them and still owns them.
func (g *gen) dropBorrowed(b *strings.Builder, f *lir.Func, from int) {
	for i := from; i < len(f.Params); i++ {
		if i < len(f.Modes) && f.Modes[i] == lir.Owned {
			continue
		}
		if st := g.dropStmt(f.Params[i].Ty, fmt.Sprintf("a%d", i)); st != "" {
			fmt.Fprintf(b, "\t%s\n", st)
		}
	}
}

// methodEntry writes the entry point of an entity's method.
func (g *gen) methodEntry(ent, k int, m hostMethod) string {
	f := m.fn
	var b strings.Builder
	fmt.Fprintf(&b, "static eye_status eyh_m%d_%d(eye_rt *rt, eye_val *state, const eye_val *args, eye_val *result) {\n\t(void)args;\n\teyh_trap t_;\n\tEYH_TRY(t_, rt);\n", ent, k)
	st := f.Params[0].Ty
	fmt.Fprintf(&b, "\t%s st = {0};\n\teyh_dec(*state, %s, &st);\n", g.ctype(st), g.saver(st).dec())
	for i := 1; i < len(f.Params); i++ {
		p := f.Params[i]
		fmt.Fprintf(&b, "\t%s a%d = {0};\n\teyh_dec(args[%d], %s, &a%d);\n", g.ctype(p.Ty), i, i-1, g.saver(p.Ty).dec(), i)
	}
	rt := g.resultTy(f)
	fmt.Fprintf(&b, "\t%s r = %s(%s);\n", g.ctype(rt), g.names[f], strings.Join(g.entryCall(f, []string{"&st"}, 1), ", "))
	fmt.Fprintf(&b, "\teye_val old = *state;\n\t*state = eyh_enc(eyh_state%d, &st);\n\teye_release(old);\n", ent)
	fmt.Fprintf(&b, "\t*result = eyh_enc(%s, &r);\n", g.saver(rt).enc())
	if s := g.dropStmt(rt, "r"); s != "" {
		fmt.Fprintf(&b, "\t%s\n", s)
	}
	if s := g.dropStmt(st, "st"); s != "" {
		fmt.Fprintf(&b, "\t%s\n", s)
	}
	g.dropBorrowed(&b, f, 1)
	b.WriteString("\teyh_leave(&t_);\n\treturn EYE_OK;\n}\n\n")
	return b.String()
}

// jobEntry writes the entry point of a job: from its first frame, or from
// where it stopped.
func (g *gen) jobEntry(i int, j hostJob) string {
	f := j.fn
	var b strings.Builder
	fmt.Fprintf(&b, "static eye_status eyh_j%d(eye_rt *rt, eye_val *frame, eye_val resumed, eye_step *step, eye_val *result) {\n\teyh_trap t_;\n\tEYH_TRY(t_, rt);\n\teyj j_;\n\teyj_begin(&j_, *frame, resumed);\n", i)
	for k, p := range f.Params {
		fmt.Fprintf(&b, "\t%s a%d = {0};\n", g.ctype(p.Ty), k)
	}
	b.WriteString("\tif (!j_.resuming) {\n")
	for k, p := range f.Params {
		fmt.Fprintf(&b, "\t\teyj_arg(&j_, %s, &a%d);\n", g.saver(p.Ty).dec(), k)
	}
	b.WriteString("\t}\n")
	rt := g.resultTy(f)
	fmt.Fprintf(&b, "\t%s r = %s(%s);\n", g.ctype(rt), g.names[f], strings.Join(g.entryCall(f, nil, 0), ", "))
	b.WriteString("\tif (j_.stopping) {\n\t\teye_val old = *frame;\n\t\t*frame = eyj_frame(&j_);\n\t\teye_release(old);\n\t\teyj_step(&j_, *frame, step);\n")
	var drops strings.Builder
	g.dropBorrowed(&drops, f, 0)
	b.WriteString(drops.String())
	b.WriteString("\t\teyj_end(&j_);\n\t\teyh_leave(&t_);\n\t\treturn EYE_WAITING;\n\t}\n")
	fmt.Fprintf(&b, "\t*result = eyh_enc(%s, &r);\n", g.saver(rt).enc())
	if s := g.dropStmt(rt, "r"); s != "" {
		fmt.Fprintf(&b, "\t%s\n", s)
	}
	b.WriteString(drops.String())
	b.WriteString("\teyj_end(&j_);\n\teyh_leave(&t_);\n\treturn EYE_OK;\n}\n\n")
	return b.String()
}
