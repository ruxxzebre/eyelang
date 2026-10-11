package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"eyelang/check"
	"eyelang/interp"
)

// inboxPath is the directory a running `eye run` reads answers to approvals
// from: .eye/inbox in the program's directory.
func inboxPath(dir string) string { return filepath.Join(dir, ".eye", "inbox") }

// runApprove implements `eye approve [--by name] [program-directory] text`.
func runApprove(args []string, stdout, stderr io.Writer) int {
	return answer("approve", true, args, stdout, stderr)
}

// runDecline implements `eye decline [--by name] [program-directory] text`.
func runDecline(args []string, stdout, stderr io.Writer) int {
	return answer("decline", false, args, stdout, stderr)
}

// answer answers the approvals a program's runs wait for whose text contains
// text, by putting the answer in the inbox of the program's `eye run`.
func answer(name string, approved bool, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	by := fs.String("by", "", "who answers, as the approval's Approval records it (default $USER)")
	journal := fs.String("journal", "", "the journal to look for waiting approvals in (default program-directory/.eye/journal)")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "usage: eye %s [--by name] [--journal file] [program-directory] text\n", name)
		fmt.Fprintln(stderr)
		fmt.Fprintf(stderr, "Answers, with %s, every approval! a run of the program waits for whose text\n", name)
		fmt.Fprintln(stderr, "contains text. The program's eye run picks the answer up within a second, and")
		fmt.Fprintln(stderr, "publishes it as approvals.answered. eye runs shows what each run waits for.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	dir, text := ".", ""
	switch fs.NArg() {
	case 1:
		text = fs.Arg(0)
	case 2:
		dir, text = fs.Arg(0), fs.Arg(1)
	default:
		fs.Usage()
		return exitUsage
	}
	if strings.TrimSpace(text) == "" {
		fmt.Fprintf(stderr, "eye %s: give text from what the run asks, as eye runs shows it\n", name)
		return exitUsage
	}
	who := *by
	if who == "" {
		who = os.Getenv("USER")
	}
	// With a journal to read, an answer nothing waits for is refused, since
	// it would only sit there; a run keeping its state elsewhere is trusted.
	path := *journal
	if path == "" {
		path = journalPath(dir)
	}
	var waiting []string
	if store, err := interp.ReadJournal(path); err == nil {
		prog, diags := check.Program(dir)
		if len(diags) > 0 {
			for _, d := range diags {
				fmt.Fprintln(stderr, d)
			}
			return exitErrors
		}
		for _, r := range interp.Runs(prog, store) {
			if r.Approval != "" && strings.Contains(r.Approval, text) {
				waiting = append(waiting, fmt.Sprintf("run %d (%s): %s", r.ID, r.Job, r.Approval))
			}
		}
		if len(waiting) == 0 {
			fmt.Fprintf(stderr, "eye %s: no run waits for an approval whose text contains %q; eye runs %s shows what runs wait for\n", name, text, dir)
			return exitErrors
		}
	}
	if err := interp.SendAnswer(inboxPath(dir), interp.InboxAnswer{What: text, Approved: approved, By: who}); err != nil {
		fmt.Fprintf(stderr, "eye %s: %v\n", name, err)
		return exitErrors
	}
	for _, w := range waiting {
		fmt.Fprintf(stdout, "%sd %s\n", name, w)
	}
	return 0
}
