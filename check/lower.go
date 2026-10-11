package check

import (
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"

	"eyelang/ir"
	"eyelang/syntax"
)

// ctxKind is the kind of code being lowered, which decides what it may do.
type ctxKind int

const (
	ctxPlain        ctxKind = iota // a function or method; it may wait, which makes it a waiting function
	ctxJob                         // a job body
	ctxEntityMethod                // an entity method: a transaction, which can't wait
	ctxOperation                   // serves a contract's operation: a transaction, which can't wait
	ctxTest                        // a test block
	ctxView                        // a view's pipeline
	ctxGlobal                      // a module-level let or a state default
	ctxMain                        // main
	ctxLambda                      // a lambda: like its enclosing function
)

// local is a variable in scope.
type local struct {
	slot    int
	mutable bool   // var, or a mut parameter
	isVar   bool   // a var, which closures can't capture
	at      ir.Pos // where it is bound
}

// lowerer lowers one function body.
type lowerer struct {
	c      *checker
	m      *module
	f      *ir.File
	fn     *ir.Func
	kind   ctxKind
	scopes []map[string]local
	parent *lowerer // for lambdas
	entity *ir.Entity
	self   *ir.Type
	wrap   *ir.Type // the error type `?` wraps into
	selfAt ir.Pos   // where a method's self parameter is declared
	loops  int
}

func (c *checker) newFunc(m *module, f *ir.File, name string, at ir.Pos, kind ctxKind) *lowerer {
	return &lowerer{
		c: c, m: m, f: f, kind: kind,
		fn:     &ir.Func{Name: name, Module: m.mod, Pos: at},
		scopes: []map[string]local{{}},
	}
}

func (l *lowerer) done() *ir.Func { return l.fn }

// lowerFunc lowers a function declaration: parameters, return type and body.
func (c *checker) lowerFunc(m *module, f *ir.File, d *syntax.Node, fn *ir.Func, kind ctxKind, entity *ir.Entity, self *ir.Type) {
	if fn.Name == "main" && kind == ctxPlain && entity == nil && self == nil {
		kind = ctxMain
	}
	l := &lowerer{c: c, m: m, f: f, fn: fn, kind: kind, scopes: []map[string]local{{}}, entity: entity, self: self}
	if params := d.ChildByField(syntax.FieldParameters); params != nil {
		for i, p := range params.NamedChildren() {
			nameNode := p.ChildByField(syntax.FieldName)
			name := text(f, nameNode)
			if i == 0 && fn.HasSelf && name == "self" {
				l.selfAt = pos(f, nameNode)
				c.use(l.selfAt, Local{At: l.selfAt})
				continue // self, in an impl's or a trait's method
			}
			if (name == "in" || name == "for") && d.ChildByField(syntax.FieldBody) != nil {
				c.errorf(pos(f, nameNode), "%s is a keyword, so the body couldn't read a parameter of that name; only an extern fn's parameters may be named in or for", name)
			}
			param := &ir.Param{Name: name, Mut: hasToken(f, p, "mut"), Type: c.typeExpr(m, f, p.ChildByField(syntax.FieldType), nil)}
			if nameNode.Kind == syntax.Wildcard {
				param.Name = "_"
				param.Slot = l.newSlot()
			} else {
				param.Slot = l.bind(nameNode, name, param.Mut, false)
			}
			if def := p.ChildByField(syntax.FieldDefault); def != nil {
				param.Default = l.expr(def)
			}
			fn.Params = append(fn.Params, param)
		}
	}
	fn.Return = c.typeExpr(m, f, d.ChildByField(syntax.FieldReturnType), nil)
	c.lowerBounds(m, f, d, fn)
	fn.Uses, fn.HasUses = usesOf(f, d)
	l.wrap = errorType(fn.Return)
	if body := d.ChildByField(syntax.FieldBody); body != nil {
		fn.Body = l.expr(body)
	}
}

// usesOf reads a declaration's uses clause: each effect as written, such as
// "net Notify.send".
func usesOf(f *ir.File, d *syntax.Node) ([]string, bool) {
	clause := d.ChildByField(syntax.FieldEffects)
	if clause == nil {
		return nil, false
	}
	var out []string
	for _, e := range clause.NamedChildren() {
		kind := text(f, e.ChildByField(syntax.FieldKind))
		if target := e.ChildByField(syntax.FieldTarget); target != nil {
			kind += " " + text(f, target)
		}
		out = append(out, kind)
	}
	return out, true
}

// errorType returns E for a function that returns Result(T, E), when E is a
// declared type.
func errorType(t *ir.TypeExpr) *ir.Type {
	if t != nil && t.Name == "Result" && len(t.Args) == 2 {
		return t.Args[1].Decl
	}
	return nil
}

func (l *lowerer) newSlot() int {
	s := l.fn.Slots
	l.fn.Slots++
	return s
}

// bind declares the local that n names.
func (l *lowerer) bind(n *syntax.Node, name string, mutable, isVar bool) int {
	s := l.newSlot()
	l.scopes[len(l.scopes)-1][name] = local{slot: s, mutable: mutable, isVar: isVar, at: l.pos(n)}
	l.c.use(l.pos(n), Local{At: l.pos(n)})
	return s
}

func (l *lowerer) push() { l.scopes = append(l.scopes, map[string]local{}) }
func (l *lowerer) pop()  { l.scopes = l.scopes[:len(l.scopes)-1] }

func (l *lowerer) pos(n *syntax.Node) ir.Pos { return pos(l.f, n) }
func (l *lowerer) at(n *syntax.Node) ir.At   { return ir.At{Pos: l.pos(n)} }
func (l *lowerer) text(n *syntax.Node) string {
	return text(l.f, n)
}

func (l *lowerer) errorf(n *syntax.Node, format string, args ...any) {
	l.c.errorf(l.pos(n), format, args...)
}

// lookupLocal finds a local in this function, or captures one from an
// enclosing function.
func (l *lowerer) lookupLocal(name string, n *syntax.Node) (local, bool) {
	for i := len(l.scopes) - 1; i >= 0; i-- {
		if v, ok := l.scopes[i][name]; ok {
			return v, true
		}
	}
	if l.parent == nil {
		return local{}, false
	}
	outer, ok := l.parent.lookupLocal(name, n)
	if ok {
		switch {
		case outer.isVar:
			l.errorf(n, "a closure can't capture %s, a var; copy it into a let first", name)
		case outer.mutable:
			l.errorf(n, "a closure can't capture %s, a mut parameter; copy it into a let first", name)
		}
		slot := l.newSlot()
		l.fn.Captures = append(l.fn.Captures, ir.Capture{From: &ir.Local{At: l.at(n), Name: name, Slot: outer.slot}, Slot: slot})
		l.scopes[0][name] = local{slot: slot, at: outer.at}
		return local{slot: slot, at: outer.at}, true
	}
	// A state field or the receiver of the enclosing method.
	if l.parent.selfField(name) || name == "self" && l.parent.hasSelf() {
		slot := l.newSlot()
		var from ir.Expr = &ir.Self{At: l.at(n)}
		if l.parent.parent != nil {
			if v, ok := l.parent.lookupLocal("self", n); ok {
				from = &ir.Local{At: l.at(n), Name: "self", Slot: v.slot}
			}
		}
		l.fn.Captures = append(l.fn.Captures, ir.Capture{From: from, Slot: slot})
		l.scopes[0]["self"] = local{slot: slot}
		return local{}, false
	}
	return local{}, false
}

// useSelf records what `self` at n refers to: the method's self parameter,
// or the entity whose method it is.
func (l *lowerer) useSelf(n *syntax.Node) {
	m := l.method()
	if m.selfAt.File != nil {
		l.c.use(l.pos(n), Local{At: m.selfAt})
	} else if m.entity != nil {
		l.c.use(l.pos(n), m.entity)
	}
}

// method returns the lowerer of the method this code is in, through lambdas.
func (l *lowerer) method() *lowerer {
	for l.parent != nil {
		l = l.parent
	}
	return l
}

func (l *lowerer) hasSelf() bool {
	m := l.method()
	return m.fn.HasSelf
}

func (l *lowerer) selfField(name string) bool {
	m := l.method()
	if m.entity == nil {
		return false
	}
	if m.entity.ID != nil && m.entity.ID.Name == name {
		return true
	}
	for _, s := range m.entity.States {
		if s.Name == name {
			return true
		}
	}
	return false
}

// canWait reports whether waiting calls may appear here.
func (l *lowerer) canWait() (bool, string) {
	switch l.method().kind {
	case ctxJob, ctxPlain:
		return true, ""
	case ctxEntityMethod:
		return false, "an entity method is a transaction and can't wait; start a job for this instead"
	case ctxOperation:
		return false, "serving an operation is a transaction and can't wait; start a job for this instead"
	case ctxTest:
		return false, "a test can't make waiting calls; start a job, then advance(…) the clock"
	case ctxMain:
		return false, "`!` marks a durable step, and only jobs have them; in main, call it without `!`"
	}
	return false, "`!` marks a durable step, and only jobs have them"
}

// ---------------------------------------------------------------- statements

func (l *lowerer) block(n *syntax.Node) ir.Expr {
	b := &ir.Block{At: l.at(n)}
	l.push()
	stmts := n.NamedChildren()
	for i, s := range stmts {
		e := l.stmt(s)
		b.Exprs = append(b.Exprs, e)
		if i == len(stmts)-1 && s.Kind.IsExpression() {
			b.Value = true
		}
	}
	l.pop()
	return b
}

func (l *lowerer) stmt(n *syntax.Node) ir.Expr {
	switch n.Kind {
	case syntax.LetStatement:
		value := l.expr(n.ChildByField(syntax.FieldValue))
		var elseBody ir.Expr
		if alt := n.ChildByField(syntax.FieldAlternative); alt != nil {
			elseBody = l.block(alt)
		}
		pat := l.pattern(n.ChildByField(syntax.FieldPattern), map[string]int{})
		return &ir.Let{At: l.at(n), Pattern: pat, Type: l.c.typeExpr(l.m, l.f, n.ChildByField(syntax.FieldType), nil),
			Value: value, Else: elseBody}
	case syntax.VarStatement:
		value := l.expr(n.ChildByField(syntax.FieldValue))
		name := n.ChildByField(syntax.FieldName)
		slot := l.bind(name, l.text(name), true, true)
		return &ir.Let{At: l.at(n), Pattern: ir.PBind{Name: l.text(name), Slot: slot},
			Type: l.c.typeExpr(l.m, l.f, n.ChildByField(syntax.FieldType), nil), Value: value}
	case syntax.Assignment:
		op := l.text(n.ChildByField(syntax.FieldOperator))
		value := l.expr(n.ChildByField(syntax.FieldRight))
		place, ok := l.place(n.ChildByField(syntax.FieldLeft))
		if !ok {
			return &ir.Block{At: l.at(n)}
		}
		return &ir.Assign{At: l.at(n), Target: place, Op: op, Value: value}
	case syntax.ForStatement:
		iter := l.expr(n.ChildByField(syntax.FieldValue))
		l.push()
		pat := l.pattern(n.ChildByField(syntax.FieldPattern), map[string]int{})
		l.loops++
		body := l.block(n.ChildByField(syntax.FieldBody))
		l.loops--
		l.pop()
		return &ir.For{At: l.at(n), Pattern: pat, Iter: iter, Body: body}
	case syntax.WhileStatement:
		cond := l.expr(n.ChildByField(syntax.FieldCondition))
		l.loops++
		body := l.block(n.ChildByField(syntax.FieldBody))
		l.loops--
		return &ir.While{At: l.at(n), Cond: cond, Body: body}
	case syntax.ReturnStatement:
		if l.kind == ctxLambda {
			l.errorf(n, "return inside a lambda would leave only the lambda; give the lambda's value instead")
		}
		r := &ir.Return{At: l.at(n)}
		if v := n.ChildByField(syntax.FieldValue); v != nil {
			r.Value = l.expr(v)
		}
		return r
	case syntax.BreakStatement, syntax.ContinueStatement:
		if l.loops == 0 {
			l.errorf(n, "%s outside a loop", l.text(n))
		}
		if n.Kind == syntax.BreakStatement {
			return &ir.Break{At: l.at(n)}
		}
		return &ir.Continue{At: l.at(n)}
	case syntax.WithStatement:
		if l.method().kind != ctxTest {
			l.errorf(n, "with applies a test handler, such as clock(…) or fakes(…), and works only in tests")
		}
		return &ir.With{At: l.at(n), Value: l.expr(n.ChildByField(syntax.FieldValue))}
	}
	return l.expr(n)
}

// place lowers the left side of an assignment or a `mut` argument.
func (l *lowerer) place(n *syntax.Node) (ir.Place, bool) {
	var path []string
	var names []ir.Pos
	root := n
	for root.Kind == syntax.FieldExpression {
		path = append([]string{l.text(root.ChildByField(syntax.FieldField))}, path...)
		names = append([]ir.Pos{l.pos(root.ChildByField(syntax.FieldField))}, names...)
		root = root.ChildByField(syntax.FieldValue)
	}
	p := ir.Place{Path: path, Pos: l.pos(n)}
	l.c.placeAt(p.Pos, names)
	if root.Kind != syntax.Ident {
		l.errorf(n, "only a variable, a mut parameter, or a field of one, can be assigned")
		return p, false
	}
	name := l.text(root)
	if name == "self" {
		l.useSelf(root)
		m := l.method()
		if !m.fn.MutSelf || l.parent != nil {
			l.errorf(root, "self can change only in a `mut self` method")
			return p, false
		}
		p.Self = true
		return p, true
	}
	if v, ok := l.lookupLocal(name, root); ok {
		l.c.use(l.pos(root), Local{At: v.at})
		if !v.mutable {
			l.errorf(root, "%s can't change: it is a let or a parameter; declare it with var, or as a mut parameter", name)
			return p, false
		}
		// A var found here is the closure's own: lookupLocal refuses to
		// capture one from outside.
		p.Slot = v.slot
		return p, true
	}
	if l.selfField(name) {
		l.c.use(l.pos(root), Member{Of: l.method().entity, Name: name})
		if l.parent != nil {
			l.errorf(root, "a closure can't change the entity's state")
			return p, false
		}
		if m := l.method(); m.entity != nil && m.entity.ID != nil && m.entity.ID.Name == name {
			l.errorf(root, "%s is the entity's identity and can't change", name)
			return p, false
		}
		p.Self = true
		p.Path = append([]string{name}, path...)
		l.c.placeAt(p.Pos, append([]ir.Pos{l.pos(root)}, names...))
		return p, true
	}
	l.errorf(root, "undefined name %s", name)
	return p, false
}

// quietPlace returns the place a method's receiver names, if it is a var, a
// mut parameter or a field of one, without reporting anything.
func (l *lowerer) quietPlace(n *syntax.Node) *ir.Place {
	var path []string
	root := n
	for root.Kind == syntax.FieldExpression {
		path = append([]string{l.text(root.ChildByField(syntax.FieldField))}, path...)
		root = root.ChildByField(syntax.FieldValue)
	}
	if root.Kind != syntax.Ident || l.parent != nil {
		return nil
	}
	name := l.text(root)
	if name == "self" && l.method().fn.MutSelf {
		return &ir.Place{Self: true, Path: path, Pos: l.pos(n)}
	}
	for i := len(l.scopes) - 1; i >= 0; i-- {
		if v, ok := l.scopes[i][name]; ok {
			if !v.mutable {
				return nil
			}
			return &ir.Place{Slot: v.slot, Path: path, Pos: l.pos(n)}
		}
	}
	return nil
}

// ---------------------------------------------------------------- expressions

func (l *lowerer) expr(n *syntax.Node) ir.Expr {
	at := l.at(n)
	switch n.Kind {
	case syntax.Block:
		return l.block(n)
	case syntax.Ident:
		return l.name(n)
	case syntax.TypeIdent:
		return l.typeName(n)
	case syntax.Int:
		return &ir.Lit{At: at, Value: parseInt(l.text(n))}
	case syntax.Float:
		f, _ := strconv.ParseFloat(strings.ReplaceAll(l.text(n), "_", ""), 64)
		return &ir.Lit{At: at, Value: f}
	case syntax.Boolean:
		return &ir.Lit{At: at, Value: l.text(n) == "true"}
	case syntax.Unit:
		return &ir.Lit{At: at, Value: ir.Unit{}}
	case syntax.Char:
		return &ir.Lit{At: at, Value: parseChar(l.text(n))}
	case syntax.String:
		return l.stringLit(n)
	case syntax.Duration:
		return &ir.Lit{At: at, Value: durationOf(l.text(n.ChildByField(syntax.FieldValue)), l.text(n.ChildByField(syntax.FieldUnit)))}
	case syntax.Size:
		return &ir.Lit{At: at, Value: sizeOf(l.text(n.ChildByField(syntax.FieldValue)), l.text(n.ChildByField(syntax.FieldUnit)))}
	case syntax.Date:
		return &ir.Lit{At: at, Value: parseDate(l.text(n))}
	case syntax.Time:
		return &ir.Lit{At: at, Value: parseTime(l.text(n))}
	case syntax.Instant, syntax.ZonedTime:
		v := ir.Instant{Time: parseTime(l.text(n.ChildByField(syntax.FieldTime)))}
		if d := n.ChildByField(syntax.FieldDate); d != nil {
			v.Date = parseDate(l.text(d))
		}
		zone := n.ChildByField(syntax.FieldZone)
		if zone.Kind == syntax.Ident {
			return &ir.InZone{At: at, Instant: v, Zone: l.name(zone)}
		}
		v.Zone = strings.Trim(l.text(zone), `"`)
		return &ir.Lit{At: at, Value: v}
	case syntax.EmptyBraces:
		return &ir.EmptyBraces{At: at}
	case syntax.List:
		lst := &ir.List{At: at}
		for _, e := range n.NamedChildren() {
			lst.Elems = append(lst.Elems, l.expr(e))
		}
		return lst
	case syntax.Tuple:
		t := &ir.Tuple{At: at}
		for _, e := range n.NamedChildren() {
			t.Elems = append(t.Elems, l.expr(e))
		}
		return t
	case syntax.Record:
		r := &ir.Record{At: at}
		var names []ir.Pos
		for _, fi := range n.NamedChildren() {
			if fi.Kind == syntax.ShorthandField {
				r.Names = append(r.Names, l.text(fi))
				r.Values = append(r.Values, l.name(fi))
				names = append(names, l.pos(fi))
				continue
			}
			r.Names = append(r.Names, l.text(fi.ChildByField(syntax.FieldName)))
			r.Values = append(r.Values, l.expr(fi.ChildByField(syntax.FieldValue)))
			names = append(names, l.pos(fi.ChildByField(syntax.FieldName)))
		}
		l.c.fieldsAt(r, names)
		return r
	case syntax.RecordUpdate:
		r := &ir.RecordUpdate{At: at, Base: l.expr(n.ChildByField(syntax.FieldBase))}
		var names []ir.Pos
		for _, fi := range n.NamedChildren() {
			if fi.Kind != syntax.FieldInitializer {
				continue
			}
			r.Names = append(r.Names, l.text(fi.ChildByField(syntax.FieldName)))
			r.Values = append(r.Values, l.expr(fi.ChildByField(syntax.FieldValue)))
			names = append(names, l.pos(fi.ChildByField(syntax.FieldName)))
		}
		l.c.fieldsAt(r, names)
		return r
	case syntax.Map:
		mp := &ir.Map{At: at}
		for _, e := range n.NamedChildren() {
			mp.Keys = append(mp.Keys, l.expr(e.ChildByField(syntax.FieldKey)))
			mp.Values = append(mp.Values, l.expr(e.ChildByField(syntax.FieldValue)))
		}
		return mp
	case syntax.ParenthesizedExpression:
		return l.expr(n.NamedChildren()[0])
	case syntax.UnaryExpression:
		return &ir.Unary{At: at, Op: l.text(n.ChildByField(syntax.FieldOperator)), X: l.expr(n.ChildByField(syntax.FieldOperand))}
	case syntax.BinaryExpression:
		return l.binary(n)
	case syntax.RangeExpression:
		r := &ir.Range{At: at, Inclusive: l.text(n.ChildByField(syntax.FieldOperator)) == "..="}
		if s := n.ChildByField(syntax.FieldStart); s != nil {
			r.Start = l.expr(s)
		}
		if e := n.ChildByField(syntax.FieldEnd); e != nil {
			r.End = l.expr(e)
		}
		return r
	case syntax.CallExpression:
		return l.call(n, n.ChildByField(syntax.FieldFunction), l.args(n.ChildByField(syntax.FieldArguments)), false)
	case syntax.WaitingCallExpression:
		if ok, why := l.canWait(); !ok {
			l.errorf(n, "%s", why)
		}
		call := l.call(n, n.ChildByField(syntax.FieldFunction), l.args(n.ChildByField(syntax.FieldArguments)), true)
		return &ir.Waiting{At: at, Call: call}
	case syntax.FieldExpression:
		return l.field(n)
	case syntax.IndexExpression:
		return &ir.Index{At: at, Value: l.expr(n.ChildByField(syntax.FieldValue)), Index: l.expr(n.ChildByField(syntax.FieldIndex))}
	case syntax.TryExpression:
		return &ir.Try{At: at, X: l.expr(n.ChildByField(syntax.FieldValue)), Wrap: l.method().wrap}
	case syntax.IfExpression:
		e := &ir.If{At: at, Cond: l.expr(n.ChildByField(syntax.FieldCondition)), Then: l.expr(n.ChildByField(syntax.FieldConsequence))}
		if alt := n.ChildByField(syntax.FieldAlternative); alt != nil {
			e.Else = l.expr(alt)
		}
		return e
	case syntax.MatchExpression:
		return l.match(n)
	case syntax.Lambda:
		return l.lambda(n)
	case syntax.ReturnStatement, syntax.BreakStatement, syntax.ContinueStatement, syntax.Assignment:
		return l.stmt(n) // an assignment here is a branch: `Ok(t) => found += [t]`
	case syntax.Placeholder:
		l.errorf(n, "_ stands for the piped value, so it works only in the arguments of a call after |>")
		return &ir.Lit{At: at, Value: ir.Unit{}}
	}
	l.errorf(n, "%s isn't supported by the interpreter yet", n.Kind)
	return &ir.Lit{At: at, Value: ir.Unit{}}
}

// name resolves a lower-case name in an expression.
func (l *lowerer) name(n *syntax.Node) ir.Expr {
	name := l.text(n)
	at := l.at(n)
	if name == "self" {
		l.useSelf(n)
	}
	if v, ok := l.lookupLocal(name, n); ok {
		l.c.use(at.Pos, Local{At: v.at})
		return &ir.Local{At: at, Name: name, Slot: v.slot}
	}
	if name == "self" {
		if l.hasSelf() {
			if l.parent != nil {
				if v, ok := l.scopes[0]["self"]; ok {
					return &ir.Local{At: at, Name: "self", Slot: v.slot}
				}
			}
			return &ir.Self{At: at}
		}
		l.errorf(n, "self is only in methods")
		return &ir.Lit{At: at, Value: ir.Unit{}}
	}
	if l.selfField(name) {
		l.c.use(at.Pos, Member{Of: l.method().entity, Name: name})
		if l.parent != nil {
			if v, ok := l.scopes[0]["self"]; ok {
				return &ir.Field{At: at, Value: &ir.Local{At: at, Name: "self", Slot: v.slot}, Name: name}
			}
		}
		return &ir.SelfField{At: at, Name: name}
	}
	if d := l.c.lookupValue(l.m, name); d != nil {
		l.c.use(at.Pos, d)
		return &ir.Ref{At: at, Decl: d}
	}
	l.errorf(n, "undefined name %s", name)
	return &ir.Lit{At: at, Value: ir.Unit{}}
}

// typeName resolves an upper-case name in an expression: a variant, an
// entity, or a type (for its static methods).
func (l *lowerer) typeName(n *syntax.Node) ir.Expr {
	name := l.text(n)
	at := l.at(n)
	switch d := l.c.lookupType(l.m, name).(type) {
	case *ir.Variant, *ir.Entity, *ir.Type, *ir.Contract:
		l.c.use(at.Pos, d)
		l.visible(n, d, name)
		return &ir.Ref{At: at, Decl: d}
	case ambiguous:
		l.errorf(n, "%s is a variant in more than one imported module; qualify it with the module's name", name)
	default:
		l.errorf(n, "undefined name %s", name)
	}
	return &ir.Lit{At: at, Value: ir.Unit{}}
}

func (l *lowerer) stringLit(n *syntax.Node) ir.Expr {
	var parts []ir.Expr
	var b strings.Builder
	interpolated := false
	var texts map[*syntax.Node]string
	if len(n.Children) > 0 && l.text(n.Children[0]) == `"""` {
		texts = l.multilineText(n)
	}
	for _, c := range n.NamedChildren() {
		switch c.Kind {
		case syntax.StringText:
			if texts != nil {
				b.WriteString(texts[c])
			} else {
				b.WriteString(l.text(c))
			}
		case syntax.StringEscape:
			b.WriteString(unescape(l.text(c)))
		case syntax.Interpolation:
			interpolated = true
			if b.Len() > 0 {
				parts = append(parts, &ir.Lit{At: l.at(c), Value: b.String()})
				b.Reset()
			}
			parts = append(parts, l.expr(c.ChildByField(syntax.FieldValue)))
		}
	}
	if !interpolated {
		return &ir.Lit{At: l.at(n), Value: b.String()}
	}
	if b.Len() > 0 {
		parts = append(parts, &ir.Lit{At: l.at(n), Value: b.String()})
	}
	return &ir.Interpolate{At: l.at(n), Parts: parts}
}

// multilineText strips the margin from a multi-line string's text. The
// string starts on the line after its opening `"""` and ends at the line
// break before its closing `"""`, whose indentation is the margin: every line
// starts with it, or is blank, and loses it. It returns each text part of the
// string as it is after stripping.
func (l *lowerer) multilineText(n *syntax.Node) map[*syntax.Node]string {
	children := n.NamedChildren()
	out := map[*syntax.Node]string{}
	errorAt := func(offset int, format string, args ...any) {
		l.c.errorf(ir.Pos{File: l.f, Offset: offset, End: offset + 1}, format, args...)
	}
	first, last := children, children
	if len(first) == 0 || first[0].Kind != syntax.StringText {
		errorAt(n.Span.Start, `a multi-line string starts on the line after its """`)
		return nil
	}
	head := l.text(first[0])
	nl := strings.IndexByte(head, '\n')
	if nl < 0 || strings.TrimLeft(head[:nl], " \t\r") != "" {
		errorAt(n.Span.Start, `a multi-line string starts on the line after its """`)
		return nil
	}
	tail := last[len(last)-1]
	closing := l.text(tail)
	if tail.Kind != syntax.StringText || strings.LastIndexByte(closing, '\n') < 0 ||
		strings.Trim(closing[strings.LastIndexByte(closing, '\n')+1:], " \t") != "" {
		errorAt(n.Span.End-3, `the closing """ of a multi-line string goes on a line of its own`)
		return nil
	}
	margin := closing[strings.LastIndexByte(closing, '\n')+1:]
	atLineStart := true
	for i, c := range children {
		if c.Kind != syntax.StringText {
			if atLineStart && margin != "" {
				errorAt(c.Span.Start, `this line is indented less than the closing """`)
			}
			atLineStart = false
			continue
		}
		text := l.text(c)
		start := c.Span.Start
		if i == 0 {
			text, start = text[nl+1:], start+nl+1
		}
		if c == tail {
			cut := strings.LastIndexByte(text, '\n')
			if cut < 0 {
				// The string is empty: only the closing line is left.
				out[c] = ""
				continue
			}
			text = text[:cut]
		}
		var b strings.Builder
		for pos := 0; pos < len(text); {
			if atLineStart {
				end := strings.IndexByte(text[pos:], '\n')
				line := text[pos:]
				if end >= 0 {
					line = text[pos : pos+end]
				}
				switch {
				case strings.TrimRight(line, " \t\r") == "" && (end >= 0 || c == tail):
					pos += len(line) // a blank line keeps only its line break
				case strings.HasPrefix(line, margin):
					pos += len(margin)
				default:
					errorAt(start+pos, `this line is indented less than the closing """`)
				}
				atLineStart = false
				continue
			}
			end := strings.IndexByte(text[pos:], '\n')
			if end < 0 {
				b.WriteString(text[pos:])
				break
			}
			b.WriteString(text[pos : pos+end+1])
			pos += end + 1
			atLineStart = true
		}
		out[c] = b.String()
	}
	return out
}

func (l *lowerer) binary(n *syntax.Node) ir.Expr {
	op := l.text(n.ChildByField(syntax.FieldOperator))
	left, right := n.ChildByField(syntax.FieldLeft), n.ChildByField(syntax.FieldRight)
	at := l.at(n)
	switch op {
	case "|>":
		return l.pipe(n, left, right)
	case "??":
		return &ir.Default{At: at, X: l.expr(left), Or: l.expr(right)}
	}
	return &ir.Binary{At: at, Op: op, L: l.expr(left), R: l.expr(right)}
}

// pipe lowers `x |> f(a)` to `f(x, a)`, or to `f(a, x)` when `_` marks
// where x goes. `x |> f` calls f with x.
func (l *lowerer) pipe(n, left, right *syntax.Node) ir.Expr {
	// `x |> f(a)?` applies `?` to the call with x, not to f(a).
	if right.Kind == syntax.TryExpression {
		inner := l.pipe(n, left, right.ChildByField(syntax.FieldValue))
		return &ir.Try{At: l.at(right), X: inner, Wrap: l.method().wrap}
	}
	value := l.expr(left)
	piped := ir.Arg{Value: value}
	switch right.Kind {
	case syntax.CallExpression, syntax.WaitingCallExpression:
		var args []ir.Arg
		placed := false
		for _, a := range right.ChildByField(syntax.FieldArguments).NamedChildren() {
			if a.Kind == syntax.Placeholder {
				args = append(args, piped)
				placed = true
				continue
			}
			args = append(args, l.arg(a))
		}
		if !placed {
			args = append([]ir.Arg{piped}, args...)
		}
		waiting := right.Kind == syntax.WaitingCallExpression
		call := l.pipedCall(right, right.ChildByField(syntax.FieldFunction), args, waiting)
		if waiting {
			if ok, why := l.canWait(); !ok {
				l.errorf(right, "%s", why)
			}
			return &ir.Waiting{At: l.at(n), Call: call}
		}
		return call
	}
	return l.pipedCall(n, right, []ir.Arg{piped}, false)
}

func (l *lowerer) args(n *syntax.Node) []ir.Arg {
	var out []ir.Arg
	for _, a := range n.NamedChildren() {
		out = append(out, l.arg(a))
	}
	return out
}

func (l *lowerer) arg(a *syntax.Node) ir.Arg {
	switch a.Kind {
	case syntax.NamedArgument:
		name := a.ChildByField(syntax.FieldName)
		value := a.ChildByField(syntax.FieldValue)
		if value == nil {
			// `key:` alone passes the variable named key.
			if n := l.text(name); n == "in" || n == "for" {
				l.errorf(a, "%s is a keyword, not a variable, so %s: needs a value", n, n)
				return ir.Arg{Name: n, Value: &ir.Lit{At: l.at(a), Value: ir.Unit{}}}
			}
			return ir.Arg{Name: l.text(name), Value: l.name(name)}
		}
		arg := l.arg(value)
		arg.Name = l.text(name)
		return arg
	case syntax.MutArgument:
		v := a.ChildByField(syntax.FieldValue)
		place, _ := l.place(v)
		return ir.Arg{Value: l.expr(v), Mut: true, Place: place}
	case syntax.Placeholder:
		l.errorf(a, "_ stands for the piped value, so it works only in a call after |>")
	}
	return ir.Arg{Value: l.expr(a)}
}

// call lowers a call of fnNode with already lowered arguments. A callee
// `x.name` is a method call, unless x is a module or a type, in which case it
// names a function.
func (l *lowerer) call(n, fnNode *syntax.Node, args []ir.Arg, waiting bool) ir.Expr {
	e := l.callee(n, fnNode, args, waiting)
	l.c.argsAt(e, l.f, n.ChildByField(syntax.FieldArguments))
	return e
}

func (l *lowerer) callee(n, fnNode *syntax.Node, args []ir.Arg, waiting bool) ir.Expr {
	at := l.at(n)
	if fnNode.Kind == syntax.FieldExpression {
		recvNode := fnNode.ChildByField(syntax.FieldValue)
		name := l.text(fnNode.ChildByField(syntax.FieldField))
		if decl := l.qualified(recvNode, name); decl != nil {
			l.c.use(l.pos(fnNode.ChildByField(syntax.FieldField)), decl)
			l.visible(fnNode.ChildByField(syntax.FieldField), decl, name)
			l.checkDurable(n, decl, waiting)
			return &ir.Call{At: at, Func: &ir.Ref{At: l.at(fnNode), Decl: decl}, Args: args}
		}
		mc := &ir.MethodCall{At: at, Recv: l.expr(recvNode), Name: name, Args: args, RecvPlace: l.quietPlace(recvNode)}
		l.c.nameAt(mc, l.pos(fnNode.ChildByField(syntax.FieldField)))
		return mc
	}
	if fnNode.Kind == syntax.Ident && l.ownMethod(fnNode) {
		// In an entity, a method is called by its name, as a state field
		// is read by its name: `unpin(p)` is `self.unpin(p)`.
		l.c.use(l.pos(fnNode), l.method().entity.Methods[l.text(fnNode)])
		return &ir.MethodCall{At: at, Recv: &ir.Self{At: at}, Name: l.text(fnNode), Args: args}
	}
	callee := l.expr(fnNode)
	if r, ok := callee.(*ir.Ref); ok {
		l.checkDurable(n, r.Decl, waiting)
	}
	return &ir.Call{At: at, Func: callee, Args: args}
}

// pipedCall lowers the call a pipe makes. A pipe calls a function, never a
// method (SPECS.md §8), so in an entity `shapes |> add(s)` is the function
// add even where the entity has a method of that name.
func (l *lowerer) pipedCall(n, fnNode *syntax.Node, args []ir.Arg, waiting bool) ir.Expr {
	if fnNode.Kind != syntax.Ident || !l.ownMethod(fnNode) {
		return l.call(n, fnNode, args, waiting)
	}
	name := l.text(fnNode)
	d := l.c.lookupValue(l.m, name)
	if d == nil {
		l.errorf(fnNode, "%s is a method of this entity, and a pipe calls a function; write %s(…) to call the method", name, name)
		return &ir.Lit{At: l.at(n), Value: ir.Unit{}}
	}
	l.checkDurable(n, d, waiting)
	l.c.use(l.pos(fnNode), d)
	e := &ir.Call{At: l.at(n), Func: &ir.Ref{At: l.at(fnNode), Decl: d}, Args: args}
	l.c.argsAt(e, l.f, n.ChildByField(syntax.FieldArguments))
	return e
}

// ownMethod reports whether n names a method of the entity whose method is
// being lowered, and nothing nearer: a local or a state field.
func (l *lowerer) ownMethod(n *syntax.Node) bool {
	name := l.text(n)
	m := l.method()
	if m.entity == nil || l != m {
		return false
	}
	if _, ok := l.lookupLocal(name, n); ok || l.selfField(name) {
		return false
	}
	_, ok := m.entity.Methods[name]
	return ok
}

// checkDurable reports a call that must be a durable step in a job but has no `!`.
func (l *lowerer) checkDurable(n *syntax.Node, decl any, waiting bool) {
	if waiting || l.method().kind != ctxJob {
		return
	}
	switch d := decl.(type) {
	case ir.Builtin:
		if d.Name == "sleep" {
			l.errorf(n, "in a job, sleep is a durable step: write sleep!(…)")
		}
	case *ir.Op:
		l.errorf(n, "in a job, a call to another service is a durable step: write %s!(…)", d.Name)
	}
}

// visible reports a use of another module's declaration that it doesn't
// make public.
func (l *lowerer) visible(at *syntax.Node, decl any, name string) {
	if msg := l.c.privateUse(l.m.mod, decl, name); msg != "" {
		l.errorf(at, "%s", msg)
	}
}

// qualified resolves `module.name` and `Type.name` to a declaration, or
// returns nil when x.name is a field or method of a value.
func (l *lowerer) qualified(recv *syntax.Node, name string) any {
	switch recv.Kind {
	case syntax.Ident:
		recvName := l.text(recv)
		if _, ok := l.lookupLocal(recvName, recv); ok || l.selfField(recvName) {
			return nil
		}
		switch d := l.c.lookupValue(l.m, recvName).(type) {
		case *ir.Module:
			l.c.use(l.pos(recv), d)
			mod := l.c.modules[d.Path]
			if decl, ok := mod.values[name]; ok {
				return decl
			}
			if decl, ok := mod.types[name]; ok {
				return decl
			}
			l.errorf(recv, "module %s has no %s", recvName, name)
			return ir.Builtin{Name: "?"}
		case ir.Builtin:
			if fns, ok := ir.BuiltinModules[d.Name]; ok {
				l.c.use(l.pos(recv), GoText{File: "ir/ir.go", Text: `"` + d.Name + `": {`})
				for _, f := range fns {
					if f == name {
						return ir.Builtin{Name: d.Name + "." + name}
					}
				}
				l.errorf(recv, "module %s has no %s", recvName, name)
				return ir.Builtin{Name: "?"}
			}
		}
	case syntax.TypeIdent:
		switch d := l.c.lookupType(l.m, l.text(recv)).(type) {
		case *ir.Type:
			if fn, ok := d.Methods[name]; ok {
				l.c.use(l.pos(recv), d)
				return fn
			}
		case *ir.Variant:
			if fn, ok := d.Type.Methods[name]; ok {
				l.c.use(l.pos(recv), d)
				return fn
			}
		}
	}
	return nil
}

func (l *lowerer) field(n *syntax.Node) ir.Expr {
	recv := n.ChildByField(syntax.FieldValue)
	name := l.text(n.ChildByField(syntax.FieldField))
	if decl := l.qualified(recv, name); decl != nil {
		l.c.use(l.pos(n.ChildByField(syntax.FieldField)), decl)
		l.visible(n.ChildByField(syntax.FieldField), decl, name)
		return &ir.Ref{At: l.at(n), Decl: decl}
	}
	f := &ir.Field{At: l.at(n), Value: l.expr(recv), Name: name}
	l.c.nameAt(f, l.pos(n.ChildByField(syntax.FieldField)))
	return f
}

func (l *lowerer) match(n *syntax.Node) ir.Expr {
	m := &ir.Match{At: l.at(n), Value: l.expr(n.ChildByField(syntax.FieldValue))}
	for _, arm := range n.ChildByField(syntax.FieldBody).NamedChildren() {
		l.push()
		a := ir.Arm{Pattern: l.pattern(arm.ChildByField(syntax.FieldPattern), map[string]int{})}
		if g := arm.ChildByField(syntax.FieldGuard); g != nil {
			a.Guard = l.expr(g)
		}
		a.Body = l.expr(arm.ChildByField(syntax.FieldValue))
		l.pop()
		m.Arms = append(m.Arms, a)
	}
	return m
}

func (l *lowerer) lambda(n *syntax.Node) ir.Expr {
	inner := &lowerer{
		c: l.c, m: l.m, f: l.f, kind: ctxLambda, parent: l,
		fn:     &ir.Func{Name: "lambda", Module: l.m.mod, Pos: l.pos(n)},
		scopes: []map[string]local{{}},
	}
	if p := n.ChildByField(syntax.FieldParameter); p != nil {
		param := &ir.Param{Name: l.text(p)}
		if p.Kind == syntax.Wildcard {
			param.Slot = inner.newSlot()
		} else {
			param.Slot = inner.bind(p, param.Name, false, false)
		}
		inner.fn.Params = append(inner.fn.Params, param)
	}
	if ps := n.ChildByField(syntax.FieldParameters); ps != nil {
		for _, p := range ps.NamedChildren() {
			nameNode := p.ChildByField(syntax.FieldName)
			param := &ir.Param{Name: l.text(nameNode), Mut: hasToken(l.f, p, "mut")}
			if nameNode.Kind == syntax.Wildcard {
				param.Slot = inner.newSlot()
			} else {
				param.Slot = inner.bind(nameNode, param.Name, param.Mut, false)
			}
			inner.fn.Params = append(inner.fn.Params, param)
		}
	}
	inner.fn.Body = inner.expr(n.ChildByField(syntax.FieldBody))
	return &ir.Lambda{At: l.at(n), Func: inner.fn}
}

// ---------------------------------------------------------------- patterns

// pattern lowers a pattern, binding its names in the current scope. binds
// holds the names bound so far, so the alternatives of an or-pattern bind
// the same slots.
func (l *lowerer) pattern(n *syntax.Node, binds map[string]int) ir.Pattern {
	switch n.Kind {
	case syntax.Ident:
		name := l.text(n)
		if slot, ok := binds[name]; ok {
			return ir.PBind{Name: name, Slot: slot}
		}
		slot := l.bind(n, name, false, false)
		binds[name] = slot
		return ir.PBind{Name: name, Slot: slot}
	case syntax.Wildcard:
		return ir.PWild{}
	case syntax.Int:
		return ir.PLit{Value: parseInt(l.text(n))}
	case syntax.Float:
		f, _ := strconv.ParseFloat(strings.ReplaceAll(l.text(n), "_", ""), 64)
		return ir.PLit{Value: f}
	case syntax.NegativeLiteral:
		inner := l.pattern(n.NamedChildren()[0], binds).(ir.PLit)
		switch v := inner.Value.(type) {
		case *big.Int:
			return ir.PLit{Value: new(big.Int).Neg(v)}
		case float64:
			return ir.PLit{Value: -v}
		}
		return inner
	case syntax.Char:
		return ir.PLit{Value: parseChar(l.text(n))}
	case syntax.String:
		lit, ok := l.stringLit(n).(*ir.Lit)
		if !ok {
			l.errorf(n, "a string pattern can't interpolate")
			return ir.PWild{}
		}
		return ir.PLit{Value: lit.Value}
	case syntax.Boolean:
		return ir.PLit{Value: l.text(n) == "true"}
	case syntax.Unit:
		return ir.PLit{Value: ir.Unit{}}
	case syntax.RangePattern:
		lo := l.pattern(n.ChildByField(syntax.FieldStart), binds).(ir.PLit)
		hi := l.pattern(n.ChildByField(syntax.FieldEnd), binds).(ir.PLit)
		return ir.PRange{Lo: lo.Value, Hi: hi.Value, Inclusive: l.text(n.ChildByField(syntax.FieldOperator)) == "..="}
	case syntax.VariantPattern:
		nameNode := n.ChildByField(syntax.FieldName)
		name := l.text(nameNode)
		p := ir.PVariant{}
		switch d := l.c.lookupType(l.m, name).(type) {
		case *ir.Variant:
			l.c.use(l.pos(nameNode), d)
			l.visible(nameNode, d, name)
			p.Variant = d
		case ambiguous:
			// Matched by name: the value decides which variant it is.
			p.Variant = &ir.Variant{Name: name}
		default:
			l.errorf(nameNode, "undefined variant %s", name)
			p.Variant = &ir.Variant{Name: name}
		}
		if args := n.ChildByField(syntax.FieldArguments); args != nil {
			for _, a := range args.NamedChildren() {
				switch a.Kind {
				case syntax.RestPattern:
					p.Rest = true
				case syntax.NamedFieldPattern:
					p.Names = append(p.Names, l.text(a.ChildByField(syntax.FieldName)))
					if v, ok := l.c.lookupType(l.m, name).(*ir.Variant); ok {
						l.c.use(l.pos(a.ChildByField(syntax.FieldName)), Member{Of: v, Name: l.text(a.ChildByField(syntax.FieldName))})
					}
					p.Args = append(p.Args, l.pattern(a.ChildByField(syntax.FieldPattern), binds))
				default:
					p.Names = append(p.Names, "")
					p.Args = append(p.Args, l.pattern(a, binds))
				}
			}
		}
		return p
	case syntax.TuplePattern:
		p := ir.PTuple{}
		for _, a := range n.NamedChildren() {
			p.Pats = append(p.Pats, l.pattern(a, binds))
		}
		return p
	case syntax.RecordPattern:
		p := ir.PRecord{}
		for _, a := range n.NamedChildren() {
			switch a.Kind {
			case syntax.RestPattern:
				p.Rest = true
			case syntax.ShorthandFieldPattern:
				p.Names = append(p.Names, l.text(a))
				p.Pats = append(p.Pats, l.pattern(a.NamedChildren()[0], binds))
			case syntax.NamedFieldPattern:
				p.Names = append(p.Names, l.text(a.ChildByField(syntax.FieldName)))
				p.Pats = append(p.Pats, l.pattern(a.ChildByField(syntax.FieldPattern), binds))
			}
		}
		if len(p.Names) == 0 && !p.Rest {
			return ir.PEmpty{}
		}
		return p
	case syntax.ListPattern:
		p := ir.PList{Rest: -1}
		for _, a := range n.NamedChildren() {
			if a.Kind == syntax.RestPattern {
				p.Rest = len(p.Pats)
				if name := a.ChildByField(syntax.FieldName); name != nil {
					p.RestPat = l.pattern(name, binds)
				}
				continue
			}
			p.Pats = append(p.Pats, l.pattern(a, binds))
		}
		return p
	case syntax.OrPattern:
		return ir.POr{Alts: []ir.Pattern{
			l.pattern(n.ChildByField(syntax.FieldLeft), binds),
			l.pattern(n.ChildByField(syntax.FieldRight), binds),
		}}
	}
	l.errorf(n, "%s isn't a pattern the interpreter supports yet", n.Kind)
	return ir.PWild{}
}

// ---------------------------------------------------------------- literals

func parseInt(s string) *big.Int {
	v, _ := new(big.Int).SetString(strings.ReplaceAll(s, "_", ""), 10)
	return v
}

func unescape(s string) string {
	switch s {
	case `\n`:
		return "\n"
	case `\r`:
		return "\r"
	case `\t`:
		return "\t"
	case `\0`:
		return "\x00"
	case `\\`, `\"`, `\'`, `\{`, `\}`:
		return s[1:]
	}
	if strings.HasPrefix(s, `\u{`) {
		r, err := strconv.ParseUint(s[3:len(s)-1], 16, 32)
		if err == nil {
			return string(rune(r))
		}
	}
	return s
}

func parseChar(s string) rune {
	inner := s[1 : len(s)-1]
	if strings.HasPrefix(inner, `\`) {
		inner = unescape(inner)
	}
	r, _ := utf8.DecodeRuneInString(inner)
	return r
}

var durationUnits = map[string]float64{
	"ms": 1e6, "s": 1e9, "min": 60e9, "h": 3600e9, "day": 86400e9, "days": 86400e9,
	"week": 7 * 86400e9, "weeks": 7 * 86400e9,
}

func durationOf(value, unit string) ir.Duration {
	f, _ := strconv.ParseFloat(strings.ReplaceAll(value, "_", ""), 64)
	return ir.Duration(f * durationUnits[unit])
}

var sizeUnits = map[string]float64{"B": 1, "KiB": 1 << 10, "MiB": 1 << 20, "GiB": 1 << 30}

func sizeOf(value, unit string) ir.Size {
	f, _ := strconv.ParseFloat(strings.ReplaceAll(value, "_", ""), 64)
	return ir.Size(f * sizeUnits[unit])
}

func parseDate(s string) ir.Date {
	y, _ := strconv.Atoi(s[0:4])
	m, _ := strconv.Atoi(s[5:7])
	d, _ := strconv.Atoi(s[8:10])
	return ir.Date{Year: y, Month: m, Day: d}
}

func parseTime(s string) ir.TimeOfDay {
	h, _ := strconv.Atoi(s[0:2])
	m, _ := strconv.Atoi(s[3:5])
	t := ir.TimeOfDay{Hour: h, Minute: m}
	if len(s) == 8 {
		t.Second, _ = strconv.Atoi(s[6:8])
	}
	return t
}
