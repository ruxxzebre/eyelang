package check

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
