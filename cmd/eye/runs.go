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

// runRuns implements `eye runs [--store url | --journal file] [--json] [program-directory] [run-id [step]]`.
func runRuns(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("runs", flag.ContinueOnError)
	fs.SetOutput(stderr)
	journal := fs.String("journal", "", "the journal to read (default program-directory/.eye/journal)")
	store := fs.String("store", "", "read the store in this Postgres database, postgres://… (default $EYE_STORE)")
	asJSON := fs.Bool("json", false, "print the runs as JSON")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: eye runs [--store url | --journal file] [--json] [program-directory] [run-id [step]]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Lists the job runs in a program's journal, or its Postgres store: each run's")
		fmt.Fprintln(stderr, "status, what it waits for and until when, and with a run's id, its steps, with")
		fmt.Fprintln(stderr, "what each model call spent. With a step's number too, it shows that step's")
		fmt.Fprintln(stderr, "whole result, such as a model's reply. It reads without changing anything, so")
		fmt.Fprintln(stderr, "it works while eye run is running.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	dir, id, stepNo := ".", int64(-1), 0
	rest := fs.Args()
	var nums []int64 // the run's id, and the step's number, at the end
	for len(rest) > 0 && len(nums) < 2 {
		n, err := strconv.ParseInt(rest[len(rest)-1], 10, 64)
		if err != nil {
			break
		}
		nums = append([]int64{n}, nums...)
		rest = rest[:len(rest)-1]
	}
	if len(nums) > 0 {
		id = nums[0]
	}
	if len(nums) == 2 {
		stepNo = int(nums[1])
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
	url := *store
	if *journal == "" {
		url = storeURL(url)
	}
	if url != "" && *journal != "" {
		fmt.Fprintln(stderr, "eye runs reads one store: give --store or --journal, not both")
		return exitUsage
	}
	path := *journal
	if path == "" {
		path = journalPath(dir)
	}
	var st interp.Store
	var err error
	if url != "" {
		st, err = interp.ReadPostgres(url)
	} else {
		st, err = interp.ReadJournal(path)
		if os.IsNotExist(err) {
			fmt.Fprintf(stderr, "eye runs: %s has no journal yet; eye run %s makes one\n", dir, dir)
			return exitErrors
		}
	}
	if err != nil {
		fmt.Fprintf(stderr, "eye runs: %v\n", err)
		return exitErrors
	}
	runs := interp.Runs(prog, st)
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
	if stepNo != 0 {
		value, err := interp.StepValue(prog, st, id, stepNo)
		if err != nil {
			fmt.Fprintf(stderr, "eye runs: %v\n", err)
			return exitErrors
		}
		step := runs[0].Steps[stepNo-1]
		if *asJSON {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(struct {
				interp.StepInfo
				Whole string `json:"whole"`
			}{step, value})
			return 0
		}
		interp.FormatRuns(stdout, runs, false, time.Now())
		interp.FormatStep(stdout, stepNo, step, time.Now())
		fmt.Fprintln(stdout, value)
		return 0
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
	if stats, err := interp.StatJournal(path); err == nil && id < 0 && url == "" {
		interp.FormatJournal(stdout, path, stats, runs)
	}
	interp.FormatRuns(stdout, runs, id >= 0, time.Now())
	return 0
}
