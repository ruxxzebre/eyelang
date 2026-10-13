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

// Main runs the program as `eye run` does, with the compiled program
// hosting what it compiled: main first, then its jobs and schedules on the
// system clock until nothing is left to happen or the process is told to
// stop. args are the process's: the program's directory, whose source gives
// the runner the declarations it schedules, and where its state is kept.
// It returns the exit status.
//
//	program [--journal file | --store postgres://…] program-directory
func Main(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(filepath.Base(args[0]), flag.ContinueOnError)
	fs.SetOutput(stderr)
	journal := fs.String("journal", "", "the journal that keeps entities, runs and events (default program-directory/.eye/journal)")
	store := fs.String("store", "", "keep entities, runs and events in this Postgres database, postgres://…")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintf(stderr, "usage: %s [--journal file | --store postgres://…] program-directory\n", fs.Name())
		return 2
	}
	dir := fs.Arg(0)
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		for _, d := range diags {
			fmt.Fprintln(stderr, d)
		}
		return 1
	}
	native, err := Load()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	opts := interp.RunOptions{Args: []string{dir}, Stdout: stdout, Stderr: stderr, Native: native}
	if *store != "" {
		pg, err := interp.WaitForPostgres(ctx, *store, stderr)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer pg.Close()
		opts.Store = pg
	} else {
		path := *journal
		if path == "" {
			path = filepath.Join(dir, ".eye", "journal")
		}
		j, err := interp.OpenJournal(path)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer j.Close()
		opts.Store = j
	}
	return interp.Run(ctx, prog, opts)
}
