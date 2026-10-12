package netpolicy

import (
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"eyelang/check"
	"eyelang/ir"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

// TestLabPolicy checks the lab's policy against testdata/lab.yaml: the
// notifier's pods, which serve Notify, accept TCP on 3001 from digest,
// trains, access and approving, and from nothing else. go test
// ./internal/netpolicy -update rewrites the file.
func TestLabPolicy(t *testing.T) {
	prog := program(t, "../../testdata/lab")
	policies := Policies(prog)
	if len(policies) != 1 {
		t.Fatalf("the lab has one contract with @port, Notify; got %d policies", len(policies))
	}
	p := policies[0]
	if p.Contract.Name != "Notify" || p.Pods != "notifier" || !p.Served || p.Port != 3001 {
		t.Errorf("got a policy for %s selecting %q (served %v) on port %d; want Notify, served by notifier on 3001", p.Contract.Name, p.Pods, p.Served, p.Port)
	}
	if want := []string{"access", "approving", "digest", "trains"}; !slices.Equal(p.From, want) {
		t.Errorf("Notify lets in %v; want %v", p.From, want)
	}
	var b strings.Builder
	if err := Write(&b, prog, Options{Source: "testdata/lab", Namespace: "homelab"}); err != nil {
		t.Fatal(err)
	}
	golden(t, "testdata/lab.yaml", b.String())
}

// TestDeployedDigestPolicy checks the digest deployed alone: nothing in it
// serves Notify, so the policy selects the pods of Notify's own module, and
// lets in only the digest. Its name differs from the lab's, so both policies
// can be applied and add up.
func TestDeployedDigestPolicy(t *testing.T) {
	var b strings.Builder
	if err := Write(&b, program(t, "../../testdata/lab/deploy/digest"), Options{Source: "testdata/lab/deploy/digest"}); err != nil {
		t.Fatal(err)
	}
	golden(t, "testdata/digest.yaml", b.String())
}

// notifyModule is a contract that digest may call and trains may not.
const notifyModule = `pub type UserId = | UserId(String)

@port(3001)
@allow(digest)
contract Notify {
  query identity(user: UserId) -> Option(String)
}
`

// TestRefusedCallHasNoRule checks that a call the checker refuses, from a
// module Notify's @allow doesn't list, lets nothing in.
func TestRefusedCallHasNoRule(t *testing.T) {
	dir := writeProgram(t, map[string]string{
		"notify/notify.eye": notifyModule,
		"digest/digest.eye": "import notify\nfn who() = notify.identity(notify.UserId(\"a\"))\n",
		"trains/trains.eye": "import notify\nfn who() = notify.identity(notify.UserId(\"a\"))\n",
		"main.eye":          "import digest\nimport trains\n",
	})
	prog, diags := check.Program(dir)
	if len(diags) != 1 || !strings.Contains(diags[0].String(), "module trains may not call identity") {
		t.Fatalf("want the checker to refuse trains' call, got:\n%v", diags)
	}
	p := only(t, Policies(prog))
	if !slices.Equal(p.From, []string{"digest"}) {
		t.Errorf("Notify lets in %v; want only digest, since trains' call is refused", p.From)
	}
}

// TestPolicies checks who is let in, and which pods are selected, in small
// programs.
func TestPolicies(t *testing.T) {
	cases := []struct {
		name   string
		files  map[string]string
		pods   string
		served bool
		from   []string
	}{
		{"tests and a fake's calls don't count", map[string]string{
			"notify/notify.eye": strings.Replace(notifyModule, "@allow(digest)", "", 1),
			"digest/digest.eye": `import notify
fn seen(sent: Handler) -> Int = sent.calls(notify.identity) |> size
test "a test calls it" {
  expect(notify.identity(notify.UserId("a")) == None)
}
`,
			"main.eye": "import digest\n",
		}, "notify", false, nil},
		{"a served contract: callers but not its server", map[string]string{
			"notify/notify.eye": strings.Replace(notifyModule, "@allow(digest)", "@allow(digest, trains)", 1),
			"server/server.eye": `import notify
impl notify.Notify {
  fn identity(user) = None
}
fn mine() = notify.identity(notify.UserId("a"))
`,
			"digest/digest.eye": "import notify\nfn who() = notify.identity(notify.UserId(\"a\"))\n",
			"trains/trains.eye": "import notify\nfn later() = notify.identity\n",
			"main.eye":          "import digest\nimport trains\nimport server\n",
		}, "server", true, []string{"digest", "trains"}},
		{"@allow(nobody) lets in no module", map[string]string{
			"notify/notify.eye": strings.Replace(notifyModule, "@allow(digest)", "@allow(nobody)", 1),
			"server/server.eye": "import notify\nimpl notify.Notify {\n  fn identity(user) = None\n}\n",
			"main.eye":          "import server\n",
		}, "server", true, nil},
		{"the program's own module goes by the directory's name", map[string]string{
			"notify/notify.eye": strings.Replace(notifyModule, "@allow(digest)", "", 1),
			"main.eye":          "import notify\nfn main() {\n  let _ = notify.identity(notify.UserId(\"a\"))\n}\n",
		}, "notify", false, []string{"shop"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := only(t, Policies(program(t, writeProgram(t, c.files))))
			if p.Pods != c.pods || p.Served != c.served || !slices.Equal(p.From, c.from) {
				t.Errorf("got pods %q, served %v, from %v; want %q, %v, %v", p.Pods, p.Served, p.From, c.pods, c.served, c.from)
			}
		})
	}
}

// TestNoPortNoPolicy checks that a contract without @port, a service outside
// the network, gets no policy, and that the output says so.
func TestNoPortNoPolicy(t *testing.T) {
	prog := program(t, writeProgram(t, map[string]string{
		"feeds/feeds.eye": "contract Feeds {\n  query count() -> Int\n}\n",
		"main.eye":        "import feeds\nfn main() {\n  let _ = feeds.count()\n}\n",
	}))
	var b strings.Builder
	if err := Write(&b, prog, Options{Source: "shop"}); err != nil {
		t.Fatal(err)
	}
	if got := b.String(); strings.Contains(got, "NetworkPolicy\n") || !strings.Contains(got, "no contract with @port, so there is no policy") {
		t.Errorf("a program without @port gives:\n%s", got)
	}
}

// TestNamespace checks that a namespace that Kubernetes would refuse is an
// error, and that without one the policies have none.
func TestNamespace(t *testing.T) {
	prog := program(t, "../../testdata/lab")
	var b strings.Builder
	if err := Write(&b, prog, Options{Source: "testdata/lab", Namespace: "Home_Lab"}); err == nil || !strings.Contains(err.Error(), `"Home_Lab" isn't a Kubernetes namespace name`) {
		t.Errorf("Home_Lab: got %v", err)
	}
	b.Reset()
	if err := Write(&b, prog, Options{Source: "testdata/lab"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "namespace:") {
		t.Errorf("without a namespace, the policy names one:\n%s", b.String())
	}
}

func program(t *testing.T, dir string) *ir.Program {
	t.Helper()
	prog, diags := check.Program(dir)
	for _, d := range diags {
		t.Error(d)
	}
	if t.Failed() {
		t.FailNow()
	}
	return prog
}

func only(t *testing.T, policies []Policy) Policy {
	t.Helper()
	if len(policies) != 1 {
		t.Fatalf("want one policy, got %d", len(policies))
	}
	return policies[0]
}

// writeProgram writes files into a directory named shop, the program's name.
func writeProgram(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "shop")
	for name, src := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func golden(t *testing.T, path, got string) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v; run go test ./internal/netpolicy -update to write it", err)
	}
	if got != string(want) {
		t.Errorf("the policy differs from %s (go test ./internal/netpolicy -update rewrites it):\ngot:\n%s\nwant:\n%s", path, got, want)
	}
}
