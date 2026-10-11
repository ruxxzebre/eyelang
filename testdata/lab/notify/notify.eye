# Notify delivers messages to people over Telegram and Web Push. This
# contract is its interface: the lab's programs call it, and the notifier
# module serves it.

import home.{UserId}
import telegram.{ChatId}

pub type Priority = Low | Normal | High

pub type Format = Plain | Markdown

pub type MessageId = | MessageId(String)

## What Notify did with a message.
pub type Delivery =
  ## The outbox has it for `deliveries` people: not proof that anyone read it,
  ## since quiet hours may hold it until morning. A repeated key is `deduped`.
  | Accepted(id: MessageId, deliveries: Int, deduped: Bool)
  ## Notify is still admitting it. Sending again with the same key later
  ## gives the outcome without sending it twice.
  | Pending(actionId: String)
  ## A bad request, or an event this caller may not send.
  | Refused(error: String)

## Why a question wasn't asked.
pub type Refusal =
  ## The caller may not ask this.
  | NotAllowed
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
@allow(digest, trains, access, approving)
@http(names: snake)
@auth(bearer: "notify-token")
contract Notify {
  ## Sends a message to everyone who receives `event`, or only to the people
  ## in `to`. Notify keeps what each key did, so sending again with the same
  ## key is how a lost reply is reconciled.
  @http("POST /v1/send", status: { 200: Accepted, 202: Pending, 400: Refused, 403: Refused })
  @key(required, header: "X-Actions-Idempotency-Key", field: "dedupe_key")
  @reconcile(send)
  command send(
    event: String,
    title: String,
    body: String,
    priority: Priority = Normal,
    format: Format = Plain,
    to: Option(List(UserId)) = None,
  ) -> Delivery

  # The operations below have no @http: the lab's Notify offers them through
  # other routes (choices on /v1/send, the private identity bridges), and the
  # notifier module serves them in Eyelang.

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
