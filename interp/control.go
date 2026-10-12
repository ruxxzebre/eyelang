package interp

import (
	"sync"

	"eyelang/ir"
)

// How return, break, continue and `?` leave.
//
// Most of them leave by a flag in the frame: the expression sets f.ctl
// (and f.ret, for a return) and gives nil, and each expression it is in
// sees the flag and gives nil too, up to the loop or the function that
// takes it. That is cheap, but it works only where every expression on the
// way up checks the flag. Those are the statement positions: a block's
// statements, an if's branches, a match's value and arms, a let's value and
// else block, a loop's body, and a return's value. A return, break,
// continue or `?` anywhere else, as in `a + (x ?? return 0)`, leaves by a
// panic, which the function's runBody or the loop catches, as all of them
// did before.
//
// ctlInfo says which is which for one function, found once by walking its
// body. The checks of f.ctl in eval, evalMatch, letElse, the loops and the
// statements of a job's body (checkpoint.go) are the other half: every
// position ctlInfo.walk goes into must check the flag there.

// ctl is what a frame's flag says is leaving.
type ctl uint8

const (
	ctlNone ctl = iota
	ctlReturn
	ctlBreak
	ctlContinue
)

// ctlInfo is what a function's body does with return, break, continue and
// `?`.
type ctlInfo struct {
	// flag holds the returns, breaks, continues and `?`s in statement
	// positions, which leave by the frame's flag.
	flag map[ir.Expr]bool
	// catch holds the loops whose body may break or continue by a panic,
	// which then run each turn under a recover.
	catch map[ir.Expr]bool
}

// funcCache holds what the interpreter works out once per function. The
// interpreters of one program, a task's included, share it.
type funcCache struct {
	ctl      sync.Map // *ir.Func → *ctlInfo
	closures sync.Map // *ir.Func → *Closure, the function as a value
	// names are what a saved function value calls its code, worked out
	// once (save.go).
	namesOnce sync.Once
	names     map[*ir.Func]string
	byName    map[string]*ir.Func
}

// closure returns fn, a named function, as a value.
func (in *Interp) closure(fn *ir.Func) *Closure {
	if c, ok := in.funcs.closures.Load(fn); ok {
		return c.(*Closure)
	}
	c, _ := in.funcs.closures.LoadOrStore(fn, &Closure{Fn: fn})
	return c.(*Closure)
}

// ctlOf returns what f's function does with return, break and continue.
func (in *Interp) ctlOf(f *frame) *ctlInfo {
	if f.info != nil {
		return f.info
	}
	if c, ok := in.funcs.ctl.Load(f.fn); ok {
		f.info = c.(*ctlInfo)
		return f.info
	}
	c := &ctlInfo{flag: map[ir.Expr]bool{}, catch: map[ir.Expr]bool{}}
	if f.fn.Body != nil {
		c.walk(f.fn.Body, false)
		c.scan(f.fn.Body, nil)
	}
	got, _ := in.funcs.ctl.LoadOrStore(f.fn, c)
	f.info = got.(*ctlInfo)
	return f.info
}

// walk finds the returns, breaks, continues and `?`s in statement positions
// of e, which is in one; loop says e is in a loop's body.
func (c *ctlInfo) walk(e ir.Expr, loop bool) {
	switch x := e.(type) {
	case *ir.Block:
		for _, s := range x.Exprs {
			c.walk(s, loop)
		}
	case *ir.If:
		c.walk(x.Then, loop)
		if x.Else != nil {
			c.walk(x.Else, loop)
		}
	case *ir.Match:
		c.walk(x.Value, loop)
		for _, arm := range x.Arms {
			c.walk(arm.Body, loop)
		}
	case *ir.Let:
		c.walk(x.Value, loop)
		if x.Else != nil {
			c.walk(x.Else, loop)
		}
	case *ir.While:
		c.walk(x.Body, true)
	case *ir.For:
		c.walk(x.Body, true)
	case *ir.Return:
		c.flag[x] = true
		if x.Value != nil {
			c.walk(x.Value, loop)
		}
	case *ir.Try:
		c.flag[x] = true
	case *ir.Break, *ir.Continue:
		if loop {
			c.flag[x] = true
		}
	}
}

// scan finds the loops whose body holds a break or continue that leaves by
// a panic. loop is the loop e is in the body of, nil outside any.
func (c *ctlInfo) scan(e ir.Expr, loop ir.Expr) {
	if e == nil {
		return
	}
	switch x := e.(type) {
	case *ir.Break, *ir.Continue:
		if loop != nil && !c.flag[x] {
			c.catch[loop] = true
		}
		return
	case *ir.While:
		c.scan(x.Cond, loop)
		c.scan(x.Body, x)
		return
	case *ir.For:
		c.scan(x.Iter, loop)
		c.scan(x.Body, x)
		return
	case *ir.Lambda:
		return // its body is a function of its own
	}
	kids, known := children(e)
	if !known && loop != nil {
		c.catch[loop] = true // an expression this doesn't know: be safe
	}
	for _, k := range kids {
		c.scan(k, loop)
	}
}

// children lists the expressions directly inside e, other than a loop's
// and a lambda's, which scan handles. known is false for an expression it
// doesn't know.
func children(e ir.Expr) (kids []ir.Expr, known bool) {
	args := func(as []ir.Arg) {
		for _, a := range as {
			kids = append(kids, a.Value)
		}
	}
	switch x := e.(type) {
	case *ir.Lit, *ir.EmptyBraces, *ir.Local, *ir.Self, *ir.SelfField, *ir.Ref, *ir.Break, *ir.Continue:
	case *ir.Call:
		kids = append(kids, x.Func)
		args(x.Args)
	case *ir.MethodCall:
		kids = append(kids, x.Recv)
		args(x.Args)
		if fc, ok := x.Target.(ir.FieldCall); ok {
			kids = append(kids, fc.Call)
		}
	case *ir.Waiting:
		kids = append(kids, x.Call)
	case *ir.Field:
		kids = append(kids, x.Value)
	case *ir.Index:
		kids = append(kids, x.Value, x.Index)
	case *ir.Unary:
		kids = append(kids, x.X)
	case *ir.Binary:
		kids = append(kids, x.L, x.R)
	case *ir.If:
		kids = append(kids, x.Cond, x.Then, x.Else)
	case *ir.Match:
		kids = append(kids, x.Value)
		for _, arm := range x.Arms {
			kids = append(kids, arm.Guard, arm.Body)
		}
	case *ir.Block:
		kids = append(kids, x.Exprs...)
	case *ir.Let:
		kids = append(kids, x.Value, x.Else)
	case *ir.Assign:
		kids = append(kids, x.Value)
	case *ir.Return:
		kids = append(kids, x.Value)
	case *ir.Record:
		kids = append(kids, x.Values...)
	case *ir.RecordUpdate:
		kids = append(kids, x.Base)
		kids = append(kids, x.Values...)
	case *ir.List:
		kids = append(kids, x.Elems...)
	case *ir.Map:
		kids = append(kids, x.Keys...)
		kids = append(kids, x.Values...)
	case *ir.Tuple:
		kids = append(kids, x.Elems...)
	case *ir.InZone:
		kids = append(kids, x.Zone)
	case *ir.Interpolate:
		kids = append(kids, x.Parts...)
	case *ir.Try:
		kids = append(kids, x.X)
	case *ir.Default:
		kids = append(kids, x.X, x.Or)
	case *ir.Range:
		kids = append(kids, x.Start, x.End)
	case *ir.With:
		kids = append(kids, x.Value)
	default:
		return nil, false
	}
	return kids, true
}

// leave makes x, a return, break, continue or `?`, leave with v: by the
// frame's flag when x is in a statement position, by a panic otherwise.
func (in *Interp) leave(f *frame, x ir.Expr, how ctl, v Value) Value {
	if in.ctlOf(f).flag[x] {
		f.ctl, f.ret = how, v
		return nil
	}
	switch how {
	case ctlBreak:
		panic(breakSignal{})
	case ctlContinue:
		panic(continueSignal{})
	}
	panic(returnSignal{v})
}

// returned takes a return the frame's flag holds, when its function's body
// has finished: the value it returned, or v if none did.
func returned(f *frame, v Value) Value {
	if f.ctl == ctlReturn {
		v = f.ret
		f.ctl, f.ret = ctlNone, nil
	}
	return v
}

// loopBody runs one turn of a loop's body, and reports whether the loop
// should stop. catch runs it under a recover, for a body that may break or
// continue by a panic (ctlInfo.catch).
func (in *Interp) loopBody(f *frame, body ir.Expr, catch bool) bool {
	if catch {
		if caught(func() { in.eval(f, body) }) {
			return true
		}
	} else {
		in.eval(f, body)
	}
	return loopFlag(f)
}

// loopFlag takes a break or continue the frame's flag holds, at the end of
// a loop's turn, and reports whether the loop should stop: on a break, or
// on a return, whose flag it leaves set for the function.
func loopFlag(f *frame) bool {
	switch f.ctl {
	case ctlNone:
		return false
	case ctlBreak:
		f.ctl = ctlNone
		return true
	case ctlContinue:
		f.ctl = ctlNone
		return false
	}
	return true
}

// caught runs turn, catching a break or continue that leaves it by a panic,
// and reports whether a break stopped it.
func caught(turn func()) (stop bool) {
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case breakSignal:
				stop = true
			case continueSignal:
				stop = false
			default:
				panic(r)
			}
		}
	}()
	turn()
	return false
}
