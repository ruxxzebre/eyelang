package codegen

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"eyelang/check"
	"eyelang/ir"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

func program(t *testing.T, dir string) *ir.Program {
	t.Helper()
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		t.Fatalf("%s doesn't check: %v", dir, diags)
	}
	return prog
}

func golden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v; go test ./internal/codegen -update writes it", err)
	}
	if string(want) != string(got) {
		t.Errorf("%s differs from what eye gen writes now; go test ./internal/codegen -update rewrites it, and the diff shows what changed", path)
	}
}

// labModules are the lab's contracts the golden files cover.
var labModules = []string{"notify", "miniflux"}

// TestLabGolden checks the Go and TypeScript written for the lab's Notify
// and Miniflux contracts against testdata/*.golden. go test
// ./internal/codegen -update rewrites them.
func TestLabGolden(t *testing.T) {
	prog := program(t, "../../testdata/lab")
	for _, name := range labModules {
		mod, err := FindModule(prog, name)
		if err != nil {
			t.Fatal(err)
		}
		src, err := Go(prog, mod, Options{Source: "testdata/lab"})
		if err != nil {
			t.Fatalf("Go for %s: %v", name, err)
		}
		golden(t, filepath.Join("testdata", name+".go.golden"), src)
		ts, err := TypeScript(prog, mod, Options{Source: "testdata/lab"})
		if err != nil {
			t.Fatalf("TypeScript for %s: %v", name, err)
		}
		golden(t, filepath.Join("testdata", name+".ts.golden"), ts)
	}
}

// TestLabGoVets builds and vets the Go written for every contract module of
// the lab, each as a package of one Go module.
func TestLabGoVets(t *testing.T) {
	prog := program(t, "../../testdata/lab")
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module labgen\n\ngo 1.22\n")
	for _, mod := range Modules(prog) {
		src, err := Go(prog, mod, Options{Source: "testdata/lab"})
		if err != nil {
			t.Fatalf("Go for %s: %v", mod.Path, err)
		}
		write(t, filepath.Join(dir, mod.Name, mod.Name+".go"), string(src))
	}
	goCmd, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go isn't on PATH, so the generated Go isn't vetted")
	}
	cmd := exec.Command(goCmd, "vet", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=-mod=mod")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go vet on the generated Go: %v\n%s", err, out)
	}
}

// TestLabTypeScriptCompiles compiles the TypeScript written for every
// contract module of the lab with tsc --strict.
func TestLabTypeScriptCompiles(t *testing.T) {
	tsc, err := exec.LookPath("tsc")
	if err != nil {
		t.Skip("tsc isn't on PATH, so the generated TypeScript isn't compiled; npm install -g typescript adds it")
	}
	prog := program(t, "../../testdata/lab")
	dir := t.TempDir()
	var files []string
	for _, mod := range Modules(prog) {
		src, err := TypeScript(prog, mod, Options{Source: "testdata/lab"})
		if err != nil {
			t.Fatalf("TypeScript for %s: %v", mod.Path, err)
		}
		path := filepath.Join(dir, mod.Name+".ts")
		write(t, path, string(src))
		files = append(files, path)
	}
	args := append(TSCArgs(filepath.Join(dir, "out")), files...)
	if out, err := exec.Command(tsc, args...).CombinedOutput(); err != nil {
		t.Fatalf("tsc on the generated TypeScript: %v\n%s", err, out)
	}
}

// storeModule has what the lab's contracts don't: generic types, a Result
// that isn't http's, a DELETE, a map and a set, two contracts in a module,
// and none of @auth and @http(names: snake).
const storeModule = `pub type Page(a) = { items: List(a), next: Option(String) }

pub type Thing = { id: Int, tags: Map(String, Int), seen: Set(String), at: Option(Instant) }

pub type Trouble =
  | Missing(what: String)
  | Broken

pub type Removed = Gone | NotThere

@port(4000)
contract Store {
  @http("GET /things")
  query things(cursor: Option(String) = None) -> Page(Thing)

  @http("POST /check")
  query check(thing: Thing) -> Result(Thing, Trouble)

  @http("DELETE /things/:id")
  command remove(id: Int) -> Removed
}

@port(4001)
contract Audit {
  @http("GET /pages/:n")
  query page(n: Int) -> Page(String)
}
`

// TestOtherShapesCompile vets and compiles what eye gen writes for
// storeModule.
func TestOtherShapesCompile(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "store", "store.eye"), storeModule)
	write(t, filepath.Join(dir, "main.eye"), "import store\n")
	prog := program(t, dir)
	mod, err := FindModule(prog, "store")
	if err != nil {
		t.Fatal(err)
	}
	src, err := Go(prog, mod, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"type PageThing struct", "type PageString struct", "type ResultThingTrouble interface", "func (c *AuditClient) Page(ctx context.Context, args PageArgs) (PageString, error)"} {
		if !strings.Contains(string(src), want) {
			t.Errorf("the Go lacks %q", want)
		}
	}
	out := t.TempDir()
	write(t, filepath.Join(out, "go.mod"), "module storegen\n\ngo 1.22\n")
	write(t, filepath.Join(out, "store", "store.go"), string(src))
	if goCmd, err := exec.LookPath("go"); err == nil {
		cmd := exec.Command(goCmd, "vet", "./...")
		cmd.Dir = out
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=-mod=mod")
		if got, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("go vet: %v\n%s", err, got)
		}
	}
	ts, err := TypeScript(prog, mod, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ts), "export type ResultThingTrouble = Ok | Err;") {
		t.Errorf("the TypeScript lacks the union of Result(Thing, Trouble):\n%s", ts)
	}
	if tsc, err := exec.LookPath("tsc"); err == nil {
		write(t, filepath.Join(out, "store.ts"), string(ts))
		args := append(TSCArgs(filepath.Join(out, "js")), filepath.Join(out, "store.ts"))
		if got, err := exec.Command(tsc, args...).CombinedOutput(); err != nil {
			t.Errorf("tsc: %v\n%s", err, got)
		}
	}
}

// TestRefusedTypes checks what has no generated type, and the message.
func TestRefusedTypes(t *testing.T) {
	cases := []struct{ src, want string }{
		{"query pairs() -> Map(Int, String)", "a Map whose keys aren't strings on the wire"},
		{"query pair() -> (Int, String)", "a tuple"},
	}
	for _, c := range cases {
		dir := t.TempDir()
		write(t, filepath.Join(dir, "svc", "svc.eye"), "@http(names: snake)\ncontract Svc {\n  @http(\"GET /x\")\n  "+c.src+"\n}\n")
		write(t, filepath.Join(dir, "main.eye"), "import svc\n")
		prog := program(t, dir)
		mod, err := FindModule(prog, "svc")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Go(prog, mod, Options{}); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want an error with %q", c.src, err, c.want)
		}
		if _, err := TypeScript(prog, mod, Options{}); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want an error with %q", c.src, err, c.want)
		}
	}
}

func write(t *testing.T, path, src string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}
