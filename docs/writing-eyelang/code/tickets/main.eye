# Writing Eyelang: every example on the page, as one program that checks and
# passes its tests. The first half is the core language; the second is a
# small ticket desk that escalates tickets nobody answered.

import fs
import notify.{UserId}

# region syntax
type Shape = Circle(radius: Float) | Rect(w: Float, h: Float)
type Priority = Low | Normal | High     # variants without fields
type User = { name: String, age: Int }  # a record, nominal

let dublin: Zone = "Europe/Dublin"  # module-level let, computed once

fn area(s: Shape) -> Float = match s {  # `=` and one expression
  Circle(r) => 3.14 * r * r
  Rect(w, h) => w * h
}

fn adults(users: List(User)) -> String {  # a block: its last expression is the value
  var count = 0                           # var: local and mutable
  for u in users {
    if u.age >= 18 then count += 1
  }
  let names = users |> filter(fn u: u.age >= 18) |> map(fn u: u.name) |> join(", ")
  "{count} adults: {names}"  # interpolation needs Show
}

fn older(u: User) -> User = { u with age: u.age + 1 }

fn firstName(users: List(User)) -> String {
  let Some(u) = users[0] else { return "nobody" }  # the else block must leave
  u.name
}

fn label(p: Priority) -> String = match p {
  High => "urgent"
  Low | Normal => "routine"
}
# endregion syntax

# region literals
let delay: Duration = 3 days + 4 h
let lunch = 12:30                       # a time of day
let launch = 2026-10-12                 # a Date
let opens = 2026-10-12 09:00 in dublin  # an Instant
let budget = 64 MiB                     # a Size
# endregion literals

# region syntax-test
test "a rectangle's area is its width times its height" {
  expect(area(Rect(w: 2.0, h: 3.0)) == 6.0)
  expect(adults([{ name: "Ann", age: 30 }, { name: "Bo", age: 9 }]) == "1 adults: Ann")
  expect(older({ name: "Bo", age: 9 }).age == 10 && firstName([]) == "nobody")
  expect(opens + delay > opens && label(High) == "urgent")
}
# endregion syntax-test

# region errors
type ConfigError =
  | Unreadable(fs.FsError)
  | BadPort(String)
  | Missing(String)

fn port(path: String) -> Result(Int, ConfigError) uses {fs} {
  let text = fs.readText(path)?  # FsError: one variant holds it, so ? wraps it
  if text |> trim == "" { return Err(Missing(path)) }
  let n = parseInt(text |> trim).mapErr(BadPort)?  # two variants hold a String: say which
  Ok(n)
}

test "a port is read from a file" {
  let _ = fs.writeText("port", "8080\n")  # a test has an empty directory of its own
  expect(port("port") == Ok(8080))
  expect(port("nope") == Err(Unreadable(NotFound("nope"))))
}
# endregion errors

# region totality
fn third(xs: List(Int)) -> Int = xs[2] ?? 0  # indexing gives an Option

fn mean(total: Int, count: Int) -> Int = match nonzero(count) {
  Some(n) => total / n  # / takes a NonZero(Int)
  None => 0
}

fn perHundred(failed: Int, sent: Int) -> Int = (failed * 100).div(sent) ?? 0

fn nextByte(b: Byte) -> Byte = b.wrappingAdd(1)         # 255 wraps to 0
fn grow(n: UInt32) -> Option(UInt32) = n.checkedAdd(1)  # None when it doesn't fit
# endregion totality

# region mut
type Counter = { hits: Int, last: Option(Instant) }

fn record(mut c: Counter, at: Instant) {  # may change the caller's counter
  c.hits += 1
  c.last = Some(at)
}

fn tally(times: List(Instant)) -> Int {
  var counter: Counter = { hits: 0, last: None }
  for t in times {
    record(mut counter, t)  # mut at the call too
  }
  counter.hits
}
# endregion mut

# region dyn
trait Sink {
  fn write(self, line: String) -> String
}

type Shout = { prefix: String }
type Tagged = { tag: String }

impl Sink for Shout {
  fn write(self, line) = "{self.prefix}{line |> upper}"
}

impl Sink for Tagged {
  fn write(self, line) = "[{self.tag}] {line}"
}

fn sinkFor(loud: Bool) -> dyn Sink {  # dyn: the type is chosen at run time
  let shout: Shout = { prefix: "! " }
  let tagged: Tagged = { tag: "log" }
  if loud then shout else tagged
}
# endregion dyn

test "totality, mut and dyn" {
  expect(third([1, 2]) == 0 && mean(7, 0) == 0 && perHundred(1, 4) == 25)
  expect(nextByte(255) == 0 && grow(1) == Some(2))
  expect(tally([opens, opens + 1 h]) == 2)
  expect(sinkFor(true).write("hi") == "! HI" && sinkFor(false).write("hi") == "[log] hi")
}

# ---------------------------------------------------------------- the ticket desk

let oncall = UserId("oncall")

# region entity
## A ticket: its state, changed only by its own methods, one transaction
## at a time.
entity Ticket(id: String) {
  state title: String = ""
  state open: Bool = false
  state escalated: Bool = false

  ## Opens the ticket and starts its escalation, in one transaction.
  pub fn raise(t: String) uses {start escalate} {
    if open { return }
    title = t
    open = true
    escalate.start(id, key: "escalate-{id}")
  }

  pub fn close() { open = false }

  pub fn markEscalated() { escalated = true }
}
# endregion entity

# region view
## Tickets that were escalated and are still open, kept up to date as
## Ticket transactions commit.
view overdue = Ticket.all |> filter(fn t: t.open && t.escalated)
# endregion view

# region tell
## A waiting function: its `!` steps are the calling job's own steps.
fn tell(to: UserId, text: String, key: Key) {
  match notify.send!(to:, text:, key:).reconcile!() {
    Accepted(_) => ()
    Refused(_) => log.warn("Notify refused {key}")
  }
}
# endregion tell

# region jobs
## Escalates a ticket that nobody answered within four hours.
job escalate(id: String) {
  let reply = next!(notify.replied, where: fn r: r.ticket == id, within: 4 h)
  if reply != None { return }
  let ticket = Ticket(id).read!()  # a snapshot, read in a step
  if !ticket.open { return }
  tell!(oncall, "{ticket.title} has waited four hours", "escalate-{id}")
  Ticket(id).markEscalated!()
}

## A reply closes its ticket.
@on(event(notify.replied))
job closeOnReply(r: notify.Reply) {
  Ticket(r.ticket).close!()
}

## Each morning, the person on call hears how many tickets are overdue.
@on(daily(09:00, in: dublin))
job morning(day: Date) {
  let late = overdue.read!()
  if late == [] { return }
  tell!(oncall, "{late |> size} tickets are overdue", "morning-{day}")
}
# endregion jobs

# region main
## Raises one ticket: `eye run . -- t-1 "Disk full on db-2"`. eye run runs
## main on every start; the key in raise starts the job once.
pub fn main(args: List(String)) -> Result((), String) uses {entity Ticket, start escalate} {
  let [_, id, title] = args else { return Err("usage: eye run . -- ID TITLE") }
  Ticket(id).raise(title)
  Ok(())
}
# endregion main

# region tests
test "an unanswered ticket is escalated after four hours, once" {
  let sent = fakes(notify, from: "recordings/")
  with sent
  with clock(2026-10-12 10:00 in dublin)
  Ticket("t-1").raise("Disk full on db-2")
  advance(3 h)
  expect(!Ticket("t-1").escalated)
  advance(1 h)
  expect(Ticket("t-1").escalated)
  let [message] = sent.calls(notify.send) else { unreachable("sent more or less than once") }
  expect(message.key == "escalate-t-1")
}

test "a reply closes the ticket, and nothing is sent" {
  let sent = fakes(notify, from: "recordings/")
  with sent
  Ticket("t-1").raise("Disk full on db-2")
  advance(1 h)
  notify.replied.publish({ ticket: "t-1", by: UserId("ann"), text: "on it" })
  advance(4 h)
  expect(!Ticket("t-1").open && !Ticket("t-1").escalated)
  expect(sent.calls(notify.send) == [])
}

test "a lost reply is settled by asking Notify, not by sending again" {
  with fakes(notify, from: "recordings/")
  Ticket("t-2").raise("Backups are late")
  advance(4 h)
  expect(Ticket("t-2").escalated)
}

test "the morning message counts overdue tickets" {
  let sent = fakes(notify, from: "recordings/")
  with sent
  with clock(2026-10-12 10:00 in dublin)
  Ticket("t-1").raise("Disk full on db-2")
  advance(23 h)  # 09:00 the next day
  expect(sent.calls(notify.send) |> size == 2)
}
# endregion tests
