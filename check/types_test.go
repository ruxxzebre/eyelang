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
		{"a schedule's time", `@on(daily("07:40", in: "Europe/Dublin"))
job morning(day: Date) { log.info("{day}") }`,
			[]string{"1:11: expected Time, found String"}},
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
		{"outside calls in jobs are durable steps", `fn stamp() -> Instant = now()
fn double(n: Int) -> Int = n * 2
job j() {
  let a = now()
  let b = env.get("HOME")
  let c = stamp()
  let d = double!(2)
  let e = [1] |> size!()
  let f = now!()
  let g = stamp!()
}`, []string{
			"4:11: in a job, now uses clock, so calling it is a durable step: write now!(…)",
			"5:11: in a job, env.get uses env, so calling it is a durable step: write env.get!(…)",
			"6:11: in a job, stamp uses clock, so calling it is a durable step: write stamp!(…)",
			"7:11: double neither waits nor reaches outside the program, so it isn't a durable step: write double(…) without !",
			"8:18: size neither waits nor reaches outside the program, so it isn't a durable step: write size(…) without !",
		}},
		{"a value is a dyn Trait only if its type implements the trait", `trait Shape {
  fn area(self) -> Int
}
type Square = { side: Int }
type Circle = { r: Int }
impl Shape for Square {
  fn area(self) = self.side * self.side
}
fn area(s: dyn Shape) -> Int = s.area()
fn circle() -> Circle = { r: 1 }
fn one() -> Int = area(circle())
fn many() -> List(dyn Shape) = [circle()]
fn back(s: dyn Shape) -> Square = s
fn more(s: dyn Shape) -> Int = s.perimeter()
fn shown(s: dyn Shape) -> String = "{s}"
job j(s: Square) {
  let d: dyn Shape = s
  sleep!(1 min)
  log.info("{d.area()}")
}`, []string{
			"11:24: expected dyn Shape, found Circle: Circle doesn't implement Shape",
			"12:33: expected dyn Shape, found Circle: Circle doesn't implement Shape",
			"13:35: expected Square, found dyn Shape",
			"14:32: dyn Shape has no method perimeter: a dyn value has only the methods of its trait",
			"15:38: showing a value in a string needs Show, and dyn Shape has no Show: a dyn Shape has only the methods of Shape",
			"19:14: d is a dyn Shape, which can't be saved, and it is still needed after a waiting call: get it again after the `!`",
		}},
		{"dyn takes a trait whose methods aren't generic", `trait Mapper {
  fn apply(self, f: fn(Int) -> b) -> b
}
fn a(m: dyn Mapper) -> Int = 0
fn b(x: dyn Add) -> Int = 0`, []string{
			"4:9: dyn Mapper can't be used: its method apply has type variables of its own (b), which a call through dyn can't choose",
			"5:9: Add takes 2 type arguments, and this dyn gives 0: dyn Add(rhs, out)",
		}},
		{"a built-in type has only the fields it declares", `fn a(d: Duration) -> Int = d.minutes
fn b(r: Range(Int)) -> Int = r.start + r.size`, []string{
			"1:28: Duration has no field minutes",
			"2:40: Range(Int) has no field size",
		}},
		{"dyn takes a trait", `type Square = { side: Int }
fn b(s: dyn Square) -> Int = 0`, []string{
			"2:13: dyn takes a trait, and Square is a type: write Square, or dyn and a trait it implements",
		}},
		{"a task in a job only computes", `fn stamp() -> Instant = now()
job j() -> List(Int) {
  let a = [1, 2] |> parallelMap(fn n {
    sleep!(1 min)
    n
  })
  let b = [1, 2] |> parallelMap(fn n: stamp())
  [1, 2] |> parallelMap(fn n: n * 2)
}`, []string{
			"4:5: a task in a job can only compute, since its steps couldn't be recorded: start a child run for this step instead",
			"7:39: a task in a job can only compute, and stamp uses clock: start a child run for this step instead",
		}},
		{"a task group in a job only computes, in its spawned tasks too", `job j() -> Int {
  taskGroup(fn group {
    group.spawn(fn { sleep!(1 min) })
    1 + 1
  })
}`, []string{
			"3:22: a task in a job can only compute, since its steps couldn't be recorded: start a child run for this step instead",
		}},
		{"a public function states its effects", `pub fn stamp() -> Instant = now()
pub fn stated() -> Instant uses {clock} = now()
pub fn partly() -> Instant uses {stdio} = now()`, []string{
			"1:1: stamp uses clock, which its signature doesn't list: add uses {clock}",
			"3:1: partly uses clock, which its uses clause doesn't list",
		}},
		{"effects pass through private functions", `import svc
fn send(t: String) = svc.raw(t)
pub fn f() = send("x")`, []string{"3:1: f uses net Svc.raw, which its signature doesn't list"}},
		{"a type variable or an untyped parameter has only what its uses need", `fn show(x) -> String = "{x}"
fn leak() -> String = show(secret("a"))
fn bigger(a, b) -> Bool = a > b
fn f(g: fn() -> Int) -> Bool = bigger(g, g)
fn call(x: a) = x()
fn index(x: a) = x[0]`, []string{
			"1:26: showing a value in a string needs Show, and Secret(String) has no Show",
			"3:27: comparing with > needs Ord, and fn() -> Int has no Ord",
			"5:17: x's type is a, which may be any type, so it can't be called",
			"6:18: a can't be indexed",
		}},
		{"effects don't escape through traits or private uses clauses", `trait Clock {
  fn read(self) -> Instant
}
trait Stamp {
  fn stamp(self) -> Instant uses {clock}
}
type Real = { n: Int }
impl Clock for Real {
  fn read(self) -> Instant = now()
}
impl Stamp for Real {
  fn stamp(self) -> Instant = now()
}
fn quiet() -> Instant uses {} = now()
pub fn later() -> Instant = quiet()
pub fn through(s: dyn Stamp) -> Instant = s.stamp()`, []string{
			"9:3: read uses clock, which Clock.read doesn't list, and a call through the trait counts only what the trait says: add uses {clock} to the trait's method",
			"14:1: quiet uses clock, which its uses clause doesn't list",
			"16:1: through uses clock, which its signature doesn't list",
		}},
		{"effects pass through functions named as values", `import svc
fn stamp(n: Int) -> Instant = now()
pub fn stamps(xs: List(Int)) -> List(Instant) = xs |> map(stamp)
pub fn later() -> Instant {
  let g = stamp
  g(1)
}
pub fn sends(xs: List(String)) = xs |> map(svc.raw)
job j(xs: List(Int)) {
  let a = xs |> map(stamp)
  let b = xs |> map(fn x: stamp!(x))
}`, []string{
			"3:1: stamps uses clock, which its signature doesn't list",
			"4:1: later uses clock, which its signature doesn't list",
			"8:1: sends uses net Svc.raw, which its signature doesn't list",
			"10:21: in a job, stamp uses clock, so each call of it is a durable step, and named as a value its calls aren't recorded: pass a lambda that calls it with !",
		}},
		{"a job's arguments are saved", `job j(token: Secret(String)) { sleep!(1 min) }`,
			[]string{"1:1: j's argument token is a Secret, which can't be saved"}},
		{"what can't be saved isn't held across a !", `job j() {
  let f = fn x: x + 1
  sleep!(1 min)
  log.info("{f(1)}")
}`, []string{"4:14: f is a function, which can't be saved, and it is still needed after a waiting call"}},
		{"/ and % take a NonZero divisor", `fn a(t: Int, n: Int) -> Int = t / n
fn b(t: Int) -> Int = t % 0
fn c(t: Int, n: Int) -> Int = match nonzero(n) {
  Some(d) => t / d
  None => 0
}
fn d(t: Int) -> Int = t / 3 + t % -2
fn e(mut t: Int, n: Int) { t /= n }
fn half(x, y) = x / y
fn f() = half(1, 0)
fn g() -> Float = 1.0 / 0.0
fn h() -> NonZero(Int) = 7`, []string{
			"1:35: dividing by Int fails when it is 0, so / and % take a NonZero(Int): match nonzero(n), which is Some(n) unless it is 0, or divide by a literal other than 0",
			"2:27: dividing by 0 has no answer",
			"8:33: dividing by Int fails when it is 0",
			"9:21: dividing by 0 has no answer",
		}},
		{"a minus sign and the built-in operators need types that have them", `type Money = { cents: Int }
fn a() = -true
fn b() = -Some(1)
fn c() = "a" - "b"
fn d(m: Money) = -m
fn e(at: Instant) = at * 2 min
fn f(x: t) -> t = -x
fn g(x: t) -> t where t: Neg(t) = -x
fn h(s: Set(Int)) = s + s
fn k() = abs("x")
fn ok(n: Int, d: Duration, f: Float) -> (Int, Duration, Float) = (-n, abs(-d), -f + -1.0)`, []string{
			"2:10: -Bool isn't defined: only numbers and durations have a minus sign",
			"3:10: -Option(Int) isn't defined: Option(Int) doesn't implement Neg",
			"4:10: String - String isn't defined",
			"5:18: Money has no -: implement Neg for it, as `impl Neg(Money) for Money`",
			"6:21: Instant * … isn't defined: a time moves by adding or subtracting a Duration",
			"7:19: -t needs t: Neg, and t may be any type: require it, as `where t: Neg(t)`",
			"9:21: Set(Int) + Set(Int) isn't defined",
			"10:10: abs needs t: Neg(String), and t is String here, which has no Neg",
		}},
		{"return gives the function's type, and with a handler", `fn a(n: Int) {
  if n > 1 { return 5 }
  print("x")
}
fn b(n: Int) {
  if n > 1 { return }
}
test "t" {
  with 5
  expect(true)
}`, []string{
			"2:21: expected (), found Int",
			"9:8: expected Handler, found Int",
		}},
		{"foreign code owns an extern type's values", `extern type Db
extern fn open(path: String) -> Db
extern fn close(d: Db)
job j() {
  let d = open("x")
  sleep!(1 min)
  close(d)
}
job k(d: Db) {}
fn show(d: Db) = print(d)
fn same(a: Db, b: Db) -> Bool = a == b`, []string{
			"7:9: d is a Db, which can't be saved, and it is still needed after a waiting call",
			"9:1: k's argument d is a Db, which can't be saved",
			"10:24: print needs Show, and Db has no Show: Db is an extern type, whose values foreign code owns, so it has none of Eq, Ord, Show, Hash, Save and Schema",
			"11:33: comparing with == needs Eq, and Db has no Eq",
		}},
		{"a channel isn't held across a !", `job j() {
  let (send, receive) = channel(capacity: 1)
  sleep!(1 min)
  send.put(1)
}`, []string{"4:3: send is a Sender, which can't be saved, and it is still needed after a waiting call"}},
		{"a channel has a capacity, and its ends their methods", `fn f() {
  let (send, receive) = channel()
  send.put(1)
  send.take()
  let n: Option(String) = receive.take()
}`, []string{
			"2:25: channel needs a capacity: how many values it holds before a put waits, as channel(capacity: 64)",
			"4:3: Sender(Int) has no method take; its methods are put and close",
			"5:27: expected Option(String), found Option(Int)",
		}},
		{"operators need types that have them", `type Money = { cents: Int }
fn f(a: Money, b: Money) = a + b`, []string{"2:28: Money has no +: implement Add for it"}},
		{"a trait's methods take self", `trait Named {
  fn name() -> String
}`, []string{"2:6: Named.name takes self first"}},
		{"an impl gives every method its trait needs, and no other", `trait Named {
  fn name(self) -> String
}
type Player = { nick: String }
type Robot = | Robot
impl Named for Player {}
impl Named for Robot {
  fn name(self) = "robot"
  fn serial(self) -> Int = 1
}`, []string{
			"6:1: impl Named for Player doesn't implement name, which Named needs",
			"9:3: serial isn't a method of Named",
		}},
		{"only a trait can be implemented", `type Player = { nick: String }
type Robot = | Robot
impl Player for Robot {}`, []string{"3:6: Player isn't a trait, so it can't be implemented"}},
		{"an impl lives with its trait or its type", `impl Show for Int {
  fn show(self) = "int"
}`, []string{"1:1: impl Show for Int belongs in the module that declares Show or the one that declares Int"}},
		{"a type implements a trait once", `type Money = { cents: Int }
impl Add(Money, Money) for Money { fn add(self, other: Money) -> Money = self }
impl Add(Money, Money) for Money { fn add(self, other: Money) -> Money = other }`,
			[]string{"3:1: Money implements Add(Money, Money) twice"}},
		{"a trait's type arguments", `type Money = { cents: Int }
impl Add(Money) for Money { fn add(self, other: Money) -> Money = self }`,
			[]string{"2:6: Add takes 2 type arguments, and this impl gives 1"}},
		{"an impl's method has its trait's types", `trait Sized {
  fn size(self, unit: String) -> Int
}
type Box = { n: Int }
impl Sized for Box {
  fn size(self, unit: Int) -> Int = self.n
}`, []string{"6:3: size's parameter unit is Int here, and Sized says String"}},
		{"a call meets its callee's bounds", `fn largest(xs: List(a)) -> Option(a) where a: Ord = xs |> first
fn f(s: Secret(String)) = largest([s])`,
			[]string{"2:27: largest needs a: Ord, and a is Secret(String) here, which has no Ord: a Secret can't be shown, saved or compared"}},
		{"a type variable has only the traits it requires", `fn biggest(a: t, b: t) -> t = if a > b then a else b
fn sum2(a: t, b: t) -> t = a + b
fn named(a: t) -> String = a.name()`, []string{
			"1:34: comparing with > needs Ord, and t may be any type: require it, as `where t: Ord`",
			"2:28: t + … needs t: Add",
			"3:28: t may be any type, so it has no method name",
		}},
		{"where names a type variable and a trait", `type Shape = | Dot
fn f(x: a) -> a where b: Ord, a: Shape = x`, []string{
			"2:23: where names b, which isn't a type variable of f",
			"2:34: Shape isn't a trait, so it can't be required of a",
		}},
		{"a trait isn't a type inside a type", `trait Named {
  fn name(self) -> String
}
fn f(xs: List(Named)) -> Int = 0`, []string{"4:1: f's parameter xs has Named inside its type, and Named is a trait"}},
		{"a value shown in a string has Show", `fn f(token: Secret(String)) = "token: {token}"`,
			[]string{"1:40: showing a value in a string needs Show, and Secret(String) has no Show"}},
		{"what a job saves has Save, field by field", `type Login = { user: String, password: Secret(String) }
job j(login: Login) { sleep!(1 min) }`,
			[]string{"2:1: j's argument login is a Login (its field password is a Secret)"}},
		{"a model is asked for a type with a Schema", `import ai
job j() {
  let r: Result(Secret(String), ai.AiError) = ai.generate!(model: "m", prompt: "p")
}`, []string{"3:47: ai.generate needs Schema, and Secret(String) has no Schema"}},
		{"a mut parameter is passed with mut, and only it", `type Counter = { hits: Int }
fn bump(mut c: Counter) { c.hits += 1 }
fn look(c: Counter) -> Int = c.hits
fn f() {
  var c: Counter = { hits: 0 }
  bump(c)
  look(mut c)
}`, []string{
			"6:8: bump changes its argument c, so the call says so: write mut c",
			"7:12: look doesn't change its argument c, so it takes it without mut",
		}},
		{"no value is changed through two names", `type Counter = { hits: Int }
fn swap(mut a: Counter, mut b: Counter) {}
fn add(mut a: Counter, n: Int) {}
fn f() {
  var c: Counter = { hits: 0 }
  swap(mut c, mut c)
  add(mut c, c.hits)
}`, []string{
			"6:19: c is passed as mut twice in one call",
			"7:14: c is passed as mut and read in the same call",
		}},
		{"a mut self method needs a var", `type Counter = { hits: Int }
impl Counter {
  fn reset(mut self) { self.hits = 0 }
}
fn f() {
  let c: Counter = { hits: 0 }
  c.reset()
}`, []string{"7:3: reset takes mut self, so it changes c, which can't change: declare it with var"}},
		{"a closure can't capture what can change", `fn f(mut n: Int) {
  let g = fn: n + 1
}`, []string{"2:15: a closure can't capture n, a mut parameter"}},
		{"? needs a Result or an Option, and a function that returns one", `fn f(o: Option(Int)) -> Result(Int, String) = Ok(o?)
fn g(n: Int) -> Option(Int) = Some(n?)`, []string{
			"1:50: ? on an Option needs a function that returns an Option, and this one returns Result(Int, String)",
			"2:36: ? passes on the error of a Result or the None of an Option; this is Int",
		}},
		{"nothing converts implicitly", `fn f() = 1 + 2.0`, []string{"1:10: expected Int, found Float"}},
		{"a mut argument is a var", `fn bump(mut n: Int) { n += 1 }
fn f() {
  let n = 1
  bump(mut n)
}`, []string{"4:12: n can't change: it is a let or a parameter"}},
		{"a served contract serves every operation, as declared", `import svc
impl svc.Svc {
  fn send(text, key) = Sent(1)
  fn raw(words, key) = Done
  fn extra() = 1
}`, []string{
			"2:1: a program that serves Svc serves every operation; this one doesn't serve sent",
			"4:6: raw takes (text), as contract Svc declares it",
			"5:6: contract Svc has no operation extra",
		}},
		{"a trigger names a contract's event", `import svc
@on(event(svc.missing))
job j(p: svc.Ping) {}`, []string{"2:11: event(…) takes a contract's event, such as telegram.updates"}},
		{"a function type is checked like any type", `fn apply(f: fn(Int) -> Int, x: Int) -> Int = f(x)
fn g() -> String = apply(fn n: n + 1, 2)
fn h() -> Int = apply(fn n: "x", 2)
fn k() -> Int = apply(fn (a, b): a, 2)
fn run(f: fn()) = f()
fn twice(f: fn(a) -> a, x: a) -> a = f(f(x))
fn ok() -> Int = twice(fn n: n * 2, 3) + apply(fn n: n, 1)
fn nothing() -> Int = run(fn: 1)`, []string{
			"2:20: expected String, found Int",
			"3:29: expected Int, found String",
			"4:23: expected Int, found (a, b)",
			"8:23: expected Int, found ()",
		}},
		{"{} is an empty map, set or record", `type User = { name: String, age: Int }
fn f() {
  let a: Int = {}
  let u: User = {}
  let s: Set(Int) = {} |> add("x")
  let m: Map(String, Int) = {} |> insert("a", 1)
  let n: Set(Int) = {}
}`, []string{
			"3:16: expected Int, found {}",
			"4:17: this User has no name, age: every field needs a value",
			"5:27: expected Set(Int), found Set(String)",
		}},
		{"every type has only its own fields and methods", `type U = { n: String }
fn m(u: U, p: (Int, Int), d: Duration) {
  let a = "abc".nosuchmethod()
  let c = p.first
  let e = 5[0]
  let f = d.nosuch()
  let g = u.n.size
  for x in 3 { print(x) }
}
fn untyped(u) = u.name
fn inline(r: { a: Int }) -> Int = r.a
contract Local {
  query ping(n: Int) -> Int
}
fn contract(s: Local) = 1`, []string{
			"3:11: String has no method nosuchmethod",
			"4:11: (Int, Int) has no field first: take a tuple apart with a pattern",
			"5:11: Int can't be indexed",
			"6:11: Duration has no method nosuch",
			"7:11: String has no field size",
			"8:12: Int isn't a collection that for can walk",
			"10:17: the type of u isn't known here, so its field name can't be found: write its type where it is declared",
			"11:14: a record type is written in a type declaration",
			"15:16: Local isn't a type",
		}},
		{"a trait's default method sees self as the trait", `trait Named {
  fn name(self) -> String
  fn greet(self) -> String = "hi {self.name()}"
  fn bad(self) -> Int = self.name()
  fn worse(self) -> String = self.nick()
}
trait Box(a) {
  fn get(self) -> a
  fn twice(self) -> (a, a) = (self.get(), self.get())
  fn wrong(self) -> a = 1
}`, []string{
			"4:25: expected Int, found String",
			"5:30: self is any type that implements Named, so it has only the methods of Named, and nick isn't one",
			"10:25: expected a, found Int",
		}},
		{"a collection function takes the collections and arguments it declares", `type User = { name: String }
fn f(u: User, xs: List(Int), m: Map(String, Int), o: Option(Int)) {
  let a = u.size()
  let b = xs |> map(fn x: x, 3)
  let c = xs |> filter(keep: fn x: x > 1, extra: 2)
  let d = xs |> keys
  let g = o |> first
  let h = xs |> map
  let k = 5 |> size
  let n = xs |> count
}
fn untyped(xs) = xs |> size`, []string{
			"3:11: size takes a List, a Set, a Map, a Range, a String, an Iter or a Bytes, and this is User",
			"4:30: too many arguments to map: it takes 2",
			"5:50: filter has no parameter extra",
			"6:11: keys takes a Map, and this is List(Int)",
			"7:11: first takes a List, a Set, a Map, a Range, a String, an Iter or a Bytes, and this is Option(Int)",
			"8:11: map needs an argument for f",
			"9:11: size takes a List, a Set, a Map, a Range, a String, an Iter or a Bytes, and this is Int",
			"12:18: the type of xs isn't known here, so size can't tell what kind of collection it is: write its type where it is declared",
		}},
		{"a built-in that takes a function is checked against its declaration", `job j() {
  let ok = poll!(every: 1 min, within: 2 h, fn { 1 })
  let late = poll!(every: 1 min, inside: 2 h, within: 3 h, check: fn { true })
}
fn t() -> Bool = timeout(1 min, fn { 1 }) == Some("x")
fn m(r: Result(Int, String)) -> Result(Int, Int) = r.mapErr(fn e: e)
fn b() -> (Int, Int) = both(fn { 1 }, fn { "two" })`, []string{
			"2:50: expected Bool, found Int",
			"3:42: poll has no parameter inside",
			"5:46: expected Option(Int), found Option(String)",
			"6:52: expected Result(Int, Int), found Result(Int, String)",
			"7:24: expected (Int, Int), found (Int, String)",
		}},
		{"a function has none of the derived traits", `fn show(f: fn() -> Int) = print(f)
fn same(f: fn() -> Int, g: fn() -> Int) -> Bool = f == g
job j(f: fn() -> Int) {}
job k() {
  let f = fn: 1
  sleep!(1 min)
  log.info("{f()}")
}`, []string{
			"1:33: print needs Show, and fn() -> Int has no Show: a function has none of Eq, Ord, Show, Hash, Save and Schema",
			"2:51: comparing with == needs Eq, and fn() -> Int has no Eq",
			"3:1: j's argument f is a function, which can't be saved",
			"7:14: f is a function, which can't be saved, and it is still needed after a waiting call",
		}},
		{"well-typed code has no diagnostics", `type Shape = | Circle(Float) | Dot
type User = { name: String, age: Int }
fn area(s: Shape) -> Float = match s {
  Circle(r) => r * r
  Dot => 0.0
}
fn largest(xs: List(a)) -> Option(a) where a: Ord {
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
		"../testdata/programs/tour", "../testdata/programs/schedules", "../testdata/programs/limits", "../testdata/programs/traits",
		"../testdata/programs/tasks",
		"../testdata/lab",
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
