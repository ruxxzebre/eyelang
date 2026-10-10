package lsp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Probe answers one request the way an editor would get it, and prints the
// result as plain text: each location as path:line:column, 1-based, with
// paths below the working directory relative to it. method is "definition"
// or "references"; at is FILE:LINE:COLUMN, the column counted in characters.
// The working directory is the workspace, where references to the prelude
// and built-ins are looked for.
func Probe(method, at string, out io.Writer) error {
	parts := strings.Split(at, ":")
	if len(parts) < 3 {
		return fmt.Errorf("%s: want FILE:LINE:COLUMN", at)
	}
	line, err1 := strconv.Atoi(parts[len(parts)-2])
	col, err2 := strconv.Atoi(parts[len(parts)-1])
	if err1 != nil || err2 != nil || line < 1 || col < 1 {
		return fmt.Errorf("%s: want FILE:LINE:COLUMN, with a line and a column from 1", at)
	}
	path, err := filepath.Abs(strings.Join(parts[:len(parts)-2], ":"))
	if err != nil {
		return err
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	s := &server{docs: map[string]*document{}, folders: []string{wd}}
	d := newDocument(pathURI(path), 1, src)
	s.docs[d.uri] = d
	p := Position{Line: line - 1, Character: col - 1}
	var result any
	var rerr *responseError
	switch method {
	case "definition":
		result, rerr = s.definition(d, p)
	case "references":
		result, rerr = s.references(d, p, true)
	default:
		return fmt.Errorf("can't probe %s: only definition and references", method)
	}
	if rerr != nil {
		return rerr
	}
	locs, _ := result.([]location)
	if len(locs) == 0 {
		fmt.Fprintln(out, "nothing")
	}
	for _, l := range locs {
		file, _ := uriPath(l.URI)
		if rel, err := filepath.Rel(wd, file); err == nil && !strings.HasPrefix(rel, "..") {
			file = rel
		}
		fmt.Fprintf(out, "%s:%d:%d\n", file, l.Range.Start.Line+1, l.Range.Start.Character+1)
	}
	return nil
}
