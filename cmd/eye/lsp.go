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
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: eye lsp")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Runs the language server, speaking LSP on standard input and output.")
		fmt.Fprintln(stderr, "Editors start it; see editors/vscode.")
	}
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if err := lsp.Serve(stdin, stdout); err != nil {
		fmt.Fprintf(stderr, "eye lsp: %v\n", err)
		return exitErrors
	}
	return 0
}
