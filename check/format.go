package check

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"eyelang/ir"
	"eyelang/syntax"
)

// FormatSession checks modules for the formatter, sharing parsed files but
// not inferred types or resolved programs. It omits the editor's name index.
// The zero value is ready to use. Parsed files are immutable and may be
// shared by concurrent checks, while each check owns its inferred program.
type FormatSession struct {
	mu       sync.Mutex
	files    map[string]*ir.File
	inputs   map[string]FormatInput
	unstable bool
}

// Check checks module and its imports within the program at dir. Each call
// reads the sources again and reuses a parsed file only if its bytes match.
func (s *FormatSession) Check(dir, moduleDir string) *Result {
	path := ""
	if moduleDir != "" {
		if rel, err := filepath.Rel(dir, moduleDir); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			path = strings.ReplaceAll(filepath.ToSlash(rel), "/", ".")
		}
	}
	c := &checker{root: dir, modules: map[string]*module{}, parsed: s}
	prog, diags := c.program(path)
	return &Result{Program: prog, Diagnostics: diags, c: c}
}

// File returns the last parsed version of path, or nil if it hasn't been
// checked. Callers must not modify it and must check its source still matches
// before using it to format a file reread from disk.
func (s *FormatSession) File(path string) *syntax.File {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f := s.files[path]; f != nil {
		return f.Tree
	}
	return nil
}

func (c *checker) parseFile(path string, src []byte) *ir.File {
	if c.parsed == nil {
		return parseFile(path, src)
	}
	s := c.parsed
	s.mu.Lock()
	f := s.files[path]
	s.mu.Unlock()
	if f != nil && bytes.Equal(f.Src, src) {
		return f
	}
	// Parsed without the lock, so concurrent checks parse at the same time.
	// When two parse the same file, the first one stored is the one shared.
	parsed := parseFile(path, src)
	s.mu.Lock()
	defer s.mu.Unlock()
	if f := s.files[path]; f != nil && bytes.Equal(f.Src, src) {
		return f
	}
	if s.files == nil {
		s.files = map[string]*ir.File{}
	}
	s.files[path] = parsed
	return parsed
}

// FormatInput records a filesystem input used by a formatting check.
// Content hashes, directory entries and missing paths invalidate saved
// results without relying on timestamps.
type FormatInput struct {
	Path  string
	Kind  string
	Value string
}

func inputDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func fileInput(path string, data []byte, err error) FormatInput {
	value := inputDigest(data)
	if err != nil {
		value = "error:" + err.Error()
	}
	return FormatInput{path, "file", value}
}

func dirInput(path string, entries []fs.DirEntry, err error) FormatInput {
	if err != nil {
		return FormatInput{path, "dir", "error:" + err.Error()}
	}
	var data strings.Builder
	for _, entry := range entries {
		fmt.Fprintf(&data, "%s\x00%t\n", entry.Name(), entry.IsDir())
	}
	return FormatInput{path, "dir", inputDigest([]byte(data.String()))}
}

func statInput(path string, info fs.FileInfo, err error) FormatInput {
	value := "file"
	if err != nil {
		value = "error:" + err.Error()
	} else if info.IsDir() {
		value = "dir"
	}
	return FormatInput{path, "stat", value}
}

// Matches rereads an input and compares exactly what the checker used.
func (in FormatInput) Matches() bool {
	var now FormatInput
	switch in.Kind {
	case "file":
		data, err := os.ReadFile(in.Path)
		now = fileInput(in.Path, data, err)
	case "dir":
		entries, err := os.ReadDir(in.Path)
		now = dirInput(in.Path, entries, err)
	case "stat":
		info, err := os.Stat(in.Path)
		now = statInput(in.Path, info, err)
	default:
		return false
	}
	return now == in
}

func (s *FormatSession) record(in FormatInput) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inputs == nil {
		s.inputs = map[string]FormatInput{}
	}
	key := in.Kind + "\x00" + in.Path
	if prev, ok := s.inputs[key]; ok && prev != in {
		s.unstable = true
	}
	s.inputs[key] = in
}

// RecordSource includes a source read by the formatter itself.
func (s *FormatSession) RecordSource(path string, data []byte, err error) {
	s.record(fileInput(path, data, err))
}

// Inputs returns the complete observed dependency set. A false stable
// means an input changed during checking and the result must not be saved.
func (s *FormatSession) Inputs() (inputs []FormatInput, stable bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, in := range s.inputs {
		inputs = append(inputs, in)
	}
	sort.Slice(inputs, func(i, j int) bool {
		if inputs[i].Path == inputs[j].Path {
			return inputs[i].Kind < inputs[j].Kind
		}
		return inputs[i].Path < inputs[j].Path
	})
	return inputs, !s.unstable
}

// Root records the ancestor manifests consulted when choosing a program.
func (s *FormatSession) Root(path string) string {
	for dir := path; ; dir = filepath.Dir(dir) {
		manifest := filepath.Join(dir, "eye.toml")
		info, err := os.Stat(manifest)
		s.record(statInput(manifest, info, err))
		if err == nil && !info.IsDir() {
			return dir
		}
		if filepath.Dir(dir) == dir {
			return path
		}
	}
}

func (c *checker) readInput(path string) ([]byte, error) {
	data, err := c.readFile(path)
	if c.parsed != nil {
		c.parsed.record(fileInput(path, data, err))
	}
	return data, err
}

func (c *checker) readDirInput(path string) ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(path)
	if c.parsed != nil {
		c.parsed.record(dirInput(path, entries, err))
	}
	return entries, err
}

func (c *checker) statInput(path string) (fs.FileInfo, error) {
	info, err := os.Stat(path)
	if c.parsed != nil {
		c.parsed.record(statInput(path, info, err))
	}
	return info, err
}

func (c *checker) repositoryInput(dir string) string {
	if c.parsed == nil {
		return repository(dir)
	}
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := c.statInput(filepath.Join(d, ".git")); err == nil {
			return d
		}
		if filepath.Dir(d) == d {
			return ""
		}
	}
}
