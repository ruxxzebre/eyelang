## Approvals that reach people. A run that calls `approval!` waits for an
## answer; this module is how the program hears that it waits, and how the
## answer comes back.
##
## When a run starts waiting, a `requested` event is published. A job of the
## program sends it on, through Notify's buttons or anything else:
##
## ```eye
## @on(event(approvals.requested))
## job ask(r: approvals.Request) { … }
## ```
##
## An answer is an `answered` event. Publishing one wakes every run waiting for
## an approval whose text contains the answer's `what`. `eye approve` and
## `eye decline` publish one from the command line, and `approve(…)` and
## `decline(…)` in tests.

# SPECS.md §15 and open question 17. The runtime publishes `requested` and
# waits for `answered`; nothing here is a service.

## An approval a run waits for.
pub type Request = {
  ## Tells one request from another; a request is published once.
  id: String,
  ## What the run asks to have approved, as it wrote it.
  what: String,
  ## Whom it asks: the `from:` of `approval!`, as text.
  from: String,
  ## When the run stops waiting and takes the answer as Expired.
  until: Instant,
}

## A person's answer.
pub type Answer = {
  ## Text from the request's `what`; the answer goes to every waiting
  ## approval whose text contains it.
  what: String,
  approved: Bool,
  ## Who answered, as text.
  by: String,
}

## Approvals as events: the runtime publishes `requested`, and an `answered`
## event wakes the runs waiting.
contract Approvals {
  @key(id)
  event requested: Request on "eye.approvals.requested"

  event answered: Answer on "eye.approvals.answered"
}
