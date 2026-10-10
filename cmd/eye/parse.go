package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"eyelang/syntax"
)

// runParse implements `eye parse [--sexp] file...`.
func runParse(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("eye parse", flag.ContinueOnError)
	fs.SetOutput(stderr)
	sexp := fs.Bool("sexp", false, "print the normalised S-expression the conformance suite compares, one file per line")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: eye parse [--sexp] file.eye...")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Prints the syntax tree of each file and reports syntax errors.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return exitUsage
	}

	code := 0
	for _, path := range fs.Args() {
		src, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "eye parse: %v\n", err)
			code = exitErrors
			continue
		}
		f := syntax.Parse(src)
		opts := syntax.SExprOptions{Fields: true, Text: true, Indent: true}
		if *sexp {
			opts = syntax.SExprOptions{Fields: true, Tokens: true, Text: true}
		}
		fmt.Fprintln(stdout, f.Root.SExpr(src, opts))
		for _, d := range f.Diagnostics {
			pos := f.Lines.Position(d.Span.Start)
			fmt.Fprintf(stderr, "%s:%d:%d: %s\n", path, pos.Line, pos.Column, d.Message)
		}
		if len(f.Diagnostics) > 0 {
			code = exitErrors
		}
	}
	return code
}
