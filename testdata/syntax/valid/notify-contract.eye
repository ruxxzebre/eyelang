@port(3001)
@allow(digest, inbox, welcome)
contract Notify {
  @key(required)
  @reconcile(receipt)
  command send(to: UserId, text: String) -> Delivery

  @fresh
  @destructive
  command forget(user: UserId) -> Forgotten

  query receipt(key: Key) -> Delivery
  query identity(user: UserId) -> Identity

  event delivered: Delivered on "notify.events"
  @unwitnessed("added before recordings existed")
  event bounced: Bounced on "notify.bounces"
}

type Delivery =
  | Sent(receipt: ReceiptId, at: Instant)
  | Pending(receipt: ReceiptId)
  | Refused(reason: Refusal)

type Forgotten = | Forgotten(at: Instant)
