package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRegexModule checks how programs use the standard module regex
// (SPECS.md §5): a Regex comes only from regex.parse, its functions are pure
// and its methods are the type's own, so a list's find or a string's split
// is still the built-in.
func TestRegexModule(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string // "line:column: message prefix", then the span's text after " @ "
	}{
		{"parse, then the methods, in a pure public function", `import regex.{Regex, RegexError, Match}
pub fn words(text: String) -> Result(List(String), RegexError) {
  let re: Regex = regex.parse("\\w+")?
  let first: Option(Match) = re.find(text)
  let all = re.findAll(text) |> map(fn m: m.text)
  let ok: Bool = re.matches(text) && first != None
  let parts: List(String) = re.split(text) |> filter(fn p: p != "")
  let fixed: String = re.replace(text, "<$0>") + re.replaceFirst(text, "$$")
  Ok(all + parts + [fixed, regex.escape("a.b"), "{re}", "{ok}"])
}`, nil},
		{"the built-ins of the same names are still the built-ins", `import regex
fn f(xs: List(Int), s: String) -> Int {
  let re = regex.parse(",")
  let parts = s.split(",")
  (xs.find(fn x: x > 1) ?? 0) + parts.size() + (if re |> isOk then 1 else 0)
}`, nil},
		{"a Regex is built only by parse", `import regex.{Regex}
fn f() -> Regex = { pattern: "(" }`, []string{"2:19: Regex is opaque outside module regex, so its fields can't be used here"}},
		{"a pattern's error can't be dropped", `import regex
fn f() {
  regex.parse("a")
}`, []string{"3:3: this drops a Result(Regex, RegexError)"}},
		{"without the import there is no regex", `fn f() = regex.parse("a")`,
			[]string{"1:10: undefined name regex"}},
		{"a Regex is a value a job can keep across a !", `import regex
job count(text: String) -> Int {
  let Ok(re) = regex.parse("a+") else { return 0 }
  sleep!(1 min)
  re.findAll(text) |> size
}`, nil},
		{"a RegexError is matched like any variant", `import regex.{RegexError}
fn reason(e: RegexError) -> String = match e {
  BadPattern(_, reason) => reason
}`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "main.eye"), []byte(c.src+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			_, diags := Program(dir)
			var got []string
			for _, d := range diags {
				got = append(got, strings.TrimPrefix(d.String(), filepath.Join(dir, "main.eye")+":")+" @ "+d.Pos.Text())
			}
			if len(got) != len(c.want) {
				t.Errorf("got %d diagnostics, want %d:\n%s", len(got), len(c.want), strings.Join(got, "\n"))
			}
			for _, w := range c.want {
				msg, span, hasSpan := strings.Cut(w, " @ ")
				found := false
				for _, g := range got {
					gmsg, gspan, _ := strings.Cut(g, " @ ")
					found = found || strings.HasPrefix(gmsg, msg) && (!hasSpan || gspan == span)
				}
				if !found {
					t.Errorf("missing %q in:\n%s", w, strings.Join(got, "\n"))
				}
			}
		})
	}
}
