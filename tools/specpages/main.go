// Command specpages writes the docs site's language reference and design
// pages from SPECS.md: one page per section, under docs/reference/language and
// docs/design, and their sidebar, docs/.vitepress/spec-sidebar.json. Run it
// from the repository root with `task generate`.
package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"eyelang/internal/specdoc"
)

const repo = "https://github.com/ruxxzebre/eyelang/blob/main/"

func main() {
	spec, err := os.ReadFile("SPECS.md")
	if err != nil {
		log.Fatal(err)
	}
	pages, sidebar := specdoc.Pages(spec, repo)
	// Both directories hold only generated pages.
	for _, dir := range []string{"docs/reference/language", "docs/design"} {
		if err := os.RemoveAll(dir); err != nil {
			log.Fatal(err)
		}
	}
	for _, p := range pages {
		path := filepath.Join("docs", p.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(p.Content), 0o644); err != nil {
			log.Fatal(err)
		}
	}
	data, err := json.MarshalIndent(sidebar, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("docs/.vitepress/spec-sidebar.json", append(data, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
}
