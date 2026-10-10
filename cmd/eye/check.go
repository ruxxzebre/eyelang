package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"eyelang/check"
	"eyelang/syntax"
)

// runCheck implements `eye check [--syntax] [path...]`.
func runCheck(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("eye check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	syntaxOnly := flags.Bool("syntax", false, "check only syntax, of files and of directories searched for .eye files")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: eye check [--syntax] [program-directory | file.eye]...")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Checks programs: a directory and the modules it imports, or the directory of a")
		fmt.Fprintln(stderr, "file. Reports errors as file:line:column: message and exits 1 if there are any.")
		fmt.Fprintln(stderr, "With no paths, checks the program in the current directory. Names, imports and")
		fmt.Fprintln(stderr, "where waiting calls may appear are checked; types come with phase 2.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	paths := flags.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	if *syntaxOnly {
		return checkSyntax(paths, stdout, stderr)
	}
	code := 0
	seen := map[string]bool{}
	for _, p := range paths {
		dir := p
		if info, err := os.Stat(p); err != nil {
			fmt.Fprintf(stderr, "eye check: %v\n", err)
			code = exitErrors
			continue
		} else if !info.IsDir() {
			dir = filepath.Dir(p)
		}
		if seen[dir] {
			continue
		}
		seen[dir] = true
		_, diags := check.Program(dir)
		for _, d := range diags {
			fmt.Fprintln(stdout, d)
			code = exitErrors
		}
	}
	return code
}

// checkSyntax reports the syntax errors of files, parsing them in parallel.
func checkSyntax(paths []string, stdout, stderr io.Writer) int {
	files, err := eyeFiles(paths)
	if err != nil {
		fmt.Fprintf(stderr, "eye check: %v\n", err)
		return exitErrors
	}
	reports := make([][]string, len(files))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	for i, path := range files {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			reports[i] = checkFile(path)
		}()
	}
	wg.Wait()

	code := 0
	for _, lines := range reports {
		for _, line := range lines {
			fmt.Fprintln(stdout, line)
			code = exitErrors
		}
	}
	return code
}

func checkFile(path string) []string {
	src, err := os.ReadFile(path)
	if err != nil {
		return []string{err.Error()}
	}
	f := syntax.Parse(src)
	var out []string
	for _, d := range f.Diagnostics {
		pos := f.Lines.Position(d.Span.Start)
		out = append(out, fmt.Sprintf("%s:%d:%d: %s", path, pos.Line, pos.Column, d.Message))
	}
	return out
}
