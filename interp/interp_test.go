package interp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"eyelang/check"
	"eyelang/ir"
)

// TestPrograms runs the tests of every program in testdata/programs: the
// tour from SPECS.md, and programs that test the language in itself.
func TestPrograms(t *testing.T) {
	dirs, err := os.ReadDir("../testdata/programs")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		t.Run(d.Name(), func(t *testing.T) {
			prog, diags := check.Program(filepath.Join("../testdata/programs", d.Name()))
			for _, diag := range diags {
				t.Fatal(diag)
			}
			results := RunTests(prog, "")
			if len(results) == 0 {
				t.Fatal("the program has no tests")
			}
			for _, r := range results {
				if r.Failure != "" {
					t.Errorf("%s: %s\n%s", r.Name, r.Failure, r.Log)
				}
			}
		})
	}
}

// TestLab checks the lab's workloads in testdata/lab, written in Eyelang to
// test the language on real programs, and runs their tests.
func TestLab(t *testing.T) {
	prog, diags := check.Program("../testdata/lab")
	for _, diag := range diags {
		t.Error(diag)
	}
	if t.Failed() {
		return
	}
	results := RunTests(prog, "")
	if len(results) < 10 {
		t.Errorf("the lab has %d tests; expected its modules' tests too", len(results))
	}
	for _, r := range results {
		if r.Failure != "" {
			t.Errorf("%s: %s\n%s", r.Name, r.Failure, r.Log)
		}
	}
}

// load checks a program made of one file.
func load(t *testing.T, src string, modules ...map[string]string) *ir.Program {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.eye"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, files := range modules {
		for name, text := range files {
			path := filepath.Join(dir, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	prog, diags := check.Program(dir)
	for _, d := range diags {
		t.Fatal(d)
	}
	return prog
}

func TestReplayDoesntRepeatOutput(t *testing.T) {
	prog := load(t, `
job j() {
  log.info("before")
  sleep!(1 min)
  log.info("between")
  sleep!(1 min)
  log.info("after")
}

test "logs once" {
  j.start()
  advance(5 min)
}
`)
	r := RunTests(prog, "")[0]
	if r.Failure != "" {
		t.Fatal(r.Failure)
	}
	if r.Log != "info: before\ninfo: between\ninfo: after\n" {
		t.Errorf("log = %q", r.Log)
	}
}

// TestReplayDoesntStartAChildAgain checks that start! is one recorded step:
// a run replayed after each sleep gets the child it started the first time.
func TestReplayDoesntStartAChildAgain(t *testing.T) {
	prog := load(t, `
entity Counter(id: Int) {
  state n: Int = 0
  pub fn bump() { n += 1 }
}

job child(id: Int) { Counter(id).bump!() }

job parent() -> Int {
  let run = child.start!(1)
  sleep!(1 min)
  sleep!(1 min)
  match run.join!() {
    Completed(_) => 1
    Failed(_) => 0
  }
}

test "starts once" {
  parent.start()
  advance(5 min)
  expect(Counter(1).read().n == 1)
}
`)
	r := RunTests(prog, "")[0]
	if r.Failure != "" {
		t.Fatal(r.Failure)
	}

	// The parent's history holds the start once.
	in := New(prog, &strings.Builder{}, &strings.Builder{})
	in.rt.startNow(prog.Root.Jobs["parent"], nil, nil, ir.Pos{})
	in.rt.advance(ir.Duration(5 * 60e9))
	data, _ := in.rt.store.Run(1)
	starts := 0
	for _, s := range decodeRun(data).Log {
		if s.Call == "start child" {
			starts++
		}
	}
	if starts != 1 {
		t.Errorf("the parent's history holds %d starts of child, want 1", starts)
	}
}

func TestRunsAreSavedAsBytes(t *testing.T) {
	prog := load(t, `
entity Counter(id: Int) {
  state n: Int = 0
  pub fn bump() { n += 1 }
}

job count(id: Int) {
  sleep!(1 min)
  Counter(id).bump!()
  sleep!(1 min)
  Counter(id).bump!()
}
`)
	in := New(prog, &strings.Builder{}, &strings.Builder{})
	in.rt.startNow(prog.Root.Jobs["count"], []Value{int64(7)}, nil, ir.Pos{})
	ref := &EntityRef{Entity: prog.Root.Entities["Counter"], ID: int64(7)}
	expectState := func(want int64, status string) {
		t.Helper()
		state, _, _ := in.rt.load(ref, ir.Pos{})
		if n, _ := state.get("n"); n != want {
			t.Errorf("n = %v, want %d", n, want)
		}
		data, _ := in.rt.store.Run(1)
		if run := decodeRun(data); run.Status != status {
			t.Errorf("run status = %s, want %s (log %d steps)", run.Status, status, len(run.Log))
		}
	}
	expectState(0, "sleeping")
	in.rt.advance(ir.Duration(60e9))
	expectState(1, "sleeping")

	// A new interpreter on the same store, as after a restart, finishes the run.
	again := New(prog, &strings.Builder{}, &strings.Builder{})
	again.rt.useStore(in.rt.index.Store)
	again.rt.now, again.rt.nextRun = in.rt.now, in.rt.nextRun
	again.rt.advance(ir.Duration(60e9))
	in = again
	expectState(2, "done")
}

func TestFaults(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`test "t" { expect([1, 2] |> size == 3) }`, "expected [1, 2] |> size == 3, but [1, 2] |> size is 2"},
		{`test "t" { unreachable("checked above") }`, "unreachable: checked above"},
		{`test "t" { let a = 1 / 0 }`, ""}, // NonZero: the checker refuses it
		{`job j() { sleep!(1 min); unreachable("in a job") }
test "t" { j.start(); advance(2 min) }`, "job j: "},
		{`import missing
test "t" { }`, ""},
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "main.eye"), []byte(c.src), 0o644); err != nil {
			t.Fatal(err)
		}
		prog, diags := check.Program(dir)
		if len(diags) > 0 {
			if c.want == "" {
				continue
			}
			t.Errorf("%s: %v", c.src, diags)
			continue
		}
		r := RunTests(prog, "")[0]
		if !strings.Contains(r.Failure, c.want) || c.want == "" {
			t.Errorf("%s\nfailure %q, want %q", c.src, r.Failure, c.want)
		}
	}
}

// TestBuiltinsAreImplemented checks that every documented built-in exists.
func TestBuiltinsAreImplemented(t *testing.T) {
	table := builtinTable()
	for _, d := range ir.BuiltinDocs {
		if _, ok := table[d.Name]; !ok {
			t.Errorf("%s is documented but not implemented", d.Name)
		}
	}
	for name := range table {
		found := false
		for _, d := range ir.BuiltinDocs {
			found = found || d.Name == name
		}
		if !found {
			t.Errorf("%s is implemented but not documented in ir.BuiltinDocs", name)
		}
	}
}

// TestDocsPrograms runs the tests of every program on the docs site: each
// directory under docs/ that has an eye.toml. The site's pages show code from
// these programs, so a change that breaks a page fails here.
func TestDocsPrograms(t *testing.T) {
	var dirs []string
	err := filepath.WalkDir("../docs", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".")) && path != "../docs" {
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
	for _, dir := range dirs {
		t.Run(strings.TrimPrefix(dir, "../docs/"), func(t *testing.T) {
			prog, diags := check.Program(dir)
			for _, diag := range diags {
				t.Fatal(diag)
			}
			results := RunTests(prog, "")
			if len(results) == 0 {
				t.Fatal("the program has no tests; every program on the site tests what its page shows")
			}
			for _, r := range results {
				if r.Failure != "" {
					t.Errorf("%s: %s\n%s", r.Name, r.Failure, r.Log)
				}
			}
		})
	}
}
