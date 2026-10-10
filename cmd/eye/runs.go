package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"eyelang/check"
	"eyelang/interp"
)

// runRuns implements `eye runs [--journal file] [--json] [program-directory] [run-id]`.
func runRuns(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("runs", flag.ContinueOnError)
	fs.SetOutput(stderr)
	journal := fs.String("journal", "", "the journal to read (default program-directory/.eye/journal)")
	asJSON := fs.Bool("json", false, "print the runs as JSON")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: eye runs [--journal file] [--json] [program-directory] [run-id]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Lists the job runs in a program's journal: each run's status, what it waits")
		fmt.Fprintln(stderr, "for and until when, and with a run's id, its steps. It reads the journal")
		fmt.Fprintln(stderr, "without changing it, so it works while eye run is running.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	dir, id := ".", int64(-1)
	rest := fs.Args()
	if len(rest) > 0 {
		if n, err := strconv.ParseInt(rest[len(rest)-1], 10, 64); err == nil {
			id = n
			rest = rest[:len(rest)-1]
		}
	}
	switch len(rest) {
	case 0:
	case 1:
		dir = rest[0]
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
	path := *journal
	if path == "" {
		path = journalPath(dir)
	}
	store, err := interp.ReadJournal(path)
	if os.IsNotExist(err) {
		fmt.Fprintf(stderr, "eye runs: %s has no journal yet; eye run %s makes one\n", dir, dir)
		return exitErrors
	}
	if err != nil {
		fmt.Fprintf(stderr, "eye runs: %v\n", err)
		return exitErrors
	}
	runs := interp.Runs(prog, store)
	if id >= 0 {
		var one []interp.RunInfo
		for _, r := range runs {
			if r.ID == id {
				one = append(one, r)
			}
		}
		if len(one) == 0 {
			fmt.Fprintf(stderr, "eye runs: there is no run %d\n", id)
			return exitErrors
		}
		runs = one
	}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if runs == nil {
			runs = []interp.RunInfo{}
		}
		if err := enc.Encode(runs); err != nil {
			fmt.Fprintln(stderr, err)
			return exitErrors
		}
		return 0
	}
	if stats, err := interp.StatJournal(path); err == nil && id < 0 {
		interp.FormatJournal(stdout, path, stats, runs)
	}
	interp.FormatRuns(stdout, runs, id >= 0, time.Now())
	return 0
}
