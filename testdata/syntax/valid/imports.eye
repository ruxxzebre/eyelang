import zeropod
import lab.notify
import std.collections.{Map, Set, toSet}
import lab.miniflux.{unread, Item,}

fn deliver(d: notify.Delivery, page: miniflux.Page(Item)) -> Option(notify.ReceiptId) = match d {
  Sent(receipt, ..) => Some(receipt)
  Pending(receipt) => Some(receipt)
  Refused(_) => None
}
