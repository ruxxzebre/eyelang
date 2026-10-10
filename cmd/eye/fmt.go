package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"eyelang/syntax"
)

// stdin is standard input for `eye fmt` and `eye lsp`; tests replace it.
var stdin io.Reader = os.Stdin

// runFmt implements `eye fmt [--check] [path...]`.
func runFmt(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("eye fmt", flag.ContinueOnError)
	flags.SetOutput(stderr)
	check := flags.Bool("check", false, "list files that aren't formatted and exit 1 if there are any, without changing them")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: eye fmt [--check] [file.eye | directory]...")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Formats files in place; directories are searched for .eye files. With no")
		fmt.Fprintln(stderr, "paths, formats standard input to standard output.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	if flags.NArg() == 0 {
		src, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "eye fmt: %v\n", err)
			return exitErrors
		}
		out, ok := format("<stdin>", src, stderr)
		if !ok {
			return exitErrors
		}
		if *check {
			if !bytes.Equal(src, out) {
				fmt.Fprintln(stdout, "<stdin>")
				return exitErrors
			}
			return 0
		}
		_, _ = stdout.Write(out)
		return 0
	}

	files, err := eyeFiles(flags.Args())
	if err != nil {
		fmt.Fprintf(stderr, "eye fmt: %v\n", err)
		return exitErrors
	}
	code := 0
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "eye fmt: %v\n", err)
			code = exitErrors
			continue
		}
		out, ok := format(path, src, stderr)
		if !ok {
			code = exitErrors
			continue
		}
		if bytes.Equal(src, out) {
			continue
		}
		if *check {
			fmt.Fprintln(stdout, path)
			code = exitErrors
			continue
		}
		if err := os.WriteFile(path, out, 0o644); err != nil {
			fmt.Fprintf(stderr, "eye fmt: %v\n", err)
			code = exitErrors
		}
	}
	return code
}

// format formats src, reporting syntax errors as path:line:column.
func format(path string, src []byte, stderr io.Writer) ([]byte, bool) {
	f := syntax.Parse(src)
	if len(f.Diagnostics) > 0 {
		for _, d := range f.Diagnostics {
			pos := f.Lines.Position(d.Span.Start)
			fmt.Fprintf(stderr, "%s:%d:%d: %s\n", path, pos.Line, pos.Column, d.Message)
		}
		return nil, false
	}
	out, err := syntax.Format(src)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", path, err)
		return nil, false
	}
	return out, true
}

// eyeFiles expands directories into the .eye files under them.
func eyeFiles(paths []string) ([]string, error) {
	var files []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			files = append(files, p)
			continue
		}
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(path, ".eye") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}
