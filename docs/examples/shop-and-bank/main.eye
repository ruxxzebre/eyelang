# A shop that charges its customers through the bank's Payments contract. Run
# it: the bank, the program in bank/, answers each call in the page, and the
# output shows every call as it crosses. Test it: the tests answer the calls
# from recordings instead.

import payments.{Charged, CustomerId, Declined}

entity Order(id: String) {
  state status: String = "open"

  pub fn settle(s: String) { status = s }
}

## Charges a customer for an order, once however often it runs: the key
## names the order.
job checkout(customer: CustomerId, order: String, cents: Int) uses {entity Order, net Payments.charge, stdio, wait} {
  let charge = payments.charge!(customer, cents, key: "order-{order}").reconcile!()
  let status = match charge {
    Charged(left) => "paid {cents} cents, {left} left"
    Declined(reason) => "declined: {reason}"
  }
  Order(order).settle!(status)
  print("order {order}: {status}")
}

## Says what a customer has left.
job report(customer: CustomerId) uses {net Payments.balance, stdio, wait} {
  print("{customer} has {payments.balance!(customer)} cents")
}

pub fn main() uses {start checkout, start report} {
  let ann = CustomerId("ann")
  checkout.start(ann, "1", 2_500, key: "checkout-1")
  # The same order again: the bank answers from its ledger.
  checkout.start(ann, "1", 2_500, key: "checkout-1-again")
  checkout.start(ann, "2", 9_000, key: "checkout-2")
  report.start(ann, key: "report")
}

test "an order is paid once, and a declined one says why" {
  with fakes(payments, from: "recordings/")
  checkout.start(CustomerId("ann"), "1", 2_500, key: "t1")
  checkout.start(CustomerId("ann"), "2", 9_000, key: "t2")
  expect(Order("1").status == "paid 2500 cents, 7500 left")
  expect(Order("2").status == "declined: only 7500 cents left")
}
