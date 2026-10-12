package lsp

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"eyelang/syntax"
)

// Workspace answers an editor's requests for one program held in memory,
// with no connection and no files on disk: the online playground's editor
// calls it through WebAssembly. Its answers are the language server's own;
// positions are LSP positions, a 0-based line and a column in UTF-16 code
// units, which is also how a browser counts a string.
type Workspace struct {
	s *server
}

// NewWorkspace starts a workspace for the program whose root directory is
// root. The directory needn't exist: every file the program has is given
// with Set.
func NewWorkspace(root string) *Workspace {
	return &Workspace{s: &server{docs: map[string]*document{}, root: filepath.Clean(root)}}
}

// Set gives the text of the file at path, a path below the root, replacing
// what it had.
func (w *Workspace) Set(path string, text []byte) {
	uri := pathURI(path)
	version := 1
	if d := w.s.docs[uri]; d != nil {
		version = d.version + 1
	}
	w.s.docs[uri] = newDocument(uri, version, text)
}

// Remove forgets the file at path.
func (w *Workspace) Remove(path string) { delete(w.s.docs, pathURI(path)) }

// Overlay returns every file's text by path, as check.Options takes it.
func (w *Workspace) Overlay() map[string][]byte { return w.s.overlay(w.s.root) }

// Diagnostic is a problem in a file, or in the program when Path is "".
type Diagnostic struct {
	Path    string `json:"path"`
	Range   Range  `json:"range"`
	Message string `json:"message"`
}

// Programs returns the directories of the workspace's programs: the root,
// then each directory below it whose eye.toml is open, sorted. A module two
// programs import is checked in both.
func (w *Workspace) Programs() []string {
	out := []string{w.s.root}
	for _, p := range w.Paths() {
		if filepath.Base(p) == "eye.toml" && filepath.Dir(p) != w.s.root {
			out = append(out, filepath.Dir(p))
		}
	}
	return out
}

// Check checks every program, every module of each, and returns their
// problems, those of each file in the order they are in it, each once.
func (w *Workspace) Check() []Diagnostic {
	out := []Diagnostic{}
	seen := map[Diagnostic]bool{}
	for _, root := range w.Programs() {
		out = w.checkProgram(root, out, seen)
	}
	return out
}

// checkProgram adds the problems of the program at root to out.
func (w *Workspace) checkProgram(root string, out []Diagnostic, seen map[Diagnostic]bool) []Diagnostic {
	res := w.s.checkAll(root, "")
	for _, d := range res.Diagnostics {
		diag := Diagnostic{Message: d.Message}
		if f := d.Pos.File; f != nil {
			doc := w.s.docs[pathURI(f.Path)]
			if doc == nil {
				// A problem in the prelude or a standard module, which the
				// editor doesn't show: said at the program's level.
				diag.Message = d.String()
			} else {
				diag.Path = f.Path
				diag.Range = doc.rangeOf(syntax.Span{Start: d.Pos.Offset, End: max(d.Pos.End, d.Pos.Offset)})
				if diag.Range.Start == diag.Range.End {
					// Show something for a missing token: the character
					// after it, if there is one on the line.
					if end := doc.position(d.Pos.Offset + 1); end.Line == diag.Range.Start.Line {
						diag.Range.End = end
					}
				}
			}
		}
		if !seen[diag] {
			seen[diag] = true
			out = append(out, diag)
		}
	}
	return out
}

// Hover returns the Markdown an editor shows for the name or keyword at p
// in the file at path, and false when there is nothing to show.
func (w *Workspace) Hover(path string, p Position) (string, bool) {
	d := w.s.docs[pathURI(path)]
	if d == nil {
		return "", false
	}
	result, err := w.s.hover(d, p)
	h, ok := result.(hoverResult)
	if err != nil || !ok {
		return "", false
	}
	return h.Contents.Value, true
}

// Completion is one thing that may be typed. Kind is a word an editor can
// pick an icon by: keyword, function, method, field, variable, type,
// trait, module, enum, variant, event or text.
type Completion struct {
	Label  string `json:"label"`
	Kind   string `json:"kind"`
	Detail string `json:"detail,omitempty"`
	Doc    string `json:"doc,omitempty"`
}

var completionKinds = map[int]string{
	completionText: "text", completionKeyword: "keyword", completionMethod: "method",
	completionFunction: "function", completionField: "field", completionVariable: "variable",
	completionClass: "type", completionInterface: "trait", completionModule: "module",
	completionEnum: "enum", completionEnumMember: "variant", completionStruct: "type",
	completionEvent: "event",
}

// Complete returns what may be typed at p in the file at path, as the
// language server offers it.
func (w *Workspace) Complete(path string, p Position) []Completion {
	out := []Completion{}
	d := w.s.docs[pathURI(path)]
	if d == nil {
		return out
	}
	result, _ := w.s.completion(d, p)
	items, _ := result.([]completionItem)
	for _, it := range items {
		c := Completion{Label: it.Label, Kind: completionKinds[it.Kind], Detail: it.Detail}
		if it.Documentation != nil {
			c.Doc = it.Documentation.Value
		}
		out = append(out, c)
	}
	return out
}

// Location is a range in one of the workspace's files.
type Location struct {
	Path  string `json:"path"`
	Range Range  `json:"range"`
}

// Definition returns where the name at p in the file at path is declared,
// when that is in one of the workspace's files: the prelude, standard
// modules and built-ins are not.
func (w *Workspace) Definition(path string, p Position) []Location {
	out := []Location{}
	d := w.s.docs[pathURI(path)]
	if d == nil {
		return out
	}
	result, _ := w.s.definition(d, p)
	locs, _ := result.([]location)
	for _, l := range locs {
		if file, ok := uriPath(l.URI); ok && w.s.docs[l.URI] != nil {
			out = append(out, Location{Path: file, Range: l.Range})
		}
	}
	return out
}

// Format returns the file at path in the one canonical style, or an error
// when it has syntax errors.
func (w *Workspace) Format(path string) ([]byte, error) {
	d := w.s.docs[pathURI(path)]
	if d == nil {
		return nil, errors.New("no file " + path)
	}
	if len(d.file.Diagnostics) > 0 {
		return nil, errors.New("fix the syntax errors before formatting")
	}
	return syntax.Format(d.text)
}

// Paths returns the workspace's files, sorted.
func (w *Workspace) Paths() []string {
	var out []string
	for uri := range w.s.docs {
		if p, ok := uriPath(uri); ok {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out
}

// Rel returns path relative to the workspace's root, with slashes.
func (w *Workspace) Rel(path string) string {
	if rel, err := filepath.Rel(w.s.root, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return path
}
