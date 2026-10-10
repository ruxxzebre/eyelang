# Notify sends messages to people in their chat, and asks them questions with
# buttons. This contract is its interface; the approvals program calls it.

pub type UserId = | UserId(String)

pub type MessageId = | MessageId(String)

## What Notify did with a message.
pub type Delivery =
  ## The outbox has it. A repeated key gives the first answer again.
  | Accepted(id: MessageId)
  ## A bad request, or a person Notify doesn't know.
  | Refused(error: String)

## A button under a message: `id` comes back in the answer.
pub type Choice = { id: String, label: String }

pub type QuestionId = | QuestionId(String)

pub type Asked =
  | Asking(QuestionId)
  | NotAsked(reason: String)

## The first press on a question's buttons. Later presses change nothing.
pub type Answered = { question: QuestionId, choice: String, by: UserId, at: Instant }

# region contract
contract Notify {
  ## Sends a message to one person. Notify keeps what each key did, so sending
  ## again with the same key is how a lost reply is settled.
  @http("POST /v1/send", status: { 200: Accepted, 400: Refused })
  @key(required)
  @reconcile(send)
  command send(to: UserId, text: String) -> Delivery

  ## Asks one person, with 2 to 4 buttons. The question closes after
  ## `within`; an answer comes as an `answered` event.
  @http("POST /v1/ask")
  @key(required)
  @reconcile(ask)
  command ask(to: UserId, text: String, choices: List(Choice), within: Duration) -> Asked

  @key(question)
  event answered: Answered on "notify.answers"
}
# endregion contract
