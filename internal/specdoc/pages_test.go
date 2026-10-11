package specdoc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestPagesUpToDate checks that the docs site's reference and design pages
// match SPECS.md as it is now. Run `task generate` after changing the spec.
func TestPagesUpToDate(t *testing.T) {
	spec, err := os.ReadFile("../../SPECS.md")
	if err != nil {
		t.Fatal(err)
	}
	pages, sidebar := Pages(spec, "https://github.com/ruxxzebre/eyelang/blob/dev/")
	want := map[string]bool{}
	for _, p := range pages {
		path := filepath.Join("../../docs", p.Path)
		want[path] = true
		data, err := os.ReadFile(path)
		if err != nil || string(data) != p.Content {
			t.Errorf("docs/%s doesn't match SPECS.md; run task generate", p.Path)
		}
	}
	for _, dir := range []string{"reference/language", "design"} {
		files, _ := filepath.Glob(filepath.Join("../../docs", dir, "*.md"))
		for _, f := range files {
			if !want[f] {
				t.Errorf("%s has no section in SPECS.md; run task generate", f)
			}
		}
	}
	data, _ := json.MarshalIndent(sidebar, "", "  ")
	got, _ := os.ReadFile("../../docs/.vitepress/spec-sidebar.json")
	if string(got) != string(data)+"\n" {
		t.Error("docs/.vitepress/spec-sidebar.json doesn't match SPECS.md; run task generate")
	}
}

// TestPagesLinks checks that every link between pages reaches a page, and that
// sections link to each other and to repository files.
func TestPagesLinks(t *testing.T) {
	spec, err := os.ReadFile("../../SPECS.md")
	if err != nil {
		t.Fatal(err)
	}
	pages, _ := Pages(spec, "https://repo/")
	if len(pages) < 20 {
		t.Fatalf("got %d pages, want a page for each section", len(pages))
	}
	exists := map[string]bool{}
	for _, p := range pages {
		exists[p.Link] = true
	}
	site := regexp.MustCompile(`\]\((/[^)#]*)(#[^)]*)?\)`)
	var internal, repo int
	for _, p := range pages {
		for _, m := range site.FindAllStringSubmatch(p.Content, -1) {
			internal++
			if !exists[m[1]] {
				t.Errorf("%s links to %s, which isn't a page", p.Path, m[1])
			}
			if m[2] != "" {
				t.Errorf("%s links to %s%s: sections have no headings to link to", p.Path, m[1], m[2])
			}
		}
		repo += strings.Count(p.Content, "](https://repo/")
		if strings.Contains(p.Content, "](#") {
			t.Errorf("%s has a link within the page that SPECS.md meant for itself", p.Path)
		}
	}
	if internal < 10 || repo < 2 {
		t.Errorf("found %d links between pages and %d to the repository; the rewriting has stopped working", internal, repo)
	}
}

func TestPagesSplit(t *testing.T) {
	spec := []byte("# Title\n\nStatus: draft.\n\nThe language.\n\n## Contents\n\n1. [A](#1-goals)\n\n" +
		"## 1. Goals\n\nSee [the tour](#3-a-tour) and section 21.\n\n" +
		"## 3. A tour\n\n```eye\n## A doc comment, not a heading.\nfn main() {}\n```\n\nSee section 1, `section 1` and [section 1](#1-goals).\n\n" +
		"## 21. What happens to Eyelang 1\n\nGone.\n")
	pages, sidebar := Pages(spec, "https://repo/")
	got := map[string]string{}
	for _, p := range pages {
		got[p.Path] = p.Content
	}
	goals := got["design/goals.md"]
	if !strings.Contains(goals, "See [the tour](/reference/language/a-tour) and section 21.") {
		t.Errorf("goals page:\n%s", goals)
	}
	tour := got["reference/language/a-tour.md"]
	for _, want := range []string{
		"## A doc comment, not a heading.\nfn main() {}",
		"See [section 1](/design/goals), `section 1` and [section 1](/design/goals).",
	} {
		if !strings.Contains(tour, want) {
			t.Errorf("tour page lacks %q:\n%s", want, tour)
		}
	}
	if _, ok := got["reference/language/what-happens-to-eyelang-1.md"]; ok {
		t.Error("section 21 has a page")
	}
	if !strings.Contains(got["reference/language/index.md"], "The language.") || strings.Contains(got["reference/language/index.md"], "Status") {
		t.Errorf("reference index:\n%s", got["reference/language/index.md"])
	}
	if len(sidebar) != 2 || len(sidebar[0].Items) != 1 || sidebar[0].Items[0].Link != "/reference/language/a-tour" {
		t.Errorf("sidebar = %+v", sidebar)
	}
}
