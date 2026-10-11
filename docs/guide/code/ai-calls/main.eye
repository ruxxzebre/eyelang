# Support triage. Each message that arrives is sorted by a model into a
# category and an urgency, with a draft reply when the answer is simple.

import ai

pub type TicketId = | TicketId(String)

# region types
## How a support message is sorted.
pub type Triage = {
  category: Category,
  urgency: Urgency,
  ## One sentence, at most 20 words, saying what the customer wants.
  summary: String,
  ## A reply to send as it is, only when the answer is simple and certain.
  draft: Option(String),
}

pub type Category =
  | Bug
  | Question
  | Billing

pub type Urgency =
  | Normal
  ## Someone can't work, or is losing money, right now.
  | Urgent(reason: String)
# endregion types

pub type Sorting =
  | Unsorted
  | Sorted(Triage)
  ## The model failed; a person sorts this one.
  | ByHand(why: String)

entity Ticket(id: TicketId) {
  state text: String = ""
  state sorting: Sorting = Unsorted

  ## Takes a message in and starts its triage, in one transaction.
  pub fn receive(message: String) {
    if text != "" { return }
    text = message
    triage.start(id, message)
  }

  pub fn sort(s: Sorting) { sorting = s }
}

# region ask
let instructions = """
  You sort messages sent to the support desk of a small web shop.
  Use only what the message says. Never promise refunds or dates.
  """

## Asks a model to sort a message. The type the result is used as, Triage,
## is the type the model is asked for.
fn sortMessage(text: String) -> Result(Triage, ai.AiError) {
  ai.generate!(model: "claude-sonnet-5-5", system: instructions, prompt: text)
}
# endregion ask

# region job
## Sorts one ticket. The model call is a recorded step: if the program
## restarts after it, the run goes on with the same answer and the model
## isn't asked, or paid, again.
job triage(id: TicketId, text: String) {
  match sortMessage!(text) {
    Ok(t) => Ticket(id).sort!(Sorted(t))
    Err(Malformed(_, problems)) => Ticket(id).sort!(ByHand(why: problems |> join("; ")))
    Err(e) => Ticket(id).sort!(ByHand(why: "{e}"))
  }
}
# endregion job

# region main
## Takes one message from the command line, for example
## `eye run . -- t-1 "I was charged twice for order 1182"`.
pub fn main(args: List(String)) -> Result((), String) uses {entity Ticket} {
  let (Some(id), Some(text)) = (args[1], args[2]) else {
    return Err("usage: eye run . -- TICKET MESSAGE")
  }
  Ticket(TicketId(id)).receive(text)
  Ok(())
}
# endregion main

# ---------------------------------------------------------------- tests

# region test-sorted
test "a recorded reply sorts the ticket" {
  with fakes(ai, from: "recordings/")
  let t = Ticket(TicketId("t-1"))
  t.receive("I was charged twice for order 1182.")
  let Sorted(triage) = t.sorting else { unreachable("the ticket should be sorted") }
  expect(triage.category == Billing)
  expect(triage.urgency == Urgent(reason: "the customer is losing money"))
  expect(triage.draft == None)
}
# endregion test-sorted

# region test-garbled
test "a reply that doesn't fit, twice, goes to a person" {
  with fakes(ai, from: "recordings/garbled/")
  let t = Ticket(TicketId("t-2"))
  t.receive("Where is my parcel?")
  expect(t.sorting == ByHand(why: "category: expected one of \"Bug\", \"Question\", \"Billing\", found \"Shipping\""))
}
# endregion test-garbled
