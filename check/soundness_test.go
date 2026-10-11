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
