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
	"os/exec"
	"path/filepath"
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
	phase   int                                               // ROADMAP.md phase that implements it
	issue   int                                               // GitHub issue tracking that phase
}

var commands []command

func init() {
	commands = []command{
		{name: "parse", summary: "print the syntax tree of files", run: runParse, phase: 1, issue: 2},
		{name: "fmt", summary: "format source files", run: runFmt, phase: 1, issue: 2},
		{name: "fix", summary: "rewrite code the language changed under, such as a durable step without its !", run: runFix, phase: 5, issue: 52},
		{name: "doc", summary: "write the API reference as Markdown", run: runDoc, phase: 3, issue: 30},
		{name: "gen", summary: "write build output from a program's contracts: network policy, Go and TypeScript", run: runGen, phase: 6, issue: 57},
		{name: "check", summary: "report errors in a program: syntax, names, types and effects", run: runCheck, phase: 1, issue: 43},
		{name: "run", summary: "run a program: main, then its jobs, kept in a journal across restarts", run: runRun, phase: 3, issue: 31},
		{name: "runs", summary: "list a program's job runs: status, what each waits for, and steps", run: runRuns, phase: 4, issue: 34},
		{name: "approve", summary: "approve what a run of eye run waits for at approval!", run: runApprove, phase: 5, issue: 51},
		{name: "decline", summary: "decline what a run of eye run waits for at approval!", run: runDecline, phase: 5, issue: 51},
		{name: "test", summary: "run the test blocks of a program", run: runTest, phase: 3, issue: 31},
		{name: "build", summary: "compile a program to a native binary, through C", run: runBuild, phase: 7, issue: 77},
		{name: "repl", summary: "start an interactive session: expressions, declarations and jobs", run: runREPL, phase: 3, issue: 31},
		{name: "lsp", summary: "run the language server for editors", run: runLSP, phase: 1, issue: 43},
	}
}

func main() {
	if root := os.Getenv("EYE_CHECKOUT_ROOT"); root != "" {
		bootstrap := os.Getenv("EYE_CHECKOUT_BOOTSTRAP") == "1"
		_ = os.Unsetenv("EYE_CHECKOUT_ROOT")
		_ = os.Unsetenv("EYE_CHECKOUT_BOOTSTRAP")
		rebuilt, err := prepareCheckout(root, bootstrap, os.Stdout, os.Stderr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "eye:", err)
			os.Exit(exitErrors)
		}
		if rebuilt {
			cmd := exec.Command(filepath.Join(root, "bin", "eye"), os.Args[1:]...)
			cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := cmd.Run(); err != nil {
				if exit, ok := err.(*exec.ExitError); ok {
					os.Exit(exit.ExitCode())
				}
				fmt.Fprintln(os.Stderr, "eye:", err)
				os.Exit(exitErrors)
			}
			os.Exit(0)
		}
	}
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
		if len(args) > 1 && args[0] == "help" {
			return help(args[1], stdout, stderr)
		}
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

// help is `eye help CMD`: what `eye CMD -h` prints, on stdout.
func help(name string, stdout, stderr io.Writer) int {
	switch name {
	case "help", "version":
		usage(stdout)
		return 0
	}
	for _, c := range commands {
		if c.name != name {
			continue
		}
		if c.run == nil {
			fmt.Fprintf(stdout, "eye %s: %s; not implemented yet, planned in phase %d (https://github.com/ruxxzebre/eyelang/issues/%d)\n",
				c.name, c.summary, c.phase, c.issue)
			return 0
		}
		c.run([]string{"-h"}, stdout, stdout)
		return 0
	}
	fmt.Fprintf(stderr, "eye help: unknown command %q\n\n", name)
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
	fmt.Fprintf(w, "  %-8s %s\n", "help", "show this message, or a command's: eye help build")
}
