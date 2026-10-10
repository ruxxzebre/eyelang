package repl

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// session runs inputs one after another and returns what each printed.
func session(t *testing.T, dir string, inputs ...string) []string {
	t.Helper()
	var out bytes.Buffer
	r, err := New(dir, &out)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var got []string
	for _, in := range inputs {
		out.Reset()
		r.Eval(in)
		got = append(got, strings.TrimRight(out.String(), "\n"))
	}
	return got
}

func expectOutputs(t *testing.T, got []string, want ...string) {
	t.Helper()
	for i, w := range want {
		if i >= len(got) {
			t.Fatalf("no output for input %d, want %q", i+1, w)
		}
		if w != "*" && got[i] != w {
			t.Errorf("input %d printed\n%s\nwant\n%s", i+1, got[i], w)
		}
	}
}

func TestExpressionsShowValueAndType(t *testing.T) {
	expectOutputs(t, session(t, "",
		`1 + 2`,
		`"a" + "b"`,
		`[1, 2] |> map(fn x: x * 10)`,
		`Some(1.5)`,
		`print("hi")`,
	),
		`3: Int`,
		`"ab": String`,
		`[10, 20]: List(Int)`,
		`Some(1.5): Option(Float)`,
		`hi`,
	)
}

func TestBindingsLast(t *testing.T) {
	expectOutputs(t, session(t, "",
		`let xs = [1, 2, 3]`,
		`var total = 0`,
		`for x in xs { total += x }`,
		`total`,
		`let (a, b) = (total, "six")`,
		`"{a} is {b}"`,
		`let xs = "shadowed"`,
		`xs`,
	),
		`xs: List(Int) = [1, 2, 3]`,
		`total: Int = 0`,
		``,
		`6: Int`,
		"a: Int = 6\nb: String = \"six\"",
		`"6 is six": String`,
		`xs: String = "shadowed"`,
		`"shadowed": String`,
	)
}

func TestDeclarationsJoinTheSession(t *testing.T) {
	expectOutputs(t, session(t, "",
		`fn double(n: Int) -> Int = n * 2`,
		`type Shape = Circle(r: Float) | Dot`,
		`double(21)`,
		`Circle(r: 2.0)`,
		`fn double(n: Int) -> Int = n * 3`,
		`double(2)`,
	),
		`defined double`,
		`defined Shape`,
		`42: Int`,
		`Circle(r: 2.0): Shape`,
		`defined double`,
		`6: Int`,
	)
}

func TestErrorsPointIntoTheInput(t *testing.T) {
	expectOutputs(t, session(t, "",
		`let n = 1`,
		`n + missing`,
		`let s: String = n`,
		"fn f() -> Int {\n  \"no\"\n}",
		`n`,
	),
		`n: Int = 1`,
		"undefined name missing\n  n + missing\n      ^",
		"expected String, found Int\n  let s: String = n\n                  ^",
		"*",
		`1: Int`,
	)
}

func TestJobsRunOnTheSessionClock(t *testing.T) {
	got := session(t, "",
		"entity Kettle(id: String) {\n  state boiled: Int = 0\n\n  pub fn done() { boiled += 1 }\n}",
		"job boil(id: String) {\n  sleep!(4 min)\n  Kettle(id).done!()\n}",
		`boil.start("office")`,
		`Kettle("office").boiled`,
		`:advance 5 min`,
		`Kettle("office").boiled`,
		`:runs`,
	)
	expectOutputs(t, got, `defined Kettle`, `defined boil`, `Run(1): Run(())`, `0: Int`, `*`, `1: Int`)
	if !strings.Contains(got[6], `#1 boil("office")  done`) {
		t.Errorf(":runs printed %q", got[6])
	}
}

func TestCommands(t *testing.T) {
	got := session(t, "",
		`:type [1, 2]`,
		`:doc size`,
		"## Twice n.\nfn twice(n: Int) -> Int = n * 2",
		`:doc twice`,
		`test "adds" { expect(twice(2) == 4) }`,
		`:test`,
		`:nope`,
	)
	expectOutputs(t, got, `List(Int)`, `*`, `defined twice`,
		"fn twice(n: Int) -> Int = n * 2\n\nTwice n.", `*`, "ok    adds\n1 of 1 tests passed",
		":nope isn't a command; :help lists them")
	if !strings.HasPrefix(got[1], "fn size(") {
		t.Errorf(":doc size printed %q", got[1])
	}
}

func TestSessionUsesAProgram(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.eye"), []byte("pub fn greet(name: String) -> String = \"hi {name}\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	expectOutputs(t, session(t, dir, `greet("ann")`), `"hi ann": String`)
}

func TestUnfinished(t *testing.T) {
	for in, want := range map[string]bool{
		`1 + 2`:               false,
		`fn f() {`:            true,
		`[1,`:                 true,
		"let s = \"\"\"":      true,
		"let s = \"\"\"\n  a": true,
		"xs\n  |>":            true,
		`f(1)`:                false,
	} {
		if got := Unfinished(in); got != want {
			t.Errorf("Unfinished(%q) = %v, want %v", in, got, want)
		}
	}
}
