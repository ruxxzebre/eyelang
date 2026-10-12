package check

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// relDiagnostics checks the program in dir and gives its diagnostics with
// paths relative to dir.
func relDiagnostics(dir string) []string {
	_, diags := Program(dir)
	var got []string
	for _, d := range diags {
		got = append(got, strings.TrimPrefix(d.String(), dir+string(filepath.Separator)))
	}
	return got
}

// notifyModule is a contract that digest may call and trains may not.
const notifyModule = `pub type UserId = | UserId(String)
pub type Delivery = | Accepted | Refused(reason: String)

@port(3001)
@allow(digest)
contract Notify {
  @key(required)
  command send(to: UserId, text: String) -> Delivery
  query identity(user: UserId) -> Option(String)
}

fn own() -> Option(String) = identity(UserId("me"))
`

// TestPortsAndCallers checks @port and @allow on contracts (SPECS.md §17):
// a module the contract's @allow doesn't list may not call its operations,
// each name in @allow is a module, @allow(nobody) lets no module call it,
// and a contract that is served, or guarded by @allow, declares its port.
func TestPortsAndCallers(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"a caller not in @allow", map[string]string{
			"notify/notify.eye": notifyModule,
			"digest/digest.eye": `import notify
fn who() -> Option(String) = notify.identity(notify.UserId("a"))
`,
			"trains/trains.eye": `import notify
fn who() -> Option(String) = notify.identity(notify.UserId("a"))
fn later() = notify.identity
test "a test may call it" {
  expect(notify.identity(notify.UserId("a")) == None)
}
`,
			"main.eye": `import digest
import trains
import notify
fn main() {
  let _ = notify.identity(notify.UserId("a"))
}
`,
		}, []string{
			"main.eye:5:18: the program's own module may not call identity, an operation of Notify: its @allow lists only digest; call it from a module the list can name",
			"trains/trains.eye:2:37: module trains may not call identity, an operation of Notify: its @allow lists only digest; add trains there to call it from here",
			"trains/trains.eye:3:21: module trains may not call identity",
		}},
		{"@allow names modules", map[string]string{
			"notify/notify.eye": strings.Replace(notifyModule, "@allow(digest)", "@allow(digest, trainz)", 1),
			"quiet/quiet.eye": `@port(3002)
@allow(nobody, digest)
contract Quiet {
  query ping() -> Int
}
`,
			"digest/digest.eye": "import notify\nimport quiet\nfn who() = notify.identity(notify.UserId(\"a\"))\n",
			"main.eye":          "import digest\n",
		}, []string{
			"notify/notify.eye:5:16: @allow names the modules that may call Notify, and the program has no module trainz",
			"quiet/quiet.eye:2:8: @allow(nobody) lets no module call Quiet, so it takes no other names",
		}},
		{"@allow(nobody) lets only the server call it", map[string]string{
			"hook/hook.eye": `@port(3003)
@allow(nobody)
contract Hook {
  query ping() -> Int
}
`,
			"server/server.eye": `import hook
impl hook.Hook {
  fn ping() = 1
}
fn self() -> Int = hook.ping()
`,
			"caller/caller.eye": "import hook\nfn call() -> Int = hook.ping()\n",
			"main.eye":          "import server\nimport caller\n",
		}, []string{
			"caller/caller.eye:2:25: module caller may not call ping, an operation of Hook: @allow(nobody) lets no module of the program call it",
		}},
		{"a served or guarded contract has a port", map[string]string{
			"hook/hook.eye": `@allow(server)
contract Hook {
  query ping() -> Int
}
`,
			"far/far.eye": `@port(70000)
contract Far {
  query ping() -> Int
}
`,
			"outside/outside.eye": `contract Outside {
  query ping() -> Int
}
`,
			"server/server.eye": `import hook
import outside
impl hook.Hook {
  fn ping() = 1
}
fn call() -> Int = outside.ping()
`,
			"main.eye": "import server\nimport far\nimport outside\nfn main() {\n  let _ = outside.ping()\n}\n",
		}, []string{
			"far/far.eye:1:7: @port takes a port from 1 to 65535, not 70000",
			"hook/hook.eye:1:1: @allow needs @port on Hook: the modules it lists are let through to that port, and a contract without one is a service outside the network, which any module may call",
			"server/server.eye:3:6: this program serves Hook, so Hook needs @port(…), the port it serves it on, such as @port(3001)",
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := relDiagnostics(writeProgram(t, c.files))
			for _, w := range c.want {
				found := false
				for _, g := range got {
					found = found || strings.HasPrefix(g, w)
				}
				if !found {
					t.Errorf("missing %q in:\n%s", w, strings.Join(got, "\n"))
				}
			}
			if len(got) != len(c.want) {
				t.Errorf("want %d diagnostics, got:\n%s", len(c.want), strings.Join(got, "\n"))
			}
		})
	}
}

// TestLabAllowNeedsApproving checks the lab with Notify's @allow as it is,
// and with approving taken out of it: then approving's call of notify.send
// is an error, at the call.
func TestLabAllowNeedsApproving(t *testing.T) {
	if got := relDiagnostics("../testdata/lab"); len(got) > 0 {
		t.Fatalf("the lab doesn't check:\n%s", strings.Join(got, "\n"))
	}
	dir := t.TempDir()
	err := filepath.WalkDir("../testdata/lab", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("../testdata/lab", path)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dir, rel), 0o755)
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, rel), src, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	notify := filepath.Join(dir, "notify", "notify.eye")
	src, err := os.ReadFile(notify)
	if err != nil {
		t.Fatal(err)
	}
	const allow = "@allow(digest, trains, access, approving)"
	if !strings.Contains(string(src), allow) {
		t.Fatalf("notify.eye has no %s", allow)
	}
	edited := strings.Replace(string(src), allow, "@allow(digest, trains, access)", 1)
	if err := os.WriteFile(notify, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	got := relDiagnostics(dir)
	want := "approving/approving.eye:27:22: module approving may not call ask, an operation of Notify: its @allow lists only digest, trains and access; add approving there to call it from here"
	if len(got) != 1 || got[0] != want {
		t.Errorf("without approving in Notify's @allow, the lab gives:\n%s\nwant:\n%s", strings.Join(got, "\n"), want)
	}
}
