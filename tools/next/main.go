// Command next answers "what's next?" from what is true now, rather than
// from notes that go stale: the last release and what landed since, the
// priorities ROADMAP.md keeps by hand, a checklist of ROADMAP.md's task
// tables, the open issues, the gaps that docs/reference/status.md lists, and
// the work in flight in every worktree. Run it from the repository root with
// `task next`.
//
// With --check it prints nothing and fails if ROADMAP.md is out of date:
// "What's next" checked before the last release or naming an issue that is
// closed, or a task row whose "(done)" disagrees with its issue. The issue
// checks need gh; without it they are skipped.
//
// Sessions working at once say which paths they change with claims, kept in
// the repository's git config, which every worktree shares and nothing
// commits, so they are never stale in a file:
//
//	task claim -- eyelang-17 internal/lsp/completion.go
//	task claim -- --release eyelang-17
//
// `task next` lists them, with how old each is.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func main() {
	check := flag.Bool("check", false, "fail if ROADMAP.md's What's next is out of date, and print nothing else")
	all := flag.Bool("all", false, "list every gap in status.md and every finished phase's tasks, not only how many")
	flag.Parse()
	switch flag.Arg(0) {
	case "claim":
		if flag.NArg() < 3 {
			fail("usage: next claim WHO PATH…")
		}
		who := flag.Arg(1)
		value := time.Now().UTC().Format(time.RFC3339) + " " + strings.Join(flag.Args()[2:], " ")
		if out, err := exec.Command("git", "config", claimKey(who), value).CombinedOutput(); err != nil {
			fail("%v: %s", err, out)
		}
		return
	case "release":
		if flag.NArg() != 2 {
			fail("usage: next release WHO")
		}
		_ = exec.Command("git", "config", "--unset", claimKey(flag.Arg(1))).Run()
		return
	case "":
	default:
		fail("unknown command %q: next, next claim WHO PATH…, or next release WHO", flag.Arg(0))
	}
	roadmap, err := os.ReadFile("ROADMAP.md")
	if err != nil {
		fail("run it from the repository root: %v", err)
	}
	next := section(string(roadmap), "## What's next")
	ps := phases(string(roadmap))
	if *check {
		problems := stale(next)
		for i, p := range problems {
			problems[i] = "What's next: " + p
		}
		for _, p := range untracked(ps) {
			problems = append(problems, "task tables: "+p)
		}
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "ROADMAP.md,", p)
		}
		if len(problems) > 0 {
			os.Exit(1)
		}
		return
	}

	heading("Released")
	if tag := git("describe", "--tags", "--abbrev=0"); tag != "" {
		fmt.Printf("%s, and %s commits on %s since\n", tag, git("rev-list", "--count", tag+"..HEAD"), git("branch", "--show-current"))
	} else {
		fmt.Println("no release tag in this clone; a shallow clone has none: git fetch --unshallow --tags")
	}
	for _, p := range stale(next) {
		fmt.Println("warning: What's next", p)
	}

	heading("What's next (ROADMAP.md, kept by hand)")
	fmt.Println(strings.TrimSpace(next))

	heading("Roadmap (ROADMAP.md's task tables)")
	checklist(ps, *all)
	for _, p := range untracked(ps) {
		fmt.Println("warning:", p)
	}

	heading("Open issues")
	if out, err := exec.Command("gh", "issue", "list", "--state", "open", "--limit", "200").Output(); err == nil {
		fmt.Print(string(out))
	} else {
		fmt.Println("gh can't list them here; see the repository's issues")
	}

	heading("Gaps (docs/reference/status.md)")
	gaps(*all)

	heading("Work in flight")
	claims()
	inFlight()
}

func claimKey(who string) string { return "eye-claim." + who + ".paths" }

// claims prints who said they change which paths, and how long ago.
func claims() {
	out := git("config", "--get-regexp", `^eye-claim\..*\.paths$`)
	if out == "" {
		fmt.Println("no claims: a session says what it changes with `task claim -- WHO PATH…`")
		return
	}
	fmt.Println("claims:")
	for _, line := range strings.Split(out, "\n") {
		key, value, _ := strings.Cut(line, " ")
		who := strings.TrimSuffix(strings.TrimPrefix(key, "eye-claim."), ".paths")
		at, paths, _ := strings.Cut(value, " ")
		age := ""
		if t, err := time.Parse(time.RFC3339, at); err == nil {
			age = time.Since(t).Round(time.Minute).String() + " ago"
		}
		fmt.Printf("  %-14s %s  (%s)\n", who, paths, age)
	}
}

// section returns a ## section of a Markdown file, without its heading.
func section(doc, title string) string {
	i := strings.Index(doc, title+"\n")
	if i < 0 {
		return ""
	}
	rest := doc[i+len(title)+1:]
	if j := strings.Index(rest, "\n## "); j >= 0 {
		rest = rest[:j]
	}
	return rest
}

var checkedLine = regexp.MustCompile(`Checked: (\d{4}-\d{2}-\d{2})`)
var issueRef = regexp.MustCompile(`#(\d+)`)

// stale lists why the What's next section is out of date.
func stale(next string) []string {
	var out []string
	if next == "" {
		return []string{"is missing"}
	}
	m := checkedLine.FindStringSubmatch(next)
	if m == nil {
		return []string{"has no \"Checked: YYYY-MM-DD\" line"}
	}
	checked, _ := time.Parse("2006-01-02", m[1])
	if tag := git("describe", "--tags", "--abbrev=0"); tag != "" {
		if t, err := time.Parse("2006-01-02", git("log", "-1", "--format=%cs", tag)); err == nil && checked.Before(t) {
			out = append(out, fmt.Sprintf("was checked on %s, before %s was released on %s: check it again", m[1], tag, t.Format("2006-01-02")))
		}
	}
	seen := map[string]bool{}
	for _, ref := range issueRef.FindAllStringSubmatch(next, -1) {
		n := ref[1]
		if seen[n] {
			continue
		}
		seen[n] = true
		state, err := issueState(n)
		if err != nil {
			continue // no gh, or no network: nothing to say
		}
		if state == "CLOSED" {
			out = append(out, fmt.Sprintf("names #%s, which is closed", n))
		}
	}
	return out
}

// issueState is issue n's state, OPEN or CLOSED. It asks gh's GraphQL API
// first and falls back to the REST API, which is all some environments (a
// cloud session's proxy) allow, so the check isn't skipped there.
func issueState(n string) (string, error) {
	if out, err := exec.Command("gh", "issue", "view", n, "--json", "state", "-q", ".state").Output(); err == nil {
		return strings.ToUpper(strings.TrimSpace(string(out))), nil
	}
	out, err := exec.Command("gh", "api", "repos/{owner}/{repo}/issues/"+n, "-q", ".state").Output()
	if err != nil {
		return "", err
	}
	return strings.ToUpper(strings.TrimSpace(string(out))), nil
}

// A task is a row of one of ROADMAP.md's task tables, such as
// "| 6.2 `@port` and `@allow` enforced (done) | [#54](…) |".
type task struct {
	id, title, issue string
	done             bool
}

// A phase is a ### section under "## Phases" and its task rows.
type phase struct {
	name  string
	tasks []task
}

var (
	taskRow   = regexp.MustCompile(`^\| (\S+) (.+?) \| \[#(\d+)\]`)
	issueLink = regexp.MustCompile(`\[(#\d+)\]\([^)]*\)`)
)

// phases reads the task tables of ROADMAP.md's "## Phases". A row says
// "(done)" when its task is done; a row that says "moved to" points at the
// task that replaced it, and is left out.
func phases(roadmap string) []*phase {
	var out []*phase
	var cur *phase
	for _, line := range strings.Split(section(roadmap, "## Phases"), "\n") {
		if name, ok := strings.CutPrefix(line, "### "); ok {
			cur = &phase{name: issueLink.ReplaceAllString(name, "$1")}
			out = append(out, cur)
			continue
		}
		m := taskRow.FindStringSubmatch(line)
		if m == nil || cur == nil || strings.Contains(m[2], "moved to") {
			continue
		}
		title := strings.TrimSuffix(m[2], " (done)")
		cur.tasks = append(cur.tasks, task{id: m[1], title: title, issue: m[3], done: strings.Contains(m[2], "(done")})
	}
	return out
}

// checklist prints each phase's tasks with a box ticked for each one done.
// A finished phase takes one line unless all is set.
func checklist(ps []*phase, all bool) {
	for _, p := range ps {
		done := 0
		for _, t := range p.tasks {
			if t.done {
				done++
			}
		}
		if len(p.tasks) == 0 {
			continue
		}
		if done == len(p.tasks) && !all {
			fmt.Printf("done  %s: %d of %d\n", p.name, done, len(p.tasks))
			continue
		}
		fmt.Printf("%s: %d of %d\n", p.name, done, len(p.tasks))
		for _, t := range p.tasks {
			box := "[ ]"
			if t.done {
				box = "[x]"
			}
			fmt.Printf("  %s %s %s #%s\n", box, t.id, t.title, t.issue)
		}
	}
	if !all {
		fmt.Println("(task next -- --all lists finished phases' tasks too)")
	}
}

// untracked lists the task rows whose "(done)" disagrees with their issue's
// state on GitHub. Without gh it says nothing.
func untracked(ps []*phase) []string {
	out, err := exec.Command("gh", "issue", "list", "--state", "all", "--limit", "1000",
		"--json", "number,state", "-q", `.[] | "\(.number) \(.state)"`).Output()
	if err != nil {
		return nil
	}
	state := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if n, s, ok := strings.Cut(line, " "); ok {
			state[n] = s
		}
	}
	var problems []string
	for _, p := range ps {
		for _, t := range p.tasks {
			switch s := state[t.issue]; {
			case s == "CLOSED" && !t.done:
				problems = append(problems, fmt.Sprintf("%s's issue #%s is closed, but its row doesn't say (done)", t.id, t.issue))
			case s == "OPEN" && t.done:
				problems = append(problems, fmt.Sprintf("%s says (done), but its issue #%s is open", t.id, t.issue))
			}
		}
	}
	return problems
}

// gaps prints the features status.md marks partial or not built, by section.
func gaps(all bool) {
	f, err := os.Open("docs/reference/status.md")
	if err != nil {
		fmt.Println("no docs/reference/status.md")
		return
	}
	defer f.Close()
	type sec struct {
		name string
		rows []string
	}
	var secs []*sec
	cur := &sec{name: "(before any section)"}
	total, open := 0, 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "## ") {
			name := strings.TrimPrefix(line, "## ")
			if i := strings.Index(name, "]("); strings.HasPrefix(name, "[") && i > 0 {
				name = name[1:i]
			}
			cur = &sec{name: name}
			secs = append(secs, cur)
			continue
		}
		if !strings.HasPrefix(line, "| ") || strings.HasPrefix(line, "| Feature") || strings.HasPrefix(line, "|---") {
			continue
		}
		total++
		if strings.Contains(line, "partial") || strings.Contains(line, "| — ") {
			open++
			feature := strings.TrimSpace(strings.Split(line, "|")[1])
			cur.rows = append(cur.rows, feature)
		}
	}
	if err := sc.Err(); err != nil {
		fmt.Println("can't read docs/reference/status.md:", err)
		return
	}
	fmt.Printf("%d of %d features are partial or not built\n", open, total)
	for _, s := range secs {
		if len(s.rows) == 0 {
			continue
		}
		if !all {
			fmt.Printf("  %-40s %d\n", s.name, len(s.rows))
			continue
		}
		fmt.Printf("  %s\n", s.name)
		for _, r := range s.rows {
			fmt.Printf("    - %s\n", r)
		}
	}
	if !all {
		fmt.Println("(task next -- --all lists them)")
	}
}

// inFlight prints every worktree: its branch, what the branch holds that
// dev doesn't, uncommitted files, its last commit, and the paths it says it
// changes, from `git config branch.<name>.description`.
func inFlight() {
	type wt struct{ path, branch string }
	var wts []wt
	var cur wt
	for _, line := range strings.Split(git("worktree", "list", "--porcelain"), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur = wt{path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "branch "):
			cur.branch = strings.TrimPrefix(line, "branch refs/heads/")
		case line == "":
			if cur.path != "" {
				wts = append(wts, cur)
			}
			cur = wt{}
		}
	}
	if cur.path != "" {
		wts = append(wts, cur)
	}
	for _, w := range wts {
		dirty := 0
		for _, l := range strings.Split(gitIn(w.path, "status", "--porcelain"), "\n") {
			if l != "" && !strings.HasSuffix(l, ".claude/") {
				dirty++
			}
		}
		last := gitIn(w.path, "log", "-1", "--format=%cr: %s")
		state := ""
		if w.branch != "" && w.branch != "dev" {
			// Commits whose change isn't on dev, even as a cherry-pick.
			ahead := 0
			for _, l := range strings.Split(git("cherry", "dev", w.branch), "\n") {
				if strings.HasPrefix(l, "+") {
					ahead++
				}
			}
			if ahead == 0 {
				state = "merged into dev"
			} else {
				state = strconv.Itoa(ahead) + " commits not on dev"
			}
		}
		fmt.Printf("%s [%s]", w.path, w.branch)
		if state != "" {
			fmt.Printf(" %s", state)
		}
		if dirty > 0 {
			fmt.Printf(", %d uncommitted files", dirty)
		}
		fmt.Println()
		fmt.Printf("    last commit %s\n", last)
		if w.branch != "" {
			if d := git("config", "branch."+w.branch+".description"); d != "" {
				fmt.Printf("    claims: %s\n", d)
			}
		}
	}
}

func git(args ...string) string { return gitIn(".", args...) }

func gitIn(dir string, args ...string) string {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func heading(s string) { fmt.Printf("\n== %s\n", s) }

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "next: "+format+"\n", args...)
	os.Exit(1)
}
