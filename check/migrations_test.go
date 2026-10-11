package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMigrationDiagnostics checks the rules for entity versions: every
// version from the oldest migration to the current one has a migration, a
// migration goes to a version that exists, the one to the current version
// gives the declared state, and a migration is pure.
func TestMigrationDiagnostics(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{"a new version needs a migration from the old one", `@version(2)
entity P(id: Int) {
  state name: String = ""
}`, []string{"1:1: P is at @version(2), so it needs migrate from 1"}},
		{"no gaps", `@version(4)
entity P(id: Int) {
  state name: String = ""
  migrate from 1 (old) { { b: old.a } }
  migrate from 3 (old) { { name: old.c } }
}`, []string{"1:1: P migrates from 1 but not from 2, so rows saved at version 2 have no way to version 4"}},
		{"migrations go to versions that exist", `@version(2)
entity P(id: Int) {
  state name: String = ""
  migrate from 1 (old) { { name: old.nick } }
  migrate from 1 (old) { { name: old.nick } }
  migrate from 2 (old) { { name: old.nick } }
  migrate from 0 (old) { { name: old.nick } }
}`, []string{
			"5:3: P has two migrations from version 1",
			"6:3: migrate from 2 needs P at a later version",
			"7:3: versions start at 1",
		}},
		{"the last migration gives the declared state", `@version(2)
entity P(id: Int) {
  state name: String
  state age: Int = 0
  migrate from 1 (old) { { nmae: old.nick } }
}`, []string{"5:34: P state has no field nmae", "5:26: this P state has no name: every field needs a value"}},
		{"older migrations give the next version's shape, read by name", `@version(3)
entity P(id: Int) {
  state name: String
  migrate from 1 (old) { { nick: old.nickname, extra: 1 } }
  migrate from 2 (old) { { name: old.nick } }
}`, nil},
		{"a migration reads what the one before gives", `@version(3)
entity P(id: Int) {
  state name: String
  migrate from 1 (old) { { nick: old.nickname } }
  migrate from 2 (old) { { name: old.nik } }
}`, []string{"5:34: the row has no field nik: the migration before this one gives nick"}},
		{"a migration is pure", `@version(2)
entity P(id: Int) {
  state name: String
  migrate from 1 (old) { { name: "{old.nick} at {now()}" } }
}`, []string{"4:3: a migration is a pure function of the stored row, and migrate from 1 uses clock"}},
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
				got = append(got, strings.TrimPrefix(d.String(), filepath.Join(dir, "main.eye")+":"))
			}
			if len(c.want) == 0 && len(got) > 0 {
				t.Errorf("unexpected diagnostics:\n%s", strings.Join(got, "\n"))
			}
			for _, w := range c.want {
				found := false
				for _, g := range got {
					found = found || strings.HasPrefix(g, w)
				}
				if !found {
					t.Errorf("missing %q in:\n%s", w, strings.Join(got, "\n"))
				}
			}
		})
	}
}
