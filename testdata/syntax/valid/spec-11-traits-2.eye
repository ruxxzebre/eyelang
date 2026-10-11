# SPECS.md section 11, example 2.
type Admission = Guest | Member | Refuse

fn admit(choice: String, user: UserId) -> Step =
  match Admission.parse(choice) {
    Some(Guest) => Admit(user, groups: toSet(["guests"]))
    Some(Member) => Admit(user, groups: toSet(["members"]))
    Some(Refuse) | None => Reject(user)
  }

let buttons = Admission.variants() |> map(fn a: { id: "{a}", label: "{a}" })
