package specdoc

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Page is one page of the docs site made from a section of SPECS.md.
type Page struct {
	Path    string // under docs/, such as "reference/language/entities.md"
	Link    string // the site path, such as "/reference/language/entities"
	Title   string // the section's title without its number
	Section int    // the section number; 0 for an index page
	Content string
}

// Sidebar is one group of the site's sidebar.
type Sidebar struct {
	Text  string        `json:"text"`
	Link  string        `json:"link,omitempty"`
	Items []SidebarItem `json:"items"`
}

// SidebarItem is one link in a sidebar group.
type SidebarItem struct {
	Text string `json:"text"`
	Link string `json:"link"`
}

// Where each section goes on the site. The rules of the language are the
// reference; the goals, decisions and open questions are the design. Sections
// left out (what happens to Eyelang 1, the implementation plan) are about
// building Eyelang, not using it; links to them point at SPECS.md on GitHub.
var (
	referenceSections = []int{3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 23}
	designSections    = []int{1, 2, 24}
)

const (
	referenceDir = "reference/language"
	designDir    = "design"
)

var (
	link       = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	sectionRef = regexp.MustCompile(`\b([Ss]ection) (\d+)\b`)
	codeSpan   = regexp.MustCompile("`[^`]*`")
	slugDrop   = regexp.MustCompile(`[^\p{L}\p{N} _-]`)
	fullLink   = regexp.MustCompile(`\[[^\]]*\]\([^)]*\)`)
)

// section is one `## N. Title` section of the spec.
type section struct {
	number int
	title  string
	anchor string // GitHub's anchor for the heading, such as "14-entities"
	lines  []string
}

// Pages splits a SPECS.md into the pages of the docs site's language
// reference and design sections, with their index pages and sidebars. Links
// between sections become links between pages, links to repository files
// point at repo (a URL ending in "/"), and "section 14" becomes a link.
func Pages(spec []byte, repo string) ([]Page, []Sidebar) {
	intro, sections := split(string(spec))
	byNumber := map[int]*section{}
	for i := range sections {
		byNumber[sections[i].number] = &sections[i]
	}

	// Where each section's page is, and which page each heading anchor is on.
	links := map[int]string{}
	anchors := map[string]string{}
	place := func(nums []int, dir string) {
		for _, n := range nums {
			s := byNumber[n]
			if s == nil {
				continue
			}
			links[n] = "/" + dir + "/" + slug(s.title)
			anchors[s.anchor] = links[n]
		}
	}
	place(referenceSections, referenceDir)
	place(designSections, designDir)
	for _, s := range sections {
		if links[s.number] == "" {
			anchors[s.anchor] = repo + "SPECS.md#" + s.anchor
		}
	}
	r := rewriter{repo: repo, links: links, anchors: anchors}

	var pages []Page
	group := func(nums []int, dir, title, summary string) Sidebar {
		sb := Sidebar{Text: title, Link: "/" + dir + "/"}
		var list strings.Builder
		for _, n := range nums {
			s := byNumber[n]
			if s == nil {
				continue
			}
			pages = append(pages, r.page(s, dir))
			sb.Items = append(sb.Items, SidebarItem{Text: s.title, Link: links[n]})
			fmt.Fprintf(&list, "- [%s](%s)\n", s.title, links[n])
		}
		index := frontmatter(title) + "# " + title + "\n\n" + summary + "\n\n" + list.String()
		pages = append(pages, Page{Path: dir + "/index.md", Link: "/" + dir + "/", Title: title, Content: index})
		return sb
	}
	reference := group(referenceSections, referenceDir, "Language reference",
		r.text(intro)+"\n\nEach page is one section of the language's specification, "+
			"[SPECS.md]("+repo+"SPECS.md), which is where the rules are written and changed.")
	design := group(designSections, designDir, "Design",
		"Why Eyelang is the way it is: the goals every feature is weighed against, "+
			"each decision with its reason, and the questions that are still open. "+
			"Eyelang changes whenever real programs show a better way, until v1.0.")
	return pages, []Sidebar{reference, design}
}

// split returns the spec's introduction (the text before the contents, without
// the title and status) and its sections.
func split(spec string) (string, []section) {
	var intro []string
	var sections []section
	var cur *section
	fence := false
	inContents := false
	for line := range strings.SplitSeq(spec, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fence = !fence
		}
		if !fence && strings.HasPrefix(line, "## ") {
			inContents = line == "## Contents"
			if m := heading.FindStringSubmatch(line); m != nil {
				n, _ := strconv.Atoi(m[1])
				sections = append(sections, section{number: n, title: m[2], anchor: githubSlug(m[1] + ". " + m[2])})
				cur = &sections[len(sections)-1]
				continue
			}
		}
		switch {
		case cur != nil:
			cur.lines = append(cur.lines, line)
		case !inContents && !strings.HasPrefix(line, "# "):
			intro = append(intro, line)
		}
	}
	// The status paragraph is about building Eyelang; the reference starts at
	// the paragraph that says what the language is.
	text := strings.TrimSpace(strings.Join(intro, "\n"))
	if i := strings.Index(text, "\n\n"); strings.HasPrefix(text, "Status:") && i >= 0 {
		text = strings.TrimSpace(text[i:])
	}
	return text, sections
}

type rewriter struct {
	repo    string
	links   map[int]string // section number to page
	anchors map[string]string
}

func (r rewriter) page(s *section, dir string) Page {
	body := strings.Trim(strings.Join(s.lines, "\n"), "\n")
	return Page{
		Path:    dir + "/" + slug(s.title) + ".md",
		Link:    r.links[s.number],
		Title:   s.title,
		Section: s.number,
		Content: frontmatter(s.title) + "# " + s.title + "\n\n" + r.text(body) + "\n",
	}
}

// text rewrites the links of Markdown text, leaving code blocks alone.
func (r rewriter) text(md string) string {
	lines := strings.Split(md, "\n")
	fence := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fence = !fence
			continue
		}
		if !fence {
			lines[i] = r.line(line)
		}
	}
	return strings.Join(lines, "\n")
}

// line rewrites one line of prose, outside its code spans.
func (r rewriter) line(line string) string {
	var b strings.Builder
	last := 0
	for _, span := range codeSpan.FindAllStringIndex(line, -1) {
		b.WriteString(r.prose(line[last:span[0]]))
		b.WriteString(line[span[0]:span[1]])
		last = span[1]
	}
	b.WriteString(r.prose(line[last:]))
	return b.String()
}

func (r rewriter) prose(s string) string {
	s = link.ReplaceAllStringFunc(s, func(m string) string {
		target := m[2 : len(m)-1]
		switch {
		case strings.HasPrefix(target, "#"):
			if to, ok := r.anchors[target[1:]]; ok {
				return "](" + to + ")"
			}
			return "](" + r.repo + "SPECS.md" + target + ")"
		case strings.Contains(target, "://"):
			return m
		default:
			return "](" + r.repo + target + ")"
		}
	})
	// "section 14" links to its page, unless it is already link text.
	return replaceOutsideLinks(s, func(t string) string {
		return sectionRef.ReplaceAllStringFunc(t, func(m string) string {
			n, _ := strconv.Atoi(sectionRef.FindStringSubmatch(m)[2])
			if to, ok := r.links[n]; ok {
				return "[" + m + "](" + to + ")"
			}
			return m
		})
	})
}

// replaceOutsideLinks applies f to the parts of s that aren't inside a
// Markdown link.
func replaceOutsideLinks(s string, f func(string) string) string {
	var b strings.Builder
	last := 0
	for _, m := range fullLink.FindAllStringIndex(s, -1) {
		b.WriteString(f(s[last:m[0]]))
		b.WriteString(s[m[0]:m[1]])
		last = m[1]
	}
	b.WriteString(f(s[last:]))
	return b.String()
}

func frontmatter(title string) string {
	return "---\ntitle: " + strconv.Quote(title) + "\n---\n\n" +
		"<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->\n\n"
}

// slug is a page's file name: "Contracts: talking to other services" is
// "contracts-talking-to-other-services".
func slug(title string) string {
	return strings.Trim(nonWord.ReplaceAllString(strings.ToLower(title), "-"), "-")
}

// githubSlug is the anchor GitHub gives a heading, which SPECS.md's own links
// use.
func githubSlug(heading string) string {
	s := strings.ToLower(heading)
	s = strings.ReplaceAll(s, "`", "")
	s = slugDrop.ReplaceAllString(s, "")
	return strings.ReplaceAll(s, " ", "-")
}
