# Notify sends messages to people. This contract is its interface; the
# tickets program calls it and waits for its replies.

pub type UserId = | UserId(String)

## What Notify did with a message.
pub type Delivery =
  ## The outbox has it. A repeated key gives the first answer again.
  | Accepted(id: String)
  ## A person Notify doesn't know.
  | Refused(error: String)

## A person's answer to a message about a ticket.
pub type Reply = { ticket: String, by: UserId, text: String }

contract Notify {
  ## Sends a message to one person. Each key is sent at most once.
  @key(required)
  @reconcile(sent)
  command send(to: UserId, text: String) -> Delivery

  ## What happened to the message sent with this key, if it arrived.
  query sent(key: Key) -> Option(Delivery)

  ## Someone answered about a ticket.
  event replied: Reply on "notify.replies"
}
