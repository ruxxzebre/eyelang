# A text message gateway. It takes no idempotency key and can't say what it
# did with a request, so a lost reply can't be reconciled.

pub type Sent =
  | Queued(id: String)
  | BadNumber

# region contract
contract Sms {
  ## Sends a text. Sending it again sends a second text.
  @http("POST /messages")
  command text(to: String, body: String) -> Sent
}
# endregion contract
