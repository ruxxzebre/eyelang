package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHalfTypedFile hovers, completes and goes to a definition in a file
// with a declaration half typed: the rest of the file is checked as usual.
func TestHalfTypedFile(t *testing.T) {
	dir := t.TempDir()
	main := `type Point = { x: Int, y: Int }

fn half(n: Int) -> Int {
  let m = n +
}

fn helper() -> Int = 1

fn good(p: Point) -> Int {
  let total = p.x + helper()
  total
}

fn typing(p: Point) -> Int {
  p.
}
`
	for name, src := range map[string]string{"eye.toml": "", "main.eye": main} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	uri := pathURI(filepath.Join(dir, "main.eye"))
	d := newDocument(uri, 1, []byte(main))
	pos := func(within string, delta int) map[string]any {
		p := d.position(strings.Index(main, within) + delta)
		return map[string]any{"line": p.Line, "character": p.Character}
	}
	s := newSession(t)
	s.open(uri, main)
	doc := map[string]any{"uri": uri}
	hover := s.request("textDocument/hover", map[string]any{"textDocument": doc, "position": pos("  total\n", 2)})
	def := s.request("textDocument/definition", map[string]any{"textDocument": doc, "position": pos("helper()", 0)})
	complete := s.request("textDocument/completion", map[string]any{"textDocument": doc, "position": pos("  p.\n", 4)})
	msgs := s.run()

	var h *hoverResult
	response(t, msgs, hover, &h)
	if h == nil || !strings.Contains(h.Contents.Value, "total: Int") {
		t.Errorf("hover on total: %+v", h)
	}
	var locs []location
	response(t, msgs, def, &locs)
	if len(locs) != 1 || locs[0].Range.Start.Line != 6 {
		t.Errorf("definition of helper: %+v, want line 7", locs)
	}
	var items []completionItem
	response(t, msgs, complete, &items)
	labels := map[string]bool{}
	for _, it := range items {
		labels[it.Label] = true
	}
	if !labels["x"] || !labels["y"] {
		t.Errorf("completion after p.: %v, want x and y", labels)
	}
}
