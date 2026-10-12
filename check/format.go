package check

import (
	"bytes"
	"path/filepath"
	"strings"

	"eyelang/ir"
	"eyelang/syntax"
)

// FormatSession checks modules for the formatter, sharing parsed files but
// not inferred types or resolved programs. It omits the editor's name index.
// The zero value is ready to use. A session is used by one goroutine.
type FormatSession struct {
	files map[string]*ir.File
}

// Check checks module and its imports within the program at dir. Each call
// reads the sources again and reuses a parsed file only if its bytes match.
func (s *FormatSession) Check(dir, moduleDir string) *Result {
	if s.files == nil {
		s.files = map[string]*ir.File{}
	}
	path := ""
	if moduleDir != "" {
		if rel, err := filepath.Rel(dir, moduleDir); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			path = strings.ReplaceAll(filepath.ToSlash(rel), "/", ".")
		}
	}
	c := &checker{root: dir, modules: map[string]*module{}, parsed: s.files}
	prog, diags := c.program(path)
	return &Result{Program: prog, Diagnostics: diags, c: c}
}

// File returns the last parsed version of path, or nil if it hasn't been
// checked. Callers must not modify it and must check its source still matches
// before using it to format a file reread from disk.
func (s *FormatSession) File(path string) *syntax.File {
	if f := s.files[path]; f != nil {
		return f.Tree
	}
	return nil
}

func (c *checker) parseFile(path string, src []byte) *ir.File {
	if f := c.parsed[path]; f != nil && bytes.Equal(f.Src, src) {
		return f
	}
	f := parseFile(path, src)
	if c.parsed != nil {
		c.parsed[path] = f
	}
	return f
}
