package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"

	"eyelang/repl"
)

func runREPL(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("repl", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: eye repl [program-directory]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Starts an interactive session: expressions print their value and type,")
		fmt.Fprintln(stderr, "statements run, and declarations join the session. Given a program, the")
		fmt.Fprintln(stderr, "session can use its modules. :help lists the commands.")
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	dir := ""
	if fs.NArg() > 1 {
		fs.Usage()
		return exitUsage
	}
	if fs.NArg() == 1 {
		dir = fs.Arg(0)
	}
	r, err := repl.New(dir, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "eye repl: %v\n", err)
		return exitErrors
	}
	defer r.Close()
	read := plainReader(stdin, stdout)
	if f, ok := stdout.(*os.File); ok && term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(f.Fd())) {
		home, _ := os.UserHomeDir()
		ed := repl.NewLineEditor(os.Stdin, f, filepath.Join(home, ".eye_history"))
		_ = ed.LoadHistory()
		defer func() { _ = ed.SaveHistory() }()
		read = func(prompt string) (string, error) {
			line, err := ed.Prompt(prompt)
			if err == nil && strings.TrimSpace(line) != "" {
				ed.AppendHistory(line)
			}
			return line, err
		}
		fmt.Fprintln(stdout, "Eyelang "+version+". :help lists the commands, Ctrl-D leaves.")
	}
	return session(r, read, stdout)
}

// session reads inputs until :quit or the end of the input; an input whose
// first line is unfinished goes on over further lines.
func session(r *repl.REPL, read func(prompt string) (string, error), stdout io.Writer) int {
	for {
		input, err := read(repl.Prompt)
		for err == nil && repl.Unfinished(input) {
			var more string
			more, err = read(repl.Continue)
			input += "\n" + more
		}
		if errors.Is(err, repl.ErrInterrupted) {
			continue
		}
		if err != nil {
			fmt.Fprintln(stdout)
			return 0
		}
		if r.Eval(input) {
			return 0
		}
	}
}

// plainReader reads lines without a terminal, as from a pipe: it prints no
// prompts.
func plainReader(in io.Reader, _ io.Writer) func(string) (string, error) {
	sc := bufio.NewScanner(in)
	return func(string) (string, error) {
		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				return "", err
			}
			return "", io.EOF
		}
		return sc.Text(), nil
	}
}
