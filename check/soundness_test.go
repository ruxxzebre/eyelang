package check

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"eyelang/ir"
)

// The tests in this file hold the checker to its own rules, so that what it
// accepts is what SPECS.md says is right.

// TestUnknownHasOneMaker checks that unknown, the type that agrees with
// every type, is made only by unknownf, which reports why. A checker that
// could make it quietly would accept wrong code without a word.
func TestUnknownHasOneMaker(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	makers := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fn, _ := d.(*ast.FuncDecl)
			ast.Inspect(d, func(n ast.Node) bool {
				var bad ast.Node
				switch n := n.(type) {
				case *ast.CompositeLit:
					if isIdent(n.Type, "tunknown") {
						if fn != nil && fn.Name.Name == "unknownf" {
							makers++
							return true
						}
						bad = n
					}
				case *ast.ValueSpec:
					if isIdent(n.Type, "tunknown") {
						bad = n
					}
				case *ast.CallExpr:
					if isIdent(n.Fun, "tunknown") || isIdent(n.Fun, "new") && len(n.Args) == 1 && isIdent(n.Args[0], "tunknown") {
						bad = n
					}
				}
				if bad != nil {
					t.Errorf("%s: makes an unknown type without reporting why: call c.unknownf(at, …) instead", fset.Position(bad.Pos()))
				}
				return true
			})
		}
	}
	if makers != 1 {
		t.Errorf("unknownf makes unknown %d times, want once", makers)
	}
}

func isIdent(e ast.Expr, name string) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == name
}

// TestBuiltinMethodArgs checks that a named argument of a built-in called
// as a method, as in s.split(separator: ","), leads to the built-in, as it
// does when the built-in is called as a function.
func TestBuiltinMethodArgs(t *testing.T) {
	dir := t.TempDir()
	src := "fn parts(s: String) -> List(String) = s.split(separator: \",\")\n"
	path := filepath.Join(dir, "main.eye")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Check(dir, Options{})
	for _, d := range res.Diagnostics {
		t.Error(d)
	}
	if _, ok := res.Index.At(path, strings.Index(src, "separator")); !ok {
		t.Error("separator: in s.split(separator: …) leads nowhere")
	}
}

// programs are the directories of every program in the repository: the
// lab, the test programs, the tour and the documentation's examples.
func programs(t *testing.T) []string {
	t.Helper()
	var dirs []string
	for _, root := range []string{"../testdata", "../docs"} {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && (d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			if d.Name() == "eye.toml" {
				dirs = append(dirs, filepath.Dir(path))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(dirs) < 20 {
		t.Fatalf("found only %d programs", len(dirs))
	}
	return dirs
}

// TestProgramsAreFullyTyped checks that every expression of every program
// that checks has a type the checker knows to the end: nothing unknown, and
// no type variable that nothing decided, other than a generic function's
// own. This is what a compiler needs, and what typed IR (7.1) starts from.
func TestProgramsAreFullyTyped(t *testing.T) {
	for _, dir := range programs(t) {
		res := Check(dir, Options{All: true})
		if len(res.Diagnostics) > 0 {
			t.Errorf("%s: %s", dir, res.Diagnostics[0])
			continue
		}
		for _, e := range res.Index.types {
			if what := undecided(e.t); what != "" {
				t.Errorf("%s: %s is %s, which has %s", e.at, e.at.Text(), show(e.t), what)
			}
		}
	}
}

// TestTypedIR checks that every expression of every program that checks
// has its type in the IR (7.1): the interpreter and the compiler read it
// there.
func TestTypedIR(t *testing.T) {
	for _, dir := range programs(t) {
		res := Check(dir, Options{All: true})
		if len(res.Diagnostics) > 0 {
			continue // TestProgramsAreFullyTyped reports it
		}
		missing := 0
		for _, m := range res.Program.Modules {
			for _, f := range Bodies(m) {
				walk(f.Body, false, func(e ir.Expr, _ bool) bool {
					if e.Ty() == nil {
						if missing < 5 {
							t.Errorf("%s: %s has no type in the IR", e.Position(), e.Position().Text())
						}
						missing++
					}
					return true
				})
			}
		}
		if missing >= 5 {
			t.Errorf("%s: %d expressions have no type in the IR", dir, missing)
		}
	}
}

// undecided names what in t isn't known: an unknown part, or a type
// variable nothing bound. It returns "" for a type known to the end.
func undecided(t typ) string {
	switch x := defaultLiteral(t).(type) {
	case tunknown:
		return "an unknown type"
	case *tvar:
		if !x.rigid && !x.never {
			return "a type nothing decided"
		}
	case *tcon:
		for _, a := range x.args {
			if s := undecided(a); s != "" {
				return s
			}
		}
	case *ttuple:
		for _, e := range x.elems {
			if s := undecided(e); s != "" {
				return s
			}
		}
	case *tfun:
		for _, p := range x.params {
			if s := undecided(p.t); s != "" {
				return s
			}
		}
		return undecided(x.ret)
	case *tdyn:
		for _, a := range x.args {
			if s := undecided(a); s != "" {
				return s
			}
		}
	}
	return ""
}

// mutation is a small change to a program that must make it wrong.
type mutation struct {
	what string // the change, for the message
	at   ir.Pos
	src  []byte // the file with the change
}

// TestMutationsAreCaught makes small mistakes in every program of the
// repository and checks that eye check reports each: a field or method
// renamed to one that doesn't exist, a variable renamed, and two
// arguments of different types swapped. A mistake the checker misses is a
// hole in it. Each kind is sampled evenly through each program.
func TestMutationsAreCaught(t *testing.T) {
	const perKind = 15
	for _, dir := range programs(t) {
		t.Run(strings.TrimPrefix(dir, "../"), func(t *testing.T) {
			t.Parallel()
			res := Check(dir, Options{All: true})
			if len(res.Diagnostics) > 0 {
				t.Skipf("doesn't check: %s", res.Diagnostics[0])
			}
			var muts []mutation
			for _, kind := range [][]mutation{renamedMembers(res, dir), renamedLocals(res, dir), swappedArgs(res, dir)} {
				muts = append(muts, sample(kind, perKind)...)
			}
			for _, m := range muts {
				got := Check(dir, Options{All: true, Overlay: map[string][]byte{m.at.File.Path: m.src}})
				if len(got.Diagnostics) == 0 {
					t.Errorf("%s: %s, and the program still checks", m.at, m.what)
				}
			}
		})
	}
}

// sample takes at most n of muts, spread evenly.
func sample(muts []mutation, n int) []mutation {
	if len(muts) <= n {
		return muts
	}
	out := make([]mutation, n)
	for i := range out {
		out[i] = muts[i*len(muts)/n]
	}
	return out
}

// own reports whether at is in one of the program's own files.
func own(at ir.Pos, dir string) bool {
	return at.File != nil && strings.HasPrefix(at.File.Path, dir+string(filepath.Separator)) && at.End > at.Offset
}

// replaced is the file at's span is in, with the span replaced by text.
func replaced(at ir.Pos, text string) []byte {
	src := at.File.Src
	out := append([]byte{}, src[:at.Offset]...)
	out = append(out, text...)
	return append(out, src[at.End:]...)
}

// renamedMembers renames each field read and method called to a name its
// type doesn't have.
func renamedMembers(res *Result, dir string) []mutation {
	types := map[ir.Pos]typ{}
	for _, e := range res.Index.types {
		types[e.at] = e.t
	}
	var out []mutation
	for e, at := range res.c.names.name {
		if !own(at, dir) {
			continue
		}
		if f, ok := e.(*ir.Field); ok {
			// The rows of an entity's oldest version were written by code
			// that is gone, so no one knows their fields.
			if k, ok := prune(types[f.Value.Position()]).(*tcon); ok && k.decl != nil {
				if r := res.c.ts.rowOf[k.decl]; r != nil && !r.made {
					continue
				}
			}
		}
		kind := "field"
		if _, ok := e.(*ir.MethodCall); ok {
			kind = "method"
		}
		out = append(out, mutation{what: "the " + kind + " " + at.Text() + " is renamed", at: at, src: replaced(at, at.Text()+"Qz")})
	}
	sortMutations(out)
	return out
}

// renamedLocals renames each use of a variable to a name nothing declares.
func renamedLocals(res *Result, dir string) []mutation {
	var out []mutation
	eachBody(res, dir, func(e ir.Expr) {
		if l, ok := e.(*ir.Local); ok && own(l.Pos, dir) && l.Pos.Text() == l.Name {
			out = append(out, mutation{what: "the variable " + l.Name + " is renamed", at: l.Pos, src: replaced(l.Pos, l.Name+"Qz")})
		}
	})
	sortMutations(out)
	return out
}

// swappedArgs swaps two arguments of a call of a function whose parameters
// have different types, written in full, when the arguments' own types
// differ too.
func swappedArgs(res *Result, dir string) []mutation {
	types := map[ir.Pos]typ{}
	for _, e := range res.Index.types {
		types[e.at] = e.t
	}
	var out []mutation
	eachBody(res, dir, func(e ir.Expr) {
		call, ok := e.(*ir.Call)
		if !ok || len(call.Args) < 2 {
			return
		}
		r, ok := call.Func.(*ir.Ref)
		if !ok {
			return
		}
		fn, ok := r.Decl.(*ir.Func)
		if !ok || len(fn.Params) < len(call.Args) {
			return
		}
		for i, p := range fn.Params[:len(call.Args)] {
			vars := map[string]bool{}
			collectVars(p.Type, vars)
			a := call.Args[i]
			if p.Type == nil || len(vars) > 0 || a.Name != "" || a.Mut || !own(a.Value.Position(), dir) {
				return
			}
			switch a.Value.(type) {
			case *ir.Lit, *ir.Interpolate, *ir.Lambda, *ir.Record, *ir.EmptyBraces, *ir.List, *ir.Map:
				return // literals may take more than one type
			}
		}
		a, b := call.Args[0].Value.Position(), call.Args[1].Value.Position()
		ta, tb := types[a], types[b]
		if ta == nil || tb == nil || show(ta) == show(tb) || a.File != b.File || a.End > b.Offset ||
			typeExprText(fn.Params[0].Type) == typeExprText(fn.Params[1].Type) {
			return
		}
		src := a.File.Src
		var s []byte
		s = append(s, src[:a.Offset]...)
		s = append(s, src[b.Offset:b.End]...)
		s = append(s, src[a.End:b.Offset]...)
		s = append(s, src[a.Offset:a.End]...)
		s = append(s, src[b.End:]...)
		out = append(out, mutation{what: "the arguments " + a.Text() + " and " + b.Text() + " are swapped", at: a, src: s})
	})
	sortMutations(out)
	return out
}

// eachBody calls visit for every expression in the program's own bodies.
func eachBody(res *Result, dir string, visit func(ir.Expr)) {
	for _, m := range res.Program.Modules {
		for _, f := range res.c.bodiesOf(m) {
			walk(f.Body, false, func(e ir.Expr, _ bool) bool {
				visit(e)
				return true
			})
		}
	}
}

func sortMutations(ms []mutation) {
	sort.Slice(ms, func(i, j int) bool {
		a, b := ms[i].at, ms[j].at
		if a.File.Path != b.File.Path {
			return a.File.Path < b.File.Path
		}
		return a.Offset < b.Offset
	})
}
