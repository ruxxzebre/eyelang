# Refund approvals. Support asks for a refund; finance says yes or no within
# a day; if nobody in finance answers, the head of finance is asked; then the
# money goes back and the customer hears what happened. The payment
# provider's refund needs an Approval, so the compiler rejects any path that
# skips the person.

import mail
import payments

# region request
pub type Request = { order: Int, cents: Int, customer: String, reason: String }

pub type Status =
  | New
  | Waiting
  | Done(refund: String, approvedBy: String)
  | Refused(why: String)
  ## Nobody answered within two days.
  | Unanswered

entity RefundRequest(id: String) {
  state status: Status = New

  ## Takes a request in and starts its review, in one transaction. Asking
  ## again with the same id starts nothing.
  pub fn submit(r: Request) uses {start review} {
    if status != New { return }
    status = Waiting
    review.start(id, r, key: "review-{id}")
  }

  pub fn close(s: Status) { status = s }
}
# endregion request

## Writes cents as dollars: 25000 is "$250.00".
pub fn dollars(cents: Int) -> String {
  let rest = cents % 100
  "${cents / 100}.{if rest < 10 then "0{rest}" else "{rest}"}"
}

# region review
## Asks finance, and the head of finance if finance doesn't answer within a
## day. Each wait is a durable step: the run sleeps on disk for as long as it
## takes, through restarts, and holds no memory while it does.
fn decide(what: String) -> Decision {
  match approval!(what, from: "finance", within: 1 day) {
    Expired => approval!("{what} (finance didn't answer)", from: "head-of-finance", within: 1 day)
    answered => answered
  }
}

## Reviews one request, refunds it if a person approved, and tells the
## customer.
job review(id: String, r: Request) {
  let status = match decide!("Refund {dollars(r.cents)} on order #{r.order}: {r.reason}") {
    Approved(ok) => refund!(id, r, ok)
    Declined => Refused(why: "declined")
    Expired => Unanswered
  }
  RefundRequest(id).close!(status)
  tell!(id, r, status)
}

## Gives the money back. payments.refund takes the Approval that decide! got;
## without one, this doesn't compile.
fn refund(id: String, r: Request, ok: Approval) -> Status {
  match payments.refund!(r.order, r.cents, approval: ok, key: "refund-{id}").reconcile!() {
    Refunded(refund) => Done(refund:, approvedBy: ok.by)
    NotRefundable(reason) => Refused(why: reason)
  }
}
# endregion review

## Tells the customer how their refund went. The key is the request's, so
## they hear once. An unanswered request is support's to follow up, so the
## customer isn't told it was refused.
fn tell(id: String, r: Request, s: Status) {
  let text = match s {
    Done(..) => "We have refunded {dollars(r.cents)} for order #{r.order}."
    Refused(..) => "We can't refund order #{r.order}. Reply to this email to talk to us."
    New | Waiting | Unanswered => return
  }
  match mail.send!(to: r.customer, subject: "Your refund", text:, key: "refund-{id}").reconcile!() {
    Sent(_) => ()
    Bounced(reason) => log.warn("the refund email for {id} bounced: {reason}")
  }
}

# ---------------------------------------------------------------- tests

let broken: Request = { order: 1182, cents: 25_000, customer: "ann@example.com", reason: "arrived broken" }

# region tests
test "finance approves, and the money goes back once" {
  let payer = fakes(payments, from: "recordings/")
  with payer
  with fakes(mail, from: "recordings/")
  RefundRequest("r-1").submit(broken)
  RefundRequest("r-1").submit(broken)
  approve("Refund $250.00 on order #1182")
  expect(RefundRequest("r-1").status == Done(refund: "re-1", approvedBy: "finance"))
  expect(payer.calls(payments.refund) |> size == 1)
}

test "only the person asked can approve" {
  with fakes(payments, mail, from: "recordings/")
  RefundRequest("r-2").submit(broken)
  approve("Refund $250.00", by: "support")
  expect(RefundRequest("r-2").status == Waiting)
  decline("Refund $250.00")
  expect(RefundRequest("r-2").status == Refused(why: "declined"))
}

test "when finance doesn't answer within a day, the head of finance is asked" {
  with fakes(payments, mail, from: "recordings/")
  RefundRequest("r-3").submit(broken)
  advance(1 day)
  approve("finance didn't answer")
  expect(RefundRequest("r-3").status == Done(refund: "re-3", approvedBy: "head-of-finance"))
}

test "when nobody answers, nothing is refunded and support follows up" {
  let payer = fakes(payments, from: "recordings/")
  let sent = fakes(mail, from: "recordings/")
  with payer
  with sent
  RefundRequest("r-4").submit(broken)
  advance(2 days)
  expect(RefundRequest("r-4").status == Unanswered)
  expect(payer.calls(payments.refund) == [] && sent.calls(mail.send) == [])
}
# endregion tests

test "a refund the provider refuses is told to the customer" {
  with fakes(payments, mail, from: "recordings/")
  RefundRequest("r-5").submit(broken)
  approve("Refund $250.00")
  expect(RefundRequest("r-5").status == Refused(why: "the charge is more than 180 days old"))
}
