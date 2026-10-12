# Notify as SPECS.md §17 shows it, with the operations the example serves:
# send, and the query that reconciles it.

pub type UserId = | UserId(String)

pub type ReceiptId = | ReceiptId(String)

pub type Refusal = | Blocked | UnknownUser

pub type Priority = Low | Normal | High

pub type Delivery =
  | Accepted(receipt: ReceiptId, at: Instant)
  | Pending(receipt: ReceiptId)
  | Refused(reason: Refusal)

@port(3001)
contract Notify {
  @key(required)
  @reconcile(receipt)
  command send(to: UserId, text: String, priority: Priority = Normal) -> Delivery

  query receipt(key: Key) -> Option(Delivery)
}
