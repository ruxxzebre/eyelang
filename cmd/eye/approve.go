package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"eyelang/check"
	"eyelang/interp"
)

// inboxPath is the directory a running `eye run` reads answers to approvals
// from: .eye/inbox in the program's directory.
func inboxPath(dir string) string { return filepath.Join(dir, ".eye", "inbox") }

// runApprove implements `eye approve [--id ID] [--by name] [program-directory] [text]`.
func runApprove(args []string, stdout, stderr io.Writer) int {
	return answer("approve", true, args, stdout, stderr)
}

// runDecline implements `eye decline [--id ID] [--by name] [program-directory] [text]`.
func runDecline(args []string, stdout, stderr io.Writer) int {
	return answer("decline", false, args, stdout, stderr)
}

// answer answers the one approval a program's run waits for whose text
// contains text, or whose id is --id, by putting the answer in the inbox of
// the program's `eye run`.
func answer(name string, approved bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "the id of the approval to answer, as eye runs and an ambiguous eye "+name+" show it")
	by := fs.String("by", "", "who answers; only the person the approval asks may (default that person)")
	journal := fs.String("journal", "", "the journal to look for waiting approvals in (default program-directory/.eye/journal)")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "usage: eye %s [--id ID] [--by name] [--journal file] [program-directory] [text]\n", name)
		fmt.Fprintln(stderr)
		fmt.Fprintf(stderr, "Answers, with %s, the one approval! a run of the program waits for whose\n", name)
		fmt.Fprintln(stderr, "text contains text, or whose id is ID. When several contain text, it answers")
		fmt.Fprintln(stderr, "none and lists them with their ids. The program's eye run picks the answer up")
		fmt.Fprintln(stderr, "within a second, and publishes it as approvals.answered. eye runs shows what")
		fmt.Fprintln(stderr, "each run waits for.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	dir, text := ".", ""
	switch {
	case *id != "" && fs.NArg() == 0:
	case *id != "" && fs.NArg() == 1:
		dir = fs.Arg(0)
	case *id == "" && fs.NArg() == 1:
		text = fs.Arg(0)
	case *id == "" && fs.NArg() == 2:
		dir, text = fs.Arg(0), fs.Arg(1)
	default:
		fs.Usage()
		return exitUsage
	}
	if *id == "" && strings.TrimSpace(text) == "" {
		fmt.Fprintf(stderr, "eye %s: give text from what the run asks, as eye runs shows it, or --id\n", name)
		return exitUsage
	}
	sent := interp.InboxAnswer{ID: *id, What: text, Approved: approved, By: *by}
	// With a journal to read, the answer names its approval by id, and one
	// that is ambiguous, names nothing waiting or comes from someone else is
	// refused here. A run keeping its state elsewhere is trusted to find the
	// approval itself.
	path := *journal
	if path == "" {
		path = journalPath(dir)
	}
	done := ""
	if store, err := interp.ReadJournal(path); err == nil {
		prog, diags := check.Program(dir)
		if len(diags) > 0 {
			for _, d := range diags {
				fmt.Fprintln(stderr, d)
			}
			return exitErrors
		}
		var waiting []interp.RunInfo
		for _, r := range interp.Runs(prog, store) {
			switch {
			case r.ApprovalID == "":
			case *id != "" && r.ApprovalID == *id, *id == "" && strings.Contains(r.Approval, text):
				waiting = append(waiting, r)
			}
		}
		switch {
		case len(waiting) == 0 && *id != "":
			fmt.Fprintf(stderr, "eye %s: no run waits for the approval %s; eye runs %s shows what runs wait for\n", name, *id, dir)
			return exitErrors
		case len(waiting) == 0:
			fmt.Fprintf(stderr, "eye %s: no run waits for an approval whose text contains %q; eye runs %s shows what runs wait for\n", name, text, dir)
			return exitErrors
		case len(waiting) > 1:
			fmt.Fprintf(stderr, "eye %s: %d approvals contain %q, and an answer goes to one; answer it with eye %s --id ID:\n", name, len(waiting), text, name)
			for _, r := range waiting {
				fmt.Fprintf(stderr, "  %s  run %d (%s): %s\n", r.ApprovalID, r.ID, r.Job, r.Approval)
			}
			return exitErrors
		}
		r := waiting[0]
		if r.ApprovalFrom != "" {
			if *by != "" && *by != r.ApprovalFrom {
				fmt.Fprintf(stderr, "eye %s: the approval %s asks %s, and only they may answer it, not %s\n", name, r.ApprovalID, r.ApprovalFrom, *by)
				return exitErrors
			}
			sent.By = r.ApprovalFrom
		}
		sent.ID, sent.What = r.ApprovalID, ""
		done = fmt.Sprintf("run %d (%s): %s", r.ID, r.Job, r.Approval)
	}
	if err := interp.SendAnswer(inboxPath(dir), sent); err != nil {
		fmt.Fprintf(stderr, "eye %s: %v\n", name, err)
		return exitErrors
	}
	if done != "" {
		fmt.Fprintf(stdout, "%sd %s\n", name, done)
	}
	return 0
}
