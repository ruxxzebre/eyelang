# A mail service. This contract is its interface; the invoices program calls it.

## What the service did with a message.
pub type Delivery =
  ## The service has it. A repeated key gives the first answer again.
  | Sent(id: String)
  ## An address the service can't deliver to.
  | Bounced(reason: String)

# region contract
contract Mail {
  ## Sends one message. Each key is sent at most once.
  @key(required)
  @reconcile(sent)
  command send(to: String, text: String) -> Delivery

  ## What happened to the message sent with this key, if it arrived.
  query sent(key: Key) -> Option(Delivery)
}
# endregion contract
