package check

import (
	"strings"
	"testing"

	"eyelang/ir"
)

// TestBuiltinDeclarations checks that calls of built-ins are checked against
// their declarations in ir/builtin.eye: its parameters, result, where clause
// and uses clause.
func TestBuiltinDeclarations(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{"a parameter's type", `fn f() -> String = trim(1)`, []string{"1:25: expected String, found Int"}},
		{"the result's type", `fn f() -> Int = upper("a")`, []string{"1:17: expected Int, found String"}},
		{"as a method", `fn f() -> Bool = "abc".startsWith(1)`, []string{"1:35: expected String, found Int"}},
		{"too many arguments", `fn f() -> List(String) = lines("a", "b")`, []string{"1:37: too many arguments to lines: it takes 1"}},
		{"a where clause", `fn f() = min(secret("a"), secret("b"))`, []string{"1:10: min needs"}},
		{"a uses clause", `pub fn f() -> Instant = now()`, []string{"1:1: f uses clock"}},
		{"a module's function", `pub fn f() -> Option(String) = env.get("HOME")`, []string{"1:1: f uses env"}},
		{"a result of any type", `fn f() -> Int = unreachable("never")`, nil},
		{"a type variable for anyone", `fn f() {
  approve("ship it", by: "ann")
  decline("ship it")
}`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := diagnose(t, c.src)
			if len(got) != len(c.want) {
				t.Fatalf("got %d diagnostics, want %d:\n%s", len(got), len(c.want), strings.Join(got, "\n"))
			}
			for i, w := range c.want {
				if !strings.HasPrefix(got[i], w) {
					t.Errorf("got %q, want %q", got[i], w)
				}
			}
		})
	}
}

// TestBuiltinRules checks that every built-in is checked one way: against
// its declaration, or by a rule in Go when it is one of goRules.
func TestBuiltinRules(t *testing.T) {
	c := &checker{root: t.TempDir(), modules: map[string]*module{}}
	c.prog = &ir.Program{}
	c.prelude = c.parse("prelude", "", []*ir.File{parseFile("<prelude>", preludeSrc)})
	c.loadBuiltins()
	if len(c.diags) > 0 {
		t.Fatalf("ir/builtin.eye: %v", c.diags)
	}
	declared := map[string]bool{}
	for _, d := range ir.BuiltinDocs {
		declared[d.Name] = true
		if c.builtins.funcs[d.Name] == nil && !goRules[d.Name] {
			t.Errorf("%s is declared, but the checker has no signature for it", d.Name)
		}
	}
	for name := range goRules {
		if !declared[name] {
			t.Errorf("goRules names %s, which ir/builtin.eye doesn't declare", name)
		}
	}
}
