# Notify sends messages to people in their chat. This contract is its
# interface; the digest calls it.

## What Notify did with a message.
pub type Delivery =
  ## The outbox has it. A repeated key gives the first answer again.
  | Accepted(id: String)
  ## A bad request, or a person Notify doesn't know.
  | Refused(error: String)

# region contract
contract Notify {
  ## Sends a message to one person. Each key is sent at most once.
  @key(required)
  @reconcile(sent)
  command send(to: String, text: String) -> Delivery

  ## What happened to the message sent with this key, if it arrived.
  query sent(key: Key) -> Option(Delivery)
}
# endregion contract
