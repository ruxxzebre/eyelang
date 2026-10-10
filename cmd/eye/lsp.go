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
	definition := flags.String("definition", "", "print where the name at `FILE:LINE:COLUMN` is declared, and exit")
	references := flags.String("references", "", "print every use of the name at `FILE:LINE:COLUMN`, and exit")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: eye lsp [--definition | --references FILE:LINE:COLUMN]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Runs the language server, speaking LSP on standard input and output.")
		fmt.Fprintln(stderr, "Editors start it; see editors/vscode. With --definition or --references it")
		fmt.Fprintln(stderr, "answers one request as an editor would get it and prints the locations as")
		fmt.Fprintln(stderr, "path:line:column, which checks the server without an editor.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if *definition != "" || *references != "" {
		method, at := "definition", *definition
		if *references != "" {
			method, at = "references", *references
		}
		if *definition != "" && *references != "" || flags.NArg() > 0 {
			flags.Usage()
			return exitUsage
		}
		if err := lsp.Probe(method, at, stdout); err != nil {
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
