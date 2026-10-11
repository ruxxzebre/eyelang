# Invoices. Issuing an invoice charges the customer's card through the payment
# service, and texts them a receipt. Refunds arrive as events.

import http
import invoices.{InvoiceId, Status}
import payments.{ChargeId, CustomerId}
import sms

entity Invoice(id: InvoiceId) {
  state customer: Option(CustomerId) = None
  state amountCents: Int = 0
  state status: Status = Open
  ## Set when a text's reply was lost: a person checks whether it arrived.
  state receiptUnsure: Bool = false

  ## Records the invoice and starts collecting it, in one transaction.
  pub fn issue(c: CustomerId, cents: Int, phone: String) {
    if customer != None { return }
    customer = Some(c)
    amountCents = cents
    collect.start(id, c, cents, phone)
  }

  pub fn settle(s: Status) { status = s }

  pub fn refund(cents: Int) { status = Refunded(amountCents: cents) }

  pub fn unsure() { receiptUnsure = true }
}

# region serve
## The program serves its own contract: one function per operation, each run
## as one transaction. The key names the invoice, so a repeated call finds
## the invoice it already made.
impl invoices.Invoices {
  fn issue(customer, amountCents, phone, key) {
    if amountCents <= 0 { return Refused(reason: "an invoice must be for more than 0") }
    let id = InvoiceId("{key}")
    Invoice(id).issue(customer, amountCents, phone)
    Issued(invoice: id)
  }

  fn invoice(invoice) = if Invoice(invoice).customer == None then None else Some(Invoice(invoice).status)
}
# endregion serve

# region collect
## Charges the customer once. The key is the invoice's id, so however often
## this step runs, the payment service takes the money once.
job collect(id: InvoiceId, customer: CustomerId, cents: Int, phone: String) {
  let charge = payments.charge!(customer, cents, "Invoice {id}", key: "invoice-{id}")
    .reconcile!()
  match charge {
    Charged(chargeId, _) => {
      Invoice(id).settle!(Paid(charge: chargeId))
      receipt!(id, phone, "Paid: invoice {id}, {cents / 100}.{cents % 100 |> pad2} EUR.")
    }
    Declined(reason) => Invoice(id).settle!(Unpaid(reason:))
  }
}
# endregion collect

fn pad2(n: Int) -> String = if n < 10 then "0{n}" else "{n}"

# region receipt
## Texts a receipt. The gateway takes no key, so a lost reply can't be
## settled by asking: a person looks instead, and nothing is sent twice.
fn receipt(id: InvoiceId, phone: String, text: String) {
  match sms.text!(phone, text) {
    Queued(_) | BadNumber => ()
    Uncertain(_) => Invoice(id).unsure!()
  }
}
# endregion receipt

# region refunded
## A refund made in the payment service's dashboard arrives as an event. Its
## @key is the charge, so a refund delivered twice runs this once.
@on(event(payments.refunded))
job refunded(r: payments.Refunded) {
  let paid = Invoice.all |> find(fn i: i.status == Paid(charge: r.charge))
  let Some(invoice) = paid else { return }
  Invoice(invoice.id).refund!(r.amountCents)
}
# endregion refunded

# region balance
## A customer's balance, or a message to show when the service can't say.
pub fn balanceText(c: CustomerId) -> String uses {net Payments.balance} {
  match payments.balance(c) {
    Ok(cents) => "{cents} cents"
    Err(BadStatus(code, _)) => "the payment service answered {code}"
    Err(NoAnswer(_)) | Err(TimedOut) => "the payment service didn't answer"
  }
}
# endregion balance

# region status-page
## Whether the payment service's public status page says all is well. The
## page has no contract, so it is read with http.get.
pub fn serviceUp() -> Bool uses {wait, net, env} {
  let url = env.get("PAYMENTS_STATUS_URL") ?? "https://status.example.com"
  match http.get!(url, headers: { "Accept": "text/html" }) {
    Ok(page) => looksUp(page)
    Err(_) => false
  }
}

## Reads a status page's answer.
pub fn looksUp(page: http.Response) -> Bool =
  page.status == 200 && page.body |> contains("All systems operational")
# endregion status-page

# region main
## Issues one invoice from the command line, for example
## `eye run . -- i-1 ann 4250 "+353 85 000 0001"`.
pub fn main(args: List(String)) -> Result((), String) uses {entity Invoice} {
  let usage = "usage: eye run . -- INVOICE CUSTOMER CENTS PHONE"
  let (Some(id), Some(customer), Some(cents), Some(phone)) = (args[1], args[2], args[3], args[4]) else {
    return Err(usage)
  }
  let n = parseInt(cents).mapErr(fn _: usage)?
  Invoice(InvoiceId(id)).issue(CustomerId(customer), n, phone)
  Ok(())
}
# endregion main

# ---------------------------------------------------------------- tests

let ann = CustomerId("ann")

# region test-charged
test "a charge that goes through marks the invoice paid" {
  with fakes(payments, sms, from: "recordings/")
  let invoice = Invoice(InvoiceId("i-1"))
  invoice.issue(ann, 4_250, "+353 85 000 0001")
  expect(invoice.status == Paid(charge: ChargeId("ch-1")))
  expect(!invoice.receiptUnsure)
}
# endregion test-charged

# region test-declined
test "a declined card is an outcome, not an error" {
  with fakes(payments, sms, from: "recordings/")
  let invoice = Invoice(InvoiceId("i-2"))
  invoice.issue(CustomerId("bob"), 990, "+353 85 000 0002")
  expect(invoice.status == Unpaid(reason: "insufficient funds"))
}
# endregion test-declined

# region test-lost
test "a lost reply is settled by asking chargeFor, not by charging again" {
  with fakes(payments, sms, from: "recordings/")
  let invoice = Invoice(InvoiceId("i-3"))
  invoice.issue(CustomerId("cara"), 1_500, "+353 85 000 0003")
  expect(invoice.status == Paid(charge: ChargeId("ch-3")))
}
# endregion test-lost

# region test-unsure
test "a text whose reply was lost goes to a person, not out again" {
  with fakes(payments, from: "recordings/")
  with fakes(sms, from: "recordings/lost-text/")
  let invoice = Invoice(InvoiceId("i-1"))
  invoice.issue(ann, 4_250, "+353 85 000 0001")
  expect(invoice.status == Paid(charge: ChargeId("ch-1")))
  expect(invoice.receiptUnsure)
}
# endregion test-unsure

# region test-refund
test "a refund event marks the invoice refunded" {
  with fakes(payments, sms, from: "recordings/")
  Invoice(InvoiceId("i-1")).issue(ann, 4_250, "+353 85 000 0001")
  payments.refunded.publish({ charge: ChargeId("ch-1"), amountCents: 4_250 })
  expect(Invoice(InvoiceId("i-1")).status == Refunded(amountCents: 4_250))
}
# endregion test-refund

# region test-balance
test "a failure the query declares comes back as a value" {
  with fakes(payments, from: "recordings/")
  expect(balanceText(ann) == "the payment service answered 503")
}
# endregion test-balance

# region test-serve
test "the served contract bills once per key" {
  with fakes(payments, sms, from: "recordings/")
  let first = invoices.issue(ann, 4_250, "+353 85 000 0001", key: "i-1")
  let again = invoices.issue(ann, 4_250, "+353 85 000 0001", key: "i-1")
  expect(first == Issued(invoice: InvoiceId("i-1")))
  expect(again == first)
  expect(invoices.invoice(InvoiceId("i-1")) == Some(Paid(charge: ChargeId("ch-1"))))
  expect(invoices.invoice(InvoiceId("nope")) == None)
  expect(invoices.issue(ann, 0, "+353 85 000 0001", key: "i-9") == Refused(reason: "an invoice must be for more than 0"))
}
# endregion test-serve

# region test-json
type Reading = { at: Instant, every: Duration, note: Option(String), level: Level }

type Level =
  | Calm
  | Busy

test "values have one JSON shape" {
  let paid: payments.Charge = Charged(id: ChargeId("ch-1"), at: 2026-10-12 09:30 in "UTC")
  expect(paid |> toJson == "\{\"at\":\"2026-10-12T09:30:00Z\",\"id\":\"ch-1\",\"kind\":\"Charged\"\}")
  let reading: Reading = { at: 2026-10-12 09:30 in "UTC", every: 90 s, note: None, level: Busy }
  expect(reading |> toJson == "\{\"at\":\"2026-10-12T09:30:00Z\",\"every\":90000,\"level\":\"Busy\",\"note\":null\}")
  expect(Some(3) |> toJson == "3")
  expect([Calm, Busy] |> toJson == "[\"Calm\",\"Busy\"]")
}
# endregion test-json

# region test-page
test "a status page is read from its status and body" {
  let page: http.Response = { status: 200, headers: {}, body: "<p>All systems operational</p>" }
  expect(looksUp(page))
  expect(!looksUp({ page with status: 503 }))
}
# endregion test-page
