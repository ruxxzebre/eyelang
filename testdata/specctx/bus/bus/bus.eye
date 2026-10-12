# A message bus that carries pings.

pub type Ping = { n: Int }

contract Bus {
  event pinged: Ping on "bus.pinged"
}
