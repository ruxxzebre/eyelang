package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestRenderSpec(t *testing.T) {
	spec, err := os.ReadFile("../../SPECS.md")
	if err != nil {
		t.Fatal(err)
	}
	page, err := Render(spec, "https://example.com/repo/", true)
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	ids := map[string]bool{}
	for _, m := range regexp.MustCompile(`id="([^"]+)"`).FindAllStringSubmatch(s, -1) {
		ids[m[1]] = true
	}
	links := regexp.MustCompile(`href="#([^"]+)"`).FindAllStringSubmatch(s, -1)
	if len(links) < 20 {
		t.Errorf("only %d links within the page", len(links))
	}
	for _, l := range links {
		if !ids[l[1]] {
			t.Errorf("the link #%s goes nowhere", l[1])
		}
	}
	for _, want := range []string{
		`<nav class="toc" aria-label="Contents">`,
		`<span class="w">sleep</span><span class="w">!</span>`,
		`<div class="table-wrap"><table>`,
		`href="https://example.com/repo/ROADMAP.md"`,
		`<p class="lede">Eyelang 2 is`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("the page lacks %s", want)
		}
	}
	if strings.Contains(s, "<h1 id=") {
		t.Error("the spec's own title should give way to the page header")
	}
}
