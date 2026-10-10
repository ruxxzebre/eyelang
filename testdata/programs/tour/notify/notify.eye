# The notification service, written in TypeScript (SPECS.md §17).

pub type UserId = | UserId(String)

pub type ReceiptId = | ReceiptId(String)

pub type Refusal = | Blocked | UnknownUser

pub type Delivery =
  | Sent(receipt: ReceiptId, at: Instant)
  | Pending(receipt: ReceiptId)
  | Refused(reason: Refusal)

@port(3001)
contract Notify {
  @key(required)
  @reconcile(receipt)
  command send(to: UserId, text: String) -> Delivery

  query receipt(key: Key) -> Delivery
}
