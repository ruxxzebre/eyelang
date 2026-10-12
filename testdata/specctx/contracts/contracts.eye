# What SPECS.md §17's Notify contract assumes (context=contracts): the types
# it names, and the modules its @allow lets call it.

type UserId = | UserId(String)

type ReceiptId = | ReceiptId(String)

type Refusal = | Blocked | UnknownUser

type Priority = Low | Normal | High

type Identity = { name: String, telegram: Option(String) }

type Delivered = { receipt: ReceiptId, at: Instant }
