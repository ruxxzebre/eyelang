package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"eyelang/check"
	"eyelang/interp"
)

// journalPath is where a program's journal lives unless --journal says
// otherwise: .eye/journal in its directory.
func journalPath(dir string) string { return filepath.Join(dir, ".eye", "journal") }

// storeURL is the database --store names, or else $EYE_STORE.
func storeURL(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv("EYE_STORE")
}

// runRun implements `eye run [--store url | --journal file | --memory] [program-directory] [-- arguments…]`.
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
	store := fs.String("store", "", "keep entities, runs and events in this Postgres database, postgres://… (default $EYE_STORE)")
	memLimit := fs.String("memory-limit", "", "the memory runs may hold, which @memory and reserve! wait for, such as 512MiB (default the cgroup's limit, such as a container's, or the host's memory)")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: eye run [--store url | --journal file | --memory] [program-directory] [-- arguments…]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Runs main, then keeps the program's jobs going: sleeping runs wake on time,")
		fmt.Fprintln(stderr, "schedules fire, and a restart carries on where the last run of eye stopped.")
		fmt.Fprintln(stderr, "It stops when nothing is left to wait for, or on Ctrl-C or SIGTERM.")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "State is kept in the program's journal, or with --store in a Postgres database,")
		fmt.Fprintln(stderr, "in the first schema of its search_path: postgres://…/db?search_path=lab gives")
		fmt.Fprintln(stderr, "a program its own schema. One eye run at a time writes a journal or a schema.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	var limit int64
	if *memLimit != "" {
		n, ok := parseSize(*memLimit)
		if !ok {
			fmt.Fprintf(stderr, "eye run: --memory-limit takes a size such as 512MiB or 2GiB; got %q\n", *memLimit)
			return exitUsage
		}
		limit = n
	}
	url := *store
	if *journal == "" && !*memory {
		url = storeURL(url)
	}
	if url != "" && (*journal != "" || *memory) || *journal != "" && *memory {
		fmt.Fprintln(stderr, "eye run keeps its state in one place: give one of --store, --journal and --memory")
		return exitUsage
	}
	if url != "" && !interp.IsPostgres(url) {
		fmt.Fprintf(stderr, "eye run: --store takes a Postgres address, postgres://user:password@host/database; got %q\n", url)
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
	opts := interp.RunOptions{Args: append([]string{dir}, progArgs...), Stdout: stdout, Stderr: stderr, Inbox: inboxPath(dir), MemoryLimit: limit}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	switch {
	case url != "":
		// A database that can't be reached yet is waited for, as one that
		// goes away later is.
		pg, err := interp.WaitForPostgres(ctx, url, stderr)
		if ctx.Err() != nil {
			if pg != nil {
				pg.Close()
			}
			return 0 // stopped while waiting, as asked
		}
		if err != nil {
			fmt.Fprintf(stderr, "eye run: %v\n", err)
			return exitErrors
		}
		defer pg.Close()
		opts.Store = pg
	case !*memory:
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
	return interp.Run(ctx, prog, opts)
}

// parseSize reads a size as a flag gives it: 512MiB, 2 GiB, 64KiB, or bytes.
func parseSize(s string) (int64, bool) {
	s = strings.ReplaceAll(strings.TrimSpace(s), " ", "")
	units := []struct {
		suffix string
		size   int64
	}{{"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10}, {"B", 1}}
	for _, u := range units {
		if num, ok := strings.CutSuffix(s, u.suffix); ok {
			n, err := strconv.ParseInt(num, 10, 64)
			return n * u.size, err == nil && n > 0
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil && n > 0
}
