//go:build eyehosted

package runner

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"eyelang/check"
	"eyelang/interp"
)

// root is where the program's files are, as the checker reads them: held
// in memory, under a directory that needn't exist.
const root = "/eye-program"

// Main runs the program as `eye run` does, with the compiled program
// hosting what it compiled: main first, then its jobs and schedules on the
// system clock until nothing is left to happen or the process is told to
// stop. With --test it runs the program's tests instead, as `eye test` does.
// sources are the program's files, by their paths below its root, which the
// binary carries: the runner checks them from memory when it starts, for
// the declarations it schedules and what the compiled program leaves to the
// interpreter. It returns the exit status.
//
//	program [--journal file | --store postgres://…] [-- arguments…]
//	program --test [--run text] [-v]
func Main(sources map[string]string, args []string, stdout, stderr io.Writer) int {
	var progArgs []string
	for i, a := range args {
		if a == "--" {
			progArgs, args = args[i+1:], args[:i]
			break
		}
	}
	name := filepath.Base(args[0])
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	journal := fs.String("journal", filepath.Join(".eye", "journal"), "the journal that keeps entities, runs and events")
	store := fs.String("store", "", "keep entities, runs and events in this Postgres database, postgres://…")
	tests := fs.Bool("test", false, "run the program's tests, as eye test does, instead of the program")
	filter := fs.String("run", "", "with --test, run only the tests whose names hold this text")
	verbose := fs.Bool("v", false, "with --test, print what each test logged")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "usage: %s [--journal file | --store postgres://…] [-- arguments…], or %s --test [--run text] [-v]\n", name, name)
		return 2
	}
	overlay := map[string][]byte{}
	for p, text := range sources {
		overlay[filepath.Join(root, filepath.FromSlash(p))] = []byte(text)
	}
	res := check.Check(root, check.Options{Overlay: overlay})
	if len(res.Diagnostics) > 0 {
		for _, d := range res.Diagnostics {
			fmt.Fprintln(stderr, d)
		}
		return 1
	}
	native, err := Load()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *tests {
		return runTests(res, native, *filter, *verbose, stdout)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	opts := interp.RunOptions{Args: append([]string{name}, progArgs...), Stdout: stdout, Stderr: stderr, Native: native}
	if *store != "" {
		pg, err := interp.WaitForPostgres(ctx, *store, stderr)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer pg.Close()
		opts.Store = pg
	} else {
		j, err := interp.OpenJournal(*journal)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer j.Close()
		opts.Store = j
	}
	return interp.Run(ctx, res.Program, opts)
}

// runTests runs the program's tests, printing a line for each and a
// summary, and returns 1 if one failed.
func runTests(res *check.Result, native *Program, filter string, verbose bool, out io.Writer) int {
	results, _, err := interp.RunTestsWith(res.Program, interp.TestOptions{Filter: filter, Native: native})
	if err != nil {
		fmt.Fprintln(out, err)
		return 1
	}
	failed := 0
	for _, r := range results {
		switch {
		case r.Skipped != "":
			fmt.Fprintf(out, "skip  %s: %s\n", r.Name, r.Skipped)
		case r.Failure != "":
			failed++
			fmt.Fprintf(out, "FAIL  %s\n      %s\n", r.Name, r.Failure)
		default:
			fmt.Fprintf(out, "ok    %s\n", r.Name)
		}
		if verbose && r.Log != "" {
			fmt.Fprint(out, r.Log)
		}
	}
	switch {
	case len(results) == 0:
		fmt.Fprintln(out, "no tests to run")
	case failed > 0:
		fmt.Fprintf(out, "%d of %d tests failed\n", failed, len(results))
		return 1
	default:
		fmt.Fprintf(out, "%d tests passed\n", len(results))
	}
	return 0
}
