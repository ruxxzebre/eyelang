package playground

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"eyelang/internal/lsp"
)

const hello = `# Says hello.

fn greeting(name: String) -> String = "Hello, {name}!"

pub fn main(args: List(String)) uses {stdio} {
  let names = args |> drop(1)
  if names == [] {
    print(greeting("world"))
  }
  for name in names { print(greeting(name)) }
}

test "greeting names who it greets" {
  expect(greeting("Ann") == "Hello, Ann!")
}
`

func TestRunAndTest(t *testing.T) {
	p := New()
	p.SetFiles(map[string]string{"eye.toml": "", "main.eye": hello})
	if d := p.Check(); len(d) != 0 {
		t.Fatalf("Check() = %v", d)
	}
	var out, errs bytes.Buffer
	rep := p.Run(RunOptions{Args: []string{"Ann", "Bo"}}, &out, &errs, nil)
	if rep.Exit != 0 || out.String() != "Hello, Ann!\nHello, Bo!\n" || errs.Len() != 0 {
		t.Errorf("Run() = %+v, stdout %q, stderr %q", rep, out.String(), errs.String())
	}
	tr := p.Test("")
	if tr.Passed != 1 || tr.Failed != 0 || len(tr.Results) != 1 || tr.Results[0].Path != "main.eye" || tr.Results[0].Line != 13 {
		t.Errorf("Test() = %+v", tr)
	}
	if !strings.HasPrefix(tr.Output, "ok    greeting names who it greets (") || !strings.Contains(tr.Output, "1 tests passed") {
		t.Errorf("Test().Output = %q", tr.Output)
	}

	// A type error stops both, and says where it is.
	p.Set("main.eye", strings.Replace(hello, `greeting("world")`, `greeting(42)`, 1))
	diags := p.Check()
	if len(diags) != 1 || diags[0].Path != "main.eye" || diags[0].Range.Start != (lsp.Position{Line: 7, Character: 19}) {
		t.Fatalf("Check() = %+v", diags)
	}
	out.Reset()
	errs.Reset()
	if rep := p.Run(RunOptions{}, &out, &errs, nil); rep.Exit != 1 || errs.String() != "main.eye:8:20: expected String, found Int\n" {
		t.Errorf("Run() of a broken program = %+v, stderr %q", rep, errs.String())
	}
	if tr := p.Test(""); len(tr.Diagnostics) != 1 || tr.Output != "main.eye:8:20: expected String, found Int\n" {
		t.Errorf("Test() of a broken program = %+v", tr)
	}
}

// TestRunWaits runs a job that sleeps for 15 minutes in no time, on the
// virtual clock.
func TestRunWaits(t *testing.T) {
	p := New()
	src, err := os.ReadFile("../../docs/tutorial/code/a-job-that-waits/main.eye")
	if err != nil {
		t.Fatal(err)
	}
	p.SetFiles(map[string]string{"main.eye": string(src)})
	var out, errs bytes.Buffer
	start := time.Now()
	rep := p.Run(RunOptions{Start: time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)}, &out, &errs, nil)
	if time.Since(start) > 2*time.Second {
		t.Errorf("Run() took %v", time.Since(start))
	}
	if rep.Exit != 0 || !strings.Contains(errs.String(), "released order-1's hold on apples at 2026-10-12 09:15") {
		t.Errorf("Run() = %+v, stdout %q, stderr %q", rep, out.String(), errs.String())
	}
}

// TestServices runs the shop and the bank of docs/examples/shop-and-bank in
// one playground: the shop's Payments calls reach the bank's impl, an order
// charged twice is paid once, and the bank's side shows each call.
func TestServices(t *testing.T) {
	p := New()
	p.SetFiles(readDir(t, "../../docs/examples/shop-and-bank"))
	if d := p.Check(); len(d) != 0 {
		t.Fatalf("Check() = %v", d)
	}
	var out, errs, services bytes.Buffer
	rep := p.Run(RunOptions{Start: time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)}, &out, &errs, &services)
	want := "order 1: paid 2500 cents, 7500 left\n" +
		"order 1: paid 2500 cents, 7500 left\n" +
		"order 2: declined: only 7500 cents left\n" +
		"ann has 7500 cents\n"
	if rep.Exit != 0 || out.String() != want {
		t.Errorf("Run() = %+v, stdout:\n%s\nwant:\n%s\nstderr:\n%s\nservices:\n%s", rep, out.String(), want, errs.String(), services.String())
	}
	for _, line := range []string{
		"bank │ payments.Payments POST /v1/charges → 201 Created\n",
		"bank │ payments.Payments POST /v1/charges → 402 Payment Required\n",
		"bank │ payments.Payments GET /v1/customers/ann/balance → 200 OK\n",
	} {
		if !strings.Contains(services.String(), line) {
			t.Errorf("the services' output has no %q:\n%s", line, services.String())
		}
	}

	// Test runs both programs' tests.
	tr := p.Test("")
	if tr.Failed != 0 || len(tr.Results) != 2 || tr.Results[1].Name != "bank: a charge sent twice takes the money once" {
		t.Errorf("Test():\n%s", tr.Output)
	}
}

func readDir(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		src, err := os.ReadFile(path)
		files[filepath.ToSlash(rel)] = string(src)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// TestDocsPrograms loads each program of the quickstart, the tutorial, the
// guide and the examples into a playground, as its examples menu does, and
// checks that it checks and that its tests pass, recordings and all.
func TestDocsPrograms(t *testing.T) {
	for _, root := range []string{"../../docs/quickstart/code", "../../docs/tutorial/code", "../../docs/guide/code", "../../docs/examples"} {
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(root, e.Name())
			t.Run(filepath.Base(root)+"/"+e.Name(), func(t *testing.T) {
				files := map[string]string{}
				err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
					if err != nil || d.IsDir() {
						return err
					}
					rel, _ := filepath.Rel(dir, path)
					src, err := os.ReadFile(path)
					files[filepath.ToSlash(rel)] = string(src)
					return err
				})
				if err != nil {
					t.Fatal(err)
				}
				p := New()
				p.SetFiles(files)
				if d := p.Check(); len(d) != 0 {
					t.Fatalf("Check() = %v", d)
				}
				tr := p.Test("")
				if tr.Failed != 0 || tr.Passed == 0 {
					t.Errorf("Test():\n%s", tr.Output)
				}
			})
		}
	}
}

func TestEditor(t *testing.T) {
	p := New()
	p.SetFiles(map[string]string{"main.eye": hello})
	if h := p.Hover("main.eye", lsp.Position{Line: 7, Character: 12}); !strings.Contains(h, "greeting(name: String) -> String") {
		t.Errorf("Hover(greeting) = %q", h)
	}
	if locs := p.Definition("main.eye", lsp.Position{Line: 7, Character: 12}); len(locs) != 1 || locs[0].Path != "main.eye" || locs[0].Range.Start.Line != 2 {
		t.Errorf("Definition(greeting) = %+v", locs)
	}
	found := false
	for _, c := range p.Complete("main.eye", lsp.Position{Line: 8, Character: 3}) {
		found = found || c.Label == "greeting" && c.Kind == "function"
	}
	if !found {
		t.Error("Complete() in main's body doesn't offer greeting")
	}
	p.Set("main.eye", "fn  shout( s:String )->String=s+\"!\"\n")
	if out, err := p.Format("main.eye"); err != nil || out != "fn shout(s: String) -> String = s + \"!\"\n" {
		t.Errorf("Format() = %q, %v", out, err)
	}
}

func TestHighlight(t *testing.T) {
	// "é" is one UTF-16 unit and two bytes; "𝄞" is two units and four bytes.
	got := Highlight("let s = \"é𝄞\" # x\n")
	want := []Span{{0, 3, "k"}, {8, 9, "s"}, {9, 12, "s"}, {12, 13, "s"}, {14, 17, "c"}}
	if len(got) != len(want) {
		t.Fatalf("Highlight() = %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Highlight()[%d] = %v; want %v", i, got[i], want[i])
		}
	}
}
