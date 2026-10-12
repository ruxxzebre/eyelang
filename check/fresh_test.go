package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// shelves is a module with a @fresh and a @destructive command, which the
// tests below import.
const shelves = `pub type Shelf = { id: String, version: Version }
pub type Cleared = | Cleared(count: Int)
pub type Burned = | Burned

@port(3004)
contract Shelves {
  query shelf(id: String) -> Shelf

  @fresh
  @key(required)
  @reconcile(clear)
  command clear(id: String) -> Cleared

  @destructive
  command burn(id: String) -> Burned
}
`

// diagnoseWith checks a program whose main module is src, with the modules
// in others beside it, and returns every diagnostic as "file:line:column:
// message", the file relative to the program's directory.
func diagnoseWith(t *testing.T, src string, others map[string]string) []string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{"main.eye": src + "\n"}
	for path, text := range others {
		files[path] = text
	}
	for path, text := range files {
		p := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, diags := Program(dir)
	var got []string
	for _, d := range diags {
		got = append(got, strings.TrimPrefix(d.String(), dir+string(filepath.Separator)))
	}
	return got
}

// TestFreshAndDestructive checks @fresh and @destructive commands (SPECS.md
// §17): a @fresh command takes basedOn: Version and may answer Stale or
// Expired, which a match must handle; a @destructive one takes an Approval.
// Each case lists every diagnostic the program has, exactly.
func TestFreshAndDestructive(t *testing.T) {
	cases := []struct {
		name   string
		src    string
		module string // shelves/shelves.eye, when not the one above
		want   []string
	}{
		{"a caller that handles every outcome checks", `import shelves.{Cleared, Burned}
job tidy(id: String) {
  let shelf = shelves.shelf!(id)
  let n = match shelves.clear!(id, basedOn: shelf.version, key: "tidy-{id}").reconcile!() {
    Cleared(n) => n
    Stale | Expired => 0
  }
  match approval!("Burn {id}?", from: "operator", within: 1 h) {
    Approved(ok) => {
      let _ = shelves.burn!(id, approval: ok)
    }
    Declined | Expired => ()
  }
}`, "", nil},
		{"a @fresh command needs basedOn", `import shelves
job tidy(id: String) {
  let _ = shelves.clear!(id, key: "k")
}`, "", []string{
			"main.eye:3:11: clear needs an argument for basedOn: clear is @fresh, so it takes the Version of the state it was based on, as basedOn: snapshot.version",
		}},
		{"basedOn is a Version, which an integer literal can be", `import shelves
job tidy(id: String) {
  let _ = shelves.clear!(id, basedOn: "v1", key: "k")
  let _ = shelves.clear!(id, basedOn: 3, key: "k")
}`, "", []string{"main.eye:3:39: expected Version, found String"}},
		{"a match on a @fresh command's result handles Stale and Expired", `import shelves.{Cleared}
job tidy(id: String, v: Version) {
  match shelves.clear!(id, basedOn: v, key: "k") {
    Cleared(_) => ()
    Uncertain(_) => ()
  }
  match shelves.clear!(id, basedOn: v, key: "k").reconcile!() {
    Cleared(_) => ()
    Stale => ()
  }
}`, "", []string{
			"main.eye:3:3: this match doesn't handle Stale: add an arm for it, or `_ => …`",
			"main.eye:7:3: this match doesn't handle Expired: add an arm for it, or `_ => …`",
		}},
		{"a @fresh command's result can't pass for its declared outcomes", `import shelves.{Cleared}
job tidy(id: String, v: Version) {
  let c: Cleared = shelves.clear!(id, basedOn: v, key: "k").reconcile!()
}`, "", []string{
			"main.eye:3:20: this command is @fresh, so it may be Stale or Expired: match it with Stale and Expired arms",
		}},
		{"a @destructive command needs an Approval, which only approval! makes", `import shelves
job burnIt(id: String, ok: Approval) {
  let _ = shelves.burn!(id)
  let _ = shelves.burn!(id, approval: "yes")
  let _ = shelves.burn!(id, approval: ok)
}`, "", []string{
			"main.eye:3:11: burn needs an argument for approval: burn is @destructive, so it takes the Approval that approval!(…) gives",
			"main.eye:4:39: expected Approval, found String",
		}},
		{"an impl takes basedOn and the approval, and answers Stale and Expired", `import shelves.{Cleared, Burned}
entity Rack(id: String) {
  state n: Int = 0
  pub fn empty() -> Int { n }
}
impl shelves.Shelves {
  fn shelf(id) = { id, version: Rack(id).read().version }
  fn clear(id, basedOn, key) {
    let rack = Rack(id).read()
    if rack.version == 0 { return Expired }
    if rack.version != basedOn then Stale else Cleared(count: Rack(id).empty())
  }
  fn burn(id, approval) = Burned
}`, "", nil},
		{"an impl declares the parameters @fresh and @destructive add", `import shelves.{Cleared, Burned}
impl shelves.Shelves {
  fn shelf(id) = { id, version: 0 }
  fn clear(id, key) = Stale
  fn burn(id) = Burned
}`, "", []string{
			"main.eye:4:6: clear takes (id, basedOn, key), as contract Shelves declares it",
			"main.eye:5:6: burn takes (id, approval), as contract Shelves declares it",
		}},
		{"only a @fresh command's impl answers Stale", `import shelves.{Cleared, Burned}
impl shelves.Shelves {
  fn shelf(id) = { id, version: 0 }
  fn clear(id, basedOn, key) = Stale
  fn burn(id, approval) = Stale
}`, "", []string{"main.eye:5:27: expected Burned, found Stale, which only a @fresh command answers"}},
		{"@fresh needs a variant result without Stale or Expired, and adds names a command can't declare", `import shelves`, `pub type Done = | Finished | Stale

contract Shelves {
  @fresh
  command clear(id: String, basedOn: Int) -> Done

  @fresh
  command count(id: String) -> Int

  @destructive
  command burn(id: String, approval: String) -> Done
}
`, []string{
			"shelves/shelves.eye:5:29: @fresh adds the parameter basedOn, so clear can't declare one of that name",
			"shelves/shelves.eye:5:46: @fresh adds Stale and Expired to clear's outcomes, and Done already has a variant Stale",
			"shelves/shelves.eye:8:32: @fresh adds Stale and Expired to count's outcomes, so it returns a variant type, and Int isn't one",
			"shelves/shelves.eye:11:28: @destructive adds the parameter approval, so burn can't declare one of that name",
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			module := c.module
			if module == "" {
				module = shelves
			}
			got := diagnoseWith(t, c.src, map[string]string{"shelves/shelves.eye": module})
			if strings.Join(got, "\n") != strings.Join(c.want, "\n") {
				t.Errorf("diagnostics:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(c.want, "\n"))
			}
		})
	}
}
