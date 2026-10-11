package lsp

import (
	"strings"
	"testing"
)

func TestKeywordsAt(t *testing.T) {
	file := strings.Join(itemStarts[scopeFile], " ")
	block := strings.Join(itemStarts[scopeBlock], " ")
	expr := strings.Join(expressionStarts, " ")
	pub := strings.Join(afterPub[scopeFile], " ")
	for _, c := range []struct{ src, want string }{
		// The ‸ is the cursor.
		{"‸", file},
		{"p‸", file},
		{"pub‸", file},
		{"pub ‸", pub},
		{"pub e‸", pub},
		{"pub opaque ‸", "type"},
		{"extern ‸", "fn type"},
		{"pub extern t‸", "fn type"},
		{"fn ‸", ""},
		{"@memory(10 MiB)\n‸", file},
		{"@memory(10 MiB) pub ‸", pub},
		{"@test\n‸", file},
		{"@mem‸", ""},
		{"@memory(‸)", ""},
		{"fn f() {}\n‸", file},
		{"type T =\n  | A\n  | B\n‸", file},
		{"type T =\n  ‸", ""},
		{"# a comment ‸", ""},
		{`let s = "pub ‸"`, ""},

		{"entity E() {\n  ‸\n}", "state pub fn migrate"},
		{"entity E() {\n  pub ‸\n}", "fn"},
		{"entity E() {\n  migrate ‸\n}", "from"},
		{"entity E() {\n  state n: Int = 0\n  ‸\n}", "state pub fn migrate"},
		{"entity E() {\n  fn f() {\n    ‸\n  }\n}", block},
		{"trait T {\n  ‸\n}", "fn"},
		{"impl T {\n  ‸\n}", "fn"},
		{"contract C {\n  ‸\n}", "command query event"},
		{"type R = {\n  ‸\n}", ""},

		{"fn f() {\n  ‸\n}", block},
		{"fn f() {\n  let x = 1\n  ‸\n}", block},
		{"fn f() {\n  let ‸\n}", ""},
		{"fn f() {\n  let x = ‸\n}", expr},
		{"fn f() {\n  g(‸)\n}", expr},
		{"fn f() {\n  if x {\n    ‸\n  }\n}", block},
		{"fn f() {\n  if x { 1 } ‸\n}", "else " + expr},
		{`fn f() = "{‸}"`, expr},
		{`test "t" {` + "\n  ‸\n}", block},
		{"fn f() {\n  1\n}\n‸", file},
	} {
		at := strings.Index(c.src, "‸")
		src := c.src[:at] + c.src[at+len("‸"):]
		words, names := keywordsAt([]byte(src), at)
		if got := strings.Join(words, " "); got != c.want {
			t.Errorf("%q:\ngot  %q\nwant %q", c.src, got, c.want)
		}
		// Names fit wherever an expression can start.
		if want := strings.Contains(c.want, "true"); names != want {
			t.Errorf("%q: names %v, want %v", c.src, names, want)
		}
	}
}

func TestCompletionRequest(t *testing.T) {
	s := newSession(t)
	s.open("file:///f.eye", "pub ")
	id := s.request("textDocument/completion", map[string]any{
		"textDocument": map[string]any{"uri": "file:///f.eye"},
		"position":     map[string]any{"line": 0, "character": 4},
	})
	msgs := s.run()

	var items []completionItem
	response(t, msgs, id, &items)
	var labels []string
	for _, it := range items {
		labels = append(labels, it.Label)
		if it.Kind != completionKeyword || it.Documentation == nil || it.Documentation.Value == "" {
			t.Errorf("item %+v: want a keyword with its summary", it)
		}
	}
	if !strings.Contains(strings.Join(labels, " "), "extern") {
		t.Errorf("after pub: got %v, want extern among them", labels)
	}
}

func TestCompletionOffersNamesInBodies(t *testing.T) {
	s := newSession(t)
	s.open("file:///f.eye", "fn f(names: List(String)) {\n  na\n}\n")
	id := s.request("textDocument/completion", map[string]any{
		"textDocument": map[string]any{"uri": "file:///f.eye"},
		"position":     map[string]any{"line": 1, "character": 4},
	})
	msgs := s.run()

	var items []completionItem
	response(t, msgs, id, &items)
	labels := map[string]int{}
	for _, it := range items {
		labels[it.Label] = it.Kind
	}
	if labels["names"] != completionText || labels["let"] != completionKeyword {
		t.Errorf("got %v, want the name names and the keyword let", labels)
	}
	if _, ok := labels["na"]; ok {
		t.Errorf("got %v, want no item for the word being typed", labels)
	}
}
