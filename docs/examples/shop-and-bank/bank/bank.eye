# The bank. It serves Payments: each operation is one function, run as one
# transaction. In the playground the shop's calls reach it in the page, with
# no network, as two `eye run`s would on real ports.

import payments.{Charge, Charged, CustomerId, Declined}

entity Account(id: CustomerId) {
  state balanceCents: Int = 10_000

  pub fn take(cents: Int) -> Charge {
    if cents > balanceCents { return Declined(reason: "only {balanceCents} cents left") }
    balanceCents -= cents
    Charged(balanceCents:)
  }
}

## What each charge did, by its key, so a charge sent again is answered from
## here and takes nothing.
entity Ledger(key: Key) {
  state charge: Option(Charge) = None

  pub fn record(c: Charge) { charge = Some(c) }
}

impl payments.Payments {
  fn charge(customer, amountCents, key) {
    match Ledger(key).charge {
      Some(done) => done
      None => {
        let c = Account(customer).take(amountCents)
        Ledger(key).record(c)
        match c {
          # Sent to the shop when this transaction commits.
          Charged(_) => payments.charged.publish({ charge: "{key}", customer, amountCents })
          Declined(_) => ()
        }
        c
      }
    }
  }

  fn chargeFor(key) = Ledger(key).charge

  fn balance(customer) = Account(customer).balanceCents
}

test "a charge sent twice takes the money once" {
  let ann = CustomerId("ann")
  expect(Account(ann).take(2_500) == Charged(balanceCents: 7_500))
  expect(Account(ann).take(9_000) == Declined(reason: "only 7500 cents left"))
}
