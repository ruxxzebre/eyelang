package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"eyelang/check"
	"eyelang/internal/apidoc"
)

// runDoc implements `eye doc [--out dir] [program-directory]`.
func runDoc(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("eye doc", flag.ContinueOnError)
	flags.SetOutput(stderr)
	out := flags.String("out", "api", "the directory to write the Markdown pages to")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: eye doc [--out dir] [program-directory]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Writes the API reference as Markdown: the standard library and, given a program,")
		fmt.Fprintln(stderr, "its modules. Each public declaration gets its signature and its ## comment.")
		fmt.Fprintln(stderr, "A [Name] in a ## comment links to that declaration; eye doc exits 1 if one")
		fmt.Fprintln(stderr, "names nothing.")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "The pages go to ./api in the current directory, or to the directory --out names.")
		fmt.Fprintln(stderr, "eye doc replaces that directory only if it is missing, empty, or holds nothing")
		fmt.Fprintln(stderr, "but pages eye doc wrote. If it holds anything else, eye doc stops, changes")
		fmt.Fprintln(stderr, "nothing and exits 1: pass --out with another directory.")
		fmt.Fprintln(stderr)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() > 1 {
		flags.Usage()
		return exitUsage
	}
	dir := ""
	if flags.NArg() == 1 {
		dir = flags.Arg(0)
	} else {
		// The standard library only: an empty program.
		tmp, err := os.MkdirTemp("", "eye-doc")
		if err != nil {
			fmt.Fprintf(stderr, "eye doc: %v\n", err)
			return exitErrors
		}
		defer os.RemoveAll(tmp)
		// The standard library alone: a program that imports every module of it.
		var src strings.Builder
		for _, m := range check.StdModules() {
			src.WriteString("import " + m + "\n")
		}
		if err := os.WriteFile(tmp+"/std.eye", []byte(src.String()), 0o644); err != nil {
			fmt.Fprintf(stderr, "eye doc: %v\n", err)
			return exitErrors
		}
		dir = tmp
	}
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		for _, d := range diags {
			fmt.Fprintln(stderr, d)
		}
		return exitErrors
	}
	pages, problems := apidoc.Pages(prog)
	if flags.NArg() == 0 {
		pages = stdOnly(pages)
	}
	if err := apidoc.Write(*out, pages); err != nil {
		fmt.Fprintf(stderr, "eye doc: %v\n", err)
		return exitErrors
	}
	fmt.Fprintf(stdout, "wrote %d pages to %s\n", len(pages), *out)
	for _, p := range problems {
		fmt.Fprintln(stderr, p)
	}
	if len(problems) > 0 {
		return exitErrors
	}
	return 0
}

// stdOnly drops the placeholder module of an empty program.
func stdOnly(pages []apidoc.Page) []apidoc.Page {
	var out []apidoc.Page
	for _, p := range pages {
		if p.Path != "main.md" {
			out = append(out, p)
		}
	}
	return apidoc.Reindex(out)
}
