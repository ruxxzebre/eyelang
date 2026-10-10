package syntax

import "testing"

func TestDocComment(t *testing.T) {
	src := []byte(`# A plain comment, not published.
## Train reminders: when to leave.
##
## Second paragraph.

# Not part of anything.
## Ignored: a blank line separates it.

## Says hello.
##   Keeps indentation past one space.
pub fn hello() = "hi"

# Plain.
fn quiet() = 1
`)
	f := Parse(src)
	if len(f.Diagnostics) > 0 {
		t.Fatal(f.Diagnostics)
	}
	if got, want := ModuleDoc(f), "Train reminders: when to leave.\n\nSecond paragraph."; got != want {
		t.Errorf("ModuleDoc = %q, want %q", got, want)
	}
	decls := f.Root.Children
	if got, want := DocComment(decls[0], src), "Says hello.\n  Keeps indentation past one space."; got != want {
		t.Errorf("DocComment(hello) = %q, want %q", got, want)
	}
	if got := DocComment(decls[1], src); got != "" {
		t.Errorf("DocComment(quiet) = %q, want none", got)
	}
}

func TestModuleDocNeedsBlankLine(t *testing.T) {
	src := []byte("## Documents the function, not the module.\npub fn f() = 1\n")
	f := Parse(src)
	if got := ModuleDoc(f); got != "" {
		t.Errorf("ModuleDoc = %q, want none", got)
	}
	if got := DocComment(f.Root.Children[0], src); got != "Documents the function, not the module." {
		t.Errorf("DocComment = %q", got)
	}
}

func TestSummary(t *testing.T) {
	for _, tc := range []struct{ doc, want string }{
		{"Says hello. Then more.", "Says hello."},
		{"Wraps\nlines. More.", "Wraps lines."},
		{"Calls `x.y. z` first. More.", "Calls `x.y. z` first."},
		{"Version 1.5 is out", "Version 1.5 is out"},
		{"First paragraph\n\nSecond.", "First paragraph"},
		{"", ""},
	} {
		if got := Summary(tc.doc); got != tc.want {
			t.Errorf("Summary(%q) = %q, want %q", tc.doc, got, tc.want)
		}
	}
}
