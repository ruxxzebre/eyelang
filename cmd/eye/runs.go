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
	"eyelang/ir"
)

// runRuns implements `eye runs [--store url | --journal file] [--json] [program-directory] [run-id [step]]`.
func runRuns(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("runs", flag.ContinueOnError)
	fs.SetOutput(stderr)
	journal := fs.String("journal", "", "the journal to read (default program-directory/.eye/journal)")
	store := fs.String("store", "", "read the store in this Postgres database, postgres://… (default $EYE_STORE)")
	asJSON := fs.Bool("json", false, "print the runs as JSON, with every value whole")
	replies := fs.Bool("show-replies", false, "with a run's id, show each model call's whole reply below its step")
	retry := new(int64)
	*retry = -1
	fs.Func("retry", "start the failed run with this id again, replaying the steps it recorded", func(s string) error {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n < 1 {
			return fmt.Errorf("--retry takes a run's id, such as 3, as eye runs lists it")
		}
		*retry = n
		return nil
	})
	retryFailed := fs.Bool("retry-failed", false, "start every failed run a retry can cure again")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: eye runs [--store url | --journal file] [--json | --show-replies] [program-directory] [run-id [step]]")
		fmt.Fprintln(stderr, "       eye runs [--store url | --journal file] --retry ID | --retry-failed [program-directory]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Lists the job runs in a program's journal, or its Postgres store: each run's")
		fmt.Fprintln(stderr, "status, what it waits for and until when, and with a run's id, its steps, with")
		fmt.Fprintln(stderr, "what each model call spent. With a step's number too, it shows that step's")
		fmt.Fprintln(stderr, "whole result, such as a model's reply. Without --retry, it reads without")
		fmt.Fprintln(stderr, "changing anything, so it works while eye run is running.")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "--retry ID starts a failed run again once its cause is fixed, and --retry-failed")
		fmt.Fprintln(stderr, "every failed run a retry can cure: those that ended as Unavailable (a service")
		fmt.Fprintln(stderr, "out of reach) or DeadlinePassed. A retried run keeps its id and key, and replays")
		fmt.Fprintln(stderr, "the steps it recorded, so a call that went through isn't made again. While")
		fmt.Fprintln(stderr, "eye run holds the store, the retry goes to its inbox and the run starts within")
		fmt.Fprintln(stderr, "a second; otherwise the next eye run carries it on.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	retrying := *retry >= 0 || *retryFailed
	if retrying && (*retry >= 0 && *retryFailed || *asJSON || *replies) {
		fmt.Fprintln(stderr, "eye runs: --retry and --retry-failed go alone, without each other, --json or --show-replies")
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
	if retrying {
		if len(nums) > 0 {
			fmt.Fprintln(stderr, "eye runs: --retry takes the run's id; give no other run or step")
			return exitUsage
		}
		var ids []int64
		if *retry >= 0 {
			ids = []int64{*retry}
		}
		return retryRuns(prog, dir, url, path, ids, stdout, stderr)
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
		step := runs[0].Steps[stepNo-runs[0].FirstStep]
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
		if step.Spent != nil && step.Spent.Options != "" {
			fmt.Fprintln(stdout, "how it was sent: "+step.Spent.Options)
		}
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
	interp.FormatRunsWith(stdout, runs, interp.FormatOptions{Steps: id >= 0, Replies: *replies}, time.Now())
	return 0
}

// retryRuns starts failed runs again, as eye runs --retry and --retry-failed
// do (SPECS.md §15): the runs with the ids given, or every failed run a retry
// can cure. In a store no eye run holds it changes the runs' records, and the
// next eye run carries them on; while eye run holds the store, it puts the
// retry in the runner's inbox, and the runner starts them within a second.
func retryRuns(prog *ir.Program, dir, url, path string, ids []int64, stdout, stderr io.Writer) int {
	var st interp.Store
	var closeStore func() error
	var err error
	if url != "" {
		var pg *interp.Postgres
		if pg, err = interp.OpenPostgres(url); err == nil {
			st, closeStore = pg, pg.Close
		}
	} else {
		if _, serr := os.Stat(path); os.IsNotExist(serr) {
			fmt.Fprintf(stderr, "eye runs: %s has no journal yet; eye run %s makes one\n", dir, dir)
			return exitErrors
		}
		var j *interp.Journal
		if j, err = interp.OpenJournal(path); err == nil {
			st, closeStore = j, j.Close
		}
	}
	all := len(ids) == 0
	none := "no failed run is one a retry can cure: only Unavailable and DeadlinePassed are retried"
	if interp.StoreHeld(err) {
		// eye run holds the store: check the retry against what it has
		// written, and leave it in the runner's inbox.
		var read interp.Store
		if url != "" {
			read, err = interp.ReadPostgres(url)
		} else {
			read, err = interp.ReadJournal(path)
		}
		if err != nil {
			fmt.Fprintf(stderr, "eye runs: %v\n", err)
			return exitErrors
		}
		plan, err := interp.RetryPlan(prog, read, ids, all)
		if err != nil {
			fmt.Fprintf(stderr, "eye runs: %v\n", err)
			return exitErrors
		}
		if len(plan) == 0 {
			fmt.Fprintln(stdout, none)
			return 0
		}
		if err := interp.SendRetry(inboxPath(dir), interp.InboxRetry{Retry: ids, All: all}); err != nil {
			fmt.Fprintf(stderr, "eye runs: %v\n", err)
			return exitErrors
		}
		for _, r := range plan {
			fmt.Fprintf(stdout, "retrying run %d (%s): eye run, which holds the store, starts it within a second, replaying %s\n", r.ID, r.Job, stepsWords(r.Steps))
		}
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "eye runs: %v\n", err)
		return exitErrors
	}
	done, err := interp.Retry(prog, st, ids, all)
	if cerr := closeStore(); err == nil {
		err = cerr
	}
	if err != nil {
		fmt.Fprintf(stderr, "eye runs: %v\n", err)
		return exitErrors
	}
	if len(done) == 0 {
		fmt.Fprintln(stdout, none)
		return 0
	}
	for _, r := range done {
		fmt.Fprintf(stdout, "run %d (%s) is ready again: eye run %s carries it on, replaying %s\n", r.ID, r.Job, dir, stepsWords(r.Steps))
	}
	return 0
}

// stepsWords is a number of recorded steps in words.
func stepsWords(n int) string {
	if n == 1 {
		return "its 1 recorded step"
	}
	return fmt.Sprintf("its %d recorded steps", n)
}
