package check

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func formatSignatures(r *Result) ([]string, bool) {
	sigs, ok := r.Signatures()
	var out []string
	for _, s := range sigs {
		out = append(out, s.Func.Pos.File.Path+":"+s.Func.Name+":"+strings.Join(s.Effects, ","))
	}
	sort.Strings(out)
	return out, ok
}

func TestFormatSession(t *testing.T) {
	dir := writeProgram(t, map[string]string{
		"eye.toml":          "",
		"a/main.eye":        "import shared\npub fn stamp() -> Instant = shared.stamp()\n",
		"b/main.eye":        "import shared\npub fn stamp() -> Instant = shared.stamp()\n",
		"shared/shared.eye": "pub fn stamp() -> Instant = now()\n",
	})
	var session FormatSession
	if session.File("missing") != nil {
		t.Fatal("an unused session returned a parsed file")
	}
	a := session.Check(dir, filepath.Join(dir, "a"))
	sharedPath := filepath.Join(dir, "shared/shared.eye")
	shared := session.File(sharedPath)
	if shared == nil {
		t.Fatal("shared import wasn't parsed")
	}
	b := session.Check(dir, filepath.Join(dir, "b"))
	if session.File(sharedPath) != shared {
		t.Fatal("unchanged imported source was parsed again")
	}
	if a.Program.Prelude == b.Program.Prelude || a.Program.Root.Funcs["stamp"] == b.Program.Root.Funcs["stamp"] {
		t.Fatal("separate checks shared mutable IR")
	}
	if a.Index != nil || b.Index != nil {
		t.Fatal("formatting built an unused name index")
	}
	for i, res := range []*Result{a, b} {
		moduleDir := filepath.Join(dir, []string{"a", "b"}[i])
		indexed := Check(dir, Options{Module: moduleDir})
		got, clean := formatSignatures(res)
		want, wantClean := formatSignatures(indexed)
		if !clean || clean != wantClean || !reflect.DeepEqual(got, want) || fmt.Sprint(res.Diagnostics) != fmt.Sprint(indexed.Diagnostics) {
			t.Fatalf("session differs from Check: signatures %v / %v, diagnostics %v / %v", got, want, res.Diagnostics, indexed.Diagnostics)
		}
	}
	if err := os.WriteFile(sharedPath, []byte("pub fn stamp() -> Instant { print(\"now\"); now() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed := session.Check(dir, filepath.Join(dir, "a"))
	if session.File(sharedPath) == shared {
		t.Fatal("a changed import reused stale syntax")
	}
	sigs, clean := formatSignatures(changed)
	if !clean || len(sigs) != 2 {
		t.Fatalf("changed signatures = %v, clean = %v; diagnostics %v", sigs, clean, changed.Diagnostics)
	}
	for _, sig := range sigs {
		if !strings.HasSuffix(sig, ":clock,stdio") {
			t.Fatalf("changed effects weren't inferred: %v", sigs)
		}
	}
	old, clean := formatSignatures(a)
	if !clean {
		t.Fatal("a later check changed the previous result")
	}
	for _, sig := range old {
		if !strings.HasSuffix(sig, ":clock") {
			t.Fatalf("a later check changed previous effects: %v", old)
		}
	}
}

func TestFormatSessionErrors(t *testing.T) {
	for _, src := range []string{"fn f( {\n", "pub fn f() -> Int = \"wrong\"\n"} {
		dir := writeProgram(t, map[string]string{"main.eye": src})
		var session FormatSession
		res := session.Check(dir, dir)
		want := Check(dir, Options{})
		if len(res.Diagnostics) == 0 || fmt.Sprint(res.Diagnostics) != fmt.Sprint(want.Diagnostics) {
			t.Fatalf("diagnostics = %v, want %v", res.Diagnostics, want.Diagnostics)
		}
		if _, clean := res.Signatures(); clean {
			t.Fatal("a program with errors returned writable effects")
		}
	}
}

func TestFormatSessionUnfinishedLambda(t *testing.T) {
	dir := writeProgram(t, map[string]string{"main.eye": "let f = fn x 1\npub fn good() -> Int = 1\n"})
	var session FormatSession
	res := session.Check(dir, dir)
	if len(res.Diagnostics) == 0 || res.Program.Root.Funcs["good"] == nil {
		t.Fatalf("unfinished lambda lost its diagnostic or the valid declaration: %v", res.Diagnostics)
	}
	if _, clean := res.Signatures(); clean {
		t.Fatal("unfinished source returned writable effects")
	}
}
