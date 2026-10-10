# NetBird access: every minute, make NetBird's groups match Authentik's; when
# someone new joins NetBird, ask the operator whether to let them in.
#
# NetBird's API takes no idempotency key and can't be asked about an earlier
# request. A change whose reply is lost is doubtful even if NetBird looks right
# afterwards, since a delayed request may still land. So a doubtful step stops
# every later change until the operator has checked it. Nothing is replayed.

import authentik.{Person}
import home.{UserId, operator}
import netbird.{PeerUser, PeerUserId, Pending}
import notify.{Choice, QuestionId}

## The group of everyone active in Authentik with VPN access.
let people = "net-people"

## One change to NetBird.
type Step =
  | Join(user: PeerUserId, group: String)
  | Leave(user: PeerUserId, group: String)
  | Admit(user: PeerUserId, groups: Set(String))
  | Reject(user: PeerUserId)

## What to change in NetBird so that it matches Authentik: users are matched
## by email, ignoring case. Service users and blocked users are left alone.
pub fn plan(members: List(Person), users: List(PeerUser)) -> List(Step) {
  let wanted = members
    |> filter(fn p: p.active)
    |> map(fn p: p.email |> lower)
    |> toSet
  var steps: List(Step) = []
  for u in users {
    if u.service || u.blocked { continue }
    let should = wanted |> contains(u.email |> lower)
    let is = u.groups |> contains(people)
    if should && !is { steps += [Join(user: u.id, group: people)] }
    if !should && is { steps += [Leave(user: u.id, group: people)] }
  }
  steps
}

## The state of the sync. A doubtful step stops it until a person clears it.
entity Sync(name: String) {
  state doubtful: Option(Step) = None
  state applied: Int = 0

  pub fn doubt(s: Step) { doubtful = Some(s) }

  pub fn count(n: Int) { applied += n }

  ## Only a person who has checked NetBird can clear a doubt.
  pub fn clear(checked: Approval) { doubtful = None }
}

let netbirdSync = Sync("netbird")

view stuck = Sync.all |> filter(fn s: s.doubtful != None)

type SyncError =
  | Stuck
  | Doubtful(Step)

## Applies one step. A doubtful one stops the sync; it isn't sent again.
fn apply(step: Step) -> Result((), SyncError) {
  let changed = match step {
    Join(user, group) => netbird.join!(user, group)
    Leave(user, group) => netbird.leave!(user, group)
    Admit(user, groups) => netbird.approve!(user, groups)
    Reject(user) => netbird.reject!(user)
  }
  match changed {
    Changed | NoSuchUser => {
      netbirdSync.count!(1)
      Ok(())
    }
    Uncertain(_) => {
      netbirdSync.doubt!(step)
      Err(Doubtful(step))
    }
  }
}

# ---------------------------------------------------------------- the sync

## Runs every minute; a run that finds the previous one still going is
## skipped.
@on(every(1 min))
@concurrency(1)
job reconcile() -> Result(Int, SyncError) {
  if netbirdSync.read!().doubtful != None { return Err(Stuck) }
  let steps = plan(authentik.members!(people), netbird.users!())
  for step in steps { apply!(step)? }
  Ok(steps |> size)
}

## A doubt waits for the operator: they check NetBird, then approve. Until
## then the sync stays stopped, and the question is asked again each week.
@on(enters(stuck))
job resolve(s: Ref(Sync)) {
  let Some(step) = s.read!().doubtful else { return }
  while true {
    match approval!(
      "NetBird may or may not have applied {step}. Check NetBird, then approve to let the sync go on.",
      from: operator,
      within: 7 days,
    ) {
      Approved(checked) => {
        s.clear!(checked)
        return
      }
      Declined | Expired => sleep!(1 day)
    }
  }
}

# ---------------------------------------------------------------- admission

let guest: Choice = { id: "guest", label: "Guest" }
let member: Choice = { id: "member", label: "Member" }
let refuse: Choice = { id: "refuse", label: "Refuse" }

type Admission =
  | Admitted(groups: Set(String))
  | Refused
  | Unanswered

type AdmitError =
  | NotAsked(notify.Refusal)
  | NotApplied(SyncError)

## Someone new joined NetBird. The operator decides, one person at a time, in
## the order they joined.
@on(event(netbird.pending))
@concurrency(1)
job admit(user: Pending) -> Result(Admission, AdmitError) {
  let asked = notify.ask!(
    event: "netbird.approval",
    to: operator,
    text: "{user.email} joined NetBird. Let them in?",
    choices: [guest, member, refuse],
    within: 7 days,
    key: "netbird-approval-{user.id}",
  ).reconcile!()
  let question = match asked {
    Asking(q) => q
    NotAsked(reason) => return Err(NotAsked(reason))
  }
  let Some(answer) = next!(notify.answered, where: fn a: a.question == question, within: 7 days) else {
    return Ok(Unanswered)
  }
  let step = match answer.choice {
    "guest" => Admit(user: user.id, groups: toSet(["guests"]))
    "member" => Admit(user: user.id, groups: toSet([people, "members"]))
    _ => Reject(user: user.id)
  }
  apply!(step).mapErr(NotApplied)?
  Ok(match step {
    Admit(groups: groups, ..) => Admitted(groups)
    _ => Refused
  })
}

# ---------------------------------------------------------------- tests

let ann: PeerUser = {
  id: PeerUserId("p1"),
  email: "Ann@example.com",
  groups: {},
  service: false,
  blocked: false,
}

test "an active person joins net-people, matched by email in any case" {
  let members: List(Person) = [{ id: UserId("ann"), email: "ann@example.com", active: true, groups: {} }]
  expect(plan(members, [ann]) == [Join(user: PeerUserId("p1"), group: "net-people")])
}

test "the operator lets someone new in as a member" {
  with fakes(notify, netbird, from: "recordings/")
  netbird.pending.publish({ id: PeerUserId("p9"), email: "new@example.com", joinedAt: now() })
  expect(netbirdSync.applied == 0)
  notify.answered.publish({ question: QuestionId("q-9"), choice: "member", by: operator, at: now() })
  expect(netbirdSync.applied == 1)
}

test "a doubtful change stops the sync until the operator has checked it" {
  with fakes(authentik, netbird, from: "recordings/")
  reconcile.start()
  expect(netbirdSync.doubtful == Some(Join(user: PeerUserId("p1"), group: "net-people")))
  expect(netbirdSync.applied == 0)
  decline("NetBird may or may not have applied")
  advance(2 h)
  expect(netbirdSync.doubtful != None)
  approve("NetBird may or may not have applied", by: operator)
  advance(1 day)
  expect(netbirdSync.doubtful == None)
}

test "someone no longer active leaves, and service users are left alone" {
  let inside = { ann with groups: toSet(["net-people"]) }
  let robot = { inside with id: PeerUserId("p2"), service: true }
  expect(plan([], [inside, robot]) == [Leave(user: PeerUserId("p1"), group: "net-people")])
}
