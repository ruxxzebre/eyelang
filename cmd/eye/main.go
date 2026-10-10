// Command eye is the Eyelang 2 toolchain: it parses, checks, formats, runs and
// tests Eyelang programs.
//
// Most subcommands are placeholders until their phase of ROADMAP.md lands.
package main

import (
	"fmt"
	"io"
	"os"
)

// version is set for releases with -ldflags "-X main.version=v0.1.0".
var version = "0.0.0-dev"

// exitUsage is the exit code for unknown or not yet implemented commands.
const exitUsage = 2

type command struct {
	name    string
	summary string
	phase   int // ROADMAP.md phase that implements it
	issue   int // GitHub issue tracking that phase
}

var commands = []command{
	{name: "parse", summary: "print the syntax tree of a file", phase: 1, issue: 2},
	{name: "fmt", summary: "format source files", phase: 1, issue: 2},
	{name: "check", summary: "check a program and report diagnostics", phase: 2, issue: 3},
	{name: "run", summary: "run a program", phase: 3, issue: 4},
	{name: "test", summary: "run the test blocks of a program", phase: 3, issue: 4},
	{name: "repl", summary: "start an interactive session", phase: 3, issue: 4},
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
		if c.name == args[0] {
			fmt.Fprintf(stderr, "eye %s: not implemented yet; planned in phase %d (https://github.com/ruxxzebre/eyelang/issues/%d)\n",
				c.name, c.phase, c.issue)
			return exitUsage
		}
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
