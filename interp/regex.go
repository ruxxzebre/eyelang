package interp

import (
	"errors"
	"regexp"
	"regexp/syntax"
	"strings"
	"sync"

	"eyelang/ir"
)

// The host functions of the standard module regex (check/std/regex.eye).
// A Regex is a record that holds its pattern; these compile the pattern the
// first time they see it and keep it in compiled. Go's regexp is RE2, so
// matching takes time linear in the text whatever the pattern.

func init() {
	hostFuncs["regex.problem"] = func(in *Interp, p map[string]Value, _ ir.Pos) Value {
		pattern, _ := p["pattern"].(string)
		if _, err := compileRegex(pattern); err != nil {
			return in.some(regexProblem(err))
		}
		return in.none()
	}
	hostFuncs["regex.escape"] = func(_ *Interp, p map[string]Value, _ ir.Pos) Value {
		text, _ := p["text"].(string)
		return regexp.QuoteMeta(text)
	}
	hostFuncs["regex.whole"] = func(in *Interp, p map[string]Value, at ir.Pos) Value {
		re := in.regex(p, at)
		text, _ := p["text"].(string)
		loc := re.whole.FindStringIndex(text)
		return loc != nil && loc[0] == 0 && loc[1] == len(text)
	}
	hostFuncs["regex.matches"] = hostRegexMatches
	hostFuncs["regex.replace"] = hostRegexReplace
	hostFuncs["regex.split"] = func(in *Interp, p map[string]Value, at ir.Pos) Value {
		re := in.regex(p, at)
		text, _ := p["text"].(string)
		return &List{Elems: toValues(re.find.Split(text, -1))}
	}
}

// A compiledRegex is a pattern compiled twice: to find its matches as RE2
// prefers them, leftmost and then first, and to find the longest, which
// says whether it matches the whole of a text.
type compiledRegex struct {
	find, whole *regexp.Regexp
}

// regexCache holds the patterns compiled so far. A program that builds many
// patterns at run time would fill it, so it starts again when it is full.
var regexCache = struct {
	sync.Mutex
	m map[string]*compiledRegex
}{m: map[string]*compiledRegex{}}

const regexCacheSize = 512

func compileRegex(pattern string) (*compiledRegex, error) {
	regexCache.Lock()
	re, ok := regexCache.m[pattern]
	regexCache.Unlock()
	if ok {
		return re, nil
	}
	find, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	// The whole text matches when the longest match at the leftmost place
	// is all of it: if any match covers the text, it starts leftmost and
	// none is longer. Wrapping the pattern in \A(?:…)\z instead would break
	// one that ends inside \Q…, which quotes the rest.
	whole := regexp.MustCompile(pattern) // it compiled once
	whole.Longest()
	re = &compiledRegex{find: find, whole: whole}
	regexCache.Lock()
	if len(regexCache.m) >= regexCacheSize {
		regexCache.m = map[string]*compiledRegex{}
	}
	regexCache.m[pattern] = re
	regexCache.Unlock()
	return re, nil
}

// regexProblem says what is wrong with a pattern: what Go's parser found,
// and the part of the pattern it found it in.
func regexProblem(err error) string {
	var se *syntax.Error
	if errors.As(err, &se) {
		if se.Expr == "" {
			return se.Code.String()
		}
		return se.Code.String() + " at `" + se.Expr + "`"
	}
	return err.Error()
}

// regex is the compiled form of p["pattern"], which regex.parse checked.
func (in *Interp) regex(p map[string]Value, at ir.Pos) *compiledRegex {
	pattern, _ := p["pattern"].(string)
	re, err := compileRegex(pattern)
	if err != nil {
		// Only a journal written by hand could hold a Regex parse didn't check.
		in.faultf(at, "the regular expression %q doesn't compile: %s", pattern, regexProblem(err))
	}
	return re
}

// regexLimit is p["limit"]: how many matches to find, -1 for all of them.
func regexLimit(p map[string]Value) int {
	if n, ok := p["limit"].(int64); ok && n >= 0 {
		return int(n)
	}
	return -1
}

// hostRegexMatches is the matches of the pattern in the text, as Match
// records.
func hostRegexMatches(in *Interp, p map[string]Value, at ir.Pos) Value {
	re := in.regex(p, at)
	text, _ := p["text"].(string)
	t := in.stdModule("regex", at).Types["Match"]
	names := re.find.SubexpNames()
	var out []Value
	for _, loc := range re.find.FindAllStringSubmatchIndex(text, regexLimit(p)) {
		groups := make([]Value, 0, len(loc)/2-1)
		named := &Map{}
		for g := 1; g < len(loc)/2; g++ {
			if loc[2*g] < 0 {
				groups = append(groups, in.none())
				continue
			}
			s := text[loc[2*g]:loc[2*g+1]]
			groups = append(groups, in.some(s))
			if names[g] != "" {
				named = named.with(names[g], s)
			}
		}
		out = append(out, &Record{
			Type:  t,
			Names: []string{"text", "range", "groups", "named"},
			Vals: []Value{
				text[loc[0]:loc[1]],
				&RangeVal{Start: in.offset(loc[0]), End: in.offset(loc[1])},
				&List{Elems: groups},
				named,
			},
		})
	}
	return &List{Elems: out}
}

// hostRegexReplace replaces the first limit matches, or all of them for -1,
// expanding the template in each (see expandTemplate).
func hostRegexReplace(in *Interp, p map[string]Value, at ir.Pos) Value {
	re := in.regex(p, at)
	text, _ := p["text"].(string)
	template, _ := p["with"].(string)
	var b strings.Builder
	last := 0
	for _, loc := range re.find.FindAllStringSubmatchIndex(text, regexLimit(p)) {
		b.WriteString(text[last:loc[0]])
		expandTemplate(&b, template, text, loc, re.find)
		last = loc[1]
	}
	b.WriteString(text[last:])
	return b.String()
}

// expandTemplate writes template with its group references filled in from
// the match at loc: `$1` and `$<1>` by number, `$<name>` by name, and `$$`
// for a `$`. A `$` before anything else is itself. Unlike Go's Expand, `$1x`
// is group 1 then x, and a name needs the angle brackets, so a letter after
// a group's number never changes which group is meant. Angle brackets, as
// JavaScript has them, rather than Go's braces, which an Eyelang string
// would read as interpolation.
func expandTemplate(b *strings.Builder, template, text string, loc []int, re *regexp.Regexp) {
	group := func(i int) {
		if i >= 0 && 2*i+1 < len(loc) && loc[2*i] >= 0 {
			b.WriteString(text[loc[2*i]:loc[2*i+1]])
		}
	}
	for i := 0; i < len(template); i++ {
		c := template[i]
		if c != '$' || i+1 == len(template) {
			b.WriteByte(c)
			continue
		}
		next := template[i+1]
		switch {
		case next == '$':
			b.WriteByte('$')
			i++
		case isDigitByte(next):
			j := i + 1
			for j < len(template) && isDigitByte(template[j]) {
				j++
			}
			group(groupNumber(template[i+1 : j]))
			i = j - 1
		case next == '<':
			end := strings.IndexByte(template[i+2:], '>')
			if end < 0 {
				b.WriteByte(c)
				continue
			}
			name := template[i+2 : i+2+end]
			switch {
			case name == "":
			case strings.Trim(name, "0123456789") == "":
				group(groupNumber(name))
			default:
				group(re.SubexpIndex(name))
			}
			i += 2 + end
		default:
			b.WriteByte(c)
		}
	}
}

func isDigitByte(c byte) bool { return c >= '0' && c <= '9' }

// groupNumber reads a group's number, or -1 for one too large to be a group.
func groupNumber(digits string) int {
	n := 0
	for _, d := range []byte(digits) {
		n = n*10 + int(d-'0')
		if n > 1<<20 {
			return -1
		}
	}
	return n
}
