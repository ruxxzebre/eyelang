# Invoices. When an invoice goes out, the customer gets one reminder three
# days later, unless they have paid by then.

import mail

# region invoice
entity Invoice(id: String) {
  state email: String = ""
  state paid: Bool = false

  ## Sends the invoice and starts its reminder, in one transaction.
  pub fn issue(to: String) uses {start remind} {
    email = to
    remind.start(id, key: "remind-{id}")
  }

  pub fn pay() { paid = true }
}
# endregion invoice

# region job
## Waits three days, then reminds the customer unless they have paid.
job remind(id: String) {
  sleep!(3 days)
  let invoice = Invoice(id).read!()
  if invoice.paid { return }
  let text = "Invoice {id} is still open."
  match mail.send!(to: invoice.email, text:, key: "remind-{id}").reconcile!() {
    Sent(_) => ()
    Bounced(_) => log.warn("the reminder for {id} bounced")
  }
}
# endregion job

# region main
## Issues one invoice from the command line, for example
## `eye run -- inv-1 ann@example.com`.
pub fn main(args: List(String)) -> Result((), String) uses {entity Invoice, start remind} {
  let (Some(id), Some(email)) = (args[1], args[2]) else {
    return Err("usage: eye run -- INVOICE EMAIL")
  }
  Invoice(id).issue(email)
  Ok(())
}
# endregion main

# region test
test "a paid invoice gets no reminder" {
  let sent = fakes(mail, from: "recordings/")
  with sent
  with clock(2026-10-12 09:00 in "UTC")
  Invoice("inv-1").issue("ann@example.com")
  advance(2 days)
  Invoice("inv-1").pay()
  advance(1 day)
  expect(sent.calls(mail.send) == [])
}

test "an unpaid invoice gets one reminder, three days later" {
  let sent = fakes(mail, from: "recordings/")
  with sent
  with clock(2026-10-12 09:00 in "UTC")
  Invoice("inv-2").issue("bob@example.com")
  Invoice("inv-2").issue("bob@example.com")  # same key: no second reminder
  advance(71 h)
  expect(sent.calls(mail.send) == [])
  advance(1 h)
  expect(sent.calls(mail.send) == [{ to: "bob@example.com", text: "Invoice inv-2 is still open.", key: "remind-inv-2" }])
}
# endregion test
