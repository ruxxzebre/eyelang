package check

import (
	"os"
	"path/filepath"
	"strings"

	"eyelang/ir"
	"eyelang/syntax"
)

// A program's eye.toml marks its root and may list modules: more
// directories to find imports in, after the program's own (SPECS.md §13).
// It is the one key the file holds so far, so this reads just enough TOML
// for it: comments, and a key whose value is a list of strings.

// readConfig reads the eye.toml in the program's root, if there is one, and
// sets the directories imports are looked for in: the root, then each
// directory modules lists, in order.
func (c *checker) readConfig() {
	c.importRoots = []string{c.root}
	path := filepath.Join(c.root, "eye.toml")
	src, err := c.readInput(path)
	if err != nil {
		return // no eye.toml: the program is its directory alone
	}
	f := &ir.File{Path: path, Src: src, Tree: syntax.Parse(src)}
	repo := c.repositoryInput(c.root)
	for _, e := range parseConfig(string(src)) {
		at := ir.Pos{File: f, Offset: e.offset, End: e.end}
		if e.err != "" {
			c.errorf(at, "%s", e.err)
			continue
		}
		if filepath.IsAbs(e.value) {
			c.errorf(at, "modules lists %s: a module directory is written relative to eye.toml, so that a copy of the program finds it", e.value)
			continue
		}
		dir := filepath.Join(c.root, filepath.FromSlash(e.value))
		if info, err := c.statInput(dir); (err != nil || !info.IsDir()) && !c.overlayHasDir(dir) {
			c.errorf(at, "modules lists %s, but there is no directory %s", e.value, dir)
			continue
		}
		if repo != "" && !within(repo, dir) {
			c.errorf(at, "modules lists %s, which is outside the repository at %s, so a copy of the repository wouldn't have it", e.value, repo)
			continue
		}
		c.importRoots = append(c.importRoots, dir)
	}
}

// configEntry is one directory modules lists, with where it is written, or
// a problem with the file.
type configEntry struct {
	value       string
	offset, end int
	err         string
}

// parseConfig reads eye.toml's text: `modules = ["../..", "vendor"]`, over
// as many lines as it needs, and comments.
func parseConfig(src string) []configEntry {
	toks := configTokens(src)
	var out []configEntry
	fail := func(t configToken, format string) []configEntry {
		return append(out, configEntry{offset: t.offset, end: t.offset + len(t.text), err: format})
	}
	for i := 0; i < len(toks); {
		key := toks[i]
		if key.kind != 'k' {
			return fail(key, "eye.toml: expected a key, as in modules = [\"../..\"], found "+key.text)
		}
		if key.text != "modules" {
			return fail(key, "eye.toml has no key "+key.text+": the one key it holds is modules")
		}
		if i+2 >= len(toks) || toks[i+1].text != "=" || toks[i+2].text != "[" {
			return fail(key, "eye.toml: modules takes a list of directories, as in modules = [\"../..\"]")
		}
		i += 3
		for {
			if i >= len(toks) {
				return fail(key, "eye.toml: the list of modules has no closing ]")
			}
			t := toks[i]
			if t.text == "]" {
				i++
				break
			}
			if t.kind != 's' {
				return fail(t, "eye.toml: expected a directory in quotes in the list of modules, found "+t.text)
			}
			out = append(out, configEntry{value: t.value, offset: t.offset, end: t.offset + len(t.text)})
			i++
			if i < len(toks) && toks[i].text == "," {
				i++
			} else if i < len(toks) && toks[i].text != "]" {
				return fail(toks[i], "eye.toml: expected , or ] in the list of modules, found "+toks[i].text)
			}
		}
	}
	return out
}

// configToken is a key ('k'), a string ('s', with its value), or one of
// = [ ] , ('p'), or anything else ('?').
type configToken struct {
	kind   byte
	text   string
	value  string
	offset int
}

func configTokens(src string) []configToken {
	var toks []configToken
	for i := 0; i < len(src); {
		ch := src[i]
		switch {
		case ch == '#':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n':
			i++
		case ch == '=' || ch == '[' || ch == ']' || ch == ',':
			toks = append(toks, configToken{kind: 'p', text: string(ch), offset: i})
			i++
		case ch == '"' || ch == '\'':
			// A basic string takes \" and \\; a literal one takes nothing.
			start := i
			var b strings.Builder
			i++
			for i < len(src) && src[i] != ch && src[i] != '\n' {
				if ch == '"' && src[i] == '\\' && i+1 < len(src) {
					i++
				}
				b.WriteByte(src[i])
				i++
			}
			if i >= len(src) || src[i] != ch {
				toks = append(toks, configToken{kind: '?', text: src[start:i], offset: start})
				continue
			}
			i++
			toks = append(toks, configToken{kind: 's', text: src[start:i], value: b.String(), offset: start})
		case ch == '_' || ch == '-' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9':
			start := i
			for i < len(src) && (src[i] == '_' || src[i] == '-' || src[i] >= 'a' && src[i] <= 'z' || src[i] >= 'A' && src[i] <= 'Z' || src[i] >= '0' && src[i] <= '9') {
				i++
			}
			toks = append(toks, configToken{kind: 'k', text: src[start:i], offset: start})
		default:
			toks = append(toks, configToken{kind: '?', text: string(ch), offset: i})
			i++
		}
	}
	return toks
}

// repository returns the root of the git repository dir is in: the nearest
// directory upward with a .git. It returns "" outside one, as in a
// container's build, where nothing is checked.
func repository(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	for d := abs; ; {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}

// within reports whether dir is root or below it.
func within(root, dir string) bool {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, abs)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
