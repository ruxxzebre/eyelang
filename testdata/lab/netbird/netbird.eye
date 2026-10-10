# NetBird, the VPN: its users and groups. Its API has no idempotency keys and
# no way to look up an earlier request, so a change whose reply is lost can't
# be confirmed.

import home.{UserId}

pub type PeerUserId = | PeerUserId(String)

pub type PeerUser = {
  id: PeerUserId,
  email: String,
  groups: Set(String),
  ## Service users are the lab's own and never change here.
  service: Bool,
  blocked: Bool,
}

pub type Pending = { id: PeerUserId, email: String, joinedAt: Instant }

pub type Change =
  | Changed
  | NoSuchUser

contract Netbird {
  query users() -> List(PeerUser)

  command join(user: PeerUserId, group: String) -> Change

  command leave(user: PeerUserId, group: String) -> Change

  ## Lets a pending user in, as a guest or a full member.
  command approve(user: PeerUserId, groups: Set(String)) -> Change

  command reject(user: PeerUserId) -> Change

  @key(id)
  event pending: Pending on "netbird.users.pending"
}
