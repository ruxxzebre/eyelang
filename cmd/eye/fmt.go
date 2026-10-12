package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/term"

	"eyelang/check"
	"eyelang/syntax"
)

// stdin is standard input for `eye fmt` and `eye lsp`; tests replace it.
var stdin io.Reader = os.Stdin

// runFmt implements `eye fmt [--check] [--syntax-only] [path...]`.
func runFmt(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("eye fmt", flag.ContinueOnError)
	flags.SetOutput(stderr)
	checkOnly := flags.Bool("check", false, "list files that aren't formatted and exit 1 if there are any, without changing them")
	cold := flags.Bool("cold", false, "run a fresh formatting check without reusing saved results (requires --check)")
	syntaxOnly := flags.Bool("syntax-only", false, "format a directory's files without checking its program, leaving uses clauses as written")
	showStats := flags.Bool("stats", false, "after formatting paths, report counts, each phase's time and the slowest files on standard error")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: eye fmt [--check] [--cold] [--syntax-only] [--stats] [file.eye | directory]...")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Formats files in place; directories are searched for .eye files. With no")
		fmt.Fprintln(stderr, "paths, formats standard input to standard output.")
		fmt.Fprintln(stderr, "Recursive searches skip hidden directories and node_modules, unless named explicitly.")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "A directory's files are also checked as programs, and each public function's")
		fmt.Fprintln(stderr, "uses clause is written from the effects its body has. A file named alone is")
		fmt.Fprintln(stderr, "formatted for syntax only.")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "At a terminal, it ends with a line saying how many files and lines it")
		fmt.Fprintln(stderr, "formatted and how long that took; --stats breaks the time down.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if *cold && !*checkOnly {
		fmt.Fprintln(stderr, "eye fmt: --cold requires --check")
		return exitUsage
	}

	if flags.NArg() == 0 {
		if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
			fmt.Fprintln(stderr, "eye fmt: waiting for source on stdin; press Ctrl-D on an empty line to finish.")
			if *checkOnly {
				fmt.Fprintln(stderr, "To check the current directory instead, press Ctrl-C and run: eye fmt --check .")
			} else {
				fmt.Fprintln(stderr, "To format the current directory instead, press Ctrl-C and run: eye fmt .")
			}
		}
		src, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "eye fmt: %v\n", err)
			return exitErrors
		}
		out, ok := format("<stdin>", src, stderr)
		if !ok {
			return exitErrors
		}
		if *checkOnly {
			if !bytes.Equal(src, out) {
				fmt.Fprintln(stdout, "<stdin>")
				return exitErrors
			}
			return 0
		}
		_, _ = stdout.Write(out)
		return 0
	}

	// Stats are kept even unasked, for a saved result to report later.
	stats := newFmtStats(*checkOnly, flags.NArg())
	files, inDirs, err := eyeFiles(flags.Args())
	if err != nil {
		fmt.Fprintf(stderr, "eye fmt: %v\n", err)
		return exitErrors
	}
	if report := stderr; *showStats || fmtSummaryWanted(report) {
		defer func() { // to stderr itself: the stats aren't part of a saved result
			if *showStats {
				stats.report(report)
			} else {
				fmt.Fprintln(report, stats.summary())
			}
		}()
	}
	stats.phase("find files", fmt.Sprintf("%s under %s", plural(len(files), ".eye file"), plural(flags.NArg(), "path")))
	cachePath := ""
	var cachedOut, cachedErr bytes.Buffer
	if *checkOnly && !*cold {
		cachePath = fmtResultPath(flags.Args(), *syntaxOnly)
		if result, ok := loadFmtResult(cachePath, files); ok {
			stats.replay(result, len(files))
			_, _ = stdout.Write(result.Stdout)
			_, _ = stderr.Write(result.Stderr)
			return result.Code
		}
		stats.cache = "miss"
		stats.phase("cache lookup", "no saved result matches")
		stdout = io.MultiWriter(stdout, &cachedOut)
		stderr = io.MultiWriter(stderr, &cachedErr)
	} else if *checkOnly {
		stats.cache = "off"
	}
	code := 0
	var sigs map[string][]check.Signature
	var session check.FormatSession
	if !*syntaxOnly {
		var ok bool
		if sigs, ok = signatures(inDirs, &session, stats, stderr); !ok {
			code = exitErrors
		}
		stats.phase("check", fmt.Sprintf("%s, %s, for uses clauses", plural(stats.programs, "program"), plural(stats.modules, "module")))
	}
	for _, path := range files {
		fileStart := time.Now()
		src, err := os.ReadFile(path)
		session.RecordSource(absPath(path), src, err)
		if err != nil {
			fmt.Fprintf(stderr, "eye fmt: %v\n", err)
			code = exitErrors
			stats.errors++
			continue
		}
		in := src
		if s := sigs[absPath(path)]; len(s) > 0 {
			in = writeUses(path, src, s, stderr)
		}
		parsed := session.File(absPath(path))
		if parsed == nil || !bytes.Equal(in, parsed.Src) {
			parsed = syntax.Parse(in)
		}
		out, ok := formatParsed(path, parsed, stderr)
		stats.file(path, src, time.Since(fileStart))
		if !ok {
			code = exitErrors
			stats.errors++
			continue
		}
		if bytes.Equal(src, out) {
			continue
		}
		if *checkOnly {
			fmt.Fprintln(stdout, path)
			code = exitErrors
			stats.unformatted++
			continue
		}
		if err := os.WriteFile(path, out, 0o644); err != nil {
			fmt.Fprintf(stderr, "eye fmt: %v\n", err)
			code = exitErrors
			stats.errors++
		} else {
			stats.wrote++
		}
	}
	detail := "read, parse unless the check did, and format each file"
	if !*checkOnly {
		detail += fmt.Sprintf("; wrote %s", plural(stats.wrote, "file"))
	}
	stats.phase("format", detail)
	if *checkOnly && !*cold {
		inputs := saveFmtResult(cachePath, files, &session, cachedOut.Bytes(), cachedErr.Bytes(), code, stats.saved())
		stats.cacheInputs = inputs
		stats.phase("save result", fmt.Sprintf("hash %s inputs for the next run", count(inputs)))
	}
	return code
}

// format formats src, reporting syntax errors as path:line:column.
func format(path string, src []byte, stderr io.Writer) ([]byte, bool) {
	return formatParsed(path, syntax.Parse(src), stderr)
}

func formatParsed(path string, f *syntax.File, stderr io.Writer) ([]byte, bool) {
	if len(f.Diagnostics) > 0 {
		for _, d := range f.Diagnostics {
			pos := f.Lines.Position(d.Span.Start)
			fmt.Fprintf(stderr, "%s:%d:%d: %s\n", path, pos.Line, pos.Column, d.Message)
		}
		return nil, false
	}
	out, err := syntax.FormatFile(f)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", path, err)
		return nil, false
	}
	return out, true
}

// eyeFiles expands directories into the .eye files under them. inDirs are
// the files found in directories, which are formatted as parts of programs.
func eyeFiles(paths []string) (files, inDirs []string, err error) {
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, nil, err
		}
		if !info.IsDir() {
			files = append(files, p)
			continue
		}
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if path != p && d.IsDir() && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			if !d.IsDir() && strings.HasSuffix(path, ".eye") {
				files = append(files, path)
				inDirs = append(inDirs, path)
			}
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}
	return files, inDirs, nil
}

// signatures checks the programs the files belong to, each module once, and
// returns by absolute path the public functions whose uses clause doesn't
// say what their bodies do. It reports a program with other errors, whose
// clauses it leaves as written, and returns false.
func signatures(files []string, session *check.FormatSession, stats *fmtStats, stderr io.Writer) (map[string][]check.Signature, bool) {
	out := map[string][]check.Signature{}
	checked := map[string]bool{} // module directories
	seen := map[string]bool{}    // functions, by file and offset: a module imported by two checked ones comes back twice
	ok := true
	for _, path := range files {
		dir := filepath.Dir(absPath(path))
		if checked[dir] {
			continue
		}
		root := session.Root(dir)
		res := session.Check(root, dir)
		checked[dir] = true
		stats.programs++
		stats.modules += len(res.Program.Modules)
		for _, m := range res.Program.Modules {
			if m.Dir != "" {
				checked[absPath(m.Dir)] = true
			}
		}
		sigs, clean := res.Signatures()
		if !clean {
			fmt.Fprintf(stderr, "eye fmt: %s has errors, so its uses clauses weren't written; eye check %s lists them\n", dir, root)
			ok = false
			continue
		}
		for _, s := range sigs {
			file := absPath(s.Func.Pos.File.Path)
			if key := fmt.Sprintf("%s:%d", file, s.Func.Pos.Offset); !seen[key] {
				seen[key] = true
				out[file] = append(out[file], s)
			}
		}
	}
	return out, ok
}

// writeUses writes the uses clause of each function in sigs into src, the
// file at path as the checker read it, for format to tidy.
func writeUses(path string, src []byte, sigs []check.Signature, stderr io.Writer) []byte {
	type edit struct {
		start, end int
		text       string
	}
	var edits []edit
	for _, s := range sigs {
		at := s.Func.Pos
		if !bytes.Equal(at.File.Src, src) {
			return src // changed since it was checked
		}
		var d *syntax.Node
		at.File.Tree.Root.Walk(func(n *syntax.Node) bool {
			if (n.Kind == syntax.FunctionDeclaration || n.Kind == syntax.JobDeclaration) && n.Span.Start == at.Offset && n.Span.End == at.End {
				d = n
			}
			return d == nil
		})
		if d == nil {
			continue
		}
		clause := "uses {" + strings.Join(s.Effects, ", ") + "}"
		if c := d.ChildByField(syntax.FieldEffects); c != nil {
			if len(s.Effects) == 0 {
				clause = ""
			}
			edits = append(edits, edit{c.Span.Start, c.Span.End, clause})
			continue
		}
		after := d.ChildByField(syntax.FieldParameters)
		if ret := d.ChildByField(syntax.FieldReturnType); ret != nil {
			if ret.Kind == syntax.FunctionType {
				// A uses clause here would be the returned function's.
				fmt.Fprintf(stderr, "%s: %s returns a function, so eye fmt can't write its uses clause: add uses {%s} by hand\n",
					at, s.Func.Name, strings.Join(s.Effects, ", "))
				continue
			}
			after = ret
		}
		edits = append(edits, edit{after.Span.End, after.Span.End, " " + clause})
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	out := src
	for _, e := range edits {
		out = append(append(append([]byte{}, out[:e.start]...), e.text...), out[e.end:]...)
	}
	return out
}

// absPath is path made absolute, or path when it can't be.
func absPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}
