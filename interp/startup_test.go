package interp

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

const startProgram = `
entity Seen(name: String) {
  state days: List(Date) = []
  pub fn add(d: Date) { days += [d] }
}

@on(daily(07:40, in: "Europe/Dublin"))
job morning(day: Date) {
  Seen("x").add!(day)
}

@on(every(15 min))
job tick() {}

job write() {}
`

// The line eye run starts with names the program, the store, the memory and
// every job, with when each scheduled one fires next on its own clock.
func TestStartLine(t *testing.T) {
	prog := load(t, startProgram)
	in := New(prog, &strings.Builder{}, &strings.Builder{})
	in.rt.now = time.Date(2026, 10, 11, 12, 5, 0, 0, time.UTC)
	got := in.rt.startLine("/program", nil, memoryText(512<<20))
	want := "eye run: started /program; store: in memory only; memory: 512 MiB; " +
		"jobs: morning (next 2026-10-12 07:40 Europe/Dublin), tick (next 2026-10-11 12:15 UTC), write"
	if got != want {
		t.Errorf("start line\n%s\nwant\n%s", got, want)
	}
}

// eye run says it started before any job runs, on stderr with its other
// logs, and says where the journal is.
func TestRunSaysItStarted(t *testing.T) {
	prog := load(t, startProgram)
	path := filepath.Join(t.TempDir(), "journal")
	j, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	var stdout, stderr strings.Builder
	Run(ctx, prog, RunOptions{Args: []string{"prog"}, Stdout: &stdout, Stderr: &stderr, Store: j, MemoryLimit: 384 << 20})
	line := regexp.MustCompile(`^eye run: started prog; store: journal ` + regexp.QuoteMeta(path) +
		`; memory: 384 MiB; jobs: morning \(next \d{4}-\d\d-\d\d 07:40 Europe/Dublin\), tick \(next [-0-9]+ [0-9:]+ UTC\), write\n$`)
	if !line.MatchString(stderr.String()) || stdout.String() != "" {
		t.Errorf("stdout %q, stderr %q; want the start line on stderr", stdout.String(), stderr.String())
	}
}

// A program without jobs runs main and nothing else, so it starts quietly.
func TestRunWithoutJobsSaysNothing(t *testing.T) {
	prog := load(t, `pub fn main(args: List(String)) uses {stdio} { print("hi") }`)
	var stdout, stderr strings.Builder
	Run(context.Background(), prog, RunOptions{Stdout: &stdout, Stderr: &stderr})
	if stdout.String() != "hi\n" || stderr.String() != "" {
		t.Errorf("stdout %q, stderr %q", stdout.String(), stderr.String())
	}
}

// A database is named by host, port, database and schema, and never with
// its user or password.
func TestPostgresNameHasNoSecrets(t *testing.T) {
	cfg, err := pgx.ParseConfig("postgresql://app:s3cret@db.example:5432/app?search_path=lab&password=other")
	if err != nil {
		t.Fatal(err)
	}
	name := pgName(cfg)
	if name != "postgres db.example:5432/app" || strings.Contains(name, "s3cret") || strings.Contains(name, "app:") {
		t.Errorf("named %q", name)
	}
}

// A step recorded before tokens in and out were kept separately shows the
// tokens it kept.
func TestSpentOfAnOldStep(t *testing.T) {
	if got := spentInfo(step{Tokens: 33000, USD: 0.165}.spent()).String(); got != "33000 tokens, $0.1650" {
		t.Errorf("shown as %q", got)
	}
	if spentInfo(step{}.spent()) != nil {
		t.Error("a step that spent nothing shows a spend")
	}
}
