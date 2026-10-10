# The access service: who is in which group. It has no expiry of its own, so
# the approvals program takes access away when it runs out.

import notify.{UserId}

pub type GrantId = | GrantId(String)

pub type Grant = { id: GrantId, person: UserId, group: String }

pub type Membership =
  | Added(Grant)
  | NoSuchGroup

pub type Removal =
  | Removed
  | AlreadyGone

# region contract
contract Access {
  ## Adds a person to a group. The service remembers what each key did.
  @http("POST /v1/grants")
  @key(required)
  @reconcile(grantFor)
  command grant(person: UserId, group: String) -> Membership

  ## What the grant with this key did, or None if it never arrived.
  @http("GET /v1/grants/by-key/:key")
  query grantFor(key: Key) -> Option(Membership)

  ## Takes a person out of a group again.
  @http("POST /v1/grants/:grant/revoke")
  @key(required)
  @reconcile(revoke)
  command revoke(grant: GrantId) -> Removal
}
# endregion contract
