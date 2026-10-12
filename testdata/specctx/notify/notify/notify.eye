# The Notify of SPECS.md §17, for the examples that call it (context=notify,
# with another context for the rest): the contract §17 shows, and the
# questions with buttons that §15's approvals send.

pub type UserId = | UserId(String)

pub type ReceiptId = | ReceiptId(String)

pub type Refusal = | Blocked | UnknownUser

pub type Priority = Low | Normal | High

pub type Delivery =
  | Accepted(receipt: ReceiptId, at: Instant)
  | Pending(receipt: ReceiptId)
  | Refused(reason: Refusal)

pub type Delivered = { receipt: ReceiptId, at: Instant }

pub type Identity = { name: String, telegram: Option(String) }

## A button under a question: `id` comes back in the answer.
pub type Choice = { id: String, label: String }

pub type QuestionId = | QuestionId(String)

pub type Asked =
  | Asking(QuestionId)
  | NotAsked(Refusal)

## The first press on a question's buttons.
pub type Answered = { question: QuestionId, choice: String, by: UserId, at: Instant }

@port(3001)
contract Notify {
  @key(required)
  @reconcile(receipt)
  command send(to: UserId, text: String, priority: Priority = Normal) -> Delivery

  query receipt(key: Key) -> Option(Delivery)
  query identity(user: UserId) -> Identity

  @key(required)
  @reconcile(question)
  command ask(to: UserId, text: String, choices: List(Choice), within: Duration) -> Asked

  query question(key: Key) -> Option(Asked)

  @key(receipt)
  event delivered: Delivered on "notify.events"

  event answered: Answered on "notify.answered"
}
