# What SPECS.md §17's Notify over HTTP assumes (context=notifyhttp): the
# types its send names.

type UserId = | UserId(String)

type ReceiptId = | ReceiptId(String)

type Refusal = | Blocked | UnknownUser

type Priority = Low | Normal | High

type Delivery =
  | Accepted(receipt: ReceiptId, at: Instant)
  | Pending(receipt: ReceiptId)
  | Refused(reason: Refusal)
