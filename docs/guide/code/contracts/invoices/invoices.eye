# The invoices program's own interface: the shop calls it to bill a customer.
# The program in the directory above serves it.

import payments.{ChargeId, CustomerId}

pub type InvoiceId = | InvoiceId(String)

## Where an invoice stands.
pub type Status =
  | Open
  | Paid(charge: ChargeId)
  | Unpaid(reason: String)
  | Refunded(amountCents: Int)

## What issuing an invoice did.
pub type Issue =
  | Issued(invoice: InvoiceId)
  | Refused(reason: String)

# region contract
@http(names: snake)
contract Invoices {
  ## Bills a customer. The key names the invoice, so issuing it again with the
  ## same key bills the customer once and gives the same answer.
  @http("POST /v1/invoices")
  @key(required)
  @reconcile(issue)
  command issue(customer: CustomerId, amountCents: Int, phone: String) -> Issue

  ## Where an invoice stands, or None if there is no invoice with this id.
  @http("GET /v1/invoices/:invoice")
  query invoice(invoice: InvoiceId) -> Option(Status)
}
# endregion contract
