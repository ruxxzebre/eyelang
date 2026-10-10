package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"

	"eyelang/syntax"
)

// runCheck implements `eye check [path...]`. Until phase 2 brings the type
// checker it reports syntax errors only.
func runCheck(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("eye check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: eye check [file.eye | directory]...")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Reports errors as file:line:column: message and exits 1 if there are any.")
		fmt.Fprintln(stderr, "Directories are searched for .eye files; with no paths, checks the current")
		fmt.Fprintln(stderr, "directory. Only syntax is checked until the type checker lands (phase 2).")
	}
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	paths := flags.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	files, err := eyeFiles(paths)
	if err != nil {
		fmt.Fprintf(stderr, "eye check: %v\n", err)
		return exitErrors
	}

	// Files are independent, so they are parsed in parallel; the reports are
	// printed in the order of the files.
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
