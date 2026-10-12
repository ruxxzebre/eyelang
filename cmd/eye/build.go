package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"eyelang/cgen"
	"eyelang/check"
)

// runBuild implements `eye build [--test] [--target os/arch] [-o file] [-g] [--emit-c dir] [program-directory]`.
func runBuild(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("eye build", flag.ContinueOnError)
	flags.SetOutput(stderr)
	tests := flags.Bool("test", false, "build a binary that runs the program's test blocks, as eye test does")
	target := flags.String("target", "", "the platform to build for: "+strings.Join(cgen.TargetNames(), ", ")+" (default: this machine)")
	out := flags.String("o", "", "the binary to write (default: the program's directory name, with .test for --test)")
	debug := flags.Bool("g", false, "compile without optimisation and with debug information (-O0 -g)")
	emitC := flags.String("emit-c", "", "write the C, the runtime's with it, into this directory instead of compiling it")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: eye build [--test] [--target os/arch] [-o file] [-g] [--emit-c dir] [program-directory]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Compiles a program to a native binary: C from the low IR, built with the zig")
		fmt.Fprintln(stderr, "that eye pins, fetched into the user's cache the first time. Object files are")
		fmt.Fprintln(stderr, "cached by the hash of what made them. The binary runs main, or with --test the")
		fmt.Fprintln(stderr, "program's tests. The C back end compiles the core language; entities, jobs,")
		fmt.Fprintln(stderr, "views and contracts still run in eye run.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() > 1 {
		flags.Usage()
		return exitUsage
	}
	dir := "."
	if flags.NArg() == 1 {
		dir = flags.Arg(0)
	}
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		for _, d := range diags {
			fmt.Fprintln(stdout, d)
		}
		return exitErrors
	}
	if *out == "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			fmt.Fprintf(stderr, "eye build: %v\n", err)
			return exitErrors
		}
		*out = filepath.Base(abs)
		if *tests {
			*out += ".test"
		}
	}
	err := cgen.Build(prog, cgen.BuildOptions{Out: *out, Target: *target, Tests: *tests, Debug: *debug, EmitC: *emitC, Progress: stderr})
	if err != nil {
		fmt.Fprintf(stderr, "eye build: %v\n", err)
		return exitErrors
	}
	return 0
}
