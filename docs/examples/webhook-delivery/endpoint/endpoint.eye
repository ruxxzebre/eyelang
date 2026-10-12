# The partner's webhook endpoint, as this program's contract describes it.
# The partner keeps the key of each event it has handled, and answers what
# it did with one when asked.

import http

## What the partner answers when it has an event.
pub type Ack = {
  ## The partner's id for what it received.
  id: String,
}

# region contract
@http(names: snake)
contract Endpoint {
  ## Delivers one event. The key goes in the Idempotency-Key header, and the
  ## partner handles each key once. A failure comes back as an Err at once,
  ## so the caller decides when to try again.
  @http("POST /webhooks")
  @key(required)
  @reconcile(receipt)
  command deliver(kind: String, body: String) -> Result(Ack, http.HttpError)

  ## What the partner did with the event sent with this key, or None if it
  ## never arrived.
  @http("GET /webhooks/:key")
  query receipt(key: Key) -> Option(Result(Ack, http.HttpError))
}
# endregion contract
