# The contract between the shop and the bank: what the bank serves and the shop
# calls. Both programs import this module.

pub type CustomerId = | CustomerId(String)

## What the bank did with a charge.
pub type Charge =
  ## The money was taken.
  | Charged(balanceCents: Int)
  ## Nothing was taken.
  | Declined(reason: String)

## A charge that took money, which the bank tells its subscribers.
pub type Receipt = { charge: String, customer: CustomerId, amountCents: Int }

@port(8090)
@http(names: snake)
@auth(bearer: "payments-token")
contract Payments {
  ## Takes money from a customer's account. The key names the charge, so a
  ## charge sent twice takes the money once.
  @http("POST /v1/charges", status: { 201: Charged, 402: Declined })
  @key(required)
  @reconcile(chargeFor)
  command charge(customer: CustomerId, amountCents: Int) -> Charge

  ## What the charge with this key did, or None if it never arrived.
  @http("GET /v1/charges/by-key/:key")
  query chargeFor(key: Key) -> Option(Charge)

  ## A customer's balance in cents.
  @http("GET /v1/customers/:customer/balance")
  query balance(customer: CustomerId) -> Int

  ## Published when a charge takes money. A program that waits for it gets
  ## it as `POST /events/payments.charged`; the key makes a repeat start
  ## nothing.
  @key(charge)
  event charged: Receipt on "payments.charged"
}
