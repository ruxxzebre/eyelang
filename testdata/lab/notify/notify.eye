# Notify delivers messages to people over Telegram and Web Push. This
# contract is its interface: the lab's programs call it, and the notifier
# module serves it.

import home.{UserId}
import telegram.{ChatId}

pub type Priority = Low | Normal | High

pub type Format = Plain | Markdown

pub type MessageId = | MessageId(String)

## What Notify did with a message. Accepted means the outbox has it, not that
## anyone has read it: quiet hours may hold it until morning.
pub type Delivery =
  | Accepted(message: MessageId, deliveries: Int)
  | Refused(reason: Refusal)

pub type Refusal =
  ## Nobody receives this event.
  | NoRecipients
  ## The caller may not send this event.
  | NotAllowed
  | TooLong(limit: Int)
  ## A question needs 2 to 4 choices.
  | BadChoices

## A button under a message: `id` comes back in the answer.
pub type Choice = { id: String, label: String }

pub type QuestionId = | QuestionId(String)

pub type Asked =
  | Asking(QuestionId)
  | NotAsked(Refusal)

## The first press on a question's buttons. Later presses change nothing.
pub type Answered = { question: QuestionId, choice: String, by: UserId, at: Instant }

@port(3001)
@allow(digest, trains, access)
contract Notify {
  ## Sends a message to everyone who receives `event`, or only to the people
  ## in `to`.
  @key(required)
  @reconcile(message)
  command send(
    event: String,
    title: String,
    body: String,
    priority: Priority = Normal,
    format: Format = Plain,
    to: List(UserId) = [],
  ) -> Delivery

  ## Asks one person, with 2 to 4 buttons. The question closes after
  ## `within`; an answer comes as an `answered` event.
  @key(required)
  @reconcile(question)
  command ask(
    event: String,
    to: UserId,
    text: String,
    choices: List(Choice),
    within: Duration,
  ) -> Asked

  ## What a send did; None if no send with this key arrived.
  query message(key: Key) -> Option(Delivery)

  query question(key: Key) -> Option(Asked)

  ## The person behind a Telegram chat, if they receive `event`.
  query identity(chat: ChatId, event: String) -> Option(UserId)

  @key(question)
  event answered: Answered on "notify.answers"
}
