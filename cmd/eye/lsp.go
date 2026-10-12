package main

import (
	"flag"
	"fmt"
	"io"

	"eyelang/internal/lsp"
)

// runLSP implements `eye lsp`: a language server on standard input and output.
func runLSP(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("eye lsp", flag.ContinueOnError)
	flags.SetOutput(stderr)
	probes := []struct{ method, usage string }{
		{"definition", "print where the name or keyword at `FILE:LINE:COLUMN` is defined, and exit"},
		{"type-definition", "print where the type of what is at `FILE:LINE:COLUMN` is declared, and exit"},
		{"references", "print every use of the name at `FILE:LINE:COLUMN`, and exit"},
		{"implementation", "print every place eye implements the keyword at `FILE:LINE:COLUMN`, and exit"},
		{"declaration", "print the SPECS.md section of the keyword at `FILE:LINE:COLUMN`, and exit"},
		{"hover", "print the signature and doc comment of the name at `FILE:LINE:COLUMN`, and exit"},
	}
	at := map[string]*string{}
	for _, p := range probes {
		at[p.method] = flags.String(p.method, "", p.usage)
	}
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: eye lsp [--definition | --type-definition | --references | --implementation | --declaration | --hover FILE:LINE:COLUMN]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Runs the language server, speaking LSP on standard input and output.")
		fmt.Fprintln(stderr, "Editors start it; see editors/vscode. With one of the flags it answers one")
		fmt.Fprintln(stderr, "request as an editor would get it and prints the locations as")
		fmt.Fprintln(stderr, "path:line:column, or a hover's Markdown, which checks the server")
		fmt.Fprintln(stderr, "without an editor.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	method, pos := "", ""
	for _, p := range probes {
		if *at[p.method] == "" {
			continue
		}
		if method != "" {
			flags.Usage()
			return exitUsage
		}
		method, pos = p.method, *at[p.method]
	}
	if method != "" {
		if flags.NArg() > 0 {
			flags.Usage()
			return exitUsage
		}
		if err := lsp.Probe(method, pos, stdout); err != nil {
			fmt.Fprintf(stderr, "eye lsp: %v\n", err)
			return exitErrors
		}
		return 0
	}
	if err := lsp.Serve(stdin, stdout); err != nil {
		fmt.Fprintf(stderr, "eye lsp: %v\n", err)
		return exitErrors
	}
	return 0
}
