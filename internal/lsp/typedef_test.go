package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTypeDefinition jumps from a name to the declaration of its type: a
// field, a local, a parameter, a call's result, the innermost declared type
// of List(T) and Option(T), both of a Map's, an entity reference's entity
// and a built-in type.
func TestTypeDefinition(t *testing.T) {
	dir := t.TempDir()
	main := `type UserId = | UserId(String)

type Article = { title: String }

entity Shelf(id: String) {
  state top: Option(Article) = None

  pub fn put(a: Article) { top = Some(a) }
}

fn pick(byUser: Map(UserId, Article), xs: List(Article)) -> Option(Article) = None

fn make() -> Article = { title: "x" }

fn count(n: Int) -> Int = n + 1

fn main() uses {entity Shelf} {
  let shelf = Shelf("a")
  let best = pick({}, [])
  let made = make()
  print("{shelf.id} {best} {made.title} {count(3)}")
}
`
	for name, src := range map[string]string{"eye.toml": "", "main.eye": main} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	uri := pathURI(filepath.Join(dir, "main.eye"))
	lines := strings.Split(main, "\n")
	// at is the position of the first character of the k-th occurrence of
	// what, counted from 1, on the line that holds within.
	at := func(within, what string) (int, int) {
		for i, l := range lines {
			if strings.Contains(l, within) {
				return i, strings.Index(l, within) + strings.Index(within, what)
			}
		}
		t.Fatalf("no line holds %q", within)
		return 0, 0
	}

	s := newSession(t)
	s.open(uri, main)
	ask := func(within, what string) int {
		line, char := at(within, what)
		return s.request("textDocument/typeDefinition", map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"position":     map[string]any{"line": line, "character": char},
		})
	}
	ids := map[string]int{
		"state field":   ask("state top", "top"),
		"record field":  ask("{ title: \"x\" }", "title"),
		"field use":     ask("{made.title}", "title"),
		"local":         ask("{best}", "best"),
		"parameter":     ask("put(a: Article)", "a:"),
		"list":          ask("xs: List", "xs"),
		"map":           ask("byUser: Map", "byUser"),
		"call result":   ask("let made = make()", "make"),
		"entity ref":    ask("{shelf.id}", "shelf"),
		"builtin":       ask("count(n: Int)", "n:"),
		"builtin use":   ask("= n + 1", "n"),
		"nothing there": ask("fn main()", "fn"),
	}
	msgs := s.run()
	get := func(name string) []string {
		var locs []location
		response(t, msgs, ids[name], &locs)
		var out []string
		for _, l := range locs {
			if !strings.HasSuffix(l.URI, "/main.eye") {
				out = append(out, filepath.Base(l.URI))
				continue
			}
			out = append(out, strings.TrimSpace(lines[l.Range.Start.Line]))
		}
		return out
	}
	for name, want := range map[string][]string{
		"state field":  {"type Article = { title: String }"},
		"record field": {"builtin.eye"},
		"field use":    {"builtin.eye"},
		"local":        {"type Article = { title: String }"},
		"parameter":    {"type Article = { title: String }"},
		"list":         {"type Article = { title: String }"},
		"map":          {"type UserId = | UserId(String)", "type Article = { title: String }"},
		"call result":  {"type Article = { title: String }"},
		"entity ref":   {"entity Shelf(id: String) {"},
		"builtin":      {"builtin.eye"},
		"builtin use":  {"builtin.eye"},
	} {
		if got := get(name); strings.Join(got, " | ") != strings.Join(want, " | ") {
			t.Errorf("type definition of %s = %q, want %q", name, got, want)
		}
	}
	if got := get("nothing there"); len(got) != 0 {
		t.Errorf("a keyword has a type definition: %q", got)
	}
}

func TestTypeDefinitionIsAdvertised(t *testing.T) {
	caps, _ := (&server{}).initialize().(map[string]any)["capabilities"].(map[string]any)
	if caps["typeDefinitionProvider"] != true {
		t.Errorf("capabilities = %v", caps)
	}
}
