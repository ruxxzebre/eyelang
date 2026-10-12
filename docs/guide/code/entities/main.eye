# Guide: entities. Prepaid credit for model calls, one wallet per team.

# region wallet
type SpendError = | Short(balance: Int)

## A team's prepaid credit, in cents.
entity Wallet(team: String) {
  state cents: Int = 0
  state spent: Int = 0
  state warned: Bool = false

  pub fn topUp(n: Int) {
    cents += n
    warned = false
  }

  pub fn spend(n: Int) -> Result((), SpendError) {
    if n > cents { return Err(Short(balance: cents)) }
    cents -= n
    spent += n
    Ok(())
  }
}
# endregion wallet

# region test-wallet
test "a wallet keeps its state between calls" {
  let ops = Wallet("ops")
  ops.topUp(1000)
  expect(ops.spend(300) == Ok(()))
  expect(ops.spend(800) == Err(Short(balance: 700)))
  expect(ops.cents == 700 && ops.spent == 300)
}
# endregion test-wallet

# region identity
test "the identity is the entity" {
  Wallet("ops").topUp(200)
  expect(Wallet("ops").cents == 200)
  expect(Wallet("research").cents == 0)  # exists from its first use
}
# endregion identity

# region ledger
## Moves credit from one team to another. Both wallets change in one
## transaction, or neither does.
entity Ledger(name: String) {
  state moves: Int = 0

  pub fn move(from: Ref(Wallet), to: Ref(Wallet), n: Int) -> Result((), SpendError) {
    from.spend(n)?
    to.topUp(n)
    moves += 1
    Ok(())
  }
}
# endregion ledger

# region test-ledger
test "nested calls commit together" {
  let ops = Wallet("ops")
  let research = Wallet("research")
  ops.topUp(1000)
  expect(Ledger("main").move(ops, research, 400) == Ok(()))
  expect(ops.cents == 600 && research.cents == 400)
  expect(Ledger("main").move(research, ops, 900) == Err(Short(balance: 400)))
  expect(Ledger("main").moves == 1)
}
# endregion test-ledger

# region snapshot
## What a team sees on its dashboard. It takes a snapshot, a plain value, so
## it can't change the wallet.
fn describe(w: Wallet) -> String {
  let left = "{w.team}: {w.cents} cents left"
  if w.spent == 0 { left } else { "{left}, {w.spent} spent" }
}
# endregion snapshot

# region test-snapshot
test "a snapshot doesn't change when the entity does" {
  let ops = Wallet("ops")
  ops.topUp(500)
  let before = ops.read()
  let _ = ops.spend(200)
  expect(before.cents == 500 && before.version == 1)
  expect(ops.read().version == 2)
  expect(describe(before) == "ops: 500 cents left")
  expect(describe(ops.read()) == "ops: 300 cents left, 200 spent")
}
# endregion test-snapshot

# region quota
## Like Wallet, but it warns the team once when it runs low.
entity Quota(team: String) {
  state cents: Int = 0
  state warned: Bool = false

  pub fn topUp(n: Int) {
    cents += n
    warned = false
  }

  pub fn spend(n: Int) -> Result((), SpendError) {
    if n > cents { return Err(Short(balance: cents)) }
    cents -= n
    if cents < 100 && !warned {
      warned = true
      warnLow.start(team)
    }
    Ok(())
  }
}

job warnLow(team: String) {
  let q = Quota(team).read!()
  Outbox(team).add!("only {q.cents} cents left")
}

## The messages a team was sent. A real program would send them with a
## contract; this one keeps them so the tests can look.
entity Outbox(team: String) {
  state sent: List(String) = []

  pub fn add(text: String) { sent += [text] }
}
# endregion quota

# region test-quota
test "a method starts a job when it commits" {
  let q = Quota("ops")
  q.topUp(500)
  let _ = q.spend(450)
  let _ = q.spend(20)
  expect(Outbox("ops").sent == ["only 50 cents left"])
}
# endregion test-quota

# region close
## A team's access to the models. Closing it deletes its data, so it needs
## a person's approval.
entity Team(name: String) {
  state members: List(String) = []

  pub fn join(who: String) { members += [who] }

  pub fn close(ok: Approval) { end(ok) }
}

job closeTeam(name: String) {
  let decision = approval!("Close team {name}?", from: "finance", within: 2 days)
  let Approved(ok) = decision else { return }
  Team(name).close!(ok)
}
# endregion close

# region test-close
test "a team ends once finance approves" {
  Team("ops").join("ann")
  closeTeam.start("ops")
  approve("Close team ops?")
  expect(Team.all |> size == 0)
}
# endregion test-close

# region all
test "Wallet.all is every wallet" {
  Wallet("ops").topUp(50)
  Wallet("research").topUp(5000)
  let low = Wallet.all |> filter(fn w: w.cents < 100) |> map(fn w: w.team)
  expect(low == ["ops"])
}
# endregion all

# region versions
## Version 1 kept whole credits as `credits`; version 2 keeps cents.
@version(2)
entity Account(team: String) {
  state cents: Int = 0
  ## Added later with a default: needs no new version.
  state frozen: Bool = false

  pub fn add(n: Int) { cents += n }

  migrate from 1 (old) {
    { cents: old.credits * 100 }
  }
}
# endregion versions

test "a new account starts at the current version" {
  Account("ops").add(250)
  expect(Account("ops").cents == 250 && !Account("ops").frozen)
}
