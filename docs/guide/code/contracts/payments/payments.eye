# The payment service takes money from customers' cards. This contract is its
# interface: the invoices program calls it.

import http

pub type CustomerId = | CustomerId(String)

pub type ChargeId = | ChargeId(String)

# region types
## What the payment service did with a charge.
pub type Charge =
  ## The money was taken.
  | Charged(id: ChargeId, at: Instant)
  ## The card was declined, and nothing was taken.
  | Declined(reason: String)

## Money given back to a customer, in part or in full.
pub type Refunded = { charge: ChargeId, amountCents: Int }
# endregion types

# region contract
@http(names: snake)
@auth(bearer: "payments-token")
contract Payments {
  ## Takes money from a customer's card. The service keeps what each key did.
  @http("POST /v1/charges", status: { 201: Charged, 402: Declined })
  @key(required)
  @reconcile(chargeFor)
  command charge(customer: CustomerId, amountCents: Int, description: String) -> Charge

  ## What the charge with this key did, or None if it never arrived.
  @http("GET /v1/charges/by-key/:key")
  query chargeFor(key: Key) -> Option(Charge)

  ## A customer's balance in cents. A failure comes back as an Err instead
  ## of being retried.
  @http("GET /v1/customers/:customer/balance")
  query balance(customer: CustomerId) -> Result(Int, http.HttpError)

  @key(charge)
  event refunded: Refunded on "payments.refunds"
}
# endregion contract
