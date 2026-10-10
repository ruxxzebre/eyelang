package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"eyelang/check"
	"eyelang/interp"
)

// runTest implements `eye test [--run text] [-v] [program-directory]`.
func runTest(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("eye test", flag.ContinueOnError)
	flags.SetOutput(stderr)
	filter := flags.String("run", "", "run only the tests whose names contain this text")
	verbose := flags.Bool("v", false, "show what passing tests logged too")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: eye test [--run text] [-v] [program-directory]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Runs the test blocks of a program's module, each with a fresh in-memory store")
		fmt.Fprintln(stderr, "and a clock that moves only when the test advances it.")
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
	results := interp.RunTests(prog, *filter)
	failed := 0
	var total time.Duration
	for _, r := range results {
		total += r.Duration
		if r.Failure == "" {
			fmt.Fprintf(stdout, "ok    %s (%s)\n", r.Name, r.Duration.Round(time.Microsecond))
			if *verbose && r.Log != "" {
				fmt.Fprint(stdout, indent(r.Log))
			}
			continue
		}
		failed++
		fmt.Fprintf(stdout, "FAIL  %s (%s)\n", r.Name, r.Duration.Round(time.Microsecond))
		fmt.Fprintln(stdout, indent(r.Failure))
		if r.Log != "" {
			fmt.Fprint(stdout, indent(r.Log))
		}
	}
	switch {
	case len(results) == 0:
		fmt.Fprintln(stdout, "no tests to run")
	case failed > 0:
		fmt.Fprintf(stdout, "%d of %d tests failed\n", failed, len(results))
		return exitErrors
	default:
		fmt.Fprintf(stdout, "%d tests passed (%s)\n", len(results), total.Round(time.Microsecond))
	}
	return 0
}

func indent(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		b.WriteString("      " + line + "\n")
	}
	return b.String()
}
