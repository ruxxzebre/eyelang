package syntax

import (
	"regexp"
	"strings"
)

// DocComment returns the documentation of a declaration, member, field or
// variant: the `##` lines directly above it, each without its `##` and one
// space after it. A blank line or a plain `#` comment ends a doc comment, so
// only the lines that touch the node count. The text is Markdown.
func DocComment(n *Node, src []byte) string {
	run, _ := docRuns(firstToken(n), src)
	return strings.TrimSpace(strings.Join(run, "\n"))
}

// ModuleDoc returns the documentation of a file's module: a `##` block before
// the first declaration that a blank line separates from it. Plain comments
// may come before the block.
//
//	## Train reminders: when to leave for the next train home.
//
//	pub fn next(...) = ...
func ModuleDoc(f *File) string {
	if f == nil || f.Root == nil {
		return ""
	}
	_, module := docRuns(firstToken(f.Root), f.Src)
	return strings.TrimSpace(strings.Join(module, "\n"))
}

// Summary returns the first sentence of a doc comment, which indexes,
// completion lists and hover headers show. A sentence ends at a full stop
// followed by a space or a line break, outside code spans, or at a blank
// line.
func Summary(doc string) string {
	if i := strings.Index(doc, "\n\n"); i >= 0 {
		doc = doc[:i]
	}
	code := false
	for i := 0; i < len(doc); i++ {
		switch doc[i] {
		case '`':
			code = !code
		case '.':
			if !code && (i+1 == len(doc) || doc[i+1] == ' ' || doc[i+1] == '\n') {
				doc = doc[:i+1]
			}
		}
	}
	return strings.Join(strings.Fields(doc), " ")
}

func firstToken(n *Node) *Node {
	for !n.IsToken() && len(n.Children) > 0 {
		n = n.Children[0]
	}
	return n
}

// docRuns reads the trivia before a token. It returns the `##` lines that
// touch the token, and the first `##` block that a blank line ends.
func docRuns(tok *Node, src []byte) (attached, detached []string) {
	var run []string
	for _, t := range tok.Leading {
		s := string(src[t.Span.Start:t.Span.End])
		switch {
		case t.Kind == Whitespace:
			if strings.Count(s, "\n") > 1 {
				if detached == nil && run != nil {
					detached = run
				}
				run = nil // a blank line ends a doc comment
			}
		case t.Kind == Comment && strings.HasPrefix(s, "##"):
			line := strings.TrimPrefix(s, "##")
			run = append(run, strings.TrimPrefix(strings.TrimRight(line, " \t\r"), " "))
		default:
			run = nil
		}
	}
	return run, detached
}

var (
	docLink  = regexp.MustCompile(`\[([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*)\]`)
	codeSpan = regexp.MustCompile("`[^`]*`")
)

// DocLinks replaces each `[Name]`, `[Type.member]` or `[module.Name]` in a
// doc comment's prose with what replace returns for the name, when it returns
// true. Code spans, code blocks and text that is already a Markdown link, such
// as `[text](url)` or `[text][ref]`, are left alone.
func DocLinks(doc string, replace func(name string) (string, bool)) string {
	lines := strings.Split(doc, "\n")
	fence := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fence = !fence
			continue
		}
		if fence || strings.HasPrefix(line, "#") {
			continue
		}
		var b strings.Builder
		last := 0
		for _, span := range codeSpan.FindAllStringIndex(line, -1) {
			b.WriteString(proseLinks(line[last:span[0]], replace))
			b.WriteString(line[span[0]:span[1]])
			last = span[1]
		}
		b.WriteString(proseLinks(line[last:], replace))
		lines[i] = b.String()
	}
	return strings.Join(lines, "\n")
}

func proseLinks(s string, replace func(string) (string, bool)) string {
	var b strings.Builder
	last := 0
	for _, m := range docLink.FindAllStringSubmatchIndex(s, -1) {
		if m[1] < len(s) && (s[m[1]] == '(' || s[m[1]] == '[') || m[0] > 0 && s[m[0]-1] == ']' {
			continue
		}
		if r, ok := replace(s[m[2]:m[3]]); ok {
			b.WriteString(s[last:m[0]] + r)
			last = m[1]
		}
	}
	return b.String() + s[last:]
}
