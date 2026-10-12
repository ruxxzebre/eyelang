# Trial reminders. A sign-up gets a 14-day trial, and one email three days
# before it ends, unless they have upgraded by then. Extending a trial moves
# the email; nothing polls a table to find it.

import mail

## How long a trial lasts.
let trialLength = 14 days

## How long before the end of a trial its reminder goes.
let notice = 3 days

# region account
pub type Plan =
  | NotSignedUp
  | Trial(ends: Instant)
  | Paid

entity Account(id: String) {
  state email: String = ""
  state plan: Plan = NotSignedUp
  state bounced: Option(String) = None

  ## Starts the trial and its reminder, in one transaction. Signing up again
  ## changes nothing and starts nothing.
  pub fn signUp(address: String, at: Instant) uses {start remind} {
    if plan != NotSignedUp { return }
    email = address
    plan = Trial(ends: at + trialLength)
    remind.start(id, key: "trial-{id}")
  }

  pub fn upgrade() { plan = Paid }

  pub fn extend(by: Duration) {
    let Trial(ends) = plan else { return }
    plan = Trial(ends: ends + by)
  }

  pub fn bounce(reason: String) { bounced = Some(reason) }
}
# endregion account

# region remind
## Sleeps until three days before the trial ends, then emails the customer.
## The trial can change while the run sleeps, so it reads the account again
## each time it wakes: an upgrade ends the run, and an extension sends it
## back to sleep.
job remind(id: String) {
  while true {
    let account = Account(id).read!()
    let Trial(ends) = account.plan else { return }
    if now!() >= ends - notice {
      email!(id, account.email)
      return
    }
    sleepUntil!(ends - notice)
  }
}
# endregion remind

## Sends the reminder. The key is the account's, so it goes once.
fn email(id: String, to: String) {
  let text = "Your trial ends in 3 days. Upgrade to keep your projects: https://example.com/upgrade"
  match mail.send!(to:, subject: "Your trial ends soon", text:, key: "trial-{id}").reconcile!() {
    Sent(_) => ()
    Bounced(reason) => Account(id).bounce!(reason)
  }
}

## Signs one customer up from the command line, for example
## `eye run . -- ann ann@example.com`.
pub fn main(args: List(String)) -> Result((), String) uses {clock, entity Account, start remind, stdio} {
  let (Some(id), Some(email)) = (args[1], args[2]) else {
    return Err("usage: eye run . -- ID EMAIL")
  }
  Account(id).signUp(email, now())
  print("{id}'s trial ends at {now() + trialLength}; the reminder goes 3 days before")
  Ok(())
}

# ---------------------------------------------------------------- tests

# region tests
test "a trial gets one reminder, three days before it ends" {
  let sent = fakes(mail, from: "recordings/")
  with sent
  with clock(2026-10-01 09:00 in "UTC")
  Account("ann").signUp("ann@example.com", now())
  advance(10 days + 23 h)
  expect(sent.calls(mail.send) == [])
  advance(1 h)  # 2026-10-12 09:00: the trial ends on the 15th
  let [reminder] = sent.calls(mail.send) else { unreachable("not one reminder") }
  expect(reminder.to == "ann@example.com")
  advance(30 days)
  expect(sent.calls(mail.send) |> size == 1)
}

test "an upgrade means no reminder" {
  let sent = fakes(mail, from: "recordings/")
  with sent
  Account("ann").signUp("ann@example.com", now())
  advance(5 days)
  Account("ann").upgrade()
  advance(30 days)
  expect(sent.calls(mail.send) == [])
}

test "an extension moves the reminder" {
  let sent = fakes(mail, from: "recordings/")
  with sent
  Account("ann").signUp("ann@example.com", now())
  advance(5 days)
  Account("ann").extend(7 days)
  advance(12 days)  # day 17: the old reminder's day, 11, has passed
  expect(sent.calls(mail.send) == [])
  advance(1 day)  # day 18: three days before the new end, day 21
  expect(sent.calls(mail.send) |> size == 1)
}
# endregion tests

test "a reminder that bounces is kept on the account" {
  with fakes(mail, from: "recordings/")
  Account("bo").signUp("bo@example.com", now())
  advance(11 days)
  expect(Account("bo").bounced == Some("no such mailbox"))
}
