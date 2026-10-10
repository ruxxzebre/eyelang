# Notify, written in Eyelang: it serves the Notify contract that the lab's
# other programs call. Each person has an outbox with quiet hours; each message
# is delivered by a job of its own; a question's first answer wins.
#
# The send key is the message's identity, so sending twice with one key
# queues one message, and nothing needs a separate dedupe table.

import home.{UserId, dublin, operator}
import notify.{Priority, Format, Choice, Delivery, QuestionId, Answered}
import telegram.{ChatId, MessageRef, Button}

## How a person wants to hear from the lab.
type Preferences = {
  chat: Option(ChatId),
  ## The events they receive, such as "digest.ireland".
  events: Set(String),
  ## While these hours last, only high-priority messages arrive at once.
  quiet: Option(Range(Time)),
  ## Messages below this priority aren't sent at all.
  least: Priority,
}

entity Account(id: UserId) {
  state prefs: Preferences = { chat: None, events: {}, quiet: Some(23:00..08:00), least: Low }

  pub fn setPreferences(p: Preferences) { prefs = p }
}

type Content = {
  event: String,
  title: String,
  body: String,
  priority: Priority,
  format: Format,
  choices: List(Choice),
}

type Copy =
  | Queued
  | Held(until: Instant)
  | Sent(MessageRef)
  ## Telegram may or may not have shown it, and it can't be asked. Held for a
  ## person to look at, never sent again.
  | Unsure
  | Undeliverable(reason: String)

## One message, named by the key it was sent with.
entity Message(key: Key) {
  state content: Option(Content) = None
  state copies: Map(UserId, Copy) = {}

  ## Queues the message for each person. Only the first call with a key
  ## does anything; a repeat gets the same answer.
  pub fn queue(c: Content, to: List(UserId)) -> Delivery {
    let deduped = content != None
    if !deduped {
      content = Some(c)
      for person in to {
        copies = copies |> insert(person, Queued)
        deliver.start(key, person)
      }
    }
    Accepted(id: notify.MessageId("{key}"), deliveries: copies |> size, deduped:)
  }

  ## What a send with this key did, if one arrived.
  pub fn delivery() -> Option(Delivery) =
    if content == None then None
    else Some(Accepted(id: notify.MessageId("{key}"), deliveries: copies |> size, deduped: true))

  pub fn mark(person: UserId, copy: Copy) {
    copies = copies |> insert(person, copy)
  }
}

# ---------------------------------------------------------------- the contract

impl notify.Notify {
  fn send(event, title, body, priority, format, to, key) {
    if body |> size > 240_000 { return Refused(error: "a message holds at most 240000 characters") }
    let recipients = to ?? receivers(event)
    if recipients == [] { return Refused(error: "nobody receives {event}") }
    Message(key).queue({ event, title, body, priority, format, choices: [] }, recipients)
  }

  fn ask(event, to, text, choices, within, key) {
    if choices |> size < 2 || choices |> size > 4 { return NotAsked(BadChoices) }
    let question = QuestionId("{key}")
    Question(question).open(choices, within:)
    Message(key).queue({ event, title: "", body: text, priority: High, format: Plain, choices }, [to])
    Asking(question)
  }

  fn message(key) = Message(key).delivery()

  fn question(key) = Message(key).delivery() |> map(fn _: Asking(QuestionId("{key}")))

  fn identity(chat, event) =
    Account.all
    |> find(fn a: a.prefs.chat == Some(chat) && a.prefs.events |> contains(event))
    |> map(fn a: a.id)
}

fn receivers(event: String) -> List(UserId) =
  Account.all
  |> filter(fn a: a.prefs.events |> contains(event))
  |> map(fn a: a.id)

# ---------------------------------------------------------------- delivery

## Delivers one copy of a message. Telegram allows one message a second in
## each chat.
@priority(interactive)
@limit(1, per: 1 s, by: person)
job deliver(key: Key, person: UserId) -> Result((), Undelivered) {
  let prefs = Account(person).read!().prefs
  let Some(content) = Message(key).read!().content else {
    unreachable("queue sets the content before it starts deliver")
  }
  if content.priority < prefs.least { return Err(BelowTheirPriority) }
  let Some(chat) = prefs.chat else { return Err(NoChat) }
  let at = now!()
  match quietUntil(prefs, content.priority, at) {
    Some(release) => {
      Message(key).mark!(person, Held(until: release))
      sleepUntil!(release)
    }
    None => ()
  }
  let buttons = content.choices |> map(fn c: { id: c.id, label: c.label })
  while true {
    match telegram.sendMessage!(chat, render(content), buttons:) {
      Posted(ref) => {
        Message(key).mark!(person, Sent(ref))
        return Ok(())
      }
      TooFast(retryAfter) => sleep!(retryAfter)
      Blocked => {
        Message(key).mark!(person, Undeliverable(reason: "they blocked the bot"))
        return Err(BlockedByThem)
      }
      Uncertain(_) => {
        # Telegram takes no idempotency key: the message may have arrived.
        # Sending it again could show it twice, so hold it for a person.
        Message(key).mark!(person, Unsure)
        return Err(Unconfirmed)
      }
    }
  }
}

type Undelivered = BelowTheirPriority | NoChat | BlockedByThem | Unconfirmed

## When a message must wait for quiet hours to end; None to send it now.
## High priority cuts through.
fn quietUntil(prefs: Preferences, priority: Priority, at: Instant) -> Option(Instant) {
  let Some(hours) = prefs.quiet else { return None }
  if priority == High || !(hours |> contains(at.time(in: dublin))) { return None }
  Some(hours.end.next(in: dublin, after: at))
}

fn render(c: Content) -> String =
  if c.title == "" then c.body else "{c.title}\n\n{c.body}"

# ---------------------------------------------------------------- questions

## A question asked with buttons. The first press answers it for everyone who
## was asked; later presses are told who answered.
entity Question(id: QuestionId) {
  state choices: List(Choice) = []
  state answer: Option(Answered) = None
  state open: Bool = false

  pub fn open(c: List(Choice), within: Duration) {
    choices = c
    open = true
    close.start(id, after: within)
  }

  pub fn press(choice: String, by: UserId, at: Instant) -> Result(Answered, Unanswerable) {
    match answer {
      Some(a) => return Err(AlreadyAnswered(a))
      None => ()
    }
    if !open { return Err(Closed) }
    if !(choices |> any(fn c: c.id == choice)) { return Err(NoSuchChoice) }
    let a: Answered = { question: id, choice, by, at }
    answer = Some(a)
    open = false
    notify.answered.publish(a)  # sent when this transaction commits
    Ok(a)
  }

  pub fn close() { open = false }
}

fn personFor(chat: ChatId) -> Option(UserId) =
  Account.all |> find(fn a: a.prefs.chat == Some(chat)) |> map(fn a: a.id)

## The question a Telegram message asked, if it asked one.
fn questionFor(ref: MessageRef) -> Option(QuestionId) =
  Message.all
  |> find(fn m: m.copies |> values |> contains(Sent(ref)))
  |> map(fn m: QuestionId("{m.key}"))

type Unanswerable =
  | AlreadyAnswered(Answered)
  | Closed
  | NoSuchChoice

job close(id: QuestionId) { Question(id).close!() }

## Button presses arrive as Telegram updates.
@on(event(telegram.updates))
job pressed(update: telegram.Update) {
  let Pressed(message, button) = update.body else { return }
  let Some(person) = personFor!(update.chat) else { return }
  let Some(question) = questionFor!(message) else { return }
  match Question(question).press!(button, by: person, at: update.sentAt) {
    Ok(a) => telegram.editMessage!(update.chat, message, "Decided: {a.choice} by {person}")
    Err(AlreadyAnswered(a)) => telegram.editMessage!(update.chat, message, "Already decided: {a.choice} by {a.by}")
    Err(Closed) | Err(NoSuchChoice) => ()
  }
}

# ---------------------------------------------------------------- tests

test "a repeated send queues one message" {
  let one = notify.send(event: "digest.ireland", title: "a", body: "b", to: Some([operator]), key: "k1")
  let two = notify.send(event: "digest.ireland", title: "a", body: "b", to: Some([operator]), key: "k1")
  expect(one == Accepted(id: notify.MessageId("k1"), deliveries: 1, deduped: false))
  expect(two == Accepted(id: notify.MessageId("k1"), deliveries: 1, deduped: true))
  expect(Message("k1").copies |> size == 1)
}

test "quiet hours hold a normal message until eight" {
  let prefs: Preferences = { chat: Some(ChatId(1)), events: {}, quiet: Some(23:00..08:00), least: Low }
  let at = 2026-10-12 07:40 in dublin
  expect(quietUntil(prefs, Normal, at) == Some(2026-10-12 08:00 in dublin))
  expect(quietUntil(prefs, High, at) == None)
  expect(quietUntil(prefs, Normal, 2026-10-12 12:00 in dublin) == None)
}

test "the first press answers a question" {
  let q = Question(QuestionId("q1"))
  q.open([{ id: "yes", label: "Yes" }, { id: "no", label: "No" }], within: 1 h)
  let at = 2026-10-12 09:00 in dublin
  expect(q.press("yes", by: operator, at:) |> isOk)
  expect(q.press("no", by: UserId("someone"), at:) |> isErr)
  expect(q.answer |> map(fn a: a.choice) == Some("yes"))
}
