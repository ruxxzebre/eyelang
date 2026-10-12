# What SPECS.md §17's example of serving Notify assumes (context=serving):
# the contract it serves (notify/notify.eye), and an entity per message that
# remembers what each key did.

import notify.{Delivery, Pending, Priority, ReceiptId, UserId}

type Outgoing = { text: String, to: UserId, priority: Priority }

entity Message(key: Key) {
  state outgoing: Option(Outgoing) = None
  state outcome: Option(Delivery) = None

  pub fn queue(m: Outgoing) -> Delivery {
    match outcome {
      Some(d) => d
      None => {
        let d = Pending(receipt: ReceiptId("{key}"))
        outgoing = Some(m)
        outcome = Some(d)
        d
      }
    }
  }

  pub fn delivery() -> Option(Delivery) = outcome
}
