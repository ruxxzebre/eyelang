package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/term"
)

// started is when eye began, before main checks the checkout: the stats
// count the time to reach the formatter as startup.
var started = time.Now()

// fmtStats measures one `eye fmt` run over paths.
type fmtStats struct {
	check  bool
	paths  int
	phases []fmtPhase
	since  time.Time // start of the phase being timed

	files, lines, bytes        int
	unformatted, errors, wrote int
	programs, modules          int
	perFile                    []fileTime

	cache       string // "", "hit", "miss" or "off"
	cacheSaved  time.Time
	cacheInputs int
	cacheCold   time.Duration // how long the run that saved the result took
}

type fmtPhase struct {
	name   string
	took   time.Duration
	detail string
}

type fileTime struct {
	path  string
	lines int
	took  time.Duration
}

func newFmtStats(check bool, paths int) *fmtStats {
	now := time.Now()
	s := &fmtStats{check: check, paths: paths, since: now}
	s.phases = append(s.phases, fmtPhase{"startup", now.Sub(started), "eye starting and checking the checkout"})
	return s
}

// phase ends the phase being timed, naming it.
func (s *fmtStats) phase(name, detail string) {
	now := time.Now()
	s.phases = append(s.phases, fmtPhase{name, now.Sub(s.since), detail})
	s.since = now
}

// file records a source read and formatted, and the time it took.
func (s *fmtStats) file(path string, src []byte, took time.Duration) {
	lines := bytes.Count(src, []byte("\n"))
	if len(src) > 0 && src[len(src)-1] != '\n' {
		lines++
	}
	s.files++
	s.lines += lines
	s.bytes += len(src)
	s.perFile = append(s.perFile, fileTime{path, lines, took})
}

// saved is what a saved result keeps of this run.
func (s *fmtStats) saved() fmtSavedStats {
	return fmtSavedStats{
		Saved: time.Now(), Took: time.Since(started),
		Lines: s.lines, Bytes: s.bytes, Unformatted: s.unformatted, Errors: s.errors,
		Programs: s.programs, Modules: s.modules,
	}
}

// replay takes the counts of the run that saved result, whose files and
// inputs were just found unchanged.
func (s *fmtStats) replay(result fmtResult, files int) {
	r := result.Stats
	s.cache, s.cacheSaved, s.cacheCold, s.cacheInputs = "hit", r.Saved, r.Took, len(result.Inputs)
	s.files, s.lines, s.bytes = files, r.Lines, r.Bytes
	s.unformatted, s.errors, s.programs, s.modules = r.Unformatted, r.Errors, r.Programs, r.Modules
	s.phase("cache lookup", fmt.Sprintf("reread %s inputs: all unchanged, so the saved output is replayed", count(len(result.Inputs))))
}

func (s *fmtStats) total() time.Duration {
	var d time.Duration
	for _, p := range s.phases {
		d += p.took
	}
	return d
}

func (s *fmtStats) took(name string) time.Duration {
	for _, p := range s.phases {
		if p.name == name {
			return p.took
		}
	}
	return 0
}

// summary is one line: how much was formatted, how long it took, and what
// came of it.
func (s *fmtStats) summary() string {
	verb := "formatted"
	if s.check {
		verb = "checked"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "eye fmt: %s %s, %s lines in %s", verb, plural(s.files, "file"), count(s.lines), duration(s.total()))
	if s.cache == "hit" {
		b.WriteString(" (saved result)")
	}
	var results []string
	switch {
	case s.check && s.unformatted > 0:
		results = append(results, fmt.Sprintf("%d not formatted", s.unformatted))
	case !s.check && s.wrote > 0:
		results = append(results, fmt.Sprintf("%d rewritten", s.wrote))
	}
	if s.errors > 0 {
		results = append(results, plural(s.errors, "file")+" with errors")
	}
	if len(results) == 0 {
		if s.check {
			results = append(results, "all formatted")
		} else {
			results = append(results, "no changes")
		}
	}
	fmt.Fprintf(&b, "; %s", strings.Join(results, ", "))
	return b.String()
}

// report writes the summary, then each phase's time and the slowest files.
func (s *fmtStats) report(w io.Writer) {
	fmt.Fprintln(w, s.summary())
	fmt.Fprintf(w, "  %-13s %s, %s, %s lines, %s\n", "input", plural(s.paths, "path"), plural(s.files, "file"), count(s.lines), size(s.bytes))
	switch s.cache {
	case "hit":
		fmt.Fprintf(w, "  %-13s hit: replayed the result saved %s, after rereading its %s inputs; that run took %s\n",
			"cache", ago(time.Since(s.cacheSaved)), count(s.cacheInputs), duration(s.cacheCold))
	case "miss":
		fmt.Fprintf(w, "  %-13s miss: no saved result matches, so this run checks everything and saves its %s inputs\n", "cache", count(s.cacheInputs))
	case "off":
		fmt.Fprintf(w, "  %-13s off: --cold ignores saved results\n", "cache")
	}
	total := s.total()
	for _, p := range s.phases {
		pct := 0.0
		if total > 0 {
			pct = 100 * float64(p.took) / float64(total)
		}
		fmt.Fprintf(w, "  %-13s %9s %4.0f%%  %s\n", p.name, duration(p.took), pct, p.detail)
	}
	fmt.Fprintf(w, "  %-13s %9s\n", "total", duration(total))
	if s.cache == "hit" || s.files == 0 {
		return
	}
	if work := s.took("check") + s.took("format"); work > 0 {
		fmt.Fprintf(w, "  %-13s %s per file, %s per 1,000 lines, %s lines/s (check and format)\n", "rate",
			duration(work/time.Duration(s.files)), duration(per(work, s.lines, 1000)), count(int(float64(s.lines)/work.Seconds())))
	}
	if s.files < 2 {
		return
	}
	sort.SliceStable(s.perFile, func(i, j int) bool { return s.perFile[i].took > s.perFile[j].took })
	n := min(5, len(s.perFile))
	fmt.Fprintf(w, "  slowest %d to format:\n", n)
	for _, f := range s.perFile[:n] {
		fmt.Fprintf(w, "    %9s  %s (%s)\n", duration(f.took), f.path, plural(f.lines, "line"))
	}
}

// fmtSummaryWanted reports whether to print the summary line uninvited:
// only to a person at a terminal, never to a script or a test.
func fmtSummaryWanted(stderr io.Writer) bool {
	f, ok := stderr.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

func per(d time.Duration, n, unit int) time.Duration {
	if n == 0 {
		return 0
	}
	return time.Duration(float64(d) * float64(unit) / float64(n))
}

func duration(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return fmt.Sprintf("%dµs", d.Microseconds())
	case d < 10*time.Millisecond:
		return fmt.Sprintf("%.2fms", float64(d)/float64(time.Millisecond))
	case d < time.Second:
		return fmt.Sprintf("%.1fms", float64(d)/float64(time.Millisecond))
	case d < time.Minute:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	return d.Round(time.Second).String()
}

func ago(d time.Duration) string {
	if d < time.Second {
		return "just now"
	}
	return duration(d.Round(time.Second)) + " ago"
}

func size(n int) string {
	switch {
	case n < 1<<10:
		return fmt.Sprintf("%d B", n)
	case n < 1<<20:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
}

// count writes n with thousands separated: 18,430.
func count(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0 && s[i-1] != '-'; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
