package specdoc

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"eyelang/check"
)

// TestExamplesCheck runs the checker on every example of SPECS.md, each in
// the program its fence names (context=NAME: testdata/specctx/NAME), as one
// more file of that program's own module. An example passes when it checks
// with no error, apart from those it shows: a line whose comment starts with
// `error` or `compile error` must have one, and every error must be on such
// a line. An example marked fails=TASK shows what that ROADMAP.md task will
// build: it must not check until the task is done, and must check once it
// is, so the mark comes off when the task lands.
//
// The tour is checked where `eye test` runs it, in testdata/programs/tour,
// whose tour.eye is the example.
func TestExamplesCheck(t *testing.T) {
	spec, err := os.ReadFile("../../SPECS.md")
	if err != nil {
		t.Fatal(err)
	}
	roadmap, err := os.ReadFile("../../ROADMAP.md")
	if err != nil {
		t.Fatal(err)
	}
	examples := Examples(spec)
	if len(examples) < 20 {
		t.Fatalf("found %d examples in SPECS.md, want at least 20", len(examples))
	}
	for _, ex := range examples {
		t.Run(strings.TrimSuffix(ex.Name, ".eye"), func(t *testing.T) {
			if len(ex.Unknown) > 0 {
				t.Errorf("SPECS.md:%d: the fence says %q; it takes context=NAME and fails=TASK", ex.Line-1, strings.Join(ex.Unknown, " "))
			}
			problems := checkExample(t, ex)
			done, known := taskDone(roadmap, ex.Fails)
			switch {
			case ex.Fails == "":
				for _, p := range problems {
					t.Error(p)
				}
			case !known:
				t.Errorf("SPECS.md:%d: the fence says fails=%s, and ROADMAP.md has no task %s", ex.Line-1, ex.Fails, ex.Fails)
			case !done:
				if len(problems) == 0 {
					t.Errorf("SPECS.md:%d: the example checks, but its fence says fails=%s, which ROADMAP.md hasn't marked done: take the mark off", ex.Line-1, ex.Fails)
				}
				for _, p := range problems {
					t.Logf("expected until %s is done: %s", ex.Fails, p)
				}
			default:
				for _, p := range problems {
					t.Errorf("%s (%s is done: the example must check now, and its fence lose fails=%s)", p, ex.Fails, ex.Fails)
				}
			}
		})
	}
}

// errorComment marks a line that shows an error: `# error: …` or
// `# compile error: …`.
var errorComment = regexp.MustCompile(`#\s*(compile )?error\b`)

// checkExample checks the example in its context and returns what is wrong,
// each as "SPECS.md:LINE: message", or as a path for an error in a context
// file.
func checkExample(t *testing.T, ex Example) []string {
	t.Helper()
	dir := t.TempDir()
	files := ex.Files()
	switch {
	case ex.Section == 3 && ex.Index == 1:
		copyDir(t, "../../testdata/programs/tour", dir)
		files = []File{{Path: "tour.eye", Source: ex.Source, Line: ex.Line}}
	case ex.Context != "":
		// context=a,b puts the programs together, so that examples can
		// share one, such as the Notify of section 17.
		for name := range strings.SplitSeq(ex.Context, ",") {
			from := filepath.Join("../../testdata/specctx", name)
			if _, err := os.Stat(from); err != nil {
				return []string{fmt.Sprintf("SPECS.md:%d: context=%s, and there is no directory testdata/specctx/%s", ex.Line-1, ex.Context, name)}
			}
			copyDir(t, from, dir)
		}
		for _, f := range files {
			if _, err := os.Stat(filepath.Join(dir, f.Path)); err == nil {
				t.Fatalf("SPECS.md:%d: the example's file %s is in its context, %s, too", f.Line, f.Path, ex.Context)
			}
		}
	}
	byPath := map[string]File{}
	for _, f := range files {
		path := filepath.Join(dir, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(f.Source), 0o644); err != nil {
			t.Fatal(err)
		}
		byPath[path] = f
	}

	// Every module, so that a block that shows two modules, neither of
	// them the program's own, has both checked.
	diags := check.Check(dir, check.Options{All: true}).Diagnostics
	var problems []string
	shown := map[string]map[int]bool{} // the lines with an error, by file
	for _, d := range diags {
		f, ok := byPath[filepathOf(d)]
		if !ok {
			rel, _ := filepath.Rel(dir, d.Pos.String())
			problems = append(problems, fmt.Sprintf("SPECS.md:%d: an error in the context, %s: %s", ex.Line-1, rel, d.Message))
			continue
		}
		line := d.Pos.File.Tree.Lines.Position(d.Pos.Offset).Line
		if shown[f.Path] == nil {
			shown[f.Path] = map[int]bool{}
		}
		shown[f.Path][line] = true
		if !errorComment.MatchString(lineOf(f.Source, line)) {
			problems = append(problems, fmt.Sprintf("SPECS.md:%d: %s", f.Line+line-1, d.Message))
		}
	}
	for _, f := range files {
		for i, text := range strings.Split(f.Source, "\n") {
			if errorComment.MatchString(text) && !shown[f.Path][i+1] {
				problems = append(problems, fmt.Sprintf("SPECS.md:%d: the comment says this is an error, and the checker reports none", f.Line+i))
			}
		}
	}
	return problems
}

func filepathOf(d check.Diagnostic) string {
	if d.Pos.File == nil {
		return ""
	}
	return d.Pos.File.Path
}

// lineOf returns line n of src, counting from 1.
func lineOf(src string, n int) string {
	lines := strings.Split(src, "\n")
	if n < 1 || n > len(lines) {
		return ""
	}
	return lines[n-1]
}

// taskDone reports whether ROADMAP.md's table row for the task says it is
// done, `| G.14 … (done) |`, and whether there is such a row at all.
func taskDone(roadmap []byte, task string) (done, known bool) {
	for line := range strings.SplitSeq(string(roadmap), "\n") {
		if strings.HasPrefix(line, "| "+task+" ") {
			return strings.Contains(line, "(done) |"), true
		}
	}
	return false, false
}

// copyDir copies the program in from into to, leaving out directories whose
// names start with a dot, such as a journal in .eye.
func copyDir(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		if d.IsDir() {
			if rel != "." && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(to, rel), 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		dest := filepath.Join(to, rel)
		if _, err := os.Stat(dest); err == nil {
			return fmt.Errorf("%s is in two contexts", rel)
		}
		return os.WriteFile(dest, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTaskDone(t *testing.T) {
	roadmap := []byte("| Task | Issue |\n|---|---|\n| G.1 Fixed-width integers (done) | [#84](…) |\n| G.12 Results (done later) | [#95](…) |\n| G.14 Record literals | [#97](…) |\n")
	for task, want := range map[string][2]bool{"G.1": {true, true}, "G.12": {false, true}, "G.14": {false, true}, "G.2": {false, false}} {
		if done, known := taskDone(roadmap, task); done != want[0] || known != want[1] {
			t.Errorf("taskDone(%s) = %v, %v, want %v, %v", task, done, known, want[0], want[1])
		}
	}
}

// TestCheckExample checks that an example with an error it doesn't show
// fails, and that a comment saying `error` must have an error on its line.
func TestCheckExample(t *testing.T) {
	for _, c := range []struct {
		name, context, src string
		want               []string
	}{
		{"clean", "", "fn f() -> Int = 1\n", nil},
		{"an error it doesn't show", "", "fn f() -> Int = \"one\"\n",
			[]string{"SPECS.md:10: expected Int, found String"}},
		{"an error it shows", "", "fn f() -> Int = \"one\"    # error: not an Int\n", nil},
		{"a shown error the checker doesn't report", "", "fn f() -> Int = 1    # error: says nothing\n",
			[]string{"SPECS.md:10: the comment says this is an error, and the checker reports none"}},
		{"a context that isn't there", "nowhere", "fn f() = 1\n",
			[]string{"SPECS.md:9: context=nowhere, and there is no directory testdata/specctx/nowhere"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			ex := Example{Name: "spec-01-x-1.eye", Source: c.src, Line: 10, Context: c.context}
			got := strings.Join(checkExample(t, ex), "\n")
			if want := strings.Join(c.want, "\n"); got != want {
				t.Errorf("problems:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// TestContextsUsed checks that every program in testdata/specctx is the
// context of some example, so that one no example names any more goes.
func TestContextsUsed(t *testing.T) {
	spec, err := os.ReadFile("../../SPECS.md")
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]bool{}
	for _, ex := range Examples(spec) {
		for name := range strings.SplitSeq(ex.Context, ",") {
			used[name] = true
		}
	}
	entries, err := os.ReadDir("../../testdata/specctx")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() && !used[e.Name()] {
			t.Errorf("testdata/specctx/%s is the context of no example in SPECS.md: delete it, or name it on a fence", e.Name())
		}
	}
}

func TestExampleFiles(t *testing.T) {
	spec := "## 17. Contracts\n\n```eye context=shelves fails=6.8\n# shelves/shelves.eye\ncontract Shelves {}\n\n# main.eye: the caller\nfn f() = 1\n```\n"
	ex := Examples([]byte(spec))[0]
	if ex.Context != "shelves" || ex.Fails != "6.8" || ex.Line != 4 || len(ex.Unknown) != 0 {
		t.Errorf("example = %+v", ex)
	}
	files := ex.Files()
	if len(files) != 2 || files[0].Path != "shelves/shelves.eye" || files[0].Line != 4 ||
		files[1].Path != "main.eye" || files[1].Line != 7 || files[1].Source != "# main.eye: the caller\nfn f() = 1\n" {
		t.Errorf("files = %+v", files)
	}
}
