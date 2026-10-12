package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixBus is what the programs eye fix's tests rewrite share: an event, an
// entity and a job to start.
const fixBus = `type Closed = { id: Int }

contract Bus {
  event closed: Closed on "bus.closed"
}

entity Counter(id: String) {
  state n: Int = 0

  pub fn add(k: Int) { n += k }
}

job audit(n: Int) {
  log.info("audit {n}")
}
`

func TestFixAddsTheBangOfDurableSteps(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{
			"publish and start in a job",
			`
job close(n: Int) {
  closed.publish({ id: n })
  let r = audit.start(n)
  let _ = r.join!()
}
`, `
job close(n: Int) {
  closed.publish!({ id: n })
  let r = audit.start!(n)
  let _ = r.join!()
}
`,
		},
		{
			"a start whose run is joined at once, and an entity call",
			`
job close(n: Int) {
  Counter("a").add(n)
  match audit.start(n).join!() {
    Completed(_) => log.info("done")
    Failed(f) => log.info("{f}")
  }
}
`, `
job close(n: Int) {
  Counter("a").add!(n)
  match audit.start!(n).join!() {
    Completed(_) => log.info("done")
    Failed(f) => log.info("{f}")
  }
}
`,
		},
		{
			"a waiting function",
			`
fn later(n: Int) {
  sleep!(1 min)
  closed.publish({ id: n })
}

job close(n: Int) { later!(n) }
`, `
fn later(n: Int) {
  sleep!(1 min)
  closed.publish!({ id: n })
}

job close(n: Int) { later!(n) }
`,
		},
		{
			"outside jobs nothing changes",
			`
test "a start and a publish" {
  let _ = audit.start(1)
  closed.publish({ id: 1 })
  Counter("a").add(1)
}
`, "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := program(t, map[string]string{"main.eye": fixBus + c.in})
			path := filepath.Join(dir, "main.eye")
			var stdout, stderr strings.Builder
			if code := run([]string{"fix", dir}, &stdout, &stderr); code != 0 {
				t.Fatalf("eye fix exited %d:\n%s%s", code, stdout.String(), stderr.String())
			}
			got, _ := os.ReadFile(path)
			want := fixBus + c.want
			if c.want == "" {
				want = fixBus + c.in
				if stdout.Len() > 0 {
					t.Errorf("eye fix said it changed something:\n%s", stdout.String())
				}
			}
			if string(got) != want {
				t.Errorf("after eye fix:\n%s\nwant:\n%s", got, want)
			}
			if code := run([]string{"check", dir}, &stdout, &stderr); code != 0 {
				t.Errorf("the fixed program doesn't check:\n%s", stdout.String())
			}
		})
	}
}

func TestFixCheckChangesNothing(t *testing.T) {
	src := fixBus + `
job close(n: Int) { closed.publish({ id: n }) }
`
	dir := program(t, map[string]string{"main.eye": src})
	var stdout, stderr strings.Builder
	if code := run([]string{"fix", "--check", dir}, &stdout, &stderr); code != exitErrors {
		t.Errorf("eye fix --check exited %d, want %d", code, exitErrors)
	}
	if !strings.Contains(stdout.String(), "main.eye:17:21: publish → publish!") {
		t.Errorf("eye fix --check said:\n%s", stdout.String())
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "main.eye")); string(got) != src {
		t.Errorf("eye fix --check changed the file:\n%s", got)
	}
}

func TestFixLeavesOtherErrors(t *testing.T) {
	dir := program(t, map[string]string{"main.eye": fixBus + `
job close(n: Int) {
  closed.publish({ id: n })
  let s: String = n
}
`})
	var stdout, stderr strings.Builder
	if code := run([]string{"fix", dir}, &stdout, &stderr); code != exitErrors {
		t.Errorf("eye fix exited %d, want %d", code, exitErrors)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "main.eye"))
	if !strings.Contains(string(got), "closed.publish!({ id: n })") {
		t.Errorf("the publish wasn't fixed:\n%s", got)
	}
	if !strings.Contains(stderr.String(), "errors eye fix can't fix") {
		t.Errorf("eye fix said:\n%s", stderr.String())
	}
}
