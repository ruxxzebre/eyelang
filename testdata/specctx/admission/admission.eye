# What SPECS.md §11's enum example assumes (context=admission): who is
# admitted, the step that admits them, and a button.

type UserId = | UserId(String)

type Step =
  | Admit(user: UserId, groups: Set(String))
  | Reject(user: UserId)

type Button = { id: String, label: String }
