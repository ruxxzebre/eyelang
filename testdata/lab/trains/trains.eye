# Train reminders: a person pins a train, and before it leaves they get a
# reminder with "Boarded" and "Not taking it" buttons; while it runs late they
# hear about it, and again when it recovers.
#
# A reminder is recorded and its job started in one transaction, so a tick
# that runs twice can't send it twice. Service dates are Irish dates.

import home.{UserId, dublin}
import irishrail.{TrainCode, StopId, Movement}
import notify.{Choice, QuestionId}

## A train a person means to take.
type Pin = { train: TrainCode, date: Date, from: StopId, to: StopId }

## How long before departure a reminder comes.
type Lead =
  | Fixed(before: Duration)
  ## The time it takes to reach the departure station from home.
  | Travel

## One reminder: for one pinned train, at one lead.
type Reminder = { pin: Pin, lead: Lead }

entity Traveller(id: UserId) {
  state pins: List(Pin) = []
  state leads: List(Lead) = [Fixed(before: 15 min)]
  state travel: Map(StopId, Duration) = {}
  state reminded: Set(Reminder) = {}
  state aboard: Option(Pin) = None
  ## The last delay each train was reported with, so that only a real change
  ## (five minutes or more) is news.
  state reported: Map(Pin, Duration) = {}

  pub fn pin(p: Pin) {
    if !(pins |> contains(p)) { pins += [p] }
  }

  pub fn unpin(p: Pin) {
    pins = pins |> filter(fn q: q != p)
  }

  pub fn board(p: Pin) {
    aboard = Some(p)
    unpin(p)
  }

  pub fn arrived() { aboard = None }

  ## Sends a reminder, unless it was sent before. The record and the start
  ## of the job that sends it commit together.
  pub fn remind(r: Reminder) {
    if reminded |> contains(r) { return }
    reminded = reminded |> add(r)
    ask.start(id, r)
  }

  pub fn report(p: Pin, delay: Duration) {
    reported = reported |> insert(p, delay)
  }
}

## Everyone with a train to watch.
view travelling = Traveller.all
  |> filter(fn t: t.pins != [] || t.aboard != None)

# ---------------------------------------------------------------- watching

## Watches a person's trains every 45 seconds, for as long as they have any.
@on(enters(travelling))
@priority(interactive)
job watch(t: Ref(Traveller)) {
  while true {
    let me = t.read!()
    if me.pins == [] && me.aboard == None { return }
    let at = now!()
    for pin in me.pins {
      let Some(movement) = irishrail.movement!(pin.train, pin.date) else { continue }
      for r in due(me, pin, movement, at) { t.remind!(r) }
      match delayNews(me.reported[pin], movement) {
        Some(news) => {
          notify.send!(
            event: "trains.delay",
            title: "{pin.train}",
            body: news,
            priority: High,
            to: [me.id],
            # A new key for each delay: a correction is a new message.
            key: "late-{me.id}-{pin.train}-{pin.date}-{movement.delay}",
          )
          t.report!(pin, movement.delay)
        }
        None => ()
      }
    }
    match me.aboard {
      Some(pin) => {
        let movement = irishrail.movement!(pin.train, pin.date)
        if hasArrived(pin, movement, at) { t.arrived!() }
      }
      None => ()
    }
    sleep!(45 s)
  }
}

## The reminders due now for one train: past their time, before departure,
## and not sent yet.
fn due(me: Traveller, pin: Pin, m: Movement, at: Instant) -> List(Reminder) {
  let Some(departs) = m.departures[pin.from] else { return [] }
  if m.cancelled || at >= departs { return [] }
  me.leads
    |> filter(fn lead: match lead {
      Fixed(before) => at >= departs - before
      Travel => me.travel[pin.from] |> map(fn trip: at >= departs - trip) ?? false
    })
    |> map(fn lead: { pin, lead })
    |> filter(fn r: !(me.reminded |> contains(r)))
}

## What to say about a delay, if it changed enough to say anything: late at
## five minutes, again for each further five, and once when back under three.
fn delayNews(reported: Option(Duration), m: Movement) -> Option(String) {
  let before = reported ?? 0 min
  if m.cancelled then return Some("{m.train} is cancelled.")
  if m.delay >= 5 min && (m.delay - before >= 5 min || before - m.delay >= 5 min) {
    return Some("{m.train} is running {m.delay} late.")
  }
  if before >= 5 min && m.delay < 3 min { return Some("{m.train} is back on time.") }
  None
}

## Whether a boarded train has reached the person's stop. Without word from
## the feed, a trip ends three hours after it was due.
fn hasArrived(pin: Pin, m: Option(Movement), at: Instant) -> Bool = match m {
  Some(m) => m.arrivals[pin.to] |> map(fn due: at >= due) ?? false
  None => at >= pin.date.at(23:59, in: dublin) + 3 h
}

# ---------------------------------------------------------------- asking

let boarded: Choice = { id: "boarded", label: "Boarded" }
let skip: Choice = { id: "skip", label: "Not taking it" }

## Sends one reminder and acts on the answer. An unanswered question is
## dropped after six hours: a press long after the train has gone isn't a
## boarding worth recording.
job ask(person: UserId, r: Reminder) -> Result((), notify.Refusal) {
  let key = "remind-{person}-{r.pin.train}-{r.pin.date}-{leadName(r.lead)}"
  let asked = notify.ask!(
    event: "trains.reminder",
    to: person,
    text: reminderText(r),
    choices: [boarded, skip],
    within: 6 h,
    key:,
  ).reconcile!()
  let question = match asked {
    Asking(q) => q
    NotAsked(reason) => return Err(reason)
  }
  let answer = next!(notify.answered, where: fn a: a.question == question, within: 6 h)
  match answer {
    Some(a) if a.choice == boarded.id => Traveller(person).board!(r.pin)
    Some(_) => Traveller(person).unpin!(r.pin)
    None => ()
  }
  Ok(())
}

fn leadName(lead: Lead) -> String = match lead {
  Fixed(before) => "{before}"
  Travel => "travel"
}

fn reminderText(r: Reminder) -> String = match r.lead {
  Fixed(before) => "{r.pin.train} leaves {r.pin.from} in {before}."
  Travel => "Leave now for the {r.pin.train} from {r.pin.from}."
}

# ---------------------------------------------------------------- tests

let pin: Pin = {
  train: TrainCode("E216"),
  date: 2026-10-12,
  from: StopId("PERSE"),
  to: StopId("MHIDE"),
}

let movement: Movement = {
  train: TrainCode("E216"),
  date: 2026-10-12,
  departures: { (StopId("PERSE")): 2026-10-12 08:30 in dublin },
  arrivals: {},
  delay: 0 min,
  cancelled: false,
}

test "a reminder is due once, inside its lead" {
  with fakes(irishrail, notify, from: "recordings/")
  let me = Traveller(UserId("ann"))
  me.pin(pin)
  expect(due(me.read(), pin, movement, 2026-10-12 08:10 in dublin) == [])
  expect(due(me.read(), pin, movement, 2026-10-12 08:16 in dublin) |> size == 1)
  me.remind({ pin, lead: Fixed(before: 15 min) })
  expect(due(me.read(), pin, movement, 2026-10-12 08:17 in dublin) == [])
}

test "a press on Boarded boards the train" {
  with fakes(irishrail, notify, from: "recordings/")
  let me = Traveller(UserId("ann"))
  me.pin(pin)
  me.remind({ pin, lead: Fixed(before: 15 min) })
  notify.answered.publish({ question: QuestionId("q-1"), choice: "boarded", by: me.id, at: now() })
  advance(1 min)
  expect(me.pins == [])
  expect(me.aboard == Some(pin))
}

test "delays are news at five minutes, and each five after" {
  let late = fn d: { movement with delay: d }
  expect(delayNews(None, late(4 min)) == None)
  expect(delayNews(None, late(6 min)) != None)
  expect(delayNews(Some(6 min), late(8 min)) == None)
  expect(delayNews(Some(6 min), late(11 min)) != None)
  expect(delayNews(Some(11 min), late(2 min)) == Some("E216 is back on time."))
}
