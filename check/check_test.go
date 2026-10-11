package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTourResolves(t *testing.T) {
	prog, diags := Program("../testdata/programs/tour")
	for _, d := range diags {
		t.Error(d)
	}
	if prog.Root == nil || len(prog.Root.Tests) != 3 {
		t.Fatalf("the tour's module should have 3 tests")
	}
}

// TestDiagnostics checks what the checker reports for one-file programs.
// Each case is a program and the messages it must produce, as
// "line:column: message".
func TestDiagnostics(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{"in and for name only an extern fn's parameters", `fn f(in: Int) = 1`,
			[]string{"1:6: in is a keyword, so the body couldn't read a parameter of that name; only an extern fn's parameters may be named in or for"}},
		{"an extern type is a type of its own", `extern type Db
@host("lab.rows")
extern fn rows(db: Db) -> Int uses {ffi}
fn f() -> Int uses {ffi} = rows("not a db")`, []string{"4:33: expected Db, found String"}},
		{"undefined names", `fn f() = g(x)
fn h() = Missing(1)`, []string{"1:10: undefined name g", "1:12: undefined name x", "2:10: undefined name Missing"}},
		{"duplicate declarations", `fn f() = 1
fn f() = 2
type T = | A
type U = | A`, []string{"2:4: f is declared twice in this module", "4:6: A is declared twice in this module"}},
		{"a let can't change", `fn f(n: Int) {
  let a = 1
  a = 2
  n += 1
}`, []string{"3:3: a can't change", "4:3: n can't change"}},
		{"var and mut can change", `fn f(mut n: Int) {
  var a = 1
  a = 2
  n += a
}`, nil},
		{"closures don't capture vars", `fn f() {
  var a = 1
  let g = fn: a + 1
}`, []string{"3:15: a closure can't capture a, a var"}},
		{"a closure changes its own vars, not those outside", `fn f() {
  var a = 1
  let g = fn {
    var b = 0
    b += 1
    a = b
  }
}`, []string{"6:5: a closure can't capture a, a var"}},
		{"waiting calls only in jobs", `entity E(id: Int) {
  state n: Int = 0
  pub fn m() { sleep!(1 min) }
}
test "t" { sleep!(1 min) }
pub fn main() { sleep!(1 min) }`, []string{
			"3:16: an entity method is a transaction and can't wait",
			"5:12: a test can't make waiting calls",
			"6:17: `!` marks a durable step",
		}},
		{"durable steps in jobs need !", `job j() { sleep(1 min) }`, []string{"1:11: in a job, sleep is a durable step: write sleep!(…)"}},
		{"attributes are declared", `@concurency(4)
job a() {}
@memory(64 MiB)
entity E(id: Int) {}
@concurrency(1)
@concurrency(2)
job b() {}`, []string{
			"1:1: @concurency isn't an attribute; did you mean @concurrency?",
			"3:1: @memory goes on a job, not on an entity",
			"6:1: @concurrency is given twice",
		}},
		{"attribute arguments", `@concurrency(0)
job a() {}
@limit(1)
job b() {}
@memory("lots")
job c() {}
@priority(urgent)
job d() {}
@concurrency(1, by: chat)
job e(room: String) {}
@limit(1, every: 1 s)
job f() {}
@fresh(now)
contract C {}`, []string{
			"1:14: @concurrency takes a whole number above zero here, not 0",
			"3:1: @limit needs per:",
			"5:9: @memory takes a size, such as 64 MiB here",
			"7:11: @priority takes interactive, normal, background or batch here",
			"9:21: @concurrency's by: names a parameter of e, and it has no parameter chat",
			"11:11: @limit has no argument every:",
			"13:1: @fresh goes on a command, not on a contract",
		}},
		{"a type variable is declared where it is used", `type Box = { inner: a }
type Opt = | Has(b) | Not
type Pair(x) = (x, y)
entity E(id: Int) {
  state v: c = 1
}
contract C {
  query q(n: d) -> Int
  event e: List(e) on "c.e"
}`, []string{
			"1:21: a is a type variable, and Box doesn't declare it: write type Box(a) = …",
			"2:18: b is a type variable, and Opt doesn't declare it: write type Opt(b) = …",
			"3:20: y is a type variable, and Pair doesn't declare it: write type Pair(x, y) = …",
			"5:12: c is a type variable, and an entity's state is of one type, which it names: write that type",
			"8:14: d is a type variable, and an operation's values cross the wire, so their types are named in full",
			"9:17: e is a type variable, and an event's values cross the wire, so its type is named in full",
		}},
		{"a uses clause lists effects that exist", `entity Order(id: Int) {}
contract Shop {
  command buy(n: Int) -> Int
}
fn a() uses {nonsense} {}
fn b() uses {net Nowhere.send, net Shop.sell, net Shop.buy, net} {}
fn c() uses {entity Ghost, entity Order, clock now} {}
fn d() uses {publish Shop.nothing, consume Nowhere, secret token, entity "x", secret "ok"} {}`, []string{
			"5:14: nonsense isn't an effect: an effect is clock, consume, entity, env, ffi, fs, net, process, publish, random, secret, stdio or wait",
			"6:18: net names a contract and its operation, as net Notify.send, and there is no contract Nowhere here",
			"6:36: contract Shop has no operation sell",
			"7:21: entity names an entity, and there is no entity Ghost here",
			"7:48: clock names nothing: write uses {clock}",
			"8:22: contract Shop has no event nothing",
			"8:44: consume names a contract's event, as consume Bus.pinged, and there is no contract Nowhere here",
			`8:60: secret names a secret as secret(…) is passed it, in quotes: secret "telegram-bot-token"`,
			"8:74: entity names what it touches without quotes, as entity Order",
		}},
		{"attributes name what exists", `pub type Sent = | Sent(Int)
pub type Ping = { id: Int }
contract Svc {
  @key(required)
  @reconcile(nosuch)
  command a(text: String) -> Sent
  @key(required)
  @reconcile(b)
  command c(text: String) -> Sent
  command b(text: String) -> Sent
  @key(required)
  @reconcile(wrong)
  command d(text: String) -> Sent
  query wrong(n: Int) -> Sent
  @key(required)
  @reconcile(e)
  command e(text: String) -> Sent
  @key(nope)
  event pings: Ping on "svc.pings"
}`, []string{
			"5:14: @reconcile names a query of Svc, or the command itself, and Svc has no operation nosuch",
			"8:14: @reconcile names a query, or the command itself, and b is another command",
			"12:14: @reconcile names a query that finds a command's result by its key: query wrong(key: Key) -> Option(…)",
			"18:8: @key names the field that tells pings's events apart, and Ping has no field nope",
		}},
		{"a deadline at a time of day needs its zone", `@deadline(13:40)
job a() {}`, []string{"1:11: @deadline at a time of day needs its zone"}},
		{"main starts a job once, by key", `job j(n: Int) {}
pub fn main() {
  j.start(1)
  j.start(2, key: "two")
}`, []string{"3:3: eye run runs main every time it starts, so this would start j again after each restart"}},
		{"break outside a loop", `fn f() { break }`, []string{"1:10: break outside a loop"}},
		{"state can change in entity methods, the identity can't", `entity E(id: Int) {
  state n: Int = 0
  pub fn m() {
    n += 1
    id = 2
  }
}`, []string{"5:5: id is the entity's identity and can't change"}},
		{"with only in tests", `fn f() { with clock(2026-10-10 12:00 in "UTC") }`, []string{"1:10: with applies a test handler"}},
		{"missing module", `import nowhere`, []string{"1:8: no module nowhere"}},
		{"views start from an entity set", `view v = [1, 2]`, []string{"1:1: a view is a pipeline that starts from an entity set"}},
		{"syntax errors", `fn f( {`, []string{"1:7: expected"}},
		{"return leaves a function, not a lambda", `fn f() {
  let g = fn: return 1
}`, []string{"2:15: return inside a lambda would leave only the lambda"}},
		{"_ goes in a piped call", `fn f() = _`, []string{"1:10: _ stands for the piped value"}},
		{"in: and for: alone need a value", `fn f() = daily(07:40, in:)`, []string{"1:23: in is a keyword, not a variable, so in: needs a value"}},
		{"impl gives methods to a type this module declares", `impl Int {
  fn twice(self) -> Int = self * 2
}`, []string{"1:6: impl for Int, which isn't a type declared in this module"}},
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
					if strings.HasPrefix(g, w) {
						found = true
					}
				}
				if !found {
					t.Errorf("missing %q in:\n%s", w, strings.Join(got, "\n"))
				}
			}
		})
	}
}

func TestImportsAcrossModules(t *testing.T) {
	dir := t.TempDir()
	write := func(path, src string) {
		p := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main.eye", "import lib.shapes\nimport lib.shapes.{Circle}\nlet a = shapes.area(Circle(1.0))\nlet b = shapes.missing\n")
	write("lib/shapes/shapes.eye", "pub type Shape = | Circle(Float)\npub fn area(s: Shape) -> Float = 1.0\n")
	_, diags := Program(dir)
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "module shapes has no missing") {
		t.Errorf("diagnostics: %v", diags)
	}
}

// TestAmbiguousVariants checks that a variant two imported modules declare
// is named with its module where nothing else says which it is.
func TestAmbiguousVariants(t *testing.T) {
	dir := t.TempDir()
	write := func(path, src string) {
		p := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main.eye", "import a\nimport b\nfn f() = Done\n")
	write("a/a.eye", "pub type A = | Done\n")
	write("b/b.eye", "pub type B = | Done\n")
	_, diags := Program(dir)
	if len(diags) != 1 || !strings.HasSuffix(diags[0].Pos.String(), "main.eye:3:10") || !strings.Contains(diags[0].Message, "Done is a variant in more than one imported module; qualify it") {
		t.Errorf("diagnostics: %v", diags)
	}
}

// TestContractCallThroughModule checks that a contract operation called
// through its module, as notify.send is, is a durable step in a job.
func TestContractCallThroughModule(t *testing.T) {
	dir := writeProgram(t, map[string]string{
		"eye.toml": "",
		"mail/mail.eye": `contract Mail {
  command send(to: String) -> Result((), String)
}
`,
		"main.eye": `import mail

job j() { mail.send("me") }
`,
	})
	res := Check(dir, Options{})
	want := "main.eye:3:11: in a job, a call to another service is a durable step: write send!(…)"
	var got []string
	for _, d := range res.Diagnostics {
		got = append(got, d.String())
	}
	if len(got) != 1 || !strings.HasSuffix(got[0], want) {
		t.Errorf("got %q, want one ending %q", got, want)
	}
}
