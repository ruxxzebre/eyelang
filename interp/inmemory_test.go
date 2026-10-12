package interp

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"eyelang/check"
	"eyelang/ir"
)

// counterContract is a contract module two programs share: one serves it,
// the other calls it.
const counterContract = `@port(8095)
@http(names: snake)
@auth(bearer: "counter-token")
contract Counter {
  @http("POST /bumps")
  @key(required)
  command bump(by: Int) -> Int

  @http("GET /total")
  query total() -> Int
}
`

// TestServiceInMemory runs two programs in one process, as the online
// playground does: the counter serves its contract with no port, and the
// caller's contract calls reach its handler. A command sent twice with one
// key counts once, the bearer token the page makes up is checked, and each
// call the service answers is written down.
func TestServiceInMemory(t *testing.T) {
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
	write("counter/counter.eye", counterContract)
	write("main.eye", `import counter

job bumps() uses {net Counter.bump, net Counter.total, stdio, wait} {
  let a = counter.bump!(2, key: "a")
  let again = counter.bump!(2, key: "a")
  sleep!(1 h)
  let b = counter.bump!(3, key: "b")
  print("{a} {again} {b} {counter.total!()}")
}

pub fn main() uses {start bumps} {
  bumps.start(key: "bumps")
}
`)
	write("server/eye.toml", "modules = [\"..\"]\n")
	write("server/server.eye", `import counter

entity Count(id: String) {
  state total: Int = 0
  state seen: Map(Key, Int) = {}

  pub fn bump(key: Key, by: Int) -> Int {
    match seen[key] {
      Some(after) => after
      None => {
        total += by
        seen = seen |> insert(key, total)
        total
      }
    }
  }
}

impl counter.Counter {
  fn bump(by, key) = Count("c").bump(key, by)

  fn total() = Count("c").total
}
`)
	load := func(root string) *ir.Program {
		prog, diags := check.Program(root)
		for _, d := range diags {
			t.Fatal(d)
		}
		return prog
	}
	start := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	secrets := map[string]string{"counter-token": "made-up"}

	var srvOut, srvErr bytes.Buffer
	svc, err := StartService(load(filepath.Join(dir, "server")), RunOptions{
		Stdout: &srvOut, Stderr: &srvErr, MemoryLimit: 1 << 30, Secrets: secrets,
		Virtual: &VirtualClock{Start: start},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := svc.Handlers()["counter"]; !ok {
		t.Fatalf("handlers = %v; want one for counter", svc.Handlers())
	}

	var out, errs bytes.Buffer
	code := Run(context.Background(), load(dir), RunOptions{
		Stdout: &out, Stderr: &errs, MemoryLimit: 1 << 30, Secrets: secrets,
		Services: svc.Handlers(), Virtual: &VirtualClock{Start: start},
	})
	svc.Finish()
	if code != 0 || out.String() != "2 2 5 5\n" {
		t.Errorf("exit %d, printed %q, want \"2 2 5 5\\n\"; stderr:\n%s", code, out.String(), errs.String())
	}
	calls := strings.Count(srvErr.String(), "counter.Counter POST /bumps → 200 OK")
	if calls != 3 || !strings.Contains(srvErr.String(), "counter.Counter GET /total → 200 OK") {
		t.Errorf("the service's log:\n%s", srvErr.String())
	}

	// A caller without the token is refused, as on a port.
	out.Reset()
	errs.Reset()
	code = Run(context.Background(), load(dir), RunOptions{
		Stdout: &out, Stderr: &errs, MemoryLimit: 1 << 30, Secrets: map[string]string{"counter-token": "wrong"},
		Services: svc.Handlers(), Virtual: &VirtualClock{Start: start},
	})
	if code == 0 || !strings.Contains(errs.String(), "status 401") {
		t.Errorf("a wrong token: exit %d, stderr:\n%s", code, errs.String())
	}
}
