package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"eyelang/ir"
)

// contract is a module the type tests import as `svc`.
const contract = `pub type Sent = | Sent(Int)
pub type Raw = | Done
pub type Ping = { id: Int, text: String }

contract Svc {
  @key(required)
  @reconcile(sent)
  command send(text: String) -> Sent

  command raw(text: String) -> Raw

  query sent(key: Key) -> Option(Sent)

  event pings: Ping on "svc.pings"
}
`

// diagnose checks a program whose main module is src, with the svc module
// beside it, and returns its diagnostics as "line:column: message".
func diagnose(t *testing.T, src string) []string {
	t.Helper()
	dir := t.TempDir()
	for path, text := range map[string]string{"main.eye": src + "\n", "svc/svc.eye": contract} {
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
		got = append(got, strings.TrimPrefix(d.String(), filepath.Join(dir, "main.eye")+":"))
	}
	return got
}

// TestTypeDiagnostics checks one program per kind of type, match or effect
// error: each must be reported at its place, and nothing else.
func TestTypeDiagnostics(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{"a value of the wrong type", `fn f() -> Int = "seven"`,
			[]string{"1:17: expected Int, found String"}},
		{"an argument of the wrong type", `fn f(n: Int) = n
fn g() = f("three")`,
			[]string{"2:12: expected Int, found String"}},
		{"arguments: too many, missing, unknown", `fn f(n: Int) = n
fn g() {
  f(1, 2)
  f()
  f(m: 1)
}`, []string{
			"3:8: too many arguments to f: it takes 1",
			"4:3: f needs an argument for n",
			"5:3: f needs an argument for n",
			"5:8: f has no parameter m",
		}},
		{"a record literal takes the type expected", `type User = { name: String, age: Int }
fn f() {
  let a: User = { name: "Ann" }
  let b: User = { name: "Bo", age: 3, height: 2 }
  let c = { name: "Cy", age: 5 }
}`, []string{
			"3:17: this User has no age: every field needs a value",
			"4:47: User has no field height",
			"5:11: which record type is this?",
		}},
		{"fields and methods", `type User = { name: String }
fn f(u: User) {
  let n = u.nick
  u.grow()
}`, []string{"3:11: User has no field nick", "4:3: User has no method grow"}},
		{"a type variable stands for any type", `fn f(x: a) -> Int = x`,
			[]string{"1:21: expected Int, found a"}},
		{"? wraps an error only in a variant that holds it", `type ParseError = | NotANumber(String)
type CountError = | Usage | BadNumber(ParseError)
type Other = | Other
fn parse(s: String) -> Result(Int, ParseError) = Err(NotANumber(s))
fn wraps(s: String) -> Result(Int, CountError) = Ok(parse(s)?)
fn wrong(s: String) -> Result(Int, Other) = Ok(parse(s)?)`,
			[]string{"6:48: ? can't pass on a ParseError from a function whose error is Other"}},
		{"a match covers every variant", `type Shape = | Circle(Float) | Square(Float) | Dot
fn area(s: Shape) -> Float = match s {
  Circle(r) => r * r
  Square(a) => a * a
}`, []string{"2:30: this match doesn't handle Dot"}},
		{"a match covers every length of a list", `fn f(xs: List(Int)) -> String = match xs {
  [] => "none"
  [one] => "one"
}`, []string{"1:33: this match doesn't handle [_, _, ..]"}},
		{"a match is exhaustive with nested patterns", `fn f(o: Option(Bool)) -> Int = match o {
  Some(true) => 1
  None => 0
}`, []string{"1:32: this match doesn't handle Some(false)"}},
		{"a let without else matches every value", `fn f(o: Option(Int)) -> Int {
  let Some(v) = o
  v
}`, []string{"2:3: this pattern doesn't match None: add `else { … }`"}},
		{"a pattern names a variant of the matched type", `type A = | One | Two
type B = | Three
fn f(a: A) -> Int = match a {
  One => 1
  Three => 3
  _ => 0
}`, []string{"3:21: Three isn't a variant of A"}},
		{"a command may come back Uncertain", `import svc
job j(text: String) {
  match svc.send!(text, key: "k-{text}") {
    Sent(_) => ()
  }
}`, []string{"3:3: this match doesn't handle Uncertain(_)"}},
		{"an Uncertain result can't pass for an outcome", `import svc
fn f(text: String) -> svc.Sent = svc.send!(text, key: "k")`,
			[]string{"2:34: this command's reply may have been lost"}},
		{"reconcile needs a @reconcile query", `import svc
job j(text: String) {
  let r = svc.raw!(text).reconcile!()
}`, []string{"3:11: this command names no @reconcile query"}},
		{"events carry their type", `import svc
job j() {
  let p = next!(svc.pings, where: fn p: p.missing == 1, within: 1 min)
  svc.pings.publish!({ id: "x", text: "t" })
}`, []string{"3:41: Ping has no field missing", "4:28: expected Int, found String"}},
		{"strings become keys and zones where those are expected", `fn keyed(k: Key) = k
fn f() -> Zone {
  let k = "abc-{1}"
  keyed(k)
  let s: String = "plain"
  keyed(s)
  "Europe/Nowhere"
}`, []string{
			"6:9: expected Key, found String",
			"7:3: unknown time zone \"Europe/Nowhere\"",
		}},
		{"a time's zone is checked", `fn f() = 07:40 in "Europe/Dublinn"`,
			[]string{"1:10: unknown time zone \"Europe/Dublinn\""}},
		{"entity calls in jobs are durable steps", `entity Counter(id: String) {
  state n: Int = 0
  pub fn bump() { n += 1 }
}
job j(id: String) {
  Counter(id).bump()
  let s = Counter(id).read()
}`, []string{
			"6:3: in a job, an entity call is a durable step: write bump!(…)",
			"7:11: in a job, an entity call is a durable step: write read!(…)",
		}},
		{"a waiting function is called with !", `fn helper() { sleep!(1 min) }
job j() { helper() }`, []string{"2:11: helper waits, so calling it is a durable step: write helper!(…)"}},
		{"a public function states its effects", `pub fn stamp() -> Instant = now()
pub fn stated() -> Instant uses {clock} = now()
pub fn partly() -> Instant uses {stdio} = now()`, []string{
			"1:1: stamp uses clock, which its signature doesn't list: add uses {clock}",
			"3:1: partly uses clock, which its uses clause doesn't list",
		}},
		{"effects pass through private functions", `import svc
fn send(t: String) = svc.raw(t)
pub fn f() = send("x")`, []string{"3:1: f uses net Svc.raw, which its signature doesn't list"}},
		{"a job's arguments are saved", `job j(token: Secret(String)) { sleep!(1 min) }`,
			[]string{"1:1: j's argument token is a Secret, which can't be saved"}},
		{"what can't be saved isn't held across a !", `job j() {
  let f = fn x: x + 1
  sleep!(1 min)
  log.info("{f(1)}")
}`, []string{"4:14: f is a function, which can't be saved, and it is still needed after a waiting call"}},
		{"operators need types that have them", `type Money = { cents: Int }
fn f(a: Money, b: Money) = a + b`, []string{"2:28: Money has no +: implement Add for it"}},
		{"well-typed code has no diagnostics", `type Shape = | Circle(Float) | Dot
type User = { name: String, age: Int }
fn area(s: Shape) -> Float = match s {
  Circle(r) => r * r
  Dot => 0.0
}
fn largest(xs: List(a)) -> Option(a) {
  var best = xs |> first?
  for x in xs {
    if x > best { best = x }
  }
  Some(best)
}
fn older(u: User) -> User = { u with age: u.age + 1 }
fn names(us: List(User)) -> List(String) = us |> map(fn u: u.name) |> filter(fn n: n != "")
fn total(m: Map(String, Int)) -> Int = m |> entries |> map(fn (_, v): v) |> sum
fn later(at: Instant) -> Instant = at + 20 min
test "t" {
  let u: User = { name: "Ann", age: 3 }
  expect(older(u).age == 4)
  expect(largest([3, 1, 2]) == Some(3))
  expect({ name: "Ann", age: 4 } == older(u))
}`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := diagnose(t, c.src)
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
			if len(got) > len(c.want) && len(c.want) > 0 {
				t.Errorf("more diagnostics than expected:\n%s", strings.Join(got, "\n"))
			}
		})
	}
}

// TestPrograms checks the repository's programs: every one type-checks.
func TestPrograms(t *testing.T) {
	for _, dir := range []string{
		"../testdata/programs/core", "../testdata/programs/jobs", "../testdata/programs/events",
		"../testdata/programs/tour", "../testdata/lab",
	} {
		_, diags := Program(dir)
		for _, d := range diags {
			t.Errorf("%s: %s", dir, d)
		}
	}
}

// TestModelCallExpectedType checks that a model call records the type its
// reply is decoded into: in the lab's digest, `let briefing: Briefing =
// ai.generate!(…).mapErr(WriteFailed)?` is a Result(Briefing, AiError).
func TestModelCallExpectedType(t *testing.T) {
	prog, diags := Program("../testdata/lab")
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	var digest *ir.Module
	for _, m := range prog.Modules {
		if m.Path == "digest" {
			digest = m
		}
	}
	if digest == nil {
		t.Fatal("no digest module")
	}
	var found *ir.TypeExpr
	walk(digest.Jobs["write"].Func.Body, false, func(e ir.Expr, _ bool) bool {
		if call, ok := e.(*ir.Call); ok {
			if r, ok := call.Func.(*ir.Ref); ok {
				if f, ok := r.Decl.(*ir.Func); ok && f.Name == "generate" {
					found = call.Expected
				}
			}
		}
		return true
	})
	if found == nil || found.Name != "Result" || len(found.Args) != 2 || found.Args[0].Name != "Briefing" ||
		found.Args[0].Decl == nil || found.Args[1].Name != "AiError" {
		t.Fatalf("ai.generate's expected type = %+v, want Result(Briefing, AiError)", found)
	}
}
