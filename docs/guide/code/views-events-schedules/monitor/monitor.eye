# The monitoring service: it sends a reading of each disk every few minutes,
# and an acknowledgement when someone has seen an alert.

## How full one disk is, in percent.
pub type Reading = { id: Int, host: String, used: Int }

## Who has seen the alert for which host.
pub type Ack = { host: String, by: String }

# region contract
contract Monitor {
  ## One reading of one disk. A reading sent twice has the same id.
  @key(id)
  event readings: Reading on "monitor.readings"

  ## Someone has seen the alert for a host.
  event acks: Ack on "monitor.acks"
}
# endregion contract
