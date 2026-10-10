// Package specdoc extracts the Eyelang examples from SPECS.md.
//
// Every `eye` code block in the spec is either a whole file or the statements
// of a function body. Statements are wrapped in `fn example() { … }` so that
// each example is a file both parsers can check.
package specdoc

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"eyelang/syntax"
)

// Example is one code block from SPECS.md.
type Example struct {
	Section int    // the section number
	Index   int    // 1 for the section's first example
	Name    string // a file name: spec-03-a-tour-1.eye
	Source  string // the example as a file, ending with a newline
	Wrapped bool   // the block was statements, wrapped in a function
}

var (
	heading = regexp.MustCompile(`^## (\d+)\. (.*)$`)
	nonWord = regexp.MustCompile(`[^a-z0-9]+`)
)

// Examples returns the eye code blocks of a SPECS.md, in order.
func Examples(spec []byte) []Example {
	var out []Example
	lines := strings.Split(string(spec), "\n")
	section, slug, count := 0, "", 0
	for i := 0; i < len(lines); i++ {
		if m := heading.FindStringSubmatch(lines[i]); m != nil {
			section, _ = strconv.Atoi(m[1]) // the pattern only matches digits
			slug = strings.Trim(nonWord.ReplaceAllString(strings.ToLower(m[2]), "-"), "-")
			count = 0
			continue
		}
		if strings.TrimSpace(lines[i]) != "```eye" {
			continue
		}
		indent := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " "))]
		var body []string
		for i++; i < len(lines) && strings.TrimSpace(lines[i]) != "```"; i++ {
			body = append(body, strings.TrimPrefix(lines[i], indent))
		}
		count++
		src := strings.TrimRight(strings.Join(body, "\n"), "\n") + "\n"
		ex := Example{
			Section: section,
			Index:   count,
			Name:    fmt.Sprintf("spec-%02d-%s-%d.eye", section, slug, count),
			Source:  src,
		}
		if !parses(src) {
			wrapped := "fn example() {\n" + indentLines(src) + "}\n"
			if parses(wrapped) {
				ex.Source, ex.Wrapped = wrapped, true
			}
		}
		out = append(out, ex)
	}
	return out
}

func parses(src string) bool {
	return len(syntax.Parse([]byte(src)).Diagnostics) == 0
}

func indentLines(src string) string {
	var b strings.Builder
	for _, line := range strings.SplitAfter(src, "\n") {
		if strings.TrimSpace(line) != "" {
			b.WriteString("  ")
		}
		b.WriteString(line)
	}
	return b.String()
}
