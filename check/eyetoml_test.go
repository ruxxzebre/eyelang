package check

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestModuleRoots checks modules in eye.toml (SPECS.md §13): an import is
// looked for below the program's own directory, then below each directory
// modules lists, in order, and the first that has it wins.
func TestModuleRoots(t *testing.T) {
	dir := writeProgram(t, map[string]string{
		".git/HEAD":                "",
		"lab/home/home.eye":        "pub let who = \"lab\"\n",
		"lab/notify/notify.eye":    "pub let where = \"lab\"\n",
		"vendor/notify/notify.eye": "pub let where = \"vendor\"\n",
		"deploy/eye.toml":          "# the lab's modules\nmodules = [\n  \"../lab\",  # first\n  '../vendor',\n]\n",
		"deploy/main.eye":          "import home\nimport notify\nimport own\n\npub fn main() uses {stdio} = print(\"{home.who} {notify.where} {own.it}\")\n",
		"deploy/own/own.eye":       "pub let it = \"own\"\n",
		"deploy/lost/eye.toml":     "modules = [\"../../lab\"]\n",
		"deploy/lost/main.eye":     "import nowhere\n",
	})
	prog, diags := Program(filepath.Join(dir, "deploy"))
	if len(diags) > 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	found := map[string]string{}
	for _, m := range prog.Modules {
		if m.Dir != "" {
			found[m.Path] = m.Dir
		}
	}
	for path, want := range map[string]string{
		"home":   filepath.Join(dir, "lab", "home"),
		"notify": filepath.Join(dir, "lab", "notify"), // ../lab comes before ../vendor
		"own":    filepath.Join(dir, "deploy", "own"),
	} {
		if found[path] != want {
			t.Errorf("module %s came from %q, want %q", path, found[path], want)
		}
	}

	// An import no directory has lists every directory searched.
	_, diags = Program(filepath.Join(dir, "deploy", "lost"))
	want := "main.eye:1:8: no module nowhere: there is no directory " +
		filepath.Join(dir, "deploy", "lost", "nowhere") + " or " + filepath.Join(dir, "lab", "nowhere")
	if len(diags) != 1 || !strings.HasSuffix(diags[0].String(), want) {
		t.Errorf("diagnostics: %v, want one ending %q", diags, want)
	}
}

// TestModuleRootErrors checks what eye.toml's modules may list: directories
// that exist, written relative to it, inside the repository.
func TestModuleRootErrors(t *testing.T) {
	for _, c := range []struct{ name, toml, want string }{
		{"a directory that doesn't exist", `modules = ["../gone"]`, `1:12: modules lists ../gone, but there is no directory`},
		{"an absolute path", `modules = ["/lab"]`, `1:12: modules lists /lab: a module directory is written relative to eye.toml`},
		{"a directory outside the repository", `modules = ["../../outside"]`, `1:12: modules lists ../../outside, which is outside the repository at`},
		{"a key eye.toml doesn't hold", "name = \"lab\"\n", `1:1: eye.toml has no key name: the one key it holds is modules`},
		{"modules that isn't a list", `modules = "../lab"`, `1:1: eye.toml: modules takes a list of directories, as in modules = ["../.."]`},
		{"a list without its ]", `modules = ["../lab"`, `1:1: eye.toml: the list of modules has no closing ]`},
		{"a directory without quotes", `modules = [lab]`, `1:12: eye.toml: expected a directory in quotes in the list of modules, found lab`},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := writeProgram(t, map[string]string{
				"repo/.git/HEAD":         "",
				"repo/lab/home/home.eye": "pub let who = \"lab\"\n",
				"repo/prog/eye.toml":     c.toml,
				"repo/prog/main.eye":     "pub fn main() = 1\n",
				"outside/x/x.eye":        "pub let x = 1\n",
			})
			_, diags := Program(filepath.Join(dir, "repo", "prog"))
			if len(diags) != 1 || !strings.Contains(diags[0].String(), "eye.toml:"+c.want) {
				t.Errorf("diagnostics: %v, want one with %q", diags, "eye.toml:"+c.want)
			}
		})
	}
}
