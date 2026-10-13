package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"eyelang/check"
	"eyelang/ir"
	"eyelang/syntax"
)

// runFix implements `eye fix [--check] [program-directory | file.eye]...`.
func runFix(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("eye fix", flag.ContinueOnError)
	flags.SetOutput(stderr)
	checkOnly := flags.Bool("check", false, "list what would change and exit 1 if anything would, without changing files")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: eye fix [--check] [program-directory | file.eye]...")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Rewrites a program's code where the language changed under it, then formats")
		fmt.Fprintln(stderr, "the files it changed. Today it adds the ! that a durable step needs in a job or")
		fmt.Fprintln(stderr, "a waiting function, wherever eye check says \"durable step: write f!(…)\":")
		fmt.Fprintln(stderr, "  - calls that wait: sleep, approval, lock and the other waiting built-ins,")
		fmt.Fprintln(stderr, "    and functions that wait")
		fmt.Fprintln(stderr, "  - calls that reach outside the program: now, env.get, http.get, fs, process")
		fmt.Fprintln(stderr, "    and random, and functions that use them")
		fmt.Fprintln(stderr, "  - a contract's operation, and reconcile")
		fmt.Fprintln(stderr, "  - an entity call, and read on an entity, a view or E.all")
		fmt.Fprintln(stderr, "  - publish and start")
		fmt.Fprintln(stderr, "It prints each change as file:line:column, and exits 1 if errors it can't fix")
		fmt.Fprintln(stderr, "are left; eye check lists them. With no paths, fixes the program in the")
		fmt.Fprintln(stderr, "current directory.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	paths := flags.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	code := 0
	seen := map[string]bool{}
	for _, p := range paths {
		dir := p
		if info, err := os.Stat(p); err != nil {
			fmt.Fprintf(stderr, "eye fix: %v\n", err)
			code = exitErrors
			continue
		} else if !info.IsDir() {
			dir = check.Root(p)
		}
		if seen[dir] {
			continue
		}
		seen[dir] = true
		if !fixProgram(dir, *checkOnly, stdout, stderr) {
			code = exitErrors
		}
	}
	return code
}

// fixProgram fixes the program in dir until checking it finds nothing more
// to fix, since a fix can let the checker go on to code it stopped before.
// It reports whether the program is left without errors (with checkOnly,
// without anything to fix either).
func fixProgram(dir string, checkOnly bool, stdout, stderr io.Writer) bool {
	for round := 0; round < 10; round++ {
		_, diags := check.Program(dir)
		edits := map[*ir.File][]int{}
		var fixed []string
		left := 0
		for _, d := range diags {
			name, at, ok := stepFix(d)
			if !ok {
				left++
				continue
			}
			edits[d.Pos.File] = append(edits[d.Pos.File], at)
			fixed = append(fixed, fmt.Sprintf("%s: %s → %s!", d.Pos, name, name))
		}
		if len(edits) == 0 {
			if left > 0 {
				fmt.Fprintf(stderr, "eye fix: %s has errors eye fix can't fix; eye check %s lists them\n", dir, dir)
			}
			return left == 0
		}
		for _, f := range fixed {
			fmt.Fprintln(stdout, f)
		}
		if checkOnly {
			return false
		}
		for f, ats := range edits {
			if !writeFixes(f, ats, stderr) {
				return false
			}
		}
	}
	fmt.Fprintf(stderr, "eye fix: %s still needs fixes after ten rounds; eye check %s lists them\n", dir, dir)
	return false
}

// stepMessage matches the checker's message for a durable step written
// without its `!`, which ends by naming the call to write.
var stepMessage = regexp.MustCompile(`durable step: write ([\pL_][\pL\pN_]*(?:\.[\pL_][\pL\pN_]*)*)!\(…\)$`)

// stepFix finds where a durable step's missing `!` goes: before the
// arguments of the call the diagnostic is at, whose function is named name.
func stepFix(d check.Diagnostic) (name string, at int, ok bool) {
	m := stepMessage.FindStringSubmatch(d.Message)
	f := d.Pos.File
	if m == nil || f == nil || f.Tree == nil {
		return "", 0, false
	}
	name, at = m[1], -1
	f.Tree.Root.Walk(func(n *syntax.Node) bool {
		if at >= 0 {
			return false
		}
		if n.Kind != syntax.CallExpression || n.Span.Start != d.Pos.Offset || (d.Pos.End > 0 && n.Span.End != d.Pos.End) {
			return true
		}
		fn, args := n.ChildByField(syntax.FieldFunction), n.ChildByField(syntax.FieldArguments)
		if fn == nil || args == nil {
			return true
		}
		text := string(f.Src[fn.Span.Start:fn.Span.End])
		if text == name || strings.HasSuffix(text, "."+name) {
			at = args.Span.Start
		}
		return true
	})
	return name, at, at >= 0
}

// writeFixes inserts a `!` at each offset of the file, formats it and writes
// it back.
func writeFixes(f *ir.File, ats []int, stderr io.Writer) bool {
	src, err := os.ReadFile(f.Path)
	if err != nil {
		fmt.Fprintf(stderr, "eye fix: %v\n", err)
		return false
	}
	if string(src) != string(f.Src) {
		fmt.Fprintf(stderr, "eye fix: %s changed while it was being checked; run eye fix again\n", f.Path)
		return false
	}
	sort.Sort(sort.Reverse(sort.IntSlice(ats)))
	out := src
	prev := -1
	for _, at := range ats {
		if at == prev {
			continue // two diagnostics at one call
		}
		prev = at
		out = append(append(append([]byte{}, out[:at]...), '!'), out[at:]...)
	}
	formatted, ok := format(f.Path, out, stderr)
	if !ok {
		return false
	}
	if err := os.WriteFile(f.Path, formatted, 0o644); err != nil {
		fmt.Fprintf(stderr, "eye fix: %v\n", err)
		return false
	}
	return true
}
