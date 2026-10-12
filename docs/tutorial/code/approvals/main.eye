# Access approvals. Someone asks for temporary access to a group; a model
# summarises the request for the approver; the approver answers with a button
# in their chat; the access is granted, and taken away again when its time is
# up. On weekday mornings the approver hears what is still waiting.

import ai
import access.{Grant}
import notify.{Choice, UserId}

# region setup
## Who answers access requests.
let approver = UserId("operator")

## Schedules and the times in messages are Irish.
let dublin: Zone = "Europe/Dublin"
# endregion setup

# region details
pub type RequestId = | RequestId(String)

## What someone asks for.
pub type Details = {
  person: UserId,
  ## A group in the access service, such as "prod-db".
  group: String,
  reason: String,
  lasting: Duration,
}
# endregion details

# region policy
## The groups that can be asked for, and the longest each can be held.
let groups: Map(String, Duration) = { "prod-db": 4 h, "billing": 8 h, "vpn-admin": 1 h }

pub type Inadmissible =
  | UnknownGroup(String)
  | TooLong(most: Duration)

## Checks a request against the rules before anyone is asked.
pub fn admissible(d: Details) -> Result((), Inadmissible) {
  let Some(most) = groups[d.group] else { return Err(UnknownGroup(d.group)) }
  if d.lasting > most { return Err(TooLong(most:)) }
  Ok(())
}
# endregion policy

# region request
pub type Status =
  | New
  | Waiting
  | Denied(why: String)
  | Unanswered
  | Active(grant: Grant, until: Instant)
  | Ended

entity Request(id: RequestId) {
  state details: Option(Details) = None
  state status: Status = New
  state summary: String = ""
  state reminders: Int = 0

  ## Takes a request in and starts its review, in one transaction. Sending
  ## the same request again changes nothing and starts nothing.
  pub fn submit(d: Details) uses {start review} {
    if status != New { return }
    details = Some(d)
    match admissible(d) {
      Ok(_) => {
        status = Waiting
        review.start(id, d, key: "review-{id}")
      }
      Err(why) => status = Denied(why: "{why}")
    }
  }

  pub fn summarise(text: String) { summary = text }

  pub fn remind() { reminders += 1 }

  pub fn activate(g: Grant, until: Instant) { status = Active(grant: g, until:) }

  pub fn close(s: Status) { status = s }
}
# endregion request

# region active
## Every request whose access has been granted and not yet taken away.
view active = Request.all
  |> filter(fn r: match r.status {
    Active(..) => true
    _ => false
  })
# endregion active

# region summary
## What the approver reads about a request, written by a model.
type Summary = {
  ## One sentence, at most 20 words: who wants what, and why.
  text: String,
  risk: Risk,
}

type Risk =
  | Low
  | Medium
  ## Production data, money or admin rights.
  | High(reason: String)

let instructions = """
  You help an operator review requests for temporary access to a group.
  Summarise the request in one sentence and rate its risk. Production data,
  money and admin rights are never Low. Use only what the request says.
  """

## Asks a model to summarise a request. If the model fails, the approver
## gets the request without a summary rather than no request at all.
fn summarise(d: Details) -> Summary {
  let reply: Result(Summary, ai.AiError) = ai.generate!(
    model: "claude-sonnet-5-5",
    system: instructions,
    prompt: d |> toJson,
  )
  match reply {
    Ok(summary) => summary
    Err(e) => { text: "No summary: the model failed ({e}).", risk: Medium }
  }
}

fn describe(r: Risk) -> String = match r {
  Low => "low risk"
  Medium => "medium risk"
  High(reason) => "HIGH RISK: {reason}"
}
# endregion summary

# region review
let allow: Choice = { id: "allow", label: "Allow" }
let deny: Choice = { id: "deny", label: "Deny" }

type ReviewError =
  | NotAsked(reason: String)
  | NotGranted(group: String)

## Reviews one request: a summary from the model, a question to the approver,
## then the grant. The approver gets one question at a time, in the order the
## requests came in.
@concurrency(1)
job review(id: RequestId, d: Details) -> Result(Status, ReviewError) {
  let summary = summarise!(d)
  Request(id).summarise!(summary.text)
  let asked = notify.ask!(
    to: approver,
    text: "{d.person} asks for {d.group} for {d.lasting} ({describe(summary.risk)}). {summary.text}",
    choices: [allow, deny],
    within: 3 days,
    key: "review-{id}",
  ).reconcile!()
  let question = match asked {
    Asking(q) => q
    NotAsked(reason) => return Err(NotAsked(reason))
  }
  let Some(answer) = next!(notify.answered, where: fn a: a.question == question, within: 3 days) else {
    Request(id).close!(Unanswered)
    tell!(d.person, "Nobody answered your request for {d.group}. Ask again if you still need it.", key: "unanswered-{id}")
    return Ok(Unanswered)
  }
  if answer.choice == deny.id {
    let denied = Denied(why: "{answer.by} said no")
    Request(id).close!(denied)
    tell!(d.person, "Your request for {d.group} was denied.", key: "denied-{id}")
    return Ok(denied)
  }
  let until = now!() + d.lasting
  match access.grant!(d.person, d.group, key: "grant-{id}").reconcile!() {
    Added(grant) => {
      Request(id).activate!(grant, until)
      tell!(d.person, "You are in {d.group} until {until.time(in: dublin)}.", key: "granted-{id}")
      Ok(Active(grant:, until:))
    }
    NoSuchGroup => Err(NotGranted(d.group))
  }
}
# endregion review

# region tell
## Tells a person how their request went. Notify keeps what each key did, so
## a lost reply is settled by sending again with the same key.
pub fn tell(person: UserId, text: String, key: Key) uses {wait, net Notify.send} {
  match notify.send!(to: person, text:, key:).reconcile!() {
    Accepted(_) => ()
    Refused(error) => log.warn("Notify refused {key}: {error}")
  }
}
# endregion tell

# region expire
## Every request that becomes active gets one run of this job. It sleeps
## until the access runs out, then takes it away.
@on(enters(active))
job expire(r: Ref(Request)) {
  let Active(grant, until) = r.read!().status else { return }
  sleepUntil!(until)
  match access.revoke!(grant.id, key: "revoke-{grant.id}").reconcile!() {
    Removed | AlreadyGone => ()
  }
  r.close!(Ended)
  tell!(grant.person, "Your access to {grant.group} has ended.", key: "ended-{grant.id}")
}
# endregion expire

# region morning
## On weekday mornings the approver hears how many requests are waiting.
@on(weekdays(09:00, in: dublin))
job morning(day: Date) {
  let open = Request.all |> filter(fn r: r.status == Waiting)
  if open == [] { return }
  for r in open { Request(r.id).remind!() }
  tell!(approver, "{open |> size} access requests are waiting for you.", key: "waiting-{day}")
}
# endregion morning

# region main
## Submits one request from the command line, for example
## `eye run . -- r-1 ann prod-db 2 "fix the stuck migration"`. Running it
## again with the same id changes nothing.
pub fn main(args: List(String)) -> Result((), String) uses {entity Request, start review} {
  let usage = "usage: eye run . -- ID PERSON GROUP HOURS REASON"
  let (Some(id), Some(person), Some(group), Some(hours), Some(reason)) = (args[1], args[2], args[3], args[4], args[5]) else {
    return Err(usage)
  }
  let n = parseInt(hours).mapErr(fn _: usage)?
  Request(RequestId(id)).submit({ person: UserId(person), group, reason, lasting: 1 h * n })
  Ok(())
}
# endregion main

# ---------------------------------------------------------------- tests

# region test-data
let ann = UserId("ann")
let first = RequestId("r-1")

let fix: Details = {
  person: ann,
  group: "prod-db",
  reason: "Fix the stuck migration",
  lasting: 2 h,
}

fn answer(q: String, choice: String) {
  notify.answered.publish({ question: notify.QuestionId(q), choice, by: approver, at: now() })
}
# endregion test-data

# region test-policy
test "a request outside the rules is denied before anyone is asked" {
  expect(admissible({ fix with group: "root" }) == Err(UnknownGroup("root")))
  Request(first).submit({ fix with lasting: 12 h })
  expect(Request(first).status == Denied(why: "TooLong(most: 4 h)"))
}
# endregion test-policy

# region test-allow
test "the approver allows, and the access ends when its time is up" {
  with fakes(ai, notify, access, from: "recordings/")
  with clock(2026-10-12 10:00 in dublin)
  let r = Request(first)
  r.submit(fix)
  expect(r.status == Waiting)
  expect(r.summary == "Ann wants two hours on the production database to fix a stuck migration.")
  answer("q-1", "allow")
  expect(r.status == Active(grant: { id: access.GrantId("g-1"), person: ann, group: "prod-db" }, until: 2026-10-12 12:00 in dublin))
  advance(119 min)
  expect(r.status != Ended)
  advance(1 min)
  expect(r.status == Ended)
}
# endregion test-allow

# region test-deny
test "the approver says no" {
  with fakes(ai, notify, from: "recordings/")
  Request(first).submit(fix)
  answer("q-1", "deny")
  expect(Request(first).status == Denied(why: "operator said no"))
}
# endregion test-deny

# region test-unanswered
test "a request nobody answers for three days is closed" {
  with fakes(ai, notify, from: "recordings/")
  with clock(2026-10-16 10:00 in dublin)  # a Friday
  Request(first).submit(fix)
  advance(71 h)
  expect(Request(first).status == Waiting)
  advance(1 h)
  expect(Request(first).status == Unanswered)
}
# endregion test-unanswered

# region test-lost
test "a lost reply from the access service is settled by asking, not by granting again" {
  with fakes(ai, notify, access, from: "recordings/")
  let r = Request(RequestId("r-2"))
  r.submit({ person: UserId("bob"), group: "billing", reason: "Check an invoice", lasting: 1 h })
  answer("q-2", "allow")
  let Active(grant, ..) = r.status else { unreachable("the request should be active") }
  expect(grant.id == access.GrantId("g-2"))
}
# endregion test-lost

# region test-model-down
test "a model that fails costs the summary, not the request" {
  with fakes(notify, from: "recordings/")
  with fakes(ai, from: "recordings/model-down/")
  Request(first).submit(fix)
  expect(Request(first).status == Waiting)
  expect(Request(first).summary |> startsWith("No summary"))
}
# endregion test-model-down

# region test-one-at-a-time
test "the approver gets one question at a time, in order" {
  with fakes(ai, notify, from: "recordings/")
  let second = Request(RequestId("r-2"))
  Request(first).submit(fix)
  second.submit({ person: UserId("bob"), group: "billing", reason: "Check an invoice", lasting: 1 h })
  expect(second.summary == "")
  answer("q-1", "deny")
  expect(second.summary == "Bob wants an hour of billing access to check an invoice.")
}
# endregion test-one-at-a-time

# region test-morning
test "on weekday mornings the approver hears what is waiting" {
  with fakes(ai, notify, from: "recordings/")
  with clock(2026-10-16 08:00 in dublin)  # a Friday
  Request(first).submit(fix)
  advance(1 h)
  expect(Request(first).reminders == 1)
  advance(2 days)  # Saturday and Sunday bring no reminder
  expect(Request(first).reminders == 1)
}
# endregion test-morning
