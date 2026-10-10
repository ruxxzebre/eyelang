# Authentik, the lab's identity provider: who exists and which groups they
# are in.

import home.{UserId}

pub type Person = { id: UserId, email: String, active: Bool, groups: Set(String) }

contract Authentik {
  query members(group: String) -> List(Person)

  query person(email: String) -> Option(Person)
}
