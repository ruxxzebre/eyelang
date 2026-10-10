# SPECS.md section 17, example 1.
@port(3001)
@allow(digest, inbox, welcome)          # who may call; everyone else is denied
contract Notify {
  @key(required)
  @reconcile(receipt)
  command send(to: UserId, text: String) -> Delivery

  query receipt(key: Key) -> Delivery
  query identity(user: UserId) -> Identity

  event delivered: Delivered on "notify.events"
}

type Delivery =
  | Sent(receipt: ReceiptId, at: Instant)
  | Pending(receipt: ReceiptId)
  | Refused(reason: Refusal)
