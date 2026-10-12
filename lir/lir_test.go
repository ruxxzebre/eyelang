package lir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"eyelang/check"
)

// programs lists the repository's programs: each directory with an eye.toml
// below testdata and docs.
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

// TestPolymorphicRecursion lowers a function that calls itself at a bigger
// type each time, which can't be specialised: an error at the call.
func TestPolymorphicRecursion(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("eye.toml", "")
	write("main.eye", `type Nested(a) = Flat(a) | Deeper(Nested(List(a)))

fn depth(n: Nested(a)) -> Int = match n {
  Flat(_) => 0
  Deeper(m) => 1 + depth(m)
}

fn twice(n: Int) -> Int = n + n

pub fn main() -> Int = depth(Flat(1)) + twice(2)
`)
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	_, errs := Lower(prog)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "main.eye:5:20: depth calls itself at bigger types each time") {
		t.Errorf("lowering said %v, want one error at the inner call of depth", errs)
	}
}

// TestEveryProgramLowers lowers every program of the repository to the low
// IR and verifies it: monomorphic, every block ending, every call naming an
// instance (7.2).
func TestEveryProgramLowers(t *testing.T) {
	for _, dir := range programs(t) {
		prog, diags := check.Program(dir)
		if len(diags) > 0 {
			t.Errorf("%s: %s", dir, diags[0])
			continue
		}
		low, errs := Lower(prog)
		for i, err := range errs {
			if i == 5 {
				t.Errorf("%s: and %d more", dir, len(errs)-5)
				break
			}
			t.Errorf("%s: %v", dir, err)
		}
		problems := Verify(low)
		// Counting (7.5) keeps the low IR valid, and its counts balance on
		// every path.
		if len(problems) == 0 {
			Count(low)
			problems = append(Verify(low), CheckCounts(low)...)
		}
		for i, p := range problems {
			if i == 5 {
				t.Errorf("%s: and %d more problems", dir, len(problems)-5)
				break
			}
			t.Errorf("%s: %s", dir, p)
		}
	}
}

// TestErrorsBecomeProblems lowers `?` into a function whose error type is
// Problem, and `.context(…)`, to code that builds the Problem, showing the
// error; and checks that a fault's block says where it is.
func TestErrorsBecomeProblems(t *testing.T) {
	prog, diags := check.Program("../testdata/programs/core")
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	low, errs := Lower(prog)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	for _, f := range low.Funcs {
		problems := 0
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				switch x := in.(type) {
				case *Builtin:
					if x.Name == "context" {
						t.Errorf("%s calls the built-in context; it should be lowered", f.Name)
					}
				case *MakeVariant:
					if x.Variant.Name == "Problem" {
						problems++
					}
				}
			}
			if _, stop := b.Term.(*Stop); stop {
				if len(b.Instrs) == 0 {
					t.Errorf("%s: b%d stops without saying where", f.Name, b.ID)
				} else if _, at := b.Instrs[0].(*At); !at {
					t.Errorf("%s: b%d stops, and doesn't start with its position", f.Name, b.ID)
				}
			}
		}
		if f.Name == "total" && problems != 2 {
			t.Errorf("total makes %d Problems, want 2: one for ? on a String error, one for context", problems)
		}
	}
}
