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
## An answer is an `answered` event that names its request's `id`. It goes to
## that request only, and counts only when its `by` is the request's `from`:
## the runtime refuses an answer from anyone else and logs it, and the run
## keeps waiting. So a job that passes on a button press can't grant consent
## for the wrong person, even if it never checks who pressed:
##
## ```eye
## approvals.answered.publish!({ id: r.id, approved: press.choice == yes.id, by: "{press.by}" })
## ```
##
## `eye approve` and `eye decline` publish one from the command line, and
## `approve(…)` and `decline(…)` in tests; both find the request by its text.

# SPECS.md §15 and open question 17. The runtime publishes `requested` and
# waits for `answered`; nothing here is a service.

## An approval a run waits for.
pub type Request = {
  ## Tells one request from another, and names it in an answer. A request
  ## is published once.
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
  ## The `id` of the request it answers.
  id: String,
  approved: Bool,
  ## Who answered, as text. The answer counts only when this is the
  ## request's `from`.
  by: String,
}

## Approvals as events: the runtime publishes `requested`, and an `answered`
## event wakes the run waiting for the request it names.
contract Approvals {
  @key(id)
  event requested: Request on "eye.approvals.requested"

  event answered: Answer on "eye.approvals.answered"
}
