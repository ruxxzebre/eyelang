// Package playground checks, tests, runs and formats a program held in
// memory, and answers its editor's requests, for the online playground: a
// page of the docs site where Eyelang is written and run in the browser,
// with this package compiled to WebAssembly (cmd/eye-wasm). It needs no
// disk, network or real clock: a run's clock skips ahead instead of waiting,
// and calls to other services are answered by the recordings the program
// holds, as in `eye test`.
package playground

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"eyelang/check"
	"eyelang/internal/highlight"
	"eyelang/internal/lsp"
	"eyelang/interp"
	"eyelang/ir"
)

// Root is where a playground's program is, for the checker and the
// interpreter. Nothing is on disk there; the paths the playground takes and
// gives are relative to it, with slashes, such as "main.eye" or
// "notify/notify.eye".
const Root = "/playground"

// Playground holds one program's files.
type Playground struct {
	ws *lsp.Workspace
}

// New starts a playground with no files.
func New() *Playground { return &Playground{ws: lsp.NewWorkspace(Root)} }

func abs(rel string) string { return filepath.Join(Root, filepath.FromSlash(path.Clean("/"+rel))) }

// SetFiles replaces every file with files, by relative path.
func (p *Playground) SetFiles(files map[string]string) {
	for _, f := range p.ws.Paths() {
		p.ws.Remove(f)
	}
	for rel, text := range files {
		p.ws.Set(abs(rel), []byte(text))
	}
}

// Set gives one file's text.
func (p *Playground) Set(rel, text string) { p.ws.Set(abs(rel), []byte(text)) }

// Remove forgets a file.
func (p *Playground) Remove(rel string) { p.ws.Remove(abs(rel)) }

// Files returns every file's text by relative path.
func (p *Playground) Files() map[string]string {
	out := map[string]string{}
	for path, text := range p.ws.Overlay() {
		out[p.ws.Rel(path)] = string(text)
	}
	return out
}

// Diagnostic is a problem in a file, or in the whole program when Path is
// "". Lines and columns are 0-based, columns in UTF-16 code units.
type Diagnostic struct {
	Path    string    `json:"path"`
	Range   lsp.Range `json:"range"`
	Message string    `json:"message"`
}

// Check checks every module of the program.
func (p *Playground) Check() []Diagnostic {
	out := []Diagnostic{}
	for _, d := range p.ws.Check() {
		rel := ""
		if d.Path != "" {
			rel = p.ws.Rel(d.Path)
		}
		out = append(out, Diagnostic{Path: rel, Range: d.Range, Message: d.Message})
	}
	return out
}

// program is one of the playground's programs: the root, or a directory
// below it with an eye.toml of its own, such as a service the root calls.
type program struct {
	dir  string // relative to Root, with slashes; "" for the root
	prog *ir.Program
}

// programs checks every program, the root's first; it returns nil, with the
// diagnostics, when any program has a problem.
func (p *Playground) programs() ([]program, []Diagnostic) {
	if diags := p.Check(); len(diags) > 0 {
		return nil, diags
	}
	var out []program
	for _, dir := range p.ws.Programs() {
		res := check.Check(dir, check.Options{Overlay: p.ws.Overlay()})
		if len(res.Diagnostics) > 0 || res.Program == nil || res.Program.Root == nil {
			// Check above said the programs were fine; this is the same check.
			var diags []Diagnostic
			for _, d := range res.Diagnostics {
				diags = append(diags, Diagnostic{Message: d.String()})
			}
			return nil, diags
		}
		rel := p.ws.Rel(dir)
		if dir == Root {
			rel = ""
		}
		out = append(out, program{dir: rel, prog: res.Program})
	}
	return out, nil
}

// files are the program's files that aren't source, such as recordings, by
// absolute path, for the interpreter to read in place of the disk.
func (p *Playground) files() map[string][]byte {
	out := map[string][]byte{}
	for path, text := range p.ws.Overlay() {
		if !strings.HasSuffix(path, ".eye") {
			out[path] = text
		}
	}
	return out
}

// TestResult is one test's outcome. Line is 1-based.
type TestResult struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Failure string `json:"failure,omitempty"`
	Skipped string `json:"skipped,omitempty"`
	Log     string `json:"log,omitempty"`
	Micros  int64  `json:"micros"`
}

// TestReport is what Test found: the program's problems when it doesn't
// check, or each test's result, and the report `eye test -v` would print.
type TestReport struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
	Results     []TestResult `json:"results"`
	Passed      int          `json:"passed"`
	Failed      int          `json:"failed"`
	Skipped     int          `json:"skipped"`
	Output      string       `json:"output"`
}

// Test runs the tests whose names contain filter, each with a fresh store
// and a clock that moves only when the test advances it, as `eye test` does:
// the root program's, then each other program's, its tests' names after its
// directory.
func (p *Playground) Test(filter string) TestReport {
	rep := TestReport{Diagnostics: []Diagnostic{}, Results: []TestResult{}}
	progs, diags := p.programs()
	if progs == nil {
		rep.Diagnostics = diags
		rep.Output = diagnosticsText(diags)
		return rep
	}
	var results []interp.TestResult
	for _, pr := range progs {
		rs, _, _ := interp.RunTestsWith(pr.prog, interp.TestOptions{Filter: filter, Files: p.files()})
		for i := range rs {
			if pr.dir != "" {
				rs[i].Name = pr.dir + ": " + rs[i].Name
			}
		}
		results = append(results, rs...)
	}
	var out strings.Builder
	var total time.Duration
	for _, r := range results {
		total += r.Duration
		tr := TestResult{Name: r.Name, Failure: r.Failure, Skipped: r.Skipped, Log: r.Log, Micros: r.Duration.Microseconds()}
		if f := r.Pos.File; f != nil {
			tr.Path = p.ws.Rel(f.Path)
			tr.Line = f.Tree.Lines.Position(r.Pos.Offset).Line
		}
		rep.Results = append(rep.Results, tr)
		switch {
		case r.Skipped != "":
			rep.Skipped++
			fmt.Fprintf(&out, "skip  %s\n%s", r.Name, indent(r.Skipped))
		case r.Failure == "":
			rep.Passed++
			fmt.Fprintf(&out, "ok    %s (%s)\n%s", r.Name, r.Duration.Round(time.Microsecond), indent(r.Log))
		default:
			rep.Failed++
			fmt.Fprintf(&out, "FAIL  %s (%s)\n%s%s", r.Name, r.Duration.Round(time.Microsecond), indent(r.Failure), indent(r.Log))
		}
	}
	also := ""
	if rep.Skipped > 0 {
		also = fmt.Sprintf(", %d skipped", rep.Skipped)
	}
	switch {
	case len(results) == 0 && filter != "":
		fmt.Fprintf(&out, "no test's name contains %q\n", filter)
	case len(results) == 0:
		fmt.Fprintln(&out, "no tests to run: add one, such as test \"it adds\" { expect(1 + 1 == 2) }")
	case rep.Failed > 0:
		fmt.Fprintf(&out, "%d of %d tests failed%s\n", rep.Failed, len(results), also)
	default:
		fmt.Fprintf(&out, "%d tests passed%s (%s)\n", rep.Passed, also, total.Round(time.Microsecond))
	}
	rep.Output = out.String()
	return rep
}

// RunOptions say how Run runs the program.
type RunOptions struct {
	// Args are the program's arguments, after its name.
	Args []string
	// Start is when the virtual clock starts; zero is now.
	Start time.Time
	// Until is how far the clock may move once main has run, and Wakes how
	// many runs may wake and schedules fire; zero is a day and a thousand.
	Until time.Duration
	Wakes int
}

// RunReport is what Run did.
type RunReport struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
	Exit        int          `json:"exit"`
}

// Run runs the program's main, then its jobs and schedules on a clock that
// skips ahead, writing what the program prints to stdout and its log and
// faults to stderr, as `eye run --memory` does.
//
// Each other program that serves a contract is started first, as a service
// in this process: the root program's calls to that contract reach it with
// no network, and what it prints, logs and answers goes to services, each
// line after its directory. The secrets the contracts' @auth names are made
// up, the same for both sides. Once the root program is done, the services
// run what its calls started, on their clocks, which its calls moved.
func (p *Playground) Run(opts RunOptions, stdout, stderr, services io.Writer) RunReport {
	progs, diags := p.programs()
	if progs == nil {
		fmt.Fprint(stderr, diagnosticsText(diags))
		return RunReport{Diagnostics: diags, Exit: 1}
	}
	if services == nil {
		services = stderr
	}
	secrets := map[string]string{}
	for _, pr := range progs {
		for _, m := range pr.prog.Modules {
			for _, ct := range m.Contracts {
				if ct.HTTP.AuthSecret != "" {
					secrets[ct.HTTP.AuthSecret] = "made-up-in-the-playground"
				}
			}
		}
	}
	clock := func() *interp.VirtualClock {
		return &interp.VirtualClock{Start: opts.Start, Until: opts.Until, Wakes: opts.Wakes}
	}
	handlers := map[string]http.Handler{}
	var started []*interp.Service
	for _, pr := range progs[1:] {
		w := &prefixWriter{prefix: pr.dir + " │ ", w: services}
		svc, err := interp.StartService(pr.prog, interp.RunOptions{
			Stdout: w, Stderr: w, MemoryLimit: 1 << 30, Files: p.files(), Secrets: secrets, Virtual: clock(),
		})
		if err != nil {
			fmt.Fprintf(w, "%v\n", err)
			continue
		}
		if len(svc.Handlers()) == 0 {
			continue // a program that serves nothing has nothing to answer
		}
		for module, h := range svc.Handlers() {
			handlers[module] = h
		}
		started = append(started, svc)
	}
	code := interp.Run(context.Background(), progs[0].prog, interp.RunOptions{
		Args:   append([]string{"main"}, opts.Args...),
		Stdout: stdout, Stderr: stderr,
		MemoryLimit: 1 << 30, // no cgroup to read
		Files:       p.files(),
		Services:    handlers,
		Secrets:     secrets,
		Virtual:     clock(),
	})
	for _, svc := range started {
		svc.Finish()
	}
	return RunReport{Diagnostics: []Diagnostic{}, Exit: code}
}

// prefixWriter writes each line after a prefix.
type prefixWriter struct {
	prefix string
	w      io.Writer
	mid    bool // the last write ended inside a line
}

func (pw *prefixWriter) Write(b []byte) (int, error) {
	var out strings.Builder
	for _, line := range strings.SplitAfter(string(b), "\n") {
		if line == "" {
			continue
		}
		if !pw.mid {
			out.WriteString(pw.prefix)
		}
		out.WriteString(line)
		pw.mid = !strings.HasSuffix(line, "\n")
	}
	if _, err := io.WriteString(pw.w, out.String()); err != nil {
		return 0, err
	}
	return len(b), nil
}

// Format returns a file in the one canonical style.
func (p *Playground) Format(rel string) (string, error) {
	out, err := p.ws.Format(abs(rel))
	return string(out), err
}

// Hover returns the Markdown shown for what is at a position, or "".
func (p *Playground) Hover(rel string, at lsp.Position) string {
	h, _ := p.ws.Hover(abs(rel), at)
	return h
}

// Complete returns what may be typed at a position.
func (p *Playground) Complete(rel string, at lsp.Position) []lsp.Completion {
	return p.ws.Complete(abs(rel), at)
}

// Location is a range in one of the program's files.
type Location struct {
	Path  string    `json:"path"`
	Range lsp.Range `json:"range"`
}

// Definition returns where what is at a position is declared, when that is
// in one of the program's files.
func (p *Playground) Definition(rel string, at lsp.Position) []Location {
	out := []Location{}
	for _, l := range p.ws.Definition(abs(rel), at) {
		out = append(out, Location{Path: p.ws.Rel(l.Path), Range: l.Range})
	}
	return out
}

func diagnosticsText(diags []Diagnostic) string {
	var b strings.Builder
	sorted := append([]Diagnostic(nil), diags...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	for _, d := range sorted {
		if d.Path == "" {
			fmt.Fprintln(&b, d.Message)
			continue
		}
		fmt.Fprintf(&b, "%s:%d:%d: %s\n", d.Path, d.Range.Start.Line+1, d.Range.Start.Character+1, d.Message)
	}
	return b.String()
}

func indent(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		b.WriteString("      " + line + "\n")
	}
	return b.String()
}

// Span is a part of a file to highlight, in UTF-16 code units from the
// file's start, as a browser counts a string. Class is one of
// internal/highlight's: k keyword, c comment, s string, n number, t type,
// a attribute, w waiting call.
type Span struct {
	From  int    `json:"from"`
	To    int    `json:"to"`
	Class string `json:"class"`
}

// Highlight returns the spans of text to highlight, from the parser, as the
// spec page highlights its examples.
func Highlight(text string) []Span {
	out := []Span{}
	units, at := 0, 0 // UTF-16 units before the byte offset at
	to16 := func(offset int) int {
		for _, r := range text[at:offset] {
			units++
			if r >= 0x10000 {
				units++
			}
		}
		at = offset
		return units
	}
	for _, s := range highlight.Spans([]byte(text)) {
		from := to16(s.Start)
		out = append(out, Span{From: from, To: to16(s.End), Class: s.Class})
	}
	return out
}
