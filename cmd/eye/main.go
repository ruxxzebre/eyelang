// Command eye is the Eyelang 2 toolchain: it parses, checks, formats, runs and
// tests Eyelang programs.
//
// Subcommands that aren't implemented yet say which phase of ROADMAP.md brings
// them.
package main

import (
	"fmt"
	"io"
	"os"
)

// version is set for releases with -ldflags "-X main.version=v0.1.0".
var version = "0.0.0-dev"

// Exit codes.
const (
	exitErrors = 1 // the input has errors
	exitUsage  = 2 // the command line is wrong, or the command isn't implemented yet
)

type command struct {
	name    string
	summary string
	run     func(args []string, stdout, stderr io.Writer) int // nil until implemented
	phase   int                                                // ROADMAP.md phase that implements it
	issue   int                                                // GitHub issue tracking that phase
}

var commands []command

func init() {
	commands = []command{
		{name: "parse", summary: "print the syntax tree of files", run: runParse, phase: 1, issue: 2},
		{name: "fmt", summary: "format source files", run: runFmt, phase: 1, issue: 2},
		{name: "check", summary: "report errors in files (syntax only until phase 2)", run: runCheck, phase: 1, issue: 43},
		{name: "run", summary: "run a program", phase: 3, issue: 4},
		{name: "test", summary: "run the test blocks of a program", phase: 3, issue: 4},
		{name: "repl", summary: "start an interactive session", phase: 3, issue: 4},
		{name: "lsp", summary: "run the language server for editors", run: runLSP, phase: 1, issue: 43},
	}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes one eye invocation and returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stdout)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		usage(stdout)
		return 0
	case "version", "--version":
		fmt.Fprintf(stdout, "eye %s\n", version)
		return 0
	}

	for _, c := range commands {
		if c.name != args[0] {
			continue
		}
		if c.run == nil {
			fmt.Fprintf(stderr, "eye %s: not implemented yet; planned in phase %d (https://github.com/ruxxzebre/eyelang/issues/%d)\n",
				c.name, c.phase, c.issue)
			return exitUsage
		}
		return c.run(args[1:], stdout, stderr)
	}

	fmt.Fprintf(stderr, "eye: unknown command %q\n\n", args[0])
	usage(stderr)
	return exitUsage
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: eye <command> [arguments]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "commands:")
	for _, c := range commands {
		fmt.Fprintf(w, "  %-8s %s\n", c.name, c.summary)
	}
	fmt.Fprintf(w, "  %-8s %s\n", "version", "print the eye version")
	fmt.Fprintf(w, "  %-8s %s\n", "help", "show this message")
}
