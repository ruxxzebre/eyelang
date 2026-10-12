package lsp

import (
	"os"
	"path/filepath"
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
	if labels["names"] != completionVariable || labels["let"] != completionKeyword {
		t.Errorf("got %v, want the name names and the keyword let", labels)
	}
	if _, ok := labels["na"]; ok {
		t.Errorf("got %v, want no item for the word being typed", labels)
	}
}

func TestNameCompletion(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("eye.toml", "")
	write("shapes/shapes.eye", "## A shape.\npub type Shape = | Square(Int) | Circle(Int)\n\n"+
		"pub opaque type Box = | Hidden(Int)\n\n## How big it is.\npub fn shapeSize(s: Shape) -> Int = 2\n\nfn hiddenHelper() -> Int = 1\n")
	write("mail/mail.eye", "contract Mail {\n  command deliver(to: String) -> Bool\n  event bounced: String on \"mail.bounced\"\n}\n")
	write("app/main.eye", "fn main() {}\n")
	header := "import shapes\n\ntype Color = | Red | Green\n\nimpl Color {\n  fn standard() -> Color = Red\n  fn label(self) -> String = \"c\"\n}\n\n" +
		"type Point = { x: Int, y: Int }\n\n" +
		"entity Counter(id: String) {\n  state count: Int = 0\n  pub fn bump() { count += 1 }\n}\n\n" +
		"fn helper() -> Int = 1\n\n"
	uri := pathURI(filepath.Join(dir, "app", "main.eye"))

	complete := func(src string) map[string]completionItem {
		at := strings.Index(src, "‸")
		src = src[:at] + src[at+len("‸"):]
		s := newSession(t)
		s.open(uri, src)
		d := newDocument(uri, 1, []byte(src))
		p := d.position(at)
		id := s.request("textDocument/completion", map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"position":     map[string]any{"line": p.Line, "character": p.Character},
		})
		msgs := s.run()
		var items []completionItem
		response(t, msgs, id, &items)
		out := map[string]completionItem{}
		for _, it := range items {
			out[it.Label] = it
		}
		return out
	}
	for _, c := range []struct {
		name, src string
		want      map[string]int // label → kind
		not       []string
	}{
		{
			name: "body",
			src:  header + "fn area(s: shapes.Shape, k: Int) -> Int {\n  let n = 1\n  ‸\n  let later = 2\n}\n",
			want: map[string]int{
				"s": completionVariable, "k": completionVariable, "n": completionVariable,
				"helper": completionFunction, "area": completionFunction, "Color": completionEnum, "Red": completionEnumMember,
				"shapes": completionModule, "Some": completionEnumMember, "Option": completionEnum,
				"print": completionFunction, "String": completionStruct, "log": completionModule, "let": completionKeyword,
			},
			not: []string{"later", "hiddenHelper", "shapeSize", "standard"},
		},
		{
			name: "a line that doesn't parse yet",
			src:  header + "fn area(k: Int) -> Int {\n  let n = (‸\n}\n",
			want: map[string]int{"k": completionVariable, "helper": completionFunction, "shapes": completionModule, "if": completionKeyword},
			not:  []string{"n"},
		},
		{
			name: "for and match",
			src:  header + "fn f(o: Option(Int)) {\n  for x in [1] {\n    match o {\n      Some(v) => ‸\n    }\n  }\n}\n",
			want: map[string]int{"x": completionVariable, "v": completionVariable, "o": completionVariable},
		},
		{
			name: "a new arm",
			src:  header + "fn f(o: Option(Int)) -> Int {\n  match o {\n    Some(v) => v\n    ‸\n  }\n}\n",
			want: map[string]int{"o": completionVariable},
			not:  []string{"v"},
		},
		{
			name: "entity",
			src:  header + "entity Tally(key: String) {\n  state total: Int = 0\n  pub fn add() {\n    ‸\n  }\n}\n",
			want: map[string]int{"key": completionVariable, "total": completionVariable},
		},
		{
			name: "module members",
			src:  header + "fn f() {\n  shapes.‸\n}\n",
			want: map[string]int{"shapeSize": completionFunction, "Shape": completionEnum, "Square": completionEnumMember, "Box": completionStruct},
			not:  []string{"hiddenHelper", "Hidden", "let", "print"},
		},
		{
			name: "a contract's operations and events",
			src:  "import mail\n\nfn f() {\n  mail.‸\n}\n",
			want: map[string]int{"deliver": completionFunction, "bounced": completionEvent},
		},
		{
			name: "module member being typed",
			src:  header + "fn f() {\n  shapes.sh‸\n}\n",
			want: map[string]int{"shapeSize": completionFunction},
		},
		{
			name: "record fields",
			src:  header + "fn f(p: Point) -> Int {\n  p.‸\n}\n",
			want: map[string]int{"x": completionField, "y": completionField},
			not:  []string{"let", "print", "p"},
		},
		{
			name: "methods",
			src:  header + "fn f(c: Color) -> String {\n  c.‸\n}\n",
			want: map[string]int{"label": completionMethod},
			not:  []string{"standard", "Red"},
		},
		{
			name: "entity members",
			src:  header + "fn f(c: Ref(Counter)) {\n  c.‸\n}\n",
			want: map[string]int{"id": completionField, "count": completionField, "bump": completionMethod},
		},
		{
			name: "a call's result",
			src:  header + "fn origin() -> Point = { x: 0, y: 0 }\n\nfn f() -> Int {\n  origin().‸\n}\n",
			want: map[string]int{"x": completionField, "y": completionField},
		},
		{
			name: "built-in module",
			src:  header + "fn f() {\n  log.‸\n}\n",
			want: map[string]int{"warn": completionFunction},
			not:  []string{"print"},
		},
		{
			name: "type",
			src:  header + "fn f() {\n  Color.‸\n}\n",
			want: map[string]int{"standard": completionFunction, "Red": completionEnumMember, "Green": completionEnumMember},
			not:  []string{"label"},
		},
		{
			name: "inside parentheses that aren't closed yet",
			src:  header + "fn f(p: Point) {\n  print(p.‸\n}\n",
			want: map[string]int{"x": completionField, "y": completionField},
		},
		{
			name: "inside brackets and parentheses, a name being typed",
			src:  header + "fn f(p: Point) {\n  print([1, p.y‸\n}\n",
			want: map[string]int{"x": completionField, "y": completionField},
		},
		{
			name: "a module inside parentheses",
			src:  header + "fn f() {\n  print(shapes.‸\n}\n",
			want: map[string]int{"shapeSize": completionFunction},
		},
		{
			name: "a field of a field",
			src:  header + "type Line = { from: Point, to: Point }\n\nfn f(l: Line) -> Int {\n  l.from.‸\n}\n",
			want: map[string]int{"x": completionField, "y": completionField},
			not:  []string{"from", "to"},
		},
		{
			name: "a built-in type: the functions that take it",
			src:  header + "fn twice(n: Int) -> Int = n * 2\n\nfn f(n: Int) {\n  n.‸\n}\n",
			want: map[string]int{"twice": completionFunction, "div": completionFunction},
			not:  []string{"let", "print", "n", "helper", "first", "shapeSize"},
		},
		{
			name: "functions that take the value, after its fields and methods",
			src: header + "fn norm(p: Point) -> Int = p.x\n\nfn label(c: Color) -> Int = 1\n\n" +
				"fn f(p: Point, c: Color) {\n  p.‸\n}\n",
			want: map[string]int{"x": completionField, "norm": completionFunction},
			not:  []string{"label", "helper", "twice"},
		},
		{
			name: "a method hides a function of its name",
			src:  header + "fn label(c: Color) -> Int = 1\n\nfn f(c: Color) {\n  c.‸\n}\n",
			want: map[string]int{"label": completionMethod},
		},
		{
			name: "generic functions and the collection functions",
			src: header + "fn largest(xs: List(a)) -> Option(a) where a: Ord = xs |> first\n\n" +
				"fn f(xs: List(Int)) {\n  xs.‸\n}\n",
			want: map[string]int{"largest": completionFunction, "first": completionFunction, "map": completionFunction},
			not:  []string{"insert", "keys", "wrappingAdd", "helper"},
		},
		{
			name: "not after a dot in a comment",
			src:  header + "# see shapes.‸\n",
			not:  []string{"shapeSize", "let"},
		},
	} {
		got := complete(c.src)
		for label, kind := range c.want {
			if it, ok := got[label]; !ok || it.Kind != kind {
				t.Errorf("%s: %s: got %+v, want kind %d", c.name, label, it, kind)
			}
		}
		for _, label := range c.not {
			if _, ok := got[label]; ok {
				t.Errorf("%s: got %s, want it left out", c.name, label)
			}
		}
	}

	got := complete(header + "fn f() {\n  shapes.‸\n}\n")
	if it := got["shapeSize"]; it.Detail != "pub fn shapeSize(s: Shape) -> Int" || it.Documentation == nil || it.Documentation.Value != "How big it is." {
		t.Errorf("shapeSize: got %+v %+v, want its signature and doc comment", it, it.Documentation)
	}
}
