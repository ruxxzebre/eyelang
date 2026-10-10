package main

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

// journalPath is where a program's journal lives unless --journal says
// otherwise: .eye/journal in its directory.
func journalPath(dir string) string { return filepath.Join(dir, ".eye", "journal") }

// runRun implements `eye run [--journal file | --memory] [program-directory] [-- arguments…]`.
func runRun(args []string, stdout, stderr io.Writer) int {
	var progArgs []string
	for i, a := range args {
		if a == "--" {
			progArgs = args[i+1:]
			args = args[:i]
			break
		}
	}
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	journal := fs.String("journal", "", "the journal that keeps entities, runs and events (default program-directory/.eye/journal)")
	memory := fs.Bool("memory", false, "keep everything in memory, so nothing survives the process")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: eye run [--journal file | --memory] [program-directory] [-- arguments…]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Runs main, then keeps the program's jobs going: sleeping runs wake on time,")
		fmt.Fprintln(stderr, "schedules fire, and a restart carries on where the last run of eye stopped.")
		fmt.Fprintln(stderr, "It stops when nothing is left to wait for, or on Ctrl-C or SIGTERM.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	dir := "."
	switch fs.NArg() {
	case 0:
	case 1:
		dir = fs.Arg(0)
		if info, err := os.Stat(dir); err == nil && !info.IsDir() {
			fmt.Fprintln(stderr, "eye run takes the program's directory; its .eye files are one module")
			return exitUsage
		}
	default:
		fs.Usage()
		return exitUsage
	}
	prog, diags := check.Program(dir)
	if len(diags) > 0 {
		for _, d := range diags {
			fmt.Fprintln(stderr, d)
		}
		return exitErrors
	}
	opts := interp.RunOptions{Args: append([]string{dir}, progArgs...), Stdout: stdout, Stderr: stderr}
	if !*memory {
		path := *journal
		if path == "" {
			path = journalPath(dir)
		}
		j, err := interp.OpenJournal(path)
		if err != nil {
			fmt.Fprintf(stderr, "eye run: %v\n", err)
			return exitErrors
		}
		defer j.Close()
		opts.Store = j
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return interp.Run(ctx, prog, opts)
}
