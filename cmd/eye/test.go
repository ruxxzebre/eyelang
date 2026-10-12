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

// runTest implements `eye test [--run text] [-v] [--live modules [--record]] [--crossings] [program-directory]`.
func runTest(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("eye test", flag.ContinueOnError)
	flags.SetOutput(stderr)
	filter := flags.String("run", "", "run only the tests whose names contain this text")
	verbose := flags.Bool("v", false, "show what passing tests logged too")
	live := flags.String("live", "", "call the real services of these `modules`, separated by commas (ai, http or contract modules),\ninstead of replaying their recordings; a test whose service has no address or secret set is skipped")
	record := flags.Bool("record", false, "with --live, write what the real services answered as the recordings\nthe fakes read, for the tests that passed")
	crossings := flags.Bool("crossings", false, "count, for each test and each job run in it, the operations that would cross\nbetween compiled code and the runner, and print them by kind at the end")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: eye test [--run text] [-v] [--live modules [--record]] [--crossings] [program-directory]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Runs the test blocks of a program's module, each with a fresh in-memory store")
		fmt.Fprintln(stderr, "and a clock that moves only when the test advances it. Calls to other services")
		fmt.Fprintln(stderr, "are answered from recordings, except for the modules --live names, whose calls")
		fmt.Fprintln(stderr, "go where eye run sends them, with the address and secret from the environment.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() > 1 {
		flags.Usage()
		return exitUsage
	}
	var names []string
	for _, n := range strings.Split(*live, ",") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	if *record && len(names) == 0 {
		fmt.Fprintln(stderr, "eye test: --record writes what the real services answered, so it needs --live and the modules to call, such as --live ai,notify")
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
	mods, err := interp.LiveModules(prog, names)
	if err != nil {
		fmt.Fprintln(stderr, "eye test:", err)
		return exitUsage
	}
	results, recorded, recErr := interp.RunTestsWith(prog, interp.TestOptions{Filter: *filter, Live: mods, Record: *record, Crossings: *crossings})
	failed, skipped := 0, 0
	var total time.Duration
	for _, r := range results {
		total += r.Duration
		switch {
		case r.Skipped != "":
			skipped++
			fmt.Fprintf(stdout, "skip  %s\n", r.Name)
			fmt.Fprint(stdout, indent(r.Skipped))
		case r.Failure == "":
			fmt.Fprintf(stdout, "ok    %s (%s)\n", r.Name, r.Duration.Round(time.Microsecond))
			if *verbose && r.Log != "" {
				fmt.Fprint(stdout, indent(r.Log))
			}
		default:
			failed++
			fmt.Fprintf(stdout, "FAIL  %s (%s)\n", r.Name, r.Duration.Round(time.Microsecond))
			fmt.Fprintln(stdout, indent(r.Failure))
			if r.Log != "" {
				fmt.Fprint(stdout, indent(r.Log))
			}
		}
	}
	if *crossings {
		writeCrossings(stdout, results)
	}
	for _, f := range recorded {
		fmt.Fprintf(stdout, "recorded %s (%s)\n", f.Path, plural(f.Exchanges, "exchange"))
	}
	if *record && len(recorded) == 0 && recErr == nil {
		fmt.Fprintln(stdout, "recorded nothing: no test that passed called a live service through `with fakes(…)`")
	}
	if recErr != nil {
		fmt.Fprintln(stdout, "eye test: recording:", recErr)
	}
	also := ""
	if skipped > 0 {
		also = fmt.Sprintf(", %d skipped", skipped)
	}
	switch {
	case len(results) == 0:
		fmt.Fprintln(stdout, "no tests to run")
	case failed > 0:
		fmt.Fprintf(stdout, "%d of %d tests failed%s\n", failed, len(results), also)
		return exitErrors
	default:
		fmt.Fprintf(stdout, "%d tests passed%s (%s)\n", len(results)-skipped, also, total.Round(time.Microsecond))
	}
	if recErr != nil {
		return exitErrors
	}
	return 0
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return count(n) + " " + word + "s"
}

func indent(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		b.WriteString("      " + line + "\n")
	}
	return b.String()
}

// writeCrossings prints each test's crossings: its body's, then each job
// run's.
func writeCrossings(w io.Writer, results []interp.TestResult) {
	fmt.Fprintln(w, "\ncrossings between compiled code and the runner (ROADMAP 7.10)")
	for _, r := range results {
		if r.Skipped != "" {
			continue
		}
		fmt.Fprintf(w, "\ntest %q\n", r.Name)
		var b strings.Builder
		interp.WriteCrossings(&b, r.Crossings)
		fmt.Fprint(w, indent(b.String()))
	}
	fmt.Fprintln(w)
}
