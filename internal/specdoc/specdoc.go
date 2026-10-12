// Package specdoc extracts the Eyelang examples from SPECS.md.
//
// Every `eye` code block in the spec is either a whole file or the statements
// of a function body. Statements are wrapped in `fn example() { … }` so that
// each example is a file both parsers can check.
//
// A block's fence can say more after `eye`, for the test that checks every
// example (check_test.go):
//
//   - context=NAME: the example uses names it doesn't declare, which the
//     program in testdata/specctx/NAME declares; the example is checked as
//     one more file of that program's own module.
//   - fails=TASK: the example shows what ROADMAP.md's task TASK will build,
//     so it doesn't check until that task is done.
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
	Line    int    // the line of SPECS.md that the block's code starts on
	Context string // context=NAME on the fence: the program it is checked in
	Fails   string // fails=TASK on the fence: the task it waits for
	// Unknown holds the words after `eye` on the fence that aren't
	// context= or fails=.
	Unknown []string
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
		info, ok := eyeFence(lines[i])
		if !ok {
			continue
		}
		indent := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " "))]
		start := i + 2 // the first line of code, counting from 1
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
			Line:    start,
		}
		for _, word := range strings.Fields(info) {
			switch key, value, _ := strings.Cut(word, "="); {
			case key == "context" && value != "":
				ex.Context = value
			case key == "fails" && value != "":
				ex.Fails = value
			default:
				ex.Unknown = append(ex.Unknown, word)
			}
		}
		if statements(src) {
			wrapped := "fn example() {\n" + indentLines(src) + "}\n"
			if parses(wrapped) {
				ex.Source, ex.Wrapped = wrapped, true
			}
		}
		out = append(out, ex)
	}
	return out
}

// eyeFence reports whether a line opens an eye code block, and returns what
// follows `eye` on it.
func eyeFence(line string) (string, bool) {
	s := strings.TrimSpace(line)
	if s == "```eye" {
		return "", true
	}
	if rest, ok := strings.CutPrefix(s, "```eye "); ok {
		return rest, true
	}
	return "", false
}

// statements reports whether src is the statements of a function body
// rather than a file: it doesn't parse as a file, or everything in it is a
// let that isn't public, as `let (a, b) = both(…)` is.
func statements(src string) bool {
	f := syntax.Parse([]byte(src))
	if len(f.Diagnostics) > 0 {
		return true
	}
	items := f.Root.NamedChildren()
	for _, item := range items {
		if item.Kind != syntax.LetStatement {
			return false
		}
		for _, c := range item.Children {
			if c.IsToken() && c.Text(f.Src) == "pub" {
				return false
			}
		}
	}
	return len(items) > 0
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

// File is one file of an example's program.
type File struct {
	Path   string // relative to the program's directory, with slashes
	Source string
	// Line is the line of SPECS.md that the file's first line is on. A
	// wrapped example's first line is the wrapper's, before the block.
	Line int
}

// fileHeader is a comment that starts a file of a block that shows several:
// `# shelves/shelves.eye`, or `# main.eye: what it shows`.
var fileHeader = regexp.MustCompile(`^# ([\w./-]+\.eye)(:.*)?$`)

// Files returns the example as the files of a program. A block that shows
// several files names each with a comment line of its own, such as
// `# notify/notify.eye`, which starts that file; what comes before the first
// is example.eye, as is a block that names none.
func (ex Example) Files() []File {
	if ex.Wrapped {
		return []File{{Path: "example.eye", Source: ex.Source, Line: ex.Line - 1}}
	}
	var files []File
	cur := &File{Path: "example.eye", Line: ex.Line}
	for i, line := range strings.SplitAfter(strings.TrimSuffix(ex.Source, "\n"), "\n") {
		if m := fileHeader.FindStringSubmatch(strings.TrimSuffix(line, "\n")); m != nil {
			if strings.TrimSpace(cur.Source) != "" {
				files = append(files, *cur)
			}
			cur = &File{Path: m[1], Line: ex.Line + i}
		}
		cur.Source += line
	}
	files = append(files, *cur)
	for i := range files {
		files[i].Source = strings.TrimRight(files[i].Source, "\n") + "\n"
	}
	return files
}
