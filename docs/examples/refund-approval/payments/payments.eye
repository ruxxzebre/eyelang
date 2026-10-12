# The payment provider. This contract is the part of its API the refunds
# program calls.

## What the provider did with a refund.
pub type Refund =
  ## The money is on its way back to the customer.
  | Refunded(id: String)
  ## The provider can't refund this order, for example because its charge is
  ## too old.
  | NotRefundable(reason: String)

# region contract
contract Payments {
  ## Gives money back for an order. @destructive adds a parameter `approval`,
  ## and only approval!(…) makes an Approval, so no code can call this
  ## without a person having said yes.
  @destructive
  @key(required)
  @reconcile(refundFor)
  command refund(order: Int, cents: Int) -> Refund

  ## What the refund sent with this key did, if it arrived.
  query refundFor(key: Key) -> Option(Refund)
}
# endregion contract
