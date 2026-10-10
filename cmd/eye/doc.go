package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"eyelang/check"
	"eyelang/internal/apidoc"
)

// runDoc implements `eye doc [--out dir] [program-directory]`.
func runDoc(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("eye doc", flag.ContinueOnError)
	flags.SetOutput(stderr)
	out := flags.String("out", "api", "the directory to write the Markdown pages to; it is replaced")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: eye doc [--out dir] [program-directory]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Writes the API reference as Markdown: the standard library and, given a program,")
		fmt.Fprintln(stderr, "its modules. Each public declaration gets its signature and its ## comment.")
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
		if err := os.WriteFile(tmp+"/empty.eye", []byte("# The standard library alone.\n"), 0o644); err != nil {
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
	pages := apidoc.Pages(prog)
	if flags.NArg() == 0 {
		pages = stdOnly(pages)
	}
	if err := apidoc.Write(*out, pages); err != nil {
		fmt.Fprintf(stderr, "eye doc: %v\n", err)
		return exitErrors
	}
	fmt.Fprintf(stdout, "wrote %d pages to %s\n", len(pages), *out)
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
