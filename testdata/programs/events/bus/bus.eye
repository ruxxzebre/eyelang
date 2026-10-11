# A message bus: commands to send, and events that arrive.

pub type Ping = { id: Int, text: String }

pub type Sent = | Sent(Int)

contract Bus {
  @key(required)
  @reconcile(sent)
  command send(text: String, urgent: Bool = false) -> Sent

  query sent(key: Key) -> Option(Sent)

  @key(id)
  event pings: Ping on "bus.pings"
}
