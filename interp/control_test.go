package interp

import (
	"os"
	"path/filepath"
	"testing"

	"eyelang/check"
	"eyelang/ir"
)

// TestChildrenKnowsEveryExpression walks every function of the test
// programs and the lab: children must know each expression the checker
// makes, or a loop around one it doesn't know runs each turn under a
// recover for nothing.
func TestChildrenKnowsEveryExpression(t *testing.T) {
	dirs, _ := filepath.Glob("../testdata/programs/*")
	dirs = append(dirs, "../testdata/lab")
	unknown := map[string]bool{}
	var visit func(e ir.Expr)
	visit = func(e ir.Expr) {
		switch x := e.(type) {
		case nil:
			return
		case *ir.While:
			visit(x.Cond)
			visit(x.Body)
			return
		case *ir.For:
			visit(x.Iter)
			visit(x.Body)
			return
		case *ir.Lambda:
			visit(x.Func.Body)
			return
		}
		kids, known := children(e)
		if !known {
			unknown[e.Position().String()] = true
		}
		for _, k := range kids {
			visit(k)
		}
	}
	for _, dir := range dirs {
		if _, err := os.Stat(filepath.Join(dir, "eye.toml")); err != nil {
			continue
		}
		prog, diags := check.Program(dir)
		if len(diags) > 0 {
			t.Fatalf("%s: %v", dir, diags[0])
		}
		for _, fn := range allFuncs(prog) {
			visit(fn.Body)
		}
	}
	for at := range unknown {
		t.Errorf("%s: children doesn't know this expression", at)
	}
}

// allFuncs lists the functions a program declares.
func allFuncs(prog *ir.Program) []*ir.Func {
	var out []*ir.Func
	add := func(fns map[string]*ir.Func) {
		for _, fn := range fns {
			out = append(out, fn)
		}
	}
	for _, m := range prog.Modules {
		add(m.Funcs)
		for _, ty := range m.Types {
			add(ty.Methods)
		}
		for _, e := range m.Entities {
			add(e.Methods)
		}
		for _, impl := range m.Impls {
			add(impl.Methods)
		}
		for _, j := range m.Jobs {
			out = append(out, j.Func)
		}
		for _, v := range m.Views {
			out = append(out, v.Func)
		}
		for _, g := range m.Globals {
			out = append(out, g.Init)
		}
		for _, test := range m.Tests {
			out = append(out, test.Func)
		}
		if m.Main != nil {
			out = append(out, m.Main)
		}
	}
	return out
}

// TestCtlInfo checks which returns, breaks and `?`s leave by the frame's
// flag, and which loops catch the ones that leave by a panic.
func TestCtlInfo(t *testing.T) {
	prog := checkSource(t, `
fn sign(n: Int) -> Int {
  if n < 0 { return -1 }
  let m = match n {
    0 => return 0
    _ => n
  }
  m
}

fn plusOne(n: Option(Int)) -> Int = 1 + (n ?? return 0)

fn half(n: Int) -> Option(Int) = if n % 2 == 0 then Some(n / 2) else None

fn quarter(n: Int) -> Option(Int) {
  let h = half(n)?
  Some(half(h)? + 0)
}

fn loops() -> Int {
  var t = 0
  for i in 0..10 {
    if i == 5 { break }
    t += i
  }
  while true {
    t += (if t > 100 then break else 1)
  }
  t
}
`)
	count := func(name string) (flag, panics, catch int) {
		fn := prog.Root.Funcs[name]
		in := New(prog, nil, nil)
		c := in.ctlOf(&frame{fn: fn})
		var visit func(e ir.Expr)
		visit = func(e ir.Expr) {
			switch x := e.(type) {
			case nil:
				return
			case *ir.Return, *ir.Break, *ir.Continue, *ir.Try:
				if c.flag[e] {
					flag++
				} else {
					panics++
				}
			case *ir.While:
				if c.catch[x] {
					catch++
				}
				visit(x.Cond)
				visit(x.Body)
				return
			case *ir.For:
				if c.catch[x] {
					catch++
				}
				visit(x.Iter)
				visit(x.Body)
				return
			}
			kids, _ := children(e)
			for _, k := range kids {
				visit(k)
			}
		}
		visit(fn.Body)
		return flag, panics, catch
	}
	for _, tc := range []struct {
		name                string
		flag, panics, catch int
	}{
		{"sign", 2, 0, 0},
		{"plusOne", 0, 1, 0},
		{"quarter", 1, 1, 0},
		{"loops", 1, 1, 1},
	} {
		flag, panics, catch := count(tc.name)
		if flag != tc.flag || panics != tc.panics || catch != tc.catch {
			t.Errorf("%s: %d by the flag, %d by a panic, %d loops that catch; want %d, %d, %d",
				tc.name, flag, panics, catch, tc.flag, tc.panics, tc.catch)
		}
	}
}
