package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"eyelang/ir"
)

func TestInvalidSyntaxReportsDiagnostics(t *testing.T) {
	files, err := filepath.Glob("../testdata/syntax/invalid/*.eye")
	if err != nil || len(files) == 0 {
		t.Fatalf("invalid syntax fixtures: %v", err)
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			dir := writeProgram(t, map[string]string{"main.eye": string(src)})
			if _, diags := Program(dir); len(diags) == 0 {
				t.Fatal("invalid syntax produced no diagnostics")
			}
		})
	}
}

// halfTyped is a module with one declaration half typed, beside others that
// check: one has a type error, and one calls the broken one.
const halfTyped = `fn broken(n: Int) -> Int {
  let x = n +
}

fn good() -> Int {
  let s: String = 3
  print(s)
  helper(2)
}

fn helper(k: Int) -> Int = broken(k)
`

// TestSyntaxErrorSkipsOnlyItsDeclaration checks that a syntax error in one
// declaration leaves the others checked: their type errors are reported,
// with the syntax error, a use of the broken declaration isn't undefined,
// and the index still knows the good declarations' names and types.
func TestSyntaxErrorSkipsOnlyItsDeclaration(t *testing.T) {
	dir := writeProgram(t, map[string]string{"main.eye": halfTyped})
	path := filepath.Join(dir, "main.eye")
	res := Check(dir, Options{})
	var got []string
	for _, d := range res.Diagnostics {
		got = append(got, d.String()[len(dir)+1:])
	}
	want := []string{
		"main.eye:2:14: expected an expression before the end of the line",
		"main.eye:6:19: expected String, found Int",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("diagnostics:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	at := strings.Index(halfTyped, "helper(2)")
	use, ok := res.Index.At(path, at)
	if fn, isFn := use.Target.(*ir.Func); !ok || !isFn || fn.Name != "helper" {
		t.Fatalf("helper in good resolves to %#v", use.Target)
	}
	if def, ok := Definition(use.Target); !ok || def.Offset != strings.Index(halfTyped, "helper(k") {
		t.Errorf("helper's definition is at %v", def)
	}
	s := strings.Index(halfTyped, "print(s)") + len("print(")
	if ty, ok := res.Index.TypeText(path, s, s+1); !ok || ty != "String" {
		t.Errorf("s in good has the type %q, want String", ty)
	}
}

// TestSyntaxErrorInATypeKeepsNames checks that a type whose declaration has
// a syntax error isn't reported as undefined where it is used, and that
// types aren't checked against it, which would only give wrong errors.
func TestSyntaxErrorInATypeKeepsNames(t *testing.T) {
	dir := writeProgram(t, map[string]string{
		"main.eye":          "type Box = { side: }\n\nfn area(b: Box) -> Int = b.side\n",
		"shapes/shapes.eye": "pub fn size(x: Int) -> Int {\n  x +\n}\n",
		"app.eye":           "import shapes.{size}\n\nfn twice() -> Int = size(2) + shapes.size(3)\n",
	})
	res := Check(dir, Options{})
	var got []string
	for _, d := range res.Diagnostics {
		got = append(got, d.Message)
	}
	want := "expected a type, found '}'\nexpected an expression before the end of the line"
	if strings.Join(got, "\n") != want {
		t.Errorf("diagnostics:\n%s\nwant:\n%s", strings.Join(got, "\n"), want)
	}
}
